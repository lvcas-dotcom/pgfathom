package infer

import (
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// applyEvidence folds join evidence into the generated set: a signal on the
// candidate that already exists, a new candidate when name matching never
// produced the pair. Runs before the threshold cut, so evidence-born
// candidates compete under the same rules as everyone.
func applyEvidence(res *Result, schemas []model.Schema, opts Options) {
	if len(opts.Evidence) == 0 {
		return
	}

	tables := tableIndex(schemas)
	existing := candidateIndex(res)

	for _, g := range groupEvidence(opts.Evidence) {
		// A composite join arrives as several equalities of one object. Read one
		// by one they are hypotheses about half a key each, which the key anchor
		// rejects — the relationship would go missing exactly where the evidence
		// is strongest. So the whole group is offered a key first.
		applyCompositeEvidence(res, existing, tables, g)

		for _, ev := range g.joins {
			// The pair is undirected; each side that holds the key anchors one
			// hypothesis. Both sides keyed means both directions are legitimate.
			applyOriented(res, existing, tables, ev, ev.Left, ev.Right)
			applyOriented(res, existing, tables, ev, ev.Right, ev.Left)
		}
	}
}

// evidenceGroup is every equality one object states about one pair of tables.
type evidenceGroup struct {
	object string
	source model.JoinSource
	left   string // table refs, ordered so the pair is undirected
	right  string
	joins  []model.JoinEvidence
}

// groupEvidence collects equalities by object and table pair, in a stable
// order. Two views stating the same composite join stay two groups, because
// they are two facts about the same relationship and the signal rule — one per
// origin — is applied where the candidate is.
func groupEvidence(evidence []model.JoinEvidence) []evidenceGroup {
	index := make(map[string]*evidenceGroup)
	var order []string

	for _, ev := range evidence {
		left, right := ev.Left.TableRef(), ev.Right.TableRef()
		if left > right {
			left, right = right, left
		}
		key := string(ev.Source) + "\x00" + ev.Object + "\x00" + left + "\x00" + right

		g, seen := index[key]
		if !seen {
			g = &evidenceGroup{object: ev.Object, source: ev.Source, left: left, right: right}
			index[key] = g
			order = append(order, key)
		}
		g.joins = append(g.joins, ev)
	}

	out := make([]evidenceGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *index[key])
	}
	return out
}

// applyCompositeEvidence anchors a group on a composite primary key, in either
// direction.
func applyCompositeEvidence(res *Result, existing map[string]int, tables map[string]model.Table, g evidenceGroup) {
	if g.left == g.right {
		// A self-join gives no way to tell which occurrence of the table is the
		// parent, and guessing would invent a direction the SQL never stated.
		return
	}
	applyCompositeOriented(res, existing, tables, g, g.left, g.right)
	applyCompositeOriented(res, existing, tables, g, g.right, g.left)
}

func applyCompositeOriented(res *Result, existing map[string]int, tables map[string]model.Table,
	g evidenceGroup, parentRef, childRef string) {

	parentTable, ok := tables[parentRef]
	if !ok || len(parentTable.PrimaryKey) < 2 {
		return
	}
	childTable, ok := tables[childRef]
	if !ok {
		return
	}

	mapping, ok := keyMapping(g, parentRef, childRef)
	if !ok {
		return
	}

	// Every position of the key, or no anchor at all: a join covering part of a
	// composite key says nothing about uniqueness, which is the whole reason a
	// key is what a candidate points at.
	childNames := make([]string, 0, len(parentTable.PrimaryKey))
	used := make(map[string]bool, len(parentTable.PrimaryKey))
	for _, k := range parentTable.PrimaryKey {
		name, found := mapping[strings.ToLower(k)]
		if !found || used[strings.ToLower(name)] {
			return
		}
		used[strings.ToLower(name)] = true
		childNames = append(childNames, name)
	}

	childCols, ok := keyColumns(childTable, childNames)
	if !ok {
		return
	}
	for _, col := range childCols {
		if !eligible(childTable, col) {
			return
		}
	}

	parentCols, ok := keyColumns(parentTable, parentTable.PrimaryKey)
	if !ok {
		return
	}
	match, ok := compareKeys(childCols, parentCols)
	if !ok {
		return
	}

	child := model.KeyRef{Schema: childTable.Schema, Table: childTable.Name, Columns: childNames}
	parent := model.KeyRef{Schema: parentTable.Schema, Table: parentTable.Name, Columns: parentTable.PrimaryKey}

	kind := model.JoinEvidence{Source: g.source}.SignalFor()
	signal := model.Signal{Kind: kind, Weight: joinWeight(kind), Detail: g.object}

	key := child.String() + "→" + parent.String()
	if i, found := existing[key]; found {
		if !res.Candidates[i].HasSignal(kind) {
			res.Candidates[i].Signals = append(res.Candidates[i].Signals, signal)
			Rescore(&res.Candidates[i])
		}
		return
	}

	signals := []model.Signal{signal, {
		Kind: model.SigCompositeArity, Weight: arityWeight(len(childCols)), Detail: parentTable.Ref(),
	}}

	if match == TypeIdentical {
		signals = append(signals, model.Signal{
			Kind: model.SigIdenticalType, Weight: weightIdenticalType, Detail: baseTypes(childCols),
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigCompatibleType, Weight: weightCompatibleType,
			Detail: baseTypes(childCols) + " into " + baseTypes(parentCols),
		})
	}
	if model.IndexLeads(childTable.Indexes, childNames) {
		signals = append(signals, model.Signal{
			Kind: model.SigChildIndexed, Weight: weightChildIndexed,
			Detail: strings.Join(childNames, ", "),
		})
	}
	if allNotNull(childCols) {
		signals = append(signals, model.Signal{Kind: model.SigNotNull, Weight: weightNotNull})
	}

	existing[key] = len(res.Candidates)
	res.Candidates = append(res.Candidates, model.Candidate{
		Child:     child,
		Parent:    parent,
		Signals:   signals,
		MetaScore: score(signals),
		Verdict:   model.VerdictUnvalidated,
		Reason:    "not validated against data",
	})
}

// keyMapping reads the group as parent column to child column. It reports
// false when the same parent column is equated to two different child columns,
// which is a join this reading cannot represent as a key.
func keyMapping(g evidenceGroup, parentRef, childRef string) (map[string]string, bool) {
	out := make(map[string]string, len(g.joins))

	for _, ev := range g.joins {
		p, c := ev.Left, ev.Right
		if p.TableRef() != parentRef {
			p, c = ev.Right, ev.Left
		}
		if p.TableRef() != parentRef || c.TableRef() != childRef {
			continue
		}

		k := strings.ToLower(p.Column)
		if prev, dup := out[k]; dup && !strings.EqualFold(prev, c.Column) {
			return nil, false
		}
		out[k] = c.Column
	}
	return out, true
}

func applyOriented(res *Result, existing map[string]int, tables map[string]model.Table,
	ev model.JoinEvidence, child, parent model.ColumnRef) {

	parentTable, ok := tables[parent.TableRef()]
	if !ok || !isSingleKey(parentTable, parent.Column) {
		return
	}
	childTable, ok := tables[child.TableRef()]
	if !ok {
		return
	}
	childCol, ok := childTable.Column(child.Column)
	if !ok || !eligible(childTable, childCol) {
		return
	}

	pkCol, ok := parentTable.Column(parent.Column)
	if !ok {
		return
	}
	match := CompareTypes(childCol.BaseType, pkCol.BaseType)
	if !match.Compatible() {
		return
	}

	kind := ev.SignalFor()
	signal := model.Signal{Kind: kind, Weight: joinWeight(kind), Detail: ev.Object}

	key := child.String() + "→" + parent.String()
	if i, found := existing[key]; found {
		// One signal per evidence kind: three views proving the same join are
		// one fact, and stacking them would let volume impersonate strength.
		if !res.Candidates[i].HasSignal(kind) {
			res.Candidates[i].Signals = append(res.Candidates[i].Signals, signal)
			Rescore(&res.Candidates[i])
		}
		return
	}

	signals := []model.Signal{signal}
	if match == TypeIdentical {
		signals = append(signals, model.Signal{
			Kind: model.SigIdenticalType, Weight: weightIdenticalType, Detail: childCol.BaseType,
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigCompatibleType, Weight: weightCompatibleType,
			Detail: childCol.BaseType + " into " + pkCol.BaseType,
		})
	}
	if childTable.IsIndexedLeading(childCol.Name) {
		signals = append(signals, model.Signal{
			Kind: model.SigChildIndexed, Weight: weightChildIndexed, Detail: childCol.Name,
		})
	}
	if !childCol.Nullable {
		signals = append(signals, model.Signal{Kind: model.SigNotNull, Weight: weightNotNull})
	}

	existing[key] = len(res.Candidates)
	res.Candidates = append(res.Candidates, model.Candidate{
		Child:     model.SingleKey(child.Schema, child.Table, child.Column),
		Parent:    model.SingleKey(parent.Schema, parent.Table, parent.Column),
		Signals:   signals,
		MetaScore: score(signals),
		Verdict:   model.VerdictUnvalidated,
		Reason:    "not validated against data",
	})
}

// isSingleKey reports whether the column is the table's single-column primary
// key — the anchor that gives a join pair a direction.
func isSingleKey(t model.Table, column string) bool {
	return len(t.PrimaryKey) == 1 && strings.EqualFold(t.PrimaryKey[0], column)
}

func joinWeight(kind model.SignalKind) float64 {
	switch kind {
	case model.SigJoinInFunction:
		return weightJoinInFunction
	case model.SigJoinInStatements:
		return weightJoinInStatements
	default:
		return weightJoinInView
	}
}

func tableIndex(schemas []model.Schema) map[string]model.Table {
	out := make(map[string]model.Table)
	for _, s := range schemas {
		for _, t := range s.Tables {
			out[s.Name+"."+t.Name] = t
		}
	}
	return out
}

func candidateIndex(res *Result) map[string]int {
	out := make(map[string]int, len(res.Candidates))
	for i, c := range res.Candidates {
		out[c.Child.String()+"→"+c.Parent.String()] = i
	}
	return out
}

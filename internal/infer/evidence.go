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

	for _, ev := range opts.Evidence {
		// The pair is undirected; each side that holds the key anchors one
		// hypothesis. Both sides keyed means both directions are legitimate.
		applyOriented(res, existing, tables, ev, ev.Left, ev.Right)
		applyOriented(res, existing, tables, ev, ev.Right, ev.Left)
	}
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
		Child:     child,
		Parent:    parent,
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

package infer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
)

// minPartialMatch is how many positions a failed match has to have resolved
// before it is worth reporting. One column of a key lining up says nothing —
// every schema has a hundred tables carrying an `empresa_id` — and reporting
// it would bury the two-of-three cases that are actually near misses.
const minPartialMatch = 2

// derivation is how a key as a whole was recognized.
type derivation int

const (
	// derivMirror: every position carries the key column's own name, as in
	// nota(empresa_id, filial_id) against empresa_filial(empresa_id, filial_id).
	// Nothing in the child names the target, so the key signature is the whole
	// of the evidence — which is why this one alone needs a unique target.
	derivMirror derivation = iota

	// derivAnchored: at least one position carries a form of the target's name
	// ahead of the key column — item(nota_empresa_id, nota_numero) — and the
	// remaining positions mirror. That covers the whole range from every
	// position anchored down to a single anchor beside discriminators, which is
	// what a composite key in a partitioned or multi-tenant schema looks like:
	// ci_build_pending_states(partition_id, build_id) against
	// p_ci_builds(id, partition_id).
	derivAnchored
)

// keyMatch is one complete resolution of a target's key against a child table.
type keyMatch struct {
	target  model.Table
	columns []string // child columns, in target key order

	derivation derivation
	origin     profile.Origin // meaningful only for derivAnchored
}

func (m keyMatch) signature() string {
	return strings.ToLower(strings.Join(m.columns, ","))
}

// generateComposite raises hypotheses against targets whose primary key spans
// more than one column.
//
// It walks child tables against targets rather than deriving targets from
// column names, because the mirror derivation has no name to derive from. The
// cost is one pass of children times composite targets, with a handful of map
// lookups each — a few million cheap operations on the largest schema measured,
// and none of them touching a database.
func generateComposite(res *Result, schemas []model.Schema, opts Options) {
	targets := compositeTargets(schemas, opts.Profile)
	if len(targets) == 0 {
		return
	}

	for _, s := range schemas {
		for _, t := range s.Tables {
			compositeFor(res, t, targets, opts)
		}
	}
}

// compositeTarget is a table a composite hypothesis can point at, carrying the
// forms of its own name.
//
// The forms are held here because every table in the schema is matched against
// every one of these, and deriving them inside that loop derives the same forms
// for the same target once per table in the database. On a schema with five
// thousand tables and five hundred composite keys that is two and a half
// million derivations of five hundred distinct answers.
type compositeTarget struct {
	table model.Table
	forms []profile.Form
}

// compositeTargets collects the tables a composite hypothesis can point at, in
// a stable order.
func compositeTargets(schemas []model.Schema, p *profile.Profile) []compositeTarget {
	var out []compositeTarget
	for _, s := range schemas {
		for _, t := range s.Tables {
			if len(t.PrimaryKey) < 2 {
				continue
			}
			if _, ok := keyColumns(t, t.PrimaryKey); ok {
				out = append(out, compositeTarget{table: t, forms: p.TableForms(t.Name)})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].table.Ref() < out[j].table.Ref() })
	return out
}

func compositeFor(res *Result, child model.Table, targets []compositeTarget, opts Options) {
	var matches []keyMatch

	for _, target := range targets {
		complete, partial := matchKey(child, target)

		switch {
		case len(complete) > 1:
			// Two readings resolving to different column sets leave no way to
			// say which key was meant. Choosing by position or by type would be
			// a hunch wearing a decision's clothes.
			res.Skipped = append(res.Skipped, Skip{
				Child:  model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: complete[0].columns},
				Target: target.table.Ref(),
				Reason: SkipAmbiguousPosition,
			})
		case len(complete) == 1:
			matches = append(matches, complete[0])
		case len(partial) >= minPartialMatch && len(partial) < len(target.table.PrimaryKey):
			res.Skipped = append(res.Skipped, Skip{
				Child:  model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: partial},
				Target: target.table.Ref(),
				Reason: SkipPartialKey,
				Detail: fmt.Sprintf("%d of %d positions", len(partial), len(target.table.PrimaryKey)),
			})
		}
	}

	for _, m := range resolveAmbiguity(res, child, matches) {
		emitComposite(res, child, m.match, m.ambiguous, opts)
	}
}

// matchKey resolves the target's key against the child's columns: once without
// any name to go on, and once per form of the target's name. It returns every
// complete resolution, and the columns of the best incomplete one for the
// near-miss report.
func matchKey(child model.Table, target compositeTarget) (complete []keyMatch, partial []string) {
	// Allocated on the first complete reading rather than up front. Most pairs
	// of tables have nothing to do with each other, and on a large schema this
	// is one map per pair against one map per match.
	var seen map[string]bool

	// A reading the child cannot offer is not a reading, so eligibility and type
	// are settled here rather than downstream. A rejected reading that survived
	// this far would count as a second interpretation and take a valid one down
	// with it as ambiguous — the child's own surrogate `id` mirroring a target's
	// `id` is exactly how that happens.
	add := func(m keyMatch, cols []string) {
		if len(cols) < len(target.table.PrimaryKey) {
			if len(cols) > len(partial) {
				partial = cols
			}
			return
		}
		// A table matching its own key is a tautology, not a relationship.
		if child.Ref() == target.table.Ref() && sameColumns(cols, target.table.PrimaryKey) {
			return
		}
		if !usable(child, target.table, cols) {
			return
		}
		m.columns = cols
		if seen[m.signature()] {
			return
		}
		if seen == nil {
			seen = make(map[string]bool, 2)
		}
		seen[m.signature()] = true
		complete = append(complete, m)
	}

	add(keyMatch{target: target.table, derivation: derivMirror}, resolveMirror(child, target.table))

	for _, f := range target.forms {
		cols, anchors := resolveAnchored(child, target.table, f.Value)
		if anchors == 0 {
			// Zero anchors is the mirror case, which the call above already
			// covered and which answers to a stricter rule.
			continue
		}
		add(keyMatch{target: target.table, derivation: derivAnchored, origin: f.Origin}, cols)
	}
	return complete, partial
}

// usable reports whether the child can actually offer this reading: every
// column eligible, and every position's type fitting its counterpart.
func usable(child, target model.Table, cols []string) bool {
	childCols, ok := keyColumns(child, cols)
	if !ok {
		return false
	}
	for _, c := range childCols {
		if !eligible(child, c) {
			return false
		}
	}

	parentCols, ok := keyColumns(target, target.PrimaryKey)
	if !ok {
		return false
	}
	_, ok = compareKeys(childCols, parentCols)
	return ok
}

// resolveMirror matches every position by the key column's own name.
func resolveMirror(child, target model.Table) []string {
	var cols []string
	for _, k := range target.PrimaryKey {
		col, ok := child.Column(k)
		if !ok {
			continue
		}
		cols = append(cols, col.Name)
	}
	return cols
}

// resolveAnchored matches each position either by the target's name ahead of
// the key column — an anchor — or by the key column's own name.
//
// One anchor is enough, and one anchor is required. It is what answers "why
// this table and not another"; the mirrored positions are discriminators like
// partition_id or empresa_id, columns that cross the whole schema and point
// nowhere on their own. Requiring every position to anchor was the rule this
// replaces, and it recovered none of the 53 composite keys in the corpus.
func resolveAnchored(child, target model.Table, form string) (cols []string, anchors int) {
	for _, k := range target.PrimaryKey {
		if col, ok := child.Column(form + "_" + k); ok {
			cols = append(cols, col.Name)
			anchors++
			continue
		}
		if col, ok := child.Column(k); ok {
			cols = append(cols, col.Name)
		}
	}
	return cols, anchors
}

// scoped pairs a match with whether its target had company.
type scoped struct {
	match     keyMatch
	ambiguous bool
}

// resolveAmbiguity decides what happens when the same child columns point at
// more than one target.
//
// An anchored match carries the target's name, so ambiguity there is the
// ordinary kind: the candidates survive and say they are ambiguous, exactly as
// at arity one. A mirror match carries no name at all, so several targets
// sharing a key signature would each be validated on their own merits — and
// more than one can reach total containment, which would confirm a
// relationship that does not exist. There is nothing to disambiguate with, so
// nothing is emitted.
func resolveAmbiguity(res *Result, child model.Table, matches []keyMatch) []scoped {
	bySignature := make(map[string][]keyMatch)
	order := make([]string, 0, len(matches))

	for _, m := range matches {
		sig := m.signature()
		if _, seen := bySignature[sig]; !seen {
			order = append(order, sig)
		}
		bySignature[sig] = append(bySignature[sig], m)
	}

	var out []scoped

	for _, sig := range order {
		group := bySignature[sig]
		if len(group) == 1 {
			out = append(out, scoped{match: group[0]})
			continue
		}

		if hasMirror(group) {
			for _, m := range group {
				res.Skipped = append(res.Skipped, Skip{
					Child:  model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: m.columns},
					Target: m.target.Ref(),
					Reason: SkipAmbiguousSignature,
					Detail: fmt.Sprintf("%d tables share this key signature", len(group)),
				})
			}
			continue
		}

		for _, m := range group {
			out = append(out, scoped{match: m, ambiguous: true})
		}
	}
	return out
}

func hasMirror(group []keyMatch) bool {
	for _, m := range group {
		if m.derivation == derivMirror {
			return true
		}
	}
	return false
}

// emitComposite turns a resolved match into a candidate, once the child
// columns are eligible and every position's type fits.
func emitComposite(res *Result, child model.Table, m keyMatch, ambiguous bool, opts Options) {
	// Eligibility and type were settled when the reading was accepted; what is
	// resolved again here is the columns themselves, for the signals.
	childCols, ok := keyColumns(child, m.columns)
	if !ok {
		return
	}
	parentCols, ok := keyColumns(m.target, m.target.PrimaryKey)
	if !ok {
		return
	}
	match, ok := compareKeys(childCols, parentCols)
	if !ok {
		return
	}

	signals := compositeSignals(child, childCols, m, parentCols, match, ambiguous)

	res.Candidates = append(res.Candidates, model.Candidate{
		Child:     model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: m.columns},
		Parent:    model.KeyRef{Schema: m.target.Schema, Table: m.target.Name, Columns: m.target.PrimaryKey},
		Signals:   signals,
		MetaScore: score(signals),
		Verdict:   model.VerdictUnvalidated,
		Reason:    "not validated against data",
	})
}

func compositeSignals(child model.Table, childCols []model.Column, m keyMatch,
	parentCols []model.Column, match TypeMatch, ambiguous bool) []model.Signal {

	signals := make([]model.Signal, 0, 6)
	names := columnNames(childCols)

	signals = append(signals, model.Signal{
		Kind:   model.SigCompositeArity,
		Weight: arityWeight(len(childCols)),
		Detail: m.target.Ref(),
	})

	// A mirror match gets no name signal, because it has no name evidence: what
	// it knows is that the key lines up, and that is the arity signal above.
	if m.derivation == derivAnchored {
		if m.origin.Exact() {
			signals = append(signals, model.Signal{
				Kind: model.SigExactName, Weight: weightExactName, Detail: m.target.Name,
			})
		} else {
			signals = append(signals, model.Signal{
				Kind: model.SigNormalizedName, Weight: weightNormalizedName,
				Detail: m.target.Name + " via " + m.origin.String(),
			})
		}
	}

	if match == TypeIdentical {
		signals = append(signals, model.Signal{
			Kind: model.SigIdenticalType, Weight: weightIdenticalType,
			Detail: baseTypes(childCols),
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigCompatibleType, Weight: weightCompatibleType,
			Detail: baseTypes(childCols) + " into " + baseTypes(parentCols),
		})
	}

	if ambiguous {
		signals = append(signals, model.Signal{
			Kind: model.SigAmbiguousTarget, Weight: penaltyAmbiguousTarget,
			Detail: strings.Join(m.target.PrimaryKey, ", "),
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigUniqueTarget, Weight: weightUniqueTarget, Detail: m.target.Ref(),
		})
	}

	if model.IndexLeads(child.Indexes, names) {
		signals = append(signals, model.Signal{
			Kind: model.SigChildIndexed, Weight: weightChildIndexed,
			Detail: strings.Join(names, ", "),
		})
	}

	if mentions(child.Comment, m.target.Name) {
		signals = append(signals, model.Signal{
			Kind: model.SigCommentMention, Weight: weightCommentMention, Detail: m.target.Name,
		})
	}

	if allNotNull(childCols) {
		signals = append(signals, model.Signal{Kind: model.SigNotNull, Weight: weightNotNull})
	}

	return signals
}

// compareKeys is the type verdict for the whole key: the weakest position
// decides, and one incompatible position brings the set down. Half a key is
// not a relationship with a column missing — it is something else.
func compareKeys(child, parent []model.Column) (TypeMatch, bool) {
	if len(child) != len(parent) {
		return TypeIncompatible, false
	}

	worst := TypeIdentical
	for i := range child {
		m := CompareTypes(child[i].BaseType, parent[i].BaseType)
		if !m.Compatible() {
			return TypeIncompatible, false
		}
		if m < worst {
			worst = m
		}
	}
	return worst, true
}

// keyColumns resolves column names against a table, in the order given.
func keyColumns(t model.Table, names []string) ([]model.Column, bool) {
	out := make([]model.Column, 0, len(names))
	for _, name := range names {
		col, ok := t.Column(name)
		if !ok {
			return nil, false
		}
		out = append(out, col)
	}
	return out, true
}

func columnNames(cols []model.Column) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.Name)
	}
	return out
}

func baseTypes(cols []model.Column) string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.BaseType)
	}
	return strings.Join(out, ", ")
}

func allNotNull(cols []model.Column) bool {
	for _, c := range cols {
		if c.Nullable {
			return false
		}
	}
	return true
}

func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

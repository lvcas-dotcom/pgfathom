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

// derivation is how a child column was recognized as standing for a column of
// the target's key.
type derivation int

const (
	// derivMirror: the child carries the key column's own name, as in
	// nota(empresa_id, filial_id) against empresa_filial(empresa_id, filial_id).
	// Nothing in the child names the target, so the key signature is the whole
	// of the evidence.
	derivMirror derivation = iota

	// derivPrefixed: a form of the target's name precedes each key column, as in
	// item(nota_empresa_id, nota_numero) against nota(empresa_id, numero). The
	// name anchors the hypothesis the way it does at arity one.
	derivPrefixed
)

// keyMatch is one complete resolution of a target's key against a child table.
type keyMatch struct {
	target  model.Table
	columns []string // child columns, in target key order

	derivation derivation
	origin     profile.Origin // meaningful only for derivPrefixed
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
	targets := compositeTargets(schemas)
	if len(targets) == 0 {
		return
	}

	for _, s := range schemas {
		for _, t := range s.Tables {
			compositeFor(res, t, targets, opts)
		}
	}
}

// compositeTargets collects the tables a composite hypothesis can point at, in
// a stable order.
func compositeTargets(schemas []model.Schema) []model.Table {
	var out []model.Table
	for _, s := range schemas {
		for _, t := range s.Tables {
			if len(t.PrimaryKey) < 2 {
				continue
			}
			if _, ok := keyColumns(t, t.PrimaryKey); ok {
				out = append(out, t)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ref() < out[j].Ref() })
	return out
}

func compositeFor(res *Result, child model.Table, targets []model.Table, opts Options) {
	var matches []keyMatch

	for _, target := range targets {
		complete, partial := matchKey(child, target, opts.Profile)

		switch {
		case len(complete) > 1:
			// Two derivations resolving to different column sets is the shape of
			// a coincidence, not of a key. Choosing between them by position or
			// by type would be a hunch wearing a decision's clothes.
			res.Skipped = append(res.Skipped, Skip{
				Child:  model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: complete[0].columns},
				Target: target.Ref(),
				Reason: SkipAmbiguousPosition,
			})
		case len(complete) == 1:
			matches = append(matches, complete[0])
		case len(partial) >= minPartialMatch && len(partial) < len(target.PrimaryKey):
			res.Skipped = append(res.Skipped, Skip{
				Child:  model.KeyRef{Schema: child.Schema, Table: child.Name, Columns: partial},
				Target: target.Ref(),
				Reason: SkipPartialKey,
				Detail: fmt.Sprintf("%d of %d positions", len(partial), len(target.PrimaryKey)),
			})
		}
	}

	for _, m := range resolveAmbiguity(res, child, matches) {
		emitComposite(res, child, m.match, m.ambiguous, opts)
	}
}

// matchKey resolves the target's key against the child's columns, once per
// available derivation. It returns every complete resolution, and the columns
// of the best incomplete one for the near-miss report.
func matchKey(child, target model.Table, p *profile.Profile) (complete []keyMatch, partial []string) {
	prefixes := []struct {
		value  string
		deriv  derivation
		origin profile.Origin
	}{{value: "", deriv: derivMirror}}

	for _, f := range p.TableForms(target.Name) {
		prefixes = append(prefixes, struct {
			value  string
			deriv  derivation
			origin profile.Origin
		}{value: f.Value + "_", deriv: derivPrefixed, origin: f.Origin})
	}

	seen := make(map[string]bool)

	for _, pre := range prefixes {
		var cols []string
		for _, k := range target.PrimaryKey {
			col, ok := child.Column(pre.value + k)
			if !ok {
				continue
			}
			cols = append(cols, col.Name)
		}

		if len(cols) < len(target.PrimaryKey) {
			if len(cols) > len(partial) {
				partial = cols
			}
			continue
		}

		// A table matching its own key is a tautology, not a relationship.
		if child.Ref() == target.Ref() && sameColumns(cols, target.PrimaryKey) {
			continue
		}

		m := keyMatch{target: target, columns: cols, derivation: pre.deriv, origin: pre.origin}
		if seen[m.signature()] {
			continue
		}
		seen[m.signature()] = true
		complete = append(complete, m)
	}
	return complete, partial
}

// scoped pairs a match with whether its target had company.
type scoped struct {
	match     keyMatch
	ambiguous bool
}

// resolveAmbiguity decides what happens when the same child columns point at
// more than one target.
//
// A prefixed match carries the target's name, so ambiguity there is the
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
	childCols, ok := keyColumns(child, m.columns)
	if !ok {
		return
	}
	for _, col := range childCols {
		if !eligible(child, col) {
			return
		}
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
	if m.derivation == derivPrefixed {
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

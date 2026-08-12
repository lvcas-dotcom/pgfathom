// Package audit derives findings that require no inference at all.
//
// Everything here comes straight from the catalog and from usage evidence
// already resolved against it: deterministic, free of guesswork, and immune
// to false positives. Every finding it emits is a fact — a missing key, a
// column real code repeatedly names with no index leading it — not a
// hypothesis about the data. This package never opens a transaction and never
// reads a row; the one probe that does, confirming a candidate key by
// counting, lives in internal/validate and is costured in by internal/cli.
package audit

import (
	"sort"
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// Options carries the usage evidence and environment the evidence-based
// findings need. Everything here is catalog output or already-resolved
// extractor output — no table data reaches this package.
type Options struct {
	Joins      []model.JoinEvidence
	Predicates []model.PredicateEvidence
	Extensions model.ExtensionSet

	// RecurrenceMin is how many distinct objects (views, functions,
	// statements) must name a column before it counts as hot. Values below 1
	// are treated as 1.
	RecurrenceMin int
}

// Findings collects the structural findings for a set of schemas.
func Findings(schemas []model.Schema, opts Options) []model.Finding {
	var out []model.Finding

	for _, s := range schemas {
		for _, t := range s.Tables {
			out = append(out, notValidConstraints(t)...)
			out = append(out, unindexedForeignKeys(t, schemas)...)
			out = append(out, missingPrimaryKeys(t)...)
		}
	}

	out = append(out, unindexedHotColumns(schemas, opts)...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Object < out[j].Object
	})

	return out
}

func notValidConstraints(t model.Table) []model.Finding {
	var out []model.Finding

	for _, fk := range t.ForeignKeys {
		if fk.Validated {
			continue
		}
		out = append(out, model.Finding{
			Kind:   model.FindingNotValidConstraint,
			Object: t.Ref() + "." + fk.Name,
			Detail: "constraint is NOT VALID: it blocks new violations but never " +
				"checked the rows that were already there",
			Metrics: map[string]int64{"child_estimated_rows": t.Stats.EstimatedRows},
		})
	}

	return out
}

func unindexedForeignKeys(t model.Table, schemas []model.Schema) []model.Finding {
	var out []model.Finding

	for _, fk := range t.ForeignKeys {
		if fk.HasIndex {
			continue
		}

		metrics := map[string]int64{"child_estimated_rows": t.Stats.EstimatedRows}
		if parent, ok := findTable(schemas, fk.RefSchema, fk.RefTable); ok {
			metrics["parent_estimated_rows"] = parent.Stats.EstimatedRows
		}

		out = append(out, model.Finding{
			Kind:   model.FindingFKWithoutIndex,
			Object: t.Ref() + "." + fk.Name,
			Detail: "no index leads with " + fk.Columns[0] + ": every delete on " +
				fk.RefSchema + "." + fk.RefTable + " scans this table sequentially",
			Metrics: metrics,
		})
	}

	return out
}

func findTable(schemas []model.Schema, schema, name string) (model.Table, bool) {
	for _, s := range schemas {
		if s.Name != schema {
			continue
		}
		for _, t := range s.Tables {
			if t.Name == name {
				return t, true
			}
		}
	}
	return model.Table{}, false
}

// missingPrimaryKeys reports a table with no primary key. When the catalog
// already proves a UNIQUE NOT NULL constraint exists, promoting it is offered
// as a zero-cost fix. Otherwise the suggestion stands without columns: naming
// a candidate key requires reading data, which is internal/cli's job to
// costure in via internal/validate, not this package's.
func missingPrimaryKeys(t model.Table) []model.Finding {
	if t.HasPrimaryKey() {
		return nil
	}

	f := model.Finding{
		Kind:    model.FindingMissingPrimaryKey,
		Object:  t.Ref(),
		Metrics: map[string]int64{"estimated_rows": t.Stats.EstimatedRows},
	}

	if u, ok := t.PromotableUnique(); ok {
		f.Detail = "no primary key: row identity is undefined, but an existing " +
			"UNIQUE NOT NULL constraint can be promoted at no cost"
		f.Suggestion = &model.Suggestion{
			Kind:    model.SuggestPromoteUnique,
			Columns: u.Columns,
		}
	} else {
		f.Detail = "no primary key: row identity is undefined, no logical replication " +
			"covers it, and every per-row update or delete scans sequentially"
		f.Suggestion = &model.Suggestion{Kind: model.SuggestCreatePrimaryKey}
	}

	return []model.Finding{f}
}

// operatorPriority orders operator classes by how much they need a method
// beyond btree, most demanding first. When a column carries more than one
// kind of predicate, the recommendation follows whichever one btree cannot
// serve well — recommending GIN for a column that also happens to see plain
// equality elsewhere costs nothing, while the reverse would miss the point.
var operatorPriority = []model.OperatorClass{
	model.OpVectorDistance,
	model.OpContainment,
	model.OpFullText,
	model.OpLikeInfix,
	model.OpRange,
	model.OpLikePrefix,
	model.OpEquality,
}

func dominantOperator(seen map[model.OperatorClass]bool) model.OperatorClass {
	for _, op := range operatorPriority {
		if seen[op] {
			return op
		}
	}
	return model.OpEquality
}

// columnUsage tallies, per column, how many distinct objects name it in a
// join or filter predicate, and which operators were seen.
type columnUsage struct {
	objects map[string]bool
	ops     map[model.OperatorClass]bool
}

func tallyColumnUsage(opts Options) map[model.ColumnRef]*columnUsage {
	usage := make(map[model.ColumnRef]*columnUsage)

	touch := func(ref model.ColumnRef, op model.OperatorClass, object string) {
		u, ok := usage[ref]
		if !ok {
			u = &columnUsage{objects: map[string]bool{}, ops: map[model.OperatorClass]bool{}}
			usage[ref] = u
		}
		u.objects[object] = true
		u.ops[op] = true
	}

	// A join implies both sides are looked up by equality, which is a
	// hot-column signal in its own right even though it never yields anything
	// beyond btree.
	for _, j := range opts.Joins {
		touch(j.Left, model.OpEquality, j.Object)
		touch(j.Right, model.OpEquality, j.Object)
	}
	for _, p := range opts.Predicates {
		touch(p.Column, p.Operator, p.Object)
	}

	return usage
}

// unindexedHotColumns reports a column that real code — a view, a function,
// or the query log — repeatedly names in a join or filter predicate, with no
// index leading it. A column already covered by fk_without_index is not
// duplicated here: it is the same problem, reported once.
func unindexedHotColumns(schemas []model.Schema, opts Options) []model.Finding {
	recurrenceMin := opts.RecurrenceMin
	if recurrenceMin < 1 {
		recurrenceMin = 1
	}

	usage := tallyColumnUsage(opts)

	var out []model.Finding
	for ref, u := range usage {
		if len(u.objects) < recurrenceMin {
			continue
		}

		t, ok := findTable(schemas, ref.Schema, ref.Table)
		if !ok || t.IsIndexedLeading(ref.Column) || leadsUnindexedForeignKey(t, ref.Column) {
			continue
		}

		col, ok := t.Column(ref.Column)
		if !ok {
			continue
		}

		method, opclass, note := model.IndexMethodFor(dominantOperator(u.ops), col.BaseType, opts.Extensions)
		if method == "" {
			// No honest recommendation exists — e.g. a vector distance operator
			// without pgvector installed, or a containment operator on a type GIN
			// has no default operator class for. Silence beats a suggestion the
			// server would reject.
			continue
		}

		out = append(out, model.Finding{
			Kind:    model.FindingUnindexedHotColumn,
			Object:  ref.String(),
			Detail:  "column leads no index but real code names it in a predicate",
			Metrics: map[string]int64{"distinct_objects": int64(len(u.objects))},
			Suggestion: &model.Suggestion{
				Kind:         model.SuggestCreateIndex,
				Columns:      []string{col.Name},
				IndexMethod:  method,
				IndexOpclass: opclass,
				Note:         note,
			},
		})
	}

	return out
}

// leadsUnindexedForeignKey reports whether the column already surfaces as
// fk_without_index, which would otherwise make the same problem show up
// twice under two different finding kinds.
func leadsUnindexedForeignKey(t model.Table, column string) bool {
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) > 0 && strings.EqualFold(fk.Columns[0], column) && !fk.HasIndex {
			return true
		}
	}
	return false
}

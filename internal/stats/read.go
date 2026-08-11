package stats

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// Querier is the slice of the pool this layer needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// queryEstimates reads null_frac and n_distinct for exactly the requested
// columns. pg_stats only exposes rows for tables the current role can read, so
// a missing row and a missing privilege look the same here — both fall into
// the no-opinion regime.
const queryEstimates = `
	SELECT s.schemaname, s.tablename, s.attname,
	       s.null_frac::float8, s.n_distinct::float8
	FROM pg_stats s
	JOIN unnest($1::text[], $2::text[], $3::text[]) AS want(nspname, relname, attname)
	  ON s.schemaname = want.nspname
	 AND s.tablename  = want.relname
	 AND s.attname    = want.attname`

// queryBounds reads the histogram endpoints, converted to float8 by the
// server. The caller restricts the requested columns to the numeric family,
// where the text-to-float8 cast is total; for any other type ordering
// semantics would turn the endpoints into a source of false signal.
//
// The endpoints are user data. They go into unexported fields and die with the
// evaluation.
const queryBounds = `
	SELECT s.schemaname, s.tablename, s.attname,
	       (s.histogram_bounds::text::float8[])[1],
	       (s.histogram_bounds::text::float8[])[array_length(s.histogram_bounds::text::float8[], 1)]
	FROM pg_stats s
	JOIN unnest($1::text[], $2::text[], $3::text[]) AS want(nspname, relname, attname)
	  ON s.schemaname = want.nspname
	 AND s.tablename  = want.relname
	 AND s.attname    = want.attname
	WHERE s.histogram_bounds IS NOT NULL`

// numericFamily are the base types whose histogram endpoints can be compared
// through float8 without ordering surprises.
var numericFamily = map[string]bool{
	"int2": true, "int4": true, "int8": true,
	"numeric": true, "float4": true, "float8": true,
}

// Fetch reads planner statistics for the columns the candidates involve — the
// child and the target key, nothing else. Directed reading is the point: the
// whole-schema alternative would pay for columns that never became candidates
// and keep user data in memory for the entire run.
func Fetch(ctx context.Context, q Querier, schemas []model.Schema, candidates []model.Candidate) (*Stats, error) {
	refs := involvedColumns(candidates)
	st := NewStats()
	if len(refs) == 0 {
		return st, nil
	}

	if err := fetchEstimates(ctx, q, refs, st); err != nil {
		return nil, err
	}

	numeric := numericOnly(refs, schemas)
	if len(numeric) == 0 {
		return st, nil
	}
	if err := fetchBounds(ctx, q, numeric, st); err != nil {
		return nil, err
	}
	return st, nil
}

func fetchEstimates(ctx context.Context, q Querier, refs []model.ColumnRef, st *Stats) error {
	nsp, rel, att := split(refs)

	rows, err := q.Query(ctx, queryEstimates, nsp, rel, att)
	if err != nil {
		return fmt.Errorf("reading pg_stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref model.ColumnRef
		var nullFrac, nDistinct float64
		if err := rows.Scan(&ref.Schema, &ref.Table, &ref.Column, &nullFrac, &nDistinct); err != nil {
			return fmt.Errorf("scanning pg_stats: %w", err)
		}
		st.Add(ref, model.ColumnStats{
			NullFraction: nullFrac,
			NDistinct:    nDistinct,
			Present:      true,
		})
	}
	return rows.Err()
}

func fetchBounds(ctx context.Context, q Querier, refs []model.ColumnRef, st *Stats) error {
	nsp, rel, att := split(refs)

	rows, err := q.Query(ctx, queryBounds, nsp, rel, att)
	if err != nil {
		return fmt.Errorf("reading histogram bounds: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref model.ColumnRef
		var low, high float64
		if err := rows.Scan(&ref.Schema, &ref.Table, &ref.Column, &low, &high); err != nil {
			return fmt.Errorf("scanning histogram bounds: %w", err)
		}
		st.AddBounds(ref, low, high)
	}
	return rows.Err()
}

// Prefilter fetches and evaluates in one call. On a fetch error the caller
// decides: the layer must never turn "could not read statistics" into an
// opinion about candidates.
func Prefilter(ctx context.Context, q Querier, schemas []model.Schema, candidates []model.Candidate, opts Options) (*Result, error) {
	st, err := Fetch(ctx, q, schemas, candidates)
	if err != nil {
		return nil, err
	}
	return Evaluate(schemas, candidates, st, opts), nil
}

func involvedColumns(candidates []model.Candidate) []model.ColumnRef {
	seen := make(map[model.ColumnRef]bool)
	var out []model.ColumnRef
	for _, c := range candidates {
		for _, ref := range append(c.Child.ColumnRefs(), c.Parent.ColumnRefs()...) {
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	return out
}

func numericOnly(refs []model.ColumnRef, schemas []model.Schema) []model.ColumnRef {
	types := make(map[model.ColumnRef]string)
	for _, s := range schemas {
		for _, t := range s.Tables {
			for _, c := range t.Columns {
				types[model.ColumnRef{Schema: s.Name, Table: t.Name, Column: c.Name}] = c.BaseType
			}
		}
	}

	var out []model.ColumnRef
	for _, ref := range refs {
		if numericFamily[types[ref]] {
			out = append(out, ref)
		}
	}
	return out
}

func split(refs []model.ColumnRef) (nsp, rel, att []string) {
	for _, r := range refs {
		nsp = append(nsp, r.Schema)
		rel = append(rel, r.Table)
		att = append(att, r.Column)
	}
	return nsp, rel, att
}

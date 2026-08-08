package sqlprobe

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// Querier is the slice of the pool this layer needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// queryViews reads view definitions for the schemas in scope. pg_views hands
// back the server's own reconstruction of the SQL — the cleanest source the
// extractor will ever see.
const queryViews = `
	SELECT schemaname, viewname, definition
	FROM pg_views
	WHERE schemaname = ANY($1)`

// queryFunctions reads bodies of sql and plpgsql functions in scope. Other
// languages hold no SQL the extractor could read.
const queryFunctions = `
	SELECT n.nspname, p.proname, p.prosrc
	FROM pg_proc p
	JOIN pg_namespace n ON n.oid = p.pronamespace
	JOIN pg_language l ON l.oid = p.prolang
	WHERE n.nspname = ANY($1)
	  AND l.lanname IN ('sql', 'plpgsql')`

// queryStatementsExtension asks whether pg_stat_statements is installed before
// touching it, so absence is a recorded fact instead of an error.
const queryStatementsExtension = `
	SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')`

// queryStatements reads the normalized query texts. Constants are replaced by
// placeholders in the normalized form, and only identifier pairs leave this
// layer anyway.
const queryStatements = `SELECT query FROM pg_stat_statements`

// Evidence is what probing produced.
type Evidence struct {
	Joins []model.JoinEvidence

	// StatementsAvailable reports whether pg_stat_statements answered. Absence
	// of usage evidence must never look like absence of usage.
	StatementsAvailable bool
}

// Probe mines join evidence from every source the catalog offers. A source
// that cannot be read degrades to a warning-free skip of that source: the
// extractor's contract — lost evidence, never wrong evidence — extends to
// fetching.
func Probe(ctx context.Context, q Querier, schemas []model.Schema) (*Evidence, error) {
	names := make([]string, 0, len(schemas))
	for _, s := range schemas {
		names = append(names, s.Name)
	}

	ev := &Evidence{}
	var raw []sourcedJoin

	views, err := fetchSources(ctx, q, queryViews, names)
	if err != nil {
		return nil, fmt.Errorf("reading view definitions: %w", err)
	}
	raw = append(raw, extractAll(views, model.JoinFromView)...)

	functions, err := fetchSources(ctx, q, queryFunctions, names)
	if err != nil {
		return nil, fmt.Errorf("reading function bodies: %w", err)
	}
	raw = append(raw, extractAll(functions, model.JoinFromFunction)...)

	statements, available := fetchStatements(ctx, q)
	ev.StatementsAvailable = available
	raw = append(raw, extractAll(statements, model.JoinFromStatements)...)

	ev.Joins = resolveAgainstCatalog(raw, schemas)
	return ev, nil
}

// source is one piece of SQL and the object it came from.
type source struct {
	object string
	sql    string
}

type sourcedJoin struct {
	join   rawJoin
	source model.JoinSource
	object string
}

func fetchSources(ctx context.Context, q Querier, query string, schemas []string) ([]source, error) {
	rows, err := q.Query(ctx, query, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []source
	for rows.Next() {
		var schema, name, sql string
		if err := rows.Scan(&schema, &name, &sql); err != nil {
			return nil, err
		}
		out = append(out, source{object: schema + "." + name, sql: sql})
	}
	return out, rows.Err()
}

// fetchStatements is opportunistic: the extension may be absent, or the role
// may lack the privilege. Both degrade to "not available", recorded.
func fetchStatements(ctx context.Context, q Querier) ([]source, bool) {
	var installed bool
	if err := q.QueryRow(ctx, queryStatementsExtension).Scan(&installed); err != nil || !installed {
		return nil, false
	}

	rows, err := q.Query(ctx, queryStatements)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var out []source
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return nil, false
		}
		out = append(out, source{object: "pg_stat_statements", sql: sql})
	}
	if rows.Err() != nil {
		return nil, false
	}
	return out, true
}

func extractAll(sources []source, kind model.JoinSource) []sourcedJoin {
	var out []sourcedJoin
	for _, s := range sources {
		for _, j := range extract(s.sql) {
			out = append(out, sourcedJoin{join: j, source: kind, object: s.object})
		}
	}
	return out
}

// resolveAgainstCatalog keeps only pairs whose both sides name a real column
// of a real table in scope, and deduplicates. Bare identifiers arrive
// lowercased from the tokenizer, so matching is case-insensitive — the same
// folding the server applies.
func resolveAgainstCatalog(raw []sourcedJoin, schemas []model.Schema) []model.JoinEvidence {
	type dedupKey struct {
		a, b   string
		source model.JoinSource
	}

	seen := make(map[dedupKey]bool)
	var out []model.JoinEvidence

	for _, sj := range raw {
		left, okL := resolveRef(sj.join.left, schemas)
		right, okR := resolveRef(sj.join.right, schemas)
		if !okL || !okR || left == right {
			continue
		}

		// The pair is undirected; canonical ordering makes a=b and b=a the
		// same evidence.
		a, b := left.String(), right.String()
		if a > b {
			left, right = right, left
			a, b = b, a
		}

		key := dedupKey{a: a, b: b, source: sj.source}
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, model.JoinEvidence{
			Left:   left,
			Right:  right,
			Source: sj.source,
			Object: sj.object,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Left.String() != out[j].Left.String() {
			return out[i].Left.String() < out[j].Left.String()
		}
		if out[i].Right.String() != out[j].Right.String() {
			return out[i].Right.String() < out[j].Right.String()
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func resolveRef(ref rawRef, schemas []model.Schema) (model.ColumnRef, bool) {
	var found model.ColumnRef
	matches := 0

	for _, s := range schemas {
		if ref.schema != "" && !strings.EqualFold(ref.schema, s.Name) {
			continue
		}
		for _, t := range s.Tables {
			if !strings.EqualFold(ref.table, t.Name) {
				continue
			}
			col, ok := t.Column(ref.column)
			if !ok {
				continue
			}
			found = model.ColumnRef{Schema: s.Name, Table: t.Name, Column: col.Name}
			matches++
		}
	}

	// Ambiguity across schemas is dropped rather than guessed: a wrong guess
	// here would manufacture evidence, which is the one thing this layer must
	// never do.
	return found, matches == 1
}

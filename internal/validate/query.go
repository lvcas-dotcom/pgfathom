package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

type sampleMethod int

const (
	sampleNone sampleMethod = iota
	sampleBernoulli
	sampleSystem
)

// sampleSpec is how the child table will be read.
type sampleSpec struct {
	kind    sampleMethod
	percent float64
	seed    int64
}

func (s sampleSpec) method() model.ValidationMethod {
	if s.kind == sampleNone {
		return model.MethodFull
	}
	return model.MethodSampled
}

// sampleFor picks the reading mode. A table that fits the target is read
// whole, which returns the conclusive mode for free. Unknown estimates also
// read whole: guessing a fraction from a number the planner does not have
// would be inventing, and the time ceiling already bounds the cost.
func sampleFor(rows int64, known bool, opts Options) sampleSpec {
	target := opts.targetRows()
	if opts.Full || !known || rows <= target {
		return sampleSpec{kind: sampleNone}
	}

	pct := 100 * float64(target) / float64(rows)
	if pct < 0.01 {
		pct = 0.01
	}

	kind := sampleSystem
	if rows < bernoulliCeiling {
		kind = sampleBernoulli
	}
	return sampleSpec{kind: kind, percent: pct, seed: opts.Seed}
}

// buildQuery assembles the aggregation for one candidate, of any arity.
// Identifiers are quoted without exception; the sampling fraction is computed
// here and rendered by strconv, never taken from user input.
//
// The key travels as one projected column per position, never as a
// concatenation: concatenating would collide distinct tuples and would push a
// user value through a text expression.
//
// The scanned CTE feeds three readings. The totals give what NULL dominance is
// measured against, and how many rows MATCH SIMPLE exempts. The per-tuple
// aggregation turns the anti-join from a row count into a cardinality.
func buildQuery(c model.Candidate, spec sampleSpec) string {
	parent := pgx.Identifier{c.Parent.Schema, c.Parent.Table}.Sanitize()

	// The alias must precede TABLESAMPLE in the from_item grammar.
	from := pgx.Identifier{c.Child.Schema, c.Child.Table}.Sanitize() +
		" AS c" + sampleClause(spec)

	n := c.Child.Arity()
	projected := make([]string, 0, n) // c."a" AS v1
	positions := make([]string, 0, n) // v1
	matched := make([]string, 0, n)   // p."k1" = cv.v1

	for i, col := range c.Child.Columns {
		v := "v" + strconv.Itoa(i+1)
		projected = append(projected, "c."+pgx.Identifier{col}.Sanitize()+" AS "+v)
		positions = append(positions, v)
		matched = append(matched,
			"p."+pgx.Identifier{c.Parent.Columns[i]}.Sanitize()+" = cv."+v)
	}

	// num_nulls over the whole key says both things at once: zero is a row the
	// constraint would check, and anything between zero and the arity is a row
	// MATCH SIMPLE lets through. At arity one the second is false by
	// construction, which is the correct answer rather than a special case.
	all := strings.Join(positions, ", ")

	return fmt.Sprintf(`
		WITH scanned AS (
		    SELECT %[1]s FROM %[2]s
		),
		totals AS (
		    SELECT count(*) AS rows_scanned,
		           count(*) FILTER (WHERE num_nulls(%[3]s) BETWEEN 1 AND %[4]d) AS partial_null
		    FROM scanned
		),
		child_vals AS (
		    SELECT %[3]s, count(*) AS n FROM scanned WHERE num_nulls(%[3]s) = 0 GROUP BY %[5]s
		),
		marked AS (
		    SELECT cv.n, EXISTS (SELECT 1 FROM %[6]s p WHERE %[7]s) AS matched
		    FROM child_vals cv
		)
		SELECT
		    (SELECT rows_scanned FROM totals)                         AS sampled_rows,
		    (SELECT partial_null FROM totals)                         AS partial_null_rows,
		    count(*)                                                  AS distinct_vals,
		    coalesce(sum(n), 0)::bigint                               AS not_null_rows,
		    count(*) FILTER (WHERE NOT matched)                       AS orphan_vals,
		    coalesce(sum(n) FILTER (WHERE NOT matched), 0)::bigint    AS orphan_rows,
		    coalesce(max(n), 0)::bigint                               AS max_rows_per_value
		FROM marked`,
		strings.Join(projected, ", "), from, all, n-1, groupBy(n), parent,
		strings.Join(matched, " AND "))
}

// groupBy renders the ordinal group list, "1, 2, ..., n".
func groupBy(n int) string {
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return strings.Join(out, ", ")
}

func sampleClause(spec sampleSpec) string {
	var method string
	switch spec.kind {
	case sampleSystem:
		method = "SYSTEM"
	case sampleBernoulli:
		method = "BERNOULLI"
	default:
		return ""
	}

	clause := " TABLESAMPLE " + method +
		" (" + strconv.FormatFloat(spec.percent, 'f', 4, 64) + ")"
	if spec.seed != 0 {
		clause += " REPEATABLE (" + strconv.FormatInt(spec.seed, 10) + ")"
	}
	return clause
}

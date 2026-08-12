package validate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// KeyProbeResult is the outcome of testing one candidate column set for
// uniqueness by counting rows against the data.
type KeyProbeResult struct {
	Columns []string

	// Verdict is KeyProbeConfirmed when a full scan proved total rows equal
	// distinct values and zero rows are NULL in any key column. Otherwise it
	// is KeyProbeUnverified — whether because the columns are genuinely not
	// unique, the probe timed out, or the role lacked privilege. There is no
	// third state: a proven duplicate and an inconclusive probe both mean "no
	// key proven here", which is all a caller needs to decide what to do next.
	Verdict model.KeyProbeVerdict

	// Reason explains an Unverified verdict. Object names and conditions only.
	Reason string
}

// ProbeUniqueness tests each candidate column set of table for uniqueness by
// counting rows: the total, the count of distinct values across the set, and
// how many rows are NULL in any column of it. Only these three integers ever
// leave the query — no value from a row is read.
//
// A candidate is confirmed only by a full table scan. TABLESAMPLE never
// appears here: a single duplicate outside the sample would make the
// confirmation false, and unlike relationship validation, this package has no
// weaker claim to make about a primary key than "proven". Candidates are
// probed one at a time, deliberately: naming a key is rare enough, and the
// scan expensive enough, that running several full scans of the same table
// concurrently is not a cost worth adding for it.
//
// An error is returned only for a failure the caller cannot act on by moving
// to the next candidate — the same split validateOne makes between a
// resolved per-candidate outcome and an infrastructure failure that makes
// every remaining result untrustworthy too.
func ProbeUniqueness(ctx context.Context, pool Beginner, table model.Table, candidates [][]string, timeout time.Duration) ([]KeyProbeResult, error) {
	out := make([]KeyProbeResult, len(candidates))

	for i, cols := range candidates {
		res, err := probeOne(ctx, pool, table, cols, timeout)
		if err != nil {
			return nil, fmt.Errorf("probing uniqueness of %s(%s): %w", table.Ref(), strings.Join(cols, ", "), err)
		}
		out[i] = res
	}

	return out, nil
}

func probeOne(ctx context.Context, pool Beginner, table model.Table, cols []string, timeout time.Duration) (KeyProbeResult, error) {
	res := KeyProbeResult{Columns: cols}

	total, distinct, nulls, err := runKeyProbeQuery(ctx, pool, buildKeyProbeQuery(table, cols), timeout)
	if err != nil {
		reason, _, resolved := resolveError(ctx, err)
		if !resolved {
			return KeyProbeResult{}, err
		}
		res.Verdict = model.KeyProbeUnverified
		res.Reason = reason
		return res, nil
	}

	if total == distinct && nulls == 0 {
		res.Verdict = model.KeyProbeConfirmed
		return res, nil
	}

	res.Verdict = model.KeyProbeUnverified
	res.Reason = "columns are not unique"
	return res, nil
}

// buildKeyProbeQuery assembles the count-only uniqueness check. Identifiers
// are quoted without exception, the same discipline buildQuery follows for
// relationship validation.
//
// (col1, col2, ...) is a row constructor for two or more columns and a plain
// parenthesized expression for one, so the same shape serves a single-column
// or a composite candidate without a special case.
func buildKeyProbeQuery(table model.Table, columns []string) string {
	rel := pgx.Identifier{table.Schema, table.Name}.Sanitize()

	quoted := make([]string, len(columns))
	nullChecks := make([]string, len(columns))
	for i, c := range columns {
		q := pgx.Identifier{c}.Sanitize()
		quoted[i] = q
		nullChecks[i] = q + " IS NULL"
	}
	tuple := "(" + strings.Join(quoted, ", ") + ")"

	return fmt.Sprintf(`
		SELECT
		    count(*)                                     AS total_rows,
		    count(DISTINCT %[1]s)                         AS distinct_vals,
		    count(*) FILTER (WHERE %[2]s)                 AS null_rows
		FROM %[3]s`,
		tuple, strings.Join(nullChecks, " OR "), rel)
}

// runKeyProbeQuery executes one probe inside its own read-only transaction,
// so the SET LOCAL ceiling dies with it — the same isolation runQuery uses.
func runKeyProbeQuery(ctx context.Context, pool Beginner, query string, timeout time.Duration) (total, distinct, nulls int64, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if timeout > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeout.Milliseconds())); err != nil {
			return 0, 0, 0, err
		}
	}

	err = tx.QueryRow(ctx, query).Scan(&total, &distinct, &nulls)
	return total, distinct, nulls, err
}

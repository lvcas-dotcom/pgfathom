// Package validate checks candidates against the actual data. It is the only
// layer in the program allowed to read rows from user tables, and the only one
// whose cost the database owner feels.
//
// The reading produces counts and dies in memory: no value from a user table
// crosses the query boundary. Every query runs under its own statement
// timeout, concurrency is capped by the session limit, and a candidate that
// cannot be resolved comes back as unvalidated with the reason — never as
// silence.
package validate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/sync/errgroup"

	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// Defaults, declared as estimates and revisited with the benchmark corpus.
const (
	// DefaultTargetRows is the sampled-mode target per table.
	DefaultTargetRows = 100_000

	// DefaultBrokenThreshold is the row containment at or above which a
	// relationship with orphans reads as broken rather than coincidence.
	DefaultBrokenThreshold = 0.90

	// DefaultRejectThreshold is the row containment below which the hypothesis
	// is dismissed as a name coincidence.
	DefaultRejectThreshold = 0.50

	// DefaultMaxNullFraction is the NULL share above which the column cannot
	// evidence anything: containment over the few remaining rows is noise.
	DefaultMaxNullFraction = 0.95
)

// bernoulliCeiling is the estimated row count below which page-level SYSTEM
// sampling distorts too much and BERNOULLI takes over.
const bernoulliCeiling = 100_000

// Options tunes a validation run.
type Options struct {
	// Full disables sampling. It is the only mode that can confirm.
	Full bool

	// TargetRows calibrates the sample per table. Zero means the default.
	TargetRows int64

	// Timeout bounds each validation query via SET LOCAL. Zero inherits the
	// session's statement_timeout unchanged.
	Timeout time.Duration

	// Concurrency caps simultaneous validations. It must match the session's
	// connection cap: two independent limits create the combination where the
	// pool blocks the group and cancellation waits for a connection.
	Concurrency int

	// Seed makes sampling reproducible via REPEATABLE. Zero samples freshly.
	Seed int64

	BrokenThreshold float64
	RejectThreshold float64
	MaxNullFraction float64
}

func (o Options) targetRows() int64 {
	if o.TargetRows <= 0 {
		return DefaultTargetRows
	}
	return o.TargetRows
}

func (o Options) concurrency() int {
	if o.Concurrency < 1 {
		return db.DefaultConcurrency
	}
	return o.Concurrency
}

func (o Options) brokenThreshold() float64 {
	if o.BrokenThreshold <= 0 {
		return DefaultBrokenThreshold
	}
	return o.BrokenThreshold
}

func (o Options) rejectThreshold() float64 {
	if o.RejectThreshold <= 0 {
		return DefaultRejectThreshold
	}
	return o.RejectThreshold
}

func (o Options) maxNullFraction() float64 {
	if o.MaxNullFraction <= 0 {
		return DefaultMaxNullFraction
	}
	return o.MaxNullFraction
}

// Result is what a validation run produced.
type Result struct {
	// Candidates carry their verdicts and metrics, in the input order.
	Candidates []model.Candidate

	// Validated is how many queries completed. Validated plus TimedOut closes
	// the account with everything that entered.
	Validated int

	// TimedOut is how many candidates hit the per-query ceiling. They come
	// back unvalidated, never dropped.
	TimedOut int
}

// Run validates every candidate concurrently under the session limit. A
// timeout or a missing privilege resolves that one candidate as unvalidated
// and the run continues; any other database error aborts, because it means the
// remaining results could not be trusted either.
func Run(ctx context.Context, pool *db.Pool, schemas []model.Schema, candidates []model.Candidate, opts Options) (*Result, error) {
	res := &Result{Candidates: make([]model.Candidate, len(candidates))}
	rows := model.RowEstimates(schemas)
	timedOut := make([]bool, len(candidates))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.concurrency())

	for i, c := range candidates {
		g.Go(func() error {
			out, timeout, err := validateOne(ctx, pool, c, rows, opts)
			if err != nil {
				return err
			}
			res.Candidates[i], timedOut[i] = out, timeout
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	for i, c := range res.Candidates {
		if c.Validation != nil {
			res.Validated++
		}
		if timedOut[i] {
			res.TimedOut++
		}
	}
	return res, nil
}

func validateOne(ctx context.Context, pool *db.Pool, c model.Candidate, rows map[string]int64, opts Options) (out model.Candidate, timedOut bool, err error) {
	childRows, childKnown := rows[c.Child.TableRef()]
	spec := sampleFor(childRows, childKnown, opts)

	v, err := runQuery(ctx, pool, buildQuery(c, spec), opts.Timeout)
	if err != nil {
		reason, timeout, resolved := resolveError(ctx, err)
		if !resolved {
			return c, false, fmt.Errorf("validating %s: %w", c.Child, err)
		}
		c.Verdict = model.VerdictUnvalidated
		c.Reason = reason
		return c, timeout, nil
	}

	v.Method = spec.method()
	c.Validation = &v
	c.Verdict, c.Reason = verdict(v, opts)
	return c, false, nil
}

// runQuery executes one validation inside its own read-only transaction, so
// the SET LOCAL ceiling dies with it.
func runQuery(ctx context.Context, pool *db.Pool, query string, timeout time.Duration) (model.Validation, error) {
	var v model.Validation

	tx, err := pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if timeout > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeout.Milliseconds())); err != nil {
			return v, err
		}
	}

	start := time.Now()
	err = tx.QueryRow(ctx, query).Scan(
		&v.SampledRows, &v.DistinctVals, &v.NotNullRows,
		&v.OrphanVals, &v.OrphanRows, &v.MaxRowsPerValue,
	)
	v.Duration = time.Since(start)
	return v, err
}

// resolveError classifies a per-candidate failure. Timeout and missing
// privilege resolve the candidate; anything else is the caller's problem.
func resolveError(ctx context.Context, err error) (reason string, timedOut, resolved bool) {
	if ctx.Err() != nil {
		return "", false, false
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", false, false
	}
	switch pgErr.Code {
	case "57014": // query_canceled: the SET LOCAL ceiling fired
		return "validation exceeded the time ceiling", true, true
	case "42501": // insufficient_privilege
		return "no privilege to read the table", false, true
	}
	return "", false, false
}

// verdict applies the fixed rules, in order. Weak-by-insufficiency comes
// first: containment measured over one distinct value, a NULL-dominated column
// or an empty table supports no conclusion whatever the number says.
func verdict(v model.Validation, opts Options) (model.Verdict, string) {
	switch {
	case v.NotNullRows == 0:
		return model.VerdictWeak, "empty child table or all-NULL column: nothing to evidence a relationship"
	case v.DistinctVals <= 1:
		return model.VerdictWeak, "a single distinct value cannot evidence a relationship"
	case v.SampledRows > 0 && nullFraction(v) > opts.maxNullFraction():
		return model.VerdictWeak, "NULLs dominate the column: containment over the remainder is noise"
	}

	if v.OrphanRows == 0 && v.OrphanVals == 0 {
		if v.Conclusive() {
			return model.VerdictConfirmed, ""
		}
		// Orphans arrive in batches and cluster on the same pages — exactly
		// what page sampling is worst at finding. A clean sample is not
		// evidence of absence.
		return model.VerdictWeak, "probable: no orphans in the sample, but only --full can prove absence"
	}

	cr := v.ContainmentRows()
	switch {
	case cr >= opts.brokenThreshold():
		return model.VerdictBroken, ""
	case cr < opts.rejectThreshold():
		return model.VerdictRejected, "low containment: the name match is a coincidence"
	default:
		return model.VerdictWeak, fmt.Sprintf(
			"containment %.0f%% sits between rejection and breakage: a human call", 100*cr)
	}
}

func nullFraction(v model.Validation) float64 {
	if v.SampledRows <= 0 {
		return 0
	}
	return 1 - float64(v.NotNullRows)/float64(v.SampledRows)
}


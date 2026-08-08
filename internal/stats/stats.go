// Package stats is the statistical prefilter: it uses planner estimates to
// kill arithmetically impossible candidates before any table I/O.
//
// Everything this layer consumes is an estimate. n_distinct and the histogram
// come from the last ANALYZE, which may never have run, may be years old, or
// may have sampled badly. That is why the layer penalizes by default, rejects
// only beyond a wide margin, and stays silent — with a record of its silence —
// when the statistics are not there. It never invents a rejection from data it
// does not have.
//
// The values read from pg_stats are user data, read from the catalog instead
// of from the table, which changes nothing about what they are. They produce
// signals and counts in memory and die there.
package stats

import (
	"fmt"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// Penalty weights. They are judgment about estimates, which is why they live
// here, next to the checks they qualify, rather than with the fact-based
// weights in infer: mixed together, nobody could tell fact from guess.
const (
	penaltyCardViolation  = -0.40
	penaltyRangeViolation = -0.30
)

// DefaultRejectFactor is the wide tolerance margin for outright rejection: the
// child's estimated distinct count must exceed this multiple of the parent's
// estimated row count. A column with twice as many distinct values as the
// target has rows is not a stale statistic — it is a different relationship.
//
// The value is an estimate declared as such, revisited with the benchmark
// corpus alongside the score threshold.
const DefaultRejectFactor = 2.0

// Options tunes the evaluation.
type Options struct {
	// RejectFactor overrides DefaultRejectFactor. Zero means the default.
	RejectFactor float64
}

func (o Options) rejectFactor() float64 {
	if o.RejectFactor <= 0 {
		return DefaultRejectFactor
	}
	return o.RejectFactor
}

// columnStats pairs the planner estimates with histogram endpoints. The
// endpoints are user data: they stay in unexported fields with no
// serialization path, and there is no accessor for them.
type columnStats struct {
	estimates model.ColumnStats
	hasBounds bool
	low       float64
	high      float64
}

// Stats holds planner statistics for the columns under evaluation.
type Stats struct {
	columns map[model.ColumnRef]columnStats
}

// NewStats returns an empty statistics set. The evaluation treats every column
// it does not contain as never analyzed.
func NewStats() *Stats {
	return &Stats{columns: make(map[model.ColumnRef]columnStats)}
}

// Add records the estimates for one column.
func (s *Stats) Add(ref model.ColumnRef, estimates model.ColumnStats) {
	c := s.columns[ref]
	c.estimates = estimates
	s.columns[ref] = c
}

// AddBounds records the histogram endpoints for one column. They feed the
// range check and never leave this package.
func (s *Stats) AddBounds(ref model.ColumnRef, low, high float64) {
	c := s.columns[ref]
	c.hasBounds, c.low, c.high = true, low, high
	s.columns[ref] = c
}

// Result is what the prefilter produced. Checked = len(Kept) + len(Rejected) +
// NoStats: the funnel that predicts how many anti-joins validation will cost.
type Result struct {
	// Kept survived, possibly rescored under a penalty signal.
	Kept []model.Candidate

	// Rejected violated cardinality beyond the tolerance margin. Each carries
	// the reason, with the estimates declared as estimates.
	Rejected []model.Candidate

	// Checked is how many candidates entered the evaluation.
	Checked int

	// NoStats is how many candidates could not be evaluated at all. They pass
	// through untouched except for the zero-weight marker signal.
	NoStats int
}

// Evaluate runs the two checks over the candidates. It is pure: statistics
// come in as an argument, nothing touches a database, and the same inputs
// always produce the same result.
func Evaluate(schemas []model.Schema, candidates []model.Candidate, st *Stats, opts Options) *Result {
	rows := model.RowEstimates(schemas)
	res := &Result{Checked: len(candidates)}

	for _, c := range candidates {
		res.add(evaluateOne(c, rows, st, opts.rejectFactor()))
	}
	return res
}

func (r *Result) add(c model.Candidate, outcome outcome) {
	switch outcome {
	case outcomeRejected:
		r.Rejected = append(r.Rejected, c)
	case outcomeNoStats:
		r.NoStats++
		r.Kept = append(r.Kept, c)
	default:
		r.Kept = append(r.Kept, c)
	}
}

type outcome int

const (
	outcomeKept outcome = iota
	outcomeRejected
	outcomeNoStats
)

func evaluateOne(c model.Candidate, rows map[string]int64, st *Stats, rejectFactor float64) (model.Candidate, outcome) {
	child, childOK := st.columns[c.Child]
	parentRows, parentOK := rows[c.Parent.TableRef()]
	childRows, childRowsOK := rows[c.Child.TableRef()]

	distinct, distinctOK := int64(0), false
	if childOK && childRowsOK {
		distinct, distinctOK = child.estimates.EstimatedDistinct(childRows)
	}

	// The cardinality check needs both ends of the arithmetic. Without them the
	// layer has no opinion, and "no opinion" must be visible on the candidate:
	// an absent penalty with no record would read as approval.
	if !distinctOK || !parentOK {
		c.Signals = append(c.Signals, model.Signal{
			Kind:   model.SigStatsUnavailable,
			Weight: 0,
			Detail: noStatsDetail(c, distinctOK, parentOK),
		})
		return c, outcomeNoStats
	}

	if distinct > parentRows {
		if float64(distinct) > rejectFactor*float64(parentRows) {
			c.Signals = append(c.Signals, model.Signal{
				Kind:   model.SigCardViolation,
				Weight: penaltyCardViolation,
				Detail: c.Parent.TableRef(),
			})
			infer.Rescore(&c)
			c.Verdict = model.VerdictRejected
			c.Reason = fmt.Sprintf(
				"estimated distinct values in %s (~%d) exceed estimated rows in %s (~%d) beyond tolerance; total containment is arithmetically impossible",
				c.Child, distinct, c.Parent.TableRef(), parentRows)
			return c, outcomeRejected
		}

		c.Signals = append(c.Signals, model.Signal{
			Kind:   model.SigCardViolation,
			Weight: penaltyCardViolation,
			Detail: c.Parent.TableRef(),
		})
		infer.Rescore(&c)
	}

	// Range: histogram endpoints are the least precise piece of the statistics
	// — a table that grew after ANALYZE has values outside the old bounds by
	// construction — so a disjoint range penalizes and never rejects.
	parent := st.columns[c.Parent]
	if child.hasBounds && parent.hasBounds &&
		(child.low > parent.high || child.high < parent.low) {
		c.Signals = append(c.Signals, model.Signal{
			Kind:   model.SigRangeViolation,
			Weight: penaltyRangeViolation,
			Detail: c.Parent.TableRef(),
		})
		infer.Rescore(&c)
	}

	return c, outcomeKept
}

func noStatsDetail(c model.Candidate, distinctOK, parentOK bool) string {
	switch {
	case !distinctOK && !parentOK:
		return "no usable statistics for " + c.Child.TableRef() + " or " + c.Parent.TableRef()
	case !distinctOK:
		return "no usable statistics for " + c.Child.String()
	default:
		return "no row estimate for " + c.Parent.TableRef()
	}
}

package infer

import "github.com/lvcas-dotcom/pgfathom/internal/model"

// Signal weights live here, together, because they are judgment rather than
// fact. The first set of numbers will be wrong; what has to stay true is the
// relative ordering — an exact name match outranks a normalized one — which is
// what the tests pin. Absolute values are expected to move once the benchmark
// corpus says something.
const (
	weightExactName      = 0.30
	weightNormalizedName = 0.15
	weightIdenticalType  = 0.25
	weightCompatibleType = 0.10
	weightUniqueTarget   = 0.20
	weightChildIndexed   = 0.15
	weightCommentMention = 0.12
	weightNotNull        = 0.05

	// A join in real code outranks any name signal: it is usage, not
	// convention, and it is the only evidence that reaches relationships whose
	// names bear no resemblance. Views outrank function bodies, which the
	// extractor reads with less context; the query log ranks last because it
	// mixes in ad-hoc sessions.
	weightJoinInView       = 0.50
	weightJoinInFunction   = 0.45
	weightJoinInStatements = 0.40

	penaltyAmbiguousTarget = -0.25
	penaltyGenericDomain   = -0.30
)

// Arity weights. A key of four columns is not twice as convincing as one of
// two — the second column already removes most of the coincidence — so the
// increment shrinks and the whole thing is capped.
//
// The cap matters more than the base. Sitting on top of a candidate that
// already carries name, type and uniqueness, a generous arity weight would
// saturate every composite at the ceiling, and a score pinned at 1.0
// discriminates nothing exactly where the risk of a wrong confirmation is
// highest. Estimates, like the rest of this file, revisited with the corpus.
const (
	weightArityBase = 0.15 // two columns
	weightArityStep = 0.05 // per column beyond the second
	weightArityMax  = 0.30
)

// arityWeight is the weight of a whole key agreeing at once. A single-column
// key agrees about nothing extra, and weighs nothing extra.
func arityWeight(n int) float64 {
	if n < 2 {
		return 0
	}
	w := weightArityBase + float64(n-2)*weightArityStep
	if w > weightArityMax {
		return weightArityMax
	}
	return w
}

// DefaultMinScore is the cut below which a candidate never reaches validation.
//
// It is an estimate, not a measurement: the honest calibration needs the
// candidate-per-table ratio from real schemas, which is why that ratio is
// reported in coverage from this phase onward and the default is revisited with
// the benchmark corpus.
const DefaultMinScore = 0.5

// DefaultSmallTableRows is the size below which a target counts as a domain
// table for the generic-name penalty.
const DefaultSmallTableRows = 1000

// score combines the signal weights, saturating at both ends.
//
// Free summation would make the range depend on how many signals happened to
// fire, so a threshold would mean different things in a schema with rich
// comments and one with none. Saturating keeps the number something a user can
// reason about: 0.5 is half the available confidence, anywhere.
func score(signals []model.Signal) float64 {
	total := 0.0
	for _, s := range signals {
		total += s.Weight
	}

	switch {
	case total < 0:
		return 0
	case total > 1:
		return 1
	default:
		return total
	}
}

// Rescore recomputes MetaScore from the candidate's signals, saturating exactly
// as generation does. A layer that appends signals after generation must
// recompose through here: a second implementation of the combination rule would
// drift, and the threshold would change meaning depending on which layer
// touched the candidate last.
func Rescore(c *model.Candidate) {
	c.MetaScore = score(c.Signals)
}

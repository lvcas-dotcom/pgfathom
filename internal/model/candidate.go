package model

import "time"

// SignalKind identifies where a piece of evidence came from. The set is closed,
// so every score can be explained back to the facts that produced it.
type SignalKind string

// Name-based evidence.
const (
	SigExactName      SignalKind = "exact_name"
	SigNormalizedName SignalKind = "normalized_name"
)

// Type evidence.
const (
	SigIdenticalType  SignalKind = "identical_type"
	SigCompatibleType SignalKind = "compatible_type"
)

// Target evidence. Ambiguity is the main source of noise in name matching.
const (
	SigUniqueTarget    SignalKind = "unique_target"
	SigAmbiguousTarget SignalKind = "ambiguous_target"
)

// SigCompositeArity is a whole key agreeing at once. Two columns lining up is
// far less likely to be coincidence than one, so arity is evidence — emitted
// once, never once per column: a four-column key would otherwise carry twenty
// signals stating four facts, and saturation would let arity pin any score to
// the ceiling.
const SigCompositeArity SignalKind = "composite_arity"

// Catalog evidence.
const (
	SigChildIndexed   SignalKind = "child_indexed"
	SigCommentMention SignalKind = "comment_mention"
	SigNotNull        SignalKind = "not_null"
)

// Usage evidence: a join the real code executes, extracted from a view, a
// function body or the query log. The strongest signal, and the only one that
// reaches relationships whose names bear no resemblance to the target table.
const (
	SigJoinInView       SignalKind = "join_in_view"
	SigJoinInFunction   SignalKind = "join_in_function"
	SigJoinInStatements SignalKind = "join_in_statements"
)

// Penalties.
const (
	SigGenericDomain  SignalKind = "generic_domain_name"
	SigRangeViolation SignalKind = "stats_range_violation"
	SigCardViolation  SignalKind = "stats_cardinality_violation"
)

// SigStatsUnavailable is a zero-weight marker: the prefilter looked for planner
// statistics and found none. It never moves the score; it exists because an
// absent penalty with no record would be indistinguishable from approval.
const SigStatsUnavailable SignalKind = "stats_unavailable"

// Signal is one piece of evidence for or against a candidate.
type Signal struct {
	Kind   SignalKind `json:"kind"`
	Weight float64    `json:"weight"`

	// Detail carries catalog object names only.
	Detail string `json:"detail,omitempty"`
}

// Verdict is the conclusion about a candidate. The set is closed.
type Verdict string

const (
	// VerdictConfirmed is a forgotten foreign key: total containment, no orphans.
	VerdictConfirmed Verdict = "confirmed"

	// VerdictBroken is a real relationship with broken integrity — the finding
	// this tool exists to produce.
	VerdictBroken Verdict = "broken"

	// VerdictWeak means the data cannot support a conclusion: one distinct
	// value, an overwhelming null fraction, an empty child table.
	VerdictWeak Verdict = "weak"

	// VerdictRejected means the evidence knocked the hypothesis down.
	VerdictRejected Verdict = "rejected"

	// VerdictUnvalidated means no evidence was gathered: a timeout, a missing
	// privilege, an unsupported shape. Not the same as rejected, and conflating
	// the two reports silence as a clean bill of health.
	VerdictUnvalidated Verdict = "unvalidated"
)

// Candidate is a possible relationship between two keys, raised by naming
// heuristics, by usage evidence, or by both. A single-column relationship is
// the arity-one case, not a separate shape.
type Candidate struct {
	Child  KeyRef `json:"child"`
	Parent KeyRef `json:"parent"`

	// Signals are the evidence behind MetaScore, kept so a score can be
	// explained rather than merely asserted.
	Signals []Signal `json:"signals"`

	// MetaScore is confidence from metadata alone, 0..1, before reading data.
	MetaScore float64 `json:"meta_score"`

	// Validation is nil until the candidate has been checked against the data.
	Validation *Validation `json:"validation,omitempty"`

	Verdict Verdict `json:"verdict"`

	// Reason explains a non-obvious verdict. Object names and conditions only.
	Reason string `json:"reason,omitempty"`
}

// HasSignal reports whether the candidate carries a signal of the given kind.
func (c Candidate) HasSignal(k SignalKind) bool {
	for _, s := range c.Signals {
		if s.Kind == k {
			return true
		}
	}
	return false
}

// EvidencedByUsage reports whether a real join was found for this pair, which
// is proof the application relates the columns whatever they are named.
func (c Candidate) EvidencedByUsage() bool {
	return c.HasSignal(SigJoinInView) ||
		c.HasSignal(SigJoinInFunction) ||
		c.HasSignal(SigJoinInStatements)
}

// ValidationMethod distinguishes a sampled run from a complete one.
type ValidationMethod string

const (
	// MethodFull examined every row, and is the only conclusive mode.
	MethodFull ValidationMethod = "full"

	// MethodSampled examined a subset. Orphans arrive in batches and cluster on
	// the same pages, which is what page-level sampling is worst at finding, so
	// a sampled run is triage rather than a conclusion.
	MethodSampled ValidationMethod = "sampled"
)

// Validation is the outcome of checking a candidate against the data.
type Validation struct {
	Method      ValidationMethod `json:"method"`
	SampledRows int64            `json:"sampled_rows,omitempty"`

	NotNullRows  int64 `json:"not_null_rows"`
	DistinctVals int64 `json:"distinct_vals"`

	// PartialNullRows counts rows with a NULL in part of the key and a value in
	// the rest. Under MATCH SIMPLE — the SQL default, and what the generated DDL
	// uses — those rows are exempt from the constraint, silently. Zero by
	// construction on a single-column key; a finding on a composite one.
	PartialNullRows int64 `json:"partial_null_rows,omitempty"`

	// OrphanRows and OrphanVals count the same failure in two units. One bad
	// value repeated a million times and a million rare bad values are
	// different problems, and neither number implies the other.
	OrphanRows int64 `json:"orphan_rows"`
	OrphanVals int64 `json:"orphan_vals"`

	// MaxRowsPerValue separates one-to-one from one-to-many.
	MaxRowsPerValue int64 `json:"max_rows_per_value"`

	Duration time.Duration `json:"duration_ns"`
}

// ContainmentRows is the proportion of examined rows present in the parent key.
func (v Validation) ContainmentRows() float64 {
	if v.NotNullRows <= 0 {
		return 0
	}
	return float64(v.NotNullRows-v.OrphanRows) / float64(v.NotNullRows)
}

// ContainmentVals is the proportion of distinct examined values present in the
// parent key.
func (v Validation) ContainmentVals() float64 {
	if v.DistinctVals <= 0 {
		return 0
	}
	return float64(v.DistinctVals-v.OrphanVals) / float64(v.DistinctVals)
}

// Conclusive reports whether this validation can support a confirmation. A
// sampled run never can: it cannot prove the absence of an orphan.
func (v Validation) Conclusive() bool { return v.Method == MethodFull }

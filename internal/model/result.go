package model

import "time"

// SchemaVersion is the version of the JSON contract this build emits. The
// serialized model is a public API — the CI baseline and third-party tooling
// consume it — so any incompatible change requires incrementing this.
const SchemaVersion = "1"

// Result is a complete analysis run.
type Result struct {
	SchemaVersion string    `json:"schema_version"`
	Tool          string    `json:"tool"`
	ToolVersion   string    `json:"tool_version"`
	GeneratedAt   time.Time `json:"generated_at"`

	// Duration is how long the run took. Two results are only comparable
	// against each other when the cost of producing them is on the record.
	Duration time.Duration `json:"duration_ns"`

	// ServerVersion is the PostgreSQL version analyzed. Whether a finding
	// applies at all can depend on it.
	ServerVersion string `json:"server_version,omitempty"`

	// Profile is the naming profile in effect. An inference result is not
	// interpretable without knowing which convention produced it. Empty for
	// commands that make no inferences.
	Profile string `json:"profile,omitempty"`

	// Naming records the conventions detected in the schema, so a run can be
	// reproduced without knowing which flags produced it.
	Naming NamingDetection `json:"naming_detection"`

	Schemas    []Schema    `json:"schemas"`
	Candidates []Candidate `json:"candidates,omitempty"`

	// Discarded fell below the score threshold or was killed by the statistical
	// prefilter. It is a separate field rather than a verdict inside Candidates
	// so that no consumer has to inspect a score to learn whether a candidate
	// survived triage — the same reason a declared key never shares a field
	// with an inferred one.
	Discarded []Candidate `json:"discarded,omitempty"`

	Findings []Finding `json:"findings,omitempty"`

	Coverage Coverage `json:"coverage"`
}

// NewResult starts a result with the contract fields filled in. Coverage is
// required at construction, so no code path can produce a result that says
// nothing about what it failed to look at.
func NewResult(toolVersion, profile string, generatedAt time.Time, coverage Coverage) *Result {
	return &Result{
		SchemaVersion: SchemaVersion,
		Tool:          "pgfathom",
		ToolVersion:   toolVersion,
		GeneratedAt:   generatedAt,
		Profile:       profile,
		Coverage:      coverage,
	}
}

// CandidatesByVerdict returns candidates with the given verdict, in order.
func (r *Result) CandidatesByVerdict(v Verdict) []Candidate {
	var out []Candidate
	for _, c := range r.Candidates {
		if c.Verdict == v {
			out = append(out, c)
		}
	}
	return out
}

// CountByVerdict tallies candidates per verdict.
func (r *Result) CountByVerdict() map[Verdict]int {
	out := make(map[Verdict]int, 5)
	for _, c := range r.Candidates {
		out[c.Verdict]++
	}
	return out
}

// Sampled reports whether any validation was sampled, in which case no result
// in this run can be treated as confirmed.
func (r *Result) Sampled() bool {
	for _, c := range r.Candidates {
		if c.Validation != nil && c.Validation.Method == MethodSampled {
			return true
		}
	}
	return false
}

package model

import "time"

// materialCoverage is the analyzed share below which a run has to say that its
// silence covers less than it appears to.
const materialCoverage = 0.80

// UnsupportedReason says why a table could not be analyzed.
type UnsupportedReason string

// Shapes that single-column inference cannot target.
const (
	ReasonCompositePK  UnsupportedReason = "composite_primary_key"
	ReasonNoPrimaryKey UnsupportedReason = "no_primary_key"
	ReasonPartitioned  UnsupportedReason = "partitioned"
	ReasonInheritance  UnsupportedReason = "table_inheritance"
)

// SkippedTable records one table that did not make it into the analysis, and why.
type SkippedTable struct {
	Table  string            `json:"table"`
	Reason UnsupportedReason `json:"reason,omitempty"`
}

// Coverage describes what was actually analyzed against what exists. It is
// mandatory in every result: a clean report has to mean "I looked and it is
// clean", never "I could not look".
type Coverage struct {
	TablesTotal    int `json:"tables_total"`
	TablesAnalyzed int `json:"tables_analyzed"`

	// TablesNoPrivilege lists tables the connecting role cannot SELECT. A
	// partial grant is the common case in the target environment, not the edge
	// case, so silence here would make an incomplete report look clean.
	TablesNoPrivilege []string `json:"tables_no_privilege,omitempty"`

	// TablesExcluded lists tables removed by user-supplied filters.
	TablesExcluded []string `json:"tables_excluded,omitempty"`

	// TablesUnsupported lists tables whose shape this version cannot analyze.
	TablesUnsupported []SkippedTable `json:"tables_unsupported,omitempty"`

	// SchemasTotal counts the schemas visible to the connected role;
	// SchemasAnalyzed, how many of them were in scope.
	SchemasTotal    int `json:"schemas_total"`
	SchemasAnalyzed int `json:"schemas_analyzed"`

	// SchemasNotAnalyzed lists schemas that exist and were never asked for. A
	// report about public that never mentions the other eleven schemas is
	// accurate and still misleading, which is the exact failure the rule against
	// reporting silence exists to prevent.
	SchemasNotAnalyzed []string `json:"schemas_not_analyzed,omitempty"`

	// SchemasExcluded lists schemas dropped by a user-supplied pattern. Kept
	// apart from the above because asking for something to be skipped and never
	// asking for it at all are different facts, and merging them would claim an
	// intent nobody expressed.
	SchemasExcluded []string `json:"schemas_excluded,omitempty"`

	CandidatesFound     int `json:"candidates_found"`
	CandidatesValidated int `json:"candidates_validated"`
	CandidatesTimedOut  int `json:"candidates_timed_out"`

	// StatsPrefilter reports whether the statistical prefilter ran at all.
	// False and "zero rejected" are different claims: one is "I did not look",
	// the other is "I looked and dropped nothing".
	StatsPrefilter bool `json:"stats_prefilter"`

	// The prefilter funnel. Checked = rejected + penalized-or-clean + without
	// stats; the split is what predicts how many anti-joins validation will
	// cost, and what proves the layer pays for itself.
	CandidatesStatsChecked  int `json:"candidates_stats_checked,omitempty"`
	CandidatesStatsRejected int `json:"candidates_stats_rejected,omitempty"`
	CandidatesWithoutStats  int `json:"candidates_without_stats,omitempty"`

	// StatsResetAt is when server statistics were last reset. Nil means unknown.
	StatsResetAt *time.Time `json:"stats_reset_at"`

	// PgStatStatements reports whether the query log was available to mine.
	PgStatStatements bool `json:"pg_stat_statements"`

	// KeyProbesSkipped lists tables whose missing-key suggestion was not
	// probed against the data, and why — too large for the configured
	// ceiling, or probing disabled outright. A skip here is not silence: the
	// table still carries its missing_primary_key finding, just without a
	// probed verdict backing a candidate key.
	KeyProbesSkipped []SkippedKeyProbe `json:"key_probes_skipped,omitempty"`
}

// SkippedKeyProbe records one table whose key candidates were not tested for
// uniqueness against the data, and why.
type SkippedKeyProbe struct {
	Table  string `json:"table"`
	Reason string `json:"reason"`
}

// Complete reports whether every table in scope was analyzed and every
// candidate resolved.
//
// A schema left out of scope disqualifies completeness too: the only run
// entitled to claim it looked at everything is the one that did.
func (c Coverage) Complete() bool {
	return c.TablesAnalyzed == c.TablesTotal &&
		len(c.TablesNoPrivilege) == 0 &&
		len(c.SchemasNotAnalyzed) == 0 &&
		c.CandidatesTimedOut == 0
}

// AnalyzedShare is the proportion of the table scope that was analyzed.
func (c Coverage) AnalyzedShare() float64 {
	if c.TablesTotal <= 0 {
		return 0
	}
	return float64(c.TablesAnalyzed) / float64(c.TablesTotal)
}

// MateriallyIncomplete reports whether enough of the scope was skipped that
// every conclusion needs reading against it.
//
// Absolute counts hide the shape of the problem: 91 tables skipped sounds minor
// until it turns out to be a quarter of the schema, which is what a real
// municipal database produced.
func (c Coverage) MateriallyIncomplete() bool {
	return c.TablesTotal > 0 && c.AnalyzedShare() < materialCoverage
}

// SkippedCount is how many tables did not make it into the analysis.
func (c Coverage) SkippedCount() int {
	return len(c.TablesNoPrivilege) + len(c.TablesExcluded) + len(c.TablesUnsupported)
}

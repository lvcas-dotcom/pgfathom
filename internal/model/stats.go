package model

import (
	"encoding/json"
	"math"
	"time"
)

// TableStats is size and usage information about a table.
type TableStats struct {
	// EstimatedRows is pg_class.reltuples verbatim. Since PostgreSQL 14 the
	// value is -1 when the table has never been ANALYZEd, which means unknown
	// and not zero. Read it through EstimatedRowCount rather than directly.
	EstimatedRows int64 `json:"estimated_rows"`

	// TotalBytes includes indexes and TOAST.
	TotalBytes int64 `json:"total_bytes"`

	Usage UsageStats `json:"usage"`
}

// RowsUnknown is the reltuples sentinel for a table that was never ANALYZEd.
const RowsUnknown = -1

// EstimatedRowCount returns the planner's row estimate. The second result is
// false when the table has never been ANALYZEd.
//
// Treating the -1 sentinel as a count is not a display glitch: it makes an
// unanalyzed table look empty, which in scoring turns every one of them into a
// small domain table and silently changes the result.
func (s TableStats) EstimatedRowCount() (int64, bool) {
	if s.EstimatedRows < 0 {
		return 0, false
	}
	return s.EstimatedRows, true
}

// RowEstimates maps schema.table to the planner's row count, for the tables
// that have one. A table whose estimate is unknown is absent rather than zero,
// so a lookup miss and an empty table can never be confused — the distinction
// the sentinel exists to preserve, and the reason this resolution has one
// owner instead of a copy in every layer that needs it.
func RowEstimates(schemas []Schema) map[string]int64 {
	out := make(map[string]int64)
	for _, s := range schemas {
		for _, t := range s.Tables {
			if n, ok := t.Stats.EstimatedRowCount(); ok {
				out[s.Name+"."+t.Name] = n
			}
		}
	}
	return out
}

// UsageCounters are the raw activity counters from pg_stat_user_tables.
type UsageCounters struct {
	SeqScans int64 `json:"seq_scans"`
	IdxScans int64 `json:"idx_scans"`
	Inserts  int64 `json:"inserts"`
	Updates  int64 `json:"updates"`
	Deletes  int64 `json:"deletes"`
}

// UsageStats binds activity counters to the moment they started counting.
//
// The counters reset on pg_stat_reset() and pg_upgrade, and are per node: a
// table read only on a replica shows zero reads on the primary. So they are
// unexported, and Counters is the only way out — it hands back the reset
// context in the same call, and a caller cannot read SeqScans without being
// handed the reason it might be lying.
type UsageStats struct {
	counters   UsageCounters
	resetAt    time.Time
	resetKnown bool
}

// NewUsageStats pairs counters with the moment statistics were last reset.
func NewUsageStats(c UsageCounters, resetAt time.Time) UsageStats {
	return UsageStats{counters: c, resetAt: resetAt, resetKnown: true}
}

// NewUsageStatsUnknownReset records counters whose reset moment is unknown.
func NewUsageStatsUnknownReset(c UsageCounters) UsageStats {
	return UsageStats{counters: c}
}

// Counters returns the activity counters and the moment they started counting.
// The third result is false when that moment is unknown, in which case the
// counters carry no meaning and must not drive a finding.
func (u UsageStats) Counters() (UsageCounters, time.Time, bool) {
	return u.counters, u.resetAt, u.resetKnown
}

// Interpretable reports whether the counters can support a conclusion.
func (u UsageStats) Interpretable() bool { return u.resetKnown }

type usageStatsJSON struct {
	Counters UsageCounters `json:"counters"`
	ResetAt  *time.Time    `json:"stats_reset_at"`
}

// MarshalJSON emits the reset moment alongside the counters, so the serialized
// form carries the same caveat the Go API does. A null reset means the counters
// are not interpretable.
func (u UsageStats) MarshalJSON() ([]byte, error) {
	out := usageStatsJSON{Counters: u.counters}
	if u.resetKnown {
		t := u.resetAt
		out.ResetAt = &t
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores counters and reset context.
func (u *UsageStats) UnmarshalJSON(b []byte) error {
	var in usageStatsJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	u.counters = in.Counters
	if in.ResetAt != nil {
		u.resetAt, u.resetKnown = *in.ResetAt, true
	} else {
		u.resetAt, u.resetKnown = time.Time{}, false
	}
	return nil
}

// ColumnStats is planner statistics for one column, used to reject impossible
// candidates before any table I/O.
type ColumnStats struct {
	// NullFraction is the estimated proportion of NULLs, 0..1.
	NullFraction float64 `json:"null_fraction"`

	// NDistinct follows the pg_stats convention: positive is an absolute count,
	// negative is a ratio of the row count, zero is unknown. See
	// EstimatedDistinct.
	NDistinct float64 `json:"n_distinct"`

	// Present is false when the column has no statistics at all, typically
	// because the table was never ANALYZEd. The prefilter must then stay silent
	// rather than invent a rejection.
	Present bool `json:"present"`

	hasBounds bool
	lowBound  string
	highBound string
}

// SetBounds records the histogram endpoints. They are user data, so they stay
// unexported and never reach any output.
func (c *ColumnStats) SetBounds(low, high string) {
	c.hasBounds, c.lowBound, c.highBound = true, low, high
}

// HasBounds reports whether histogram endpoints are available. There is no
// exported accessor for the values themselves and there must not be one; the
// typed range comparison belongs inside this package.
func (c ColumnStats) HasBounds() bool { return c.hasBounds }

// EstimatedDistinct resolves the pg_stats n_distinct convention against a row
// count. The second result is false when the estimate is unavailable.
func (c ColumnStats) EstimatedDistinct(rows int64) (int64, bool) {
	switch {
	case !c.Present, c.NDistinct == 0:
		return 0, false
	case c.NDistinct > 0:
		return int64(math.Round(c.NDistinct)), true
	default:
		if rows <= 0 {
			return 0, false
		}
		return int64(math.Round(-c.NDistinct * float64(rows))), true
	}
}

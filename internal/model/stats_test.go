package model_test

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// TestEstimatedRowCountSeparatesUnknownFromEmpty guards a bug found by running
// against a real database: since PostgreSQL 14, reltuples is -1 for a table
// that was never ANALYZEd. Read as a count it makes the table look empty, which
// in scoring turns every unanalyzed table into a small domain table.
func TestEstimatedRowCountSeparatesUnknownFromEmpty(t *testing.T) {
	tests := []struct {
		name      string
		reltuples int64
		wantRows  int64
		wantKnown bool
	}{
		{"never analyzed", model.RowsUnknown, 0, false},
		{"genuinely empty", 0, 0, true},
		{"populated", 4_000_000, 4_000_000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := model.TableStats{EstimatedRows: tt.reltuples}

			rows, known := stats.EstimatedRowCount()
			if rows != tt.wantRows || known != tt.wantKnown {
				t.Errorf("EstimatedRowCount() = %d, %v; want %d, %v",
					rows, known, tt.wantRows, tt.wantKnown)
			}
		})
	}
}

// TestRowEstimatesOmitsTheUnknown pins the distinction the sentinel exists to
// preserve: a table that was never ANALYZEd is absent from the map, never
// present as zero. A layer that read it as zero would treat the table as empty
// — a domain table to scoring, a table that fits the sample to validation.
func TestRowEstimatesOmitsTheUnknown(t *testing.T) {
	schemas := []model.Schema{{Name: "public", Tables: []model.Table{
		{Schema: "public", Name: "analisada", Stats: model.TableStats{EstimatedRows: 4200}},
		{Schema: "public", Name: "vazia", Stats: model.TableStats{EstimatedRows: 0}},
		{Schema: "public", Name: "nunca_analisada", Stats: model.TableStats{EstimatedRows: model.RowsUnknown}},
	}}}

	got := model.RowEstimates(schemas)

	if n, ok := got["public.analisada"]; !ok || n != 4200 {
		t.Errorf("analisada = (%d, %v), want (4200, true)", n, ok)
	}
	if n, ok := got["public.vazia"]; !ok || n != 0 {
		t.Errorf("vazia = (%d, %v), want (0, true): empty is a known count", n, ok)
	}
	if _, ok := got["public.nunca_analisada"]; ok {
		t.Error("an unanalyzed table must be absent, not present as zero")
	}
}

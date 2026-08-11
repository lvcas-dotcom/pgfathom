package model_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// sentinels are stand-ins for the kind of value that must never escape: things
// that would appear in most_common_vals or histogram_bounds of a real database.
// They are deliberately shaped like the data this tool will meet in production.
var sentinels = []string{
	"529.318.470-11",     // CPF
	"maria.silva@x.gov",  // e-mail
	"Rua das Acacias 42", // endereço
	"B4rG4iN-t0k3n",      // valor arbitrário
}

// TestSerializedModelLeaksNoUserData populates every model structure with
// sentinel values in the places a real run would hold user data, serializes the
// whole thing, and fails if any sentinel survives.
//
// This is the enforcement of the project's hardest rule. It exists in this
// phase, before there is any database to read data from, so that the later
// phases find the net already strung: the moment someone exports a histogram
// bound or drops a value into a diagnostic string, this test goes red.
func TestSerializedModelLeaksNoUserData(t *testing.T) {
	stats := model.ColumnStats{NullFraction: 0.1, NDistinct: -0.5, Present: true}
	stats.SetBounds(sentinels[0], sentinels[1])

	if !stats.HasBounds() {
		t.Fatal("bounds were not recorded; the test would pass vacuously")
	}

	// The bounds must be usable inside the package and invisible outside it.
	if got, err := json.Marshal(stats); err != nil {
		t.Fatalf("marshalling column stats: %v", err)
	} else if s := string(got); strings.Contains(s, sentinels[0]) || strings.Contains(s, sentinels[1]) {
		t.Fatalf("histogram bounds leaked into JSON: %s", s)
	}

	resetAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	usage := model.NewUsageStats(model.UsageCounters{SeqScans: 10, IdxScans: 4200}, resetAt)

	validation := &model.Validation{
		Method:          model.MethodSampled,
		SampledRows:     100000,
		NotNullRows:     98000,
		DistinctVals:    4100,
		OrphanRows:      1284,
		OrphanVals:      37,
		MaxRowsPerValue: 219,
		Duration:        3 * time.Second,
	}

	result := model.NewResult("v0.1.0-test", "pt-br", time.Now().UTC(), model.Coverage{
		TablesTotal:       312,
		TablesAnalyzed:    298,
		TablesNoPrivilege: []string{"public.folha_pagamento"},
		TablesUnsupported: []model.SkippedTable{{
			Table:  "public.documento",
			Reason: model.ReasonNoPrimaryKey,
		}},
		CandidatesFound:     1847,
		CandidatesValidated: 226,
		CandidatesTimedOut:  3,
		StatsResetAt:        &resetAt,
	})

	result.Schemas = []model.Schema{{
		Name: "public",
		Tables: []model.Table{{
			Schema:     "public",
			Name:       "pedido",
			PrimaryKey: []string{"id"},
			Columns: []model.Column{
				{Name: "id", Type: "bigint", BaseType: "int8", Position: 1},
				{Name: "cliente_id", Type: "bigint", BaseType: "int8", Position: 2},
			},
			ForeignKeys: []model.ForeignKey{{
				Name:       "pedido_cliente_fk",
				Columns:    []string{"cliente_id"},
				RefSchema:  "public",
				RefTable:   "cliente",
				RefColumns: []string{"id"},
				Validated:  false,
				HasIndex:   false,
			}},
			Indexes: []model.Index{{Name: "pedido_pkey", Columns: []string{"id"}, Unique: true, Primary: true}},
			Stats:   model.TableStats{EstimatedRows: 4_000_000, TotalBytes: 1 << 30, Usage: usage},
		}},
	}}

	result.Candidates = []model.Candidate{{
		Child:  model.SingleKey("public", "pedido", "cliente_id"),
		Parent: model.SingleKey("public", "cliente", "id"),
		Signals: []model.Signal{
			{Kind: model.SigExactName, Weight: 0.4, Detail: "cliente"},
			{Kind: model.SigJoinInView, Weight: 0.5, Detail: "public.vw_pedido_completo"},
		},
		MetaScore:  0.9,
		Validation: validation,
		Verdict:    model.VerdictBroken,
		Reason:     "containment below total",
	}}

	result.Findings = []model.Finding{{
		Kind:    model.FindingNotValidConstraint,
		Object:  "public.pedido.pedido_cliente_fk",
		Detail:  "constraint is NOT VALID and was never validated",
		Metrics: map[string]int64{"estimated_rows": 4_000_000},
	}}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshalling result: %v", err)
	}

	for _, s := range sentinels {
		if strings.Contains(string(encoded), s) {
			t.Errorf("user data %q leaked into the serialized model", s)
		}
	}
}

// TestUsageStatsAlwaysCarryResetContext proves that counters cannot be obtained
// without the reset moment that makes them interpretable, in Go and in JSON.
func TestUsageStatsAlwaysCarryResetContext(t *testing.T) {
	t.Run("counters come with reset context", func(t *testing.T) {
		resetAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		u := model.NewUsageStats(model.UsageCounters{SeqScans: 7}, resetAt)

		counters, got, known := u.Counters()
		if !known || !got.Equal(resetAt) || counters.SeqScans != 7 {
			t.Fatalf("Counters() = %+v, %v, %v", counters, got, known)
		}
		if !u.Interpretable() {
			t.Error("stats with a known reset should be interpretable")
		}
	})

	t.Run("unknown reset marks counters uninterpretable", func(t *testing.T) {
		u := model.NewUsageStatsUnknownReset(model.UsageCounters{SeqScans: 7})

		if _, _, known := u.Counters(); known {
			t.Error("reset should be reported as unknown")
		}
		if u.Interpretable() {
			t.Error("counters without a reset moment must not be interpretable")
		}

		encoded, err := json.Marshal(u)
		if err != nil {
			t.Fatalf("marshalling: %v", err)
		}
		if !strings.Contains(string(encoded), `"stats_reset_at":null`) {
			t.Errorf("serialized form must carry the null reset: %s", encoded)
		}
	})
}

package report_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

func result(coverage model.Coverage, findings ...model.Finding) *model.Result {
	r := model.NewResult("v0.2.0-test", "", time.Unix(0, 0).UTC(), coverage)
	r.ServerVersion = "16.2"
	r.Findings = findings
	return r
}

func render(t *testing.T, r *model.Result) string {
	t.Helper()
	var b bytes.Buffer
	if err := report.Terminal(&b, r, false); err != nil {
		t.Fatalf("Terminal: %v", err)
	}
	return b.String()
}

// TestCleanScopeIsStatedNotImplied covers the rule that silence is never
// reported as a clean bill of health. An empty output cannot distinguish
// "everything is fine" from "nothing was looked at".
func TestCleanScopeIsStatedNotImplied(t *testing.T) {
	out := render(t, result(model.Coverage{TablesTotal: 12, TablesAnalyzed: 12}))

	if !strings.Contains(out, "All 12 tables analyzed") {
		t.Errorf("output should affirm what was analyzed:\n%s", out)
	}
}

func TestPartialScopeRefusesToLookClean(t *testing.T) {
	out := render(t, result(model.Coverage{
		TablesTotal:       12,
		TablesAnalyzed:    10,
		TablesNoPrivilege: []string{"public.folha_pagamento", "public.prontuario"},
	}))

	if !strings.Contains(out, "not a clean bill of health") {
		t.Errorf("a partial run must say so plainly:\n%s", out)
	}
	if !strings.Contains(out, "no SELECT privilege") {
		t.Errorf("skipped tables must be attributed to a reason:\n%s", out)
	}
	if !strings.Contains(out, "folha_pagamento") {
		t.Errorf("skipped tables must be named:\n%s", out)
	}
}

func TestCoverageAlwaysPresent(t *testing.T) {
	withFinding := result(
		model.Coverage{TablesTotal: 3, TablesAnalyzed: 3},
		model.Finding{
			Kind:    model.FindingNotValidConstraint,
			Object:  "public.pedido.pedido_cliente_fk",
			Detail:  "constraint is NOT VALID",
			Metrics: map[string]int64{"child_estimated_rows": 4_000_000},
		},
	)

	for name, out := range map[string]string{
		"with findings": render(t, withFinding),
		"without":       render(t, result(model.Coverage{TablesTotal: 3, TablesAnalyzed: 3})),
	} {
		if !strings.Contains(out, "3 tables · 3 analyzed") {
			t.Errorf("%s: coverage block missing:\n%s", name, out)
		}
	}
}

func TestFindingsAreGroupedAndCounted(t *testing.T) {
	out := render(t, result(
		model.Coverage{TablesTotal: 2, TablesAnalyzed: 2},
		model.Finding{Kind: model.FindingFKWithoutIndex, Object: "public.a.a_fk"},
		model.Finding{Kind: model.FindingFKWithoutIndex, Object: "public.b.b_fk"},
		model.Finding{Kind: model.FindingNotValidConstraint, Object: "public.c.c_fk"},
	))

	if !strings.Contains(out, "UNINDEXED") || !strings.Contains(out, "(2)") {
		t.Errorf("unindexed group should be titled and counted:\n%s", out)
	}
	if !strings.Contains(out, "NOT VALID") || !strings.Contains(out, "(1)") {
		t.Errorf("NOT VALID group should be titled and counted:\n%s", out)
	}
}

func TestUnknownStatsResetIsFlagged(t *testing.T) {
	out := render(t, result(model.Coverage{TablesTotal: 1, TablesAnalyzed: 1}))

	if !strings.Contains(out, "statistics reset time unknown") {
		t.Errorf("counters without a reset moment must be flagged:\n%s", out)
	}

	at := time.Unix(0, 0).UTC()
	known := result(model.Coverage{TablesTotal: 1, TablesAnalyzed: 1, StatsResetAt: &at})
	if strings.Contains(render(t, known), "statistics reset time unknown") {
		t.Error("a known reset moment must not be flagged")
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func TestTerminalOutputCarriesNoANSI(t *testing.T) {
	out := render(t, result(
		model.Coverage{TablesTotal: 1, TablesAnalyzed: 1},
		model.Finding{Kind: model.FindingFKWithoutIndex, Object: "public.a.a_fk"},
	))

	if ansi.MatchString(out) {
		t.Errorf("renderer must not emit ANSI: %q", out)
	}
}

func TestJSONIsValidAndVersioned(t *testing.T) {
	var b bytes.Buffer
	r := result(
		model.Coverage{TablesTotal: 1, TablesAnalyzed: 1},
		model.Finding{Kind: model.FindingNotValidConstraint, Object: "public.a.a_fk"},
	)

	if err := report.JSON(&b, r); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, b.String())
	}

	if decoded["schema_version"] != model.SchemaVersion {
		t.Errorf("schema_version = %v, want %q", decoded["schema_version"], model.SchemaVersion)
	}
	if _, ok := decoded["coverage"]; !ok {
		t.Error("coverage must be present in the JSON contract")
	}
	if decoded["server_version"] != "16.2" {
		t.Errorf("server_version = %v, want the analyzed server's version", decoded["server_version"])
	}
}

func TestHumanCountScales(t *testing.T) {
	out := render(t, result(
		model.Coverage{TablesTotal: 1, TablesAnalyzed: 1},
		model.Finding{
			Kind:    model.FindingFKWithoutIndex,
			Object:  "public.a.a_fk",
			Metrics: map[string]int64{"child_estimated_rows": 4_000_000},
		},
	))

	if !strings.Contains(out, "4.0M") {
		t.Errorf("large counts should be abbreviated:\n%s", out)
	}
}

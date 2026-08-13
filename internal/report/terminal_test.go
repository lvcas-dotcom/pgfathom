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
	if err := report.Terminal(&b, r, report.NoEmphasis); err != nil {
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

// TestSchemaLeftOutIsCountedInItsOwnUnits covers the run where every table in
// scope was analyzed and the scope itself was the omission. Counting that in
// tables would report "0 were skipped" underneath a refusal to call the run
// clean, which is the confusing answer sitting exactly where the honest one goes.
func TestSchemaLeftOutIsCountedInItsOwnUnits(t *testing.T) {
	out := render(t, result(model.Coverage{
		TablesTotal: 4, TablesAnalyzed: 4,
		SchemasTotal: 3, SchemasAnalyzed: 1,
		SchemasNotAnalyzed: []string{"vendas", "financeiro"},
	}))

	if !strings.Contains(out, "not a clean bill of health") {
		t.Errorf("a run that skipped a whole schema must not look clean:\n%s", out)
	}
	if !strings.Contains(out, "2 schemas were never looked at") {
		t.Errorf("the omission must be counted in schemas, not tables:\n%s", out)
	}
	if strings.Contains(out, "0 tables were skipped") {
		t.Errorf("no table was skipped, so nothing should claim otherwise:\n%s", out)
	}
	if !strings.Contains(out, "--all-schemas") {
		t.Errorf("the coverage block must point at the flag that widens scope:\n%s", out)
	}
}

// TestBothOmissionsAreNamed keeps one omission from masking the other.
func TestBothOmissionsAreNamed(t *testing.T) {
	out := render(t, result(model.Coverage{
		TablesTotal: 12, TablesAnalyzed: 10,
		TablesNoPrivilege:  []string{"public.folha", "public.prontuario"},
		SchemasTotal:       3,
		SchemasAnalyzed:    1,
		SchemasNotAnalyzed: []string{"vendas"},
	}))

	if !strings.Contains(out, "2 tables were skipped") {
		t.Errorf("skipped tables must still be counted:\n%s", out)
	}
	if !strings.Contains(out, "1 schemas were never looked at") {
		t.Errorf("the schema left out must be counted alongside:\n%s", out)
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
		if !strings.Contains(out, "3 tables · 3 analyzed (100%)") {
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
	// The schema block is highlighted when colour is on, so it has to be in
	// scope here: a piped run that leaks an escape sequence corrupts the file it
	// was redirected into.
	for name, coverage := range map[string]model.Coverage{
		"single schema": {TablesTotal: 1, TablesAnalyzed: 1},
		"schemas left out": {
			TablesTotal: 1, TablesAnalyzed: 1,
			SchemasTotal: 3, SchemasAnalyzed: 1,
			SchemasNotAnalyzed: []string{"vendas"},
			SchemasExcluded:    []string{"auditoria"},
		},
	} {
		out := render(t, result(coverage,
			model.Finding{Kind: model.FindingFKWithoutIndex, Object: "public.a.a_fk"},
		))

		if ansi.MatchString(out) {
			t.Errorf("%s: renderer must not emit ANSI: %q", name, out)
		}
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

// TestMateriallyIncompleteScopeIsFlagged covers a shape a real municipal
// database produced: 91 tables skipped reads as minor until it turns out to be a
// quarter of the schema.
func TestMateriallyIncompleteScopeIsFlagged(t *testing.T) {
	out := render(t, result(model.Coverage{
		TablesTotal:       338,
		TablesAnalyzed:    247,
		TablesUnsupported: []model.SkippedTable{{Table: "public.x", Reason: model.ReasonNoPrimaryKey}},
	}))

	if !strings.Contains(out, "(73%)") {
		t.Errorf("the analyzed share must be shown, not only the counts:\n%s", out)
	}
	if !strings.Contains(out, "covers that fraction") {
		t.Errorf("a materially incomplete run must say what its conclusions cover:\n%s", out)
	}
}

func TestCompleteScopeCarriesNoWarning(t *testing.T) {
	out := render(t, result(model.Coverage{TablesTotal: 100, TablesAnalyzed: 100}))

	if strings.Contains(out, "covers that fraction") {
		t.Errorf("a complete run must not carry the partial-coverage warning:\n%s", out)
	}
	if !strings.Contains(out, "100 analyzed (100%)") {
		t.Errorf("the share is shown either way:\n%s", out)
	}
}

// TestDetectionCountsReadAsEnglish covers the branch the golden files cannot:
// they exercise counts above one, where singular and plural render the same
// bytes. A schema with a single table and a single declared key is a real
// first run — the smallest one somebody tries the tool on — and "1 tables and
// 1 declared keys" is the first thing they read.
func TestDetectionCountsReadAsEnglish(t *testing.T) {
	r := model.NewResult("test", "en", time.Unix(0, 0).UTC(),
		model.Coverage{TablesTotal: 1, TablesAnalyzed: 1})

	var b bytes.Buffer
	view := report.DiscoverView{
		Result:    r,
		Detection: model.NamingDetection{Enabled: true, Tables: 1, DeclaredKeys: 1},
	}
	if err := report.Discover(&b, view); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got := b.String(); !strings.Contains(got, "1 table and 1 declared key;") {
		t.Errorf("counts of one must read as singular, got:\n%s", got)
	}
}

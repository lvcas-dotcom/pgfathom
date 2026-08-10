package report_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

func candidate(child, parent string, score float64, signals ...model.SignalKind) model.Candidate {
	c := model.Candidate{
		Child:     model.ColumnRef{Schema: "public", Table: "pedido", Column: child},
		Parent:    model.ColumnRef{Schema: "public", Table: parent, Column: "id"},
		MetaScore: score,
		Verdict:   model.VerdictUnvalidated,
	}
	for _, k := range signals {
		c.Signals = append(c.Signals, model.Signal{Kind: k, Weight: 0.2})
	}
	return c
}

func discoverView(candidates, discarded []model.Candidate, show bool, findings ...model.Finding) report.DiscoverView {
	r := model.NewResult("v0.3.0-test", "pt-br", time.Unix(0, 0).UTC(),
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})
	r.ServerVersion = "16.2"
	r.Candidates = candidates
	r.Findings = findings

	return report.DiscoverView{
		Result:          r,
		Discarded:       discarded,
		MinScore:        0.5,
		ShowDiscarded:   show,
		ValidationStage: "! candidates only — the data has NOT been consulted",
	}
}

func renderDiscover(t *testing.T, v report.DiscoverView) string {
	t.Helper()
	var b bytes.Buffer
	if err := report.Discover(&b, v); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return b.String()
}

// TestLimitationIsAlwaysStated is the guard against the mistake the tool exists
// to prevent: a well-scored candidate read as a confirmed relationship, and a
// constraint created from a name match.
func TestLimitationIsAlwaysStated(t *testing.T) {
	for name, v := range map[string]report.DiscoverView{
		"with candidates": discoverView([]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, nil, false),
		"empty":           discoverView(nil, nil, false),
	} {
		out := renderDiscover(t, v)
		if !strings.Contains(out, "NOT been consulted") {
			t.Errorf("%s: output must state that the data was not read:\n%s", name, out)
		}
	}
}

func TestProfileAndThresholdAppearInTheHeader(t *testing.T) {
	out := renderDiscover(t, discoverView([]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, nil, false))

	if !strings.Contains(out, "profile pt-br") {
		t.Errorf("a result is not interpretable without the naming profile:\n%s", out)
	}
	if !strings.Contains(out, "threshold 0.50") {
		t.Errorf("the threshold shapes the whole result and must be visible:\n%s", out)
	}
}

func TestSignalsAreShownSoScoresCanBeArguedWith(t *testing.T) {
	out := renderDiscover(t, discoverView(
		[]model.Candidate{candidate("cliente_id", "cliente", 0.75, model.SigExactName, model.SigIdenticalType)},
		nil, false))

	for _, want := range []string{"exact_name", "identical_type"} {
		if !strings.Contains(out, want) {
			t.Errorf("signal %q missing; a score without provenance is not auditable:\n%s", want, out)
		}
	}
}

func TestDiscardedAreCountedEvenWhenHidden(t *testing.T) {
	discarded := []model.Candidate{candidate("status_id", "status", 0.30)}

	hidden := renderDiscover(t, discoverView(
		[]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, discarded, false))

	if strings.Contains(hidden, "status_id") {
		t.Errorf("discarded candidates should not be listed unless asked for:\n%s", hidden)
	}
	if !strings.Contains(hidden, "--include-rejected") {
		t.Errorf("their existence must be announced so nobody wonders why a column was ignored:\n%s", hidden)
	}

	shown := renderDiscover(t, discoverView(
		[]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, discarded, true))

	if !strings.Contains(shown, "status_id") {
		t.Errorf("--include-rejected must list them:\n%s", shown)
	}
}

func TestPenaltiesAreVisuallyDistinct(t *testing.T) {
	c := candidate("status_id", "status", 0.30)
	c.Signals = append(c.Signals, model.Signal{Kind: model.SigGenericDomain, Weight: -0.3})

	out := renderDiscover(t, discoverView([]model.Candidate{c}, nil, false))

	if !strings.Contains(out, "-generic_domain_name") {
		t.Errorf("a penalty must read as a penalty:\n%s", out)
	}
}

func TestRecognizedButUnsupportedIsReported(t *testing.T) {
	out := renderDiscover(t, discoverView(nil, nil, false,
		model.Finding{
			Kind:   model.FindingPolymorphicPair,
			Object: "public.documento.entidade_id",
			Detail: "only meaningful beside entidade_tipo",
		},
	))

	if !strings.Contains(out, "NOT ANALYZED") {
		t.Errorf("a recognized limitation must read as an observation, not as silence:\n%s", out)
	}
	if !strings.Contains(out, "entidade_tipo") {
		t.Errorf("the observation must name what it recognized:\n%s", out)
	}
}

func TestEmptyResultIsExplicit(t *testing.T) {
	out := renderDiscover(t, discoverView(nil, nil, false))

	if !strings.Contains(out, "No candidate relationships above the threshold") {
		t.Errorf("an empty result must say so:\n%s", out)
	}
}

func TestDiscoverCoverageIsPresent(t *testing.T) {
	out := renderDiscover(t, discoverView([]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, nil, false))

	if !strings.Contains(out, "4 tables · 4 analyzed (100%)") {
		t.Errorf("coverage must accompany every run:\n%s", out)
	}
}

func TestDiscoverEmitsNoANSI(t *testing.T) {
	out := renderDiscover(t, discoverView([]model.Candidate{candidate("cliente_id", "cliente", 0.75)}, nil, false))

	if ansi.MatchString(out) {
		t.Errorf("renderer must not emit ANSI: %q", out)
	}
}

func TestDetectionIsReported(t *testing.T) {
	v := discoverView(nil, nil, false)
	v.Detection = model.NamingDetection{
		Enabled:        true,
		ColumnSuffixes: []model.NamingEvidence{{Affix: "_idkey", Occurrences: 102, Share: 0.22}},
		DeclaredKeys:   470,
		Tables:         338,
	}

	out := renderDiscover(t, v)

	if !strings.Contains(out, "_idkey") {
		t.Errorf("a convention learned from the schema must be named:\n%s", out)
	}
	if !strings.Contains(out, "102 occurrences") {
		t.Errorf("the evidence must accompany it, or the result cannot be argued with:\n%s", out)
	}
}

func TestDetectionOffIsStated(t *testing.T) {
	out := renderDiscover(t, discoverView(nil, nil, false))

	if !strings.Contains(out, "detection is off") {
		t.Errorf("running without detection must say so:\n%s", out)
	}
}

func TestNothingDetectedIsStated(t *testing.T) {
	v := discoverView(nil, nil, false)
	v.Detection = model.NamingDetection{Enabled: true, Tables: 12, DeclaredKeys: 0}

	out := renderDiscover(t, v)

	if !strings.Contains(out, "Nothing detected") {
		t.Errorf("an empty detection must be stated, not implied:\n%s", out)
	}
}

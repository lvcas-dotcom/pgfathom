package report_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// The golden fixtures pin formatting, and formatting is only pinnable when the
// input is fixed. Version, timestamp and duration are frozen here rather than
// scrubbed out of the rendered text: normalizing on the way out would mean the
// renderer carries a branch that exists solely to make a test possible.
const (
	goldenVersion  = "v0.7.0-test"
	goldenServer   = "16.2"
	goldenDuration = 4200 * time.Millisecond
)

func goldenTime() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }

func validated(method model.ValidationMethod, notNull, distinct, orphanRows, orphanVals int64) *model.Validation {
	return &model.Validation{
		Method:          method,
		SampledRows:     notNull,
		NotNullRows:     notNull,
		DistinctVals:    distinct,
		OrphanRows:      orphanRows,
		OrphanVals:      orphanVals,
		MaxRowsPerValue: 812,
		Duration:        320 * time.Millisecond,
	}
}

func rel(table, column, parent string) (model.ColumnRef, model.ColumnRef) {
	return model.ColumnRef{Schema: "public", Table: table, Column: column},
		model.ColumnRef{Schema: "public", Table: parent, Column: "id"}
}

func verdictCandidate(table, column, parent string, verdict model.Verdict, v *model.Validation, reason string) model.Candidate {
	child, p := rel(table, column, parent)
	return model.Candidate{
		Child:      child,
		Parent:     p,
		Signals:    []model.Signal{{Kind: model.SigExactName, Weight: 0.4}, {Kind: model.SigIdenticalType, Weight: 0.2}},
		MetaScore:  0.82,
		Validation: v,
		Verdict:    verdict,
		Reason:     reason,
	}
}

// goldenResult is a run with one candidate per verdict, which is the shape the
// grouped report exists to render.
func goldenResult(method model.ValidationMethod, coverage model.Coverage) *model.Result {
	r := model.NewResult(goldenVersion, "pt-br", goldenTime(), coverage)
	r.ServerVersion = goldenServer
	r.Duration = goldenDuration
	r.Schemas = goldenSchemas()

	// A clean result means different things in the two modes, and the fixture
	// has to respect that: a sampled run cannot confirm, so the same candidate
	// that comes back confirmed under --full comes back weak here. Pinning a
	// golden file that showed "confirmed / sampled" would enshrine the one
	// combination the tool promises never to produce.
	clean := verdictCandidate("pedido", "cliente_id", "cliente",
		model.VerdictConfirmed, validated(method, 1_284_000, 48_120, 0, 0), "")
	if method == model.MethodSampled {
		clean.Verdict = model.VerdictWeak
		clean.Reason = "probable: no orphans in the sample, but only --full can prove absence"
	}

	r.Candidates = []model.Candidate{
		verdictCandidate("nota_fiscal", "pedido_id", "pedido",
			model.VerdictBroken, validated(method, 812_400, 31_002, 1284, 12), ""),
		clean,
		verdictCandidate("item", "produto_id", "produto",
			model.VerdictWeak, validated(method, 40_000, 900, 12_000, 270),
			"containment 70% sits between rejection and breakage: a human call"),
		verdictCandidate("log", "usuario_id", "usuario",
			model.VerdictUnvalidated, nil, "validation exceeded the time ceiling"),
	}
	return r
}

// goldenSchemas backs the SQL artifacts: pedido.cliente_id has no index, which
// is what makes the CREATE INDEX CONCURRENTLY block appear.
func goldenSchemas() []model.Schema {
	return []model.Schema{{
		Name: "public",
		Tables: []model.Table{
			{
				Schema: "public", Name: "pedido",
				Columns:    []model.Column{{Name: "id"}, {Name: "cliente_id"}},
				PrimaryKey: []string{"id"},
				Indexes:    []model.Index{{Name: "pedido_pkey", Columns: []string{"id"}, Primary: true, Unique: true}},
			},
			{
				Schema: "public", Name: "nota_fiscal",
				Columns:    []model.Column{{Name: "id"}, {Name: "pedido_id"}},
				PrimaryKey: []string{"id"},
				Indexes: []model.Index{
					{Name: "nota_fiscal_pkey", Columns: []string{"id"}, Primary: true, Unique: true},
					{Name: "ix_nf_pedido", Columns: []string{"pedido_id", "emitida_em"}},
				},
			},
		},
	}}
}

func goldenView(r *model.Result, stage string) report.DiscoverView {
	return report.DiscoverView{
		Result:          r,
		MinScore:        0.5,
		ValidationStage: stage,
		Detection: model.NamingDetection{
			Enabled:        true,
			ColumnSuffixes: []model.NamingEvidence{{Affix: "_id", Occurrences: 102, Share: 0.22}},
			DeclaredKeys:   470,
			Tables:         338,
		},
	}
}

const (
	stageFull    = "full validation — every row was examined; verdicts are conclusive"
	stageSampled = "! sampled validation (100000 rows/table target) — orphan counts are floors, " +
		"nothing here is confirmed; re-run with --full to prove absence"
)

func TestDiscoverGolden(t *testing.T) {
	full := model.Coverage{TablesTotal: 4, TablesAnalyzed: 4, CandidatesFound: 9, CandidatesValidated: 3, CandidatesTimedOut: 1}

	partial := model.Coverage{
		TablesTotal: 12, TablesAnalyzed: 9,
		TablesNoPrivilege: []string{"public.folha", "public.salario"},
		TablesUnsupported: []model.SkippedTable{{Table: "public.evento", Reason: model.ReasonNoPrimaryKey}},
		CandidatesFound:   9, CandidatesValidated: 3, CandidatesTimedOut: 1,
		StatsPrefilter: true, CandidatesStatsChecked: 9, CandidatesStatsRejected: 4, CandidatesWithoutStats: 1,
	}

	for name, view := range map[string]report.DiscoverView{
		"discover_all_verdicts":    goldenView(goldenResult(model.MethodFull, full), stageFull),
		"discover_sampled":         goldenView(goldenResult(model.MethodSampled, full), stageSampled),
		"discover_partial_scope":   goldenView(goldenResult(model.MethodFull, partial), stageFull),
		"discover_nothing_to_show": goldenView(emptyResult(full), stageFull),
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Golden(t, name, renderDiscover(t, view))
		})
	}
}

func emptyResult(coverage model.Coverage) *model.Result {
	r := model.NewResult(goldenVersion, "pt-br", goldenTime(), coverage)
	r.ServerVersion = goldenServer
	r.Duration = goldenDuration
	return r
}

// TestDiscoverGroupsBrokenFirst pins the ordering decision itself, so it cannot
// be lost in a golden update that nobody reads closely.
func TestDiscoverGroupsBrokenFirst(t *testing.T) {
	out := renderDiscover(t, goldenView(goldenResult(model.MethodFull,
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}), stageFull))

	broken := strings.Index(out, "BROKEN")
	confirmed := strings.Index(out, "CONFIRMED")

	switch {
	case broken < 0 || confirmed < 0:
		t.Fatalf("both groups must be present:\n%s", out)
	case broken > confirmed:
		t.Errorf("broken is the urgent finding and must lead the report:\n%s", out)
	}
}

// TestEmptyVerdictGroupsAreStated guards rule 4 at the group level: an absent
// group cannot be told apart from one that was never evaluated.
func TestEmptyVerdictGroupsAreStated(t *testing.T) {
	r := model.NewResult(goldenVersion, "pt-br", goldenTime(),
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})
	r.ServerVersion = goldenServer
	r.Candidates = []model.Candidate{
		verdictCandidate("pedido", "cliente_id", "cliente",
			model.VerdictConfirmed, validated(model.MethodFull, 1000, 100, 0, 0), ""),
	}

	out := renderDiscover(t, goldenView(r, stageFull))

	for _, want := range []string{"BROKEN", "WEAK", "UNVALIDATED"} {
		if !strings.Contains(out, want) {
			t.Errorf("group %q must be stated even when empty:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "none") {
		t.Errorf("an empty group must say so explicitly:\n%s", out)
	}
}

// TestSampledWarningAppearsTwice covers the caveat a reader must not miss. It
// leads the report and closes it, because a sampled clean result read as proof
// of absence is precisely the misreading the tool exists to prevent.
func TestSampledWarningAppearsTwice(t *testing.T) {
	out := renderDiscover(t, goldenView(
		goldenResult(model.MethodSampled, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}), stageSampled))

	if n := strings.Count(out, "--full"); n < 2 {
		t.Errorf("the sampling caveat must open and close the report, saw %d mentions:\n%s", n, out)
	}

	clean := renderDiscover(t, goldenView(
		goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}), stageFull))

	if strings.Contains(clean, "sampled run") {
		t.Errorf("a full run must not carry the sampling caveat:\n%s", clean)
	}
}

// TestSampledReportNeverShowsAConfirmation is the renderer-side guard on the
// rule with no exceptions. The verdict is assigned upstream, but a report that
// could print "confirmed / sampled" is a report someone will one day cite.
func TestSampledReportNeverShowsAConfirmation(t *testing.T) {
	out := renderDiscover(t, goldenView(
		goldenResult(model.MethodSampled, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}), stageSampled))

	// The caveats themselves carry both words, so the assertion has to look at
	// the candidate rows: a relation line is the only place a verdict and a
	// method meet.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, " → ") && strings.Contains(line, string(model.VerdictConfirmed)) {
			t.Errorf("a sampled run must never render a confirmation: %q", line)
		}
	}

	if !strings.Contains(out, "CONFIRMED — total containment, verified row by row  (0)") {
		t.Errorf("the confirmed group must be present and empty in a sampled run:\n%s", out)
	}
}

// TestEmphasisIsOptInAndOffByDefault proves the pipe stays clean and that the
// renderer takes the decision from its caller rather than from the environment.
func TestEmphasisIsOptInAndOffByDefault(t *testing.T) {
	view := goldenView(goldenResult(model.MethodSampled,
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}), stageSampled)

	plain := renderDiscover(t, view)
	if ansi.MatchString(plain) {
		t.Errorf("with emphasis off no escape may be emitted:\n%q", plain)
	}

	view.Emphasis = true
	colored := renderDiscover(t, view)
	if !ansi.MatchString(colored) {
		t.Error("with emphasis on the sampling caveat must be highlighted")
	}
	if ansi.ReplaceAllString(colored, "") != plain {
		t.Error("emphasis may only add escapes; the text itself must be identical")
	}
}

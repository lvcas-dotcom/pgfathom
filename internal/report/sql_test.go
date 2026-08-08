package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func artifactByName(t *testing.T, artifacts []report.Artifact, name string) report.Artifact {
	t.Helper()
	for _, a := range artifacts {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("artifact %s was not generated; got %d artifacts", name, len(artifacts))
	return report.Artifact{}
}

// notValidResult carries a declared constraint that never verified the rows
// already in the table, including a composite one.
func notValidResult() *model.Result {
	r := model.NewResult(goldenVersion, "", goldenTime(), model.Coverage{TablesTotal: 3, TablesAnalyzed: 3})
	r.ServerVersion = goldenServer
	r.Schemas = []model.Schema{{
		Name: "public",
		Tables: []model.Table{
			{
				Schema: "public", Name: "nota_fiscal",
				ForeignKeys: []model.ForeignKey{
					{
						Name: "nota_fiscal_pedido_fkey", Columns: []string{"pedido_id"},
						RefSchema: "public", RefTable: "pedido", RefColumns: []string{"id"},
						Validated: false, HasIndex: true,
					},
					{
						Name: "nota_fiscal_ok_fkey", Columns: []string{"emitente_id"},
						RefSchema: "public", RefTable: "emitente", RefColumns: []string{"id"},
						Validated: true, HasIndex: true,
					},
				},
			},
			{
				Schema: "public", Name: "item",
				ForeignKeys: []model.ForeignKey{{
					Name:      "item_composto_fkey",
					Columns:   []string{"empresa_id", "produto_id"},
					RefSchema: "public", RefTable: "produto",
					RefColumns: []string{"empresa_id", "id"},
					Validated:  false,
				}},
			},
		},
	}}
	return r
}

func TestSQLArtifactsGolden(t *testing.T) {
	full := model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}

	discoverFull := report.DiscoverArtifacts(goldenResult(model.MethodFull, full))
	discoverSampled := report.DiscoverArtifacts(goldenResult(model.MethodSampled, full))
	audit := report.AuditArtifacts(notValidResult())

	for name, content := range map[string]string{
		"sql_confirmed":         artifactByName(t, discoverFull, report.FileConfirmed).Content,
		"sql_broken":            artifactByName(t, discoverFull, report.FileBroken).Content,
		"sql_confirmed_sampled": artifactByName(t, discoverSampled, report.FileConfirmed).Content,
		"sql_not_valid":         artifactByName(t, audit, report.FileNotValid).Content,
	} {
		t.Run(name, func(t *testing.T) { testutil.Golden(t, name, content) })
	}
}

// TestEveryArtifactCarriesTheReviewHeader covers the rule that nothing emitted
// is meant to be run unread — including the files for categories that came back
// empty, which are still written.
func TestEveryArtifactCarriesTheReviewHeader(t *testing.T) {
	empty := model.NewResult(goldenVersion, "pt-br", goldenTime(),
		model.Coverage{TablesTotal: 2, TablesAnalyzed: 2})
	empty.ServerVersion = goldenServer

	sets := [][]report.Artifact{
		report.DiscoverArtifacts(goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})),
		report.DiscoverArtifacts(empty),
		report.AuditArtifacts(notValidResult()),
		report.AuditArtifacts(empty),
	}

	for _, set := range sets {
		if len(set) == 0 {
			t.Fatal("a category with no findings must still produce a file")
		}
		for _, a := range set {
			if !strings.Contains(a.Content, "REVIEW BEFORE RUNNING") {
				t.Errorf("%s must open with the mandatory-review header:\n%s", a.Name, a.Content)
			}
			if !strings.Contains(a.Content, goldenVersion) {
				t.Errorf("%s must record the tool version that produced it", a.Name)
			}
		}
	}
}

// TestEmptyCategoryStatesWhyNotJustHowMany guards rule 4 in the artifacts. In
// sampled mode "nothing confirmed" is a consequence of the mode, and reporting
// a count of zero would hide that.
func TestEmptyCategoryStatesWhyNotJustHowMany(t *testing.T) {
	sampled := goldenResult(model.MethodSampled, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})
	confirmed := artifactByName(t, report.DiscoverArtifacts(sampled), report.FileConfirmed)

	if confirmed.Count != 0 {
		t.Fatalf("a sampled run cannot confirm; got %d confirmations", confirmed.Count)
	}
	if !strings.Contains(confirmed.Content, "could not have") {
		t.Errorf("the file must say the mode made confirmation impossible, not merely report zero:\n%s",
			confirmed.Content)
	}
}

// TestBrokenArtifactLooksBeforeItAlters pins the ordering that keeps someone
// from creating a constraint over rows they have not seen.
func TestBrokenArtifactLooksBeforeItAlters(t *testing.T) {
	broken := artifactByName(t,
		report.DiscoverArtifacts(goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})),
		report.FileBroken)

	query := strings.Index(broken.Content, "SELECT c.")
	ddl := strings.Index(broken.Content, "ADD CONSTRAINT")

	switch {
	case query < 0 || ddl < 0:
		t.Fatalf("both the orphan query and the DDL must be present:\n%s", broken.Content)
	case query > ddl:
		t.Errorf("the orphan query must precede the DDL:\n%s", broken.Content)
	}

	for _, line := range strings.Split(broken.Content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "ALTER TABLE") {
			t.Errorf("DDL for a broken relationship must stay commented out: %q", line)
		}
	}
}

// TestNoRowMutationIsEverSuggested is the read-only rule applied to what the
// tool hands the user. Cleaning up an orphan is a domain decision.
func TestNoRowMutationIsEverSuggested(t *testing.T) {
	artifacts := append(
		report.DiscoverArtifacts(goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})),
		report.AuditArtifacts(notValidResult())...)

	for _, a := range artifacts {
		upper := strings.ToUpper(a.Content)
		for _, forbidden := range []string{"DELETE FROM", "UPDATE ", "TRUNCATE", "DROP "} {
			if strings.Contains(upper, forbidden) {
				t.Errorf("%s suggests %q; the tool proposes additions only", a.Name, forbidden)
			}
		}
	}
}

// TestInconclusiveVerdictsProduceNoDDL keeps the artifacts from suggesting an
// action the evidence does not support.
func TestInconclusiveVerdictsProduceNoDDL(t *testing.T) {
	artifacts := report.DiscoverArtifacts(goldenResult(model.MethodFull,
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}))

	for _, a := range artifacts {
		for _, absent := range []string{"produto_id", "usuario_id"} {
			if strings.Contains(a.Content, absent) {
				t.Errorf("%s mentions %s, whose verdict was weak or unvalidated:\n%s", a.Name, absent, a.Content)
			}
		}
	}
}

// TestIndexSuggestionOnlyWhenMissing covers both halves: the trap of an
// unindexed child side, and the noise of suggesting an index that exists.
func TestIndexSuggestionOnlyWhenMissing(t *testing.T) {
	r := goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})
	confirmed := artifactByName(t, report.DiscoverArtifacts(r), report.FileConfirmed)

	if !strings.Contains(confirmed.Content, "CREATE INDEX CONCURRENTLY") {
		t.Fatalf("pedido.cliente_id has no leading index and must get the suggestion:\n%s", confirmed.Content)
	}
	if !strings.Contains(confirmed.Content, "does NOT run inside a transaction block") {
		t.Error("the CONCURRENTLY trap is not guessable from the line and must be written out")
	}
	if !strings.Contains(confirmed.Content, "INVALID index") {
		t.Error("a failed CONCURRENTLY leaves an invalid index behind; that must be stated")
	}

	// nota_fiscal.pedido_id leads a composite index, which is usable.
	r.Candidates = []model.Candidate{verdictCandidate("nota_fiscal", "pedido_id", "pedido",
		model.VerdictConfirmed, validated(model.MethodFull, 1000, 100, 0, 0), "")}

	indexed := artifactByName(t, report.DiscoverArtifacts(r), report.FileConfirmed)
	if strings.Contains(indexed.Content, "CREATE INDEX") {
		t.Errorf("a leading column of a composite index is already usable:\n%s", indexed.Content)
	}
}

// TestValidateIsSeparateAndCommented covers the two-step decision: running the
// whole file must create the constraint without triggering a full scan under
// the stronger lock.
func TestValidateIsSeparateAndCommented(t *testing.T) {
	confirmed := artifactByName(t,
		report.DiscoverArtifacts(goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})),
		report.FileConfirmed)

	if !strings.Contains(confirmed.Content, "NOT VALID;") {
		t.Fatalf("the executable DDL must be NOT VALID:\n%s", confirmed.Content)
	}
	for _, line := range strings.Split(confirmed.Content, "\n") {
		if strings.Contains(line, "VALIDATE CONSTRAINT") && !strings.HasPrefix(strings.TrimSpace(line), "--") {
			t.Errorf("VALIDATE must stay commented so running the file stays cheap: %q", line)
		}
	}
}

// TestGenerationIsDeterministic is what makes golden files and run-to-run
// comparison possible at all.
func TestGenerationIsDeterministic(t *testing.T) {
	r := goldenResult(model.MethodFull, model.Coverage{TablesTotal: 4, TablesAnalyzed: 4})

	first := report.DiscoverArtifacts(r)
	second := report.DiscoverArtifacts(r)

	for i := range first {
		if first[i].Content != second[i].Content {
			t.Errorf("%s differs between two generations of the same input", first[i].Name)
		}
	}
}

func TestWriteArtifactsCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "out")

	artifacts := report.DiscoverArtifacts(goldenResult(model.MethodFull,
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}))

	if err := report.WriteArtifacts(dir, artifacts); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	for _, a := range artifacts {
		got, err := os.ReadFile(filepath.Join(dir, a.Name))
		if err != nil {
			t.Fatalf("reading %s: %v", a.Name, err)
		}
		if string(got) != a.Content {
			t.Errorf("%s on disk differs from what was generated", a.Name)
		}
	}
}

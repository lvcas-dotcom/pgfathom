package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// TestManifestNamesEveryFileAndItsCount covers the one mode where stdout is
// neither a table nor a document: a successful run that printed nothing is
// indistinguishable from a failure.
func TestManifestNamesEveryFileAndItsCount(t *testing.T) {
	artifacts := report.DiscoverArtifacts(goldenResult(model.MethodFull,
		model.Coverage{TablesTotal: 4, TablesAnalyzed: 4}))

	var b bytes.Buffer
	if err := report.Manifest(&b, "findings", artifacts); err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	out := b.String()

	for _, a := range artifacts {
		if !strings.Contains(out, a.Name) {
			t.Errorf("the manifest must name %s:\n%s", a.Name, out)
		}
	}
	if !strings.Contains(out, "Review every file") {
		t.Errorf("the manifest must repeat that the files need reading:\n%s", out)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("columns must be padded, not tab-separated:\n%q", out)
	}
}

package report_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// constraintNamesIn pulls the generated names back out of the artifact, which
// is the only surface the package exposes them on.
func constraintNamesIn(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "--"))
		if !strings.HasPrefix(trimmed, "ADD CONSTRAINT ") {
			continue
		}
		out = append(out, strings.Trim(strings.TrimPrefix(trimmed, "ADD CONSTRAINT "), `"`))
	}
	return out
}

func confirmedFor(t *testing.T, candidates []model.Candidate, schemas []model.Schema) string {
	t.Helper()

	r := model.NewResult(goldenVersion, "pt-br", goldenTime(),
		model.Coverage{TablesTotal: len(candidates), TablesAnalyzed: len(candidates)})
	r.ServerVersion = goldenServer
	r.Schemas = schemas
	r.Candidates = candidates

	return artifactByName(t, report.DiscoverArtifacts(r), report.FileConfirmed).Content
}

func confirmedCandidate(table, column, parent string) model.Candidate {
	return verdictCandidate(table, column, parent, model.VerdictConfirmed,
		validated(model.MethodFull, 1000, 100, 0, 0), "")
}

// TestLongNamesAreTruncatedWithoutColliding covers the failure NAMEDATALEN
// causes silently: two distinct long names cut to the same 63 bytes make the
// second ADD CONSTRAINT fail as a duplicate.
func TestLongNamesAreTruncatedWithoutColliding(t *testing.T) {
	prefix := strings.Repeat("comprovante_de_recolhimento_", 3)

	content := confirmedFor(t, []model.Candidate{
		confirmedCandidate(prefix+"municipal", "referencia_id", "referencia"),
		confirmedCandidate(prefix+"estadual", "referencia_id", "referencia"),
	}, nil)

	names := constraintNamesIn(content)
	if len(names) != 2 {
		t.Fatalf("expected two constraint names, got %d:\n%s", len(names), content)
	}

	for _, n := range names {
		if len(n) > 63 {
			t.Errorf("name %q is %d bytes; the server truncates past 63 without saying so", n, len(n))
		}
	}
	if names[0] == names[1] {
		t.Errorf("two distinct relations produced the same name %q: the second ADD CONSTRAINT would fail", names[0])
	}
	if !strings.Contains(content, "shortened to fit") {
		t.Errorf("a shortened name must say so and carry the original:\n%s", content)
	}
}

// TestTruncationLandsOnARuneBoundary covers the other half: the budget is
// counted in bytes because that is how the server counts it, but cutting inside
// a UTF-8 sequence produces an identifier the server cannot parse.
func TestTruncationLandsOnARuneBoundary(t *testing.T) {
	// Accented characters are two bytes each, so the cut point falls inside a
	// sequence for at least one length in this range.
	for n := 20; n <= 40; n++ {
		content := confirmedFor(t, []model.Candidate{
			confirmedCandidate(strings.Repeat("çã", n), "referência_id", "referência"),
		}, nil)

		for _, name := range constraintNamesIn(content) {
			if !utf8.ValidString(name) {
				t.Fatalf("name cut mid-sequence at n=%d: %q", n, name)
			}
			if len(name) > 63 {
				t.Fatalf("name at n=%d is %d bytes, over the server budget", n, len(name))
			}
		}
	}
}

// TestExoticNamesAreQuoted is the case unquoted SQL breaks on, and the reason
// every emitted identifier goes through the sanitizer.
func TestExoticNamesAreQuoted(t *testing.T) {
	content := confirmedFor(t, []model.Candidate{
		confirmedCandidate("Pedido De Compra", "Cliente ID", "Cliente"),
	}, nil)

	for _, want := range []string{`"public"."Pedido De Compra"`, `"Cliente ID"`, `"public"."Cliente"`} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %s to be quoted in:\n%s", want, content)
		}
	}
}

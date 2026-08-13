package infer_test

import (
	"math"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
)

func TestTrigramSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want float64
	}{
		{"identical strings score 1", "cliente", "cliente", 1.0},
		{"no shared trigram scores 0", "abc", "xyz", 0.0},
		{"empty left side scores 0", "", "cliente", 0.0},
		{"empty right side scores 0", "cliente", "", 0.0},
		{"both empty scores 0", "", "", 0.0},
		{"case is ignored", "Cliente", "CLIENTE", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infer.TrigramSimilarity(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("TrigramSimilarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestTrigramSimilarityCorpusExamples pins the metric against the real misses
// documented in docs/PGFATHOM.md — abbreviation and reordering that no
// affix/plural match reaches. Bounds, not exact literals: the value is
// computed by the function itself, and the test documents why that range is
// expected rather than asserting a hand-guessed float.
func TestTrigramSimilarityCorpusExamples(t *testing.T) {
	tests := []struct {
		name        string
		a, b        string
		wantAtLeast float64
	}{
		// idkey_operador -> entity "operador", stripped by the pt-br profile,
		// is a literal prefix of the target table name: high overlap expected.
		{"operador is a prefix of operadorbasecalculo", "operador", "operadorbasecalculo", 0.45},
		// atorevogacao_idkey -> entity "atorevogacao" shares its first three
		// letters with the target "ato" and nothing else: overlap is real but
		// modest, consistent with a short target name diluting the coefficient.
		{"atorevogacao shares a prefix with ato", "atorevogacao", "ato", 0.20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infer.TrigramSimilarity(tt.a, tt.b)
			if got < tt.wantAtLeast || got > 1.0 {
				t.Errorf("TrigramSimilarity(%q, %q) = %v, want >= %v", tt.a, tt.b, got, tt.wantAtLeast)
			}
			t.Logf("TrigramSimilarity(%q, %q) = %v", tt.a, tt.b, got)
		})
	}

	// tptramite_idkey -> entity "tptramite" against the target "tramitetipo":
	// abbreviated and reordered, the case this metric exists for. Asserted as
	// a range, not a floor: the reordering costs enough shared trigrams that
	// this is the borderline case the default cutoff (0.65) is calibrated
	// against, not a clean high match.
	got := infer.TrigramSimilarity("tptramite", "tramitetipo")
	if got <= 0 || got >= 1 {
		t.Errorf("TrigramSimilarity(tptramite, tramitetipo) = %v, want strictly between 0 and 1", got)
	}
	if math.IsNaN(got) {
		t.Fatal("TrigramSimilarity must never return NaN")
	}
	t.Logf("TrigramSimilarity(tptramite, tramitetipo) = %v", got)
}

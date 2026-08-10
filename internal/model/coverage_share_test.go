package model_test

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestAnalyzedShare(t *testing.T) {
	tests := []struct {
		name            string
		total, analyzed int
		wantShare       float64
		wantIncomplete  bool
	}{
		{"everything analyzed", 100, 100, 1.0, false},
		{"just above the line", 100, 80, 0.80, false},
		{"just below the line", 100, 79, 0.79, true},
		{"the municipal case", 338, 247, 0.7307692307692307, true},
		{"empty scope", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Coverage{TablesTotal: tt.total, TablesAnalyzed: tt.analyzed}

			if got := c.AnalyzedShare(); got != tt.wantShare {
				t.Errorf("AnalyzedShare() = %v, want %v", got, tt.wantShare)
			}
			if got := c.MateriallyIncomplete(); got != tt.wantIncomplete {
				t.Errorf("MateriallyIncomplete() = %v, want %v", got, tt.wantIncomplete)
			}
		})
	}
}

// TestEmptyScopeIsNotCalledIncomplete keeps the warning meaningful: a run over
// nothing has no fraction to report, and claiming one would put a number on an
// absence.
func TestEmptyScopeIsNotCalledIncomplete(t *testing.T) {
	if (model.Coverage{}).MateriallyIncomplete() {
		t.Error("a scope of zero tables must not be reported as materially incomplete")
	}
}

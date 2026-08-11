//go:build benchmark

package bench_test

import (
	"os"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/bench"
)

// TestManifestIsWellFormed needs neither Docker nor network, so a typo in the
// corpus recipe fails in a second instead of thirty minutes into a benchmark
// run, after images have been pulled and a thousand-table schema loaded.
func TestManifestIsWellFormed(t *testing.T) {
	m, err := bench.LoadManifest()
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	seen := make(map[string]bool)
	for _, s := range m.Schemas {
		if seen[s.Name] {
			t.Errorf("%s appears twice; the report would carry two sections of the same name", s.Name)
		}
		seen[s.Name] = true

		if s.Kind == bench.FromSQL && len(s.SHA256) != 64 {
			t.Errorf("%s: sha256 %q is not a sha256", s.Name, s.SHA256)
		}
		if s.Note == "" {
			t.Errorf("%s: a corpus entry is published with its note; an unexplained row invites the wrong reading", s.Name)
		}
	}
}

// TestCacheIsNotVersioned guards the decision that the repository holds the
// recipe and never the dumps. A corpus checked in by accident would be several
// megabytes in a repository whose selling point is being auditable in one
// sitting.
func TestCacheIsNotVersioned(t *testing.T) {
	m, err := bench.LoadManifest()
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	ignored, err := os.ReadFile(".gitignore")
	if err != nil {
		// The .gitignore lives at the repository root, not beside this package.
		ignored, err = os.ReadFile("../../.gitignore")
		if err != nil {
			t.Fatalf("reading .gitignore: %v", err)
		}
	}

	if !strings.Contains(string(ignored), "/bench/cache/") {
		t.Error(".gitignore must cover the corpus cache")
	}
	for _, s := range m.Schemas {
		if s.Kind == bench.FromLocal && !strings.Contains(string(ignored), "/bench/local/") {
			t.Error(".gitignore must cover local dumps: they are real data")
		}
	}
}

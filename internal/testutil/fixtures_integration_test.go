//go:build integration

package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// knownFixtures is every fixture the integration suites ask for by name.
//
// The list is duplicated from the callers on purpose: it is what turns a typo
// or a moved file into a failure here, in a test that needs no container, one
// second into the run — instead of thirty seconds in, spread across four
// packages, after Docker has already started pulling images.
//
// A fixture added without its line here fails TestNoOrphanFixtures, which is
// the direction that costs nothing: it says so instead of going quiet. The
// list of values those fixtures plant lives in PlantedValues and has the
// opposite failure mode, which is why that one is not duplicated at all.
var knownFixtures = []string{
	"clean_schema",
	"inferable",
	"no_constraints",
	"not_valid_constraints",
	"restricted_privileges",
	"stats_prefilter",
	"unindexed_fks",
	"unsupported_shapes",
	"usage_evidence",
	"validation",
}

func TestEveryKnownFixtureResolves(t *testing.T) {
	for _, name := range knownFixtures {
		t.Run(name, func(t *testing.T) {
			path := fixturePath(t, name)

			if !filepath.IsAbs(path) {
				t.Errorf("path %q is relative; it must not depend on the caller's working directory", path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if !strings.Contains(strings.ToUpper(string(content)), "CREATE TABLE") {
				t.Errorf("fixture %s creates no table; an empty schema makes a test pass by finding nothing", name)
			}
		})
	}
}

// TestNoOrphanFixtures catches the opposite drift: a file left behind after the
// test that used it was deleted or renamed.
func TestNoOrphanFixtures(t *testing.T) {
	known := make(map[string]bool, len(knownFixtures))
	for _, n := range knownFixtures {
		known[n] = true
	}

	dir := filepath.Dir(fixturePath(t, knownFixtures[0]))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading fixture directory: %v", err)
	}

	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".sql")
		if !known[name] {
			t.Errorf("fixture %s is not referenced by any suite", e.Name())
		}
	}
}

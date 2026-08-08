package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// userRelation matches a FROM or JOIN against anything outside the system
// catalogs. It is deliberately blunt: the point is to fail loudly when a layer
// that must not read table data grows a query that does, and a blunt matcher
// that occasionally names a false relation is cheaper than a precise one that
// misses the real case.
var userRelation = regexp.MustCompile(`(?is)\b(from|join)\s+(?:only\s+)?([a-z_][a-z0-9_.]*)`)

// AssertCatalogOnly fails when a SQL constant in the given source file reads
// any relation outside allowed. Callers pass the relations their layer is
// entitled to, and reason says what the layer is not allowed to touch — it
// lands in the failure message, where it is worth more than in a comment.
//
// The check exists per layer rather than once for the whole project because
// each layer's entitlement differs: the catalog reads system catalogs, the
// prefilter reads planner statistics, and only validation may read a user
// table at all.
func AssertCatalogOnly(t *testing.T, path string, allowed map[string]bool, reason string) {
	t.Helper()

	for name, sql := range SQLConstants(t, path) {
		for _, m := range userRelation.FindAllStringSubmatch(sql, -1) {
			relation := strings.ToLower(m[2])
			if allowed[relation] {
				continue
			}
			t.Errorf("%s reads %q: %s", name, relation, reason)
		}
	}
}

// SQLConstants returns the SQL string constants of a Go source file, by name.
// Scanning the raw file would match the surrounding prose instead of the
// queries, so the source is parsed and only string literals bound to a name
// are considered.
func SQLConstants(t *testing.T, path string) map[string]string {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	out := make(map[string]string)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if i >= len(spec.Values) {
				continue
			}
			lit, ok := spec.Values[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("unquoting %s: %v", name.Name, err)
			}
			if !strings.Contains(strings.ToUpper(value), "SELECT") {
				continue
			}
			out[name.Name] = value
		}
		return true
	})

	if len(out) == 0 {
		t.Fatalf("no SQL constants found in %s; the check would pass vacuously", path)
	}
	return out
}

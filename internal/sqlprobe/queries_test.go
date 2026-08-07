package sqlprobe

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// userRelation matches a FROM or JOIN against anything outside the system
// catalogs, as in the catalog and stats layers. Mining SQL is pure catalog
// work, and the day it stops being that has to fail loudly.
var userRelation = regexp.MustCompile(`(?is)\b(from|join)\s+(?:only\s+)?([a-z_][a-z0-9_.]*)`)

var allowedRelations = map[string]bool{
	"pg_views": true, "pg_proc": true, "pg_namespace": true, "pg_language": true,
	"pg_extension": true, "pg_stat_statements": true,
}

func TestQueriesTouchOnlyTheCatalog(t *testing.T) {
	for name, sql := range queryLiterals(t, "probe.go") {
		for _, m := range userRelation.FindAllStringSubmatch(sql, -1) {
			relation := strings.ToLower(m[2])
			if allowedRelations[relation] {
				continue
			}
			t.Errorf("%s reads %q: mining usage evidence must touch only the catalog", name, relation)
		}
	}
}

func queryLiterals(t *testing.T, path string) map[string]string {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	file, err := parser.ParseFile(gotoken.NewFileSet(), path, src, 0)
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
			if !ok || lit.Kind != gotoken.STRING {
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

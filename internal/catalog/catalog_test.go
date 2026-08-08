package catalog

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name          string
		patterns      []string
		schema, table string
		want          bool
	}{
		{"no patterns", nil, "public", "pedido", false},
		{"bare name", []string{"pedido"}, "public", "pedido", true},
		{"glob on bare name", []string{"tmp_*"}, "public", "tmp_import", true},
		{"qualified name", []string{"public.pedido"}, "public", "pedido", true},
		{"glob on qualified name", []string{"audit.*"}, "audit", "log_2019", true},
		{"non-matching schema", []string{"audit.*"}, "public", "log_2019", false},
		{"no match", []string{"tmp_*"}, "public", "pedido", false},
		{"malformed pattern is ignored", []string{"["}, "public", "pedido", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAny(tt.patterns, tt.schema, tt.table); got != tt.want {
				t.Errorf("matchesAny(%v, %q, %q) = %v, want %v",
					tt.patterns, tt.schema, tt.table, got, tt.want)
			}
		})
	}
}

func TestUnsupportedReason(t *testing.T) {
	tests := []struct {
		name  string
		table model.Table
		want  model.UnsupportedReason
	}{
		{"single-column key is supported",
			model.Table{PrimaryKey: []string{"id"}}, ""},
		{"composite key",
			model.Table{PrimaryKey: []string{"a", "b"}}, model.ReasonCompositePK},
		{"no key",
			model.Table{}, model.ReasonNoPrimaryKey},
		{"partitioned outranks the key check",
			model.Table{Partitioned: true, PrimaryKey: []string{"id"}}, model.ReasonPartitioned},
		{"inheritance outranks the key check",
			model.Table{Inherits: true, PrimaryKey: []string{"id"}}, model.ReasonInheritance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tt.table
			if got := unsupportedReason(&table); got != tt.want {
				t.Errorf("unsupportedReason = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyUnsupportedKeepsCoverageBalanced covers the invariant the report
// depends on: analyzed plus skipped has to add up to the scope, or the coverage
// block is lying.
func TestClassifyUnsupportedKeepsCoverageBalanced(t *testing.T) {
	tables := map[tableKey]*model.Table{
		key("public", "ok"):          {Schema: "public", Name: "ok", PrimaryKey: []string{"id"}},
		key("public", "composite"):   {Schema: "public", Name: "composite", PrimaryKey: []string{"a", "b"}},
		key("public", "keyless"):     {Schema: "public", Name: "keyless"},
		key("public", "partitioned"): {Schema: "public", Name: "partitioned", Partitioned: true},
	}

	coverage := model.Coverage{TablesTotal: len(tables)}
	classifyUnsupported(tables, &coverage)

	if coverage.TablesAnalyzed != 1 {
		t.Errorf("analyzed = %d, want 1", coverage.TablesAnalyzed)
	}
	if got := coverage.TablesAnalyzed + coverage.SkippedCount(); got != coverage.TablesTotal {
		t.Errorf("analyzed + skipped = %d, want %d", got, coverage.TablesTotal)
	}
}

func TestClassifyUnsupportedIsOrderStable(t *testing.T) {
	build := func() map[tableKey]*model.Table {
		return map[tableKey]*model.Table{
			key("public", "zeta"):  {Schema: "public", Name: "zeta"},
			key("public", "alpha"): {Schema: "public", Name: "alpha"},
			key("public", "mid"):   {Schema: "public", Name: "mid"},
		}
	}

	var first []model.SkippedTable
	for i := 0; i < 20; i++ {
		coverage := model.Coverage{}
		classifyUnsupported(build(), &coverage)

		if first == nil {
			first = coverage.TablesUnsupported
			continue
		}
		for j := range first {
			if first[j] != coverage.TablesUnsupported[j] {
				t.Fatalf("ordering changed between runs: %v vs %v", first, coverage.TablesUnsupported)
			}
		}
	}
}

// TestLinkForeignKeyIndexesRequiresLeadingPosition is the case that decides
// whether the tool reports all clear on a table where the problem is real. A
// composite index holding the child column anywhere but first is useless for
// the lookup a parent DELETE triggers.
func TestLinkForeignKeyIndexesRequiresLeadingPosition(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		want    bool
	}{
		{"dedicated index", []string{"cliente_id"}, true},
		{"leading in a composite index", []string{"cliente_id", "criado_em"}, true},
		{"not leading", []string{"criado_em", "cliente_id"}, false},
		{"absent", []string{"criado_em"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables := map[tableKey]*model.Table{
				key("public", "pedido"): {
					Schema:      "public",
					Name:        "pedido",
					Indexes:     []model.Index{{Name: "ix", Columns: tt.columns}},
					ForeignKeys: []model.ForeignKey{{Name: "fk", Columns: []string{"cliente_id"}}},
				},
			}

			linkForeignKeyIndexes(tables)

			if got := tables[key("public", "pedido")].ForeignKeys[0].HasIndex; got != tt.want {
				t.Errorf("HasIndex = %v, want %v for index on %v", got, tt.want, tt.columns)
			}
		})
	}
}

func TestSortedSchemas(t *testing.T) {
	got := SortedSchemas([]string{"public", "  audit  ", "", "public", "zeta"})
	want := []string{"audit", "public", "zeta"}

	if len(got) != len(want) {
		t.Fatalf("SortedSchemas = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedSchemas = %v, want %v", got, want)
		}
	}
}

func TestDefaultScopeIsPublic(t *testing.T) {
	if got := (Options{}).schemas(); len(got) != 1 || got[0] != "public" {
		t.Errorf("default schemas = %v, want [public]", got)
	}
}

func TestResolveScope(t *testing.T) {
	visible := []string{"auditoria_2019", "auditoria_2020", "public", "vendas"}

	tests := []struct {
		name    string
		visible []string
		opts    ScopeOptions
		want    Scope
	}{
		{
			name:    "no option analyzes public and declares what it left out",
			visible: visible,
			want: Scope{
				Schemas:     []string{"public"},
				NotAnalyzed: []string{"auditoria_2019", "auditoria_2020", "vendas"},
				Total:       4,
			},
		},
		{
			name:    "explicit list is normalized and sorted",
			visible: visible,
			opts:    ScopeOptions{Schemas: []string{"vendas", "  public  ", "vendas"}},
			want: Scope{
				Schemas:     []string{"public", "vendas"},
				NotAnalyzed: []string{"auditoria_2019", "auditoria_2020"},
				Total:       4,
			},
		},
		{
			name:    "every schema in scope leaves nothing unanalyzed",
			visible: visible,
			opts:    ScopeOptions{All: true},
			want:    Scope{Schemas: visible, Total: 4},
		},
		{
			name:    "exclusion by glob",
			visible: visible,
			opts:    ScopeOptions{All: true, ExcludeSchemas: []string{"auditoria_*"}},
			want: Scope{
				Schemas:  []string{"public", "vendas"},
				Excluded: []string{"auditoria_2019", "auditoria_2020"},
				Total:    4,
			},
		},
		{
			// Asked to be skipped and never asked for are different facts, and a
			// schema that lands in one must not also appear in the other.
			name:    "an excluded schema is not also reported as unasked",
			visible: visible,
			opts: ScopeOptions{
				Schemas:        []string{"public", "vendas"},
				ExcludeSchemas: []string{"vendas"},
			},
			want: Scope{
				Schemas:     []string{"public"},
				Excluded:    []string{"vendas"},
				NotAnalyzed: []string{"auditoria_2019", "auditoria_2020"},
				Total:       4,
			},
		},
		{
			name:    "blank entries fall back to the default scope",
			visible: []string{"public"},
			opts:    ScopeOptions{Schemas: []string{"", "   "}},
			want:    Scope{Schemas: []string{"public"}, Total: 1},
		},
		{
			// A misspelled schema stays in scope and produces an empty read. The
			// visible one it displaced is still reported as unanalyzed, which is
			// what makes the mistake legible in the coverage block.
			name:    "a requested schema outside the visible set is kept",
			visible: []string{"public"},
			opts:    ScopeOptions{Schemas: []string{"arquivo"}},
			want: Scope{
				Schemas:     []string{"arquivo"},
				NotAnalyzed: []string{"public"},
				Total:       1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveScope(tt.visible, tt.opts)
			if err != nil {
				t.Fatalf("resolveScope: %v", err)
			}
			if diff := cmp.Diff(&tt.want, got); diff != "" {
				t.Errorf("scope mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestResolveScopeRejectsEmptyScope covers the two ways the scope can end up
// empty. Running over nothing would produce a report about nothing, which reads
// exactly like a database with no findings — and the message has to say which of
// the two emptinesses happened.
func TestResolveScopeRejectsEmptyScope(t *testing.T) {
	tests := []struct {
		name    string
		visible []string
		opts    ScopeOptions
		want    string
	}{
		{
			name:    "everything removed by an exclusion pattern",
			visible: []string{"public", "vendas"},
			opts:    ScopeOptions{All: true, ExcludeSchemas: []string{"*"}},
			want:    "exclusion pattern",
		},
		{
			name:    "the role can open no schema at all",
			visible: nil,
			opts:    ScopeOptions{All: true},
			want:    "visible to the connected role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveScope(tt.visible, tt.opts)
			if !errors.Is(err, ErrEmptyScope) {
				t.Fatalf("error = %v, want it to wrap ErrEmptyScope", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("message %q does not explain the cause (%q)", err, tt.want)
			}
		})
	}
}

func TestResolvedScopeSupersedesTheSchemaList(t *testing.T) {
	opts := Options{
		Schemas: []string{"ignored"},
		Scope:   &Scope{Schemas: []string{"vendas"}},
	}
	if got := opts.schemas(); len(got) != 1 || got[0] != "vendas" {
		t.Errorf("schemas() = %v, want [vendas]", got)
	}
}

func TestScopeStampsCoverage(t *testing.T) {
	scope := Scope{
		Schemas:     []string{"public"},
		Excluded:    []string{"auditoria"},
		NotAnalyzed: []string{"vendas"},
		Total:       3,
	}

	var coverage model.Coverage
	scope.stamp(&coverage)

	want := model.Coverage{
		SchemasTotal:       3,
		SchemasAnalyzed:    1,
		SchemasNotAnalyzed: []string{"vendas"},
		SchemasExcluded:    []string{"auditoria"},
	}
	if diff := cmp.Diff(want, coverage); diff != "" {
		t.Errorf("coverage mismatch (-want +got):\n%s", diff)
	}
}

// TestCoverageIsIncompleteWithASchemaLeftOut pins the consequence of the schema
// dimension: the only run entitled to claim it looked at everything is the one
// that did.
func TestCoverageIsIncompleteWithASchemaLeftOut(t *testing.T) {
	complete := model.Coverage{TablesTotal: 3, TablesAnalyzed: 3}
	if !complete.Complete() {
		t.Fatal("a run with every table analyzed and no schema left out must be complete")
	}

	complete.SchemasNotAnalyzed = []string{"vendas"}
	if complete.Complete() {
		t.Error("a run with a schema left out must not report complete coverage")
	}
}

// userRelation matches a FROM or JOIN against anything outside the system
// catalogs. It is deliberately blunt: the point is to fail loudly if this layer
// ever grows a read of user data, which is a boundary the whole no-leak claim
// for this phase rests on.
var userRelation = regexp.MustCompile(`(?is)\b(from|join)\s+(?:only\s+)?([a-z_][a-z0-9_.]*)`)

var allowedRelations = map[string]bool{
	"pg_class": true, "pg_namespace": true, "pg_attribute": true, "pg_type": true,
	"pg_attrdef": true, "pg_constraint": true, "pg_index": true, "pg_inherits": true,
	"pg_stat_user_tables": true, "pg_stat_database": true,
	"unnest": true, "lateral": true,
}

func TestQueriesTouchOnlyTheCatalog(t *testing.T) {
	for name, sql := range queryLiterals(t, "queries.go") {
		for _, m := range userRelation.FindAllStringSubmatch(sql, -1) {
			relation := strings.ToLower(m[2])
			if allowedRelations[relation] {
				continue
			}
			t.Errorf("%s reads %q: the catalog layer must touch only system catalogs "+
				"and statistics views, because reading data starts in a later phase",
				name, relation)
		}
	}
}

// queryLiterals returns the SQL string constants, by name. Scanning the raw
// file would match the surrounding prose instead of the queries.
func queryLiterals(t *testing.T, path string) map[string]string {
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
			out[name.Name] = value
		}
		return true
	})

	if len(out) == 0 {
		t.Fatalf("no SQL constants found in %s; the check would pass vacuously", path)
	}
	return out
}

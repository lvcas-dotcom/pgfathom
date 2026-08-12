package audit_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/audit"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// forbiddenForAudit are packages that could read table data or open a
// transaction. internal/validate is where the one probe that reads data
// lives, on purpose, in a different package: audit stays provably free of
// that capability.
var forbiddenForAudit = []string{
	"github.com/lvcas-dotcom/pgfathom/internal/validate",
	"github.com/lvcas-dotcom/pgfathom/internal/db",
	"github.com/jackc/pgx/v5",
}

// TestPackageNeverReadsData enforces that internal/audit cannot open a
// transaction or read a row. It is the same shape as internal/model's purity
// test, and exists for the same reason: the property is easy to break with
// one convenient import and expensive to notice afterward without a test.
func TestPackageNeverReadsData(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0

	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		checked++

		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		file, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		for _, imp := range file.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquoting import %s: %v", path, imp.Path.Value, err)
			}
			for _, forbidden := range forbiddenForAudit {
				if p == forbidden {
					t.Errorf("%s imports %q: internal/audit must never read table data", path, p)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no non-test source files found; the purity check would pass vacuously")
	}
}

func schemaWith(tables ...model.Table) []model.Schema {
	return []model.Schema{{Name: "public", Tables: tables}}
}

func table(name string, fks ...model.ForeignKey) model.Table {
	return model.Table{
		Schema:      "public",
		Name:        name,
		PrimaryKey:  []string{"id"},
		ForeignKeys: fks,
		Stats:       model.TableStats{EstimatedRows: 4_000_000},
	}
}

func fk(name, column, refTable string, validated, hasIndex bool) model.ForeignKey {
	return model.ForeignKey{
		Name:       name,
		Columns:    []string{column},
		RefSchema:  "public",
		RefTable:   refTable,
		RefColumns: []string{"id"},
		Validated:  validated,
		HasIndex:   hasIndex,
	}
}

func kinds(findings []model.Finding) []model.FindingKind {
	out := make([]model.FindingKind, len(findings))
	for i, f := range findings {
		out[i] = f.Kind
	}
	return out
}

func countKind(findings []model.Finding, k model.FindingKind) int {
	n := 0
	for _, f := range findings {
		if f.Kind == k {
			n++
		}
	}
	return n
}

func TestNotValidConstraintIsReported(t *testing.T) {
	schemas := schemaWith(table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", false, true)))

	findings := audit.Findings(schemas, audit.Options{})

	if got := countKind(findings, model.FindingNotValidConstraint); got != 1 {
		t.Fatalf("got %d NOT VALID findings, want 1 (kinds: %v)", got, kinds(findings))
	}

	f := findings[0]
	if !strings.Contains(f.Object, "pedido_cliente_fk") {
		t.Errorf("object = %q, want it to name the constraint", f.Object)
	}
	if f.Metrics["child_estimated_rows"] != 4_000_000 {
		t.Errorf("metrics = %v, want the child row estimate", f.Metrics)
	}
}

func TestValidatedConstraintIsQuiet(t *testing.T) {
	schemas := schemaWith(table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", true, true)))

	if got := countKind(audit.Findings(schemas, audit.Options{}), model.FindingNotValidConstraint); got != 0 {
		t.Errorf("got %d NOT VALID findings on a fully validated schema, want 0", got)
	}
}

func TestUnindexedForeignKeyIsReported(t *testing.T) {
	parent := table("cliente")
	parent.Stats.EstimatedRows = 90_000

	schemas := schemaWith(
		table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", true, false)),
		parent,
	)

	findings := audit.Findings(schemas, audit.Options{})

	if got := countKind(findings, model.FindingFKWithoutIndex); got != 1 {
		t.Fatalf("got %d unindexed findings, want 1 (kinds: %v)", got, kinds(findings))
	}

	f := findings[0]
	if f.Metrics["child_estimated_rows"] != 4_000_000 {
		t.Errorf("metrics = %v, want the child row estimate", f.Metrics)
	}
	if f.Metrics["parent_estimated_rows"] != 90_000 {
		t.Errorf("metrics = %v, want the parent row estimate, which is what shows the severity", f.Metrics)
	}
	if !strings.Contains(f.Detail, "cliente_id") {
		t.Errorf("detail = %q, want it to name the child column", f.Detail)
	}
}

func TestIndexedForeignKeyIsQuiet(t *testing.T) {
	schemas := schemaWith(table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", true, true)))

	if got := countKind(audit.Findings(schemas, audit.Options{}), model.FindingFKWithoutIndex); got != 0 {
		t.Errorf("got %d unindexed findings on an indexed schema, want 0", got)
	}
}

func TestOneConstraintCanProduceBothFindings(t *testing.T) {
	schemas := schemaWith(table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", false, false)))

	findings := audit.Findings(schemas, audit.Options{})

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (kinds: %v)", len(findings), kinds(findings))
	}
	if countKind(findings, model.FindingNotValidConstraint) != 1 ||
		countKind(findings, model.FindingFKWithoutIndex) != 1 {
		t.Errorf("kinds = %v, want one of each", kinds(findings))
	}
}

// TestFindingsAreOrderStable matters because the output is compared against
// golden files from the next phase onward, and map iteration would otherwise
// shuffle it between runs.
func TestFindingsAreOrderStable(t *testing.T) {
	schemas := schemaWith(
		table("zeta", fk("zeta_fk", "a_id", "alpha", false, false)),
		table("alpha", fk("alpha_fk", "b_id", "beta", false, false)),
	)

	first := audit.Findings(schemas, audit.Options{})
	for i := 0; i < 20; i++ {
		again := audit.Findings(schemas, audit.Options{})
		for j := range first {
			if first[j].Object != again[j].Object || first[j].Kind != again[j].Kind {
				t.Fatalf("ordering changed between runs at %d: %v vs %v", j, first[j], again[j])
			}
		}
	}
}

func TestEmptySchemaProducesNothing(t *testing.T) {
	if findings := audit.Findings(nil, audit.Options{}); len(findings) != 0 {
		t.Errorf("got %d findings from no schemas, want 0", len(findings))
	}
	if findings := audit.Findings(schemaWith(), audit.Options{}); len(findings) != 0 {
		t.Errorf("got %d findings from an empty schema, want 0", len(findings))
	}
}

// tableNoPK builds a table without a primary key, with columns and optional
// uniques and indexes — the shape the missing-key and hot-column generators
// need to inspect.
func tableNoPK(name string, columns []model.Column, uniques []model.UniqueConstraint, indexes []model.Index) model.Table {
	return model.Table{
		Schema:  "public",
		Name:    name,
		Columns: columns,
		Uniques: uniques,
		Indexes: indexes,
		Stats:   model.TableStats{EstimatedRows: 1_500_000},
	}
}

func col(name string, nullable bool) model.Column {
	return model.Column{Name: name, Type: "bigint", BaseType: "int8", Nullable: nullable}
}

func TestMissingPrimaryKeyWithPromotableUniqueOffersPromotion(t *testing.T) {
	t.Parallel()
	tbl := tableNoPK("cadastro",
		[]model.Column{col("id", false), col("cpf", false)},
		[]model.UniqueConstraint{{Name: "cadastro_cpf_key", Columns: []string{"cpf"}}}, nil)

	findings := audit.Findings(schemaWith(tbl), audit.Options{})

	if got := countKind(findings, model.FindingMissingPrimaryKey); got != 1 {
		t.Fatalf("got %d missing-PK findings, want 1 (kinds: %v)", got, kinds(findings))
	}

	s := findings[0].Suggestion
	if s == nil || s.Kind != model.SuggestPromoteUnique || len(s.Columns) != 1 || s.Columns[0] != "cpf" {
		t.Errorf("suggestion = %+v, want promote_unique over [cpf]", s)
	}
}

func TestMissingPrimaryKeyWithNullableUniqueIsNotPromotable(t *testing.T) {
	t.Parallel()
	tbl := tableNoPK("cadastro",
		[]model.Column{col("id", false), col("cpf", true)},
		[]model.UniqueConstraint{{Name: "cadastro_cpf_key", Columns: []string{"cpf"}}}, nil)

	findings := audit.Findings(schemaWith(tbl), audit.Options{})

	s := findings[0].Suggestion
	if s == nil || s.Kind != model.SuggestCreatePrimaryKey || len(s.Columns) != 0 {
		t.Errorf("suggestion = %+v, want create_primary_key with no columns: a nullable unique cannot be promoted", s)
	}
}

func TestMissingPrimaryKeyWithoutAnyUniqueNeedsProbing(t *testing.T) {
	t.Parallel()
	tbl := tableNoPK("cadastro", []model.Column{col("id", false)}, nil, nil)

	findings := audit.Findings(schemaWith(tbl), audit.Options{})

	if got := countKind(findings, model.FindingMissingPrimaryKey); got != 1 {
		t.Fatalf("got %d missing-PK findings, want 1", got)
	}
	s := findings[0].Suggestion
	if s == nil || s.Kind != model.SuggestCreatePrimaryKey || s.KeyProbe != "" {
		t.Errorf("suggestion = %+v, want create_primary_key with no probe verdict yet: "+
			"naming a candidate key needs data, which this package never reads", s)
	}
}

func TestTableWithPrimaryKeyProducesNoMissingKeyFinding(t *testing.T) {
	t.Parallel()
	schemas := schemaWith(table("pedido"))

	if got := countKind(audit.Findings(schemas, audit.Options{}), model.FindingMissingPrimaryKey); got != 0 {
		t.Errorf("got %d missing-PK findings on a table that has one, want 0", got)
	}
}

func ref(table, column string) model.ColumnRef {
	return model.ColumnRef{Schema: "public", Table: table, Column: column}
}

func TestHotColumnFromRepeatedJoinsIsReported(t *testing.T) {
	t.Parallel()
	tbl := table("pedido")
	tbl.Columns = []model.Column{col("id", false), col("cliente_id", false)}
	schemas := schemaWith(tbl, table("cliente"))

	joins := []model.JoinEvidence{
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromView, Object: "vw_a"},
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromFunction, Object: "fn_b"},
	}

	findings := audit.Findings(schemas, audit.Options{Joins: joins, RecurrenceMin: 2})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 1 {
		t.Fatalf("got %d hot-column findings, want 1 (kinds: %v)", got, kinds(findings))
	}
}

func TestHotColumnBelowRecurrenceThresholdIsQuiet(t *testing.T) {
	t.Parallel()
	tbl := table("pedido")
	tbl.Columns = []model.Column{col("id", false), col("cliente_id", false)}
	schemas := schemaWith(tbl, table("cliente"))

	joins := []model.JoinEvidence{
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromView, Object: "vw_a"},
	}

	findings := audit.Findings(schemas, audit.Options{Joins: joins, RecurrenceMin: 2})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 0 {
		t.Errorf("got %d hot-column findings below the recurrence threshold, want 0", got)
	}
}

func TestHotColumnAlreadyIndexedIsQuiet(t *testing.T) {
	t.Parallel()
	tbl := table("pedido")
	tbl.Columns = []model.Column{col("id", false), col("cliente_id", false)}
	tbl.Indexes = []model.Index{{Name: "ix_pedido_cliente", Columns: []string{"cliente_id"}}}
	schemas := schemaWith(tbl, table("cliente"))

	joins := []model.JoinEvidence{
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromView, Object: "vw_a"},
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromFunction, Object: "fn_b"},
	}

	findings := audit.Findings(schemas, audit.Options{Joins: joins, RecurrenceMin: 2})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 0 {
		t.Errorf("got %d hot-column findings on an already-indexed column, want 0", got)
	}
}

// TestHotColumnCoveredByFKFindingIsNotDuplicated pins the rule that an
// unindexed FK child column is reported once, as fk_without_index, never
// twice under a second finding kind for the same problem.
func TestHotColumnCoveredByFKFindingIsNotDuplicated(t *testing.T) {
	t.Parallel()
	tbl := table("pedido", fk("pedido_cliente_fk", "cliente_id", "cliente", true, false))
	tbl.Columns = []model.Column{col("id", false), col("cliente_id", false)}
	schemas := schemaWith(tbl, table("cliente"))

	joins := []model.JoinEvidence{
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromView, Object: "vw_a"},
		{Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"), Source: model.JoinFromFunction, Object: "fn_b"},
	}

	findings := audit.Findings(schemas, audit.Options{Joins: joins, RecurrenceMin: 2})

	if got := countKind(findings, model.FindingFKWithoutIndex); got != 1 {
		t.Errorf("got %d fk_without_index findings, want 1", got)
	}
	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 0 {
		t.Errorf("got %d unindexed_hot_column findings, want 0: already covered by fk_without_index", got)
	}
}

func TestHotColumnIndexMethodFollowsContainmentPredicate(t *testing.T) {
	t.Parallel()
	tbl := table("evento")
	tbl.Columns = []model.Column{col("id", false), {Name: "dados", Type: "jsonb", BaseType: "jsonb"}}
	schemas := schemaWith(tbl)

	preds := []model.PredicateEvidence{
		{Column: ref("evento", "dados"), Operator: model.OpContainment, Source: model.JoinFromFunction, Object: "fn_a"},
		{Column: ref("evento", "dados"), Operator: model.OpContainment, Source: model.JoinFromView, Object: "vw_b"},
	}

	findings := audit.Findings(schemas, audit.Options{Predicates: preds, RecurrenceMin: 2})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 1 {
		t.Fatalf("got %d hot-column findings, want 1 (kinds: %v)", got, kinds(findings))
	}
	s := findings[0].Suggestion
	if s == nil || s.IndexMethod != "gin" {
		t.Errorf("suggestion = %+v, want GIN for a containment predicate", s)
	}
}

// TestHotColumnVectorDistanceWithoutPgvectorIsOmitted pins the rule that a
// method-gated recommendation degrades to silence rather than to a
// recommendation the database cannot satisfy.
func TestHotColumnVectorDistanceWithoutPgvectorIsOmitted(t *testing.T) {
	t.Parallel()
	tbl := table("documento")
	tbl.Columns = []model.Column{col("id", false), {Name: "embedding", Type: "vector", BaseType: "vector"}}
	schemas := schemaWith(tbl)

	preds := []model.PredicateEvidence{
		{Column: ref("documento", "embedding"), Operator: model.OpVectorDistance, Source: model.JoinFromFunction, Object: "fn_a"},
		{Column: ref("documento", "embedding"), Operator: model.OpVectorDistance, Source: model.JoinFromView, Object: "vw_b"},
	}

	findings := audit.Findings(schemas, audit.Options{Predicates: preds, RecurrenceMin: 2, Extensions: model.NewExtensionSet(nil)})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 0 {
		t.Errorf("got %d hot-column findings, want 0: no honest recommendation without pgvector", got)
	}
}

func TestHotColumnVectorDistanceWithPgvectorRecommendsHNSW(t *testing.T) {
	t.Parallel()
	tbl := table("documento")
	tbl.Columns = []model.Column{col("id", false), {Name: "embedding", Type: "vector", BaseType: "vector"}}
	schemas := schemaWith(tbl)

	preds := []model.PredicateEvidence{
		{Column: ref("documento", "embedding"), Operator: model.OpVectorDistance, Source: model.JoinFromFunction, Object: "fn_a"},
		{Column: ref("documento", "embedding"), Operator: model.OpVectorDistance, Source: model.JoinFromView, Object: "vw_b"},
	}

	findings := audit.Findings(schemas, audit.Options{
		Predicates: preds, RecurrenceMin: 2, Extensions: model.NewExtensionSet([]string{"vector"}),
	})

	if got := countKind(findings, model.FindingUnindexedHotColumn); got != 1 {
		t.Fatalf("got %d hot-column findings, want 1", got)
	}
	if s := findings[0].Suggestion; s == nil || s.IndexMethod != "hnsw" {
		t.Errorf("suggestion = %+v, want hnsw with pgvector installed", findings[0].Suggestion)
	}
}

//go:build integration

package catalog_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func open(t *testing.T, fixture string) (*db.Pool, context.Context) {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, fixture)

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, ctx
}

func read(t *testing.T, fixture string) *catalog.Result {
	t.Helper()

	pool, ctx := open(t, fixture)
	res, err := catalog.Read(ctx, pool, catalog.Options{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}
	return res
}

func findTable(t *testing.T, res *catalog.Result, name string) model.Table {
	t.Helper()

	for _, s := range res.Schemas {
		for _, tbl := range s.Tables {
			if tbl.Name == name {
				return tbl
			}
		}
	}
	t.Fatalf("table %q not found in the model", name)
	return model.Table{}
}

// TestSessionIsReadOnly proves the guarantee by behaviour rather than by
// inspecting the setting: whatever the pool hands out, a write must fail.
//
// The result has to be drained before the error is trusted. pgx defers a
// server-side failure to the row iteration, so reading only what Query returns
// finds nil and reports a write that never happened — and leaves the
// connection checked out, which then blocks Close for as long as the test
// binary is willing to wait.
func TestSessionIsReadOnly(t *testing.T) {
	pool, ctx := open(t, "clean_schema")

	err := drain(pool.Query(ctx, "INSERT INTO municipio (id, nome) VALUES (999, 'x')"))
	if err == nil {
		t.Fatal("a write succeeded on a read-only session")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Errorf("write failed with %v, want a read-only transaction error", err)
	}

	// The row is not there, which is the claim that matters: the statement was
	// refused rather than merely reported as refused.
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM municipio WHERE id = 999").Scan(&count); err != nil {
		t.Fatalf("counting after the refused write: %v", err)
	}
	if count != 0 {
		t.Errorf("the refused INSERT left %d rows behind", count)
	}
}

// drain consumes a result set so a deferred server error surfaces and the
// connection returns to the pool. A caller that skips it gets a nil error and
// a pool that will not close.
func drain(rows pgx.Rows, err error) error {
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
	}
	return rows.Err()
}

func TestApplicationNameIsAnnounced(t *testing.T) {
	pool, ctx := open(t, "clean_schema")

	var name string
	if err := pool.QueryRow(ctx,
		"SELECT current_setting('application_name')").Scan(&name); err != nil {
		t.Fatalf("reading application_name: %v", err)
	}
	if name != db.ApplicationName {
		t.Errorf("application_name = %q, want %q so the DBA can see what is running", name, db.ApplicationName)
	}
}

func TestSessionTimeoutsAreApplied(t *testing.T) {
	pool, ctx := open(t, "clean_schema")

	for _, setting := range []string{"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout"} {
		var value string
		if err := pool.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatalf("reading %s: %v", setting, err)
		}
		if value == "0" || value == "" {
			t.Errorf("%s = %q, want a bound: an unbounded query can hold a production server", setting, value)
		}
	}
}

func TestCancellationEndsTheQuery(t *testing.T) {
	pool, _ := open(t, "clean_schema")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := drain(pool.Query(ctx, "SELECT 1")); err == nil {
		t.Fatal("a query ran on a cancelled context")
	}
}

func TestNotValidConstraintIsPreserved(t *testing.T) {
	res := read(t, "not_valid_constraints")

	pedido := findTable(t, res, "pedido")
	if len(pedido.ForeignKeys) != 1 {
		t.Fatalf("pedido has %d foreign keys, want 1", len(pedido.ForeignKeys))
	}
	if pedido.ForeignKeys[0].Validated {
		t.Error("pedido_cliente_fk was created NOT VALID and must not read as validated")
	}

	item := findTable(t, res, "item_pedido")
	if !item.ForeignKeys[0].Validated {
		t.Error("item_pedido_pedido_fk is validated and must read as such")
	}
}

func TestLeadingIndexDetection(t *testing.T) {
	res := read(t, "unindexed_fks")

	tests := map[string]bool{
		"contrato":    false, // no index at all
		"nota_fiscal": false, // present, but not leading — useless for the lookup
		"empenho":     true,  // leading in a composite index
	}

	for table, want := range tests {
		fks := findTable(t, res, table).ForeignKeys
		if len(fks) != 1 {
			t.Fatalf("%s has %d foreign keys, want 1", table, len(fks))
		}
		if got := fks[0].HasIndex; got != want {
			t.Errorf("%s: HasIndex = %v, want %v", table, got, want)
		}
	}
}

func TestUnsupportedShapesAreRecorded(t *testing.T) {
	res := read(t, "unsupported_shapes")

	byTable := make(map[string]model.UnsupportedReason)
	for _, s := range res.Coverage.TablesUnsupported {
		byTable[s.Table] = s.Reason
	}

	for table, want := range map[string]model.UnsupportedReason{
		"public.matricula":              model.ReasonCompositePK,
		"public.log_importacao":         model.ReasonNoPrimaryKey,
		"public.lancamento":             model.ReasonPartitioned,
		"public.documento_digitalizado": model.ReasonInheritance,
	} {
		if got := byTable[table]; got != want {
			t.Errorf("%s recorded as %q, want %q", table, got, want)
		}
	}

	// Partitions must not appear as tables of their own: reading them
	// separately would double-count rows.
	for _, s := range res.Schemas {
		for _, tbl := range s.Tables {
			if strings.HasPrefix(tbl.Name, "lancamento_") {
				t.Errorf("partition %s leaked into the model", tbl.Name)
			}
		}
	}
}

// TestCoverageBalances is the invariant the whole report rests on. If analyzed
// plus skipped does not equal the scope, the coverage block is lying.
func TestCoverageBalances(t *testing.T) {
	for _, fixture := range []string{"clean_schema", "not_valid_constraints", "unindexed_fks", "unsupported_shapes"} {
		t.Run(fixture, func(t *testing.T) {
			c := read(t, fixture).Coverage
			if got := c.TablesAnalyzed + c.SkippedCount(); got != c.TablesTotal {
				t.Errorf("analyzed(%d) + skipped(%d) = %d, want total %d",
					c.TablesAnalyzed, c.SkippedCount(), got, c.TablesTotal)
			}
		})
	}
}

func TestExclusionIsRecordedSeparately(t *testing.T) {
	pool, ctx := open(t, "clean_schema")

	res, err := catalog.Read(ctx, pool, catalog.Options{
		Schemas: []string{"public"},
		Exclude: []string{"endereco"},
	})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	// Excluding on request and skipping under duress are different things and
	// must not blur together in the report.
	if len(res.Coverage.TablesExcluded) != 1 {
		t.Fatalf("excluded = %v, want one entry", res.Coverage.TablesExcluded)
	}
	if len(res.Coverage.TablesNoPrivilege) != 0 {
		t.Errorf("a user filter must not be reported as a privilege problem: %v",
			res.Coverage.TablesNoPrivilege)
	}
}

func TestTypesAndCommentsAreRead(t *testing.T) {
	res := read(t, "not_valid_constraints")
	cliente := findTable(t, res, "cliente")

	if cliente.Comment != "cadastro de clientes" {
		t.Errorf("table comment = %q, want the declared one", cliente.Comment)
	}

	id, ok := cliente.Column("id")
	if !ok {
		t.Fatal("column id not found")
	}
	if id.BaseType != "int8" {
		t.Errorf("base type = %q, want int8 so equivalent spellings compare equal", id.BaseType)
	}
	if id.Type != "bigint" {
		t.Errorf("formatted type = %q, want bigint", id.Type)
	}
}

func TestStatsCarryTheResetMoment(t *testing.T) {
	res := read(t, "clean_schema")

	// A fresh container has never had statistics reset, so the moment may be
	// absent. Either way the counters must declare which case they are in.
	table := findTable(t, res, "municipio")
	if _, _, known := table.Stats.Usage.Counters(); known != table.Stats.Usage.Interpretable() {
		t.Error("the counters must agree with themselves about being interpretable")
	}
}

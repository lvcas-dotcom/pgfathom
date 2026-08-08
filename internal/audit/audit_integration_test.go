//go:build integration

package audit_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/audit"
	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// plantedValues are the recognizable strings the fixtures put in user tables.
// They stand in for what a real target database holds — national ID numbers,
// names, addresses — and none of them may ever reach an output.
var plantedValues = []string{
	"529.318.470-11",
	"145.892.663-04",
	"Maria Aparecida Silva",
	"Joao Carlos Pereira",
	"Rua das Acacias 42",
	"Construtora Horizonte LTDA",
	"CT-2019-0041",
	"Sao Bernardo do Campo",
}

func run(t *testing.T, fixture string) *model.Result {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, fixture)

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cat, err := catalog.Read(ctx, pool, catalog.Options{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	result := model.NewResult("integration", "", time.Unix(0, 0).UTC(), cat.Coverage)
	result.ServerVersion = pool.ServerVersion()
	result.Schemas = cat.Schemas
	result.Findings = audit.Findings(cat.Schemas)
	return result
}

func objects(findings []model.Finding, kind model.FindingKind) []string {
	var out []string
	for _, f := range findings {
		if f.Kind == kind {
			out = append(out, f.Object)
		}
	}
	return out
}

func TestAuditFindsNotValidConstraints(t *testing.T) {
	result := run(t, "not_valid_constraints")

	got := objects(result.Findings, model.FindingNotValidConstraint)
	if len(got) != 1 || !strings.Contains(got[0], "pedido_cliente_fk") {
		t.Errorf("NOT VALID findings = %v, want exactly pedido_cliente_fk", got)
	}
}

func TestAuditFindsUnindexedForeignKeys(t *testing.T) {
	result := run(t, "unindexed_fks")

	got := objects(result.Findings, model.FindingFKWithoutIndex)

	// empenho is covered by a leading composite index and must stay quiet;
	// nota_fiscal has the column in a composite index but not leading, which is
	// useless for the lookup a parent DELETE triggers.
	joined := strings.Join(got, " ")
	for _, want := range []string{"contrato", "nota_fiscal"} {
		if !strings.Contains(joined, want) {
			t.Errorf("unindexed findings = %v, want one for %s", got, want)
		}
	}
	if strings.Contains(joined, "empenho") {
		t.Errorf("unindexed findings = %v, want empenho excluded: its index leads with the child column", got)
	}
}

func TestAuditIsQuietOnACleanSchema(t *testing.T) {
	result := run(t, "clean_schema")

	if len(result.Findings) != 0 {
		t.Errorf("findings = %v, want none on a clean schema", result.Findings)
	}
	if !result.Coverage.Complete() {
		t.Errorf("coverage = %+v, want a complete run", result.Coverage)
	}
}

// TestAuditInventsNothingWithoutConstraints guards the boundary between audit
// and inference: with no constraints declared there is nothing for audit to say,
// and saying something anyway would mean it started guessing.
func TestAuditInventsNothingWithoutConstraints(t *testing.T) {
	if findings := run(t, "no_constraints").Findings; len(findings) != 0 {
		t.Errorf("findings = %v, want none: audit reports declared constraints, not hypotheses", findings)
	}
}

func TestRestrictedPrivilegesAreListed(t *testing.T) {
	ctx := context.Background()

	dsn := testutil.Postgres(t, "restricted_privileges")
	restricted := strings.Replace(dsn, "pgfathom_test:pgfathom_test", "pgfathom_reader:pgfathom_reader", 1)

	cfg := db.DefaultConfig()
	cfg.DSN = restricted

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool as the restricted role: %v", err)
	}
	defer pool.Close()

	cat, err := catalog.Read(ctx, pool, catalog.Options{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	skipped := strings.Join(cat.Coverage.TablesNoPrivilege, " ")
	for _, want := range []string{"folha_pagamento", "prontuario"} {
		if !strings.Contains(skipped, want) {
			t.Errorf("tables without privilege = %v, want %s listed", cat.Coverage.TablesNoPrivilege, want)
		}
	}

	if cat.Coverage.Complete() {
		t.Error("a run that could not read two tables must not report itself complete")
	}
}

// TestNoPlantedValueEscapes is the enforcement of the project's hardest rule,
// run against real data in a real server rather than against structs built by
// hand.
func TestNoPlantedValueEscapes(t *testing.T) {
	for _, fixture := range []string{
		"clean_schema", "no_constraints", "not_valid_constraints",
		"unindexed_fks", "unsupported_shapes",
	} {
		t.Run(fixture, func(t *testing.T) {
			result := run(t, fixture)

			var terminal, jsonOut bytes.Buffer
			if err := report.Terminal(&terminal, result, false); err != nil {
				t.Fatalf("rendering terminal: %v", err)
			}
			if err := report.JSON(&jsonOut, result); err != nil {
				t.Fatalf("rendering JSON: %v", err)
			}

			for name, out := range map[string]string{
				"terminal": terminal.String(),
				"json":     jsonOut.String(),
			} {
				for _, planted := range plantedValues {
					if strings.Contains(out, planted) {
						t.Errorf("%s output leaked %q from a user table", name, planted)
					}
				}
			}
		})
	}
}

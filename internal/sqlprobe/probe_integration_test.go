//go:build integration

package sqlprobe_test

import (
	"context"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/sqlprobe"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func probe(t *testing.T) *sqlprobe.Evidence {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, "usage_evidence")

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cat, err := catalog.Read(ctx, pool, catalog.Options{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	ev, err := sqlprobe.Probe(ctx, pool, cat.Schemas)
	if err != nil {
		t.Fatalf("probing: %v", err)
	}
	return ev
}

func has(ev *sqlprobe.Evidence, a, b string, source model.JoinSource) bool {
	for _, j := range ev.Joins {
		pair := j.Left.String() + "|" + j.Right.String()
		if (pair == a+"|"+b || pair == b+"|"+a) && j.Source == source {
			return true
		}
	}
	return false
}

func TestViewJoinIsMined(t *testing.T) {
	ev := probe(t)

	if !has(ev, "public.os_servico.resp_tecnico", "public.funcionario.id", model.JoinFromView) {
		t.Errorf("the view join was not mined; got %+v", ev.Joins)
	}
}

func TestFunctionBodyJoinIsMined(t *testing.T) {
	ev := probe(t)

	if !has(ev, "public.os_servico.patrimonio", "public.equipamento.id", model.JoinFromFunction) {
		t.Errorf("the plpgsql body join was not mined; got %+v", ev.Joins)
	}
}

// TestDynamicSQLYieldsNothing is the guard against manufactured evidence: the
// join inside the EXECUTE string is text, and extracting from it would invent
// a relationship between cliente and equipamento.
func TestDynamicSQLYieldsNothing(t *testing.T) {
	ev := probe(t)

	for _, j := range ev.Joins {
		if j.Left.Table == "cliente" && j.Right.Table == "equipamento" ||
			j.Left.Table == "equipamento" && j.Right.Table == "cliente" {
			t.Errorf("evidence was invented from dynamically assembled SQL: %+v", j)
		}
	}
}

func TestEvidenceNamesItsSource(t *testing.T) {
	ev := probe(t)

	for _, j := range ev.Joins {
		if j.Object == "" {
			t.Errorf("evidence %+v carries no source object: the user cannot go check it", j)
		}
	}
}

// TestStatementsAbsenceIsRecorded pins that a missing extension is a recorded
// fact rather than an error: absence of usage evidence must never look like
// absence of usage.
func TestStatementsAbsenceIsRecorded(t *testing.T) {
	if probe(t).StatementsAvailable {
		t.Error("the fixture has no pg_stat_statements; availability must read false")
	}
}

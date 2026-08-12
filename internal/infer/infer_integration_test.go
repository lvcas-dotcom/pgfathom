//go:build integration

package infer_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func inferFromFixture(t *testing.T) (*infer.Result, *model.Result) {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, "inferable")

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cat, err := catalog.Read(ctx, pool, catalog.Options{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}

	naming, err := profile.Embedded("pt-br")
	if err != nil {
		t.Fatalf("loading profile: %v", err)
	}

	res := infer.Generate(cat.Schemas, infer.Options{Profile: naming, MinScore: 0.001})

	coverage := cat.Coverage
	coverage.CandidatesFound = len(res.Candidates) + len(res.Discarded)

	out := model.NewResult("integration", naming.Name, time.Unix(0, 0).UTC(), coverage)
	out.ServerVersion = pool.ServerVersion()
	out.Schemas = cat.Schemas
	out.Candidates = append(append([]model.Candidate{}, res.Candidates...), res.Discarded...)

	return res, out
}

func lookup(res *infer.Result, child, parent string) (model.Candidate, bool) {
	for _, c := range append(append([]model.Candidate{}, res.Candidates...), res.Discarded...) {
		if c.Child.String() == child && c.Parent.String() == parent {
			return c, true
		}
	}
	return model.Candidate{}, false
}

// TestInfersThroughLegacyNaming is the end-to-end proof that the naming profile
// earns its place: neither relationship is reachable without stripping a legacy
// prefix and depluralizing.
func TestInfersThroughLegacyNaming(t *testing.T) {
	res, _ := inferFromFixture(t)

	for _, want := range []struct{ child, parent string }{
		{"public.pedido.cliente_id", "public.tb_clientes.id"},
		{"public.pedido.municipio_id", "public.municipios.id"},
	} {
		c, ok := lookup(res, want.child, want.parent)
		if !ok {
			t.Errorf("%s → %s was not inferred", want.child, want.parent)
			continue
		}
		if !c.HasSignal(model.SigNormalizedName) {
			t.Errorf("%s should record that it matched through normalization", want.child)
		}
	}
}

func TestTrapRanksBelowTheGenuineRelationship(t *testing.T) {
	res, _ := inferFromFixture(t)

	trap, ok := lookup(res, "public.pedido.status_id", "public.status.id")
	if !ok {
		t.Fatal("the trap candidate should be generated: it is a plausible hypothesis")
	}
	if !trap.HasSignal(model.SigGenericDomain) {
		t.Error("a generic name pointing at a two-row table must be penalized")
	}

	real, ok := lookup(res, "public.pedido.cliente_id", "public.tb_clientes.id")
	if !ok {
		t.Fatal("the genuine relationship was not inferred")
	}
	if trap.MetaScore >= real.MetaScore {
		t.Errorf("trap %.2f must rank below the genuine relationship %.2f",
			trap.MetaScore, real.MetaScore)
	}
}

func TestPolymorphicAndUnsupportedAreRecognized(t *testing.T) {
	res, _ := inferFromFixture(t)

	if len(res.Polymorphic) != 1 || res.Polymorphic[0].ReferenceColumn != "entidade_id" {
		t.Errorf("polymorphic pairs = %+v, want entidade_id beside entidade_tipo", res.Polymorphic)
	}

	// A single column cannot stand for a key of two, and the near miss is the
	// note that keeps the gap between name matching and the composite pass
	// visible.
	var sawArityMismatch bool
	for _, s := range res.Skipped {
		if s.Reason == infer.SkipArityMismatch {
			sawArityMismatch = true
		}
	}
	if !sawArityMismatch {
		t.Errorf("skipped = %+v, want the arity mismatch recorded", res.Skipped)
	}
}

func TestNoCandidateIsConfirmed(t *testing.T) {
	res, _ := inferFromFixture(t)

	for _, c := range append(res.Candidates, res.Discarded...) {
		if c.Verdict != model.VerdictUnvalidated {
			t.Errorf("%s carries verdict %q; no data was consulted", c.Child, c.Verdict)
		}
	}
}

// TestDiscoverOutputLeaksNothing runs the leak scan against real data from a
// real server, which is the only version of this check that proves anything.
func TestDiscoverOutputLeaksNothing(t *testing.T) {
	infered, result := inferFromFixture(t)

	var terminal, jsonOut bytes.Buffer

	if err := report.Discover(&terminal, report.DiscoverView{
		Result:          result,
		Discarded:       infered.Discarded,
		MinScore:        0.001,
		ShowDiscarded:   true,
		ValidationStage: "! candidates only",
	}); err != nil {
		t.Fatalf("rendering terminal: %v", err)
	}

	if err := report.JSON(&jsonOut, result); err != nil {
		t.Fatalf("rendering JSON: %v", err)
	}

	testutil.AssertNoLeak(t, map[string]string{
		"terminal": terminal.String(),
		"json":     jsonOut.String(),
	})
}

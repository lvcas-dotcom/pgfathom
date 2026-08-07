//go:build integration

package validate_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/stats"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// fixtureRun opens the validation fixture and carries candidates through the
// same pipeline discover uses: catalog, inference, prefilter, validation.
type fixtureRun struct {
	pool       *db.Pool
	schemas    []model.Schema
	candidates []model.Candidate
}

func setup(t *testing.T) (*fixtureRun, context.Context) {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, "validation")

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

	inferred := infer.Generate(cat.Schemas, infer.Options{Profile: naming})
	pre, err := stats.Prefilter(ctx, pool, cat.Schemas, inferred.Candidates, stats.Options{})
	if err != nil {
		t.Fatalf("running prefilter: %v", err)
	}

	return &fixtureRun{pool: pool, schemas: cat.Schemas, candidates: pre.Kept}, ctx
}

func (f *fixtureRun) validate(ctx context.Context, t *testing.T, opts validate.Options) *validate.Result {
	t.Helper()

	res, err := validate.Run(ctx, f.pool, f.schemas, f.candidates, opts)
	if err != nil {
		t.Fatalf("running validation: %v", err)
	}
	return res
}

func byChild(t *testing.T, res *validate.Result, child string) model.Candidate {
	t.Helper()

	for _, c := range res.Candidates {
		if c.Child.String() == child {
			return c
		}
	}
	t.Fatalf("candidate %s not found", child)
	return model.Candidate{}
}

func TestFullModeVerdicts(t *testing.T) {
	f, ctx := setup(t)
	res := f.validate(ctx, t, validate.Options{Full: true})

	tests := map[string]model.Verdict{
		"public.item_pedido.pedido_id":   model.VerdictConfirmed,
		"public.pedido.cliente_id":       model.VerdictBroken,
		"public.lancamento.conta_id":     model.VerdictBroken,
		"public.fatura.contrato_id":      model.VerdictRejected,
		"public.chamado.equipe_id":       model.VerdictWeak,
		"public.nota.moeda_id":           model.VerdictWeak,
		"public.pagamento.fornecedor_id": model.VerdictWeak,
	}
	for child, want := range tests {
		if c := byChild(t, res, child); c.Verdict != want {
			t.Errorf("%s: verdict = %q (%s), want %q", child, c.Verdict, c.Reason, want)
		}
	}

	if res.Validated != len(res.Candidates) || res.TimedOut != 0 {
		t.Errorf("validated %d of %d with %d timeouts, want everything resolved",
			res.Validated, len(res.Candidates), res.TimedOut)
	}
}

// TestPlantedOrphansAreCountedExactly is the phase's deliverable: the fixture
// plants 3 orphan rows over 2 values, and the tool must report exactly that.
func TestPlantedOrphansAreCountedExactly(t *testing.T) {
	f, ctx := setup(t)
	res := f.validate(ctx, t, validate.Options{Full: true})

	c := byChild(t, res, "public.pedido.cliente_id")
	v := c.Validation
	if v == nil {
		t.Fatal("the broken candidate must carry its validation metrics")
	}
	if v.OrphanRows != 3 || v.OrphanVals != 2 {
		t.Errorf("orphans = %d rows / %d values, want the planted 3/2", v.OrphanRows, v.OrphanVals)
	}
	if v.Method != model.MethodFull {
		t.Errorf("method = %q, want full: the table fits in one read", v.Method)
	}
}

func TestSampledModeNeverConfirms(t *testing.T) {
	f, ctx := setup(t)
	res := f.validate(ctx, t, validate.Options{TargetRows: 30, Seed: 42})

	// A table that fits the target is read whole, which is the conclusive mode
	// and may confirm even in a sampled run. What can never confirm is a
	// validation that actually sampled.
	for _, c := range res.Candidates {
		if c.Verdict == model.VerdictConfirmed && c.Validation.Method == model.MethodSampled {
			t.Errorf("%s came back confirmed from a sampled read", c.Child)
		}
	}

	clean := byChild(t, res, "public.item_pedido.pedido_id")
	if clean.Verdict != model.VerdictWeak || !strings.Contains(clean.Reason, "--full") {
		t.Errorf("clean sampled candidate = %q (%s), want weak pointing at --full",
			clean.Verdict, clean.Reason)
	}
}

// TestOrphanInSampleIsARealFinding pins the asymmetry: sampling weakens claims
// of absence, never of presence. The 20 planted orphans among 400 rows make a
// clean 30-row sample astronomically unlikely, and the seed freezes it.
func TestOrphanInSampleIsARealFinding(t *testing.T) {
	f, ctx := setup(t)
	res := f.validate(ctx, t, validate.Options{TargetRows: 100, Seed: 42})

	c := byChild(t, res, "public.lancamento.conta_id")
	if c.Validation == nil || c.Validation.Method != model.MethodSampled {
		t.Fatalf("lancamento should have been sampled: 400 rows against a 100-row target")
	}
	if c.Validation.OrphanRows == 0 {
		t.Fatal("the sample missed every planted orphan; adjust the seed, not the claim")
	}
	if c.Verdict != model.VerdictBroken {
		t.Errorf("verdict = %q (%s), want broken: orphans found in a sample are real",
			c.Verdict, c.Reason)
	}
}

func TestTimeoutResolvesWithoutAborting(t *testing.T) {
	f, ctx := setup(t)
	res := f.validate(ctx, t, validate.Options{Full: true, Timeout: time.Millisecond})

	if res.TimedOut == 0 {
		t.Fatal("a 1ms ceiling should have fired at least once")
	}
	for i, c := range res.Candidates {
		if c.Validation == nil && c.Verdict != model.VerdictUnvalidated {
			t.Errorf("candidate %d resolved as %q without metrics", i, c.Verdict)
		}
		if c.Verdict == model.VerdictUnvalidated && !strings.Contains(c.Reason, "ceiling") {
			t.Errorf("unvalidated candidate must carry the timeout reason, got %q", c.Reason)
		}
	}
}

// TestExoticIdentifiersValidate proves quoting against a real server, on a
// hand-built candidate: inference is not under test here, the SQL is.
func TestExoticIdentifiersValidate(t *testing.T) {
	f, ctx := setup(t)

	exotic := []model.Candidate{{
		Child:  model.ColumnRef{Schema: "public", Table: "Empenho 2024", Column: "unidade gestora"},
		Parent: model.ColumnRef{Schema: "public", Table: "Unidade Gestora", Column: "id"},
	}}

	res, err := validate.Run(ctx, f.pool, f.schemas, exotic, validate.Options{Full: true})
	if err != nil {
		t.Fatalf("validating quoted identifiers: %v", err)
	}
	if got := res.Candidates[0].Verdict; got != model.VerdictConfirmed {
		t.Errorf("verdict = %q (%s), want confirmed: every row is contained",
			got, res.Candidates[0].Reason)
	}
}

//go:build integration

package discovery_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/discovery"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// runner opens one database and runs the pipeline against it as many times as
// a test needs. The connection is shared on purpose: a second container is a
// second database, and comparing two of those measures the containers rather
// than the pipeline.
type runner struct {
	pool *db.Pool
	opts discovery.Options
}

func newRunner(t *testing.T, fixture string) *runner {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, fixture)

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	naming, err := profile.Embedded("pt-br")
	if err != nil {
		t.Fatalf("loading profile: %v", err)
	}

	return &runner{pool: pool, opts: discovery.Options{
		Profile:     naming,
		Scope:       &catalog.Scope{Schemas: []string{"public"}, Total: 1},
		ToolVersion: "integration",
		Validation:  validate.Options{Full: true},
	}}
}

func (r *runner) run(t *testing.T) *discovery.Result {
	t.Helper()

	res, err := discovery.Run(context.Background(), r.pool, r.opts)
	if err != nil {
		t.Fatalf("running discovery: %v", err)
	}
	return res
}

// TestRunIsCallableWithoutACommand is the property the benchmark harness needs:
// the pipeline executes end to end from a plain call, so what gets measured is
// the path the user runs rather than an imitation of it.
func TestRunIsCallableWithoutACommand(t *testing.T) {
	res := newRunner(t, "validation").run(t)

	byChild := make(map[string]model.Verdict)
	for _, c := range res.Result.Candidates {
		byChild[c.Child.String()] = c.Verdict
	}

	for child, want := range map[string]model.Verdict{
		"public.item_pedido.pedido_id": model.VerdictConfirmed,
		"public.pedido.cliente_id":     model.VerdictBroken,
		"public.fatura.contrato_id":    model.VerdictRejected,
	} {
		if got := byChild[child]; got != want {
			t.Errorf("%s: verdict = %q, want %q", child, got, want)
		}
	}

	if res.Result.Coverage.CandidatesValidated == 0 {
		t.Error("coverage must account for what the run validated")
	}
}

// TestRunIsDeterministic guards the property every later phase rests on: the
// same database twice produces the same findings, so a benchmark number means
// something and a report diff is readable.
//
// The comparison goes through the JSON contract rather than the structs: it is
// what consumers actually see, and the model deliberately hides fields behind
// accessors that a reflective diff cannot reach.
func TestRunIsDeterministic(t *testing.T) {
	// Both fixtures, because composite matching walks maps of targets and of
	// key signatures, and a map is where ordering goes to die.
	for _, fixture := range []string{"validation", "composite_keys"} {
		t.Run(fixture, func(t *testing.T) {
			r := newRunner(t, fixture)
			first, second := encode(t, r.run(t)), encode(t, r.run(t))

			if diff := cmp.Diff(first, second); diff != "" {
				t.Errorf("two runs of the same database differ (-first +second):\n%s", diff)
			}
		})
	}
}

// volatile matches the fields that legitimately change between two runs: when
// the run happened, and how long each step took.
var volatile = regexp.MustCompile(`"(generated_at|duration_ns)":\s*("[^"]*"|\d+)`)

func encode(t *testing.T, res *discovery.Result) string {
	t.Helper()

	out, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("encoding result: %v", err)
	}
	return volatile.ReplaceAllString(string(out), `"$1":0`)
}

// TestCompositeKeysEndToEnd is the phase in one test: a key of two columns
// travels from the catalog through matching, prefilter and validation to a
// verdict, and the shapes that must not become candidates do not.
func TestCompositeKeysEndToEnd(t *testing.T) {
	res := newRunner(t, "composite_keys").run(t)

	byChild := make(map[string]model.Candidate)
	for _, c := range res.Result.Candidates {
		byChild[c.Child.String()] = c
	}

	for child, want := range map[string]model.Verdict{
		"public.item.(empresa_id, numero)":             model.VerdictConfirmed,
		"public.rateio.(nota_empresa_id, nota_numero)": model.VerdictBroken,
		// One anchor beside a discriminator: the shape the corpus said the
		// first rule was missing.
		"public.frete.(empresa_id, nota_numero)": model.VerdictConfirmed,
	} {
		got, found := byChild[child]
		if !found {
			t.Errorf("%s produced no candidate at all", child)
			continue
		}
		if got.Verdict != want {
			t.Errorf("%s: verdict = %q (%s), want %q", child, got.Verdict, got.Reason, want)
		}
	}

	// MATCH SIMPLE exempts the three partially-NULL rows. Counting them as
	// orphans would produce a verdict the generated constraint contradicts.
	broken := byChild["public.rateio.(nota_empresa_id, nota_numero)"]
	if v := broken.Validation; v == nil {
		t.Fatal("the broken candidate carries no metrics")
	} else {
		if v.PartialNullRows != 3 {
			t.Errorf("exempt rows = %d, want 3", v.PartialNullRows)
		}
		if v.OrphanRows != 4 || v.OrphanVals != 2 {
			t.Errorf("orphans = %d rows over %d tuples, want 4 over 2", v.OrphanRows, v.OrphanVals)
		}
	}
}

// TestCompositeTrapsProduceNothing covers the shapes that look like keys and
// are not. Every one of them would validate as confirmed if generated, which is
// exactly why generation is where they have to die.
func TestCompositeTrapsProduceNothing(t *testing.T) {
	res := newRunner(t, "composite_keys").run(t)

	all := append(append([]model.Candidate{}, res.Result.Candidates...), res.Discarded...)
	for _, c := range all {
		switch c.Child.TableRef() {
		case "public.movimentacao", "public.lotacao", "public.alocacao", "public.posto":
			t.Errorf("a key signature shared by several tables must not be guessed: %s → %s",
				c.Child, c.Parent)
		case "public.aditivo":
			t.Errorf("two of three positions is not a key: %s → %s", c.Child, c.Parent)
		}
	}

	// Near misses are observations, never silence.
	var partial, ambiguous bool
	for _, f := range res.Result.Findings {
		if f.Object == "public.aditivo.(empresa_id, ano)" {
			partial = true
		}
		if f.Object == "public.movimentacao.(unidade_id, setor_id)" {
			ambiguous = true
		}
	}
	if !partial {
		t.Errorf("the partial match must be reported; findings: %v", res.Result.Findings)
	}
	if !ambiguous {
		t.Errorf("the shared signature must be reported; findings: %v", res.Result.Findings)
	}
}

// TestCompositeTablesCountAsAnalyzed closes the loop with the coverage block:
// the shape that used to be a quarter of a real schema's skip list is in scope
// now, and the proportion has to say so.
func TestCompositeTablesCountAsAnalyzed(t *testing.T) {
	res := newRunner(t, "composite_keys").run(t)

	cov := res.Result.Coverage
	if cov.TablesAnalyzed != cov.TablesTotal {
		t.Errorf("analyzed %d of %d; no table in this fixture has an unsupported shape",
			cov.TablesAnalyzed, cov.TablesTotal)
	}
	for _, s := range cov.TablesUnsupported {
		t.Errorf("%s recorded as unsupported for %q", s.Table, s.Reason)
	}
}

// TestStagesAccountForTheRun is the cost measurement checked against reality:
// the stages have to be the run, not a sample of it. A stage that quietly
// dropped out would make the benchmark publish a cost that misses work the
// user pays for.
func TestStagesAccountForTheRun(t *testing.T) {
	res := newRunner(t, "validation").run(t)

	if len(res.Stages) == 0 {
		t.Fatal("a finished run must report where its time went")
	}

	var sum time.Duration
	for _, s := range res.Stages {
		if s.Duration <= 0 {
			t.Errorf("stage %q reports %s", s.Stage, s.Duration)
		}
		sum += s.Duration
	}

	total := res.Result.Duration
	if sum > total {
		t.Errorf("stages sum to %s over a total of %s", sum, total)
	}
	// Assembling the result happens after the last stage, so the stages fall
	// short of the total by a little. Falling short by a lot would mean work is
	// happening outside every stage, which is the thing worth catching.
	if sum < total/2 {
		t.Errorf("stages account for %s of %s: most of the run happened outside any stage", sum, total)
	}

	want := []discovery.Stage{
		discovery.StageCatalog, discovery.StageDetection, discovery.StageEvidence,
		discovery.StageGeneration, discovery.StagePrefilter, discovery.StageValidation,
	}
	var got []discovery.Stage
	for _, s := range res.Stages {
		got = append(got, s.Stage)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("stage order differs from execution order (-want +got):\n%s", diff)
	}
}

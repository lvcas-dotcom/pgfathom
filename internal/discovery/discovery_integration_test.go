//go:build integration

package discovery_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

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
	r := newRunner(t, "validation")
	first, second := encode(t, r.run(t)), encode(t, r.run(t))

	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("two runs of the same database differ (-first +second):\n%s", diff)
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

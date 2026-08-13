//go:build integration

package validate_test

import (
	"context"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// keyProbePool opens a pool against the missing_pk_composite fixture, which
// carries two tables of the same shape — one with a real composite key, one
// with a planted duplicate on it.
func keyProbePool(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()

	ctx := context.Background()
	cfg := db.DefaultConfig()
	cfg.DSN = testutil.Postgres(t, "missing_pk_composite")

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, ctx
}

// TestProbeUniquenessConfirmsARealKey covers the confirming path: a full scan
// finds total rows equal distinct values with zero nulls.
func TestProbeUniquenessConfirmsARealKey(t *testing.T) {
	pool, ctx := keyProbePool(t)
	table := model.Table{Schema: "public", Name: "item_pedido"}

	results, err := validate.ProbeUniqueness(ctx, pool, table, [][]string{{"pedido_id", "sequencia"}}, 0)
	if err != nil {
		t.Fatalf("ProbeUniqueness: %v", err)
	}
	if len(results) != 1 || results[0].Verdict != model.KeyProbeConfirmed {
		t.Fatalf("results = %+v, want one confirmed result", results)
	}
}

// TestProbeUniquenessNeverConfirmsARealDuplicate is the assertion the whole
// feature exists to make: the same shape of candidate, but the data actually
// has a duplicate, must never come back confirmed.
func TestProbeUniquenessNeverConfirmsARealDuplicate(t *testing.T) {
	pool, ctx := keyProbePool(t)
	table := model.Table{Schema: "public", Name: "pagamento_parcela"}

	results, err := validate.ProbeUniqueness(ctx, pool, table, [][]string{{"contrato_id", "parcela"}}, 0)
	if err != nil {
		t.Fatalf("ProbeUniqueness: %v", err)
	}
	if len(results) != 1 || results[0].Verdict != model.KeyProbeUnverified {
		t.Fatalf("results = %+v, want one unverified result: a duplicate exists", results)
	}
	if results[0].Reason == "" {
		t.Error("an unverified result must carry a reason")
	}
}

// TestProbeUniquenessTimeoutResolvesWithoutAborting proves an absurdly short
// ceiling turns into an Unverified result, not an error the caller has to
// abort a whole run over.
func TestProbeUniquenessTimeoutResolvesWithoutAborting(t *testing.T) {
	pool, ctx := keyProbePool(t)
	table := model.Table{Schema: "public", Name: "item_pedido"}

	results, err := validate.ProbeUniqueness(ctx, pool, table, [][]string{{"pedido_id", "sequencia"}}, time.Nanosecond)
	if err != nil {
		t.Fatalf("ProbeUniqueness: %v", err)
	}
	if len(results) != 1 || results[0].Verdict != model.KeyProbeUnverified {
		t.Fatalf("results = %+v, want one unverified result under a 1ns ceiling", results)
	}
}

// TestProbeUniquenessMultipleCandidatesAreIndependent proves one candidate's
// outcome does not leak into another's: the confirmed and the duplicate
// candidate probed together must each carry their own verdict.
func TestProbeUniquenessMultipleCandidatesAreIndependent(t *testing.T) {
	pool, ctx := keyProbePool(t)
	table := model.Table{Schema: "public", Name: "item_pedido"}

	results, err := validate.ProbeUniqueness(ctx, pool, table, [][]string{
		{"pedido_id", "sequencia"}, // real key
		{"pedido_id"},              // not unique on its own: three rows share pedido_id 1 and 2
	}, 0)
	if err != nil {
		t.Fatalf("ProbeUniqueness: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Verdict != model.KeyProbeConfirmed {
		t.Errorf("(pedido_id, sequencia) = %+v, want confirmed", results[0])
	}
	if results[1].Verdict != model.KeyProbeUnverified {
		t.Errorf("(pedido_id) alone = %+v, want unverified: it repeats", results[1])
	}
}

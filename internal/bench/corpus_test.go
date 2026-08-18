//go:build benchmark

package bench_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/bench"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// TestFetchCorpus is `make corpus`: it acquires and verifies, and it is the
// only thing here that touches the network. Measuring offline is what keeps a
// reported time from including whatever the carrier did that afternoon, and an
// outage from reading as a benchmark failure.
func TestFetchCorpus(t *testing.T) {
	manifest, err := bench.LoadManifest()
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	ctx := context.Background()

	for _, e := range manifest.Schemas {
		t.Run(e.Name, func(t *testing.T) {
			if e.Kind == bench.FromLocal {
				if _, err := bench.Acquire(ctx, e); err != nil {
					t.Skipf("optional local schema not on this machine: %v", err)
				}
				return
			}

			downloaded, err := bench.Acquire(ctx, e)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if downloaded {
				t.Logf("downloaded and verified %s", e.CachePath())
			} else {
				t.Logf("already cached and verified: %s", e.CachePath())
			}
		})
	}
}

// TestCorpus is `make benchmark`. It measures every schema the manifest
// declares and writes the reports under docs/benchmark.
func TestCorpus(t *testing.T) {
	manifest, err := bench.LoadManifest()
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	started := time.Now()
	var results []bench.EntryResult
	var skipped []bench.Skipped

	for _, e := range manifest.Schemas {
		if os.Getenv("PGFATHOM_BENCH_DSN_"+strings.ToUpper(strings.ReplaceAll(e.Name, "-", "_"))) == "" && !bench.Ready(e) {
			if e.Kind == bench.FromLocal {
				// Optional by design: a real dump cannot live in a public
				// manifest. Its absence is stated rather than omitted — in the
				// published report, not only here, or regenerating the file on
				// a machine without the dump would delete the row and read as
				// if it had never been measured at all.
				const reason = "optional local dump, absent from the machine that ran this"
				t.Logf("SKIP %s: %s", e.Name, reason)
				skipped = append(skipped, bench.Skipped{Name: e.Name, Reason: reason})
				continue
			}
			t.Fatalf("%s is not in the cache; run: make corpus", e.Name)
		}

		res := measureSchema(t, e)
		results = append(results, res)
	}

	if len(results) == 0 {
		t.Fatal("no schema was measured")
	}
	if err := bench.WriteReports(results, skipped, time.Since(started)); err != nil {
		t.Fatalf("writing reports: %v", err)
	}
}

// dsnFor decides where the schema is measured.
//
// The default is a throwaway container, which is what makes the number
// reproducible by anyone. The override exists because a corpus schema is
// expensive to load and some machines cannot run Docker at all; pointing at a
// server that already holds the schema keeps the harness usable there. It
// changes nothing about the measurement, and the harness still reads the truth
// and drops the keys itself — so whatever is pointed at must be disposable.
func dsnFor(t *testing.T, e bench.Entry) (loadPath, dsn string) {
	t.Helper()

	env := "PGFATHOM_BENCH_DSN_" + strings.ToUpper(strings.ReplaceAll(e.Name, "-", "_"))
	if dsn := os.Getenv(env); dsn != "" {
		t.Logf("%s: measuring against %s, not a container — the keys in it will be dropped", e.Name, env)
		return "", dsn
	}

	// A published schema applies cleanly, so the container entrypoint loads it
	// under ON_ERROR_STOP and a failure is a real failure. A dump from a live
	// database is the other case entirely, and gets the tolerant loader.
	if e.Kind == bench.FromLocal {
		return e.CachePath(), testutil.PostgresEmpty(t, e.Postgres)
	}
	return "", testutil.PostgresScript(t, e.Postgres, e.CachePath())
}

func measureSchema(t *testing.T, e bench.Entry) bench.EntryResult {
	t.Helper()

	ctx := context.Background()
	loadPath, dsn := dsnFor(t, e)

	var loadFailures int
	if loadPath != "" {
		n, err := bench.Load(ctx, dsn, loadPath)
		if err != nil {
			t.Fatalf("%s: %v", e.Name, err)
		}
		loadFailures = n
		if n > 0 {
			t.Logf("%s: %d statements of the dump did not apply", e.Name, n)
		}
	}

	// Two connections, on purpose. This one belongs to the harness and is
	// allowed to write; it is what drops the keys. The pool below is the one
	// the tool receives, with the read-only session policies it always has.
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("%s: connecting as the harness: %v", e.Name, err)
	}
	defer func() { _ = admin.Close(ctx) }()

	truth, outOfScope, err := bench.Truth(ctx, admin, e.Schema)
	if err != nil {
		t.Fatalf("%s: %v", e.Name, err)
	}
	if len(truth) == 0 {
		t.Fatalf("%s declares no foreign key in the measured schema; there is nothing to recover", e.Name)
	}

	t.Logf("%s: %d keys in scope, %d reaching outside it", e.Name, len(truth), outOfScope)

	cfg := db.DefaultConfig()
	cfg.DSN = dsn
	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("%s: opening pool: %v", e.Name, err)
	}
	defer pool.Close()

	result := bench.EntryResult{
		Entry:          e,
		ServerVersion:  pool.ServerVersion(),
		Truth:          len(truth),
		TruthComposite: bench.CountComposite(truth),
		OutOfScope:     outOfScope,
		LoadFailures:   loadFailures,
	}

	// Partial first, then greenfield: greenfield starts from exactly the state
	// partial leaves behind, so the second regime costs no second load and both
	// describe the same database loaded the same way.
	removed, kept := bench.Split(truth)

	for _, regime := range bench.Regimes {
		scored := removed
		toDrop := removed
		if regime == bench.RegimeGreenfield {
			scored = truth
			toDrop = kept
		}

		dropped, err := bench.DropForeignKeys(ctx, admin, e.Schema, toDrop)
		if err != nil {
			t.Fatalf("%s / %s: %v", e.Name, regime, err)
		}
		t.Logf("%s / %s: %d keys dropped, scoring against %d", e.Name, regime, dropped, len(scored))

		for _, cfg := range bench.Configs {
			m, err := bench.Measure(ctx, pool, e, regime, cfg, scored)
			if err != nil {
				t.Fatalf("%s / %s / %s: %v", e.Name, regime, cfg.Name, err)
			}
			result.Tables = m.Coverage.TablesTotal
			result.Measurements = append(result.Measurements, m)

			t.Logf("  %s / %s: %d of %d recovered (%.1f%%), %d candidates, %d outside",
				regime, cfg.Name, m.Recovered, m.Truth, 100*m.Recall(),
				m.Candidates, m.Unmatched)
		}
	}

	return result
}

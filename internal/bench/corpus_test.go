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

	for _, s := range manifest.Schemas {
		t.Run(s.Name, func(t *testing.T) {
			if s.Kind == bench.FromLocal {
				if _, err := bench.Acquire(ctx, s); err != nil {
					t.Skipf("optional local schema not on this machine: %v", err)
				}
				return
			}

			downloaded, err := bench.Acquire(ctx, s)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if downloaded {
				t.Logf("downloaded and verified %s", s.CachePath())
			} else {
				t.Logf("already cached and verified: %s", s.CachePath())
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
	var results []bench.SchemaResult

	for _, s := range manifest.Schemas {
		if os.Getenv("PGFATHOM_BENCH_DSN_"+strings.ToUpper(strings.ReplaceAll(s.Name, "-", "_"))) == "" && !bench.Ready(s) {
			if s.Kind == bench.FromLocal {
				// Optional by design: a real dump cannot live in a public
				// manifest. Its absence is stated rather than omitted.
				t.Logf("SKIP %s: optional local schema absent from this machine", s.Name)
				continue
			}
			t.Fatalf("%s is not in the cache; run: make corpus", s.Name)
		}

		res := measureSchema(t, s)
		results = append(results, res)
	}

	if len(results) == 0 {
		t.Fatal("no schema was measured")
	}
	if err := bench.WriteReports(results, time.Since(started)); err != nil {
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
func dsnFor(t *testing.T, s bench.Schema) string {
	t.Helper()

	env := "PGFATHOM_BENCH_DSN_" + strings.ToUpper(strings.ReplaceAll(s.Name, "-", "_"))
	if dsn := os.Getenv(env); dsn != "" {
		t.Logf("%s: measuring against %s, not a container — the keys in it will be dropped", s.Name, env)
		return dsn
	}
	return testutil.PostgresScript(t, s.Postgres, s.CachePath())
}

func measureSchema(t *testing.T, s bench.Schema) bench.SchemaResult {
	t.Helper()

	ctx := context.Background()
	dsn := dsnFor(t, s)

	// Two connections, on purpose. This one belongs to the harness and is
	// allowed to write; it is what drops the keys. The pool below is the one
	// the tool receives, with the read-only session policies it always has.
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("%s: connecting as the harness: %v", s.Name, err)
	}
	defer func() { _ = admin.Close(ctx) }()

	truth, outOfScope, err := bench.Truth(ctx, admin)
	if err != nil {
		t.Fatalf("%s: %v", s.Name, err)
	}
	if len(truth) == 0 {
		t.Fatalf("%s declares no foreign key in the measured schema; there is nothing to recover", s.Name)
	}

	dropped, err := bench.DropForeignKeys(ctx, admin)
	if err != nil {
		t.Fatalf("%s: %v", s.Name, err)
	}
	t.Logf("%s: %d keys in scope (%d out), %d dropped", s.Name, len(truth), outOfScope, dropped)

	cfg := db.DefaultConfig()
	cfg.DSN = dsn
	pool, err := db.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("%s: opening pool: %v", s.Name, err)
	}
	defer pool.Close()

	result := bench.SchemaResult{
		Schema:         s,
		ServerVersion:  pool.ServerVersion(),
		Truth:          len(truth),
		TruthComposite: bench.CountComposite(truth),
		OutOfScope:     outOfScope,
	}

	for _, cfg := range bench.Configs {
		m, err := bench.Measure(ctx, pool, s, cfg, truth)
		if err != nil {
			t.Fatalf("%s / %s: %v", s.Name, cfg.Name, err)
		}
		result.Tables = m.Coverage.TablesTotal
		result.Measurements = append(result.Measurements, m)

		t.Logf("%s / %s: %d of %d recovered (%.1f%%), %d candidates, %d outside the truth set",
			s.Name, cfg.Name, m.Recovered, len(truth), 100*m.Recall(len(truth)),
			m.Candidates, m.Unmatched)
	}

	return result
}

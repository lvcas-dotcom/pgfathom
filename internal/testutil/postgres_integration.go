//go:build integration || benchmark

package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresImage is the server the integration suite runs against. It sits at
// the supported floor rather than the newest release, so a query that silently
// depends on a newer catalog fails here instead of in a user's database.
const PostgresImage = "postgres:13-alpine"

// startupTimeout covers loading the init script, not just booting the server.
// A corpus schema of a thousand tables takes minutes to apply, and a ceiling
// tuned to the fixtures would turn that into a spurious failure.
const startupTimeout = 10 * time.Minute

// Postgres starts a throwaway server, loads a named fixture and returns a DSN.
//
// A real server is the only honest way to test catalog reading: a fake would
// test the fake. The container is torn down when the test ends.
func Postgres(t *testing.T, fixture string) string {
	t.Helper()
	return PostgresImageDSN(t, PostgresImage, fixture)
}

// PostgresImageDSN is Postgres with the server image chosen by the caller, so a
// test can assert behaviour against a version the tool does not support.
func PostgresImageDSN(t *testing.T, image, fixture string) string {
	t.Helper()
	return PostgresScript(t, image, fixturePath(t, fixture))
}

// PostgresScript is Postgres loaded from an SQL file named by absolute path,
// which is how the benchmark harness loads a corpus schema that does not live
// in testdata. Loading a corpus schema can take minutes, so the startup ceiling
// is generous here in a way the fixture path never needs.
func PostgresScript(t *testing.T, image, script string) string {
	t.Helper()

	ctx := context.Background()

	container, err := postgres.Run(ctx, image,
		postgres.WithDatabase("pgfathom_test"),
		postgres.WithUsername("pgfathom_test"),
		postgres.WithPassword("pgfathom_test"),
		postgres.WithInitScripts(script),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(startupTimeout),
		),
	)
	if err != nil {
		t.Fatalf("starting PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminating container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("resolving connection string: %v", err)
	}
	return dsn
}

// fixturePath resolves a fixture against this package's source directory, not
// against the working directory.
//
// go test runs each package with its own directory as the working directory, so
// a relative "testdata/x.sql" would resolve differently for every caller and
// find the file only in the package that happens to hold it. The fixtures are
// shared across catalog, audit, db and infer tests, so they live here and are
// located from here.
//
// A missing fixture fails loudly: a typo must never turn into a silently empty
// schema, which would make a test pass by finding nothing.
func fixturePath(t *testing.T, fixture string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate the testutil source directory")
	}

	path := filepath.Join(filepath.Dir(thisFile), "testdata", fixture+".sql")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture %q not found at %s: %v", fixture, path, err)
	}
	return path
}

//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/cli"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// runCommand executes the binary's command tree against a live server and
// returns both streams, failing on a non-zero exit.
func runCommand(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader("")}

	if code := cli.Run(append(args, "--color", "never"), streams); code != 0 {
		t.Fatalf("exit code %d for %v, stderr:\n%s", code, args, errOut.String())
	}
	return out.String(), errOut.String()
}

func readArtifact(t *testing.T, dir, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading generated %s: %v", name, err)
	}
	return string(content)
}

// executable strips the commentary and leaves what psql would actually run.
// The commented DDL is the point of half these files, so a test that applied
// the whole text would prove nothing about the half that is meant to execute.
func executable(content string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// TestGeneratedSQLRunsWithoutEditing is the exit criterion for the artifacts.
// It is the failure mode unit tests cannot reach: text that reads perfectly and
// does not parse. The statements are applied to the very database that produced
// them, which is the only claim the file makes.
func TestGeneratedSQLRunsWithoutEditing(t *testing.T) {
	dsn := testutil.Postgres(t, "validation")
	dir := t.TempDir()

	runCommand(t, "discover", "--full", "--dsn", dsn, "--out", dir)

	conn := connect(t, dsn)
	ctx := context.Background()

	confirmed := readArtifact(t, dir, report.FileConfirmed)
	if stmts := executable(confirmed); stmts != "" {
		if _, err := conn.Exec(ctx, stmts); err != nil {
			t.Fatalf("the confirmed artifact does not run as generated: %v\n--- statements ---\n%s", err, stmts)
		}
	}

	// Every constraint the file created must be NOT VALID: running the whole
	// artifact is supposed to be the cheap half, never a full table scan under
	// the stronger lock.
	rows, err := conn.Query(ctx,
		`SELECT conname, convalidated FROM pg_constraint WHERE contype = 'f' AND conname LIKE 'fk\_%'`)
	if err != nil {
		t.Fatalf("reading pg_constraint: %v", err)
	}
	defer rows.Close()

	var created int
	for rows.Next() {
		var name string
		var validated bool
		if err := rows.Scan(&name, &validated); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		created++
		if validated {
			t.Errorf("constraint %s was created as validated; the DDL must stay NOT VALID", name)
		}
	}
	if rows.Err() != nil {
		t.Fatalf("iterating: %v", rows.Err())
	}
	if created == 0 {
		t.Fatal("the validation fixture has confirmable relationships; none reached the artifact")
	}

	// The orphan queries are executable on purpose: looking comes before
	// altering, and a query that does not parse breaks that first step.
	for _, stmt := range strings.Split(executable(readArtifact(t, dir, report.FileBroken)), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Errorf("an orphan query does not run: %v\n--- query ---\n%s", err, stmt)
		}
	}
}

// TestGeneratedAuditSQLRuns covers the other artifact against the fixture built
// for it: constraints declared NOT VALID with orphans already in place.
func TestGeneratedAuditSQLRuns(t *testing.T) {
	dsn := testutil.Postgres(t, "not_valid_constraints")
	dir := t.TempDir()

	runCommand(t, "audit", "--dsn", dsn, "--out", dir)

	content := readArtifact(t, dir, report.FileNotValid)
	if !strings.Contains(content, "VALIDATE CONSTRAINT") {
		t.Fatalf("the fixture has unvalidated constraints; the artifact does not mention them:\n%s", content)
	}

	conn := connect(t, dsn)
	for _, stmt := range strings.Split(executable(content), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := conn.Exec(context.Background(), stmt); err != nil {
			t.Errorf("a check query does not run: %v\n--- query ---\n%s", err, stmt)
		}
	}
}

// TestExoticNamesSurviveGeneration is the case unquoted SQL breaks on, proven
// against a real parser rather than against a string comparison.
func TestExoticNamesSurviveGeneration(t *testing.T) {
	dsn := testutil.Postgres(t, "validation")
	dir := t.TempDir()

	runCommand(t, "discover", "--full", "--dsn", dsn, "--out", dir)

	conn := connect(t, dsn)
	for _, name := range []string{report.FileConfirmed, report.FileBroken} {
		for _, stmt := range strings.Split(executable(readArtifact(t, dir, name)), ";") {
			if !strings.Contains(stmt, `"`) || strings.TrimSpace(stmt) == "" {
				continue
			}
			// EXPLAIN parses and plans without executing, which is enough to
			// catch a quoting mistake in a statement already applied above.
			if strings.HasPrefix(strings.TrimSpace(stmt), "SELECT") {
				if _, err := conn.Exec(context.Background(), "EXPLAIN "+stmt); err != nil {
					t.Errorf("%s: statement does not parse: %v\n%s", name, err, stmt)
				}
			}
		}
	}
}

// TestArtifactsNeverCarryUserData extends the end-to-end leak scan to the files
// the tool writes. They are a third output surface, and the rule applies to all
// three without exception.
func TestArtifactsNeverCarryUserData(t *testing.T) {
	cases := []struct {
		fixture string
		args    []string
		files   []string
	}{
		{"validation", []string{"discover", "--full"}, []string{report.FileConfirmed, report.FileBroken}},
		{"inferable", []string{"discover", "--include-rejected"}, []string{report.FileConfirmed, report.FileBroken}},
		{"stats_prefilter", []string{"discover"}, []string{report.FileConfirmed, report.FileBroken}},
		{"not_valid_constraints", []string{"audit"}, []string{report.FileNotValid}},
		{"restricted_privileges", []string{"audit"}, []string{report.FileNotValid}},
		{"unindexed_fks", []string{"audit"}, []string{report.FileNotValid}},
	}

	for _, tc := range cases {
		t.Run(tc.fixture+" "+tc.args[0], func(t *testing.T) {
			dsn := testutil.Postgres(t, tc.fixture)
			dir := t.TempDir()

			runCommand(t, append(tc.args, "--dsn", dsn, "--out", dir, "--log-level", "debug")...)

			for _, file := range tc.files {
				content := readArtifact(t, dir, file)
				for _, planted := range plantedValues {
					if strings.Contains(content, planted) {
						t.Errorf("%s leaked %q from a user table", file, planted)
					}
				}
			}
		})
	}
}

// TestManifestIsTheResultInSQLMode covers the one case where stdout carries
// neither a table nor a document: an empty stdout on a successful run reads
// exactly like a failure.
func TestManifestIsTheResultInSQLMode(t *testing.T) {
	dsn := testutil.Postgres(t, "validation")
	dir := t.TempDir()

	stdout, _ := runCommand(t, "discover", "--full", "--format", "sql", "--dsn", dsn, "--out", dir)

	for _, name := range []string{report.FileConfirmed, report.FileBroken} {
		if !strings.Contains(stdout, name) {
			t.Errorf("the manifest must list %s:\n%s", name, stdout)
		}
	}
	if !strings.Contains(stdout, "Review every file") {
		t.Errorf("the manifest must repeat that the files need reading:\n%s", stdout)
	}
}

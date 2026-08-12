//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/cli"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// runAuditJSON runs `audit --format json` against dsn with the given extra
// flags and decodes the result.
func runAuditJSON(t *testing.T, dsn string, extra ...string) *model.Result {
	t.Helper()

	var out, errOut bytes.Buffer
	streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader("")}

	args := append([]string{"audit", "--format", "json", "--dsn", dsn, "--color", "never"}, extra...)
	if code := cli.Run(args, streams); code != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", code, errOut.String())
	}

	var res model.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decoding JSON output: %v", err)
	}
	return &res
}

func findingsOfKind(r *model.Result, kind model.FindingKind) []model.Finding {
	var out []model.Finding
	for _, f := range r.Findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func findingByObject(t *testing.T, r *model.Result, object string) model.Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Object == object {
			return f
		}
	}
	t.Fatalf("no finding for %s among %+v", object, r.Findings)
	return model.Finding{}
}

// TestMissingPrimaryKeyPromotedFromUnique covers the cheap, catalog-only path:
// a table with no PK but a UNIQUE NOT NULL constraint is offered promotion,
// with no data read required to prove it.
func TestMissingPrimaryKeyPromotedFromUnique(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_promotable")
	res := runAuditJSON(t, dsn)

	findings := findingsOfKind(res, model.FindingMissingPrimaryKey)
	if len(findings) != 1 {
		t.Fatalf("got %d missing_primary_key findings, want 1: %+v", len(findings), res.Findings)
	}

	s := findings[0].Suggestion
	if s == nil || s.Kind != model.SuggestPromoteUnique {
		t.Fatalf("suggestion = %+v, want promote_unique", s)
	}
	if len(s.Columns) != 1 || s.Columns[0] != "cpf" {
		t.Errorf("suggestion columns = %v, want [cpf]", s.Columns)
	}
	if len(res.Coverage.KeyProbesSkipped) != 0 {
		t.Errorf("a promotable unique never needs a data probe: coverage = %+v", res.Coverage.KeyProbesSkipped)
	}
}

// TestMissingCompositeKeyConfirmedByProbe covers the one place audit reads
// table data: a full-scan count confirms a composite key the catalog alone
// could not prove — and, in the same fixture, refuses to confirm a candidate
// that actually has a duplicate.
func TestMissingCompositeKeyConfirmedByProbe(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_composite")
	res := runAuditJSON(t, dsn)

	if got := findingsOfKind(res, model.FindingMissingPrimaryKey); len(got) != 2 {
		t.Fatalf("got %d missing_primary_key findings, want 2 (one per table): %+v", len(got), res.Findings)
	}

	clean := findingByObject(t, res, "public.item_pedido")
	s := clean.Suggestion
	if s == nil || s.Kind != model.SuggestCreatePrimaryKey {
		t.Fatalf("item_pedido suggestion = %+v, want create_primary_key", s)
	}
	if s.KeyProbe != model.KeyProbeConfirmed {
		t.Fatalf("item_pedido key_probe = %q, want confirmed: the planted data has no duplicate pair", s.KeyProbe)
	}
	want := map[string]bool{"pedido_id": true, "sequencia": true}
	if len(s.Columns) != 2 || !want[s.Columns[0]] || !want[s.Columns[1]] {
		t.Errorf("item_pedido suggestion columns = %v, want pedido_id and sequencia", s.Columns)
	}

	// pagamento_parcela carries a planted duplicate on the same shape of
	// candidate: the probe must not confirm it, and must not name columns for
	// a candidate it could not prove.
	dup := findingByObject(t, res, "public.pagamento_parcela")
	if ds := dup.Suggestion; ds == nil || ds.KeyProbe != model.KeyProbeUnverified || len(ds.Columns) != 0 {
		t.Errorf("pagamento_parcela suggestion = %+v, want unverified with no columns: a real duplicate exists", dup.Suggestion)
	}
}

// TestNoProbeKeysFlagStaysDataFree proves --no-probe-keys really disables the
// only data read audit ever performs: the same fixture that gets a confirmed
// composite key above must come back unconfirmed here, with no columns named.
func TestNoProbeKeysFlagStaysDataFree(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_composite")
	res := runAuditJSON(t, dsn, "--no-probe-keys")

	clean := findingByObject(t, res, "public.item_pedido")
	s := clean.Suggestion
	if s == nil || s.KeyProbe != "" || len(s.Columns) != 0 {
		t.Errorf("suggestion = %+v, want no probe verdict and no columns with --no-probe-keys", s)
	}
}

// TestProbeKeysMaxRowsSkipsLargeTables proves the row ceiling is honored: with
// it set below the fixtures' table sizes, the probe never runs and the skip is
// recorded in coverage rather than silently absent.
func TestProbeKeysMaxRowsSkipsLargeTables(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_composite")
	res := runAuditJSON(t, dsn, "--probe-keys-max-rows", "0")

	clean := findingByObject(t, res, "public.item_pedido")
	if s := clean.Suggestion; s == nil || s.KeyProbe != "" {
		t.Errorf("suggestion = %+v, want no probe verdict above the row ceiling", clean.Suggestion)
	}
	if len(res.Coverage.KeyProbesSkipped) != 2 {
		t.Fatalf("coverage.key_probes_skipped = %v, want both tables listed", res.Coverage.KeyProbesSkipped)
	}
}

// TestHotColumnFoundFromViewAndFunction covers the join-and-filter evidence
// path: a column two different objects name, with no index leading it.
func TestHotColumnFoundFromViewAndFunction(t *testing.T) {
	dsn := testutil.Postgres(t, "hot_column_unindexed")
	res := runAuditJSON(t, dsn)

	findings := findingsOfKind(res, model.FindingUnindexedHotColumn)
	if len(findings) != 1 {
		t.Fatalf("got %d unindexed_hot_column findings, want 1: %+v", len(findings), res.Findings)
	}

	f := findings[0]
	if !strings.Contains(f.Object, "centro_custo_id") {
		t.Errorf("object = %q, want it to name centro_custo_id", f.Object)
	}
	if f.Suggestion == nil || f.Suggestion.IndexMethod != "btree" {
		t.Errorf("suggestion = %+v, want btree", f.Suggestion)
	}
}

// TestRecurrenceMinFiltersOutSingleMention proves the threshold is enforced:
// raising it above the fixture's two mentions must silence the finding.
func TestRecurrenceMinFiltersOutSingleMention(t *testing.T) {
	dsn := testutil.Postgres(t, "hot_column_unindexed")
	res := runAuditJSON(t, dsn, "--recurrence-min", "3")

	if findings := findingsOfKind(res, model.FindingUnindexedHotColumn); len(findings) != 0 {
		t.Errorf("got %d hot-column findings above the fixture's recurrence, want 0: %+v", len(findings), findings)
	}
}

// TestJsonbContainmentRecommendsGIN covers the type-gated index method: jsonb
// containment gets GIN with no operator class, since jsonb_ops is the default.
func TestJsonbContainmentRecommendsGIN(t *testing.T) {
	dsn := testutil.Postgres(t, "jsonb_containment")
	res := runAuditJSON(t, dsn)

	findings := findingsOfKind(res, model.FindingUnindexedHotColumn)
	if len(findings) != 1 {
		t.Fatalf("got %d unindexed_hot_column findings, want 1: %+v", len(findings), res.Findings)
	}

	s := findings[0].Suggestion
	if s == nil || s.IndexMethod != "gin" || s.IndexOpclass != "" {
		t.Errorf("suggestion = %+v, want gin with no operator class for jsonb", s)
	}
}

// TestSuggestedKeysArtifactPromotesLiveUnique proves the cheap promotion
// statement is not just well-formed but actually runs against the database
// that produced it.
func TestSuggestedKeysArtifactPromotesLiveUnique(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_promotable")
	dir := t.TempDir()

	runCommand(t, "audit", "--dsn", dsn, "--out", dir)

	content := readArtifact(t, dir, report.FileSuggestedKeys)
	stmts := executable(content)
	if stmts == "" {
		t.Fatalf("expected a live promotion statement:\n%s", content)
	}

	conn := connect(t, dsn)
	if _, err := conn.Exec(context.Background(), stmts); err != nil {
		t.Fatalf("the promotion statement does not run as generated: %v\n--- statements ---\n%s", err, stmts)
	}

	var hasPK bool
	err := conn.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'cadastro_pessoa'::regclass AND contype = 'p')`,
	).Scan(&hasPK)
	if err != nil {
		t.Fatalf("checking for the promoted key: %v", err)
	}
	if !hasPK {
		t.Error("running the artifact must leave cadastro_pessoa with a primary key")
	}
}

// TestSuggestedIndexesArtifactParsesUnderExplain proves the commented
// CONCURRENTLY statement is syntactically real SQL, not just plausible text —
// the same discipline TestExoticNamesSurviveGeneration applies to discover.
func TestSuggestedIndexesArtifactParsesUnderExplain(t *testing.T) {
	dsn := testutil.Postgres(t, "hot_column_unindexed")
	dir := t.TempDir()

	runCommand(t, "audit", "--dsn", dsn, "--out", dir)

	content := readArtifact(t, dir, report.FileSuggestedIndexes)

	var stmt string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "-- ")
		if strings.HasPrefix(trimmed, "CREATE INDEX CONCURRENTLY") {
			stmt = strings.TrimSuffix(trimmed, ";")
			break
		}
	}
	if stmt == "" {
		t.Fatalf("expected a commented CREATE INDEX CONCURRENTLY line:\n%s", content)
	}

	// CONCURRENTLY cannot run inside a transaction, and EXPLAIN cannot plan
	// DDL either; a plain, separate connection running it for real is the
	// only honest parse check available, and it also proves the artifact's
	// own claim about the fixture.
	conn := connect(t, dsn)
	if _, err := conn.Exec(context.Background(), stmt); err != nil {
		t.Errorf("the suggested index does not run as written: %v\n--- statement ---\n%s", err, stmt)
	}
}

// TestEfficiencyFindingsNeverLeakUserData extends the artifact leak scan to
// the two new files: the probe reads real rows, and the discipline that keeps
// values from ever reaching output has to hold there too.
func TestEfficiencyFindingsNeverLeakUserData(t *testing.T) {
	cases := []struct {
		fixture string
		files   []string
	}{
		{"missing_pk_promotable", []string{report.FileSuggestedKeys}},
		{"missing_pk_composite", []string{report.FileSuggestedKeys}},
		{"hot_column_unindexed", []string{report.FileSuggestedIndexes}},
		{"jsonb_containment", []string{report.FileSuggestedIndexes}},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dsn := testutil.Postgres(t, tc.fixture)
			dir := t.TempDir()

			stdout, stderr := runCommand(t, "audit", "--dsn", dsn, "--out", dir, "--format", "json", "--log-level", "debug")

			artifacts := map[string]string{"stdout": stdout, "stderr": stderr}
			for _, file := range tc.files {
				artifacts[file] = readArtifact(t, dir, file)
			}
			testutil.AssertNoLeak(t, artifacts)
		})
	}
}

// pgvectorImage has the extension preinstalled; the standard postgres image
// used everywhere else in this suite does not carry it.
const pgvectorImage = "pgvector/pgvector:pg13"

// TestVectorDistanceRecommendsHNSW covers the extension-gated method end to
// end. It needs an image most CI environments will not have cached, so a
// container that fails to start skips the test instead of failing the suite.
func TestVectorDistanceRecommendsHNSW(t *testing.T) {
	dsn, ok := testutil.TryPostgresImageDSN(t, pgvectorImage, "pgvector_unindexed")
	if !ok {
		t.Skip("pgvector image not available in this environment")
	}

	stdout, stderr := runCommand(t, "audit", "--dsn", dsn, "--format", "json", "--log-level", "debug")
	testutil.AssertNoLeak(t, map[string]string{"stdout": stdout, "stderr": stderr})

	var res model.Result
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding JSON output: %v", err)
	}

	findings := findingsOfKind(&res, model.FindingUnindexedHotColumn)
	if len(findings) != 1 {
		t.Fatalf("got %d unindexed_hot_column findings, want 1: %+v", len(findings), res.Findings)
	}

	s := findings[0].Suggestion
	if s == nil || s.IndexMethod != "hnsw" || s.IndexOpclass != "vector_l2_ops" {
		t.Errorf("suggestion = %+v, want hnsw/vector_l2_ops with pgvector installed", s)
	}
}

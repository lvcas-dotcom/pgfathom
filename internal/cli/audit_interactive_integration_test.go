//go:build integration

package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/cli"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// runAuditJSONInteractive is runAuditJSON with Interactive forced true and a
// canned answer feed — the shape the one, schema-wide prompt
// resolveUnconfirmedKeys asks reads stdin through. It returns stderr too,
// since that is where the prompt itself, not the result, has to land.
func runAuditJSONInteractive(t *testing.T, dsn, answers string, extra ...string) (*model.Result, string) {
	t.Helper()

	var out, errOut bytes.Buffer
	streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader(answers), Interactive: true}

	args := append([]string{"audit", "--format", "json", "--dsn", dsn, "--color", "never"}, extra...)
	if code := cli.Run(args, streams); code != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", code, errOut.String())
	}

	var res model.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decoding JSON output: %v\nstderr:\n%s", err, errOut.String())
	}
	return &res, errOut.String()
}

// TestInteractiveCompositeKeyChosenGlobally covers the gap candidateKeys
// cannot see on its own: a bridge table whose two single-column foreign
// keys, with no index over the pair, are its real key. Answering "a" at the
// one schema-wide prompt is what makes resolveUnconfirmedKeys try that
// combination — the automatic path never does.
func TestInteractiveCompositeKeyChosenGlobally(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_fk_bridge")

	res, stderr := runAuditJSONInteractive(t, dsn, "a\n")

	f := findingByObject(t, res, "public.pedido_produto")
	s := f.Suggestion
	if s == nil || s.Kind != model.SuggestCreatePrimaryKey {
		t.Fatalf("suggestion = %+v, want create_primary_key", s)
	}
	if s.KeyProbe != model.KeyProbeConfirmed {
		t.Fatalf("key_probe = %q, want confirmed: (pedido_id, produto_id) has no duplicate pair", s.KeyProbe)
	}
	want := map[string]bool{"pedido_id": true, "produto_id": true}
	if len(s.Columns) != 2 || !want[s.Columns[0]] || !want[s.Columns[1]] {
		t.Errorf("columns = %v, want pedido_id and produto_id", s.Columns)
	}
	if !strings.Contains(stderr, "table(s) have no confirmed primary key") {
		t.Errorf("the summary must be printed once, before the prompt:\n%s", stderr)
	}
	if !strings.Contains(stderr, "untested composite candidate") {
		t.Errorf("the summary must say how many tables have a composite candidate:\n%s", stderr)
	}
}

// TestInteractiveSyntheticColumnChosenGlobally covers the other branch of the
// same prompt: answering "b" names the synthetic column after the schema's
// own convention — never typed — with no data probe at all.
func TestInteractiveSyntheticColumnChosenGlobally(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_fk_bridge")

	res, stderr := runAuditJSONInteractive(t, dsn, "b\n")

	f := findingByObject(t, res, "public.pedido_produto")
	s := f.Suggestion
	if s == nil || s.Kind != model.SuggestSyntheticPrimaryKey {
		t.Fatalf("suggestion = %+v, want synthesize_primary_key", s)
	}
	if len(s.Columns) != 1 || s.Columns[0] != "idkey" {
		t.Errorf("columns = %v, want [idkey]: the name pedido, produto and situacao already use", s.Columns)
	}
	if s.KeyProbe != "" {
		t.Errorf("key_probe = %q, want empty: a synthetic column is never confirmed by data", s.KeyProbe)
	}
	if !strings.Contains(s.Note, "schema convention") {
		t.Errorf("note = %q, want it to cite the schema convention the name came from", s.Note)
	}
	if !strings.Contains(stderr, "schema convention for a primary key name") {
		t.Errorf("the summary must state the convention before asking:\n%s", stderr)
	}
}

// TestInteractiveEmptyAnswerSkips covers the third branch: an empty line
// leaves every unresolved finding exactly where the automatic path left it.
func TestInteractiveEmptyAnswerSkips(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_fk_bridge")

	res, _ := runAuditJSONInteractive(t, dsn, "\n")

	f := findingByObject(t, res, "public.pedido_produto")
	s := f.Suggestion
	if s == nil || s.Kind != model.SuggestCreatePrimaryKey || len(s.Columns) != 0 {
		t.Errorf("suggestion = %+v, want create_primary_key with no columns: skipping must not invent one", s)
	}
}

// TestInteractiveInvalidAnswerReprompts proves a mistyped answer is not
// silently treated as a skip: the run must say so and ask again, resolving
// on whatever valid answer eventually arrives.
func TestInteractiveInvalidAnswerReprompts(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_fk_bridge")

	res, stderr := runAuditJSONInteractive(t, dsn, "z\nb\n")

	if !strings.Contains(stderr, "invalid answer") {
		t.Errorf("a mistyped answer must be reported as invalid, not silently skipped:\n%s", stderr)
	}

	f := findingByObject(t, res, "public.pedido_produto")
	if s := f.Suggestion; s == nil || s.Kind != model.SuggestSyntheticPrimaryKey {
		t.Errorf("suggestion = %+v, want the run to resolve on the valid answer that followed", f.Suggestion)
	}
}

// TestNonInteractiveNeverPrompts is the regression the whole feature is gated
// behind: the default Streams every other test in this package builds, with
// Interactive left at its zero value, must never write a prompt or block on
// stdin, even against a fixture that would otherwise trigger one.
func TestNonInteractiveNeverPrompts(t *testing.T) {
	dsn := testutil.Postgres(t, "missing_pk_fk_bridge")

	stdout, stderr := runCommand(t, "audit", "--dsn", dsn, "--format", "json")
	if strings.Contains(stderr, "table(s) have no confirmed primary key") {
		t.Errorf("a non-interactive run must never print the resolution summary or prompt:\n%s", stderr)
	}

	var res model.Result
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding JSON output: %v", err)
	}

	f := findingByObject(t, &res, "public.pedido_produto")
	if s := f.Suggestion; s == nil || s.Kind != model.SuggestCreatePrimaryKey || len(s.Columns) != 0 {
		t.Errorf("suggestion = %+v, want create_primary_key with no columns when nothing answered it", s)
	}
}

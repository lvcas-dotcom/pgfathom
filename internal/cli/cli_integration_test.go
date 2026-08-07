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

// plantedValues is the union of the recognizable strings every fixture plants
// in user tables. The commands under test run against a subset, but scanning
// for all of them costs nothing and survives fixture reshuffling.
var plantedValues = []string{
	"529.318.470-11",
	"145.892.663-04",
	"Maria Aparecida Silva",
	"Joao Carlos Pereira",
	"Rua das Acacias 42",
	"Construtora Horizonte LTDA",
	"CT-2019-0041",
	"Sao Bernardo do Campo",
	"servico de manutencao",
	"peca de reposicao",
	"Leitura Manual Bloco C",
	"Secretaria de Obras",
	"Compressor Industrial",
	"Prefeitura de Sao Bernardo",
}

// TestNoUserDataInAnyStream runs the real commands end to end and scans every
// byte the process would emit — stdout, and stderr carrying both diagnostics
// and the debug log. The layer-level scans prove the renderers are clean; this
// one proves no other path around them leaks, because a user pointing the tool
// at a production database gets this composition, not the layers.
func TestNoUserDataInAnyStream(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		args    []string
	}{
		{"audit table", "not_valid_constraints", []string{"audit"}},
		{"audit json", "not_valid_constraints", []string{"audit", "--format", "json"}},
		{"discover table", "inferable", []string{"discover", "--include-rejected"}},
		{"discover json", "inferable", []string{"discover", "--format", "json"}},
		{"discover with prefilter", "stats_prefilter", []string{"discover", "--include-rejected"}},
		{"discover json with prefilter", "stats_prefilter", []string{"discover", "--format", "json"}},
		{"discover validated", "validation", []string{"discover", "--full", "--include-rejected"}},
		{"discover validated json", "validation", []string{"discover", "--format", "json"}},
		{"discover with usage evidence", "usage_evidence", []string{"discover", "--full", "--include-rejected"}},
		{"discover usage evidence json", "usage_evidence", []string{"discover", "--full", "--format", "json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := testutil.Postgres(t, tc.fixture)

			var out, errOut bytes.Buffer
			streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader("")}

			// Debug is the most talkative level the tool has; if a value can
			// reach a log line, this is where it surfaces.
			args := append(tc.args, "--dsn", dsn, "--log-level", "debug", "--color", "never")

			if code := cli.Run(args, streams); code != 0 {
				t.Fatalf("exit code %d, stderr:\n%s", code, errOut.String())
			}

			for name, text := range map[string]string{
				"stdout": out.String(),
				"stderr": errOut.String(),
			} {
				for _, planted := range plantedValues {
					if strings.Contains(text, planted) {
						t.Errorf("%s leaked %q from a user table", name, planted)
					}
				}
			}
		})
	}
}

// TestNoStatsFlagRemovesTheLayer proves the prefilter is switchable as a
// whole: a penalty derived from estimates is only auditable if a run with and
// without it can be compared.
func TestNoStatsFlagRemovesTheLayer(t *testing.T) {
	dsn := testutil.Postgres(t, "stats_prefilter")

	discover := func(extra ...string) *model.Result {
		t.Helper()

		var out, errOut bytes.Buffer
		streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader("")}

		args := append([]string{"discover", "--format", "json", "--dsn", dsn, "--color", "never"}, extra...)
		if code := cli.Run(args, streams); code != 0 {
			t.Fatalf("exit code %d, stderr:\n%s", code, errOut.String())
		}

		var res model.Result
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("decoding JSON output: %v", err)
		}
		return &res
	}

	withStats := discover()
	if !withStats.Coverage.StatsPrefilter {
		t.Error("coverage must declare the prefilter ran")
	}
	if withStats.Coverage.CandidatesStatsRejected == 0 {
		t.Error("the arithmetically impossible candidate should be rejected")
	}

	without := discover("--no-stats")
	if without.Coverage.StatsPrefilter {
		t.Error("coverage must declare the prefilter did not run")
	}
	for _, c := range without.Candidates {
		for _, kind := range []model.SignalKind{
			model.SigCardViolation, model.SigRangeViolation, model.SigStatsUnavailable,
		} {
			if c.HasSignal(kind) {
				t.Errorf("%s carries %s with the prefilter off", c.Child, kind)
			}
		}
	}
}

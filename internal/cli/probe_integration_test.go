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

func discoverJSON(t *testing.T, dsn string, extra ...string) *model.Result {
	t.Helper()

	var out, errOut bytes.Buffer
	streams := &cli.Streams{Out: &out, Err: &errOut, In: strings.NewReader("")}

	args := append([]string{"discover", "--format", "json", "--full",
		"--dsn", dsn, "--color", "never"}, extra...)
	if code := cli.Run(args, streams); code != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", code, errOut.String())
	}

	var res model.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decoding JSON output: %v", err)
	}
	return &res
}

func candidate(res *model.Result, child string) (model.Candidate, bool) {
	for _, c := range res.Candidates {
		if c.Child.String() == child {
			return c, true
		}
	}
	return model.Candidate{}, false
}

// TestProbeTurnsAnInvisibleRelationshipIntoAConfirmedOne is what phase 6
// exists to demonstrate: resp_tecnico reaches funcionario through no naming
// convention in any language, and the view the database already stores is what
// makes the relationship findable — then provable.
func TestProbeTurnsAnInvisibleRelationshipIntoAConfirmedOne(t *testing.T) {
	dsn := testutil.Postgres(t, "usage_evidence")

	without := discoverJSON(t, dsn, "--no-probe")
	if _, found := candidate(without, "public.os_servico.resp_tecnico"); found {
		t.Fatal("the fixture is broken: name matching must not reach this relationship")
	}

	with := discoverJSON(t, dsn)
	c, found := candidate(with, "public.os_servico.resp_tecnico")
	if !found {
		t.Fatal("usage evidence did not surface the relationship")
	}
	if c.Verdict != model.VerdictConfirmed {
		t.Errorf("verdict = %q (%s), want confirmed: the fixture data is contained",
			c.Verdict, c.Reason)
	}
	if !c.HasSignal(model.SigJoinInView) {
		t.Error("the candidate must record which evidence produced it")
	}
}

func TestProbeReinforcesWhatNameAlreadyFound(t *testing.T) {
	dsn := testutil.Postgres(t, "usage_evidence")

	without, ok := candidate(discoverJSON(t, dsn, "--no-probe"), "public.os_servico.cliente_id")
	if !ok {
		t.Fatal("cliente_id should be reachable by name alone")
	}
	with, ok := candidate(discoverJSON(t, dsn), "public.os_servico.cliente_id")
	if !ok {
		t.Fatal("cliente_id disappeared with the probe on")
	}

	if with.MetaScore <= without.MetaScore {
		t.Errorf("score with evidence %.2f must exceed %.2f without",
			with.MetaScore, without.MetaScore)
	}
}

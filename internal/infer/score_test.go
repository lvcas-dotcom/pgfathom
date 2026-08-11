package infer_test

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// scoreOf builds a one-column schema around the given shape and returns the
// score of the resulting candidate.
func scoreOf(t *testing.T, parent model.Table, child model.Table, column string) float64 {
	t.Helper()

	res := infer.Generate(schema(parent, child), infer.Options{Profile: ptBR(t), MinScore: 0.001})

	for _, c := range append(res.Candidates, res.Discarded...) {
		if c.Child.Columns[0] == column {
			return c.MetaScore
		}
	}
	t.Fatalf("no candidate generated for %s", column)
	return 0
}

// The tests here pin relative ordering rather than absolute values. The weights
// are judgment and the first set will be wrong; what has to stay true is that an
// exact match outranks a normalized one, not that exact is worth 0.30.

func TestExactNameOutranksNormalized(t *testing.T) {
	exact := scoreOf(t, tbl("cliente"), tbl("pedido", col("cliente_id")), "cliente_id")
	normalized := scoreOf(t, tbl("tb_clientes"), tbl("pedido", col("cliente_id")), "cliente_id")

	if exact <= normalized {
		t.Errorf("exact %.2f must outrank normalized %.2f", exact, normalized)
	}
}

func TestIdenticalTypeOutranksMerelyCompatible(t *testing.T) {
	identical := scoreOf(t, tbl("cliente"), tbl("pedido", col("cliente_id")), "cliente_id")

	narrower := tbl("pedido", typed("cliente_id", "int4"))
	compatible := scoreOf(t, tbl("cliente"), narrower, "cliente_id")

	if identical <= compatible {
		t.Errorf("identical %.2f must outrank compatible %.2f", identical, compatible)
	}
}

func TestIndexedChildOutranksUnindexed(t *testing.T) {
	unindexed := tbl("pedido", col("cliente_id"))

	indexed := tbl("pedido", col("cliente_id"))
	indexed.Indexes = []model.Index{{Name: "ix", Columns: []string{"cliente_id"}}}

	if a, b := scoreOf(t, tbl("cliente"), indexed, "cliente_id"),
		scoreOf(t, tbl("cliente"), unindexed, "cliente_id"); a <= b {
		t.Errorf("indexed %.2f must outrank unindexed %.2f: an index means somebody joins on it", a, b)
	}
}

func TestCommentMentionRaisesTheScore(t *testing.T) {
	plain := tbl("pedido", col("cliente_id"))

	commented := tbl("pedido", model.Column{
		Name: "cliente_id", Type: "bigint", BaseType: "int8",
		Comment: "referencia o cliente responsavel",
	})

	if a, b := scoreOf(t, tbl("cliente"), commented, "cliente_id"),
		scoreOf(t, tbl("cliente"), plain, "cliente_id"); a <= b {
		t.Errorf("a comment naming the target %.2f must outrank none %.2f", a, b)
	}
}

func TestAmbiguityLowersTheScore(t *testing.T) {
	unique := scoreOf(t, tbl("cliente"), tbl("pedido", col("cliente_id")), "cliente_id")

	schemas := []model.Schema{
		{Name: "public", Tables: []model.Table{tbl("cliente"), tbl("pedido", col("cliente_id"))}},
		{Name: "legado", Tables: []model.Table{{
			Schema: "legado", Name: "cliente",
			Columns: []model.Column{col("id")}, PrimaryKey: []string{"id"},
			Stats: model.TableStats{EstimatedRows: 100_000},
		}}},
	}

	res := infer.Generate(schemas, infer.Options{Profile: ptBR(t), MinScore: 0.001})
	all := append(res.Candidates, res.Discarded...)
	if len(all) == 0 {
		t.Fatal("no candidates generated")
	}

	if all[0].MetaScore >= unique {
		t.Errorf("an ambiguous target %.2f must score below an unambiguous one %.2f",
			all[0].MetaScore, unique)
	}
}

// TestScoreSaturates keeps the threshold meaningful. Free summation would make
// the range depend on how many signals happened to fire, so 0.5 would mean
// different things in a schema with rich comments and one with none.
func TestScoreSaturates(t *testing.T) {
	loaded := tbl("pedido", model.Column{
		Name: "cliente_id", Type: "bigint", BaseType: "int8",
		Comment: "cliente", Nullable: false,
	})
	loaded.Indexes = []model.Index{{Name: "ix", Columns: []string{"cliente_id"}}}
	loaded.Comment = "cliente"

	if got := scoreOf(t, tbl("cliente"), loaded, "cliente_id"); got > 1 {
		t.Errorf("score = %.2f, want at most 1", got)
	}

	// Every penalty at once, and no positive worth speaking of.
	status := tbl("tb_status")
	status.Stats.EstimatedRows = 3

	schemas := []model.Schema{
		{Name: "public", Tables: []model.Table{status, tbl("pedido", typed("status_id", "int4"))}},
		{Name: "legado", Tables: []model.Table{{
			Schema: "legado", Name: "tb_status",
			Columns: []model.Column{col("id")}, PrimaryKey: []string{"id"},
			Stats: model.TableStats{EstimatedRows: 3},
		}}},
	}

	res := infer.Generate(schemas, infer.Options{Profile: ptBR(t), MinScore: -1})
	for _, c := range append(res.Candidates, res.Discarded...) {
		if c.MetaScore < 0 {
			t.Errorf("score = %.2f, want at least 0", c.MetaScore)
		}
	}
}

// TestScoreIsReconstructibleFromSignals enforces that no scoring path can leave
// the facts behind: a number without provenance is not auditable, and the whole
// tool depends on a user being able to disagree with a score by reading what
// produced it.
func TestScoreIsReconstructibleFromSignals(t *testing.T) {
	res := generate(t, schema(
		tbl("cliente"), tbl("municipio"),
		tbl("pedido", col("cliente_id"), col("municipio_id")),
	), func(o *infer.Options) { o.MinScore = 0.001 })

	for _, c := range append(res.Candidates, res.Discarded...) {
		sum := 0.0
		for _, s := range c.Signals {
			sum += s.Weight
		}
		if sum < 0 {
			sum = 0
		}
		if sum > 1 {
			sum = 1
		}

		if diff := sum - c.MetaScore; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: signals sum to %.4f but the score is %.4f", c.Child, sum, c.MetaScore)
		}
		if len(c.Signals) == 0 {
			t.Errorf("%s: a score with no signals behind it is not auditable", c.Child)
		}
	}
}

// TestTrapScenario is the case the spec singles out: a column named status_id in
// a schema that has a status table, where the column in fact holds something
// else. Inference cannot know that, so the requirement is not that it reject —
// it is that the candidate carry the penalty that keeps it out of the way until
// the data settles it.
func TestTrapScenario(t *testing.T) {
	status := tbl("status")
	status.Stats.EstimatedRows = 4

	pedido := tbl("pedido", col("status_id"), col("cliente_id"))

	res := generate(t, schema(status, tbl("cliente"), pedido))

	trap, ok := find(res, "public.pedido.status_id", "public.status.id")
	if !ok {
		t.Fatal("the trap candidate should exist: it is a plausible hypothesis")
	}
	if !trap.HasSignal(model.SigGenericDomain) {
		t.Error("the trap must carry the generic-domain penalty")
	}

	real, ok := find(res, "public.pedido.cliente_id", "public.cliente.id")
	if !ok {
		t.Fatal("the genuine candidate was not generated")
	}

	if trap.MetaScore >= real.MetaScore {
		t.Errorf("the trap %.2f must rank below the genuine relationship %.2f",
			trap.MetaScore, real.MetaScore)
	}
}

// TestRescoreReproducesGeneration pins the single-owner rule for the
// combination: a layer that appends signals after generation recomposes
// through Rescore, and that path must agree exactly with what generation
// produced from the same signals.
func TestRescoreReproducesGeneration(t *testing.T) {
	res := generate(t, schema(
		tbl("cliente"),
		tbl("municipio"),
		tbl("pedido", col("cliente_id"), col("municipio_id")),
	))

	for _, c := range append(append([]model.Candidate{}, res.Candidates...), res.Discarded...) {
		got := c
		infer.Rescore(&got)
		if got.MetaScore != c.MetaScore {
			t.Errorf("%s: rescore = %.4f, generation = %.4f; the two paths must agree",
				c.Child, got.MetaScore, c.MetaScore)
		}
	}
}

// TestRescoreSaturatesAtZero pins the floor: accumulated penalties never push
// a recomposed score negative, so the threshold keeps its meaning.
func TestRescoreSaturatesAtZero(t *testing.T) {
	c := model.Candidate{
		Signals: []model.Signal{
			{Kind: model.SigNormalizedName, Weight: 0.15},
			{Kind: model.SigCardViolation, Weight: -0.40},
			{Kind: model.SigRangeViolation, Weight: -0.30},
		},
	}
	infer.Rescore(&c)
	if c.MetaScore != 0 {
		t.Errorf("score = %.4f, want zero: saturation holds on recomposition", c.MetaScore)
	}
}

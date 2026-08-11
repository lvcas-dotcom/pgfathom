package infer_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func ref(table, column string) model.ColumnRef {
	return model.ColumnRef{Schema: "public", Table: table, Column: column}
}

func withEvidence(ev ...model.JoinEvidence) func(*infer.Options) {
	return func(o *infer.Options) { o.Evidence = ev }
}

// TestEvidenceCreatesTheUnreachableCandidate is the reason the phase exists:
// resp_tecnico bears no resemblance to funcionario, and only the join a view
// executes can produce the hypothesis.
func TestEvidenceCreatesTheUnreachableCandidate(t *testing.T) {
	tables := schema(
		tbl("funcionario"),
		tbl("os_servico", col("resp_tecnico")),
	)

	base := generate(t, tables)
	if _, found := find(base, "public.os_servico.resp_tecnico", "public.funcionario.id"); found {
		t.Fatal("the fixture is broken: the pair must be unreachable by name")
	}

	res := generate(t, tables, withEvidence(model.JoinEvidence{
		Left:   ref("os_servico", "resp_tecnico"),
		Right:  ref("funcionario", "id"),
		Source: model.JoinFromView,
		Object: "public.vw_os",
	}))

	c, found := find(res, "public.os_servico.resp_tecnico", "public.funcionario.id")
	if !found {
		t.Fatal("the evidence-born candidate was not created")
	}
	if !c.HasSignal(model.SigJoinInView) {
		t.Error("the candidate must carry the join-in-view signal")
	}
	if c.MetaScore < infer.DefaultMinScore {
		t.Errorf("score %.2f fell below the default threshold: usage evidence plus an "+
			"identical type must be enough to investigate", c.MetaScore)
	}
}

func TestEvidenceReinforcesTheNameBornCandidate(t *testing.T) {
	tables := schema(
		tbl("cliente"),
		tbl("pedido", col("cliente_id")),
	)
	ev := model.JoinEvidence{
		Left:   ref("pedido", "cliente_id"),
		Right:  ref("cliente", "id"),
		Source: model.JoinFromFunction,
		Object: "public.fn_relatorio",
	}

	without, _ := find(generate(t, tables), "public.pedido.cliente_id", "public.cliente.id")
	with, found := find(generate(t, tables, withEvidence(ev)), "public.pedido.cliente_id", "public.cliente.id")

	if !found {
		t.Fatal("the name-born candidate disappeared")
	}
	if !with.HasSignal(model.SigJoinInFunction) {
		t.Error("the reinforced candidate must carry the join signal")
	}
	if with.MetaScore <= without.MetaScore {
		t.Errorf("score with evidence %.2f must exceed %.2f without", with.MetaScore, without.MetaScore)
	}
}

// TestSameEvidenceKindCountsOnce pins the volume rule: three views proving the
// same join are one fact, and stacking them would let volume impersonate
// strength.
func TestSameEvidenceKindCountsOnce(t *testing.T) {
	tables := schema(
		tbl("cliente"),
		tbl("pedido", col("cliente_id")),
	)
	one := model.JoinEvidence{
		Left: ref("pedido", "cliente_id"), Right: ref("cliente", "id"),
		Source: model.JoinFromView, Object: "public.vw_a",
	}
	other := one
	other.Object = "public.vw_b"

	single, _ := find(generate(t, tables, withEvidence(one)), "public.pedido.cliente_id", "public.cliente.id")
	double, _ := find(generate(t, tables, withEvidence(one, other)), "public.pedido.cliente_id", "public.cliente.id")

	if single.MetaScore != double.MetaScore {
		t.Errorf("two views scored %.2f against %.2f for one: the same kind must count once",
			double.MetaScore, single.MetaScore)
	}
}

func TestPairWithoutKeyAnchorCreatesNothing(t *testing.T) {
	tables := schema(
		tbl("pedido", col("observacao_id")),
		tbl("nota", col("comentario_id")),
	)

	res := generate(t, tables, withEvidence(model.JoinEvidence{
		Left:   ref("pedido", "observacao_id"),
		Right:  ref("nota", "comentario_id"),
		Source: model.JoinFromView,
		Object: "public.vw_x",
	}))

	for _, c := range append(append([]model.Candidate{}, res.Candidates...), res.Discarded...) {
		if c.HasSignal(model.SigJoinInView) {
			t.Errorf("candidate %s → %s was born without a key anchor", c.Child, c.Parent)
		}
	}
}

func TestBothSidesKeyedGenerateBothDirections(t *testing.T) {
	tables := schema(
		tbl("pessoa"),
		tbl("usuario"),
	)

	res := generate(t, tables, withEvidence(model.JoinEvidence{
		Left:   ref("pessoa", "id"),
		Right:  ref("usuario", "id"),
		Source: model.JoinFromView,
		Object: "public.vw_login",
	}))

	// Both columns are their tables' primary keys, so neither side is eligible
	// as a child — a PK column referencing elsewhere is out of scope, exactly
	// as in name generation. What must NOT happen is a crash or a one-sided
	// guess; the pair simply creates nothing.
	for _, c := range append(append([]model.Candidate{}, res.Candidates...), res.Discarded...) {
		if c.HasSignal(model.SigJoinInView) {
			t.Errorf("candidate %s → %s: a PK-to-PK pair must not become a candidate", c.Child, c.Parent)
		}
	}
}

func TestGenerationWithEvidenceIsDeterministic(t *testing.T) {
	tables := schema(
		tbl("funcionario"),
		tbl("cliente"),
		tbl("os_servico", col("resp_tecnico"), col("cliente_id")),
	)
	evidence := withEvidence(model.JoinEvidence{
		Left:   ref("os_servico", "resp_tecnico"),
		Right:  ref("funcionario", "id"),
		Source: model.JoinFromView,
		Object: "public.vw_os",
	})

	first := generate(t, tables, evidence)
	for range 10 {
		if diff := cmp.Diff(first, generate(t, tables, evidence)); diff != "" {
			t.Fatalf("generation is not deterministic (-first +again):\n%s", diff)
		}
	}
}

// TestCompositeJoinBecomesOneCandidate is the composite counterpart of the
// unreachable case: read one equality at a time, each half of the key fails the
// anchor and the relationship vanishes exactly where the evidence is strongest.
func TestCompositeJoinBecomesOneCandidate(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	// Neither child column resembles the target, so only the view can reach it.
	remessa := tbl("remessa", col("org"), col("doc"))

	res := generate(t, schema(nota, remessa), withEvidence(
		model.JoinEvidence{
			Left:   ref("remessa", "org"),
			Right:  ref("nota", "empresa_id"),
			Source: model.JoinFromView, Object: "public.vw_remessa",
		},
		model.JoinEvidence{
			Left:   ref("remessa", "doc"),
			Right:  ref("nota", "numero"),
			Source: model.JoinFromView, Object: "public.vw_remessa",
		},
	))

	c, found := find(res, "public.remessa.(org, doc)", "public.nota.(empresa_id, numero)")
	if !found {
		t.Fatalf("the composite join must produce one candidate; got %d survivors", len(res.Candidates))
	}
	if !c.HasSignal(model.SigJoinInView) || !c.HasSignal(model.SigCompositeArity) {
		t.Errorf("the candidate must carry both the join and the arity: %v", c.Signals)
	}

	for _, other := range res.Candidates {
		if other.Child.TableRef() == "public.remessa" && !other.Child.Composite() {
			t.Errorf("half a key is not a hypothesis: %s → %s", other.Child, other.Parent)
		}
	}
}

func TestPartialCompositeJoinAnchorsNothing(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	remessa := tbl("remessa", col("org"))

	res := generate(t, schema(nota, remessa), withEvidence(model.JoinEvidence{
		Left:   ref("remessa", "org"),
		Right:  ref("nota", "empresa_id"),
		Source: model.JoinFromView, Object: "public.vw_remessa",
	}))

	for _, c := range res.Candidates {
		if c.Parent.TableRef() == "public.nota" {
			t.Errorf("a join covering one of two key columns must anchor nothing: %s → %s", c.Child, c.Parent)
		}
	}
}

// TestCompositeEvidenceStrengthensTheNamedCandidate keeps one signal per
// origin: the view proves what the name already suggested, it does not double it.
func TestCompositeEvidenceStrengthensTheNamedCandidate(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	item := keyed("item", []string{"empresa_id", "numero", "sequencia"})

	byName := generate(t, schema(nota, item))
	named, found := find(byName, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)")
	if !found {
		t.Fatal("the fixture is broken: the pair must be reachable by name")
	}

	res := generate(t, schema(nota, item), withEvidence(
		model.JoinEvidence{
			Left: ref("item", "empresa_id"), Right: ref("nota", "empresa_id"),
			Source: model.JoinFromView, Object: "public.vw_item",
		},
		model.JoinEvidence{
			Left: ref("item", "numero"), Right: ref("nota", "numero"),
			Source: model.JoinFromView, Object: "public.vw_item",
		},
	))

	c, found := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)")
	if !found {
		t.Fatal("the candidate disappeared once evidence was added")
	}
	if c.MetaScore <= named.MetaScore {
		t.Errorf("score = %.2f with evidence and %.2f without: usage has to add", c.MetaScore, named.MetaScore)
	}

	seen := 0
	for _, s := range c.Signals {
		if s.Kind == model.SigJoinInView {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the view signal appears %d times; one object proving a join is one fact", seen)
	}
}

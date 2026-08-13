package infer_test

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestSimilarityFallbackGeneratesWhenAffixFindsNothing(t *testing.T) {
	res := generate(t, schema(
		tbl("operadorbasecalculo"),
		tbl("basecalculo", col("operador_id")),
	))

	c, ok := find(res, "public.basecalculo.operador_id", "public.operadorbasecalculo.id")
	if !ok {
		t.Fatalf("expected candidate not generated; got %d survivors, %d discarded",
			len(res.Candidates), len(res.Discarded))
	}
	if !c.HasSignal(model.SigNameSimilarity) {
		t.Error("operador_id matching operadorbasecalculo should carry the name-similarity signal")
	}
	if c.HasSignal(model.SigExactName) || c.HasSignal(model.SigNormalizedName) {
		t.Error("a candidate raised by the similarity fallback should not also carry a profile-match signal")
	}
}

func TestSimilarityBelowCutoffGeneratesNothing(t *testing.T) {
	res := generate(t, schema(
		tbl("cliente"),
		tbl("pedido", col("xyzabc_id")),
	))

	if _, ok := find(res, "public.pedido.xyzabc_id", "public.cliente.id"); ok {
		t.Error("entity with no lexical overlap to any table should never raise a candidate")
	}
}

func TestAffixMatchSuppressesSimilarityFallback(t *testing.T) {
	// clientefiel is lexically close to cliente but never checked: cliente
	// itself already answers the profile index exactly, so the fallback is
	// never evaluated for this column.
	res := generate(t, schema(
		tbl("cliente"),
		tbl("clientefiel"),
		tbl("pedido", col("cliente_id")),
	))

	c, ok := find(res, "public.pedido.cliente_id", "public.cliente.id")
	if !ok {
		t.Fatal("exact affix match should still generate a candidate")
	}
	if !c.HasSignal(model.SigExactName) {
		t.Error("cliente_id matching cliente should carry the exact-name signal, unaffected by this change")
	}

	if _, ok := find(res, "public.pedido.cliente_id", "public.clientefiel.id"); ok {
		t.Error("similarity fallback must not run when the profile index already found a table")
	}
}

func TestSimilarityFallbackHandlesAmbiguity(t *testing.T) {
	res := generate(t, schema(
		tbl("fornecedorpj"),
		tbl("fornecedorpf"),
		tbl("pedido", col("fornecedor_id")),
	))

	pj, ok := find(res, "public.pedido.fornecedor_id", "public.fornecedorpj.id")
	if !ok {
		t.Fatal("expected candidate towards fornecedorpj not generated")
	}
	pf, ok := find(res, "public.pedido.fornecedor_id", "public.fornecedorpf.id")
	if !ok {
		t.Fatal("expected candidate towards fornecedorpf not generated")
	}

	if !pj.HasSignal(model.SigAmbiguousTarget) || !pf.HasSignal(model.SigAmbiguousTarget) {
		t.Error("both candidates from an ambiguous similarity match should carry the ambiguous-target signal")
	}
}

// TestSimilarityFallbackReproducesCorpusMiss mirrors, in the synthetic format
// the rest of this file already uses, the real gap documented in
// docs/PGFATHOM.md — atotramite.tptramite_idkey -> tramitetipo, abbreviated
// and reordered. The real column uses the _idkey suffix, which the embedded
// pt-br profile does not ship (it is learned per schema by naming detection,
// out of scope for this unit test) — substituting the recognized _id suffix
// isolates the similarity mechanism this change adds without depending on
// detection.
func TestSimilarityFallbackReproducesCorpusMiss(t *testing.T) {
	res := generate(t, schema(
		tbl("tramitetipo"),
		tbl("atotramite", col("tptramite_id")),
	))

	c, ok := find(res, "public.atotramite.tptramite_id", "public.tramitetipo.id")
	if !ok {
		t.Fatalf("expected candidate not generated; got %d survivors, %d discarded",
			len(res.Candidates), len(res.Discarded))
	}
	if !c.HasSignal(model.SigNameSimilarity) {
		t.Error("tptramite_id matching tramitetipo should carry the name-similarity signal")
	}
}

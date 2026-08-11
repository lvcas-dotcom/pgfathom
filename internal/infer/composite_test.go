package infer_test

import (
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// keyed builds a table whose primary key spans the given columns, which is the
// shape this whole file is about. Unlike tbl it adds no surrogate id.
func keyed(name string, key []string, columns ...model.Column) model.Table {
	cols := make([]model.Column, 0, len(key)+len(columns))
	for _, k := range key {
		cols = append(cols, col(k))
	}
	return model.Table{
		Schema:     "public",
		Name:       name,
		Columns:    append(cols, columns...),
		PrimaryKey: key,
		Stats:      model.TableStats{EstimatedRows: 100_000},
	}
}

func skipFor(res *infer.Result, child, target string) (infer.Skip, bool) {
	for _, s := range res.Skipped {
		if s.Child.String() == child && s.Target == target {
			return s, true
		}
	}
	return infer.Skip{}, false
}

func TestMirrorDerivationMatches(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	item := keyed("item", []string{"empresa_id", "numero", "sequencia"})

	res := generate(t, schema(nota, item))

	c, ok := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)")
	if !ok {
		t.Fatalf("a child carrying the target's own key columns must be a candidate; got %d survivors",
			len(res.Candidates))
	}
	if !c.HasSignal(model.SigCompositeArity) {
		t.Error("the whole evidence of a mirror match is the arity agreement, and it must be on the record")
	}
	if c.HasSignal(model.SigExactName) || c.HasSignal(model.SigNormalizedName) {
		t.Error("a mirror match carries no name evidence and must claim none")
	}
}

func TestPrefixedDerivationMatches(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	rateio := tbl("rateio", col("nota_empresa_id"), col("nota_numero"))

	res := generate(t, schema(nota, rateio))

	c, ok := find(res, "public.rateio.(nota_empresa_id, nota_numero)", "public.nota.(empresa_id, numero)")
	if !ok {
		t.Fatalf("the target's name prefixing each key column must match; got %d survivors",
			len(res.Candidates))
	}
	if !c.HasSignal(model.SigExactName) {
		t.Error("a prefixed match is anchored by the target's name and must say so")
	}
}

func TestMixedDerivationDoesNotMatch(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	// One position by mirror, the other only by prefix: the shape of two common
	// column names sharing a table, not of a key.
	frete := tbl("frete", col("empresa_id"), col("nota_numero"))

	res := generate(t, schema(nota, frete))

	for _, c := range res.Candidates {
		if c.Child.Composite() {
			t.Errorf("mixed derivations must not produce %s", c.Child)
		}
	}
}

func TestPartialMatchIsObservedNeverGenerated(t *testing.T) {
	contrato := keyed("contrato", []string{"empresa_id", "ano", "numero"})
	aditivo := tbl("aditivo", col("empresa_id"), col("ano"))

	res := generate(t, schema(contrato, aditivo))

	for _, c := range res.Candidates {
		if c.Parent.TableRef() == "public.contrato" {
			t.Errorf("two of three positions is not a key: %s", c.Child)
		}
	}

	s, ok := skipFor(res, "public.aditivo.(empresa_id, ano)", "public.contrato")
	if !ok {
		t.Fatalf("a near miss must be observed, never silent; skips: %v", res.Skipped)
	}
	if s.Reason != infer.SkipPartialKey {
		t.Errorf("reason = %q, want the partial-key reason", s.Reason)
	}
	if !strings.Contains(s.Detail, "2 of 3") {
		t.Errorf("detail = %q, want the fraction that matched", s.Detail)
	}
}

func TestSharedKeySignatureIsSkippedNotGuessed(t *testing.T) {
	// Two tables answering to the same key, and nothing in the child naming
	// either. Both could reach total containment, and confirming both would
	// confirm one relationship that does not exist.
	res := generate(t, schema(
		keyed("empresa_filial", []string{"empresa_id", "filial_id"}),
		keyed("filial_ativa", []string{"empresa_id", "filial_id"}),
		tbl("pedido", col("empresa_id"), col("filial_id")),
	))

	for _, c := range res.Candidates {
		if c.Child.Composite() && c.Child.TableRef() == "public.pedido" {
			t.Errorf("an unanchored ambiguous signature must not be guessed: %s → %s", c.Child, c.Parent)
		}
	}

	s, ok := skipFor(res, "public.pedido.(empresa_id, filial_id)", "public.empresa_filial")
	if !ok {
		t.Fatalf("the ambiguity must be recorded; skips: %v", res.Skipped)
	}
	if s.Reason != infer.SkipAmbiguousSignature {
		t.Errorf("reason = %q, want the ambiguous-signature reason", s.Reason)
	}
}

func TestOneIncompatiblePositionSinksTheKey(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	// numero as text against a bigint key: numeric never matches textual.
	item := tbl("item", col("empresa_id"), typed("numero", "text"))

	res := generate(t, schema(nota, item))

	if c, ok := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)"); ok {
		t.Errorf("one incompatible position must sink the whole key; got %v", c.Signals)
	}
}

// TestArityOutscoresASinglePosition compares like with like: both candidates
// are anchored by the target's name, and only the arity differs.
func TestArityOutscoresASinglePosition(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	empresa := tbl("empresa")

	res := generate(t, schema(nota, empresa,
		tbl("rateio", col("nota_empresa_id"), col("nota_numero"), col("empresa_id"))))

	composite, ok := find(res, "public.rateio.(nota_empresa_id, nota_numero)", "public.nota.(empresa_id, numero)")
	if !ok {
		t.Fatal("the composite candidate is missing")
	}
	single, ok := find(res, "public.rateio.empresa_id", "public.empresa.id")
	if !ok {
		t.Fatal("the single-column candidate is missing")
	}

	if composite.MetaScore <= single.MetaScore {
		t.Errorf("composite scored %.2f and single %.2f: a whole key agreeing at once is stronger",
			composite.MetaScore, single.MetaScore)
	}
}

// TestMirrorScoresBelowANamedSinglePosition holds the honest ordering: two
// columns lining up with nothing naming the target is weaker evidence than one
// column whose name says where it points.
func TestMirrorScoresBelowANamedSinglePosition(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	empresa := tbl("empresa")

	res := generate(t, schema(nota, empresa, tbl("item", col("empresa_id"), col("numero"))))

	mirror, ok := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)")
	if !ok {
		t.Fatal("the mirror candidate is missing")
	}
	named, ok := find(res, "public.item.empresa_id", "public.empresa.id")
	if !ok {
		t.Fatal("the single-column candidate is missing")
	}

	if mirror.MetaScore >= named.MetaScore {
		t.Errorf("mirror scored %.2f and the named single %.2f: a match with no name evidence must not outrank one with it",
			mirror.MetaScore, named.MetaScore)
	}
}

func TestArityIsOneSignalNotOnePerColumn(t *testing.T) {
	wide := keyed("wide", []string{"a_id", "b_id", "c_id", "d_id"})
	child := tbl("child", col("a_id"), col("b_id"), col("c_id"), col("d_id"))

	res := generate(t, schema(wide, child))

	c, ok := find(res, "public.child.(a_id, b_id, c_id, d_id)", "public.wide.(a_id, b_id, c_id, d_id)")
	if !ok {
		t.Fatal("the four-column candidate is missing")
	}

	counts := make(map[model.SignalKind]int)
	for _, s := range c.Signals {
		counts[s.Kind]++
	}
	for kind, n := range counts {
		if n > 1 {
			t.Errorf("signal %s appears %d times: a four-column key must not restate one fact four times",
				kind, n)
		}
	}
}

func TestIdentifyingRelationshipIsReached(t *testing.T) {
	// The reason most composite keys exist: the child is keyed on the parent's
	// key plus a discriminator. Excluding key members from candidacy would make
	// this whole class invisible.
	nota := keyed("nota", []string{"empresa_id", "numero"})
	item := keyed("item", []string{"empresa_id", "numero", "sequencia"})

	res := generate(t, schema(nota, item))

	if _, ok := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)"); !ok {
		t.Error("a child keyed on the parent's key plus a discriminator must be reachable")
	}
}

func TestTableDoesNotMatchItsOwnKey(t *testing.T) {
	res := generate(t, schema(keyed("nota", []string{"empresa_id", "numero"})))

	for _, c := range res.Candidates {
		if c.Child.TableRef() == c.Parent.TableRef() && c.Child.Composite() {
			t.Errorf("a table matching its own key is a tautology: %s → %s", c.Child, c.Parent)
		}
	}
}

func TestDeclaredKeyDisqualifiesTheWholeSet(t *testing.T) {
	nota := keyed("nota", []string{"empresa_id", "numero"})
	item := tbl("item", col("empresa_id"), col("numero"))
	item.ForeignKeys = []model.ForeignKey{{
		Name: "fk_item_nota", Columns: []string{"empresa_id", "numero"},
		RefSchema: "public", RefTable: "nota", RefColumns: []string{"empresa_id", "numero"},
		Validated: true,
	}}

	res := generate(t, schema(nota, item))

	if _, ok := find(res, "public.item.(empresa_id, numero)", "public.nota.(empresa_id, numero)"); ok {
		t.Error("a relationship already in the catalog needs no inference")
	}
}

func TestCompositeGenerationIsDeterministic(t *testing.T) {
	schemas := schema(
		keyed("nota", []string{"empresa_id", "numero"}),
		keyed("item", []string{"empresa_id", "numero", "sequencia"}),
		tbl("rateio", col("nota_empresa_id"), col("nota_numero")),
	)

	first := generate(t, schemas)
	for i := 0; i < 3; i++ {
		again := generate(t, schemas)
		if len(again.Candidates) != len(first.Candidates) {
			t.Fatalf("candidate count changed between runs: %d vs %d",
				len(first.Candidates), len(again.Candidates))
		}
		for j := range first.Candidates {
			if first.Candidates[j].Child.String() != again.Candidates[j].Child.String() ||
				first.Candidates[j].Parent.String() != again.Candidates[j].Parent.String() {
				t.Fatalf("ordering changed at %d: %s vs %s",
					j, first.Candidates[j].Child, again.Candidates[j].Child)
			}
		}
	}
}

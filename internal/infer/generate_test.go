package infer_test

import (
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
)

func ptBR(t *testing.T) *profile.Profile {
	t.Helper()
	p, err := profile.Embedded("pt-br")
	if err != nil {
		t.Fatalf("loading profile: %v", err)
	}
	return p
}

// col builds a NOT NULL bigint column, the shape most reference columns have.
func col(name string) model.Column {
	return model.Column{Name: name, Type: "bigint", BaseType: "int8"}
}

func typed(name, base string) model.Column {
	return model.Column{Name: name, Type: base, BaseType: base}
}

func tbl(name string, columns ...model.Column) model.Table {
	return model.Table{
		Schema:     "public",
		Name:       name,
		Columns:    append([]model.Column{col("id")}, columns...),
		PrimaryKey: []string{"id"},
		Stats:      model.TableStats{EstimatedRows: 100_000},
	}
}

func schema(tables ...model.Table) []model.Schema {
	return []model.Schema{{Name: "public", Tables: tables}}
}

func generate(t *testing.T, tables []model.Schema, opts ...func(*infer.Options)) *infer.Result {
	t.Helper()
	o := infer.Options{Profile: ptBR(t)}
	for _, fn := range opts {
		fn(&o)
	}
	return infer.Generate(tables, o)
}

func find(res *infer.Result, child, parent string) (model.Candidate, bool) {
	for _, c := range append(append([]model.Candidate{}, res.Candidates...), res.Discarded...) {
		if c.Child.String() == child && c.Parent.String() == parent {
			return c, true
		}
	}
	return model.Candidate{}, false
}

// survivor finds a candidate among the ones that reached validation only.
// find looks in both lists, which is what most tests want and exactly what a
// test about the threshold must not do.
func survivor(res *infer.Result, child, parent string) (model.Candidate, bool) {
	for _, c := range res.Candidates {
		if c.Child.String() == child && c.Parent.String() == parent {
			return c, true
		}
	}
	return model.Candidate{}, false
}

func discarded(res *infer.Result, child, parent string) (model.Candidate, bool) {
	for _, c := range res.Discarded {
		if c.Child.String() == child && c.Parent.String() == parent {
			return c, true
		}
	}
	return model.Candidate{}, false
}

func TestGeneratesTheObviousCandidate(t *testing.T) {
	res := generate(t, schema(
		tbl("cliente"),
		tbl("pedido", col("cliente_id")),
	))

	c, ok := find(res, "public.pedido.cliente_id", "public.cliente.id")
	if !ok {
		t.Fatalf("expected candidate not generated; got %d survivors", len(res.Candidates))
	}
	if !c.HasSignal(model.SigExactName) {
		t.Error("cliente_id matching cliente should carry the exact-name signal")
	}
	if !c.HasSignal(model.SigIdenticalType) {
		t.Error("int8 to int8 should carry the identical-type signal")
	}
	if c.Verdict != model.VerdictUnvalidated {
		t.Errorf("verdict = %q, want unvalidated: no data was consulted", c.Verdict)
	}
}

func TestNormalizedNameMatchesThroughPrefixAndPlural(t *testing.T) {
	res := generate(t, schema(
		tbl("tb_clientes"),
		tbl("pedido", col("cliente_id")),
	))

	c, ok := find(res, "public.pedido.cliente_id", "public.tb_clientes.id")
	if !ok {
		t.Fatal("cliente_id should reach tb_clientes through the profile forms")
	}
	if c.HasSignal(model.SigExactName) {
		t.Error("a match obtained through normalization must not claim to be exact")
	}
	if !c.HasSignal(model.SigNormalizedName) {
		t.Error("expected the normalized-name signal")
	}
}

func TestColumnWithDeclaredForeignKeyIsSkipped(t *testing.T) {
	pedido := tbl("pedido", col("cliente_id"))
	pedido.ForeignKeys = []model.ForeignKey{{
		Name: "fk", Columns: []string{"cliente_id"},
		RefSchema: "public", RefTable: "cliente", RefColumns: []string{"id"},
		Validated: true,
	}}

	res := generate(t, schema(tbl("cliente"), pedido))

	if _, ok := find(res, "public.pedido.cliente_id", "public.cliente.id"); ok {
		t.Error("a column already covered by a declared foreign key needs no hypothesis")
	}
}

func TestPrimaryKeyOfItsOwnTableIsSkipped(t *testing.T) {
	// A table named "id" would otherwise make every id column a candidate.
	res := generate(t, schema(tbl("id"), tbl("pedido")))

	if _, ok := find(res, "public.pedido.id", "public.id.id"); ok {
		t.Error("a table's own primary key must not become a candidate")
	}
}

func TestIncompatibleTypeBlocksTheCandidate(t *testing.T) {
	cliente := tbl("cliente")
	cliente.Columns[0] = typed("id", "uuid")

	res := generate(t, schema(cliente, tbl("pedido", col("cliente_id"))))

	if _, ok := find(res, "public.pedido.cliente_id", "public.cliente.id"); ok {
		t.Error("int8 must not be offered as a candidate for a uuid key")
	}
}

func TestAmbiguityGeneratesEveryTargetAndPenalizesAll(t *testing.T) {
	schemas := []model.Schema{
		{Name: "public", Tables: []model.Table{tbl("cliente"), tbl("pedido", col("cliente_id"))}},
		{Name: "legado", Tables: []model.Table{{
			Schema: "legado", Name: "cliente",
			Columns:    []model.Column{col("id")},
			PrimaryKey: []string{"id"},
			Stats:      model.TableStats{EstimatedRows: 100_000},
		}}},
	}

	res := infer.Generate(schemas, infer.Options{Profile: ptBR(t), MinScore: 0.01})

	var found int
	for _, c := range append(res.Candidates, res.Discarded...) {
		if c.Child.String() != "public.pedido.cliente_id" {
			continue
		}
		found++
		if !c.HasSignal(model.SigAmbiguousTarget) {
			t.Errorf("candidate for %s should be marked ambiguous", c.Parent)
		}
	}
	if found != 2 {
		t.Errorf("got %d candidates for an ambiguous name, want one per target", found)
	}
}

func TestSingleColumnCannotStandForACompositeKey(t *testing.T) {
	matricula := tbl("matricula")
	matricula.PrimaryKey = []string{"aluno_id", "turma_id"}

	res := generate(t, schema(matricula, tbl("historico", col("matricula_id"))))

	if _, ok := find(res, "public.historico.matricula_id", "public.matricula.id"); ok {
		t.Error("one column cannot stand for a key of two")
	}
	if len(res.Skipped) == 0 {
		t.Fatal("the skip must be recorded, not silent: the relationship may well be real")
	}
	if res.Skipped[0].Reason != infer.SkipArityMismatch {
		t.Errorf("reason = %q, want the arity-mismatch reason", res.Skipped[0].Reason)
	}
}

func TestSelfReferenceIsAllowed(t *testing.T) {
	// Hierarchy through self-reference is common in legacy schemas, and
	// excluding it would lose a whole class of real relationship.
	res := generate(t, schema(tbl("funcionario", col("funcionario_id"))))

	if _, ok := find(res, "public.funcionario.funcionario_id", "public.funcionario.id"); !ok {
		t.Error("a table should be able to reference itself")
	}
}

func TestGenericNameWithSmallTargetIsPenalized(t *testing.T) {
	status := tbl("status")
	status.Stats.EstimatedRows = 6

	res := generate(t, schema(status, tbl("pedido", col("status_id"))))

	c, ok := find(res, "public.pedido.status_id", "public.status.id")
	if !ok {
		t.Fatal("the candidate should still be generated: domain relationships are real")
	}
	if !c.HasSignal(model.SigGenericDomain) {
		t.Error("a generic name pointing at a small table should be penalized")
	}
}

// TestGenericPenaltyRanksButDoesNotCut pins the boundary the penalty must not
// cross. It ranks a boring relationship last; it does not decide the
// relationship is false.
//
// The case is the ordinary one, not a contrived one: a plural domain table.
// An exact name match absorbs the penalty and survives on arithmetic alone
// (0.30+0.25+0.20+0.05-0.30 = 0.50, exactly the threshold), which is why this
// went unnoticed — the bug only bites when the table is plural and the name
// match is therefore normalized rather than exact, at 0.15. That is every
// Rails and Django schema in existence: categories, statuses, types.
func TestGenericPenaltyRanksButDoesNotCut(t *testing.T) {
	tipos := tbl("tipos")
	tipos.Stats.EstimatedRows = 6

	res := generate(t, schema(tipos, tbl("pedido", col("tipo_id"))))

	c, ok := survivor(res, "public.pedido.tipo_id", "public.tipos.id")
	if !ok {
		t.Fatal("a generic name must still reach validation: the penalty says boring, not false")
	}
	if !c.HasSignal(model.SigGenericDomain) {
		t.Fatal("the candidate must carry the penalty; without it this test proves nothing")
	}
	// Without this the test would pass vacuously the day the weights move and
	// the penalized score stops falling below the threshold at all.
	if c.MetaScore >= infer.DefaultMinScore {
		t.Errorf("MetaScore = %.2f, which already clears the %.2f threshold: "+
			"this case no longer exercises the exemption",
			c.MetaScore, infer.DefaultMinScore)
	}
}

// TestGenericPenaltyStillSortsLast is the other half: exempting the penalty
// from the cut must not exempt it from the ordering, which is the whole reason
// it exists.
func TestGenericPenaltyStillSortsLast(t *testing.T) {
	tipos := tbl("tipos")
	tipos.Stats.EstimatedRows = 6

	res := generate(t, schema(
		tipos,
		tbl("cliente"),
		tbl("pedido", col("tipo_id"), col("cliente_id")),
	))

	boring, ok := survivor(res, "public.pedido.tipo_id", "public.tipos.id")
	if !ok {
		t.Fatal("the domain candidate should have survived the cut")
	}
	interesting, ok := survivor(res, "public.pedido.cliente_id", "public.cliente.id")
	if !ok {
		t.Fatal("the ordinary candidate should have survived the cut")
	}
	if boring.MetaScore >= interesting.MetaScore {
		t.Errorf("domain candidate scores %.2f against %.2f: the penalty has to keep ranking",
			boring.MetaScore, interesting.MetaScore)
	}
}

// TestGenericPenaltyDoesNotRescueAWeakCandidate keeps the exemption narrow.
// A candidate the threshold would have cut on its other signals is cut, penalty
// or no penalty — otherwise the exemption becomes a way for the weakest
// hypotheses in the schema to reach the user's database.
func TestGenericPenaltyDoesNotRescueAWeakCandidate(t *testing.T) {
	tipos := tbl("tipos")
	tipos.Stats.EstimatedRows = 6

	// Nullable and merely compatible in type: 0.15 + 0.10 + 0.20 = 0.45,
	// under the threshold before the penalty is even considered.
	weak := model.Column{Name: "tipo_id", Type: "integer", BaseType: "int4", Nullable: true}

	res := generate(t, schema(tipos, tbl("pedido", weak)))

	c, ok := discarded(res, "public.pedido.tipo_id", "public.tipos.id")
	if !ok {
		t.Fatal("a candidate weak on its own signals must still be discarded")
	}
	if c.Reason == "" {
		t.Error("a discard must carry its reason")
	}
}

// TestAmbiguousPenaltyStillCuts keeps the exemption to one signal. The other
// negative signals say the hypothesis is less likely to be true, which is what
// a confidence threshold is for; only the generic-name penalty says the
// relationship is uninteresting.
func TestAmbiguousPenaltyStillCuts(t *testing.T) {
	// The same entity name in two schemas: neither target can be the one
	// meant, and the tool does not get to pick.
	first := model.Schema{Name: "public", Tables: []model.Table{
		tbl("cliente"),
		tbl("pedido", model.Column{Name: "cliente_id", Type: "integer", BaseType: "int4", Nullable: true}),
	}}
	second := model.Schema{Name: "arquivo", Tables: []model.Table{tbl("cliente")}}
	for i := range second.Tables {
		second.Tables[i].Schema = "arquivo"
	}

	res := generate(t, []model.Schema{first, second})

	c, ok := find(res, "public.pedido.cliente_id", "public.cliente.id")
	if !ok {
		t.Fatal("the candidate should have been generated")
	}
	if !c.HasSignal(model.SigAmbiguousTarget) {
		t.Fatal("two tables answering to the same name must be marked ambiguous")
	}
	if _, still := survivor(res, "public.pedido.cliente_id", "public.cliente.id"); still {
		t.Error("ambiguity is a statement about truth, not about interest: it has to keep cutting")
	}
}

// TestGenericNameWithLargeTargetIsNotPenalized is the other half of the rule: a
// generic name pointing at a large table is not a domain table at all.
func TestGenericNameWithLargeTargetIsNotPenalized(t *testing.T) {
	status := tbl("status")
	status.Stats.EstimatedRows = 8_000_000

	res := generate(t, schema(status, tbl("pedido", col("status_id"))))

	c, ok := find(res, "public.pedido.status_id", "public.status.id")
	if !ok {
		t.Fatal("candidate not generated")
	}
	if c.HasSignal(model.SigGenericDomain) {
		t.Error("a large target is not a domain table and must not be penalized")
	}
}

func TestPolymorphicPairIsRecognized(t *testing.T) {
	documento := tbl("documento",
		col("entidade_id"),
		model.Column{Name: "entidade_tipo", Type: "text", BaseType: "text"},
	)

	res := generate(t, schema(documento))

	if len(res.Polymorphic) != 1 {
		t.Fatalf("got %d polymorphic pairs, want 1", len(res.Polymorphic))
	}
	if res.Polymorphic[0].ReferenceColumn != "entidade_id" ||
		res.Polymorphic[0].TypeColumn != "entidade_tipo" {
		t.Errorf("pair = %+v, want entidade_id beside entidade_tipo", res.Polymorphic[0])
	}
}

func TestPlainReferenceIsNotMistakenForPolymorphic(t *testing.T) {
	res := generate(t, schema(tbl("cliente"), tbl("pedido", col("cliente_id"))))

	if len(res.Polymorphic) != 0 {
		t.Errorf("polymorphic pairs = %+v, want none", res.Polymorphic)
	}
}

func TestThresholdMovesTheSurvivingSet(t *testing.T) {
	schemas := schema(tbl("cliente"), tbl("pedido", col("cliente_id")))

	strict := infer.Generate(schemas, infer.Options{Profile: ptBR(t), MinScore: 0.99})
	loose := infer.Generate(schemas, infer.Options{Profile: ptBR(t), MinScore: 0.01})

	if len(strict.Candidates) >= len(loose.Candidates) {
		t.Errorf("a stricter threshold must survive fewer candidates: %d vs %d",
			len(strict.Candidates), len(loose.Candidates))
	}
	if len(strict.Discarded) == 0 {
		t.Fatal("discarded candidates must be kept, not dropped")
	}
	if !strings.Contains(strict.Discarded[0].Reason, "threshold") {
		t.Errorf("reason = %q, want it to name the threshold", strict.Discarded[0].Reason)
	}
}

func TestGenerationIsDeterministic(t *testing.T) {
	schemas := schema(
		tbl("cliente"), tbl("fornecedor"), tbl("municipio"),
		tbl("pedido", col("cliente_id"), col("municipio_id")),
		tbl("contrato", col("fornecedor_id"), col("municipio_id")),
	)

	first := generate(t, schemas)

	for i := 0; i < 20; i++ {
		again := generate(t, schemas)

		if len(again.Candidates) != len(first.Candidates) {
			t.Fatalf("candidate count changed between runs: %d vs %d",
				len(again.Candidates), len(first.Candidates))
		}
		for j := range first.Candidates {
			if first.Candidates[j].Child.String() != again.Candidates[j].Child.String() ||
				first.Candidates[j].Parent.String() != again.Candidates[j].Parent.String() {
				t.Fatalf("ordering changed at %d: %v vs %v",
					j, first.Candidates[j].Child, again.Candidates[j].Child)
			}
		}
	}
}

func TestCandidatesAreOrderedByDescendingScore(t *testing.T) {
	res := generate(t, schema(
		tbl("cliente"), tbl("municipio"), tbl("fornecedor"),
		tbl("pedido", col("cliente_id"), col("municipio_id"), col("fornecedor_id")),
	))

	for i := 1; i < len(res.Candidates); i++ {
		if res.Candidates[i-1].MetaScore < res.Candidates[i].MetaScore {
			t.Fatalf("candidates are not ordered by score: %.2f before %.2f",
				res.Candidates[i-1].MetaScore, res.Candidates[i].MetaScore)
		}
	}
}

func TestNilProfileProducesNothing(t *testing.T) {
	res := infer.Generate(schema(tbl("cliente"), tbl("pedido", col("cliente_id"))), infer.Options{})

	if len(res.Candidates) != 0 || len(res.Discarded) != 0 {
		t.Error("without a naming profile there is no basis for any hypothesis")
	}
}

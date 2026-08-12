package sqlprobe

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func joins(t *testing.T, sql string) []rawJoin {
	t.Helper()
	j, _ := extract(sql)
	return j
}

func predicates(t *testing.T, sql string) []rawPredicate {
	t.Helper()
	_, p := extract(sql)
	return p
}

func pair(lt, lc, rt, rc string) rawJoin {
	return rawJoin{
		left:  rawRef{table: lt, column: lc},
		right: rawRef{table: rt, column: rc},
	}
}

func TestExtractsExplicitJoin(t *testing.T) {
	got := joins(t, `SELECT * FROM os_servico o JOIN funcionario f ON o.resp_tecnico = f.id`)

	want := []rawJoin{pair("os_servico", "resp_tecnico", "funcionario", "id")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractsImplicitWhereJoin(t *testing.T) {
	got := joins(t, `SELECT 1 FROM pedido p, cliente c WHERE p.cliente_id = c.id AND p.valor > 0`)

	want := []rawJoin{pair("pedido", "cliente_id", "cliente", "id")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestResolvesWithoutAlias(t *testing.T) {
	got := joins(t, `SELECT 1 FROM pedido JOIN cliente ON pedido.cliente_id = cliente.id`)

	want := []rawJoin{pair("pedido", "cliente_id", "cliente", "id")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemaQualifiedTableSurvives(t *testing.T) {
	got := joins(t, `SELECT 1 FROM billing.fatura f JOIN billing.cliente c ON f.cliente_id = c.id`)

	want := []rawJoin{{
		left:  rawRef{schema: "billing", table: "fatura", column: "cliente_id"},
		right: rawRef{schema: "billing", table: "cliente", column: "id"},
	}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestMultipleJoinsAllExtracted(t *testing.T) {
	got := joins(t, `
		SELECT * FROM pedido p
		JOIN cliente c ON p.cliente_id = c.id
		LEFT JOIN municipio m ON c.municipio_id = m.id
		WHERE p.status_id = 3`)

	want := []rawJoin{
		pair("pedido", "cliente_id", "cliente", "id"),
		pair("cliente", "municipio_id", "municipio", "id"),
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

// TestEqualityAgainstLiteralIsNotEvidence pins the bare-column rule: without a
// qualifier there is no table to resolve, and p.status_id = 3 is a filter, not
// a join. It is still predicate evidence: an equality filter is a hot-column
// signal in its own right.
func TestEqualityAgainstLiteralIsNotEvidence(t *testing.T) {
	sql := `SELECT 1 FROM pedido p WHERE p.status_id = 3 AND p.tipo = 'x'`

	if got := joins(t, sql); len(got) != 0 {
		t.Errorf("joins = %+v, want none", got)
	}

	want := []rawPredicate{
		{ref: rawRef{table: "pedido", column: "status_id"}, op: predEquality},
		{ref: rawRef{table: "pedido", column: "tipo"}, op: predEquality},
	}
	if diff := cmp.Diff(want, predicates(t, sql), cmp.AllowUnexported(rawPredicate{}, rawRef{})); diff != "" {
		t.Errorf("predicates mismatch (-want +got):\n%s", diff)
	}
}

func TestStringsAndCommentsAreInvisible(t *testing.T) {
	sql := `
		-- fake: a.x = b.y
		/* also fake: c.x = d.y  /* nested */ still comment */
		SELECT 'literal with a.x = b.y inside',
		       E'escaped \' and a.x = b.y',
		       $tag$dynamic: e.x = f.y$tag$
		FROM pedido p JOIN cliente c ON p.cliente_id = c.id`

	want := []rawJoin{pair("pedido", "cliente_id", "cliente", "id")}
	if diff := cmp.Diff(want, joins(t, sql), cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

// TestDollarWrappedBodyIsCode covers the CREATE FUNCTION form: a body handed
// over wrapped in one dollar quote is code, while embedded quotes stay strings.
func TestDollarWrappedBodyIsCode(t *testing.T) {
	sql := `$fn$
		SELECT count(*) FROM os_servico o JOIN funcionario f ON o.resp_tecnico = f.id;
	$fn$`

	want := []rawJoin{pair("os_servico", "resp_tecnico", "funcionario", "id")}
	if diff := cmp.Diff(want, joins(t, sql), cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestQuotedIdentifiersKeepCaseAndSpaces(t *testing.T) {
	got := joins(t, `SELECT 1 FROM "Empenho 2024" e JOIN "Unidade Gestora" u ON e."unidade gestora" = u.id`)

	want := []rawJoin{pair("Empenho 2024", "unidade gestora", "Unidade Gestora", "id")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

func TestOtherOperatorsAreNotEquality(t *testing.T) {
	sql := `SELECT 1 FROM a x JOIN b y ON x.k <= y.k WHERE x.v != y.v AND x.w <> y.w AND x.z >= y.z`
	if got := joins(t, sql); len(got) != 0 {
		t.Errorf("joins = %+v, want none: <=, >=, != and <> are not equality", got)
	}

	// Each left-hand qualified column still produces range predicate evidence,
	// which is what a hot-column recommendation needs even when no join formed.
	preds := predicates(t, sql)
	if len(preds) != 4 {
		t.Fatalf("predicates = %+v, want 4 range predicates", preds)
	}
	for _, p := range preds {
		if p.op != predRange {
			t.Errorf("predicate %+v: op = %v, want predRange", p, p.op)
		}
	}
}

func TestStatementsDoNotShareAliases(t *testing.T) {
	sql := `
		SELECT 1 FROM pedido p WHERE p.id > 0;
		SELECT 2 FROM cliente c WHERE c.id = p.cliente_id`

	// p is unknown in the second statement; the reference stays unresolved and
	// is left for catalog resolution, where no table "p" will exist.
	got := joins(t, sql)
	if len(got) != 1 || got[0].right.table != "p" {
		t.Fatalf("joins = %+v, want the unresolved reference kept as written", got)
	}
}

func TestMalformedSQLYieldsNothingAndNoPanic(t *testing.T) {
	cases := []string{
		"",
		"SELECT FROM WHERE =",
		"FROM ((((",
		`SELECT 'unterminated string FROM a JOIN b ON a.x = b.y`,
		"$body$ unterminated dollar quote FROM a b ON a.x = b.y",
		"CREATE TRIGGER weird EXECUTE $$",
		"合同 JOIN ON = 数据",
	}
	for _, sql := range cases {
		j, p := extract(sql)
		if len(j) != 0 || len(p) != 0 {
			t.Errorf("extract(%q) = joins %+v, predicates %+v, want nothing", sql, j, p)
		}
	}
}

func TestSubqueryAliasIsIgnoredNotMisresolved(t *testing.T) {
	sql := `SELECT 1 FROM (SELECT id FROM cliente) sub JOIN pedido p ON p.cliente_id = sub.id`

	// sub resolves to no table; the pair keeps "sub" and dies at catalog
	// resolution instead of being guessed at.
	got := joins(t, sql)
	if len(got) != 1 || got[0].right.table != "sub" {
		t.Fatalf("joins = %+v, want one pair with the subquery alias unresolved", got)
	}
	if got[0].left.table != "pedido" {
		t.Errorf("left side = %+v, want pedido resolved through its alias", got[0].left)
	}
}

// TestServerReconstructedViewShape covers the form pg_views actually returns:
// the server wraps multi-way joins in parentheses, and reading those as a
// subquery would skip every table in the statement.
func TestServerReconstructedViewShape(t *testing.T) {
	sql := `
		 SELECT o.id,
		    f.nome AS responsavel,
		    c.nome AS cliente
		   FROM ((os_servico o
		     JOIN funcionario f ON ((o.resp_tecnico = f.id)))
		     JOIN cliente c ON ((o.cliente_id = c.id)));`

	want := []rawJoin{
		pair("os_servico", "resp_tecnico", "funcionario", "id"),
		pair("os_servico", "cliente_id", "cliente", "id"),
	}
	if diff := cmp.Diff(want, joins(t, sql), cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}
}

// TestPredicateContainmentIsClassified covers the jsonb/array operators GIN
// serves and btree does not.
func TestPredicateContainmentIsClassified(t *testing.T) {
	cases := []struct {
		sql string
		op  predOp
	}{
		{`SELECT 1 FROM t WHERE t.dados @> '{"a":1}'::jsonb`, predContainment},
		{`SELECT 1 FROM t WHERE t.tags <@ '{"a","b"}'::text[]`, predContainment},
		{`SELECT 1 FROM t WHERE t.dados ? 'chave'`, predContainment},
		{`SELECT 1 FROM t WHERE t.dados ?| array['a','b']`, predContainment},
		{`SELECT 1 FROM t WHERE t.dados ?& array['a','b']`, predContainment},
		{`SELECT 1 FROM t WHERE t.busca @@ to_tsquery('portuguese', 'x')`, predFullText},
	}

	for _, c := range cases {
		got := predicates(t, c.sql)
		if len(got) != 1 || got[0].op != c.op {
			t.Errorf("predicates(%q) = %+v, want one predicate with op %v", c.sql, got, c.op)
		}
	}
}

// TestPredicateLikeDistinguishesPrefixFromInfix pins the distinction that
// drives the trigram-index recommendation: an infix wildcard defeats a
// leading-anchored btree scan, a prefix pattern does not.
func TestPredicateLikeDistinguishesPrefixFromInfix(t *testing.T) {
	sql := `SELECT 1 FROM t WHERE t.nome LIKE '%silva%' AND t.cod ILIKE 'ABC%'`

	got := predicates(t, sql)
	want := []rawPredicate{
		{ref: rawRef{table: "t", column: "nome"}, op: predLikeInfix},
		{ref: rawRef{table: "t", column: "cod"}, op: predLikePrefix},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(rawPredicate{}, rawRef{})); diff != "" {
		t.Errorf("predicates mismatch (-want +got):\n%s", diff)
	}
}

// TestPredicateLikeWithUnseenRightHandSideDefaultsToPrefix covers a pattern
// the extractor cannot read — a parameter or a function call — which must
// never be guessed as infix.
func TestPredicateLikeWithUnseenRightHandSideDefaultsToPrefix(t *testing.T) {
	sql := `SELECT 1 FROM t WHERE t.nome LIKE upper('x')`

	got := predicates(t, sql)
	if len(got) != 1 || got[0].op != predLikePrefix {
		t.Errorf("predicates = %+v, want one predLikePrefix", got)
	}
}

// TestPredicateVectorDistanceIsClassified covers pgvector's nearest-neighbor
// operators.
func TestPredicateVectorDistanceIsClassified(t *testing.T) {
	for _, op := range []string{"<->", "<=>", "<#>"} {
		sql := `SELECT 1 FROM t ORDER BY t.embedding ` + op + ` '[1,2,3]' LIMIT 10`
		got := predicates(t, sql)
		if len(got) != 1 || got[0].op != predVectorDistance {
			t.Errorf("predicates(%q) = %+v, want one predVectorDistance", sql, got)
		}
	}
}

// TestUnrecognizedOperatorProducesNoPredicateAndNoPanic pins the extractor's
// central contract: an operator it does not classify is ignored, never
// guessed, and never a crash.
func TestUnrecognizedOperatorProducesNoPredicateAndNoPanic(t *testing.T) {
	sql := `SELECT 1 FROM t WHERE t.nome ~ '^abc' AND t.nome !~ 'xyz'`
	if got := predicates(t, sql); len(got) != 0 {
		t.Errorf("predicates = %+v, want none for an unrecognized operator", got)
	}
}

// TestJoinExtractionIsUnaffectedByPredicateExtraction pins the requirement
// that the existing join contract does not change now that predicates are
// extracted alongside it.
func TestJoinExtractionIsUnaffectedByPredicateExtraction(t *testing.T) {
	sql := `SELECT 1 FROM pedido p JOIN cliente c ON p.cliente_id = c.id WHERE p.valor > 100`

	wantJoins := []rawJoin{pair("pedido", "cliente_id", "cliente", "id")}
	if diff := cmp.Diff(wantJoins, joins(t, sql), cmp.AllowUnexported(rawJoin{}, rawRef{})); diff != "" {
		t.Errorf("joins mismatch (-want +got):\n%s", diff)
	}

	wantPreds := []rawPredicate{{ref: rawRef{table: "pedido", column: "valor"}, op: predRange}}
	if diff := cmp.Diff(wantPreds, predicates(t, sql), cmp.AllowUnexported(rawPredicate{}, rawRef{})); diff != "" {
		t.Errorf("predicates mismatch (-want +got):\n%s", diff)
	}
}

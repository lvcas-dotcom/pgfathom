package validate

import (
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestKeyProbeQueryQuotesIdentifiers(t *testing.T) {
	table := model.Table{Schema: "public", Name: `Ordem Servico`}

	q := buildKeyProbeQuery(table, []string{"cliente_id", `uni"dade_id`})

	for _, want := range []string{`"Ordem Servico"`, `"cliente_id"`, `"uni""dade_id"`} {
		if !strings.Contains(q, want) {
			t.Errorf("query must quote %s; got:\n%s", want, q)
		}
	}
}

// TestKeyProbeQuerySingleColumnUsesSameTupleShape pins the reason the query
// builder needs no special case for a single-column candidate: a
// one-element parenthesized expression is not a row constructor in
// PostgreSQL, so count(DISTINCT (col)) already means the same thing as
// count(DISTINCT col).
func TestKeyProbeQuerySingleColumnUsesSameTupleShape(t *testing.T) {
	table := model.Table{Schema: "public", Name: "cliente"}

	q := buildKeyProbeQuery(table, []string{"cpf"})

	if !strings.Contains(q, `count(DISTINCT ("cpf"))`) {
		t.Errorf("query must count distinct over the single-column tuple; got:\n%s", q)
	}
	if !strings.Contains(q, `"cpf" IS NULL`) {
		t.Errorf("query must check the column for NULL; got:\n%s", q)
	}
}

func TestKeyProbeQueryCompositeChecksAllColumnsForNull(t *testing.T) {
	table := model.Table{Schema: "public", Name: "item_pedido"}

	q := buildKeyProbeQuery(table, []string{"pedido_id", "sequencia"})

	if !strings.Contains(q, `count(DISTINCT ("pedido_id", "sequencia"))`) {
		t.Errorf("query must count distinct over the composite tuple; got:\n%s", q)
	}
	if !strings.Contains(q, `"pedido_id" IS NULL OR "sequencia" IS NULL`) {
		t.Errorf("query must flag a row NULL in any key column; got:\n%s", q)
	}
}

// TestKeyProbeQueryNeverSamples pins the rule that confirming a key never
// reads a fraction of the table: TABLESAMPLE must never appear in this
// query, because a duplicate outside the sample would make a "confirmed"
// verdict false.
func TestKeyProbeQueryNeverSamples(t *testing.T) {
	table := model.Table{Schema: "public", Name: "cliente"}

	q := buildKeyProbeQuery(table, []string{"cpf"})

	if strings.Contains(q, "TABLESAMPLE") {
		t.Errorf("key probe query must never sample; got:\n%s", q)
	}
}

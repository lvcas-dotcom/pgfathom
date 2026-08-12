package cli

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestExampleSuffix(t *testing.T) {
	if got := exampleSuffix(nil); got != "" {
		t.Errorf("exampleSuffix(nil) = %q, want empty: a convention with no evidence must not fabricate one", got)
	}
	if got := exampleSuffix([]string{"cliente", "pedido"}); got != " (e.g. cliente, pedido)" {
		t.Errorf("exampleSuffix = %q, want the examples cited so the convention is checkable", got)
	}
}

func TestParseKeyResolutionAnswer(t *testing.T) {
	tests := []struct {
		name               string
		line               string
		compositeAvailable bool
		pkName             string
		wantAction         keyResolutionAction
		wantOK             bool
	}{
		{"empty line skips", "", true, "idkey", keyResolutionSkip, true},
		{"whitespace-only line skips", "   \n", true, "idkey", keyResolutionSkip, true},
		{"a chooses composite when available", "a\n", true, "idkey", keyResolutionComposite, true},
		{"A is case-insensitive", "A\n", true, "idkey", keyResolutionComposite, true},
		{"a is invalid when composite is not available", "a\n", false, "idkey", keyResolutionSkip, false},
		{"b chooses synthetic when a convention exists", "b\n", true, "idkey", keyResolutionSynthetic, true},
		{"B is case-insensitive", "B\n", false, "idkey", keyResolutionSynthetic, true},
		{"b is invalid when no convention was detected", "b\n", true, "", keyResolutionSkip, false},
		{"an unrecognized answer is invalid, not a skip", "x\n", true, "idkey", keyResolutionSkip, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, ok := parseKeyResolutionAnswer(tt.line, tt.compositeAvailable, tt.pkName)
			if action != tt.wantAction {
				t.Errorf("action = %v, want %v", action, tt.wantAction)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestResolvePKName(t *testing.T) {
	t.Run("no evidence names nothing", func(t *testing.T) {
		if name := resolvePKName(nil); name != "" {
			t.Errorf("resolvePKName(nil) = %q, want empty", name)
		}
	})

	t.Run("whatever tops the ranking is what the rest of the schema already uses", func(t *testing.T) {
		name := resolvePKName([]model.NamingEvidence{
			{Affix: "idkey", Occurrences: 42, Share: 0.42},
			{Affix: "id", Occurrences: 38, Share: 0.38},
		})
		if name != "idkey" {
			t.Errorf("resolvePKName = %q, want idkey: there is no separate confidence bar to clear", name)
		}
	})
}

func fkTable(name string, fkColumns ...string) model.Table {
	fks := make([]model.ForeignKey, len(fkColumns))
	for i, c := range fkColumns {
		fks[i] = model.ForeignKey{Columns: []string{c}, RefSchema: "public", RefTable: "other", RefColumns: []string{"idkey"}}
	}
	return model.Table{Schema: "public", Name: name, ForeignKeys: fks}
}

func TestFKKeyCandidate(t *testing.T) {
	t.Run("two single-column FKs form a candidate", func(t *testing.T) {
		cols, ok := fkKeyCandidate(fkTable("pedido_item", "idkey_pedido", "idkey_produto"), nil)
		if !ok {
			t.Fatal("want a candidate")
		}
		if len(cols) != 2 || cols[0] != "idkey_pedido" || cols[1] != "idkey_produto" {
			t.Errorf("cols = %v, want [idkey_pedido idkey_produto]", cols)
		}
	})

	t.Run("a single FK is not enough", func(t *testing.T) {
		if _, ok := fkKeyCandidate(fkTable("pedido_item", "idkey_pedido"), nil); ok {
			t.Error("want no candidate with only one FK column")
		}
	})

	t.Run("no FKs at all", func(t *testing.T) {
		if _, ok := fkKeyCandidate(fkTable("staging"), nil); ok {
			t.Error("want no candidate with no FKs")
		}
	})

	t.Run("a multi-column FK does not count toward the composite", func(t *testing.T) {
		tbl := model.Table{Schema: "public", Name: "t", ForeignKeys: []model.ForeignKey{
			{Columns: []string{"a", "b"}, RefTable: "other"},
			{Columns: []string{"c"}, RefTable: "third"},
		}}
		if _, ok := fkKeyCandidate(tbl, nil); ok {
			t.Error("want no candidate: only one single-column FK is available")
		}
	})

	t.Run("already tried combination is not offered again", func(t *testing.T) {
		tbl := fkTable("pedido_item", "idkey_pedido", "idkey_produto")
		alreadyTried := [][]string{{"idkey_produto", "idkey_pedido"}}
		if _, ok := fkKeyCandidate(tbl, alreadyTried); ok {
			t.Error("want no candidate: the same set was already tried, in a different order")
		}
	})
}

func TestApplySyntheticKey(t *testing.T) {
	f := &model.Finding{Kind: model.FindingMissingPrimaryKey, Suggestion: &model.Suggestion{
		Kind: model.SuggestCreatePrimaryKey, KeyProbe: model.KeyProbeUnverified,
	}}

	applySyntheticKey(f, "idkey", "user-provided name")

	if f.Suggestion.Kind != model.SuggestSyntheticPrimaryKey {
		t.Errorf("Kind = %v, want %v", f.Suggestion.Kind, model.SuggestSyntheticPrimaryKey)
	}
	if len(f.Suggestion.Columns) != 1 || f.Suggestion.Columns[0] != "idkey" {
		t.Errorf("Columns = %v, want [idkey]", f.Suggestion.Columns)
	}
	if f.Suggestion.KeyProbe != "" {
		t.Errorf("KeyProbe = %q, want empty: a synthetic column is never confirmed by data", f.Suggestion.KeyProbe)
	}
	if f.Suggestion.Note != "user-provided name" {
		t.Errorf("Note = %q, want %q", f.Suggestion.Note, "user-provided name")
	}
}

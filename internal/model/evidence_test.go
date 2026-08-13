package model_test

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

func TestIndexMethodForDefaultsToBtree(t *testing.T) {
	for _, op := range []model.OperatorClass{model.OpEquality, model.OpRange, model.OpLikePrefix} {
		method, opclass, note := model.IndexMethodFor(op, "int8", model.NewExtensionSet(nil))
		if method != "btree" || opclass != "" || note != "" {
			t.Errorf("IndexMethodFor(%v) = %q, %q, %q; want btree, no opclass, no note", op, method, opclass, note)
		}
	}
}

func TestIndexMethodForContainmentGatesOnType(t *testing.T) {
	tests := []struct {
		baseType   string
		wantMethod string
	}{
		{"jsonb", "gin"},
		{"hstore", "gin"},
		{"_int4", "gin"}, // integer[]: base_type carries pg_type.typname's underscore prefix
		{"text", ""},     // GIN has no default operator class for plain text
	}

	for _, tt := range tests {
		method, _, _ := model.IndexMethodFor(model.OpContainment, tt.baseType, model.NewExtensionSet(nil))
		if method != tt.wantMethod {
			t.Errorf("IndexMethodFor(containment, %q) method = %q, want %q", tt.baseType, method, tt.wantMethod)
		}
	}
}

// TestIndexMethodForFullTextRequiresTsvector pins the rule that a GIN index
// is only recommended for a stored tsvector column: a plain text column
// under @@ needs an expression index this layer must not guess.
func TestIndexMethodForFullTextRequiresTsvector(t *testing.T) {
	if method, _, _ := model.IndexMethodFor(model.OpFullText, "tsvector", model.NewExtensionSet(nil)); method != "gin" {
		t.Errorf("tsvector column: method = %q, want gin", method)
	}
	if method, _, _ := model.IndexMethodFor(model.OpFullText, "text", model.NewExtensionSet(nil)); method != "" {
		t.Errorf("plain text column: method = %q, want no recommendation", method)
	}
}

func TestIndexMethodForLikeInfixDegradesWithoutPgTrgm(t *testing.T) {
	method, opclass, note := model.IndexMethodFor(model.OpLikeInfix, "text", model.NewExtensionSet(nil))
	if method != "btree" || opclass != "" || note == "" {
		t.Errorf("without pg_trgm: got %q, %q, %q; want btree fallback with a note", method, opclass, note)
	}

	method, opclass, note = model.IndexMethodFor(model.OpLikeInfix, "text", model.NewExtensionSet([]string{"pg_trgm"}))
	if method != "gin" || opclass != "gin_trgm_ops" || note != "" {
		t.Errorf("with pg_trgm: got %q, %q, %q; want gin/gin_trgm_ops, no note", method, opclass, note)
	}
}

func TestIndexMethodForVectorDistanceRequiresExtensionAndType(t *testing.T) {
	if method, _, note := model.IndexMethodFor(model.OpVectorDistance, "vector", model.NewExtensionSet(nil)); method != "" || note == "" {
		t.Errorf("without pgvector: got method %q, note %q; want no method and a note", method, note)
	}

	vec := model.NewExtensionSet([]string{"vector"})
	if method, _, note := model.IndexMethodFor(model.OpVectorDistance, "int8", vec); method != "" || note != "" {
		t.Errorf("pgvector installed but column not vector-typed: got %q, %q; want no honest recommendation", method, note)
	}

	method, opclass, note := model.IndexMethodFor(model.OpVectorDistance, "vector", vec)
	if method != "hnsw" || opclass != "vector_l2_ops" || note == "" {
		t.Errorf("pgvector installed, vector column: got %q, %q, %q; want hnsw/vector_l2_ops with a caveat note", method, opclass, note)
	}
}

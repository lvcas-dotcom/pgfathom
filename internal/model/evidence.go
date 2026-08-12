package model

import "strings"

// JoinSource says where a join predicate was found.
type JoinSource string

// Where a join predicate can come from, strongest context first: a view
// definition is reconstructed by the server, a function body is read as
// written, and the query log mixes in ad-hoc sessions.
const (
	JoinFromView       JoinSource = "view"
	JoinFromFunction   JoinSource = "function"
	JoinFromStatements JoinSource = "statements"
)

// JoinEvidence is one equality between two resolved columns, extracted from
// SQL the database itself stores. It carries no direction: which side is the
// parent is decided by who holds a key, not by which side of the operator the
// extractor happened to see first.
type JoinEvidence struct {
	Left   ColumnRef  `json:"left"`
	Right  ColumnRef  `json:"right"`
	Source JoinSource `json:"source"`

	// Object names the view or function the join came from — the fact a user
	// can go read to check the evidence.
	Object string `json:"object"`
}

// SignalFor maps the evidence origin to its signal kind.
func (e JoinEvidence) SignalFor() SignalKind {
	switch e.Source {
	case JoinFromFunction:
		return SigJoinInFunction
	case JoinFromStatements:
		return SigJoinInStatements
	default:
		return SigJoinInView
	}
}

// OperatorClass groups predicate operators by the index access method they
// call for. The set is closed: an operator the extractor cannot classify
// produces no PredicateEvidence at all, rather than a guessed class.
type OperatorClass string

const (
	// OpEquality is served by btree, which is never the wrong recommendation.
	OpEquality OperatorClass = "eq"

	// OpRange is a comparison served by btree, same as OpEquality.
	OpRange OperatorClass = "range"

	// OpLikePrefix is a LIKE/ILIKE anchored at the start of the pattern —
	// still a btree case.
	OpLikePrefix OperatorClass = "like_prefix"

	// OpLikeInfix is unanchored and wants a trigram index.
	OpLikeInfix OperatorClass = "like_infix"

	// OpContainment covers jsonb/array containment and membership (@>, <@, ?,
	// ?|, ?&), served by GIN.
	OpContainment OperatorClass = "containment"

	// OpFullText is the @@ text-search match operator, served by GIN.
	OpFullText OperatorClass = "fulltext"

	// OpVectorDistance is a pgvector distance operator (<->, <=>, <#>), served
	// by a nearest-neighbor index method such as HNSW.
	OpVectorDistance OperatorClass = "vector_distance"
)

// PredicateEvidence is one predicate on a resolved column, extracted from SQL
// the database itself stores. Unlike JoinEvidence, which pairs two columns,
// this describes one column's operator, and exists to drive index method
// recommendations rather than relationship inference.
type PredicateEvidence struct {
	Column   ColumnRef     `json:"column"`
	Operator OperatorClass `json:"operator"`
	Source   JoinSource    `json:"source"`

	// Object names the view, function, or statement the predicate came from —
	// the fact a user can go read to check the evidence.
	Object string `json:"object"`
}

// IndexMethodFor maps a predicate operator and the column's base type to the
// index access method that serves it and, when the method needs one, the
// operator class — given the extensions actually installed.
//
// btree is the default for equality, range, and prefix LIKE: it is never the
// wrong recommendation. A method gated on an extension or a type it cannot
// honestly claim degrades to btree with a note when btree is still a
// reasonable fallback, or to an empty method — meaning no honest
// recommendation exists — when it is not. An empty method must never reach a
// CREATE INDEX statement the server would reject: GIN has no default operator
// class for plain text, and a bare column reference on a non-jsonb,
// non-array, non-tsvector type would fail exactly that way.
func IndexMethodFor(op OperatorClass, baseType string, ext ExtensionSet) (method, opclass, note string) {
	switch op {
	case OpContainment:
		if isContainerType(baseType) {
			return "gin", "", ""
		}
		return "", "", ""
	case OpFullText:
		if baseType == "tsvector" {
			return "gin", "", ""
		}
		// A GIN index over the raw column would fail: full-text search over a
		// plain text column needs an expression index on to_tsvector(...), and
		// guessing the text search configuration inside the expression is not
		// this layer's call to make.
		return "", "", ""
	case OpLikeInfix:
		if ext.Has("pg_trgm") {
			return "gin", "gin_trgm_ops", ""
		}
		return "btree", "", "infix LIKE would benefit from pg_trgm, which is not installed"
	case OpVectorDistance:
		if !ext.Has("vector") {
			return "", "", "vector distance operator found but pgvector is not installed"
		}
		if baseType != "vector" {
			return "", "", ""
		}
		return "hnsw", "vector_l2_ops", "defaulted to the L2 operator class; switch to " +
			"vector_cosine_ops or vector_ip_ops if the query actually uses <=> or <#>"
	default:
		return "btree", "", ""
	}
}

// isContainerType reports whether GIN has a default operator class for
// baseType, so a containment predicate can be indexed without an expression.
// Array types carry base_type as pg_type.typname does — an underscore prefix,
// e.g. "_int4" for integer[] — which is what the trailing check catches.
func isContainerType(baseType string) bool {
	switch baseType {
	case "jsonb", "hstore":
		return true
	}
	return strings.HasPrefix(baseType, "_")
}

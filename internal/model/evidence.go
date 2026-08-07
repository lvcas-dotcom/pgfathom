package model

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

package model

// FindingKind identifies a structural finding that requires no inference:
// straight from the catalog, deterministic, immune to false positives.
type FindingKind string

const (
	// FindingNotValidConstraint is a constraint created NOT VALID and never
	// validated. See ForeignKey.Validated.
	FindingNotValidConstraint FindingKind = "not_valid_constraint"

	// FindingFKWithoutIndex is a declared foreign key with no usable index on
	// the child side.
	FindingFKWithoutIndex FindingKind = "fk_without_index"

	// FindingOrphanReference is a reference column pointing at a table that no
	// longer exists. Correctly rejected by validation, but a finding in itself.
	FindingOrphanReference FindingKind = "orphan_reference"

	// FindingPolymorphicPair is a reference column that only makes sense beside
	// a discriminator column. Validation would reject it for low containment,
	// which reads as "name coincidence" where a real relationship exists that
	// this version does not model.
	FindingPolymorphicPair FindingKind = "polymorphic_pair"

	// FindingUnsupportedTarget is a target that was recognized and could not be
	// turned into a hypothesis: no primary key, part of a composite key with a
	// counterpart, more than one way to resolve the same key, or several tables
	// carrying the same key. Each is a near miss, and near misses are where the
	// recall that got away is legible.
	FindingUnsupportedTarget FindingKind = "unsupported_target"

	// FindingMissingPrimaryKey is a table with no primary key: no row
	// identity, no logical replication, and a sequential scan behind every
	// per-row update or delete.
	FindingMissingPrimaryKey FindingKind = "missing_primary_key"

	// FindingUnindexedHotColumn is a column that real code — a view, a
	// function, or the query log — repeatedly names in a join or filter
	// predicate, with no index leading on it.
	FindingUnindexedHotColumn FindingKind = "unindexed_hot_column"
)

// Finding is a structural observation that did not require inference.
type Finding struct {
	Kind FindingKind `json:"kind"`

	// Object is the schema-qualified catalog object the finding is about.
	Object string `json:"object"`

	// Detail describes the finding. Object names and conditions only.
	Detail string `json:"detail,omitempty"`

	Metrics map[string]int64 `json:"metrics,omitempty"`

	// Suggestion is the remediation the finding proposes, when it has one
	// concrete enough to act on.
	Suggestion *Suggestion `json:"suggestion,omitempty"`
}

// SuggestionKind identifies what a Suggestion proposes.
type SuggestionKind string

const (
	// SuggestPromoteUnique proposes promoting an existing UNIQUE NOT NULL
	// constraint to primary key — a change the catalog already proves safe.
	SuggestPromoteUnique SuggestionKind = "promote_unique"

	// SuggestCreatePrimaryKey proposes a new primary key over columns whose
	// uniqueness the catalog cannot prove on its own. See KeyProbe.
	SuggestCreatePrimaryKey SuggestionKind = "create_primary_key"

	// SuggestCreateIndex proposes a new index over a column real code uses
	// repeatedly but that leads no index today.
	SuggestCreateIndex SuggestionKind = "create_index"

	// SuggestSyntheticPrimaryKey proposes a brand-new identity column as
	// primary key, for a table with no confirmed natural key. Columns carries
	// the chosen column name; KeyProbe stays empty because correctness here
	// comes from creating the column, not from anything already in the data.
	SuggestSyntheticPrimaryKey SuggestionKind = "synthesize_primary_key"
)

// KeyProbeVerdict is the outcome of confirming candidate key columns against
// the data by counting rows. The set is closed and deliberately asymmetric:
// there is no "not a key" value, because the probe that finds a duplicate
// simply produces no confirmed suggestion at all.
type KeyProbeVerdict string

const (
	// KeyProbeConfirmed means a full scan found total = distinct and zero
	// nulls: the columns are a real key, proven, not guessed.
	KeyProbeConfirmed KeyProbeVerdict = "confirmed"

	// KeyProbeUnverified means the probe could not reach a conclusion —
	// timeout, table too large, or disabled — and the suggestion stands as an
	// unconfirmed hypothesis.
	KeyProbeUnverified KeyProbeVerdict = "unverified"
)

// Suggestion is a remediation an audit finding proposes. Every field names a
// catalog object, a method, or a verdict — never a value read from a table.
type Suggestion struct {
	Kind SuggestionKind `json:"kind"`

	// Columns are the catalog column names involved: the candidate key for a
	// primary-key suggestion, the leading column for an index suggestion.
	Columns []string `json:"columns,omitempty"`

	// IndexMethod is the recommended access method — "btree", "gin", "hnsw" —
	// for a SuggestCreateIndex. Empty otherwise.
	IndexMethod string `json:"index_method,omitempty"`

	// IndexOpclass is the operator class IndexMethod needs on the column,
	// e.g. "gin_trgm_ops" or "vector_l2_ops". Empty when the method's default
	// operator class already applies.
	IndexOpclass string `json:"index_opclass,omitempty"`

	// Note carries a caveat such as a missing extension. Object names and
	// conditions only.
	Note string `json:"note,omitempty"`

	// KeyProbe is the verdict of confirming Columns as a real key by counting
	// rows. Empty when no probe ran, which includes the promote_unique case:
	// the catalog already proved that key, so no probe was needed.
	KeyProbe KeyProbeVerdict `json:"key_probe,omitempty"`
}

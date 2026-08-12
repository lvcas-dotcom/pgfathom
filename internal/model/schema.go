package model

import "strings"

// Schema is a namespace of tables read from the catalog.
type Schema struct {
	Name   string  `json:"name"`
	Tables []Table `json:"tables"`
}

// Table is a relation and everything the catalog says about it.
type Table struct {
	Schema  string   `json:"schema"`
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`

	// PrimaryKey lists column names in key order. Empty when there is none.
	PrimaryKey []string `json:"primary_key,omitempty"`

	// Uniques holds every UNIQUE constraint declared on the table, name and
	// columns together. The name is what lets a promotable one be turned into
	// a primary key with ADD CONSTRAINT ... USING INDEX rather than a guess.
	Uniques []UniqueConstraint `json:"uniques,omitempty"`

	// ForeignKeys holds DECLARED constraints only. Inferred relationships live
	// in Candidate, so no consumer can mistake one for the other.
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`

	Indexes []Index    `json:"indexes,omitempty"`
	Stats   TableStats `json:"stats"`

	// Partitioned marks a partitioned parent: read from it, never iterate the
	// partitions, and expect statistics to behave differently.
	Partitioned bool `json:"partitioned,omitempty"`

	Inherits bool `json:"inherits,omitempty"`

	Comment string `json:"comment,omitempty"`
}

// Ref returns the schema-qualified name.
func (t Table) Ref() string { return t.Schema + "." + t.Name }

// HasPrimaryKey reports whether the table declares a primary key of any shape,
// single-column or composite.
func (t Table) HasPrimaryKey() bool { return len(t.PrimaryKey) > 0 }

// PromotableUnique returns the first UNIQUE constraint whose columns are all
// NOT NULL — the shape PostgreSQL accepts for
// ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY USING INDEX <name> without
// rewriting a row. Keeping the name is what makes that statement expressible
// without guessing one. The second result is false when no such unique exists.
func (t Table) PromotableUnique() (UniqueConstraint, bool) {
	for _, u := range t.Uniques {
		if t.allNotNull(u.Columns) {
			return u, true
		}
	}
	return UniqueConstraint{}, false
}

func (t Table) allNotNull(columns []string) bool {
	for _, name := range columns {
		col, ok := t.Column(name)
		if !ok || col.Nullable {
			return false
		}
	}
	return true
}

// Column looks up a column by name. The second result is false when absent.
func (t Table) Column(name string) (Column, bool) {
	for _, c := range t.Columns {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Column{}, false
}

// IsIndexedLeading reports whether an index has the column in leading position,
// which is what makes it usable for a foreign key lookup.
func (t Table) IsIndexedLeading(column string) bool {
	return IndexLeads(t.Indexes, []string{column})
}

// IndexLeads reports whether some index opens with the given columns, in the
// given order. A wider index counts, because the leading columns are what the
// lookup uses; one over the same columns in another order does not.
//
// One owner for the rule: inference reads it to score a candidate and the SQL
// artifacts read it to decide whether to suggest an index, and the two
// disagreeing would suggest an index that already exists.
func IndexLeads(indexes []Index, columns []string) bool {
	if len(columns) == 0 {
		return false
	}
	for _, idx := range indexes {
		if len(idx.Columns) < len(columns) {
			continue
		}
		match := true
		for i, want := range columns {
			if !strings.EqualFold(idx.Columns[i], want) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Column is an attribute of a table.
type Column struct {
	Name string `json:"name"`

	// Type is the type as PostgreSQL formats it, e.g. "character varying(60)".
	Type string `json:"type"`

	// BaseType is normalized for comparison, e.g. "int8". Comparing formatted
	// types directly produces false negatives between equivalent spellings.
	BaseType string `json:"base_type"`

	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Position int    `json:"position"`
	Comment  string `json:"comment,omitempty"`
}

// UniqueConstraint is a UNIQUE constraint as declared in the catalog.
type UniqueConstraint struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// ColumnRef points at one column of one table. It is the right unit where the
// subject really is a single column: planner statistics are per column, and an
// equality mined from SQL relates two of them.
type ColumnRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Column string `json:"column"`
}

// String renders the reference as schema.table.column.
func (r ColumnRef) String() string { return r.Schema + "." + r.Table + "." + r.Column }

// TableRef renders the owning table as schema.table.
func (r ColumnRef) TableRef() string { return r.Schema + "." + r.Table }

// KeyRef points at a key of one or more columns of one table. Arity one is the
// ordinary case of this type rather than a type of its own: a second
// representation for the scalar case would eventually be read by a consumer
// holding a composite key, and that consumer would emit half a key in silence.
type KeyRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`

	// Columns are in key order, and the order is part of the key's identity. A
	// foreign key matches position by position, so a reordered list links the
	// wrong columns without any syntax error to notice.
	Columns []string `json:"columns"`
}

// SingleKey builds the arity-one key.
func SingleKey(schema, table, column string) KeyRef {
	return KeyRef{Schema: schema, Table: table, Columns: []string{column}}
}

// String renders the key as schema.table.column, and as
// schema.table.(a, b) once there is more than one column.
func (r KeyRef) String() string {
	if len(r.Columns) == 1 {
		return r.Schema + "." + r.Table + "." + r.Columns[0]
	}
	return r.Schema + "." + r.Table + ".(" + strings.Join(r.Columns, ", ") + ")"
}

// TableRef renders the owning table as schema.table.
func (r KeyRef) TableRef() string { return r.Schema + "." + r.Table }

// Arity is how many columns the key spans.
func (r KeyRef) Arity() int { return len(r.Columns) }

// Composite reports whether the key spans more than one column.
func (r KeyRef) Composite() bool { return len(r.Columns) > 1 }

// ColumnRefs expands the key into one reference per column, in key order.
func (r KeyRef) ColumnRefs() []ColumnRef {
	out := make([]ColumnRef, 0, len(r.Columns))
	for _, c := range r.Columns {
		out = append(out, ColumnRef{Schema: r.Schema, Table: r.Table, Column: c})
	}
	return out
}

// ForeignKey is a constraint that exists in the catalog. That is not the same
// as a guarantee of integrity: see Validated.
type ForeignKey struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefSchema  string   `json:"ref_schema"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`

	// Validated mirrors pg_constraint.convalidated. False means NOT VALID: the
	// constraint blocks new violations but never checked the rows already
	// there, while looking identical in \d and in any ERD tool.
	Validated bool `json:"validated"`

	// HasIndex reports a usable index on the child side. Without one, every
	// parent delete becomes a sequential scan of the child.
	HasIndex bool `json:"has_index"`
}

// Index is an index as declared in the catalog.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
}

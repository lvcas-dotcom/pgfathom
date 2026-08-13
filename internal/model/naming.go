package model

// MaxNamingExamples caps how many objects back a NamingEvidence. It exists
// to let a reader verify a detected convention by looking at a few real
// tables, not to enumerate every one that matched — that number is already
// in Occurrences. internal/profile is the only detector today, and it caps
// its accumulation at this same value rather than trimming after the fact.
const MaxNamingExamples = 3

// NamingEvidence is one naming convention detected in a schema, with what
// supports it.
type NamingEvidence struct {
	Affix       string  `json:"affix"`
	Occurrences int     `json:"occurrences"`
	Share       float64 `json:"share"`

	// Examples names a few of the objects the convention was read from — a
	// table, or a schema-qualified column — so a reader can check the claim
	// against the schema instead of taking Occurrences on faith. Capped at
	// MaxNamingExamples regardless of how many objects actually matched.
	Examples []string `json:"examples,omitempty"`
}

// NamingDetection is what a schema revealed about its own naming convention.
//
// It belongs to the result rather than to the detector: without it a run is not
// reproducible, because the reader cannot tell which conventions were in effect.
type NamingDetection struct {
	Enabled bool `json:"enabled"`

	ColumnSuffixes []NamingEvidence `json:"column_suffixes,omitempty"`
	ColumnPrefixes []NamingEvidence `json:"column_prefixes,omitempty"`
	TablePrefixes  []NamingEvidence `json:"table_prefixes,omitempty"`

	// PrimaryKeyNames ranks the literal column name a table with a
	// single-column primary key uses, most common first. It is read from
	// declared primary keys only, never from a data probe, which is what lets
	// audit apply it to a synthetic-column suggestion without reading a row.
	PrimaryKeyNames []NamingEvidence `json:"primary_key_names,omitempty"`

	// DeclaredKeys is how many declared foreign keys the reference affixes were
	// read from. Zero means the schema had nothing to say.
	DeclaredKeys int `json:"declared_keys"`
	Tables       int `json:"tables"`

	// SinglePKTables is how many tables PrimaryKeyNames was tabulated from —
	// the population its Share is relative to, not Tables.
	SinglePKTables int `json:"single_pk_tables"`
}

// Empty reports whether nothing was detected.
func (d NamingDetection) Empty() bool {
	return len(d.ColumnSuffixes) == 0 && len(d.ColumnPrefixes) == 0 &&
		len(d.TablePrefixes) == 0 && len(d.PrimaryKeyNames) == 0
}

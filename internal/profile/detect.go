package profile

import (
	"sort"
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// separators end a table-name prefix. Requiring one is what separates a
// convention from a shared subject: auth_ and tpl_ are conventions, while a
// dozen tables starting with "ato" merely talk about the same thing.
var separators = []string{"_"}

// Thresholds are proportions of the relevant population rather than counts:
// three occurrences among eighteen tables is a convention, and the same three
// among eight hundred is noise. The floors keep a tiny schema from turning one
// isolated case into a rule.
const (
	minRefAffixShare    = 0.15
	minRefAffixCount    = 3
	minTablePrefixShare = 0.10
	minTablePrefixCount = 3
	minPKNameShare      = 0.15
	minPKNameCount      = 3
)

// namingAccumulator counts occurrences of a candidate affix or name and keeps
// a few example objects to back it. Examples are capped as they accumulate,
// not trimmed afterward — a schema with thousands of tables never grows one
// past a handful of short strings.
type namingAccumulator struct {
	count    int
	examples []string
}

func accumulate(m map[string]*namingAccumulator, key, example string) {
	a, ok := m[key]
	if !ok {
		a = &namingAccumulator{}
		m[key] = a
	}
	a.count++
	if len(a.examples) < model.MaxNamingExamples {
		a.examples = append(a.examples, example)
	}
}

// Detect derives naming conventions from a schema, using only what the catalog
// read already produced. It issues no queries and reads no data.
func (p *Profile) Detect(schemas []model.Schema) model.NamingDetection {
	d := model.NamingDetection{Enabled: true}

	suffixes, prefixes := map[string]*namingAccumulator{}, map[string]*namingAccumulator{}
	tablePrefixes := map[string]*namingAccumulator{}
	pkNames := map[string]*namingAccumulator{}

	for _, s := range schemas {
		for _, t := range s.Tables {
			d.Tables++

			for _, prefix := range candidateTablePrefixes(t.Name) {
				accumulate(tablePrefixes, prefix, t.Name)
			}

			if t.HasSingleColumnPK() {
				d.SinglePKTables++
				accumulate(pkNames, strings.ToLower(strings.TrimSpace(t.PrimaryKey[0])), t.Name)
			}

			for _, fk := range t.ForeignKeys {
				if len(fk.Columns) != 1 || fk.RefTable == "" {
					continue
				}
				d.DeclaredKeys++

				suffix, prefix, ok := p.referenceAffix(fk.Columns[0], fk.RefTable)
				if !ok {
					continue
				}
				example := t.Name + "." + fk.Columns[0]
				if suffix != "" {
					accumulate(suffixes, suffix, example)
				}
				if prefix != "" {
					accumulate(prefixes, prefix, example)
				}
			}
		}
	}

	d.ColumnSuffixes = rank(suffixes, d.DeclaredKeys, minRefAffixShare, minRefAffixCount)
	d.ColumnPrefixes = rank(prefixes, d.DeclaredKeys, minRefAffixShare, minRefAffixCount)
	d.TablePrefixes = rank(tablePrefixes, d.Tables, minTablePrefixShare, minTablePrefixCount)
	d.PrimaryKeyNames = rank(pkNames, d.SinglePKTables, minPKNameShare, minPKNameCount)

	return d
}

// referenceAffix compares a child column name with the forms of its target
// table and returns the residue around the match.
//
// A database with hundreds of declared keys is stating its own convention;
// reading the residue is taking that statement at face value, with no guessing
// involved.
func (p *Profile) referenceAffix(column, refTable string) (suffix, prefix string, ok bool) {
	column = strings.ToLower(strings.TrimSpace(column))

	for _, form := range p.TableForms(refTable) {
		f := form.Value
		if f == "" || f == column {
			return "", "", false
		}

		switch {
		case strings.HasPrefix(column, f):
			return column[len(f):], "", true
		case strings.HasSuffix(column, f):
			return "", column[:len(column)-len(f)], true
		}
	}
	return "", "", false
}

// candidateTablePrefixes returns every prefix of a table name that ends in a
// separator.
func candidateTablePrefixes(table string) []string {
	name := strings.ToLower(strings.TrimSpace(table))

	var out []string
	for i, r := range name {
		if !isSeparator(r) {
			continue
		}
		prefix := name[:i+1]
		if prefix != name {
			out = append(out, prefix)
		}
	}
	return out
}

func isSeparator(r rune) bool {
	for _, s := range separators {
		if string(r) == s {
			return true
		}
	}
	return false
}

// rank keeps the candidates frequent enough to be a convention, strongest first.
func rank(counts map[string]*namingAccumulator, population int, minShare float64, minCount int) []model.NamingEvidence {
	if population <= 0 {
		return nil
	}

	out := make([]model.NamingEvidence, 0, len(counts))
	for affix, a := range counts {
		share := float64(a.count) / float64(population)
		if a.count < minCount || share < minShare {
			continue
		}
		out = append(out, model.NamingEvidence{Affix: affix, Occurrences: a.count, Share: share, Examples: a.examples})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Occurrences != out[j].Occurrences {
			return out[i].Occurrences > out[j].Occurrences
		}
		return out[i].Affix < out[j].Affix
	})
	return out
}

// WithDetected returns a copy of the profile carrying the detected conventions
// as well as its own.
//
// The merge only ever adds. TableForms already returns a set, so an affix
// detected wrongly costs one spurious candidate form — noise that scoring
// penalizes and validation knocks down. Removing or overriding a base rule
// would cost a candidate that never gets generated, and one that never gets
// generated is invisible.
func (p *Profile) WithDetected(d model.NamingDetection) *Profile {
	merged := *p
	merged.ColumnSuffixes = mergeAffixes(p.ColumnSuffixes, d.ColumnSuffixes, longestFirstSuffix)
	merged.ColumnPrefixes = mergeAffixes(p.ColumnPrefixes, d.ColumnPrefixes, longestFirstPrefix)
	merged.TablePrefixes = mergeAffixes(p.TablePrefixes, d.TablePrefixes, longestFirstPrefix)
	return &merged
}

// mergeAffixes adds what is new, then reorders so no affix shadows a longer one
// declared after it — the same rule Validate enforces on a hand-written profile.
func mergeAffixes(base []string, detected []model.NamingEvidence, order func(a, b string) bool) []string {
	seen := make(map[string]bool, len(base)+len(detected))
	out := make([]string, 0, len(base)+len(detected))

	for _, a := range base {
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	for _, e := range detected {
		if !seen[e.Affix] {
			seen[e.Affix] = true
			out = append(out, e.Affix)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return order(out[i], out[j]) })
	return out
}

func longestFirstSuffix(a, b string) bool {
	if strings.HasSuffix(b, a) && len(b) > len(a) {
		return false
	}
	if strings.HasSuffix(a, b) && len(a) > len(b) {
		return true
	}
	return false
}

func longestFirstPrefix(a, b string) bool {
	if strings.HasPrefix(b, a) && len(b) > len(a) {
		return false
	}
	if strings.HasPrefix(a, b) && len(a) > len(b) {
		return true
	}
	return false
}

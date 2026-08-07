package infer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
)

// Options configures generation and scoring.
type Options struct {
	Profile *profile.Profile

	// MinScore is the cut below which a candidate is discarded with a reason.
	MinScore float64

	// SmallTableRows is the size below which a target counts as a domain table
	// for the generic-name penalty.
	SmallTableRows int64

	// Evidence is join predicates mined from SQL the database stores. Each one
	// strengthens the matching candidate or creates the candidate that name
	// matching structurally cannot reach.
	Evidence []model.JoinEvidence
}

func (o Options) minScore() float64 {
	if o.MinScore <= 0 {
		return DefaultMinScore
	}
	return o.MinScore
}

func (o Options) smallTableRows() int64 {
	if o.SmallTableRows <= 0 {
		return DefaultSmallTableRows
	}
	return o.SmallTableRows
}

// SkipReason says why a possible target could not be used.
type SkipReason string

// Targets a single-column relationship cannot point at. They are recorded
// rather than passed over: the relationship may well be real and merely out of
// scope for this version.
const (
	SkipCompositeKey SkipReason = "target has a composite primary key"
	SkipNoKey        SkipReason = "target has no primary key"
)

// Skip records a target that a name matched but that could not be used.
type Skip struct {
	Child  model.ColumnRef
	Target string
	Reason SkipReason
}

// PolymorphicPair is a reference column that only makes sense beside a
// discriminator column, as in documento_id next to documento_tipo.
type PolymorphicPair struct {
	Table           string
	ReferenceColumn string
	TypeColumn      string
}

// Result is what inference produced, including what it threw away.
type Result struct {
	// Candidates survived the score cut, ordered by descending score.
	Candidates []model.Candidate

	// Discarded fell below the cut, each carrying the reason. They are kept so
	// nobody has to wonder why an obvious-looking column was ignored.
	Discarded []model.Candidate

	Skipped     []Skip
	Polymorphic []PolymorphicPair
}

// target is a table a name could refer to.
type target struct {
	table    model.Table
	pkColumn model.Column
}

// Generate produces and scores candidates for the given schemas.
//
// It generates liberally and cuts strictly. Filtering earlier would be cheaper,
// but the asymmetry matters: a candidate cut by the threshold shows up in the
// discarded list with its reason, and a user who disagrees lowers the
// threshold. A candidate that was never generated exists nowhere, and nobody
// can disagree with what they cannot see.
func Generate(schemas []model.Schema, opts Options) *Result {
	res := &Result{}
	if opts.Profile == nil {
		return res
	}

	index := buildTargetIndex(schemas, opts.Profile)

	for _, s := range schemas {
		for _, t := range s.Tables {
			res.Polymorphic = append(res.Polymorphic, polymorphicPairs(t, opts.Profile)...)

			for _, col := range t.Columns {
				if !eligible(t, col) {
					continue
				}
				generateFor(res, index, t, col, opts)
			}
		}
	}

	applyEvidence(res, schemas, opts)

	finalize(res, opts.minScore())
	return res
}

// eligible reports whether a column is worth a hypothesis. A column already
// covered by a declared foreign key needs no inference: the relationship is in
// the catalog, and reprocessing it would only duplicate noise in the report.
func eligible(t model.Table, col model.Column) bool {
	for _, pk := range t.PrimaryKey {
		if strings.EqualFold(pk, col.Name) {
			return false
		}
	}
	for _, fk := range t.ForeignKeys {
		for _, c := range fk.Columns {
			if strings.EqualFold(c, col.Name) {
				return false
			}
		}
	}
	return true
}

// buildTargetIndex maps every candidate form of every table name to the tables
// answering to it, together with how the form was derived.
func buildTargetIndex(schemas []model.Schema, p *profile.Profile) map[string][]indexedTarget {
	index := make(map[string][]indexedTarget)

	for _, s := range schemas {
		for _, t := range s.Tables {
			for _, form := range p.TableForms(t.Name) {
				index[form.Value] = append(index[form.Value], indexedTarget{table: t, origin: form.Origin})
			}
		}
	}
	return index
}

type indexedTarget struct {
	table  model.Table
	origin profile.Origin
}

func generateFor(res *Result, index map[string][]indexedTarget, t model.Table, col model.Column, opts Options) {
	entity := opts.Profile.EntityName(col.Name)
	if entity == "" {
		return
	}

	child := model.ColumnRef{Schema: t.Schema, Table: t.Name, Column: col.Name}

	// Collapse to one entry per table: a table can answer to several forms, and
	// the strongest match is the one that counts.
	best := make(map[string]indexedTarget)
	for _, it := range index[entity] {
		key := it.table.Schema + "." + it.table.Name
		if prev, seen := best[key]; !seen || it.origin < prev.origin {
			best[key] = it
		}
	}

	usable := make([]target, 0, len(best))
	origins := make(map[string]profile.Origin, len(best))

	names := make([]string, 0, len(best))
	for name := range best {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		it := best[name]

		switch {
		case len(it.table.PrimaryKey) > 1:
			res.Skipped = append(res.Skipped, Skip{Child: child, Target: name, Reason: SkipCompositeKey})
			continue
		case len(it.table.PrimaryKey) == 0:
			res.Skipped = append(res.Skipped, Skip{Child: child, Target: name, Reason: SkipNoKey})
			continue
		}

		pk, ok := it.table.Column(it.table.PrimaryKey[0])
		if !ok {
			continue
		}
		usable = append(usable, target{table: it.table, pkColumn: pk})
		origins[name] = it.origin
	}

	// Ambiguity is kept rather than resolved by guesswork. Picking the largest
	// or the same-schema table would be a hunch dressed as a decision, and it
	// would hide from the user that there was any uncertainty at all.
	ambiguous := len(usable) > 1

	for _, tgt := range usable {
		match := CompareTypes(col.BaseType, tgt.pkColumn.BaseType)
		if !match.Compatible() {
			continue
		}

		key := tgt.table.Schema + "." + tgt.table.Name
		signals := buildSignals(t, col, tgt, origins[key], match, ambiguous, entity, opts)

		res.Candidates = append(res.Candidates, model.Candidate{
			Child: child,
			Parent: model.ColumnRef{
				Schema: tgt.table.Schema,
				Table:  tgt.table.Name,
				Column: tgt.pkColumn.Name,
			},
			Signals:   signals,
			MetaScore: score(signals),
			Verdict:   model.VerdictUnvalidated,
			Reason:    "not validated against data",
		})
	}
}

func buildSignals(t model.Table, col model.Column, tgt target, origin profile.Origin,
	match TypeMatch, ambiguous bool, entity string, opts Options) []model.Signal {

	signals := make([]model.Signal, 0, 6)

	if origin.Exact() {
		signals = append(signals, model.Signal{
			Kind: model.SigExactName, Weight: weightExactName, Detail: tgt.table.Name,
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigNormalizedName, Weight: weightNormalizedName,
			Detail: tgt.table.Name + " via " + origin.String(),
		})
	}

	if match == TypeIdentical {
		signals = append(signals, model.Signal{
			Kind: model.SigIdenticalType, Weight: weightIdenticalType, Detail: col.BaseType,
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigCompatibleType, Weight: weightCompatibleType,
			Detail: col.BaseType + " into " + tgt.pkColumn.BaseType,
		})
	}

	if ambiguous {
		signals = append(signals, model.Signal{
			Kind: model.SigAmbiguousTarget, Weight: penaltyAmbiguousTarget, Detail: entity,
		})
	} else {
		signals = append(signals, model.Signal{
			Kind: model.SigUniqueTarget, Weight: weightUniqueTarget, Detail: entity,
		})
	}

	if t.IsIndexedLeading(col.Name) {
		signals = append(signals, model.Signal{
			Kind: model.SigChildIndexed, Weight: weightChildIndexed, Detail: col.Name,
		})
	}

	if mentions(col.Comment, entity) || mentions(t.Comment, entity) {
		signals = append(signals, model.Signal{
			Kind: model.SigCommentMention, Weight: weightCommentMention, Detail: entity,
		})
	}

	if !col.Nullable {
		signals = append(signals, model.Signal{
			Kind: model.SigNotNull, Weight: weightNotNull,
		})
	}

	// A generic name pointing at a small table is almost always a real
	// relationship, and almost always the least interesting one in the schema.
	// Excluding it would be wrong — some domain tables do have orphans — but
	// leaving it unpenalized would let dozens of them push the findings that
	// justify the tool to the bottom of the report.
	//
	// The penalty needs both halves: a generic name pointing at a large table
	// is not a domain table. An unanalyzed table has no size at all, and
	// guessing "small" there would penalize on nothing.
	rows, known := tgt.table.Stats.EstimatedRowCount()
	if isGeneric(entity, opts.Profile) && known && rows < opts.smallTableRows() {
		signals = append(signals, model.Signal{
			Kind: model.SigGenericDomain, Weight: penaltyGenericDomain, Detail: entity,
		})
	}

	return signals
}

func mentions(comment, entity string) bool {
	if comment == "" || entity == "" {
		return false
	}
	return strings.Contains(strings.ToLower(comment), strings.ToLower(entity))
}

func isGeneric(entity string, p *profile.Profile) bool {
	for _, g := range p.GenericEntities {
		if strings.EqualFold(g, entity) {
			return true
		}
	}
	return false
}

// polymorphicPairs finds reference columns that only make sense beside a
// discriminator column.
//
// Validation would find low containment and reject, which is the right outcome
// by the wrong route: the user reads "name coincidence" where a real
// relationship exists that this version does not model. Naming the pattern
// turns a false dismissal into a useful observation.
func polymorphicPairs(t model.Table, p *profile.Profile) []PolymorphicPair {
	if len(p.TypeSuffixes) == 0 {
		return nil
	}

	var out []PolymorphicPair

	for _, col := range t.Columns {
		entity := p.EntityName(col.Name)
		if entity == "" || strings.EqualFold(entity, col.Name) {
			// No reference affix was stripped, so this is not a reference column.
			continue
		}

		for _, suffix := range p.TypeSuffixes {
			sibling := entity + suffix
			if _, ok := t.Column(sibling); !ok {
				continue
			}
			out = append(out, PolymorphicPair{
				Table:           t.Ref(),
				ReferenceColumn: col.Name,
				TypeColumn:      sibling,
			})
			break
		}
	}

	return out
}

// finalize splits candidates at the threshold and orders everything
// deterministically. Unstable ordering would break the golden files of later
// phases and make any report diff unreadable.
func finalize(res *Result, minScore float64) {
	survivors := res.Candidates[:0:0]

	for _, c := range res.Candidates {
		if c.MetaScore >= minScore {
			survivors = append(survivors, c)
			continue
		}
		c.Reason = fmt.Sprintf("score %.2f below threshold %.2f", c.MetaScore, minScore)
		res.Discarded = append(res.Discarded, c)
	}

	res.Candidates = survivors

	sortCandidates(res.Candidates)
	sortCandidates(res.Discarded)

	sort.SliceStable(res.Skipped, func(i, j int) bool {
		if res.Skipped[i].Child.String() != res.Skipped[j].Child.String() {
			return res.Skipped[i].Child.String() < res.Skipped[j].Child.String()
		}
		return res.Skipped[i].Target < res.Skipped[j].Target
	})

	sort.SliceStable(res.Polymorphic, func(i, j int) bool {
		if res.Polymorphic[i].Table != res.Polymorphic[j].Table {
			return res.Polymorphic[i].Table < res.Polymorphic[j].Table
		}
		return res.Polymorphic[i].ReferenceColumn < res.Polymorphic[j].ReferenceColumn
	})
}

func sortCandidates(candidates []model.Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].MetaScore != candidates[j].MetaScore {
			return candidates[i].MetaScore > candidates[j].MetaScore
		}
		if candidates[i].Child.String() != candidates[j].Child.String() {
			return candidates[i].Child.String() < candidates[j].Child.String()
		}
		return candidates[i].Parent.String() < candidates[j].Parent.String()
	})
}

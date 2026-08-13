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

	// MinNameSimilarity is the cut below which the lexical-similarity fallback
	// never raises a candidate at all. Distinct from MinScore, and far more
	// permissive by design — see DefaultMinNameSimilarity.
	MinNameSimilarity float64

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

func (o Options) minNameSimilarity() float64 {
	if o.MinNameSimilarity <= 0 {
		return DefaultMinNameSimilarity
	}
	return o.MinNameSimilarity
}

// SkipReason says why a possible target could not be used.
type SkipReason string

// Targets that were recognized and could not be used. They are recorded rather
// than passed over: the relationship may well be real, and a near miss is where
// the recall that got away is legible.
const (
	SkipNoKey SkipReason = "target has no primary key"

	// SkipArityMismatch is a name matching a target whose key is composite,
	// from a single column. The composite pass reaches that target by the key's
	// own columns, not by this name, so the two never meet.
	SkipArityMismatch SkipReason = "name matches a target whose key is composite"

	// SkipPartialKey is a composite key with a counterpart for some of its
	// positions and not all. Proposing the partial constraint would propose one
	// that rejects valid rows.
	SkipPartialKey SkipReason = "only part of the target's composite key has a counterpart"

	// SkipAmbiguousPosition is more than one way to resolve the same key
	// against the same child, which is what a coincidence looks like.
	SkipAmbiguousPosition SkipReason = "the target's key resolves against this child in more than one way"

	// SkipAmbiguousSignature is several tables answering to the same composite
	// key. With no name anchoring the child to any of them, more than one could
	// reach total containment, and confirming both would confirm one that is
	// not there.
	SkipAmbiguousSignature SkipReason = "more than one table carries this composite key"
)

// Skip records a target that was recognized but that could not be used.
type Skip struct {
	Child  model.KeyRef
	Target string
	Reason SkipReason

	// Detail carries catalog names and counts only.
	Detail string
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
	tables := flattenTables(schemas)

	for _, s := range schemas {
		for _, t := range s.Tables {
			res.Polymorphic = append(res.Polymorphic, polymorphicPairs(t, opts.Profile)...)

			for _, col := range t.Columns {
				if !eligible(t, col) {
					continue
				}
				generateFor(res, index, tables, t, col, opts)
			}
		}
	}

	generateComposite(res, schemas, opts)
	applyEvidence(res, schemas, opts)

	finalize(res, opts.minScore())
	return res
}

// eligible reports whether a column is worth a hypothesis.
//
// A column already covered by a declared foreign key needs no inference: the
// relationship is in the catalog, and reprocessing it would only duplicate
// noise in the report.
//
// A single-column primary key is a surrogate and points at nothing, so it is
// out. A column that merely takes part in a composite one is in: excluding it
// would throw away the identifying relationship — item(nota_id, seq) keyed on
// both and referencing nota through the first — which is why most composite
// keys exist in the first place.
func eligible(t model.Table, col model.Column) bool {
	if len(t.PrimaryKey) == 1 && strings.EqualFold(t.PrimaryKey[0], col.Name) {
		return false
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

// flattenTables lists every table across every schema in scope, in the order
// Generate already walks them. The affix index is keyed by normalized form
// and cannot be searched by proximity — the similarity fallback needs a
// plain list to scan instead.
// similarityTarget is a table with the trigrams of its own name already
// extracted. The fallback compares one column against every one of these, so
// extracting them inside that loop would extract the same set once per column
// in the database rather than once per table.
type similarityTarget struct {
	table    model.Table
	trigrams map[string]bool
}

func flattenTables(schemas []model.Schema) []similarityTarget {
	var tables []similarityTarget
	for _, s := range schemas {
		for _, t := range s.Tables {
			tables = append(tables, similarityTarget{table: t, trigrams: trigramSet(t.Name)})
		}
	}
	return tables
}

// nameMatch is a table that answered a column's entity name, however it was
// found, paired with the name signal that match earns.
type nameMatch struct {
	tgt    target
	signal model.Signal
}

// resolveKeyTarget checks whether a named table can anchor a single-column
// candidate: it must carry a primary key, and that key must be a single
// column. Composite and missing keys are recorded as a Skip rather than
// dropped in silence — the relationship may well be real, and this is where a
// near miss stays legible.
func resolveKeyTarget(child model.KeyRef, targetName string, table model.Table) (tgt target, skip *Skip, ok bool) {
	switch {
	case len(table.PrimaryKey) > 1:
		// The composite pass is the one that can reach this target, and it
		// looks for the key's own columns rather than for this name. A note
		// here is what keeps the gap between the two visible.
		return target{}, &Skip{Child: child, Target: targetName, Reason: SkipArityMismatch}, false
	case len(table.PrimaryKey) == 0:
		return target{}, &Skip{Child: child, Target: targetName, Reason: SkipNoKey}, false
	}

	pk, found := table.Column(table.PrimaryKey[0])
	if !found {
		return target{}, nil, false
	}
	return target{table: table, pkColumn: pk}, nil, true
}

func generateFor(res *Result, index map[string][]indexedTarget, tables []similarityTarget, t model.Table, col model.Column, opts Options) {
	entity := opts.Profile.EntityName(col.Name)
	if entity == "" {
		return
	}

	child := model.SingleKey(t.Schema, t.Name, col.Name)

	var matches []nameMatch
	if indexed := index[entity]; len(indexed) > 0 {
		matches = resolveByAffix(res, indexed, child)
	} else {
		// The affix index found nothing at all for this entity — never when it
		// found something and every hit was skipped for arity or a missing key,
		// which already has its own Skip explaining why. Trying a second route
		// at that point would risk confusing "why was this table ignored" with
		// "why did this other one appear from nowhere".
		matches = resolveBySimilarity(res, tables, entity, child, opts)
	}

	finalizeMatches(res, matches, t, col, entity, opts)
}

// resolveByAffix collapses the profile index to one entry per table — a table
// can answer to several forms, and the strongest match is the one that counts
// — then resolves each against its primary key.
func resolveByAffix(res *Result, indexed []indexedTarget, child model.KeyRef) []nameMatch {
	best := make(map[string]indexedTarget)
	for _, it := range indexed {
		key := it.table.Schema + "." + it.table.Name
		if prev, seen := best[key]; !seen || it.origin < prev.origin {
			best[key] = it
		}
	}

	names := make([]string, 0, len(best))
	for name := range best {
		names = append(names, name)
	}
	sort.Strings(names)

	matches := make([]nameMatch, 0, len(names))
	for _, name := range names {
		it := best[name]

		tgt, skip, ok := resolveKeyTarget(child, name, it.table)
		if skip != nil {
			res.Skipped = append(res.Skipped, *skip)
		}
		if !ok {
			continue
		}

		matches = append(matches, nameMatch{tgt: tgt, signal: nameSignalFromOrigin(it.origin, it.table.Name)})
	}
	return matches
}

// resolveBySimilarity is the fallback the profile index cannot serve: it
// scans every table in scope by lexical proximity to the entity name instead
// of the profile's affix/plural forms. Only called by generateFor when the
// affix index found nothing.
func resolveBySimilarity(res *Result, tables []similarityTarget, entity string, child model.KeyRef, opts Options) []nameMatch {
	type scored struct {
		table      model.Table
		similarity float64
	}

	// Extracted once for the column, against sets the targets already carry.
	entityTrigrams := trigramSet(entity)

	cutoff := opts.minNameSimilarity()
	var above []scored
	for _, target := range tables {
		similarity := dice(entityTrigrams, target.trigrams)
		if similarity >= cutoff {
			above = append(above, scored{table: target.table, similarity: similarity})
		}
	}

	// Deterministic order: strongest similarity first, ties broken by name —
	// the same discipline sortCandidates applies to the final output.
	sort.SliceStable(above, func(i, j int) bool {
		if above[i].similarity != above[j].similarity {
			return above[i].similarity > above[j].similarity
		}
		return above[i].table.Name < above[j].table.Name
	})

	matches := make([]nameMatch, 0, len(above))
	for _, s := range above {
		name := s.table.Schema + "." + s.table.Name

		tgt, skip, ok := resolveKeyTarget(child, name, s.table)
		if skip != nil {
			res.Skipped = append(res.Skipped, *skip)
		}
		if !ok {
			continue
		}

		matches = append(matches, nameMatch{tgt: tgt, signal: nameSignalFromSimilarity(s.table.Name, s.similarity)})
	}
	return matches
}

// finalizeMatches turns resolved name matches into scored candidates. Shared
// by every match source — profile affix or lexical fallback — because type
// compatibility, ambiguity and the rest of the signal set do not depend on
// how the name was found.
func finalizeMatches(res *Result, matches []nameMatch, t model.Table, col model.Column, entity string, opts Options) {
	// Ambiguity is kept rather than resolved by guesswork. Picking the largest
	// or the same-schema table would be a hunch dressed as a decision, and it
	// would hide from the user that there was any uncertainty at all.
	ambiguous := len(matches) > 1

	for _, m := range matches {
		match := CompareTypes(col.BaseType, m.tgt.pkColumn.BaseType)
		if !match.Compatible() {
			continue
		}

		signals := buildSignals(m.signal, t, col, m.tgt, match, ambiguous, entity, opts)

		res.Candidates = append(res.Candidates, model.Candidate{
			Child:     model.SingleKey(t.Schema, t.Name, col.Name),
			Parent:    model.SingleKey(m.tgt.table.Schema, m.tgt.table.Name, m.tgt.pkColumn.Name),
			Signals:   signals,
			MetaScore: score(signals),
			Verdict:   model.VerdictUnvalidated,
			Reason:    "not validated against data",
		})
	}
}

func nameSignalFromOrigin(origin profile.Origin, tableName string) model.Signal {
	if origin.Exact() {
		return model.Signal{Kind: model.SigExactName, Weight: weightExactName, Detail: tableName}
	}
	return model.Signal{
		Kind: model.SigNormalizedName, Weight: weightNormalizedName,
		Detail: tableName + " via " + origin.String(),
	}
}

func nameSignalFromSimilarity(tableName string, similarity float64) model.Signal {
	return model.Signal{
		Kind: model.SigNameSimilarity, Weight: nameSimilarityWeight(similarity),
		Detail: tableName + " via lexical similarity",
	}
}

func buildSignals(nameSignal model.Signal, t model.Table, col model.Column, tgt target,
	match TypeMatch, ambiguous bool, entity string, opts Options) []model.Signal {

	signals := make([]model.Signal, 0, 6)
	signals = append(signals, nameSignal)

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

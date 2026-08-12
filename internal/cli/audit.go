package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/audit"
	"github.com/lvcas-dotcom/pgfathom/internal/buildinfo"
	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/sqlprobe"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// Defaults for the key-probing flags, declared as estimates like validate's
// own defaults and revisited with the benchmark corpus.
const (
	// DefaultProbeKeysMaxRows is the estimated-row ceiling above which a
	// table's missing key is reported without a data probe: a full scan of a
	// table this size is a cost the command should never impose by default.
	DefaultProbeKeysMaxRows = 5_000_000

	// DefaultRecurrenceMin is how many distinct views, functions or
	// statements must name a column before it counts as hot.
	DefaultRecurrenceMin = 2

	// maxKeyProbeCandidatesPerTable caps how many full-scan uniqueness probes
	// one table can receive in a single run. Probing is rare enough, and a
	// full scan costly enough, that trying every non-unique index plus every
	// column would turn one missing key into a burst of table scans.
	maxKeyProbeCandidatesPerTable = 3
)

type auditOptions struct {
	connection       connectionOptions
	profile          string
	format           string
	out              string
	noProbeKeys      bool
	probeKeysMaxRows int64
	recurrenceMin    int
}

func newAuditCommand(streams *Streams) *cobra.Command {
	opts := &auditOptions{
		connection:       defaultConnectionOptions(),
		profile:          profile.DefaultName,
		format:           "table",
		probeKeysMaxRows: DefaultProbeKeysMaxRows,
		recurrenceMin:    DefaultRecurrenceMin,
	}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report structural findings that need no inference",
		Long: `Report structural findings taken straight from the catalog and from usage
evidence already resolved against it.

audit makes no inferences: every finding it emits is a fact, not a hypothesis.
It reports foreign keys declared NOT VALID and never verified, foreign keys
with no index on the child side, tables with no primary key, and columns real
code repeatedly names in a join or filter predicate with no index leading them.

For a table with no primary key and no promotable unique, audit confirms a
candidate key by counting rows — never sampled, and never affirmed unless a
full scan proves it. This is the one place the command reads table data; pass
--no-probe-keys to keep it catalog-only.

When run at an interactive terminal, every table that reaches the end of that
path with no key confirmed is resolved once, together, before the run
continues: audit reports how many tables are still unresolved, how many have
an untested composite candidate built from their own foreign keys, and what
the schema's declared keys already say a primary key is usually called, then
asks once whether to recommend the composite candidates, a synthetic column
named after that same convention, or neither. A synthetic column is never
named by typing one in; it always follows the convention the rest of the
schema already uses. Piped output, redirected input, and non-interactive runs
never see a prompt.

Relationship inference is a separate command; one does not substitute for the
other.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAudit(cmd.Context(), streams, opts)
		},
	}

	registerConnectionFlags(cmd, &opts.connection)

	f := cmd.Flags()
	f.StringVar(&opts.profile, "profile", opts.profile,
		"naming profile: a built-in name or a path to a TOML file; only used to name a synthetic primary key")
	f.StringVar(&opts.format, "format", opts.format, "output format: table, json or sql")
	f.StringVar(&opts.out, "out", "",
		"directory for the reviewable .sql artifacts; required by --format sql")
	f.BoolVar(&opts.noProbeKeys, "no-probe-keys", false,
		"never read table data to confirm a candidate primary key; catalog-only suggestions, and no interactive key resolution")
	f.Int64Var(&opts.probeKeysMaxRows, "probe-keys-max-rows", opts.probeKeysMaxRows,
		"tables above this estimated row count are not probed for a missing key")
	f.IntVar(&opts.recurrenceMin, "recurrence-min", opts.recurrenceMin,
		"distinct views/functions/statements a column must appear in before it counts as hot")

	return cmd
}

func runAudit(ctx context.Context, streams *Streams, opts *auditOptions) error {
	if err := checkOutput(opts.format, opts.out); err != nil {
		return err
	}

	naming, err := profile.Load(opts.profile)
	if err != nil {
		return UsageError(err)
	}

	started := time.Now()

	warn := func(msg string) { _, _ = fmt.Fprintln(streams.Err, "warning: "+msg) }

	pool, scope, err := connect(ctx, opts.connection, warn)
	if err != nil {
		return err
	}
	defer pool.Close()

	cat, err := catalog.Read(ctx, pool, catalog.Options{Scope: scope, Exclude: opts.connection.exclude})
	if err != nil {
		return err
	}

	// Usage evidence is catalog and view/function source, never a user row. A
	// failure to read it costs signal — fewer hot-column findings — never
	// correctness, so it degrades to a warning like it does in discover.
	var evidence sqlprobe.Evidence
	if probed, err := sqlprobe.Probe(ctx, pool, cat.Schemas); err != nil {
		warn("usage evidence skipped: " + err.Error())
	} else {
		evidence = *probed
	}

	coverage := cat.Coverage
	coverage.PgStatStatements = evidence.StatementsAvailable

	findings := audit.Findings(cat.Schemas, audit.Options{
		Joins:         evidence.Joins,
		Predicates:    evidence.Predicates,
		Extensions:    cat.Extensions,
		RecurrenceMin: opts.recurrenceMin,
	})

	if !opts.noProbeKeys {
		findings, err = probeMissingKeys(ctx, pool, cat.Schemas, findings, opts, &coverage, warn)
		if err != nil {
			return err
		}

		if streams.Interactive {
			detection := naming.Detect(cat.Schemas)
			if err := resolveUnconfirmedKeys(ctx, streams, pool, cat.Schemas, findings, detection, opts); err != nil {
				return err
			}
		}
	}

	version, _, _ := buildinfo.Resolve()
	result := model.NewResult(version, "", time.Now().UTC(), coverage)
	result.ServerVersion = pool.ServerVersion()
	result.Schemas = cat.Schemas
	result.Findings = findings
	result.Duration = time.Since(started)

	if opts.out != "" {
		artifacts := report.AuditArtifacts(result)
		if err := report.WriteArtifacts(opts.out, artifacts); err != nil {
			return err
		}
		if opts.format == formatSQL {
			return report.Manifest(streams.Out, opts.out, artifacts)
		}
	}

	if opts.format == formatJSON {
		return report.JSON(streams.Out, result)
	}
	return report.Terminal(streams.Out, result, report.Emphasis(streams.Color()))
}

// probeMissingKeys confirms, by counting rows, a candidate key for every
// missing_primary_key finding that has no unique to promote. A table above
// the configured row ceiling is left unprobed and recorded in coverage
// instead: this is the only place audit reads table data, and it stays
// bounded by default.
func probeMissingKeys(ctx context.Context, pool *db.Pool, schemas []model.Schema, findings []model.Finding,
	opts *auditOptions, coverage *model.Coverage, warn func(string),
) ([]model.Finding, error) {
	for i := range findings {
		f := &findings[i]
		if f.Kind != model.FindingMissingPrimaryKey || f.Suggestion == nil ||
			f.Suggestion.Kind != model.SuggestCreatePrimaryKey {
			continue
		}

		table, ok := tableByRef(schemas, f.Object)
		if !ok {
			continue
		}

		rows, known := table.Stats.EstimatedRowCount()
		if !known || rows > opts.probeKeysMaxRows {
			coverage.KeyProbesSkipped = append(coverage.KeyProbesSkipped, model.SkippedKeyProbe{
				Table:  f.Object,
				Reason: "exceeds --probe-keys-max-rows",
			})
			continue
		}

		candidates := candidateKeys(table, maxKeyProbeCandidatesPerTable)
		if len(candidates) == 0 {
			continue
		}

		results, err := validate.ProbeUniqueness(ctx, pool, table, candidates, opts.connection.statementTimeout)
		if err != nil {
			warn("key probe skipped for " + f.Object + ": " + err.Error())
			continue
		}

		applyKeyProbeResults(f, results)
	}

	return findings, nil
}

// tableByRef looks up a table by its schema-qualified reference, the same
// string missing_primary_key uses as Finding.Object.
func tableByRef(schemas []model.Schema, ref string) (model.Table, bool) {
	for _, s := range schemas {
		for _, t := range s.Tables {
			if t.Ref() == ref {
				return t, true
			}
		}
	}
	return model.Table{}, false
}

// candidateKeys names catalog-only candidate column sets to test for
// uniqueness on a table that has no promotable unique. It prefers the columns
// of an existing non-unique index that are all NOT NULL — the schema already
// grouped them for some reason — and falls back to each individual NOT NULL
// column in declaration order. Either way this only narrows what gets probed:
// confirmation always comes from counting rows, never from this heuristic.
func candidateKeys(t model.Table, budget int) [][]string {
	var out [][]string

	for _, idx := range t.Indexes {
		if idx.Unique || idx.Primary || len(idx.Columns) == 0 {
			continue
		}
		if allColumnsNotNull(t, idx.Columns) {
			out = append(out, idx.Columns)
		}
	}

	if len(out) == 0 {
		for _, c := range t.Columns {
			if !c.Nullable {
				out = append(out, []string{c.Name})
			}
		}
	}

	if len(out) > budget {
		out = out[:budget]
	}
	return out
}

func allColumnsNotNull(t model.Table, columns []string) bool {
	for _, name := range columns {
		col, ok := t.Column(name)
		if !ok || col.Nullable {
			return false
		}
	}
	return true
}

// applyKeyProbeResults costures the probe verdict into the finding's
// suggestion. Columns are only ever populated on a confirmed key: a
// candidate that failed to confirm — whether proven non-unique or merely
// timed out — must never be presented as a named hypothesis, since a reader
// cannot tell those two outcomes apart from the columns alone.
func applyKeyProbeResults(f *model.Finding, results []validate.KeyProbeResult) {
	for _, r := range results {
		if r.Verdict == model.KeyProbeConfirmed {
			f.Suggestion.Columns = r.Columns
			f.Suggestion.KeyProbe = model.KeyProbeConfirmed
			return
		}
	}

	f.Suggestion.KeyProbe = model.KeyProbeUnverified
	if len(results) > 0 {
		f.Suggestion.Note = fmt.Sprintf("tried %d candidate key(s); none confirmed unique", len(results))
	}
}

// unresolvedKey pairs a missing_primary_key finding probeMissingKeys left
// unconfirmed with the table it is about and, when the table qualifies for
// one, an untested composite candidate built from its own single-column
// foreign keys.
type unresolvedKey struct {
	finding     *model.Finding
	table       model.Table
	fkCandidate []string
}

// resolveUnconfirmedKeys asks, at most once per run, what to do about every
// missing_primary_key finding probeMissingKeys left without a confirmed key.
// This is a schema-wide decision, not a per-table one: the operator answers
// for every unresolved table at once, and a synthetic column is always named
// from the convention the schema's own declared keys already establish —
// never typed, the same way any other table in the schema would be named.
// It must only be called when streams.Interactive is true: this is the one
// place audit blocks on user input, and the caller gates that already.
func resolveUnconfirmedKeys(ctx context.Context, streams *Streams, pool *db.Pool, schemas []model.Schema,
	findings []model.Finding, detection model.NamingDetection, opts *auditOptions,
) error {
	var pending []unresolvedKey
	for i := range findings {
		f := &findings[i]
		if f.Kind != model.FindingMissingPrimaryKey || f.Suggestion == nil ||
			f.Suggestion.Kind != model.SuggestCreatePrimaryKey || f.Suggestion.KeyProbe == model.KeyProbeConfirmed {
			continue
		}

		table, ok := tableByRef(schemas, f.Object)
		if !ok {
			continue
		}

		var fkCandidate []string
		if rows, known := table.Stats.EstimatedRowCount(); !known || rows <= opts.probeKeysMaxRows {
			fkCandidate, _ = fkKeyCandidate(table, candidateKeys(table, maxKeyProbeCandidatesPerTable))
		}

		pending = append(pending, unresolvedKey{finding: f, table: table, fkCandidate: fkCandidate})
	}

	if len(pending) == 0 {
		return nil
	}

	withComposite := 0
	for _, p := range pending {
		if len(p.fkCandidate) > 0 {
			withComposite++
		}
	}
	pkName := resolvePKName(detection.PrimaryKeyNames)

	_, _ = fmt.Fprint(streams.Err, formatKeyResolutionSummary(len(pending), withComposite, detection, pkName))

	if withComposite == 0 && pkName == "" {
		// Nothing to recommend either way: no composite candidate anywhere in
		// scope, and no naming convention to name a synthetic column from.
		// Asking would only ever offer skip, so there is nothing to ask.
		return nil
	}

	action, err := promptKeyResolution(streams, withComposite > 0, pkName)
	if err != nil {
		return err
	}

	switch action {
	case keyResolutionComposite:
		for _, p := range pending {
			if len(p.fkCandidate) == 0 {
				continue
			}
			results, err := validate.ProbeUniqueness(ctx, pool, p.table, [][]string{p.fkCandidate}, opts.connection.statementTimeout)
			if err != nil {
				_, _ = fmt.Fprintln(streams.Err, "key probe skipped for "+p.finding.Object+": "+err.Error())
				continue
			}
			applyKeyProbeResults(p.finding, results)
		}
	case keyResolutionSynthetic:
		note := fmt.Sprintf("schema convention: %q names the primary key elsewhere in the schema", pkName)
		if len(detection.PrimaryKeyNames) > 0 {
			top := detection.PrimaryKeyNames[0]
			note = fmt.Sprintf("schema convention: %q names the primary key in %d of %d single-column-PK tables (%.0f%%)%s",
				pkName, top.Occurrences, detection.SinglePKTables, top.Share*100, exampleSuffix(top.Examples))
		}
		for _, p := range pending {
			applySyntheticKey(p.finding, pkName, note)
		}
	}

	return nil
}

// resolvePKName returns the schema's most common single-column primary key
// name, or "" when the schema has none to offer. detection.PrimaryKeyNames is
// already ranked strongest-first by profile.Detect; whatever tops it is what
// every other table in the schema is already named after, so there is
// nothing left to ask about the name itself.
func resolvePKName(evidence []model.NamingEvidence) string {
	if len(evidence) == 0 {
		return ""
	}
	return evidence[0].Affix
}

// fkKeyCandidate names, for a table with two or more single-column declared
// foreign keys, the combination of those columns as a composite key
// candidate — the shape a join or association table's real key usually
// takes, and the one candidateKeys cannot see: it only looks at columns an
// existing index already groups, and a table missing its primary key
// commonly has no index over that pair either, which is often why the key is
// missing in the first place. It returns false when there are fewer than
// two, or when the exact combination was already tried.
func fkKeyCandidate(t model.Table, alreadyTried [][]string) ([]string, bool) {
	var columns []string
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) == 1 {
			columns = append(columns, fk.Columns[0])
		}
	}
	if len(columns) < 2 {
		return nil, false
	}

	for _, tried := range alreadyTried {
		if sameColumnSet(tried, columns) {
			return nil, false
		}
	}
	return columns, true
}

func sameColumnSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, c := range a {
		seen[c] = true
	}
	for _, c := range b {
		if !seen[c] {
			return false
		}
	}
	return true
}

// applySyntheticKey turns a missing_primary_key finding into a synthetic
// column suggestion. It never touches KeyProbe: a brand-new column's
// uniqueness does not depend on any row that already exists, so there is
// nothing for a probe to confirm.
func applySyntheticKey(f *model.Finding, column, note string) {
	f.Suggestion = &model.Suggestion{
		Kind:    model.SuggestSyntheticPrimaryKey,
		Columns: []string{column},
		Note:    note,
	}
}

// exampleSuffix renders the objects a NamingEvidence was read from, so a
// convention cited by audit — applied on its own or offered in a prompt —
// can be checked against the schema instead of taken on faith.
func exampleSuffix(examples []string) string {
	if len(examples) == 0 {
		return ""
	}
	return " (e.g. " + strings.Join(examples, ", ") + ")"
}

// formatKeyResolutionSummary renders the one-time, schema-wide picture the
// operator sees before being asked anything: how many tables are still
// unresolved, how many of those have an untested composite candidate, and
// what the schema itself says a primary key is usually called. It is printed
// once per run, never per table.
func formatKeyResolutionSummary(pending, withComposite int, detection model.NamingDetection, pkName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%d table(s) have no confirmed primary key.\n", pending)
	if withComposite > 0 {
		fmt.Fprintf(&b, "  %d of them have an untested composite candidate from their own foreign keys.\n", withComposite)
	}

	if pkName == "" {
		b.WriteString("  no schema-wide convention detected for a primary key name.\n")
	} else {
		top := detection.PrimaryKeyNames[0]
		fmt.Fprintf(&b, "  schema convention for a primary key name: %q (%d of %d single-column-PK tables, %.0f%%)%s\n",
			pkName, top.Occurrences, detection.SinglePKTables, top.Share*100, exampleSuffix(top.Examples))
	}
	return b.String()
}

// keyResolutionAction is what the operator's answer to the key resolution
// prompt resolved to. It applies to every unresolved table in the run at
// once — this is a schema-wide decision, not a per-table one.
type keyResolutionAction int

const (
	keyResolutionSkip keyResolutionAction = iota
	keyResolutionComposite
	keyResolutionSynthetic
)

// formatKeyResolutionPrompt renders the menu, adapted to what is actually on
// offer: the composite option only appears when at least one pending table
// has a candidate, and the synthetic option only when the schema has a
// convention to name it from. Skip is always available, since it is what
// happens today when nothing is confirmed.
func formatKeyResolutionPrompt(compositeAvailable bool, pkName string) string {
	var b strings.Builder
	if compositeAvailable {
		b.WriteString("  [a] recommend a composite primary key wherever a candidate exists\n")
	}
	if pkName != "" {
		fmt.Fprintf(&b, "  [b] recommend a new %q primary key column for the rest\n", pkName)
	}
	b.WriteString("  [enter] skip these tables\n> ")
	return b.String()
}

// parseKeyResolutionAnswer turns one line of operator input into a
// resolution action, with no I/O of its own — the shape that makes it
// testable without a terminal. ok is false when the line matches neither the
// available options nor a skip, which is the caller's signal to say so and
// ask again rather than silently guessing.
func parseKeyResolutionAnswer(line string, compositeAvailable bool, pkName string) (action keyResolutionAction, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return keyResolutionSkip, true
	}

	switch {
	case compositeAvailable && strings.EqualFold(line, "a"):
		return keyResolutionComposite, true
	case pkName != "" && strings.EqualFold(line, "b"):
		return keyResolutionSynthetic, true
	}
	return keyResolutionSkip, false
}

// promptKeyResolution asks the key resolution question, reprompting on an
// answer that is not recognized instead of silently treating it as a skip —
// a mistyped character is not the same as a deliberate skip, and this is the
// only place in a run the operator gets to say which one it was. Stdin
// closing outright resolves to skip: ReadString then returns an empty line
// alongside io.EOF, which parses as skip on its own, so there is nothing
// further to special-case.
func promptKeyResolution(streams *Streams, compositeAvailable bool, pkName string) (keyResolutionAction, error) {
	reader := bufio.NewReader(streams.In)
	prompt := formatKeyResolutionPrompt(compositeAvailable, pkName)

	for {
		_, _ = fmt.Fprint(streams.Err, prompt)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return keyResolutionSkip, fmt.Errorf("reading key resolution answer: %w", err)
		}

		if action, ok := parseKeyResolutionAnswer(line, compositeAvailable, pkName); ok {
			return action, nil
		}
		_, _ = fmt.Fprintln(streams.Err, "invalid answer")
	}
}

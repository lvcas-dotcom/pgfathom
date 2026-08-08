package report

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// File names are fixed, so running twice into the same directory overwrites
// rather than accumulating. Comparing two runs is the point; a directory that
// grows a timestamped file per execution makes that harder, not easier.
const (
	FileConfirmed = "confirmed.sql"
	FileBroken    = "broken.sql"
	FileNotValid  = "not_valid.sql"
)

// maxIdentifierBytes is NAMEDATALEN-1. The server truncates past this without
// saying so, which turns two distinct long names into one and makes the second
// ADD CONSTRAINT fail as a duplicate.
const maxIdentifierBytes = 63

// Artifact is one generated file: its name, how many findings it covers, and
// its full text. Generation is a pure function of the model, so the whole
// content can be asserted without touching a filesystem or a server.
type Artifact struct {
	Name    string
	Count   int
	Content string
}

// DiscoverArtifacts renders the reviewable SQL for an inference run: one file
// per category, always both, even when a category is empty.
func DiscoverArtifacts(r *model.Result) []Artifact {
	h := newHeader(r)
	confirmed := r.CandidatesByVerdict(model.VerdictConfirmed)
	broken := r.CandidatesByVerdict(model.VerdictBroken)

	return []Artifact{
		{Name: FileConfirmed, Count: len(confirmed), Content: confirmedFile(h, r, confirmed)},
		{Name: FileBroken, Count: len(broken), Content: brokenFile(h, broken)},
	}
}

// AuditArtifacts renders the SQL for the structural audit: the validation of
// every constraint the catalog carries as NOT VALID.
func AuditArtifacts(r *model.Result) []Artifact {
	h := newHeader(r)
	pending := notValidKeys(r.Schemas)

	return []Artifact{
		{Name: FileNotValid, Count: len(pending), Content: notValidFile(h, pending)},
	}
}

// WriteArtifacts writes every artifact into dir, creating it when absent.
func WriteArtifacts(dir string, artifacts []Artifact) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", dir, err)
	}
	for _, a := range artifacts {
		path := filepath.Join(dir, a.Name)
		if err := os.WriteFile(path, []byte(a.Content), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

// Manifest is the result in artifact mode: stdout would otherwise be empty on a
// successful run, which reads exactly like a failure.
func Manifest(w io.Writer, dir string, artifacts []Artifact) error {
	var b strings.Builder
	b.WriteString("\n")

	tw := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	for _, a := range artifacts {
		writeRow(tw, filepath.Join(dir, a.Name), fmt.Sprintf("%d", a.Count))
	}
	_ = tw.Flush()

	b.WriteString("\n  Review every file before running any of it.\n\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// header is the block every generated file opens with.
type header struct {
	tool, version, server, mode string
	generated                   time.Time
	sampled                     bool
}

func newHeader(r *model.Result) header {
	sampled := r.Sampled()
	mode := "full — every row was examined"
	switch {
	case sampled:
		mode = "sampled — a subset of rows was examined"
	case len(r.Candidates) == 0:
		mode = "none — this run made no inferences to validate"
	}

	return header{
		tool: r.Tool, version: r.ToolVersion, server: r.ServerVersion,
		mode: mode, generated: r.GeneratedAt, sampled: sampled,
	}
}

func (h header) render(b *strings.Builder, subject string) {
	fmt.Fprintf(b, "-- %s %s — %s\n", h.tool, h.version, subject)
	fmt.Fprintf(b, "-- generated %s\n", h.generated.UTC().Format(time.RFC3339))
	fmt.Fprintf(b, "-- server     PostgreSQL %s\n", nonEmpty(h.server, "unknown"))
	fmt.Fprintf(b, "-- validation %s\n", h.mode)
	b.WriteString("--\n")
	b.WriteString("-- REVIEW BEFORE RUNNING. Every statement below was written from an\n")
	b.WriteString("-- inference about your schema. This tool has never altered your database\n")
	b.WriteString("-- and does not intend to; you are the one who decides what runs.\n")

	if h.sampled {
		b.WriteString("--\n")
		b.WriteString("-- ! THIS RUN WAS SAMPLED. Nothing here is confirmed, and every orphan\n")
		b.WriteString("--   count is a floor rather than a total: orphans arrive in batches and\n")
		b.WriteString("--   cluster on the same pages, which is what page sampling is worst at\n")
		b.WriteString("--   finding. Re-run with --full before concluding anything from absence.\n")
	}
	b.WriteString("\n")
}

func confirmedFile(h header, r *model.Result, confirmed []model.Candidate) string {
	var b strings.Builder
	h.render(&b, "relationships the data confirms")

	if len(confirmed) == 0 {
		b.WriteString(emptyNote(h, r))
		return b.String()
	}

	indexed := indexedColumns(r.Schemas)

	for _, c := range confirmed {
		name := constraintName(c)
		writeRelationComment(&b, c, name)

		if v := c.Validation; v != nil {
			fmt.Fprintf(&b, "-- containment %.1f%% of %d rows over %d distinct values, no orphans\n",
				100*v.ContainmentRows(), v.NotNullRows, v.DistinctVals)
		}

		fmt.Fprintf(&b, "ALTER TABLE %s\n", qualify(c.Child))
		fmt.Fprintf(&b, "    ADD CONSTRAINT %s\n", ident(name.value))
		fmt.Fprintf(&b, "    FOREIGN KEY (%s) REFERENCES %s (%s)\n",
			ident(c.Child.Column), qualify(c.Parent), ident(c.Parent.Column))
		b.WriteString("    NOT VALID;\n\n")

		b.WriteString("-- Validate separately. The statement above scanned nothing and held the\n")
		b.WriteString("-- stronger lock for an instant; this one scans the table under a lock that\n")
		b.WriteString("-- blocks neither reads nor writes. Run it in a window of your choosing.\n")
		fmt.Fprintf(&b, "-- ALTER TABLE %s VALIDATE CONSTRAINT %s;\n\n",
			qualify(c.Child), ident(name.value))

		if !indexed[indexKey(c.Child)] {
			writeIndexSuggestion(&b, c)
		}
	}

	return b.String()
}

// writeIndexSuggestion emits the index the child side is missing, commented,
// with both traps spelled out. Neither is guessable from the line itself.
func writeIndexSuggestion(b *strings.Builder, c model.Candidate) {
	idx := truncateIdent("ix_" + c.Child.Table + "_" + c.Child.Column)

	fmt.Fprintf(b, "-- No index has %s in leading position. Without one, every DELETE on\n",
		ident(c.Child.Column))
	fmt.Fprintf(b, "-- %s becomes a sequential scan of %s.\n", c.Parent.TableRef(), c.Child.TableRef())
	b.WriteString("--\n")
	b.WriteString("-- CONCURRENTLY does NOT run inside a transaction block: this fails under\n")
	b.WriteString("-- psql --single-transaction, and a failure leaves an INVALID index behind\n")
	b.WriteString("-- that has to be dropped by hand. Run it on its own.\n")
	if idx.truncated {
		fmt.Fprintf(b, "-- Name shortened to fit %d bytes; in full it would be %s.\n",
			maxIdentifierBytes, idx.full)
	}
	fmt.Fprintf(b, "-- CREATE INDEX CONCURRENTLY %s ON %s (%s);\n\n",
		ident(idx.value), qualify(c.Child), ident(c.Child.Column))
}

func brokenFile(h header, broken []model.Candidate) string {
	var b strings.Builder
	h.render(&b, "relationships the data contradicts")

	if len(broken) == 0 {
		b.WriteString("-- This run found no broken relationships.\n")
		return b.String()
	}

	b.WriteString("-- Each block below is a relationship the data supports and whose integrity\n")
	b.WriteString("-- is already violated. The DDL is commented out on purpose: it cannot pass\n")
	b.WriteString("-- while an orphan remains, and what to do with an orphan row — delete it,\n")
	b.WriteString("-- repoint it, null it — is a decision about your domain, not about SQL.\n\n")

	for _, c := range broken {
		name := constraintName(c)
		writeRelationComment(&b, c, name)

		if v := c.Validation; v != nil {
			floor := ""
			if v.Method == model.MethodSampled {
				floor = " (a floor: this was sampled)"
			}
			fmt.Fprintf(&b, "-- %d orphan rows over %d distinct values%s; containment %.1f%% of %d rows examined\n",
				v.OrphanRows, v.OrphanVals, floor, 100*v.ContainmentRows(), v.NotNullRows)
		}

		b.WriteString("\n-- Step 1 — look at what is orphaned.\n")
		b.WriteString(orphanQuery(c))

		b.WriteString("\n-- Step 2 — decide what happens to those rows. This tool does not decide.\n\n")

		b.WriteString("-- Step 3 — only once nothing comes back from step 1, declare the key.\n")
		fmt.Fprintf(&b, "-- ALTER TABLE %s\n", qualify(c.Child))
		fmt.Fprintf(&b, "--     ADD CONSTRAINT %s\n", ident(name.value))
		fmt.Fprintf(&b, "--     FOREIGN KEY (%s) REFERENCES %s (%s)\n",
			ident(c.Child.Column), qualify(c.Parent), ident(c.Parent.Column))
		b.WriteString("--     NOT VALID;\n\n")
	}

	return b.String()
}

// orphanQuery lists the offending values. It carries no literal taken from the
// data: the anti-join finds them at run time, on the user's own screen, which
// is the only place they ever belong.
func orphanQuery(c model.Candidate) string {
	return fmt.Sprintf(`SELECT c.%[1]s AS orphan_value, count(*) AS orphan_rows
FROM %[2]s AS c
WHERE c.%[1]s IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM %[3]s AS p WHERE p.%[4]s = c.%[1]s)
GROUP BY 1
ORDER BY 2 DESC;
`,
		ident(c.Child.Column), qualify(c.Child), qualify(c.Parent), ident(c.Parent.Column))
}

// pendingKey is one declared-but-never-verified constraint.
type pendingKey struct {
	table model.Table
	fk    model.ForeignKey
}

func notValidFile(h header, pending []pendingKey) string {
	var b strings.Builder
	h.render(&b, "constraints declared NOT VALID and never verified")

	if len(pending) == 0 {
		b.WriteString("-- Every declared foreign key in scope has been validated.\n")
		return b.String()
	}

	b.WriteString("-- A NOT VALID constraint blocks new violations but never checked the rows\n")
	b.WriteString("-- that were already there. It renders as an ordinary foreign key in \\d and\n")
	b.WriteString("-- in every ERD tool, which is what makes it worth finding.\n")
	b.WriteString("--\n")
	b.WriteString("-- Running VALIDATE blindly on a table with historical violations fails only\n")
	b.WriteString("-- after a full scan, so the check comes first.\n\n")

	for _, p := range pending {
		fmt.Fprintf(&b, "-- %s — constraint %s references %s\n",
			p.table.Ref(), ident(p.fk.Name), p.fk.RefSchema+"."+p.fk.RefTable)

		b.WriteString("-- Step 1 — would it pass?\n")
		b.WriteString(violationQuery(p))

		b.WriteString("\n-- Step 2 — if step 1 returns zero, validate. It scans the table under a\n")
		b.WriteString("-- lock that blocks neither reads nor writes.\n")
		fmt.Fprintf(&b, "-- ALTER TABLE %s VALIDATE CONSTRAINT %s;\n\n",
			identTable(p.table.Schema, p.table.Name), ident(p.fk.Name))
	}

	return b.String()
}

// violationQuery counts rows the constraint would reject. Composite keys join
// on every column pair, and a NULL in any of them exempts the row, which is
// what MATCH SIMPLE — the default — means.
func violationQuery(p pendingKey) string {
	notNull := make([]string, 0, len(p.fk.Columns))
	match := make([]string, 0, len(p.fk.Columns))

	for i, col := range p.fk.Columns {
		notNull = append(notNull, "c."+ident(col)+" IS NOT NULL")
		if i < len(p.fk.RefColumns) {
			match = append(match, "p."+ident(p.fk.RefColumns[i])+" = c."+ident(col))
		}
	}

	return fmt.Sprintf(`SELECT count(*) AS violating_rows
FROM %s AS c
WHERE %s
  AND NOT EXISTS (SELECT 1 FROM %s AS p WHERE %s);
`,
		identTable(p.table.Schema, p.table.Name),
		strings.Join(notNull, "\n  AND "),
		identTable(p.fk.RefSchema, p.fk.RefTable),
		strings.Join(match, " AND "))
}

// emptyNote states what an empty category means. In sampled mode the count is
// not the story: the mode is, because it could not have confirmed anything.
func emptyNote(h header, r *model.Result) string {
	if h.sampled {
		return "-- This run confirmed nothing, and could not have: a sampled run cannot\n" +
			"-- prove the absence of an orphan. Re-run with --full to reach a confirmation.\n"
	}
	if len(r.Candidates) == 0 {
		return "-- This run produced no candidates above the score threshold, so there was\n" +
			"-- nothing to confirm.\n"
	}
	return "-- This run confirmed no relationships. Candidates were validated against the\n" +
		"-- data and none reached total containment.\n"
}

func writeRelationComment(b *strings.Builder, c model.Candidate, name identifier) {
	fmt.Fprintf(b, "-- %s → %s\n", c.Child.String(), c.Parent.String())
	if name.truncated {
		fmt.Fprintf(b, "-- Constraint name shortened to fit %d bytes; in full it would be %s.\n",
			maxIdentifierBytes, name.full)
	}
}

// identifier is a generated name plus what had to be done to make it fit.
type identifier struct {
	value     string
	full      string
	truncated bool
}

func constraintName(c model.Candidate) identifier {
	return truncateIdent("fk_" + c.Child.Table + "_" + c.Child.Column)
}

// truncateIdent fits a name into the server's identifier budget. The budget is
// counted in bytes, because that is how the server counts it, but the cut lands
// on a rune boundary: counting characters overruns the limit on accented names,
// and cutting mid-sequence produces an invalid identifier.
//
// The hash suffix is what keeps two distinct long names from collapsing into
// one and making the second ADD CONSTRAINT fail as a duplicate.
func truncateIdent(name string) identifier {
	lower := strings.ToLower(name)
	if len(lower) <= maxIdentifierBytes {
		return identifier{value: lower, full: lower}
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(lower))
	suffix := fmt.Sprintf("_%08x", h.Sum32())

	budget := maxIdentifierBytes - len(suffix)
	cut := lower[:budget]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}

	return identifier{value: cut + suffix, full: lower, truncated: true}
}

// ident and qualify route every emitted name through the same sanitizer the
// validation layer uses. Not about injection — these names come from the
// catalog — but about the name with a capital, a space or a reserved word,
// which is what breaks unquoted SQL.
func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

func identTable(schema, table string) string {
	return pgx.Identifier{schema, table}.Sanitize()
}

func qualify(r model.ColumnRef) string { return identTable(r.Schema, r.Table) }

// indexedColumns maps every column that leads an index. Leading position is
// what makes an index usable for the foreign key lookup, so a composite index
// counts only for its first column.
func indexedColumns(schemas []model.Schema) map[string]bool {
	out := make(map[string]bool)
	for _, s := range schemas {
		for _, t := range s.Tables {
			for _, idx := range t.Indexes {
				if len(idx.Columns) > 0 {
					out[strings.ToLower(s.Name+"."+t.Name+"."+idx.Columns[0])] = true
				}
			}
		}
	}
	return out
}

func indexKey(r model.ColumnRef) string { return strings.ToLower(r.String()) }

// notValidKeys collects the declared constraints that never verified the rows
// already in the table, in a stable order so the artifact is reproducible.
func notValidKeys(schemas []model.Schema) []pendingKey {
	var out []pendingKey
	for _, s := range schemas {
		for _, t := range s.Tables {
			for _, fk := range t.ForeignKeys {
				if !fk.Validated {
					out = append(out, pendingKey{table: t, fk: fk})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].table.Ref() != out[j].table.Ref() {
			return out[i].table.Ref() < out[j].table.Ref()
		}
		return out[i].fk.Name < out[j].fk.Name
	})
	return out
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

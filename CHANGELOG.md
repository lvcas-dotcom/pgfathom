# Changelog

Notable changes to pgfathom, newest first. This file is written by hand, and it
is what GitHub publishes as the release notes — a generated list of commits
tells you what was touched, not what changed for you.

Versions follow [semantic versioning](https://semver.org/spec/v2.0.0.html). While
the major version is `0`, the command-line surface and the JSON contract may
change between minor versions; both are versioned and documented when they do.

## [Unreleased]

### Added

- **`pgfathom audit` reports structural inefficiency**, not just missing
  integrity: tables with no primary key, and hot columns with no index behind
  them. Where a table has no declared key, it probes candidate column sets
  against the data and reports one as promotable only when a full scan proves
  every row distinct and non-null — never on the strength of a name.
- **Lexical similarity raises candidates the shipped profile cannot reach.**
  When no naming convention resolves a column, it is compared against table
  names by trigram similarity, the same coefficient `pg_trgm` uses. It recovers
  relationships like `idkey_operador → operadorbasecalculo`, where the name is
  reordered and abbreviated. The signal is deliberately weaker than a profile
  match, because lexical proximity carries no stated convention behind it.

  Measured against the public corpus, it is decisive exactly where nothing else
  reaches: on a Portuguese-named schema with every key removed, recovery goes
  from **1.8% to 16.6%**. On GitLab, which writes `_id` and needs no help
  reading its own names, it adds half a point. It costs 5% more candidates on
  GitLab and 18% on Discourse — each one a validation query against your
  database, which is why the number is published here.

- **The detection section no longer prints a heading with nothing under it.**
  A new dimension — the name a schema gives its primary keys — was added to the
  model and counted as evidence, but the terminal report did not know how to
  print it. A schema whose only detected convention was that one got a section
  announcing conventions and naming none.

### Fixed

- **The most ordinary foreign keys in an English schema are no longer discarded
  unseen.** A domain table — `categories`, `statuses`, `types` — carries a
  penalty so that dozens of uninteresting-but-real relationships do not bury the
  findings that justify the tool. That penalty was reaching the score threshold,
  which is a different thing entirely: `category_id → categories` scored 0.65,
  took the -0.30, landed at 0.35 under a threshold of 0.50, and was dropped
  before validation ever ran. The relationship the data would have confirmed at
  100% containment was never looked at, and the report named a count with no
  relation beside it.

  The penalty now ranks and never cuts on its own: the threshold sees the score
  without it, and the reported score still carries it, so the ordering it exists
  for is unchanged. Every other negative signal still cuts — an ambiguous target
  means the tool does not know which table is meant, which is exactly what a
  confidence threshold is for. A candidate already below the threshold on its
  other signals is still discarded, with its reason.

  It bit hardest where the table name is plural and the name match is therefore
  normalized rather than exact, which is every Rails and Django schema. Measured
  against a real server, an English schema goes from 4 confirmed to 5. The cost
  is one validation query per real domain key. The public corpus cannot measure
  this either way: it is DDL without a single row, so the penalty, which needs a
  row estimate, is never emitted there at all.

- **The benchmark report says which corpus schemas it could not measure.** The
  manifest promises that an absent local dump is stated rather than omitted; the
  statement went to the test log, which the published document does not carry.
  Regenerating `docs/benchmark/recall.md` on a machine without the optional dump
  silently deleted that schema's section, and the result read as a complete
  corpus rather than a partly measured one.

- **The integration suite no longer fails on Windows.** One assertion required
  every pipeline stage to report a strictly positive duration. Naming detection
  is in-memory work over a catalog already read — the published cost table
  prints it as `0s` even on Linux — and Go's monotonic clock on Windows is
  coarser than that, so it measured exactly zero and the suite went red on a
  clean clone. Durations are now allowed to be zero and rejected only if
  negative; a stage that drops out is caught by the exhaustive stage list, which
  is what was actually guarding against it.

### Changed

- The lexical fallback no longer extracts a table's trigrams once per column in
  the database. On a 1,000-table schema the generation stage drops from 17.4s to
  2.5s and from 16 GB of allocation to 11 MB.

### Documentation

- The README said the binary carried four dependencies. It carries six direct
  ones and links twenty-five modules at 11 MB — the interactive `setup` guide
  arrived in between, and the sentence did not. The cost is now stated with the
  reason it was accepted, which is the argument the reader opening `go.mod` is
  actually looking for.
- The README now says that the shipped profile default is `pt-br`, and what
  passing `--profile en` buys an English schema. Every run already printed the
  profile it used; nothing said which one you would get.
- The status badge tracked a hand-written "pre-release" through three releases.
  It now reads the latest release from GitHub, so it cannot go stale again.
- "Weak and rejected are reported too" was true of weak and half-true of
  rejected: discards are counted on every run, and named by `--include-rejected`.
  The sentence now says so.

## [0.1.2] — 2026-08-13

### Fixed

- **`--out` no longer writes SQL silently.** The manifest — what was written,
  where, and the reminder to read it before running it — appeared only when the
  output format was SQL. The ordinary run, a report on screen and `--out` for
  the files, left generated SQL on disk with no mention of it. It now appears in
  every format, and goes to stderr under `--format json` so the document on
  stdout stays parseable.
- Counts of one read as English: "1 table and 1 declared key", not "1 tables and
  1 declared keys".

### Changed

- The composite pass of inference no longer derives the same target's name forms
  once per table in the database, or allocates a map for every pair of tables.
  On a 5,000-table schema it is 35% faster and allocates 62% less.
- The example in the README is now a real run against a demo schema published in
  `docs/DEMO.md`, which reproduces it exactly. It used to be a mockup of output
  the tool does not produce.
- The README says plainly which measurements are the published ones. It carried a
  promise to remeasure the early figures "before any of it is published as a
  release number" — composite support has shipped and release numbers have been
  published since, from the public corpus, which is what that sentence was
  waiting for.

## [0.1.1] — 2026-08-12

### Added

- **`pgfathom setup`**, a guided first run. It reads the schemas from the server
  with their table counts, asks what to analyse, and prints the `discover`
  command your answers compose — so the second run does not need the guide. It
  never asks for a password.
- **Live progress on long runs.** `discover` against a large schema used to be
  minutes of silence, which is expensive the first time someone points this at
  production with the person who authorised it watching.
- **`.deb` and `.rpm` packages** for amd64 and arm64, with shell completions
  installed where the system keeps them.
- **The measured corpus reports two regimes.** The naming conventions a database
  states through its declared keys are evidence, and dropping every key before
  measuring destroyed exactly that evidence. Measuring both ways is what
  produced the project's headline number: **84.2% recall** on a real
  Portuguese-named schema when the declared keys it still has are readable.

### Changed

- **Colour carries meaning.** Confirmed relationships have their own colour
  instead of sharing bold with every heading, so a report is read by scanning
  it. Emphasis is now a level — none, 16 colours, 24-bit — resolved once from
  the terminal, and it still never reaches a pipe.

### Fixed

- The release now takes its version from the tag that triggered it. When a
  release candidate and the final version mark the same commit, `git describe`
  can pick either, and v0.1.0 tried to publish itself as `v0.1.0-rc.2`.

## [0.1.0] — 2026-08-12

First public release.

### Added

- **`pgfathom discover`** — finds relationships that exist in the data but were
  never declared, and validates each one against the rows before saying
  anything about it. Every candidate ends as `BROKEN`, `CONFIRMED` or
  `UNPROVEN`, and the number that decided it is printed beside it.
- **`pgfathom audit`** — reports what the schema already declares and what is
  wrong with it: foreign keys with no index behind them, type mismatches across
  a declared reference.
- **Composite keys**, end to end. Multi-column relationships are inferred,
  scored, validated by tuple anti-join, and emitted as DDL with MATCH SIMPLE
  semantics respected.
- **Reviewable SQL artifacts** — one file per category, for a human to read
  before running any of it. Nothing is ever executed against your database.
- **JSON output** with a versioned contract, for consumption by other tools.
- **A public benchmark corpus** built from real open-source schemas, with the
  method and the cost published alongside the recall.
- **Distribution**: binaries for Linux, macOS and Windows on amd64 and arm64, a
  container image on ghcr.io, and a Homebrew cask for macOS.

### Guarantees

These held from the first release and are covered by tests:

- **Read-only, absolutely.** The tool opens a read-only session and never issues
  a statement that writes.
- **Your data never leaves.** Reports carry counts, proportions and object
  names — never a value from a row.
- **Silence is never absence.** Every report states what was not analysed, so
  "nothing found" can never be confused with "nothing was looked at".

[0.1.2]: https://github.com/lvcas-dotcom/pgfathom/releases/tag/v0.1.2
[0.1.1]: https://github.com/lvcas-dotcom/pgfathom/releases/tag/v0.1.1
[0.1.0]: https://github.com/lvcas-dotcom/pgfathom/releases/tag/v0.1.0

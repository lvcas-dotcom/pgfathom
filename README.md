<p align="center">
  <img src="assets/logo.png" alt="pgfathom" width="440">
</p>

<p align="center">
  <strong>Sound the depth of a legacy PostgreSQL schema.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-pre--release-E04A3F" alt="status: pre-release">
  <img src="https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.25+">
  <img src="https://img.shields.io/badge/postgres-13%2B-336791?logo=postgresql&logoColor=white" alt="PostgreSQL 13+">
  <img src="https://img.shields.io/badge/mode-read--only-2E7D32" alt="read-only">
  <img src="https://img.shields.io/badge/license-Apache--2.0-555" alt="Apache-2.0">
</p>

<p align="center">
  <a href="#the-problem">Problem</a> ·
  <a href="#what-it-does">What it does</a> ·
  <a href="#scope">Scope</a> ·
  <a href="#safety">Safety</a> ·
  <a href="#how-it-works">How it works</a> ·
  <a href="#prior-art">Prior art</a> ·
  <a href="#roadmap">Roadmap</a>
</p>

---

`pgfathom` finds the relationships your database has but never declared — and proves them against the data instead of guessing from column names.

> [!IMPORTANT]
> **Pre-release. Under active development.**
> The design is specified in [`docs/PGFATHOM.md`](docs/PGFATHOM.md) and implementation is
> tracked in [`docs/ROADMAP.md`](docs/ROADMAP.md). `pgfathom audit` and `pgfathom discover`
> both run end to end today, verdicts and reviewable `.sql` artifacts included, and have
> been exercised against real production schemas — see
> [measurements](#first-measurements). Still missing before a release: composite keys, which
> put a quarter of a typical target schema out of reach, and the public benchmark corpus.
> Terminal output shown below is the target design, not a recording.

---

## The problem

Old PostgreSQL databases carry more structure than they declare.

```
=# \d pedido
      Column     |  Type  | Nullable
-----------------+--------+----------
 id              | bigint | not null
 cliente_id      | bigint | not null
 ...
Indexes:
    "pedido_pkey" PRIMARY KEY, btree (id)
```

No foreign keys. But `cliente_id` points at `cliente.id` in every single row — the ORM of
the day never created the constraint, or someone dropped constraints for a bulk load and
never put them back.

Two things follow. Nobody can read the model, because `\d` shows nothing and your ERD tool
draws a page of disconnected boxes. And because the constraint was never there, nothing
ever stopped orphan rows from getting in — so they probably already did, years ago,
silently, and no one has looked.

There is a nastier variant. A foreign key can be *declared* and still guarantee nothing, if
it was created `NOT VALID` and never validated. It shows up in `\d`. It draws an arrow in
your diagram. It never checked a single pre-existing row.

## What it does

```console
$ pgfathom discover --schema public
```

```
  profile pt-br · sampled validation (100k rows/table) · 312 tables

  BROKEN — the relationship is real, the integrity is not
  ──────────────────────────────────────────────────────────────────────────
  os_servico.resp_tecnico    → funcionario.id      99.7%    1,284 orphans
  pedido.cliente_id          → cliente.id          99.9%       37 orphans
  lancamento.conta_id        → conta.id            94.2%   11,903 orphans

  CONFIRMED — undeclared foreign key, no orphans found
  ──────────────────────────────────────────────────────────────────────────
  item_pedido.pedido_id      → pedido.id          100.0%        0 orphans
  endereco.municipio_id      → municipio.id       100.0%        0 orphans

  WEAK — insufficient evidence to conclude
  ──────────────────────────────────────────────────────────────────────────
  documento.entidade_id      → entidade.id         41.0%   polymorphic pair
                                                            detected

  312 tables · 298 analyzed · 14 skipped (no SELECT privilege)
  1,847 candidates · 226 validated · 3 timed out

  ! sampled run — nothing here is confirmed. orphan rows cluster on disk, which is
    exactly what page-level sampling is worst at finding. re-run with --full to prove
    absence.
```

Three verdicts, three different responses.

**Broken** is the point of the tool. It is a data bug that has been in production for years
and nobody knows. `pgfathom` writes you the query that lists the orphans, because they have
to be resolved before any constraint can be added.

**Confirmed** is a foreign key someone forgot to declare. You get the DDL, with the
`VALIDATE CONSTRAINT` split out so the initial `ALTER TABLE` doesn't hold a heavy lock —
plus the `CREATE INDEX CONCURRENTLY` on the child column when it's missing, because an
unindexed FK child is a classic delete trap.

**Weak** and **rejected** are reported too, so you never wonder why an obvious-looking
column was ignored.

Note the first row: `os_servico.resp_tecnico → funcionario.id`. No name-matching heuristic
in the world finds that one — `resp_tecnico` looks nothing like `funcionario`. `pgfathom`
finds it by reading the join predicates out of your own view and function definitions.

Output comes as a terminal report, a versioned JSON model, and reviewable `.sql` artifacts.

```console
$ pgfathom discover --full --out ./findings
```

```
  findings/confirmed.sql   2
  findings/broken.sql      3

  Review every file before running any of it.
```

The `.sql` files are written, never piped. Each one opens with the version, the timestamp
and the validation mode that produced it, and with the reminder that nothing inside was
meant to be run unread. DDL for a broken relationship stays commented out — it cannot pass
while an orphan remains, and deciding what happens to an orphan row is a call about your
domain, not one this tool is entitled to make.

## Scope

`pgfathom` analyses `public` and nothing else unless you say otherwise. That
default is conservative on purpose: this tool gets pointed at production databases
owned by someone who is nervous about it, and the run you make before reading any
documentation should be the cheapest one, not the most expensive.

```console
$ pgfathom discover --all-schemas --exclude-schema 'auditoria_*'
```

| Flag | Scope |
|---|---|
| *(none)* | `public` |
| `--schema a,b` | exactly those schemas |
| `--all-schemas` | every non-system schema the role can access |
| `--exclude-schema` | glob patterns of schemas to drop from scope |
| `--exclude` | glob patterns of **tables** to skip, matched inside the schemas in scope |

`--schema` and `--all-schemas` are mutually exclusive — there is no defensible
precedence between them, so passing both is a usage error rather than a guess.

Schema patterns and table patterns are separate flags, deliberately. If one
pattern meant both, `--exclude legacy` would quietly stop skipping a table called
`legacy` and start dropping the whole schema of that name.

**Whatever is left out gets named.** Every run reports the schemas it did not
analyse, including the run with no flags at all — a report about `public` that
never mentions the other eleven schemas is accurate and still misleading, and
whoever already knows to pass `--all-schemas` is not the person that silence was
fooling.

```
  12 schemas · 1 analyzed
    11 not analyzed: arquivo, financeiro, vendas, rh, fiscal and 6 more
       — pass --all-schemas to include them
```

## Safety

This tool is designed to be pointed at a production database owned by someone who is
nervous about it. Every guarantee below is a hard requirement in the spec, not a goal.

**Read-only, structurally.** `pgfathom` never issues a statement that modifies the database
under analysis — there is no write mode, under any flag, in any phase. The session sets
`default_transaction_read_only`, and a read-only role is documented as the recommended
setup. It emits `.sql` files for you to review and run yourself. Nothing generated is meant
to be executed unreviewed.

**Your data never leaves memory.** The tool reads values to compare keys. What comes *out*
of it are counts, ratios, and object names — never a value from your tables, in any output,
log, JSON field, or error message. This is enforced by a test that serializes every
structure and scans the result, not by code review. The target use case includes
public-sector databases holding national ID numbers, health records, and taxpayer data.

**It won't take your server down.** Every validation query runs under `statement_timeout`,
`lock_timeout`, and `idle_in_transaction_session_timeout`. Concurrency is capped and
configurable, defaulting low. The connection announces itself as `pgfathom` in
`pg_stat_activity` so your DBA can see exactly what is running. A candidate that times out
is recorded as unvalidated and the run continues.

**No claim without evidence.** Every inferred relationship carries a verdict and the metric
behind it. Sampled runs can never report a *confirmed* relationship — only `--full` can
prove absence of orphans.

**Silence is never reported as a clean bill of health.** Tables skipped for missing
privileges, candidates that timed out, schemas not covered — all of it appears in the
coverage block on every run. A clean report means "I looked and it's clean", never "I
couldn't look".

**Small dependency tree, on purpose.** Four dependencies in the binary. No cgo. The person
who has to approve running this against production will open `go.mod` first, and we intend
that to be a short read.

## How it works

```
catalog  →  usage evidence  →  candidates  →  scoring  →  stats prefilter  →  validation
```

1. **Read the catalog.** Tables, columns, keys, indexes, comments, declared foreign keys
   *and their validation state*, usage statistics with their reset timestamp.

2. **Mine usage evidence.** Join predicates extracted from view definitions, function
   bodies, and — when available — `pg_stat_statements`. A view that joins two columns is
   proof that your code treats them as related, whatever they happen to be named. This is
   pure catalog: no user data, no cost, and it finds relationships name matching
   structurally cannot.

3. **Generate candidates.** Column-name affixes are stripped and matched against
   depluralized table names using a **naming profile** — a config file, not hardcoded
   rules. Ships with `pt-br`, `en`, and `es`.

4. **Score on metadata alone.** Exact name match, type identity, target ambiguity, existing
   index, comment mentions. Weak candidates are dropped before anything touches data.

5. **Prefilter on planner statistics.** If the child column has more distinct values than
   the parent has rows, full containment is arithmetically impossible. Free, from
   `pg_stats`, no I/O.

6. **Validate against the data.** One aggregate per surviving candidate — never fetching
   rows, only counts. Containment is reported in two dimensions, by row and by distinct
   value, because a single bad value repeated a million times and a million rare bad values
   are different problems.

## Naming profiles

Most schema tools assume English. The databases that need this tool most often aren't.

Affix and plural rules live in TOML, not in Go, so teaching `pgfathom` a new convention is
a config file rather than a patch:

```toml
name = "pt-br"

column_suffixes = ["_id", "_codigo", "_cod", "_key", "_ref", "_fk"]
table_prefixes  = ["tb_", "tbl_", "sys_", "cad_", "mov_"]

[[plural]]                     # opcoes → opcao
suffix = "oes"
singular = "ao"

[[plural]]                     # animais → animal
suffix = "ais"
singular = "al"
```

Normalization returns a *set* of candidate forms rather than one, so ambiguous plurals
(`logins` → `logim`? `login`?) cost nothing in recall — every form is tried, and the one
that matched is reported so scoring can tell an exact match from an aggressive one.

**Adding a profile for your language is the easiest possible contribution.** It needs no
knowledge of the rest of the codebase — a TOML file and a table of test cases.

## Prior art

`pgfathom` is not new science, and says so.

Containment is known in the data-profiling literature as an **inclusion dependency** — the
automatically testable part of a foreign key. There is a mature body of algorithms for
discovering them (SPIDER, BINDER, MIND), implemented in
[Metanome](https://hpi.de/naumann/projects/repeatability/data-profiling/metanome-ind-algorithms.html).
Commercial GUI modelers such as Hackolade infer relationships from metadata.
[Azimutt](https://github.com/azimuttapp/azimutt) flags `_id` columns without declared
relations as part of its schema analysis.

What none of them is: a native PostgreSQL CLI that validates its inferences against the
actual data, mines evidence from the catalog itself, speaks the naming conventions of
non-English legacy schemas, reports its own coverage honestly, and hands you DDL you can
review.

`pgfathom` deliberately does *not* compete with the tools that already solved their
problems well — [Squawk](https://squawkhq.com/) for migration linting,
[Atlas](https://atlasgo.io/) for drift, [SchemaSpy](https://schemaspy.org/) and Azimutt for
diagrams, [sqlc](https://sqlc.dev/) and [jOOQ](https://www.jooq.org/) for code generation.
The versioned JSON model is the integration point: consume it and generate whatever you
like from a schema that finally knows its own relationships.

## Roadmap

| Phase | Capability | Status |
|---|---|---|
| 1 | Core model and naming profiles | Done |
| 2 | Catalog inspection · `pgfathom audit` | Done |
| 3 | Name-based candidate inference | Done |
| 3.5 | Naming-convention detection from the schema itself | Done |
| 4 | Planner-statistics prefilter | Done |
| 5 | Data validation · `pgfathom discover` | Done |
| 6 | Join mining from views and functions | Done |
| 7 | Terminal, JSON and SQL output | Done |
| 8 | Composite keys, benchmark corpus and release | Planned |

Full detail in [`docs/ROADMAP.md`](docs/ROADMAP.md).

**After v0.1:** `pgfathom check --baseline` for CI — fail the build when a new undeclared
relationship appears or an orphan count grows. Then structural findings, cross-cutting
patterns (tenant columns, polymorphic pairs), and DBML/Mermaid/PlantUML export.

Code generation is explicitly *not* on the roadmap.

## How correctness is measured

The headline metric is **recovery rate**: take a schema with complete foreign keys, drop
every one of them, run `pgfathom`, and count how many come back.

### First measurements

Against real production schemas, nothing configured: shipped profile, detection on.

| Schema | Tables | FKs | Profile alone | + detection | + join mining |
|---|---:|---:|---:|---:|---:|
| Municipal management system (pt-BR) | 784 | 985 | 0.5% | 79.0% | **79.7%** |
| Tax system (pt-BR) | 31 | 18 | 22.2% | **72.2%** | 72.2% |
| Django application (en) | 18 | 10 | 0.0% | **90.0%** | 90.0% |

The first column is why naming detection exists. A shipped profile cannot know that one
vendor writes `lote_idkey` and another writes `idkey_lote`; read off the declared keys, both
fall out in a single pass. The Django row is the same problem from the other side — its
tables carry an application prefix that belongs to no language.

**Join mining contributes far less than expected**: seven extra relationships on the only
schema that had views, and nothing on the other two. Fifty-three join predicates extracted
from 128 views and 1,676 functions is a low yield, and whether that is the schemas or the
extractor is an open question rather than a settled result. It is reported here because a
feature that measured smaller than its argument should say so.

These are private databases, so the numbers are reproducible by their owners rather than by
you. A public corpus — GitLab, Odoo, Discourse, Redmine, Mastodon — is the next step, so
anyone can check them.

### What the remaining gap is

Recall settles well below 100%, and that is expected rather than a failure. The misses are
relationships whose column names bear no resemblance to the target:

```
atotramite.tptramite_idkey        → tramitetipo          abbreviated, reordered
basecalculo.idkey_operador        → operadorbasecalculo  named for its role
ato.atorevogacao_idkey            → ato                  named for its role
```

No naming heuristic reaches these, by construction.

### Coverage is part of the metric

On the municipal schema, 25% of the tables are out of reach because their primary keys are
composite — a shape this version does not target. Every run says so, and says it as a
proportion rather than a count, because "91 tables skipped" reads as minor until you notice
it is a quarter of the database.

A recall number quoted without that fraction would be misleading by omission, which is why
composite keys and the benchmark corpus land in the same phase.

The metric that has no tolerance is the other one: **zero confirmed false positives.** A
missed relationship costs you a finding. A wrong one confirmed costs you the tool.

## Building from source

Requires Go 1.25 or newer. No cgo, no other toolchain.

```console
$ git clone https://github.com/lvcas-dotcom/pgfathom
$ cd pgfathom
$ make build          # → bin/pgfathom
$ make test           # unit suite: no Docker, no network
```

`make test` is the whole suite for everything that exists today, and it is meant
to stay that way: anything needing a database lives behind the `integration`
build tag and runs with `make test-integration`. Contributing a naming profile
for your language should never require installing a container runtime.

```console
$ make help           # list every target
$ make lint           # golangci-lint
$ make cover          # coverage report
$ make crosscheck     # prove the cross-platform build still works
```

## Contributing

Not open for code contributions yet — there is no code. The design is, though, and it is
the cheapest moment to change it. If you have run into this problem on a real legacy
database, [open an issue](../../issues): what the schema looked like, what naming
convention it used, and what a tool would have needed to find.

Once implementation starts, the two most valuable contributions will be **naming profiles
for other languages** and **real-world schemas for the benchmark corpus**.

## License

[Apache-2.0](LICENSE) — chosen for the explicit patent grant that corporate legal teams
look for before approving a tool into a pipeline.

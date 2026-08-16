# Known pitfalls

← [Guide](../README.md) › [Features & Safety](README.md)

Not edge cases to solve later. Specific ways the tool can be **wrong with confidence** —
the one way it could actually lose credibility.

## Sampling is weaker against orphans than against anything else

Orphans are almost never random. They arrive in batches — a bad load, a weekend migration,
a period when the application had a bug — and end up **physically clustered on the same
pages**. `TABLESAMPLE SYSTEM` samples by page. So the default sampling mode is biased
*against* exactly the pattern the tool exists to find: half a million clustered orphans
can survive a clean-looking sample.

**Design response:** sampled mode is **triage, not evidence**. `--full` is the answer, and
the report says so explicitly, not in a footnote. See
[Safety guarantees](safety-guarantees.md) rule 5.

## Usage counters don't survive what people assume they survive

`pg_stat_user_tables` resets on `pg_stat_reset()`, resets on `pg_upgrade`, and is **per
node** — a table read exclusively on a read replica shows zero reads on the primary.
Saying "this table hasn't been used in years" from this, on a database where someone might
act on it, is the worst thing the tool could do. Every finding in this family always
carries the reset timestamp and the replica caveat — the phrasing is always "no usage
recorded since X on this node", never "unused".

## Silent partial coverage

`pg_stats` only exposes rows for tables the current user can read. If the read-only role
lacks `SELECT` on the full schema — plausible in a public-sector org, where getting
privilege is a political process — the analysis is partial and the report ends up clean
**for the wrong reason**. That's why the `Coverage` struct is mandatory on every output.

## Cases that get skipped with a note, never silently

- **Polymorphic relationships** (`document_id` only makes sense with `document_type`) —
  validation correctly rejects on low containment, but the tool should recognize the
  pattern from neighboring columns and say so
- **Partially-matched composite FKs** — if the child side covers only part of the target
  key's positions, no candidate is generated, and the matched fraction is noted
- **Partitioned tables** — reads from the parent, never iterates partitions
- **Dangling references** — columns pointing at tables that no longer exist; correctly
  rejected, but worth reporting separately since it's a finding in itself

## Full detail

`docs/PGFATHOM.md` § [Armadilhas conhecidas](../../PGFATHOM.md#armadilhas-conhecidas)
(Portuguese) has the full list, including cases specific to multi-schema and table
inheritance.

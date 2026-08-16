# What it does

← [Guide](../README.md) › [Overview](README.md)

`pgfathom` connects to a PostgreSQL instance, reads the system catalog, cross-references
usage statistics, mines view and function definitions for real joins, and samples data in
a controlled way. From that it builds a semantic model of the database — entities, real
relationships (declared, inferred, evidenced by usage), and findings about the schema's
structural health.

The main output isn't a diagram. It's an actionable report plus ready-to-use artifacts:
suggested DDL, diagnostic queries, and a versioned JSON model other tools can consume.

## Two commands

### `pgfathom audit`

Findings that need **no inference at all** — deterministic, no false-positive risk,
because they're raw catalog facts:

- a declared constraint that's `NOT VALID` and never validated
- a declared FK with no index on the child column (classic `DELETE` trap)
- the coverage block for the analysis

### `pgfathom discover`

The full pipeline — see [02 · Architecture](../02-architecture/README.md) for the 7
stages. Produces a verdict per candidate.

## The five verdicts

| Verdict | Means | Response |
|---|---|---|
| **confirmed** | total containment, no orphans | a forgotten FK — generate the DDL |
| **broken** | real relationship, high but not total containment | **the most valuable finding** — a data bug that's been in production for years. Generates the orphan-listing query before any DDL |
| **weak** | insufficient evidence (single distinct value, mostly nulls, empty table) | doesn't conclude, and says so |
| **rejected** | low containment | name coincidence, discarded — but reported, so an obviously-named column never looks silently ignored |
| **unvalidated** | timeout, permission, unsupported case | never confused with "clean" |

`broken` is the finding that justifies the project: **integrity broken in production, and
nobody knew.**

## Full detail

Real output example, the SQL-artifact conventions, and the full README walkthrough live in
the root [`README.md`](../../../README.md#what-it-does).

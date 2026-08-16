# Algorithm

← [Guide](../README.md) › [Architecture](README.md)

```
catalog  →  usage evidence  →  candidates  →  scoring  →  stats prefilter  →  validation
```

**1. Read the catalog.** Tables, columns, types, keys, indexes, comments, declared FKs —
including `pg_constraint.convalidated`, which is what reveals a `NOT VALID` constraint.
Catalog only, no user data.

**2. Mine usage evidence.** Extracts join predicates from view definitions, function
bodies, and (when available) `pg_stat_statements` — a narrow tokenizing extractor, not a
full SQL parser (a full parser would need `pg_query_go`, which requires cgo). It's allowed
to fail: unrecognized SQL is simply ignored. A missed join is a lost signal, never a wrong
answer — see [Known pitfalls](../03-features-and-safety/known-pitfalls.md).

**3. Generate candidates.** Strips reference affixes from column names
(`_id`, `_key`, `_fk`, …) and matches the result against depluralized table names, using a
**naming profile** — see [Naming profiles](../03-features-and-safety/naming-profiles.md).
A join found in step 2 also creates a candidate directly, bypassing name matching.

**4. Score on metadata alone.** A join found in a view/function is the strongest signal
there is. Exact name match, identical type, single unambiguous target, existing index, and
comment mentions add confidence; ambiguity and generic domain names (`status`, `type`)
subtract it. Below `--min-score`, a candidate is dropped before touching data.

**5. Prefilter on planner statistics.** Reads `pg_stats` and `pg_class.reltuples` — free,
no I/O on tables. If the child column has more distinct values than the parent has rows,
full containment is arithmetically impossible.

**6. Validate against the data.** One aggregate query per surviving candidate — never
fetches rows, only counts, aggregated by distinct value before the anti-join. Containment
is reported in two dimensions (by row and by distinct value), because a single bad value
repeated a million times and a million rare bad values are different problems. Every query
runs under `statement_timeout`; a candidate that times out is marked `unvalidated` and the
run continues.

**7. Verdict.** Total containment on both dimensions → `confirmed`. High but not total →
`broken`, with the orphan counts. Below the rejection threshold → `rejected`. Insufficient
evidence → `weak`, regardless of containment. **In sampled mode, no candidate can ever be
marked `confirmed`** — only `--full` proves the absence of orphans.

## Full detail

The exact SQL for step 6, the type-compatibility rules for step 3, and the statistical
tolerance margins for step 5 are in
[`docs/PGFATHOM.md` § Algoritmo](../../PGFATHOM.md#algoritmo) (Portuguese).

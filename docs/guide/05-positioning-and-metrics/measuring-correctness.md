# Measuring correctness

← [Guide](../README.md) › [Positioning & Metrics](README.md)

The headline metric is **recovery rate**: take a schema with complete foreign keys, drop
every one, run `pgfathom`, count how many come back. `make corpus && make benchmark`
automates this against a versioned recipe (`bench/corpus.toml`); full numbers live in the
root [`README.md` § How correctness is measured](../../../README.md#how-correctness-is-measured).

## Two regimes, because they answer different questions

- **Partial** — removes half the declared FKs, measures recovery on that half. The
  ordinary case: a database that declared part of its integrity and forgot the rest.
- **Greenfield** — removes everything, measures on everything. The hardest case, and the
  one this tool exists for.

On the pt-BR municipal schema in the corpus, partial recall is **84.9%**; greenfield, on
the exact same schema, is **16.6%**. That gap is the entire argument for naming-convention
detection — see [Naming profiles](../03-features-and-safety/naming-profiles.md).

## Four things the recall table does not say

1. **No verdict is measured** — the public dumps have no rows, so nothing gets confirmed
   or broken. What's measured is whether the right candidate was *raised*.
2. **Candidates outside the ground truth aren't errors** — in a real schema, a true
   relationship nobody declared is the product, not a false positive.
3. **There is no precision figure** — recall is measured, precision is not, because that
   needs rows (so a verdict exists to be right about) and a labelled answer for the
   candidates outside the truth set. The honest claim stays the narrow one: no false
   positive has ever been confirmed against the integration fixtures. Tracked as
   [issue #36](https://github.com/lvcas-dotcom/pgfathom/issues/36).
4. **Composite keys: 1 of 53 recovered on GitLab**, and the other 52 are explained (they
   only match half the key's shape) — not silently absorbed into a lower headline number.

## Coverage is part of the metric

Early measurements were taken with a quarter of a schema out of reach (composite-key
tables, unsupported at the time). That's why the public corpus — not those early numbers —
is what the project publishes as its release figure, and why every run's coverage block is
non-negotiable regardless of which number gets quoted where.

**The metric with zero tolerance stays separate from recall: zero confirmed false
positives.** See [Safety guarantees](../03-features-and-safety/safety-guarantees.md).

## Full detail

[`README.md` § How correctness is measured](../../../README.md#how-correctness-is-measured)
has the full corpus table, per-schema breakdown, and the "what the remaining gap is"
analysis.

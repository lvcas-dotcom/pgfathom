# Safety guarantees

← [Guide](../README.md) › [Features & Safety](README.md)

> A violation of any of these is a maximum-severity bug, not a style question.

The target use case includes public-sector databases holding national ID numbers, health
records, taxpayer data. That shapes every rule below.

**1. Read-only, absolute.** No code path issues a statement that alters the database under
analysis. Enforced by `default_transaction_read_only` set in the pool's connection hook —
never per-query — so a new connection cannot exist without it, and by a test that proves a
write attempt fails in the session.

**2. User data never leaves.** What comes *out*: counts, ratios, object names — never a
row value, in any output, log, JSON field, or error message. The specific trap:
`pg_stats.most_common_vals` and `histogram_bounds` **are real user data**, even though
they come from a system view; they're read legitimately for scoring, live in memory only,
and never get serialized. Enforced by a dedicated test that scans every output structure of
every test scenario for a leaked value.

**3. No claim without evidence.** Every verdict prints the metric that produced it.

**4. Silence is never reported as a clean bill of health.** A table skipped for missing
privilege, a candidate that timed out, a schema out of scope — all of it appears in the
mandatory coverage block on every run, including the run with no flags at all.

**5. Zero confirmed false positives.** A missed relationship costs a finding. A wrong one,
confirmed, costs the tool's credibility. Between recovering more and never being wrong,
never being wrong wins — which is why **a sampled run can never mark a candidate
`confirmed`**; only `--full` proves the absence of orphans.

## Other hard operational guarantees

- **Won't take the server down** — `statement_timeout`, `lock_timeout`,
  `idle_in_transaction_session_timeout` on every validation query; concurrency capped and
  configurable, defaulting low
- **Credential precedence** avoids leaking a DSN via `ps`/shell history — `PGFATHOM_DSN` →
  libpq env vars → `--dsn` last, with the leak risk documented in `--help`
- **Conservative default scope** — `public` only, unless told otherwise, because the run
  someone makes before reading any documentation is against a production database that
  isn't theirs
- **Small dependency tree, on purpose** — see
  [Layers](../02-architecture/layers.md#the-dependency-tree-is-a-product-requirement)

## Full detail

`docs/PGFATHOM.md` § [Regras invioláveis](../../PGFATHOM.md#regras-invioláveis) and §
[Segurança operacional](../../PGFATHOM.md#segurança-operacional) (Portuguese).

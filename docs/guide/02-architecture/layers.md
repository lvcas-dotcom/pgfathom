# Layers

← [Guide](../README.md) › [Architecture](README.md)

One-way dependency. Every layer testable in isolation.

```
cmd/pgfathom        CLI entry, flag parsing, orchestration
  |
internal/db         connection, pool, security policies, timeouts
  |
internal/catalog    reads pg_catalog and information_schema
  |
internal/sqlprobe   extracts join predicates from views and functions
  |
internal/model      internal model, pure types, no I/O
  |
internal/profile    naming profiles, loaded from file
  |
internal/infer      candidate generation and scoring (metadata only)
  |
internal/stats      planner-statistics prefilter
  |
internal/validate   validation against data, sampling, anti-join
  |
internal/report     terminal, JSON, SQL rendering
```

Structural rules:

- **`internal/model` imports nothing from any other layer.** Enforced by an automated
  test, not review — it's the easiest dependency to introduce by accident.
- **`internal/infer`, `internal/profile`, `internal/sqlprobe` are deterministic**, no
  database access. Testable without Docker or network.
- **`internal/validate` is the only layer that reads user table data.** Everything else in
  the pipeline operates on catalog, planner statistics, or in-memory model only.

## Tech stack

| | |
|---|---|
| Language | Go, floor 1.25 |
| Driver | `github.com/jackc/pgx/v5` + `pgxpool` — no ORM |
| CLI | `github.com/spf13/cobra` |
| Config | `github.com/pelletier/go-toml/v2` |
| Concurrency | `golang.org/x/sync/errgroup` with `SetLimit` |
| Log | `log/slog` (stdlib) |
| Terminal table | `text/tabwriter` (stdlib) |
| Interactive guide | `bubbletea` + `lipgloss`, only in `pgfathom setup` |
| Testing | `testing` + `github.com/google/go-cmp` |
| Integration | `testcontainers-go`, behind `//go:build integration` |
| Release | `goreleaser` |
| Lint | `golangci-lint` |

**No cgo.** Cross-compiling has to stay trivial — that's why the SQL-mining layer uses a
narrow extractor of its own instead of `pg_query_go`.

## The dependency tree is a product requirement

> The DBA who has to approve running this against production opens `go.mod` first.

Every new dependency needs a measured cost (modules + binary size) in the PR description —
see [06 · Contributing](../06-contributing/README.md). The full ledger of what's been
accepted and refused, with the numbers, lives in
[`openspec/project.md`](../../../openspec/project.md#stack) (Portuguese).

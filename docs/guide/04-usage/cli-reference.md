# CLI reference

← [Guide](../README.md) › [Usage](README.md)

## Install

```console
# Debian/Ubuntu/Fedora/RHEL
$ sudo dpkg -i pgfathom_*_linux_amd64.deb     # or: sudo rpm -i pgfathom_*_linux_amd64.rpm

# Go
$ go install github.com/lvcas-dotcom/pgfathom/cmd/pgfathom@latest

# Homebrew (macOS)
$ brew install lvcas-dotcom/tap/pgfathom

# Docker
$ docker run --rm ghcr.io/lvcas-dotcom/pgfathom:latest discover --dsn "$DSN"
```

Full install matrix, including checksum verification, in the root
[`README.md`](../../../README.md#installing).

## First run: `pgfathom setup`

Twenty-one flags is a lot to meet at once. The interactive guide asks for the connection,
lists the schemas the server actually has (with table counts — the most common way a
first run ends in "nothing found" is pointing at a schema with six tables while the other
sixty live elsewhere), asks how thoroughly to validate, then **prints the `discover`
command your answers composed** before running it. Never asks for a password —
`PGFATHOM_DSN` or `~/.pgpass`.

## Scope — `public` by default, nothing beyond

```console
$ pgfathom discover --all-schemas --exclude-schema 'audit_*'
```

| Flag | Scope |
|---|---|
| *(none)* | `public` |
| `--schema a,b` | exactly those schemas |
| `--all-schemas` | every non-system schema the role can access |
| `--exclude-schema` | glob patterns of schema to drop |
| `--exclude` | glob patterns of **table** to skip |

`--schema` and `--all-schemas` are mutually exclusive — passing both is a usage error, not
a guessed precedence. Whatever is left out of scope is always named in the coverage block,
even on the flagless run.

## Flags

```
pgfathom discover [flags]

  --dsn string           connection string (prefer PGFATHOM_DSN)
  --schema strings       schemas to analyse (default: public)
  --all-schemas          every accessible non-system schema; excludes --schema
  --exclude-schema       schema glob patterns to drop from scope
  --exclude strings      table glob patterns to skip
  --profile string       naming profile (default: pt-br)
  --full                 validate against every row, no sampling
  --sample int           target rows per sample (default: 100000)
  --min-score float      metadata threshold to validate (default: 0.5)
  --no-sql-probe         disable view/function join mining
  --timeout duration     statement_timeout per validation query (default: 30s)
  --concurrency int      concurrent validations (default: 4)
  --format string        table | json | sql (default: table)
  --out string           output directory for artifacts
  --include-rejected     also show discarded candidates

pgfathom audit [flags]
  # same connection/scope flags
  # findings that need no inference
```

## The three outputs

**Terminal** — grouped by verdict, `broken` first. **JSON** — the full model, treated as a
public API (`schema_version` field). **SQL** — one file per category, written (never
piped), each opening with a mandatory review header; DDL for a `broken` relationship stays
commented out until orphans are resolved.

Full detail on each format: root [`README.md`](../../../README.md#how-it-works).

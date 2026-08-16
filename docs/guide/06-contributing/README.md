# 06 · Contributing

← [Guide](../README.md)

The process itself lives in [`CONTRIBUTING.md`](../../../CONTRIBUTING.md) — this page is
the shortest path into it, not a replacement.

## The shortest path to a first PR

1. **Read [`CONTRIBUTING.md`](../../../CONTRIBUTING.md)** — in particular the
   [language convention](../../../CONTRIBUTING.md#language) (English in the tree, Portuguese
   in commit messages and the design record) and the rule that decides what needs an issue
   first: docs, bug fixes, refactors and tests go straight to a PR; new or changed behavior
   needs an issue to agree on shape before it gets an `openspec/changes/` proposal.
2. **Pick something.** A [naming profile](../03-features-and-safety/naming-profiles.md)
   for a language `pgfathom` doesn't speak yet is the easiest entry point in the project —
   it needs no knowledge of the rest of the codebase, just a TOML file and a table of test
   cases. Otherwise, check the
   [open issues](https://github.com/lvcas-dotcom/pgfathom/issues) on GitHub.
3. **If it's new behavior**, see how a real proposal looks in
   [`openspec/changes/archive/`](../../../openspec/changes/archive/) before writing your
   own — `openspec validate <name> --strict` has to pass before it merges.

## The five rules every PR is evaluated against, no exceptions

See [Safety guarantees](../03-features-and-safety/safety-guarantees.md) for the full
detail behind each one:

1. Read-only, absolute
2. User data never leaves
3. No claim without evidence
4. Silence is never reported as a clean bill of health
5. Zero confirmed false positives

## Before opening a PR

```sh
make fmt && make lint && make test
```

`make test` is what CI gates every PR on; `make test-integration` and `make benchmark`
need Docker (and, for the latter, `psql`) and aren't required for a docs-only or
metadata-only change.

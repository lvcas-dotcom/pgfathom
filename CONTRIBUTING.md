# Contributing

Thanks for looking. This document is the short version of how the project
works; the reasoning behind the rules lives in `openspec/project.md`.

## What you need

- **Go 1.25** or later
- **Docker**, for the integration suite — it starts real PostgreSQL containers
- **`psql`**, only if you run the benchmark corpus

```sh
git clone https://github.com/lvcas-dotcom/pgfathom
cd pgfathom
make build          # ./bin/pgfathom
make test           # unit suite, no database needed
make help           # every target, with a line on what it does
```

## Running the tests

The suite is split by build tag, because the parts have very different costs.

| Command | Tag | What it needs |
| --- | --- | --- |
| `make test` | none | nothing; this is what CI gates every PR on |
| `make test-integration` | `integration` | Docker; starts PostgreSQL containers |
| `make benchmark` | `benchmark` | Docker and `psql`; downloads real schemas |
| `make lint` | all three | `golangci-lint` |

`make lint` runs the linter under each tag in turn. Code behind a tag is
invisible to the default build, so a change that compiles and passes `make test`
can still be broken — this is how you find out before CI does.

Before opening a pull request:

```sh
make fmt && make lint && make test
```

## The four rules that are not negotiable

These are what the tool is for. A change that weakens one will not be merged,
however good the rest of it is:

1. **Read-only, absolutely.** No path may write to the target database.
2. **User data never leaves.** Reports carry counts, proportions and object
   names. Never a value from a row — not in output, not in a log, not in an
   error message.
3. **No assertion without evidence.** Every verdict prints the number that
   produced it.
4. **Silence is never absence.** Anything not analysed is stated. "Nothing
   found" must never be readable as "nothing was looked at".

## How changes are proposed

Behaviour is specified before it is built, under `openspec/`. It is lighter than
it sounds, and it exists because this tool makes claims about someone's
production database — what a verdict means has to be written down somewhere that
is not the implementation.

- **Bug fix, refactor, docs, tests** — just open a pull request.
- **New behaviour, or changed behaviour** — open an issue first so we can agree
  on the shape. Then the change gets a directory under `openspec/changes/`, with
  a proposal, the spec deltas, and tasks. `openspec validate <name> --strict`
  has to pass.

Look at `openspec/changes/archive/` for what a real one looks like.

## Dependencies

**The dependency tree is a product requirement, not an implementation detail.**
The person who has to approve running this against production will open `go.mod`
first, and a short list is part of the argument.

A pull request that adds a dependency needs to say, in the description, what it
buys and what it costs — measured, in modules and in binary size, not estimated.
Two have been accepted so far, and `openspec/project.md` records the numbers for
both.

## Style

- **Match the surrounding code.** Naming, comment density, error handling.
- **Comments say why, not what.** If a comment restates the line below it,
  delete it. The ones worth writing explain a decision that is not obvious from
  the code — including decisions that were wrong the first time.
- **Tests state what they protect.** A test named `TestFoo2` tells the next
  person nothing when it fails.

### Language

**Everything in the repository tree is English** — code, comments, `--help`,
build files, CI workflows, and the documents you are reading. If a contributor
can open it, it is in English.

Two exceptions, both deliberate: **commit messages** and the design record under
`openspec/` are written in Portuguese, the maintainer's language. They are the
project's history and its reasoning, not its interface.

Commits follow [Conventional Commits](https://www.conventionalcommits.org/), and
the body should say why, not restate the diff.

## Reporting bugs

Use the issue templates — they ask for the PostgreSQL version, the shape of the
schema and the coverage block from the report, which are the three things
every diagnosis starts from.

For a security issue, do not open an issue. See [SECURITY.md](SECURITY.md).

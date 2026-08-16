# Naming profiles

← [Guide](../README.md) › [Features & Safety](README.md)

> Most schema tools assume English. The databases that need this tool most, usually
> aren't.

Affix and plural rules live in **TOML**, never in Go code. The binary embeds the official
profiles via `embed`; `--profile` accepts a path to a user file.

```toml
name = "pt-br"
column_suffixes = ["_id", "_codigo", "_cod", "_key", "_ref", "_fk"]
table_prefixes  = ["tb_", "tbl_", "sys_", "cad_", "mov_"]

[[plural]]                     # opcoes → opcao
suffix = "oes"
singular = "ao"
```

## Why normalization returns a set, not a string

The obvious path — apply rules in order, return the first match — breaks on real
ambiguity. `logins` produces `logim` under the `ns → m` rule (correct for `armazens`), and
`login` under the generic drop-`s` rule. Nothing in the name resolves which is right.

So table-name normalization returns an **ordered set of candidate forms**, always
including the unmodified original. A match succeeds if *any* form matches. The matched
form is reported alongside the candidate, which is what lets scoring tell an exact match
from an aggressive one apart.

Accepted cost: candidate noise from an over-aggressive form. That's fine — scoring
penalizes it, and validation against real data rejects what turns out to be coincidence.
**The project accepts candidate noise. It does not accept a confirmed false positive.**

## Convention detection, before you pick a profile

Before requiring a profile choice, `pgfathom` tries to **derive the convention from the
schema itself** — table-prefix by frequency, reference affix from the FKs the schema
already declares. On a real municipal database, this took recall from 0.5% to 79.0%; on
another, from 22.2% to 72.2%. Detection is always reported, never silently swaps the base
profile — it only adds candidate forms on top of it.

## The easiest contribution in the project

A new naming profile needs no knowledge of the rest of the codebase — a TOML file plus a
table of test cases. See [06 · Contributing](../06-contributing/README.md).

## Full detail

The complete pt-br rule table and the full case for the set-based design are in
[`docs/PGFATHOM.md` § Perfis de nomenclatura](../../PGFATHOM.md#perfis-de-nomenclatura)
(Portuguese).

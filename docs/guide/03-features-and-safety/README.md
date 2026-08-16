# 03 · Features & Safety

← [Guide](../README.md)

What makes it safe to point at a production database, and the two most delicate gears in
the product: recognizing naming convention in any language, and knowing where its own
confidence runs out.

| Page | Covers |
|---|---|
| [Naming profiles](naming-profiles.md) | How pt-br/en/es work, why depluralization returns a *set* of forms, why a new profile is the easiest contribution in the project |
| [Safety guarantees](safety-guarantees.md) | The 5 non-negotiable rules — and, more importantly, how each is enforced by a test, not a promise |
| [Known pitfalls](known-pitfalls.md) | Specific ways the tool can be wrong with confidence: sampling against clustered orphans, usage counters that reset silently, silent partial coverage |

## Why these three live together

All three answer "what can go wrong, and what the project does about it" — one about
input (naming the parser doesn't recognize), one about policy (a rule that never bends),
one about interpretation (right number, wrong conclusion). Anyone reviewing a PR in
[02 · Architecture](../02-architecture/README.md) should know all three before approving.

## See also

[06 · Contributing](../06-contributing/README.md) — these are the same rules a PR gets
evaluated against.

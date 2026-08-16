# Contributor guide

This is the map. `pgfathom` already has a single source of truth for behavior —
[`docs/PGFATHOM.md`](../PGFATHOM.md) (the design record) and
[`openspec/specs/`](../../openspec/specs/) (the formal specs, one per layer). Both are
authoritative, and both are written in Portuguese — the maintainer's language for design
reasoning, per [`CONTRIBUTING.md`](../../CONTRIBUTING.md#language).

This guide doesn't replace either. It's the orientation layer for someone who has never
read either one: what to read first, in what order, and why each piece exists — in
English, since that's the project's convention for anything meant to reach outside
contributors. Every page here links back to the canonical source for the full detail; if
the two ever disagree, the canonical source wins.

## Sections

| # | Section | What it covers |
|---|---|---|
| 01 | [Overview](01-overview/README.md) | Why the tool exists, what it does end to end |
| 02 | [Architecture](02-architecture/README.md) | The layers, the core Go types, the 7-stage pipeline |
| 03 | [Features & Safety](03-features-and-safety/README.md) | Naming profiles, the non-negotiable rules, known failure modes |
| 04 | [Usage](04-usage/README.md) | Install, first run, flag reference |
| 05 | [Positioning & Metrics](05-positioning-and-metrics/README.md) | Where this sits versus other tools, and what the recall numbers actually mean |
| 06 | [Contributing](06-contributing/README.md) | Where to start, and how a change gets proposed |

New to the project? Start at [01 · Overview](01-overview/README.md). Ready to send a
patch? Skip to [06 · Contributing](06-contributing/README.md).

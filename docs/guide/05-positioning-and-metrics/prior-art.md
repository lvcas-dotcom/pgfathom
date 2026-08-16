# Prior art

← [Guide](../README.md) › [Positioning & Metrics](README.md)

> `pgfathom` is not new science, and says so.

The full text lives in the root [`README.md` § Prior art](../../../README.md#prior-art) —
this page is the map of what it covers, not a copy of it.

## The shape of the argument

1. **Read the ORM first, if you can.** Rails' `schema.rb`, Django's `models.py`, a JPA
   entity — when application source is available, it states the intended relationship in
   one place, no inference needed. `pgfathom` doesn't compete with that.
2. **What source doesn't tell you** is whether that intent ever became an enforced
   constraint, and whether the data still obeys it. `pgfathom` starts exactly where a
   model file's authority ends.
3. **Academic data-profiling literature is mature** — containment is formally an
   *inclusion dependency*, with published algorithms (SPIDER, BINDER, MIND) implemented in
   the Metanome framework. `pgfathom`'s niche is engineering and product, not algorithmic
   invention: Metanome doesn't speak PostgreSQL, doesn't read `pg_catalog`, doesn't
   generate DDL.
4. **Partial overlap exists in open source** (Azimutt flags undeclared `_id` columns) and
   **in commercial GUI tools** (Hackolade, Oracle Data Modeler) — both metadata-only,
   never validated against real data.
5. **The actual boundary:** not inferring — **validating the inference against real data
   and reporting the uncertainty honestly**. A view that joins two columns is proof no
   naming heuristic could ever produce.
6. **Deliberately doesn't compete** with Squawk (migration linting), Atlas (drift), sqlc/
   jOOQ/Ent/Prisma (code generation from schema) — the versioned JSON model is the
   integration point for those instead.

## Full text

[`README.md` § Prior art](../../../README.md#prior-art).

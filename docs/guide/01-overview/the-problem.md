# The problem

← [Guide](../README.md) › [Overview](README.md)

Old, large PostgreSQL databases carry more structure than they declare. Years of
maintenance, different teams, and botched migrations strip away whatever gave the schema
its meaning.

## The common symptom: a missing foreign key

```
=# \d order
      Column     |  Type  | Nullable
-----------------+--------+----------
 id              | bigint | not null
 customer_id     | bigint | not null
```

`customer_id` points at `customer.id` in **every row** — but no constraint says so.
Typical reasons, legitimate and not: someone disabled constraints for a bulk load and
never turned them back on, the ORM of the day never created it, a migration from an even
older database brought only the tables, or nobody thought it mattered.

## The cost, which shows up later

- `\d table` shows no relationship at all — nobody new to the project can read the model
- an ERD tool draws a page of disconnected boxes
- a code generator produces structs with no association
- worst: without the constraint, the database never stopped orphan rows from getting in —
  so they probably already did, silently, years ago

## The sneakier variant: `NOT VALID`

A constraint can be **declared** and still guarantee nothing, if it was created `NOT
VALID` and never validated. Postgres starts blocking *new* violations, but never checked
the rows that were already there. `\d` shows the FK, the ERD tool draws the arrow,
everyone assumes integrity — and the old orphans stay invisible.

That's exactly the second finding `pgfathom audit` looks for — see
[What it does](what-it-does.md).

## Full detail

The design record goes further into this — the "layer of lost knowledge" beyond foreign
keys (inconsistent soft-delete, missing tenant columns, `varchar` acting as an
undeclared enum) that's out of scope for the v0.1 but shaped the roadmap. See
[`docs/PGFATHOM.md` § O problema](../../PGFATHOM.md#o-problema) (Portuguese).

# The demo schema

The output in the README is a real run against the schema below. It is here so
you can reproduce it rather than take our word for it, and because it is the
shortest thing that exercises every verdict.

Start a throwaway PostgreSQL:

```console
$ docker run -d --name pgfathom-demo -e POSTGRES_PASSWORD=demo \
    -e POSTGRES_DB=loja -p 55432:5432 postgres:16-alpine
```

The schema is [`demo/schema.sql`](../demo/schema.sql), kept as one copy so this page and
the compose file cannot drift apart. Load it:

```console
$ docker exec -i pgfathom-demo psql -U postgres -d loja < demo/schema.sql
```

Or skip both steps — `docker compose -f demo/compose.yaml up` does all of it and runs the
report.

Run it:

```console
$ export PGFATHOM_DSN='postgres://postgres:demo@localhost:55432/loja?sslmode=disable'
$ pgfathom discover --schema public --full
```

## What each part of the schema is for

**`pedido.cliente_id → cliente.id`** is the ordinary case: the name says what it
references, every row points at something real, and nobody declared it. It comes
back **CONFIRMED**.

**`item_pedido(pedido_id, seq)`** is keyed on a pair, and the first position
references `pedido`. This is the identifying relationship a composite key exists
for, and it is what the single-column pass structurally cannot reach.

**`os_servico.resp_tecnico → funcionario.id`** is the interesting one. No name
matching finds it — `resp_tecnico` looks nothing like `funcionario`. It is found
by reading the join predicate out of `os_por_tecnico`, a view already in the
database. And because 200 of the 8,000 rows name a technician who does not
exist, it comes back **BROKEN** rather than confirmed.

**`nota_fiscal.pedido_id`** already has a declared constraint, so inference
leaves it alone. It is in the schema to prove that: a tool that re-proposed keys
you already have would be noise.

Clean up when you are done:

```console
$ docker rm -f pgfathom-demo
```

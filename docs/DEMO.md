# The demo schema

The output in the README is a real run against the schema below. It is here so
you can reproduce it rather than take our word for it, and because it is the
shortest thing that exercises every verdict.

Start a throwaway PostgreSQL:

```console
$ docker run -d --name pgfathom-demo -e POSTGRES_PASSWORD=demo \
    -e POSTGRES_DB=loja -p 55432:5432 postgres:16-alpine
```

Load it:

```sql
CREATE TABLE cliente (
  id bigserial PRIMARY KEY,
  nome text NOT NULL
);

CREATE TABLE funcionario (
  id bigserial PRIMARY KEY,
  nome text NOT NULL
);

CREATE TABLE pedido (
  id bigserial PRIMARY KEY,
  cliente_id bigint NOT NULL,
  total numeric(12,2) NOT NULL DEFAULT 0
);

CREATE TABLE item_pedido (
  pedido_id bigint NOT NULL,
  seq integer NOT NULL,
  descricao text NOT NULL,
  PRIMARY KEY (pedido_id, seq)
);

CREATE TABLE os_servico (
  id bigserial PRIMARY KEY,
  resp_tecnico bigint,
  aberta_em date NOT NULL DEFAULT current_date
);

INSERT INTO cliente (nome) SELECT 'cliente ' || g FROM generate_series(1, 4000) g;
INSERT INTO funcionario (nome) SELECT 'func ' || g FROM generate_series(1, 300) g;

-- Every order points at a customer that exists: this relationship is sound, and
-- nobody ever declared it.
INSERT INTO pedido (cliente_id, total)
SELECT 1 + (g % 4000), (g % 500)::numeric
FROM generate_series(1, 20000) g;

INSERT INTO item_pedido (pedido_id, seq, descricao)
SELECT p.id, s, 'item ' || s
FROM pedido p, generate_series(1, 3) s;

-- One in forty service orders names a technician who is not in the table. There
-- was never a constraint to stop it.
INSERT INTO os_servico (resp_tecnico)
SELECT CASE WHEN g % 40 = 0 THEN 900000 + g ELSE 1 + (g % 300) END
FROM generate_series(1, 8000) g;

-- The report that ships with the system joins the two tables, which is how the
-- relationship survived in the database even though the constraint did not.
CREATE VIEW os_por_tecnico AS
  SELECT f.nome, count(*) AS abertas
  FROM os_servico o
  JOIN funcionario f ON f.id = o.resp_tecnico
  GROUP BY f.nome;

-- A table added later, by people who did declare their keys.
CREATE TABLE nota_fiscal (
  id bigserial PRIMARY KEY,
  pedido_id bigint NOT NULL REFERENCES pedido(id),
  emitida_em date NOT NULL DEFAULT current_date
);
INSERT INTO nota_fiscal (pedido_id) SELECT id FROM pedido WHERE id % 3 = 0;

ANALYZE;
```

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

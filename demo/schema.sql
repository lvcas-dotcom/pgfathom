-- The demo schema. It is the single copy: docs/DEMO.md points here, and
-- demo/compose.yaml loads it, so the two cannot drift apart.
--
-- Every table here exists to exercise one verdict. See docs/DEMO.md for what
-- each one is for.

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

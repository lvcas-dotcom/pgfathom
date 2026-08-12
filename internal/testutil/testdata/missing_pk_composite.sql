-- Cenário: tabela sem PK e sem nenhum UNIQUE declarado, mas com um índice não
-- único sobre duas colunas NOT NULL cuja combinação é, na prática, única. É o
-- caso que só uma sondagem por contagem contra os dados resolve — o catálogo
-- não tem como provar isto sozinho.

CREATE TABLE item_pedido (
    pedido_id bigint NOT NULL,
    sequencia int    NOT NULL,
    produto   text   NOT NULL,
    quantidade numeric(10,2) NOT NULL
);

-- Não-único de propósito: o schema já agrupou estas colunas, mas nunca
-- declarou a constraint. A sondagem é o que confirma.
CREATE INDEX ix_item_pedido_pedido_sequencia ON item_pedido (pedido_id, sequencia);

INSERT INTO item_pedido (pedido_id, sequencia, produto, quantidade) VALUES
    (1, 1, 'Cimento CP-II 50kg', 10),
    (1, 2, 'Areia media m3',     3),
    (2, 1, 'Cimento CP-II 50kg', 25),
    (2, 2, 'Brita 1 m3',         5),
    (3, 1, 'Tijolo ceramico',    500);

-- Segundo cenário no mesmo arquivo: colunas com a mesma forma, mas com uma
-- duplicata plantada. A sondagem tem que dizer "não é chave", nunca confirmar
-- por engano.
CREATE TABLE pagamento_parcela (
    contrato_id bigint NOT NULL,
    parcela     int    NOT NULL,
    valor       numeric(10,2) NOT NULL
);

CREATE INDEX ix_pagamento_parcela_contrato_parcela ON pagamento_parcela (contrato_id, parcela);

INSERT INTO pagamento_parcela (contrato_id, parcela, valor) VALUES
    (1, 1, 500.00),
    (1, 2, 500.00),
    (1, 2, 500.00),  -- duplicata plantada: (contrato_id, parcela) = (1, 2) duas vezes
    (2, 1, 300.00);

ANALYZE;

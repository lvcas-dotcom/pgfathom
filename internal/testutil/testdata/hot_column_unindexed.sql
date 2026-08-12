-- Cenário: coluna recorrente em predicado de junção e de filtro no código
-- real, sem índice liderando ela. Duas fontes distintas — uma view e uma
-- função — tocam a mesma coluna, o suficiente para passar do limiar de
-- recorrência padrão.

CREATE TABLE centro_custo (
    id   bigint PRIMARY KEY,
    nome text NOT NULL
);

CREATE TABLE movimentacao (
    id              bigint PRIMARY KEY,
    centro_custo_id bigint NOT NULL,  -- sem índice de propósito: é o achado
    valor           numeric(12,2) NOT NULL
);

CREATE VIEW vw_movimentacao_por_centro AS
SELECT m.id, c.nome AS centro, m.valor
FROM movimentacao m
JOIN centro_custo c ON m.centro_custo_id = c.id;

CREATE FUNCTION fn_total_centro(p_centro bigint)
RETURNS numeric
LANGUAGE sql
AS $$
    SELECT sum(m.valor) FROM movimentacao m WHERE m.centro_custo_id = p_centro;
$$;

INSERT INTO centro_custo (id, nome) VALUES (1, 'Secretaria de Obras'), (2, 'Secretaria de Saude');
INSERT INTO movimentacao (id, centro_custo_id, valor) VALUES
    (1, 1, 1000.00), (2, 1, 2500.00), (3, 2, 300.00);

ANALYZE;

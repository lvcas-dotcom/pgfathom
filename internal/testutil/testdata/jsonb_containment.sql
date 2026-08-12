-- Cenário: coluna jsonb sondada por contenção (@>) em duas fontes distintas,
-- sem índice GIN. GIN tem classe de operador padrão para jsonb, então a
-- recomendação não depende de extensão nenhuma.

CREATE TABLE processo (
    id    bigint PRIMARY KEY,
    dados jsonb NOT NULL
);

CREATE VIEW vw_processo_urgente AS
SELECT p.id
FROM processo p
WHERE p.dados @> '{"prioridade": "urgente"}'::jsonb;

CREATE FUNCTION fn_processos_com_tag(p_tag jsonb)
RETURNS SETOF bigint
LANGUAGE sql
AS $$
    SELECT p.id FROM processo p WHERE p.dados @> p_tag;
$$;

INSERT INTO processo (id, dados) VALUES
    (1, '{"prioridade": "urgente", "setor": "Secretaria de Obras"}'),
    (2, '{"prioridade": "normal", "setor": "Secretaria de Saude"}');

ANALYZE;

-- Cenário: coluna vector sondada por operador de distância em duas fontes
-- distintas, sem índice de vizinhança. Só roda contra uma imagem que já tem
-- pgvector instalada — a suíte que carrega esta fixture escolhe essa imagem
-- e pula o teste quando não consegue subir o contêiner.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE documento (
    id        bigint PRIMARY KEY,
    titulo    text NOT NULL,
    embedding vector(3) NOT NULL
);

CREATE VIEW vw_documento_similar AS
SELECT d.id, d.titulo
FROM documento d
ORDER BY d.embedding <-> '[0,0,0]'
LIMIT 5;

CREATE FUNCTION fn_documentos_proximos(p_ref vector(3))
RETURNS TABLE (id bigint)
LANGUAGE sql
AS $$
    SELECT d.id FROM documento d ORDER BY d.embedding <-> p_ref LIMIT 5;
$$;

INSERT INTO documento (id, titulo, embedding) VALUES
    (1, 'Memorial descritivo', '[0.1, 0.2, 0.3]'),
    (2, 'Laudo tecnico',       '[0.9, 0.8, 0.7]');

ANALYZE;

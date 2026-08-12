-- Cenario: tabela-ponte sem PK e sem nenhum indice ou unique sobre suas duas
-- FKs de coluna unica. candidateKeys (a heuristica automatica de audit) so
-- olha para colunas de um indice nao-unico ja existente, entao nunca chega a
-- tentar este par -- e' exatamente a lacuna que a resolucao interativa fecha,
-- oferecendo a combinacao das FKs como candidato.
--
-- pedido, produto e situacao dao ao schema tres tabelas com PK de coluna
-- unica chamada "idkey" -- o minimo que profile.Detect exige para tabular uma
-- convencao de nome de PK (minPKNameCount = 3), o que deixa a opcao de coluna
-- sintetica da resolucao interativa testavel.

CREATE TABLE pedido (
    idkey bigint PRIMARY KEY,
    numero text NOT NULL
);

CREATE TABLE produto (
    idkey bigint PRIMARY KEY,
    nome text NOT NULL
);

CREATE TABLE situacao (
    idkey bigint PRIMARY KEY,
    descricao text NOT NULL
);

INSERT INTO situacao (idkey, descricao) VALUES (1, 'Aberta'), (2, 'Fechada');

CREATE TABLE pedido_produto (
    pedido_id  bigint NOT NULL REFERENCES pedido (idkey),
    produto_id bigint NOT NULL REFERENCES produto (idkey),
    quantidade numeric(10,2) NOT NULL
);

INSERT INTO pedido (idkey, numero) VALUES (1, 'PED-1'), (2, 'PED-2'), (3, 'PED-3');
INSERT INTO produto (idkey, nome) VALUES (10, 'Cimento'), (20, 'Areia'), (30, 'Brita');

-- (pedido_id, produto_id) nunca se repete: e' uma chave composta real, so'
-- que sem indice ou constraint nenhuma provando isso. quantidade repete de
-- proposito (10.00 e 25.00 aparecem duas vezes cada), assim como pedido_id e
-- produto_id isoladamente: nenhuma coluna sozinha pode parecer, por
-- coincidencia dos dados plantados, uma chave candidata valida.
INSERT INTO pedido_produto (pedido_id, produto_id, quantidade) VALUES
    (1, 10, 10.00),
    (1, 20, 10.00),
    (2, 10, 25.00),
    (2, 30, 25.00),
    (3, 10, 500.00);

ANALYZE;

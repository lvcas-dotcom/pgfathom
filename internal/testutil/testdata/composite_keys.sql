-- Cenário das chaves compostas. Cada bloco existe para um comportamento
-- nomeado, e nenhum deles é inferível pelo caminho de coluna única.
--
-- Órfãos usam números 900+ para nunca colidirem com chave válida. Os valores de
-- texto plantados alimentam a varredura de vazamento e estão registrados em
-- testutil.PlantedValues.

-- Alvo de chave composta. É a tabela que quase todo o resto aponta.
CREATE TABLE nota (
    empresa_id bigint NOT NULL,
    numero     bigint NOT NULL,
    emitente   text   NOT NULL,
    PRIMARY KEY (empresa_id, numero)
);

-- Espelho, e relacionamento identificador: a filha é chaveada pela chave da
-- mãe mais um discriminador. É a forma que a regra de elegibilidade antiga
-- descartava inteira.
CREATE TABLE item (
    empresa_id bigint NOT NULL,
    numero     bigint NOT NULL,
    sequencia  bigint NOT NULL,
    descricao  text   NOT NULL,
    PRIMARY KEY (empresa_id, numero, sequencia)
);

-- Prefixada, com órfãos de tupla plantados e linhas de nulidade parcial. As
-- parciais escapam da constraint por MATCH SIMPLE e não podem ser contadas
-- como órfãs.
CREATE TABLE rateio (
    id              bigint PRIMARY KEY,
    nota_empresa_id bigint,
    nota_numero     bigint,
    centro_custo    text NOT NULL
);

-- Parcial: casa duas das três posições de contrato, e não pode virar
-- candidato. Vira observação.
CREATE TABLE contrato (
    empresa_id bigint NOT NULL,
    ano        bigint NOT NULL,
    numero     bigint NOT NULL,
    objeto     text   NOT NULL,
    PRIMARY KEY (empresa_id, ano, numero)
);

CREATE TABLE aditivo (
    id         bigint PRIMARY KEY,
    empresa_id bigint NOT NULL,
    ano        bigint NOT NULL,
    resumo     text   NOT NULL
);

-- Armadilha: duas tabelas dividem a mesma assinatura de chave, e nada em
-- movimentacao nomeia qualquer uma delas. As duas alcançariam contenção total,
-- e no máximo uma é real — nenhum candidato pode sair daqui.
CREATE TABLE lotacao (
    unidade_id bigint NOT NULL,
    setor_id   bigint NOT NULL,
    vigencia   text   NOT NULL,
    PRIMARY KEY (unidade_id, setor_id)
);

CREATE TABLE alocacao (
    unidade_id bigint NOT NULL,
    setor_id   bigint NOT NULL,
    turno      text   NOT NULL,
    PRIMARY KEY (unidade_id, setor_id)
);

CREATE TABLE posto (
    unidade_id bigint NOT NULL,
    setor_id   bigint NOT NULL,
    endereco   text   NOT NULL,
    PRIMARY KEY (unidade_id, setor_id)
);

CREATE TABLE movimentacao (
    id         bigint PRIMARY KEY,
    unidade_id bigint NOT NULL,
    setor_id   bigint NOT NULL,
    motivo     text   NOT NULL
);

-- Âncora mais discriminador: `nota_numero` nomeia o alvo, `empresa_id` é a
-- coluna que atravessa o schema e sozinha não aponta para nada. É a forma
-- canônica de chave composta em base particionada ou multi-tenant, e as 53 do
-- GitLab são todas assim. Esta fixture chegou a existir como armadilha, sob a
-- regra que exigia derivação uniforme; o corpus mostrou que a regra estava
-- errada, e o cenário mudou de lado.
CREATE TABLE frete (
    id          bigint PRIMARY KEY,
    empresa_id  bigint NOT NULL,
    nota_numero bigint NOT NULL,
    transportadora text NOT NULL
);

INSERT INTO nota (empresa_id, numero, emitente)
SELECT e.id, n.id, CASE e.id WHEN 1 THEN 'Prefeitura de Sao Bernardo' ELSE 'Secretaria de Obras' END
FROM generate_series(1, 2) AS e(id), generate_series(1, 40) AS n(id);

-- Toda tupla de item existe em nota: confirmada.
INSERT INTO item (empresa_id, numero, sequencia, descricao)
SELECT n.empresa_id, n.numero, s.id, 'peca de reposicao'
FROM nota n, generate_series(1, 3) AS s(id);

-- 60 linhas contidas, 4 órfãs em 2 tuplas distintas: quebrada.
INSERT INTO rateio (id, nota_empresa_id, nota_numero, centro_custo)
SELECT row_number() OVER (), n.empresa_id, n.numero, 'Secretaria de Saude'
FROM nota n WHERE n.numero <= 30;

INSERT INTO rateio (id, nota_empresa_id, nota_numero, centro_custo) VALUES
    (901, 1, 901, 'Secretaria de Saude'),
    (902, 1, 901, 'Secretaria de Saude'),
    (903, 2, 902, 'Secretaria de Obras'),
    (904, 2, 902, 'Secretaria de Obras');

-- Nulidade parcial: a constraint jamais olha para estas linhas.
INSERT INTO rateio (id, nota_empresa_id, nota_numero, centro_custo) VALUES
    (911, 1, NULL, 'Secretaria de Saude'),
    (912, NULL, 7, 'Secretaria de Obras'),
    (913, 2, NULL, 'Secretaria de Saude');

INSERT INTO contrato (empresa_id, ano, numero, objeto)
SELECT 1, 2024, n.id, 'servico de manutencao' FROM generate_series(1, 10) AS n(id);

INSERT INTO aditivo (id, empresa_id, ano, resumo)
SELECT n.id, 1, 2024, 'servico de manutencao' FROM generate_series(1, 10) AS n(id);

INSERT INTO lotacao (unidade_id, setor_id, vigencia)
SELECT u.id, s.id, '2024' FROM generate_series(1, 4) AS u(id), generate_series(1, 5) AS s(id);

INSERT INTO alocacao (unidade_id, setor_id, turno)
SELECT u.id, s.id, 'Equipe de Campo' FROM generate_series(1, 4) AS u(id), generate_series(1, 5) AS s(id);

INSERT INTO posto (unidade_id, setor_id, endereco)
SELECT u.id, s.id, 'Rua das Acacias 42' FROM generate_series(1, 4) AS u(id), generate_series(1, 5) AS s(id);

INSERT INTO movimentacao (id, unidade_id, setor_id, motivo)
SELECT row_number() OVER (), u.id, s.id, 'Leitura Manual Bloco C'
FROM generate_series(1, 4) AS u(id), generate_series(1, 5) AS s(id);

INSERT INTO frete (id, empresa_id, nota_numero, transportadora)
SELECT n.id, 1, n.id, 'Construtora Horizonte LTDA' FROM generate_series(1, 10) AS n(id);

ANALYZE;

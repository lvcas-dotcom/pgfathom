-- Cenário da mineração de evidência de uso: relações que o casamento de nome
-- não alcança e que o SQL guardado no próprio banco declara.
--
-- O caso central é `resp_tecnico → funcionario.id`: nenhuma heurística de
-- nomenclatura chega lá, e a view chega. Os dados são íntegros, para que a
-- validação confirme e o ganho seja demonstrável ponta a ponta.

CREATE TABLE funcionario (
    id   bigint PRIMARY KEY,
    nome text NOT NULL
);

CREATE TABLE equipamento (
    id        bigint PRIMARY KEY,
    descricao text NOT NULL
);

CREATE TABLE cliente (
    id   bigint PRIMARY KEY,
    cpf  text NOT NULL,
    nome text NOT NULL
);

CREATE TABLE os_servico (
    id           bigint PRIMARY KEY,
    resp_tecnico bigint NOT NULL,   -- só a view revela: aponta para funcionario
    patrimonio   bigint NOT NULL,   -- só a função revela: aponta para equipamento
    cliente_id   bigint NOT NULL    -- alcançável por nome; a view reforça
);

INSERT INTO funcionario (id, nome) VALUES
    (1, 'Maria Aparecida Silva'),
    (2, 'Joao Carlos Pereira');

INSERT INTO equipamento (id, descricao) VALUES
    (10, 'Compressor Industrial'),
    (20, 'Bomba Centrifuga');

INSERT INTO cliente (id, cpf, nome) VALUES
    (1, '529.318.470-11', 'Construtora Horizonte LTDA'),
    (2, '145.892.663-04', 'Prefeitura de Sao Bernardo');

INSERT INTO os_servico (id, resp_tecnico, patrimonio, cliente_id) VALUES
    (1, 1, 10, 1),
    (2, 2, 20, 2),
    (3, 1, 10, 1),
    (4, 2, 20, 2);

-- A view é a evidência: o código real trata resp_tecnico como referência a
-- funcionario, e cliente_id como referência a cliente.
CREATE VIEW vw_os_completa AS
SELECT o.id,
       f.nome AS responsavel,
       c.nome AS cliente
FROM os_servico o
JOIN funcionario f ON o.resp_tecnico = f.id
JOIN cliente c ON o.cliente_id = c.id;

-- Corpo de função em dollar quoting: mesma prova, outra fonte.
CREATE FUNCTION fn_equipamentos_da_os(p_os bigint)
RETURNS TABLE (descricao text)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT e.descricao
    FROM os_servico o
    JOIN equipamento e ON o.patrimonio = e.id
    WHERE o.id = p_os;
END;
$$;

-- SQL montado em string: o extrator precisa vê-lo como texto e ignorá-lo.
-- Se ele extrair daqui, inventa uma relação entre cliente e equipamento.
CREATE FUNCTION fn_dinamica(p_tabela text)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    v_total bigint;
BEGIN
    EXECUTE 'SELECT count(*) FROM cliente c JOIN equipamento e ON c.id = e.id'
        INTO v_total;
    RETURN v_total;
END;
$$;

ANALYZE;

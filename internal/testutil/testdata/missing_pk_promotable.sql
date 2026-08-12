-- Cenário: tabela sem PK, mas com uma constraint UNIQUE cujas colunas são
-- todas NOT NULL. O catálogo já prova a chave — nenhuma sondagem de dado é
-- necessária, só a promoção.

CREATE TABLE cadastro_pessoa (
    id  bigint NOT NULL,
    cpf text   NOT NULL,
    nome text,
    UNIQUE (cpf)
);

-- Suportada, para provar que o achado não engole tabela saudável.
CREATE TABLE municipio (
    id   bigint PRIMARY KEY,
    nome text NOT NULL
);

INSERT INTO cadastro_pessoa (id, cpf, nome) VALUES
    (1, '529.318.470-11', 'Maria Aparecida Silva'),
    (2, '145.892.663-04', 'Joao Carlos Pereira');

INSERT INTO municipio (id, nome) VALUES (1, 'Sao Bernardo do Campo');

ANALYZE;

## ADDED Requirements

### Requirement: Tabela sem chave primária é reportada

O comando SHALL reportar toda tabela em escopo que não tenha chave primária, acompanhada da estimativa de linhas, que é o que indica a gravidade.

Sem PK a tabela não tem identidade de linha: replicação lógica não a cobre e todo `UPDATE`/`DELETE` por linha vira varredura. Quando existe um `UNIQUE` cujas colunas são todas `NOT NULL`, o achado SHALL oferecer a promoção dessa unique a PK, que é um caminho provado pelo catálogo e de custo baixo.

#### Scenario: Tabela sem PK com unique promovível

- **WHEN** uma tabela não tem PK mas tem uma constraint `UNIQUE` com todas as colunas `NOT NULL`
- **THEN** o achado aparece com a sugestão de promover essa unique a chave primária

#### Scenario: Tabela sem PK e sem unique promovível

- **WHEN** uma tabela não tem PK nem unique promovível
- **THEN** o achado aparece marcado como precisando de sondagem de chave, sem afirmar qual é a chave

#### Scenario: Tabela com PK não gera achado

- **WHEN** todas as tabelas do escopo têm chave primária
- **THEN** nenhum achado desse tipo é emitido

### Requirement: A chave sugerida é confirmada por contagem, nunca por estimativa

Quando não há unique promovível, o comando MAY sondar a unicidade de um conjunto de colunas para nomear a chave. A sondagem SHALL emitir apenas contagens — nenhum valor de coluna em struct, log, JSON ou erro. Unicidade SHALL ser confirmada apenas em varredura completa da tabela: modo amostrado nunca confirma chave, porque uma duplicata pode estar fora da amostra.

Um conjunto sondado que estoure o `statement_timeout` SHALL sair como `unverified` e a execução prossegue. Estimativa de `n_distinct` MAY priorizar o que sondar, mas nunca SHALL ser afirmada como chave.

#### Scenario: Chave composta confirmada por contagem

- **WHEN** a sondagem completa uma tabela cuja unicidade real é composta e as contagens provam `total = distinct` sem nulos
- **THEN** o achado nomeia a chave composta como confirmada

#### Scenario: Amostra não confirma chave

- **WHEN** a tabela é grande e só pôde ser lida por amostra
- **THEN** nenhuma chave sai como confirmada; a chave sai `unverified` e a tabela consta da cobertura

#### Scenario: Sondagem não vaza valor

- **WHEN** a sondagem roda contra fixtures com valores plantados
- **THEN** nenhum desses valores aparece na saída em terminal, no JSON ou no log

### Requirement: Coluna quente sem índice é reportada com o tipo de índice apropriado

O comando SHALL reportar toda coluna que apareça em predicado de junção ou de filtro no código real — view, função ou `pg_stat_statements` — com recorrência acima do limiar configurável e sem índice que a lidere. O achado SHALL recomendar o método de índice apropriado ao operador observado e ao tipo da coluna.

A recomendação SHALL ser `btree` por padrão, servindo igualdade e faixa. `GIN` SHALL ser recomendado para contenção em `jsonb`/array e para full-text; `GIN gin_trgm_ops` para `LIKE`/`ILIKE` de infixo quando `pg_trgm` estiver instalada; um método de vizinhança para coluna `vector` quando `pgvector` estiver instalada. Extensão ausente nunca vira erro nem recomendação impossível: degrada para `btree` com nota ou omite o achado.

#### Scenario: Coluna de junção sem índice

- **WHEN** uma view cruza repetidamente uma coluna que não lidera nenhum índice
- **THEN** o achado aparece recomendando `btree` sobre essa coluna

#### Scenario: Contenção em jsonb sem GIN

- **WHEN** uma função usa `@>` sobre uma coluna `jsonb` sem índice `GIN`
- **THEN** o achado recomenda `GIN` para essa coluna

#### Scenario: Infixo sem pg_trgm

- **WHEN** um predicado `LIKE '%x%'` incide sobre uma coluna sem índice e `pg_trgm` não está instalada
- **THEN** o achado recomenda `btree` com a nota de que o infixo pede `pg_trgm`, sem falhar

#### Scenario: FK sem índice não é duplicada

- **WHEN** uma coluna já é reportada por `fk_without_index`
- **THEN** ela não gera também um achado de coluna quente

### Requirement: A leitura de dado da sondagem é opcional e limitada por teto

A sondagem de chave SHALL rodar automaticamente apenas para tabelas abaixo de um teto de linhas estimadas, configurável. Tabelas acima do teto SHALL constar da cobertura como não sondadas, mantendo a oferta de promoção de unique quando houver. Uma flag SHALL desligar a leitura de dado por inteiro, devolvendo o comando ao comportamento catálogo-puro.

#### Scenario: Tabela grande não é sondada

- **WHEN** uma tabela sem PK excede o teto de tamanho
- **THEN** a sondagem não roda contra ela e a cobertura registra a tabela como não sondada

#### Scenario: Leitura de dado desligada

- **WHEN** o comando roda com a sondagem desligada
- **THEN** nenhuma transação de leitura de dado é aberta e os achados de PK saem apenas do catálogo

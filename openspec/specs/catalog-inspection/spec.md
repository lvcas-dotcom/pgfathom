# catalog-inspection Specification

## Purpose
TBD - created by archiving change catalog-inspection. Update Purpose after archive.
## Requirements
### Requirement: Leitura completa da estrutura declarada

A ferramenta SHALL ler do catálogo, para cada schema no escopo: tabelas, colunas com tipo formatado e tipo base normalizado, nulabilidade, valor padrão e posição, chaves primárias em ordem, constraints unique, índices com suas colunas em ordem, e comentários de tabela e de coluna.

O tipo base SHALL ser normalizado para comparação. Comparar tipos formatados diretamente produz falso negativo entre grafias equivalentes do mesmo tipo.

#### Scenario: Estrutura básica

- **WHEN** um schema com tabelas, chaves e índices é lido
- **THEN** o modelo resultante contém todas as tabelas do escopo com colunas, chaves, índices e comentários

#### Scenario: Tipos equivalentes normalizam igual

- **WHEN** uma coluna é declarada como `bigint` e outra como `int8`
- **THEN** ambas produzem o mesmo tipo base no modelo

### Requirement: Estado de validação da chave estrangeira é preservado

Toda chave estrangeira lida SHALL carregar o valor de `pg_constraint.convalidated`.

Uma constraint criada `NOT VALID` e nunca validada bloqueia violações novas mas nunca verificou as linhas preexistentes, enquanto aparece idêntica a qualquer outra em `\d` e em ferramenta de diagrama. Descartar esse campo na leitura elimina a informação que sustenta um achado inteiro.

A ferramenta SHALL também determinar, para cada chave estrangeira, se existe índice utilizável na coluna filha — ou seja, um índice com a coluna em posição inicial.

#### Scenario: Constraint não validada é distinguível

- **WHEN** o schema contém uma FK criada com `NOT VALID` e nunca validada
- **THEN** o modelo a marca como não validada

#### Scenario: Índice do lado filho é detectado

- **WHEN** uma FK tem índice cuja coluna inicial é a coluna filha
- **THEN** a FK é marcada como indexada

#### Scenario: Índice em posição não inicial não conta

- **WHEN** a coluna filha aparece num índice composto, mas não em posição inicial
- **THEN** a FK é marcada como não indexada, porque esse índice não serve para a busca

### Requirement: Estatística de uso lida junto do timestamp de reset

A ferramenta SHALL ler as estatísticas de uso de tabela sempre acompanhadas do momento do último reset de estatística do banco.

Contador de uso sem esse timestamp não tem significado. Quando o momento do reset não puder ser determinado, os contadores SHALL ser marcados como não interpretáveis, e nenhum achado pode ser derivado deles.

#### Scenario: Contadores acompanhados do reset

- **WHEN** as estatísticas de uso são lidas
- **THEN** o timestamp de reset consta do modelo, ou o estado é explicitamente marcado como desconhecido

### Requirement: Nenhum dado de tabela do usuário é lido nesta fase

A camada de catálogo MUST NOT emitir consulta que leia linha de tabela do usuário. Ela lê exclusivamente `pg_catalog`, `information_schema` e as visões de estatística.

Leitura de dado começa na validação, numa fase posterior e numa camada separada. Manter essa fronteira explícita é o que permite afirmar, sobre esta fase, que nenhum valor pode vazar porque nenhum valor é lido.

#### Scenario: Consultas restritas ao catálogo

- **WHEN** um `audit` completo é executado
- **THEN** nenhuma consulta emitida referencia uma tabela do usuário

### Requirement: Tabela pulada é registrada, nunca silenciada

Tabela que não puder ser analisada SHALL ser registrada na cobertura com o motivo, e a execução SHALL continuar. Os motivos SHALL cobrir: falta de privilégio `SELECT`, chave primária composta, ausência de chave primária, tabela particionada e herança de tabela.

Tabela particionada SHALL ser lida a partir da tabela pai, e as partições MUST NOT ser iteradas separadamente.

#### Scenario: Falta de privilégio

- **WHEN** o papel conectado não tem `SELECT` numa tabela do escopo
- **THEN** a tabela aparece na lista de puladas por privilégio, e a execução prossegue

#### Scenario: Forma não suportada

- **WHEN** uma tabela tem chave primária composta
- **THEN** ela aparece na cobertura com o motivo correspondente

#### Scenario: Partições não são iteradas

- **WHEN** o schema contém uma tabela particionada com várias partições
- **THEN** o modelo contém a tabela pai e não contém uma entrada por partição

### Requirement: Escopo controlável por schema e por padrão de exclusão

A ferramenta SHALL aceitar a lista de schemas a analisar, com padrão `public`, e uma lista de padrões de tabela a excluir.

A ferramenta SHALL oferecer um modo que resolve o escopo a partir do catálogo, incluindo todo schema não-sistema sobre o qual o papel conectado tenha privilégio `USAGE`. Schema sem `USAGE` MUST NOT entrar no escopo: cada tabela dele apareceria como pulada por falta de privilégio, o que apresentaria como achado aquilo que é apenas ausência de relação entre o papel e aquela parte do banco.

A ferramenta SHALL aceitar padrões de exclusão de schema, aplicados antes de qualquer leitura de catálogo. Esses padrões SHALL ser distintos dos padrões de exclusão de tabela: um padrão de tabela MUST NOT remover um schema do escopo, sob pena de mudar o significado de invocações já existentes.

Schema removido por padrão de exclusão SHALL ser registrado em campo distinto do schema que apenas não foi pedido. Exclusão pedida e ausência de pedido são fatos diferentes sobre a execução, e fundi-los faria o relatório afirmar intenção onde não houve nenhuma.

Escopo vazio SHALL ser erro de uso, e a mensagem SHALL distinguir lista vazia de exclusão total. Executar sobre nenhum schema produziria um relatório sobre nada, indistinguível de um banco sem achado.

Padrão de exclusão de tabela SHALL ser casado dentro dos schemas em escopo, contra o nome nu e contra o nome qualificado. Padrão que referencia schema fora do escopo não tem efeito.

#### Scenario: Exclusão por padrão

- **WHEN** um padrão de exclusão casa com algumas tabelas
- **THEN** essas tabelas não são analisadas e aparecem na cobertura como excluídas

#### Scenario: Resolução de todos os schemas

- **WHEN** o modo de escopo total é solicitado num banco com vários schemas
- **THEN** todo schema não-sistema com privilégio `USAGE` entra no escopo, e os schemas de sistema não

#### Scenario: Schema sem privilégio de uso fica fora do escopo

- **WHEN** o modo de escopo total é solicitado e o papel não tem `USAGE` sobre um dos schemas
- **THEN** esse schema não entra no escopo, e suas tabelas não aparecem na lista de puladas por falta de privilégio

#### Scenario: Exclusão de schema por padrão

- **WHEN** um padrão de exclusão de schema casa com um ou mais schemas
- **THEN** esses schemas não são consultados e aparecem na cobertura como excluídos por filtro

#### Scenario: Padrão de tabela não remove schema

- **WHEN** um padrão de exclusão de tabela coincide com o nome de um schema em escopo
- **THEN** o schema permanece no escopo, e apenas as tabelas cujo nome casa com o padrão são excluídas

#### Scenario: Escopo vazio falha

- **WHEN** os padrões de exclusão de schema removem todos os schemas do escopo
- **THEN** a execução falha com erro de uso, e a mensagem indica que a exclusão esvaziou o escopo

### Requirement: Cobertura preenchida em toda execução

O resultado SHALL conter a estrutura de cobertura preenchida, com o total de tabelas do escopo, o total analisado e todas as listas de puladas.

#### Scenario: Contagens fecham

- **WHEN** uma execução termina
- **THEN** o total analisado somado às tabelas puladas por todos os motivos é igual ao total do escopo

### Requirement: Cobertura declara a dimensão de schema

O resultado SHALL registrar, em toda execução, o total de schemas visíveis ao papel conectado, quantos foram analisados, a lista dos que existem e ficaram fora do escopo, e a lista dos removidos por padrão de exclusão.

Esta exigência vale independentemente de qual modo de escopo foi usado, e em particular vale para a execução sem flag alguma. Um relatório sobre `public` que não mencione os outros onze schemas do banco está correto e ainda assim engana, que é precisamente o modo de falha que a regra de não reportar silêncio existe para impedir. Quem já sabe que precisa ampliar o escopo não é quem está sendo enganado pelo relatório.

Uma execução SHALL ser considerada completa apenas quando nenhum schema visível tiver ficado fora do escopo, além das condições já existentes sobre tabelas e candidatos.

#### Scenario: Execução no padrão declara o que não olhou

- **WHEN** a ferramenta é executada sem flag de escopo num banco que tem outros schemas além de `public`
- **THEN** a cobertura lista os schemas não analisados e indica como incluí-los

#### Scenario: Banco de schema único não gera ruído

- **WHEN** a ferramenta é executada num banco cujo único schema não-sistema é o que está em escopo
- **THEN** nenhuma menção a schema fora do escopo aparece, porque não há nada a declarar

#### Scenario: Cobertura incompleta com schema fora do escopo

- **WHEN** existe schema visível fora do escopo analisado
- **THEN** a execução não é reportada como cobertura completa


## MODIFIED Requirements

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

## ADDED Requirements

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

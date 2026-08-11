# usage-evidence Specification

## Purpose
TBD - created by archiving change sql-join-probe. Update Purpose after archive.
## Requirements
### Requirement: O extrator reconhece igualdades qualificadas e ignora o resto sem erro

O extrator SHALL reconhecer igualdades entre duas referências de coluna qualificadas, resolvendo aliases declarados nas cláusulas `FROM` e `JOIN` do mesmo statement. SQL que ele não reconhece MUST ser ignorado sem erro: a extração degrada, nunca falha.

Junção perdida é sinal perdido, e o candidato volta ao caminho da inferência por nome. Extração errada gera candidato que a validação derruba. Em nenhum caso o extrator produz resposta errada.

#### Scenario: Junção em view é extraída

- **WHEN** uma view junta duas colunas por igualdade com aliases
- **THEN** o par de colunas resolvido aparece na evidência extraída

#### Scenario: SQL malformado não derruba a extração

- **WHEN** um corpo de função contém SQL que o extrator não entende
- **THEN** a extração daquele corpo retorna vazio e as demais fontes seguem processadas

### Requirement: O tokenizador atravessa o que faria o extrator mentir

O tokenizador SHALL tratar comentário de linha e de bloco, string com aspas simples, dollar quoting com e sem tag, e identificador entre aspas duplas — preservando caixa e espaços no identificador citado e normalizando o identificador nu para minúsculas.

Um `=` dentro de string ou comentário viraria predicado fantasma; corpo de função vem inteiro dentro de `$$`.

#### Scenario: Igualdade dentro de string é invisível

- **WHEN** o SQL contém `'a.x = b.y'` dentro de uma string ou comentário
- **THEN** nenhuma evidência é extraída daquele trecho

#### Scenario: Dollar quoting delimita o corpo, não o esconde

- **WHEN** um corpo de função PL/pgSQL vem entre `$$` ou `$tag$`
- **THEN** as junções do corpo são extraídas normalmente

### Requirement: A evidência vem do catálogo, e a disponibilidade é declarada

As fontes SHALL ser as definições de view e os corpos de função `sql` e `plpgsql` dos schemas em escopo, e — quando a extensão responder — as queries normalizadas de `pg_stat_statements`. A cobertura SHALL declarar se `pg_stat_statements` estava disponível. Nenhuma linha de tabela de usuário é lida.

Ausência de evidência de uso não pode parecer ausência de uso.

#### Scenario: Extensão ausente não é erro

- **WHEN** `pg_stat_statements` não está instalada
- **THEN** a execução segue com as demais fontes e a cobertura registra a indisponibilidade

### Requirement: Junção vira sinal de peso máximo e vira candidato com âncora de chave

Cada junção extraída SHALL virar sinal no candidato correspondente, com peso superior a qualquer sinal de nome, identificando a origem: view, função ou statements. Quando o par não existe por nome, a junção SHALL criar o candidato, desde que o lado pai seja a chave primária da tabela, os tipos sejam compatíveis e a coluna filha não participe de FK declarada. Sem âncora de chave, o par MUST NOT virar candidato.

Igualdades do mesmo objeto entre o mesmo par de tabelas SHALL ser agrupadas antes de ancorar. Quando o conjunto de colunas do lado pai for exatamente a chave primária composta daquela tabela, o grupo SHALL produzir **um** candidato composto, com as colunas na ordem da chave, e MUST NOT produzir um candidato por igualdade. Quando o conjunto for parcial, nenhum candidato composto nasce dali.

Sem o agrupamento, uma junção composta em view chegaria como duas hipóteses de coluna única, cada uma apontando para metade de uma chave que sozinha não é única — exatamente o tipo de candidato que a âncora de chave existe para recusar. A relação ficaria invisível justamente no caso em que a evidência é mais forte.

Junção no código real é uso, não convenção — e é o único sinal que alcança relações cujos nomes não se parecem.

#### Scenario: Relação invisível ao nome nasce da view

- **WHEN** uma view junta uma coluna cujo nome não se parece com a tabela alvo à chave primária dessa tabela
- **THEN** o candidato existe, carrega o sinal de junção em view, e pode ser confirmado pela validação

#### Scenario: Evidência reforça candidato nascido do nome

- **WHEN** uma junção extraída coincide com um candidato já gerado por nome
- **THEN** o candidato ganha o sinal de junção e seu score sobe

#### Scenario: Par sem âncora não vira candidato

- **WHEN** nenhum dos lados da igualdade é chave primária da sua tabela
- **THEN** nenhum candidato nasce daquele par

#### Scenario: Junção composta vira um candidato composto

- **WHEN** uma view junta duas colunas de uma tabela às duas colunas da chave primária composta de outra
- **THEN** nasce um único candidato composto, com as colunas na ordem da chave, e nenhum candidato de coluna única para o mesmo par

#### Scenario: Junção que cobre parte da chave não ancora

- **WHEN** uma view junta apenas uma das duas colunas de uma chave primária composta
- **THEN** nenhum candidato nasce daquela junção

### Requirement: Evidência de uso nunca conclui sozinha

Candidato nascido ou reforçado por evidência de uso SHALL seguir o mesmo caminho de todos: pré-filtro, validação e veredito. A evidência MUST NOT alterar veredito diretamente.

#### Scenario: Candidato de view ainda é validado

- **WHEN** um candidato nasce de junção em view
- **THEN** ele entra na validação como qualquer outro e o veredito vem dos dados


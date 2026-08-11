## MODIFIED Requirements

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

## MODIFIED Requirements

### Requirement: Sinais positivos e seus pesos relativos

O sistema SHALL emitir os seguintes sinais positivos, em ordem decrescente de peso:

Casamento exato do nome de entidade com o nome da tabela, sem normalização, é o sinal de nome mais forte. Tipo base idêntico à chave alvo é forte. Alvo único para aquele nome é forte, porque ambiguidade é o principal gerador de ruído. Coluna já possuir índice é moderado, porque indica que alguém faz join por ela. Comentário de coluna ou de tabela mencionando a entidade alvo é moderado. Coluna ser `NOT NULL` é fraco mas positivo.

Concordância de aridade — todas as colunas de uma chave de mais de uma coluna casando de uma vez — SHALL emitir um sinal positivo próprio, uma única vez, com peso crescendo sublinearmente na aridade. Ele MUST NOT ser emitido para chave de coluna única.

Os demais sinais SHALL ser avaliados sobre a chave inteira e emitidos uma vez por candidato, qualquer que seja a aridade: tipo compatível significa todas as posições compatíveis; coluna indexada significa índice cujas colunas iniciais são a chave filha na ordem dela; `NOT NULL` significa todas as colunas não nulas.

Emitir os sinais existentes uma vez por coluna seria errado duas vezes. A soma satura no mesmo teto, então a aridade viraria um caminho indireto para fixar o score no máximo; e o score deixaria de ser explicável, porque uma chave de quatro colunas exibiria vinte sinais dizendo quatro fatos.

#### Scenario: Casamento exato pontua mais que normalizado

- **WHEN** dois candidatos são idênticos exceto pela forma de casamento, um exato e outro normalizado
- **THEN** o de casamento exato tem score estritamente maior

#### Scenario: Tipo idêntico pontua mais que apenas compatível

- **WHEN** dois candidatos são idênticos exceto pelo tipo, um idêntico e outro apenas compatível
- **THEN** o de tipo idêntico tem score estritamente maior

#### Scenario: Coluna indexada pontua mais

- **WHEN** dois candidatos são idênticos exceto por um ter índice na coluna filha
- **THEN** o indexado tem score estritamente maior

#### Scenario: Concordância de aridade pontua mais que uma coluna só

- **WHEN** dois candidatos são idênticos exceto pela aridade, um de duas colunas e outro de uma
- **THEN** o de duas colunas tem score estritamente maior, e carrega o sinal de aridade uma única vez

#### Scenario: Aridade não multiplica os outros sinais

- **WHEN** um candidato de chave composta é inspecionado
- **THEN** cada sinal de nome, tipo, índice, comentário e nulidade aparece no máximo uma vez

#### Scenario: Casamento espelho não reivindica evidência de nome

- **WHEN** um candidato composto nasce de casamento espelho, em que nada no filho nomeia o alvo
- **THEN** ele carrega o sinal de aridade e nenhum sinal de nome, e pontua abaixo de um candidato de coluna única cujo nome aponta para o alvo

#### Scenario: Índice conta pela ordem da chave

- **WHEN** a tabela filha tem índice sobre as colunas da chave em ordem diferente da chave
- **THEN** o sinal de coluna indexada não é emitido

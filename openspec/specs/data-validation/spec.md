# data-validation Specification

## Purpose
TBD - created by archiving change data-validation. Update Purpose after archive.
## Requirements
### Requirement: A validação lê contagens, nunca linhas

A query de validação SHALL agregar por valor distinto antes do anti-join e SHALL retornar exclusivamente contagens: valores distintos, linhas não nulas, órfãos por valor, órfãos por linha e máximo de linhas por valor. Nenhum valor de tabela do usuário MUST sair da query para o programa.

Contenção por linha e por valor contam histórias diferentes — um valor inválido repetido um milhão de vezes e um milhão de valores raros inválidos são problemas distintos — e ambas SHALL ser reportadas.

#### Scenario: Só contagens atravessam a fronteira

- **WHEN** uma validação executa
- **THEN** o resultado carrega apenas contagens e durações, e a varredura de vazamento sobre a saída completa do binário não encontra valor plantado

#### Scenario: As duas contenções saem sempre

- **WHEN** um candidato é validado
- **THEN** a contenção por linha e a por valor distinto aparecem nas métricas, junto com as contagens de órfãos nas duas unidades

### Requirement: Modo amostrado é triagem e nunca confirma

O modo padrão SHALL amostrar a tabela filha com `TABLESAMPLE SYSTEM` calibrado para um alvo de linhas, com `BERNOULLI` em tabela pequena e leitura direta quando a estimativa de linhas cabe no alvo. Em modo amostrado, nenhum candidato MUST receber o veredito de confirmada — sem exceção. Órfão encontrado em amostra é real, então o veredito de quebrada SHALL permanecer possível, com as contagens declaradas como piso.

Órfãos entram em lote e se agrupam fisicamente nas mesmas páginas — exatamente o que amostragem por página é pior em encontrar. Amostra limpa não é evidência de ausência.

#### Scenario: Amostra limpa não confirma

- **WHEN** uma validação amostrada não encontra nenhum órfão
- **THEN** o veredito é fraca, com motivo apontando que só `--full` pode confirmar

#### Scenario: Órfão em amostra é achado real

- **WHEN** uma validação amostrada encontra órfãos acima do limiar de quebra
- **THEN** o veredito é quebrada e as contagens de órfãos são reportadas como mínimo, não como total

#### Scenario: Tabela pequena é lida direta

- **WHEN** a estimativa de linhas da filha não excede o alvo de amostra
- **THEN** a tabela é lida por inteiro e a validação vale como modo completo

#### Scenario: O modo aparece na saída

- **WHEN** `discover` valida em modo amostrado
- **THEN** a saída declara o modo e o aviso de que nada amostrado é confirmado

### Requirement: Vereditos seguem regras fixas com zona morta explícita

A atribuição SHALL seguir, nesta ordem: filha vazia, um único valor distinto ou nulos acima do teto tornam o candidato fraca, independente da contenção; contenção total por linha e por valor com mais de um valor distinto, em modo completo, torna confirmada; contenção por linha no limiar de quebra ou acima, com órfãos presentes, torna quebrada; contenção abaixo do limiar de rejeição torna rejeitada; o que fica entre os limiares torna fraca, com motivo.

Setenta por cento de contenção não é relação quebrada nem coincidência: é um caso para um humano, e fingir certeza para qualquer lado é o erro que a ferramenta promete não cometer.

#### Scenario: Contenção total confirma em modo completo

- **WHEN** uma validação completa encontra contenção total nas duas dimensões e mais de um valor distinto
- **THEN** o veredito é confirmada

#### Scenario: Órfãos plantados produzem quebrada com contagem exata

- **WHEN** uma validação completa encontra contenção acima do limiar de quebra e órfãos plantados na fixture
- **THEN** o veredito é quebrada e as contagens de órfãos por linha e por valor batem com o que foi plantado

#### Scenario: Contenção baixa rejeita

- **WHEN** a contenção fica abaixo do limiar de rejeição
- **THEN** o veredito é rejeitada

#### Scenario: A zona morta vira fraca com motivo

- **WHEN** a contenção fica entre o limiar de rejeição e o de quebra
- **THEN** o veredito é fraca e o motivo registra a ambiguidade

#### Scenario: Evidência estatística insuficiente vira fraca

- **WHEN** a coluna filha tem um único valor distinto, nulos dominantes ou tabela vazia
- **THEN** o veredito é fraca independente da contenção medida

### Requirement: Timeout não interrompe a execução

Cada validação SHALL rodar sob um `statement_timeout` próprio, aplicado por `SET LOCAL` em transação somente-leitura. Candidato cuja validação estoura o teto SHALL sair como não validado com o motivo, o contador de estourados na cobertura SHALL subir, e a execução MUST continuar para os demais.

Abortar a execução inteira porque uma tabela não coube no teto jogaria fora o trabalho de todas as que couberam.

#### Scenario: Um timeout não derruba a corrida

- **WHEN** uma validação excede o teto de tempo
- **THEN** aquele candidato sai como não validado com motivo de timeout, a cobertura conta o estouro, e os demais candidatos são validados normalmente

### Requirement: Concorrência limitada pelo teto da sessão

As validações SHALL rodar concorrentes sob o mesmo limite que dimensiona o pool de conexões, e o cancelamento do contexto SHALL propagar para toda query em voo.

Dois limites independentes criariam a combinação onde o pool bloqueia o grupo e o cancelamento espera conexão.

#### Scenario: O limite é respeitado

- **WHEN** há mais candidatos que o limite de concorrência
- **THEN** o número de validações simultâneas nunca excede o limite configurado

#### Scenario: Cancelamento derruba as queries em voo

- **WHEN** o contexto é cancelado no meio das validações
- **THEN** nenhuma query permanece ativa no servidor

### Requirement: Identificadores entram na query citados

Todo nome de schema, tabela e coluna interpolado na query de validação SHALL passar por citação de identificador. A fração de amostragem SHALL ser calculada pelo código e interpolada como literal numérico, nunca vinda de entrada do usuário.

A sessão read-only impede escrita; citação impede que um identificador exótico quebre a query — ou mude o que ela mede.

#### Scenario: Nome exótico não quebra a validação

- **WHEN** uma tabela ou coluna tem nome com maiúsculas, espaço ou aspas
- **THEN** a validação executa corretamente contra o objeto certo

### Requirement: A cobertura fecha a conta da validação

A cobertura SHALL informar quantos candidatos foram validados e quantos estouraram o teto, e a soma de validados, estourados e não tentados SHALL fechar com o total de candidatos que chegaram à validação.

#### Scenario: O funil da validação fecha

- **WHEN** `discover` termina uma execução com validação
- **THEN** validados mais estourados fecham com o conjunto que entrou, e a cobertura reporta os dois números


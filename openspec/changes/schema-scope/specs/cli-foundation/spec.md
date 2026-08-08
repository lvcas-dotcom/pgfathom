## MODIFIED Requirements

### Requirement: Binário único com subcomandos

O projeto SHALL produzir um binário chamado `pgfathom`, sem dependência de runtime externo e sem cgo, com subcomandos.

`pgfathom` sem argumento SHALL exibir a ajuda e sair com código zero.

Os subcomandos que produzem resultado SHALL aceitar `--format` com os valores `table`, `json` e `sql`, e `--out` apontando um diretório de artefatos. `--format sql` SHALL exigir `--out`, e sua ausência SHALL ser erro de uso.

`--out` SHALL ser aceito junto de qualquer formato: quando presente, os artefatos são escritos e o relatório do formato escolhido continua saindo. Formato desconhecido SHALL ser erro de uso, nunca degradação silenciosa para um padrão.

SQL não sai em stdout porque um formato que cabe num pipe convida ao pipe, e o cabeçalho de revisão obrigatória vira decoração que ninguém lê porque ninguém abriu o arquivo.

Os subcomandos que leem catálogo SHALL aceitar `--schema` com a lista explícita de schemas, `--all-schemas` para resolver o escopo a partir do catálogo, e `--exclude-schema` com padrões glob de schemas a remover do escopo.

`--schema` e `--all-schemas` SHALL ser mutuamente exclusivas, e fornecê-las juntas SHALL ser erro de uso. Nenhuma precedência entre elas é aceitável: qualquer que fosse, produziria uma linha de comando cujo escopo real não é o que ela aparenta pedir.

A detecção de que `--schema` foi fornecida SHALL usar o estado de alteração da flag, e MUST NOT ser inferida da comparação com o valor padrão — `--schema public --all-schemas` é exatamente o caso ambíguo que a exclusividade existe para recusar.

Escopo de schema vazio SHALL ser erro de uso.

#### Scenario: Versão

- **WHEN** `pgfathom version` é executado
- **THEN** a versão, o commit e a data de build são impressos em stdout e o código de saída é 0

#### Scenario: Sem argumento

- **WHEN** `pgfathom` é executado sem argumento
- **THEN** a ajuda é impressa e o código de saída é 0

#### Scenario: Subcomando desconhecido

- **WHEN** `pgfathom naoexiste` é executado
- **THEN** a mensagem de erro vai para stderr e o código de saída é 2

#### Scenario: `--format sql` sem `--out` é erro de uso

- **WHEN** um comando é executado com `--format sql` e sem `--out`
- **THEN** a mensagem explica que o formato exige um diretório e o código de saída é o de erro de uso

#### Scenario: `--out` acompanha os outros formatos

- **WHEN** um comando é executado com `--format table` e `--out`
- **THEN** os artefatos são escritos no diretório e o relatório em tabela sai normalmente em stdout

#### Scenario: Formato desconhecido não degrada

- **WHEN** um comando é executado com um valor de `--format` que não existe
- **THEN** a execução falha com erro de uso, sem cair para nenhum formato padrão

#### Scenario: `--schema` junto de `--all-schemas` é erro de uso

- **WHEN** um comando é executado com `--schema` e `--all-schemas` ao mesmo tempo
- **THEN** a execução falha com código 2 e a mensagem cita as duas flags como mutuamente exclusivas

#### Scenario: `--schema public --all-schemas` também falha

- **WHEN** um comando é executado com `--schema` recebendo exatamente o valor padrão e `--all-schemas` junto
- **THEN** a execução falha com código 2, porque a flag foi fornecida ainda que o valor coincida com o padrão

#### Scenario: Exclusão que esvazia o escopo é erro de uso

- **WHEN** `--exclude-schema` remove todos os schemas que entrariam no escopo
- **THEN** a execução falha com código 2 e a mensagem indica que a exclusão esvaziou o escopo

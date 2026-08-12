## MODIFIED Requirements

### Requirement: Binário único com subcomandos

O projeto SHALL produzir um binário chamado `pgfathom`, sem dependência de runtime externo e sem cgo, com subcomandos.

`pgfathom` sem argumento SHALL exibir a ajuda e sair com código zero. Um guia interativo MUST NOT tomar o lugar desse comportamento: quem usa o comando em script conta com ele, e um guia que se abre sozinho é surpresa em ambiente que não tem quem responda.

Os subcomandos que produzem resultado SHALL aceitar `--format` com os valores `table`, `json` e `sql`, e `--out` apontando um diretório de artefatos. `--format sql` SHALL exigir `--out`, e sua ausência SHALL ser erro de uso.

`--out` SHALL ser aceito junto de qualquer formato: quando presente, os artefatos são escritos e o relatório do formato escolhido continua saindo. Formato desconhecido SHALL ser erro de uso, nunca degradação silenciosa para um padrão.

SQL não sai em stdout porque um formato que cabe num pipe convida ao pipe, e o cabeçalho de revisão obrigatória vira decoração que ninguém lê porque ninguém abriu o arquivo.

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
- **THEN** a tabela sai em stdout e os artefatos são escritos no diretório

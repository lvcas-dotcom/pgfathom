## MODIFIED Requirements

### Requirement: Separação entre resultado e diagnóstico

Resultado destinado a consumo — tabela, JSON, SQL — SHALL ser escrito em stdout. Diagnóstico, aviso, progresso e erro SHALL ser escritos em stderr.

Esta separação SHALL valer sem exceção, porque a saída da ferramenta será canalizada para arquivo e para pipeline de CI, e diagnóstico misturado em stdout corrompe o consumo programático.

Quando o resultado for entregue como arquivo em disco, stdout SHALL receber o manifesto do que foi escrito — um caminho por artefato, com a contagem de achados de cada categoria. O manifesto é o resultado naquele modo; a ausência dele deixaria stdout vazio numa execução bem-sucedida, o que é indistinguível de falha.

#### Scenario: Redirecionamento preserva o resultado

- **WHEN** um comando é executado com stdout redirecionado para arquivo
- **THEN** o arquivo contém apenas o resultado, sem nenhuma linha de progresso ou aviso

#### Scenario: Erro não polui stdout

- **WHEN** um comando falha
- **THEN** a mensagem de erro está em stderr e stdout está vazio ou contém apenas resultado parcial válido

#### Scenario: Artefato em disco reporta o manifesto

- **WHEN** um comando escreve artefatos num diretório de saída
- **THEN** stdout lista cada caminho escrito com a contagem da respectiva categoria, e os avisos permanecem em stderr

### Requirement: Binário único com subcomandos

O projeto SHALL produzir um binário chamado `pgfathom`, sem dependência de runtime externo e sem cgo, com subcomandos.

`pgfathom` sem argumento SHALL exibir a ajuda e sair com código zero.

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
- **THEN** os artefatos são escritos no diretório e o relatório em tabela sai normalmente em stdout

#### Scenario: Formato desconhecido não degrada

- **WHEN** um comando é executado com um valor de `--format` que não existe
- **THEN** a execução falha com erro de uso, sem cair para nenhum formato padrão

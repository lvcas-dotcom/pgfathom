## ADDED Requirements

### Requirement: A marca é diagnóstica, e só aparece a quem está chegando

A marca em ASCII SHALL ser escrita em stderr, nunca em stdout, e SHALL aparecer apenas quando o destino for um terminal interativo.

Ela SHALL aparecer em exatamente dois momentos: o binário invocado sem argumento, e a abertura do guia interativo. Nenhum comando que produz resultado — `discover`, `audit`, `version` — SHALL imprimi-la.

#### Scenario: Marca não contamina resultado

- **WHEN** qualquer subcomando tem sua saída canalizada
- **THEN** nenhum byte da marca aparece em stdout

#### Scenario: Sem terminal, sem marca

- **WHEN** o binário é invocado sem argumento com stderr redirecionado para arquivo
- **THEN** o arquivo contém a ajuda e nenhum byte da marca

#### Scenario: Execução repetida não paga o custo

- **WHEN** `discover` roda em terminal interativo
- **THEN** a marca não é impressa

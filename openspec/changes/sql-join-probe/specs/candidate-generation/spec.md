## ADDED Requirements

### Requirement: A geração aceita evidência de junção como segunda origem

A geração SHALL aceitar, além do casamento de nome, uma lista de evidências de junção, e SHALL aplicar às hipóteses vindas dela as mesmas regras duras: alvo com chave única de coluna única, compatibilidade de tipo, coluna filha sem FK declarada.

As duas origens produzem o mesmo tipo de candidato, indistinguível no restante do pipeline exceto pelos sinais que carrega.

#### Scenario: Origens convergem no mesmo candidato

- **WHEN** nome e junção apontam o mesmo par de colunas
- **THEN** existe um único candidato carregando os sinais das duas origens

#### Scenario: Determinismo se mantém

- **WHEN** a geração roda repetidas vezes com o mesmo modelo e a mesma evidência
- **THEN** a saída é idêntica, na mesma ordem

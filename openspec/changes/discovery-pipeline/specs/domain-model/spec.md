## ADDED Requirements

### Requirement: A estimativa de linhas tem um único intérprete

A resolução de `pg_class.reltuples` — incluindo a sentinela que distingue "nunca analisada" de "vazia" — SHALL ter uma única implementação, no modelo, e toda camada que precise de estimativa de linhas MUST consumi-la por ela.

Desde o PostgreSQL 14 uma tabela nunca analisada devolve `-1`. Lida como contagem, ela parece vazia: a pontuação passa a tratá-la como tabela de domínio e a amostragem passa a tratá-la como cabendo inteira no alvo. São dois comportamentos errados e diferentes a partir da mesma leitura, o que é o desfecho previsível de duas implementações da mesma regra.

#### Scenario: Sentinela nunca é confundida com zero

- **WHEN** uma tabela nunca analisada tem sua estimativa de linhas consultada
- **THEN** o resultado declara que a estimativa é desconhecida, e nenhuma camada a interpreta como zero

#### Scenario: Uma implementação só

- **WHEN** as camadas que consomem estimativa de linhas são inspecionadas
- **THEN** todas resolvem a sentinela pela mesma função do modelo

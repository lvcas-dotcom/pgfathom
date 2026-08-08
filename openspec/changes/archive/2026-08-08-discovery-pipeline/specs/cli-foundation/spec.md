## ADDED Requirements

### Requirement: A execução é invocável fora do comando

A sequência completa de uma execução de descoberta — catálogo, detecção de nomenclatura, evidência de uso, geração, pré-filtro, validação e montagem do resultado — SHALL viver numa unidade com entrada programática, e o comando de linha SHALL apenas validar flags, montar opções, chamá-la e renderizar o que ela devolver.

Cada estágio SHALL ser identificável, para que a medição por etapa exigida pelo benchmark não precise reabrir a camada que executa.

Um número publicado sobre a ferramenta só é reproduzível por terceiros se medir o mesmo caminho que o usuário executa. Uma orquestração que existe apenas dentro de uma função de comando obriga quem mede a atravessar o binário ou a reimplementar a sequência, e nos dois casos o que se mede deixa de ser o que se entrega.

#### Scenario: Execução programática produz o mesmo resultado do comando

- **WHEN** a unidade de execução é chamada diretamente com as mesmas opções que o comando montaria
- **THEN** o resultado é o mesmo que o comando produz, sem passar por análise de flags nem por renderização

#### Scenario: Estágio que degrada avisa quem chamou

- **WHEN** um estágio cujo regime é degradar — evidência de uso, pré-filtro estatístico ou verificação de privilégio — falha
- **THEN** a unidade reporta o aviso a quem a chamou, em vez de escrever em um stream por conta própria, e a execução continua

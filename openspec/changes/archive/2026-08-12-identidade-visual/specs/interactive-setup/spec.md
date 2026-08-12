## ADDED Requirements

### Requirement: O guia veste a mesma convenção de cor do relatório

O guia SHALL usar a paleta do projeto com os mesmos papéis que o relatório: `#e2483d` onde a atenção vai, `#7ec9b8` para o que ficou assentado, `#8c7d73` para o que apoia sem competir.

Diferente do relatório, o guia SHALL adaptar os tons claros ao fundo do terminal, porque tem alguém olhando e nada nele é fixado por golden file.

#### Scenario: Uma escolha marcada se lê como assentada

- **WHEN** um item é marcado numa lista de múltipla escolha
- **THEN** a marca aparece na mesma cor que o relatório usa para relação confirmada

#### Scenario: Fundo claro não engole o texto

- **WHEN** o guia roda em terminal de fundo claro
- **THEN** os títulos usam a contraparte escura da cor clara da paleta, e permanecem legíveis

## Why

A change `2026-08-12-efficiency-audit` deu ao `audit` um caminho de sondagem para tabelas sem PK: promove `UNIQUE NOT NULL` quando existe, e tenta confirmar por contagem completa uma chave candidata quando não existe. Quando nem a promoção nem a sondagem confirmam nada — sem candidato plausível, ou candidato testado e rejeitado — o achado fica sem sugestão acionável. Quem revisa o relatório não tem caminho: precisa descobrir sozinho se a tabela tem uma chave real escondida numa combinação de colunas que a heurística de catálogo não tentou, ou se o caminho certo é criar uma coluna sintética.

Duas lacunas concretas do estado atual, ambas visíveis num audit real contra schema de gestão pública:

1. **Tabela-ponte sem índice.** `candidateKeys` só considera colunas de um índice não-único já existente. Uma tabela de associação com duas FKs de coluna única (`idkey_a`, `idkey_b`) e nenhum índice sobre o par nunca é testada — é exatamente o caso mais comum de tabela sem PK em schema legado, e a heurística atual não olha para lá.
2. **Nenhuma chave real existe.** Algumas tabelas legítimas não têm nem podem ter uma chave natural (log de auditoria, staging, tabela de carga). A única saída honesta é uma coluna sintética — e o schema já declara, nas milhares de FKs existentes, que convenção usa para nomear chave primária (`idkey`, `id`, etc.). Ninguém pediu essa informação ao schema ainda.

Esta change fecha as duas, como parte do fluxo padrão do `audit` — sem flag nova. Quando o terminal é interativo, toda tabela que chegar ao fim do caminho automático sem chave confirmada é resolvida **de uma vez, ao final da execução**: o comando relata quantas tabelas estão sem chave, quantas têm um candidato composto ainda não testado (as FKs de coluna única da própria tabela) e o que o schema já diz sobre como nomear uma chave primária, e pergunta uma única vez o que fazer — recomendar a chave composta onde houver candidato, recomendar uma coluna sintética nomeada pela convenção do próprio schema (nunca digitada — segue o mesmo padrão que qualquer outra tabela já usa), ou pular. Uma resposta não reconhecida é reportada como inválida e perguntada de novo, nunca tratada como "pular" por engano. Fora de terminal interativo, ou com `--no-probe-keys`, nada disso roda: o comportamento de hoje (achado sem sugestão) é preservado sem exceção.

## What Changes

- `internal/profile`: `Detect` passa a tabular o nome literal da coluna de PK em toda tabela de PK de coluna única, com os mesmos limiares de proporção que já regem prefixo/sufixo de referência. `NamingDetection` ganha `PrimaryKeyNames` e `SinglePKTables`. Cada `NamingEvidence` (as quatro convenções detectadas) passa a carregar `Examples` — poucos objetos concretos que sustentam a convenção, para que a citação seja verificável, não só uma porcentagem.
- `internal/model`: novo `SuggestionKind` — `synthesize_primary_key` — para a sugestão de coluna sintética. Reaproveita `Suggestion.Columns` para o nome da coluna nova e `Suggestion.Note` para a proveniência (convenção detectada, com exemplos).
- `internal/cli`: `Streams` ganha o campo `Interactive`, decidido por `StdStreams` a partir de stdin **e** stdout serem terminais reais — nunca de uma flag, para que o comportamento siga o ambiente de execução em vez de precisar ser lembrado. `audit` ganha `--profile` (mesma flag e mesmo default de `discover`), calcula a detecção de convenção a partir do catálogo já lido (nenhuma query nova), e, só quando `Interactive` e a sondagem de chave está ligada, junta toda tabela cujo achado de PK ausente não confirmou nada, mostra o resumo do cenário e pergunta uma única vez — a resposta se aplica a todas de uma vez, não tabela a tabela.
- `internal/report`: terminal, a seção DETECTED do `discover` e `suggested_keys.sql` passam a renderizar `synthesize_primary_key` e os exemplos por trás de cada convenção citada — DDL de duas etapas (criar a coluna, então promover), o mesmo padrão de lock já usado para chave confirmada por sondagem.

## Capabilities

### Modified Capabilities

- `structural-audit`: ganha a resolução interativa de chave ausente — convenção de nomenclatura de PK detectada do catálogo, candidato de chave composta a partir de FKs de coluna única não cobertas pela heurística existente, e a sugestão de coluna sintética.

### New Capabilities

Nenhuma. Estende `structural-audit`, que já cobre PK ausente desde a change anterior.

## Impact

`internal/cli` passa a ler stdin no meio da execução do `audit` — primeiro lugar do produto que faz isso. Gated por TTY em ambas as pontas (stdin e stdout), nunca por flag: um pipe, redirecionamento ou CI nunca vê um prompt, porque `Interactive` já sai falso antes de qualquer leitura ser tentada. `internal/profile` passa a ser dependência de `audit`, não só de `discover`. Nenhuma dependência nova no binário — a detecção de TTY já existe em `internal/cli/output.go` (`isTerminal`), sem biblioteca externa.

`schema_version` não incrementa: `Suggestion` já é aditivo desde a change anterior, e `synthesize_primary_key` é só um novo valor de um campo `string` já existente — o mesmo raciocínio que não incrementou a versão quando novos `FindingKind` foram adicionados.

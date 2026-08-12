## MODIFIED Requirements

### Requirement: O JSON é contrato público e sua forma é congelada

O documento JSON SHALL ser tratado como API pública a partir deste release. Seus campos de topo SHALL ser: `schema_version`, `tool`, `tool_version`, `generated_at`, `duration_ns`, `server_version`, `profile`, `naming_detection`, `schemas`, `candidates`, `discarded`, `findings` e `coverage`.

Em cada candidato, `child` e `parent` SHALL ser referências de chave: schema, tabela e **lista ordenada** de colunas. Chave de coluna única SHALL sair como lista de um elemento, e MUST NOT existir forma alternativa para o caso escalar. Um consumidor que trate a lista corretamente trata as duas aridades sem ramificação, o que é a razão de não haver duas formas.

Candidato descartado pelo limiar ou pelo pré-filtro SHALL residir em `discarded`, nunca em `candidates`. Um consumidor MUST NOT precisar inspecionar veredito ou score para saber se um candidato sobreviveu à triagem — a distinção é estrutural, como a que separa relação declarada de inferida.

Renomear ou remover qualquer campo, ou mudar o tipo de um campo existente, SHALL exigir incremento de `schema_version`. Acrescentar campo é compatível e MUST ainda assim passar por revisão explícita.

A forma de chave entra antes do primeiro release, e por isso `schema_version` permanece `"1"`. Incrementar para `"2"` antes de existir consumidor de `"1"` publicaria uma história que ninguém viveu; depois do release, a mesma mudança seria quebra de contrato.

É o arquivo que o `check --baseline` e ferramenta de terceiro vão consumir. Um contrato que muda em silêncio não é contrato.

#### Scenario: A forma é verificada por teste

- **WHEN** um campo do documento é renomeado, removido ou acrescentado sem atualização do contrato
- **THEN** o teste de contrato falha apontando o caminho de chave que divergiu

#### Scenario: `schema_version` sai em toda execução

- **WHEN** qualquer comando emite JSON
- **THEN** `schema_version` está presente e preenchido

#### Scenario: Descartado não se confunde com sobrevivente

- **WHEN** uma execução com exibição de descartados emite JSON
- **THEN** os descartados aparecem em `discarded` e nenhum deles aparece em `candidates`

#### Scenario: Coluna única sai como lista de um

- **WHEN** um candidato de coluna única é serializado
- **THEN** `child.columns` e `parent.columns` são listas de um elemento, e não existe campo escalar equivalente

#### Scenario: A ordem das colunas é a da chave

- **WHEN** um candidato de chave composta é serializado
- **THEN** as colunas do pai saem na ordem da chave primária e as do filho na posição correspondente

## ADDED Requirements

### Requirement: O JSON é contrato público e sua forma é congelada

O documento JSON SHALL ser tratado como API pública a partir deste release. Seus campos de topo SHALL ser: `schema_version`, `tool`, `tool_version`, `generated_at`, `duration_ns`, `server_version`, `profile`, `naming_detection`, `schemas`, `candidates`, `discarded`, `findings` e `coverage`.

Candidato descartado pelo limiar ou pelo pré-filtro SHALL residir em `discarded`, nunca em `candidates`. Um consumidor MUST NOT precisar inspecionar veredito ou score para saber se um candidato sobreviveu à triagem — a distinção é estrutural, como a que separa relação declarada de inferida.

Renomear ou remover qualquer campo, ou mudar o tipo de um campo existente, SHALL exigir incremento de `schema_version`. Acrescentar campo é compatível e MUST ainda assim passar por revisão explícita.

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

### Requirement: O JSON carrega o resultado inteiro, incluindo o que ficou de fora

O documento SHALL conter o que foi lido do catálogo, o que foi inferido com todos os sinais preservados, as métricas de validação de cada candidato, e o bloco de cobertura completo.

O bloco de cobertura MUST estar presente em todo documento, inclusive quando não houve nenhum achado. Consumidor programático não tem como distinguir "nada encontrado" de "nada analisado" sem ele.

#### Scenario: Sinais sobrevivem à serialização

- **WHEN** um candidato é serializado
- **THEN** todos os seus sinais aparecem com origem e peso, e seu score pode ser reconstruído a partir deles

#### Scenario: Cobertura acompanha o documento vazio

- **WHEN** uma execução não produz candidato nenhum
- **THEN** o documento ainda traz `coverage` preenchido

### Requirement: Nenhum valor de dado do usuário aparece no JSON, em nenhum cenário

Nenhum campo do documento MUST conter valor lido de tabela do usuário. Isso SHALL ser verificado por varredura automatizada sobre a saída de todos os cenários de fixture, e não por revisão.

A varredura SHALL cobrir também os campos de texto livre — motivo de veredito, detalhe de sinal, mensagem de erro — que são o caminho mais provável de vazamento acidental.

#### Scenario: Valor plantado não aparece em nenhuma fixture

- **WHEN** o binário roda contra cada fixture e a saída JSON é varrida pelos valores plantados
- **THEN** nenhuma ocorrência é encontrada

#### Scenario: Texto livre carrega só nome de objeto

- **WHEN** um candidato recebe motivo de veredito ou detalhe de sinal
- **THEN** o conteúdo é composto apenas de nome de objeto de catálogo e de condições, nunca de valor

### Requirement: JSON é o documento inteiro em stdout, sem diagnóstico

Em formato JSON, stdout SHALL conter exclusivamente o documento. Aviso, degradação e erro recuperável SHALL ir para stderr.

Uma linha de diagnóstico em stdout corrompe o documento para quem o consome programaticamente, que é o único público deste formato.

#### Scenario: Aviso não contamina o documento

- **WHEN** uma execução em formato JSON emite avisos de privilégio ou de estatística indisponível
- **THEN** stdout continua sendo um documento JSON válido e os avisos aparecem em stderr

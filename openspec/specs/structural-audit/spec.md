# structural-audit Specification

## Purpose
TBD - created by archiving change catalog-inspection. Update Purpose after archive.
## Requirements
### Requirement: O comando audit não depende de inferência

`pgfathom audit` SHALL reportar apenas achados derivados diretamente do catálogo, sem heurística de nome, sem pontuação e sem leitura de dado.

Isso torna o comando determinístico e imune a falso positivo: todo achado que ele emite é um fato do catálogo, não uma hipótese. É também o que permite executá-lo em banco onde a inferência não teria nada a dizer.

#### Scenario: Nenhum candidato inferido na saída

- **WHEN** `pgfathom audit` é executado
- **THEN** a saída não contém candidato, score nem veredito de inferência

### Requirement: Constraint declarada mas nunca validada é reportada

O comando SHALL reportar toda constraint de chave estrangeira cujo estado `convalidated` seja falso.

O achado SHALL identificar a constraint, a tabela e a estimativa de linhas da tabela filha, e SHALL explicar que a constraint bloqueia violações novas mas nunca verificou as linhas preexistentes.

#### Scenario: NOT VALID encontrada

- **WHEN** o schema contém uma FK criada com `NOT VALID` e nunca validada
- **THEN** o achado correspondente aparece na saída, identificando a constraint e sua tabela

#### Scenario: Constraint validada não gera achado

- **WHEN** todas as FKs do schema foram validadas
- **THEN** nenhum achado desse tipo é emitido

### Requirement: Chave estrangeira sem índice do lado filho é reportada

O comando SHALL reportar toda FK declarada sem índice utilizável na coluna filha, acompanhada da estimativa de linhas de ambas as tabelas, que é o que indica a gravidade.

Sem esse índice, todo `DELETE` na tabela pai vira varredura sequencial da filha.

#### Scenario: FK sem índice

- **WHEN** uma FK declarada não tem índice com a coluna filha em posição inicial
- **THEN** o achado aparece na saída com as estimativas de linha de pai e filha

### Requirement: Saída em terminal e em JSON

O comando SHALL suportar saída em tabela no terminal e em JSON, selecionável por flag.

A saída em terminal SHALL agrupar os achados por tipo e SHALL terminar com o bloco de cobertura. A saída em JSON SHALL seguir o contrato versionado do modelo.

Resultado vai para stdout; aviso, progresso e erro vão para stderr, sem exceção.

#### Scenario: JSON consumível

- **WHEN** `pgfathom audit --format json` é executado com stdout redirecionado
- **THEN** o arquivo contém JSON válido e nada além dele

#### Scenario: Cobertura sempre presente

- **WHEN** o comando termina, em qualquer formato
- **THEN** o bloco de cobertura consta da saída

#### Scenario: Ausência de achado é afirmativa

- **WHEN** nenhum achado é encontrado e todas as tabelas do escopo foram analisadas
- **THEN** a saída afirma que o escopo foi analisado e está limpo, em vez de apenas não listar nada

### Requirement: Nenhum valor de dado do usuário na saída

Nenhuma saída do comando, em qualquer formato ou nível de log, SHALL conter valor lido de tabela do usuário. O que sai são nomes de objeto de catálogo, contagens e proporções.

#### Scenario: Varredura da saída

- **WHEN** o comando é executado contra as fixtures de teste, que contêm valores reconhecíveis plantados
- **THEN** nenhum desses valores aparece na saída em terminal, no JSON ou no log

### Requirement: Código de saída reflete execução, não achados

O comando SHALL sair com zero quando a execução completa, mesmo tendo encontrado achados. Achado é o produto do comando, não uma falha dele.

Falha de conexão, de privilégio ou erro interno SHALL sair com o código de falha; erro de linha de comando, com o código de uso.

#### Scenario: Achados não alteram o código de saída

- **WHEN** o comando encontra achados e completa normalmente
- **THEN** o código de saída é zero

#### Scenario: Falha de conexão

- **WHEN** a conexão não pode ser estabelecida
- **THEN** o código de saída é o de falha e a mensagem vai para stderr

### Requirement: Tabela sem chave primária é reportada

O comando SHALL reportar toda tabela em escopo que não tenha chave primária, acompanhada da estimativa de linhas, que é o que indica a gravidade.

Sem PK a tabela não tem identidade de linha: replicação lógica não a cobre e todo `UPDATE`/`DELETE` por linha vira varredura. Quando existe um `UNIQUE` cujas colunas são todas `NOT NULL`, o achado SHALL oferecer a promoção dessa unique a PK, que é um caminho provado pelo catálogo e de custo baixo.

#### Scenario: Tabela sem PK com unique promovível

- **WHEN** uma tabela não tem PK mas tem uma constraint `UNIQUE` com todas as colunas `NOT NULL`
- **THEN** o achado aparece com a sugestão de promover essa unique a chave primária

#### Scenario: Tabela sem PK e sem unique promovível

- **WHEN** uma tabela não tem PK nem unique promovível
- **THEN** o achado aparece marcado como precisando de sondagem de chave, sem afirmar qual é a chave

#### Scenario: Tabela com PK não gera achado

- **WHEN** todas as tabelas do escopo têm chave primária
- **THEN** nenhum achado desse tipo é emitido

### Requirement: A chave sugerida é confirmada por contagem, nunca por estimativa

Quando não há unique promovível, o comando MAY sondar a unicidade de um conjunto de colunas para nomear a chave. A sondagem SHALL emitir apenas contagens — nenhum valor de coluna em struct, log, JSON ou erro. Unicidade SHALL ser confirmada apenas em varredura completa da tabela: modo amostrado nunca confirma chave, porque uma duplicata pode estar fora da amostra.

Um conjunto sondado que estoure o `statement_timeout` SHALL sair como `unverified` e a execução prossegue. Estimativa de `n_distinct` MAY priorizar o que sondar, mas nunca SHALL ser afirmada como chave.

#### Scenario: Chave composta confirmada por contagem

- **WHEN** a sondagem completa uma tabela cuja unicidade real é composta e as contagens provam `total = distinct` sem nulos
- **THEN** o achado nomeia a chave composta como confirmada

#### Scenario: Amostra não confirma chave

- **WHEN** a tabela é grande e só pôde ser lida por amostra
- **THEN** nenhuma chave sai como confirmada; a chave sai `unverified` e a tabela consta da cobertura

#### Scenario: Sondagem não vaza valor

- **WHEN** a sondagem roda contra fixtures com valores plantados
- **THEN** nenhum desses valores aparece na saída em terminal, no JSON ou no log

### Requirement: Coluna quente sem índice é reportada com o tipo de índice apropriado

O comando SHALL reportar toda coluna que apareça em predicado de junção ou de filtro no código real — view, função ou `pg_stat_statements` — com recorrência acima do limiar configurável e sem índice que a lidere. O achado SHALL recomendar o método de índice apropriado ao operador observado e ao tipo da coluna.

A recomendação SHALL ser `btree` por padrão, servindo igualdade e faixa. `GIN` SHALL ser recomendado para contenção em `jsonb`/array e para full-text; `GIN gin_trgm_ops` para `LIKE`/`ILIKE` de infixo quando `pg_trgm` estiver instalada; um método de vizinhança para coluna `vector` quando `pgvector` estiver instalada. Extensão ausente nunca vira erro nem recomendação impossível: degrada para `btree` com nota ou omite o achado.

#### Scenario: Coluna de junção sem índice

- **WHEN** uma view cruza repetidamente uma coluna que não lidera nenhum índice
- **THEN** o achado aparece recomendando `btree` sobre essa coluna

#### Scenario: Contenção em jsonb sem GIN

- **WHEN** uma função usa `@>` sobre uma coluna `jsonb` sem índice `GIN`
- **THEN** o achado recomenda `GIN` para essa coluna

#### Scenario: Infixo sem pg_trgm

- **WHEN** um predicado `LIKE '%x%'` incide sobre uma coluna sem índice e `pg_trgm` não está instalada
- **THEN** o achado recomenda `btree` com a nota de que o infixo pede `pg_trgm`, sem falhar

#### Scenario: FK sem índice não é duplicada

- **WHEN** uma coluna já é reportada por `fk_without_index`
- **THEN** ela não gera também um achado de coluna quente

### Requirement: A leitura de dado da sondagem é opcional e limitada por teto

A sondagem de chave SHALL rodar automaticamente apenas para tabelas abaixo de um teto de linhas estimadas, configurável. Tabelas acima do teto SHALL constar da cobertura como não sondadas, mantendo a oferta de promoção de unique quando houver. Uma flag SHALL desligar a leitura de dado por inteiro, devolvendo o comando ao comportamento catálogo-puro.

#### Scenario: Tabela grande não é sondada

- **WHEN** uma tabela sem PK excede o teto de tamanho
- **THEN** a sondagem não roda contra ela e a cobertura registra a tabela como não sondada

#### Scenario: Leitura de dado desligada

- **WHEN** o comando roda com a sondagem desligada
- **THEN** nenhuma transação de leitura de dado é aberta e os achados de PK saem apenas do catálogo

### Requirement: A resolução interativa de chave ausente é gated por terminal, nunca por flag

O comando SHALL só oferecer resolução interativa de chave ausente quando stdin e stdout forem ambos um terminal interativo. O comando SHALL NOT introduzir uma flag para ligar ou desligar esse comportamento por si só — ele segue o ambiente de execução. `--no-probe-keys` SHALL desligar a resolução interativa junto com a sondagem automática, porque ambas leem dado da tabela.

#### Scenario: Saída redirecionada nunca pausa

- **WHEN** o comando roda com stdout redirecionado para um arquivo ou pipe
- **THEN** nenhuma tabela sem chave confirmada dispara um prompt, e a saída é idêntica à que o comando produziria sem esta capability

#### Scenario: --no-probe-keys desliga tudo

- **WHEN** o comando roda com `--no-probe-keys`, mesmo em terminal interativo
- **THEN** nenhuma leitura de dado acontece, incluindo a resolução interativa

### Requirement: A resolução de chave ausente é uma decisão única por execução, não por tabela

Quando houver ao menos uma tabela sem chave confirmada e o terminal for interativo, o comando SHALL primeiro avaliar todo o catálogo e só então relatar, uma única vez, quantas tabelas estão pendentes, quantas têm um candidato composto ainda não testado, e o nome de primary key mais comum entre as tabelas do escopo que já têm uma — cada convenção citada acompanhada de exemplos concretos dos objetos que a sustentam. O comando SHALL perguntar no máximo uma vez por execução o que fazer, e a resposta SHALL se aplicar a todas as tabelas pendentes de uma vez, nunca tabela a tabela.

#### Scenario: Resumo antes da pergunta

- **WHEN** três tabelas ficam sem chave confirmada, duas delas com candidato composto
- **THEN** o comando relata as três tabelas, os dois candidatos compostos, e a convenção de nome de PK — antes de perguntar qualquer coisa

### Requirement: A coluna sintética é sempre nomeada pela convenção do schema, nunca digitada

Quando o operador escolher a recomendação de coluna sintética, o comando SHALL nomeá-la com o nome de primary key mais comum entre as tabelas do escopo que já têm uma chave de coluna única — o mesmo nome que qualquer outra tabela do schema já usa. O comando SHALL NOT aceitar um nome de coluna digitado livremente. Quando nenhum nome puder ser determinado a partir do schema, a opção de coluna sintética SHALL NOT ser oferecida.

#### Scenario: Coluna sintética segue a convenção

- **WHEN** o operador escolhe a recomendação de coluna sintética e o schema majoritariamente nomeia a chave primária de um jeito
- **THEN** toda tabela pendente resolvida por essa escolha ganha uma coluna sintética com esse nome, sem sondagem de dado

#### Scenario: Sem convenção, sem opção de coluna sintética

- **WHEN** nenhuma tabela do escopo tem chave primária de coluna única o suficiente para tabular uma convenção
- **THEN** o menu não oferece a recomendação de coluna sintética, só o candidato composto (quando existir) e pular

### Requirement: Composto e sintético são recomendações globais, aplicadas a toda tabela pendente

Ao escolher a recomendação de chave composta, o comando SHALL testar, por contagem completa, o candidato de cada tabela pendente que tiver um — nunca afirmado sem essa prova. Ao escolher a recomendação de coluna sintética, o comando SHALL aplicá-la a toda tabela pendente, independentemente de ela ter ou não um candidato composto.

#### Scenario: Chave composta confirmada pela escolha global

- **WHEN** o operador escolhe a recomendação de chave composta e a sondagem por contagem completa confirma unicidade para uma das tabelas pendentes
- **THEN** o achado dessa tabela é resolvido como chave composta confirmada, com o mesmo veredito `confirmed` que o caminho automático produz

#### Scenario: Pular não resolve nada

- **WHEN** o operador responde vazio ou o stdin fecha (EOF) antes de uma resposta válida
- **THEN** todo achado pendente permanece exatamente como o caminho automático o deixou, sem sugestão adicional

### Requirement: Uma resposta não reconhecida é reportada como inválida e perguntada de novo

O comando SHALL NOT tratar uma resposta que não corresponda a nenhuma opção oferecida como um pedido para pular. Ele SHALL informar que a resposta foi inválida e perguntar novamente, até receber uma resposta reconhecida ou o stdin fechar.

#### Scenario: Resposta inválida não pula silenciosamente

- **WHEN** o operador digita algo que não corresponde a nenhuma opção do menu
- **THEN** o comando informa que a resposta foi inválida e pergunta de novo, em vez de tratar a tabela como pulada

### Requirement: Coluna sintética nunca é afirmada como confirmada por dado

Uma sugestão de coluna sintética SHALL NOT carregar um veredito de sondagem: sua correção não depende de nenhum dado existente, só da criação da coluna. O artefato `.sql` gerado para ela SHALL declarar em duas etapas — criação da coluna, depois promoção a chave primária — e SHALL observar que a criação de uma coluna `GENERATED ALWAYS AS IDENTITY` já reescreve a tabela.

#### Scenario: Artefato de coluna sintética

- **WHEN** um achado tem uma sugestão de coluna sintética
- **THEN** `suggested_keys.sql` emite a criação da coluna e a promoção a chave primária em duas etapas comentadas, com a ressalva sobre reescrita da tabela


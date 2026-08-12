## Context

O `audit` foi desenhado na fase 2 como o comando que não erra: catálogo-puro, determinístico, imune a falso positivo, executável num banco onde a inferência não teria nada a dizer. Esta change amplia o que ele aponta sem abrir mão dessa identidade — com uma exceção deliberada e cercada: a sondagem de chave, que lê contagem de dado real.

A pergunta que governa cada decisão aqui é a mesma da fase 2: a ferramenta será apontada para o banco de produção de outra pessoa. Uma recomendação errada custa uma conversa; uma query que trava o servidor custa a adoção. Toda leitura de dado nesta change é opcional, limitada por teto e por `statement_timeout`, e degrada para "não sondado, registrado na cobertura" — nunca para trava.

Restrições herdadas, todas preservadas: read-only absoluto; nenhum valor de dado do usuário em qualquer saída (a sondagem emite só contagens); nenhuma afirmação sem evidência; silêncio nunca reportado como ausência de problema; nenhum falso positivo confirmado.

## Goals / Non-Goals

**Goals**

Apontar tabela sem PK e, quando o catálogo permitir, o caminho de promoção sem custo. Confirmar por contagem a chave — simples ou composta — quando não há unique promovível. Apontar coluna quente sem índice a partir do uso real que o extrator já lê, com o tipo de índice certo para o operador e o tipo da coluna. Manter o `audit` num comando só.

**Non-Goals**

Não reescrever o schema: a ferramenta gera `.sql` revisável, nunca executa DDL. Não recomendar índice por palpite de nome — só por uso observado no código. Não sondar unicidade de todo subconjunto de colunas: a explosão combinatória é cortada por heurística de catálogo. Não cobrir `GiST`, `SP-GiST`, `BRIN` nesta change — `btree`/`hash`/`GIN` e os métodos de extensão (`gin_trgm_ops`, `HNSW`) cobrem o que o extrator consegue justificar; o resto entra quando houver operador extraído que o exija.

## Decisions

### Avaliação da ideia — por que ela cabe, e onde ela quase não coube

A ideia é sólida porque o `audit` já é, na prática, um auditor de eficiência estrutural: seus dois achados atuais são exatamente isso. Somar PK ausente e índice ausente é continuar a mesma frase, não começar outra. E a matéria-prima já existe: o catálogo dá PK, unique, índice e tipo; o extrator da fase 6 dá o uso real das colunas. A change é mais montagem do que invenção.

Onde ela quase não coube: **"identificar a PK vendo colunas sem repetição"** colide de frente com a regra 5. O catálogo não sabe se uma coluna sem unique é única — só os dados sabem. As três saídas honestas eram: (a) só promover unique existente, que não cobre a tabela sem nenhum unique; (b) estimar por `pg_stats.n_distinct`, que é palpite e não pode ser afirmado; (c) provar por contagem contra os dados. Escolhida a (a) com fallback (c): promoção quando o catálogo basta, sondagem por contagem quando não. A (b) fica de fora porque estimativa afirmada como fato é o falso positivo que a regra 5 proíbe.

### A sondagem de chave mora em `internal/validate`, não em `internal/audit`

A arquitetura tem uma fronteira dura: `internal/validate` é a única camada que lê dado de tabela do usuário. A sondagem de unicidade lê dado — logo mora lá, ao lado da validação de contenção, reusando o mesmo `Beginner`, a mesma transação com `SET LOCAL statement_timeout`, a mesma disciplina de cancelamento por contexto. O `audit` continua catálogo-puro; quem costura catálogo, extrator e sondagem é o `cli`, exatamente como o `discover` já costura catálogo, inferência e validação.

### Unicidade só confirma em varredura completa

Amostra prova contenção alta, mas nunca unicidade: uma única duplicata fora da amostra derruba a chave. Então a sondagem de chave ignora o modo amostrado e roda `count(*) = count(DISTINCT (cols))` na tabela inteira. Tabela grande demais para caber no `statement_timeout` sai como `unverified` e entra na cobertura — nunca como chave confirmada. É a mesma assimetria da fase 5: recuperar menos e nunca errar vence.

A consulta é `SELECT count(*), count(DISTINCT (c1,...,cn)), count(*) FILTER (WHERE c1 IS NULL OR ...)`. Chave viável exige `total = distinct` e `nulls = 0`. Só três inteiros saem — nenhum valor de coluna, nem em struct, nem em log, nem em erro. A varredura de vazamento cobre essa saída.

### O teto de tamonho é o que mantém o `audit` barato por padrão

A sondagem roda automaticamente só para tabelas abaixo de um teto de linhas estimadas, conservador e configurável (`--probe-keys-max-rows`, padrão a fixar na implementação, ordem de milhões). Acima do teto, a tabela sai como "chave não sondada — grande demais" na cobertura, com a promoção de unique ainda oferecida se houver. `--no-probe-keys` desliga a leitura de dado inteira e devolve o `audit` ao catálogo-puro. A decisão pende para sondar por padrão porque o pedido era um fallback que acontece, não um que o usuário precisa lembrar de ligar — mas o teto garante que "por padrão" nunca significa "query pesada sem aviso".

### Candidatos a chave composta saem do catálogo, não da força bruta

Sondar todo subconjunto de colunas é fatorial e inviável. A heurística de nomeação de candidatos é catálogo-only e barata:
- Coluna única `NOT NULL` cujo `n_distinct` estimado se aproxima de `reltuples` → candidata a PK de coluna única.
- Conjunto de colunas de um índice não-único `NOT NULL` já existente → candidato a PK composta (o schema já agrupou essas colunas por algum motivo).
- Teto de sondagens por tabela, para que uma tabela com muitos índices não vire uma rajada de varreduras completas.

Estimativa aqui só **prioriza** o que sondar; a confirmação vem sempre da contagem completa. Estimativa errada custa uma sondagem à toa, nunca um falso positivo.

### Tipo de índice: `btree` é o padrão seguro, o resto exige o operador extraído

O extrator hoje só vê `=` entre duas colunas qualificadas. Para recomendar tipo, ele passa a emitir o operador de um predicado de coluna qualificada. O mapa:
- `=`, `<`, `>`, `BETWEEN`, `LIKE 'prefixo%'` → `btree`. `btree` serve igualdade e faixa; é a escolha correta e nunca errada.
- `LIKE '%infixo%'`, `ILIKE` → `GIN gin_trgm_ops`, **se `pg_trgm` presente**; senão `btree` com nota de que o infixo pede `pg_trgm`.
- `@>`, `<@`, `?`, `?|`, `?&` em `jsonb`/array → `GIN`.
- `@@` (full-text) → `GIN`.
- Coluna de tipo `vector` com operador de distância (`<->`, `<=>`, `<#>`) → `HNSW`, **se `pgvector` presente**.

`hash` fica de fora da recomendação confirmada: só ganha de `btree` em igualdade pura e carrega ressalvas que não valem a economia numa recomendação automática. Recomendar `btree` onde `hash` bastaria custa alguns bytes; recomendar `hash` onde a coluna também é ordenada custa um índice inútil. `btree` como padrão nunca é o achado errado.

### Oportunidades de extensão — pgvector, pg_trgm e vizinhas

O pedido incluía avaliar libs/extensões que agregam. A inferência de tipo é o gancho natural para elas, e o contrato de detecção já existe no projeto (`pg_stat_statements` mostra como: perguntar a `pg_extension`, degradar sem erro na ausência). Nesta change entram, gated por presença:
- **pg_trgm**: recomendação `GIN gin_trgm_ops` para `LIKE`/`ILIKE` de infixo — o padrão de busca textual mais comum em schema legado sem índice adequado.
- **pgvector**: recomendação `HNSW` (ou `ivfflat`, com nota de trade-off recall/velocidade) para coluna `vector` cruzada por operador de distância. Detectar coluna `vector` sem nenhum índice de vizinhança é um achado de alto valor onde a extensão já foi adotada mas o índice esqueceram.
- **btree_gin / btree_gist**: fora de escopo confirmado, anotadas como extensão futura para predicado misto (igualdade + contenção na mesma coluna), que o extrator ainda não distingue.

Regra transversal: extensão ausente nunca vira erro nem recomendação impossível. Vira `btree` com nota, ou omissão do achado quando não há alternativa honesta. Recomendar `CREATE EXTENSION` fica como sugestão no artefato `.sql`, comentada, nunca como pré-requisito silencioso.

### Artefatos `.sql` com o custo de lock à mostra

`suggested_indexes.sql` usa `CREATE INDEX CONCURRENTLY`, que não trava escrita — o único jeito honesto de sugerir índice em produção. `suggested_keys.sql` prefere `ADD CONSTRAINT ... USING INDEX` sobre uma unique já existente (lock curto) e, quando cria do zero, comenta o custo do lock de `ADD PRIMARY KEY` e sugere o caminho em duas etapas (criar unique concurrently, depois promover). O artefato é revisável e nunca executado pela ferramenta — a regra read-only não tem exceção.

## Risks / Trade-offs

- **Leitura de dado por padrão muda o contrato do `audit`.** Mitigado por teto conservador, `--no-probe-keys`, e cobertura explícita das tabelas não sondadas. É a decisão que mais merece o olhar do usuário na aprovação.
- **Recomendação de índice pode ter falso positivo** (coluna quente que o DBA decidiu não indexar por escrita pesada). Aceito pela mesma razão que a fase 2 aceitou para FK sem índice: falso positivo em recomendação custa conversa, falso negativo custa incidente. O limiar de recorrência corta o ruído.
- **Inferência de tipo depende do extrator degradável.** Operador não reconhecido → sem recomendação de tipo para aquela coluna, cai no `btree` padrão ou some. Nunca recomenda tipo errado.
- **PK composta confirmada por contagem é cara** na tabela grande. O teto e o `statement_timeout` a barram; ela sai `unverified`, não confirmada.

## Migration

Consumidor de JSON: `Finding` ganha `suggestion` opcional; ausente nos achados antigos, presente nos novos. `schema_version` incrementa. Nenhum campo existente muda de forma.

Operador: quem depende de `audit` barato em CI passa `--no-probe-keys` para manter o comportamento catálogo-puro anterior.

## Open Questions

- Valor exato do teto padrão de `--probe-keys-max-rows` — a fixar na implementação medindo contra as fixtures e o corpus da change de benchmark.
- Se a recomendação `ivfflat` vs `HNSW` para `pgvector` deve depender de tamanho estimado da tabela (ivfflat escala melhor em ingestão, HNSW em recall) — decidir na implementação do gerador de artefato.

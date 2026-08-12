## Why

O `audit` hoje emite dois achados de eficiência estrutural — constraint `NOT VALID` e FK sem índice do lado filho — e ambos provaram valor: saem direto do catálogo, custam quase nada e são imunes a falso positivo. Um banco legado carrega mais problemas de eficiência do que esses dois, e todos são invisíveis do mesmo jeito que os relacionamentos não declarados: o catálogo sabe o suficiente para apontar, ninguém olhou.

Dois deles são recorrentes em schema de gestão pública, que é o alvo declarado do projeto:

1. **Tabela sem chave primária.** Sem PK a tabela não tem identidade de linha, replicação lógica não a cobre, e todo `UPDATE`/`DELETE` por linha vira varredura. O catálogo diz que não há PK; muitas vezes diz também qual coluna já é `UNIQUE NOT NULL` e poderia ser promovida sem custo. Quando não diz, uma sondagem de contagem — sem nenhum valor saindo — prova a unicidade contra os dados e nomeia a chave, inclusive composta.

2. **Coluna quente sem índice.** O extrator de junção da fase 6 já lê view, função e `pg_stat_statements` e sabe quais colunas o código real cruza. Uma coluna que aparece repetidamente em predicado de junção mas não lidera nenhum índice é uma varredura sequencial que o schema pede em toda execução. O mesmo extrator, estendido para reconhecer o operador do predicado, permite recomendar não só o índice mas o **tipo** certo: `btree` para igualdade e faixa, `GIN` para contenção em `jsonb`/array e para `LIKE` de infixo com `pg_trgm`, `HNSW` para coluna `vector` com `pgvector`.

Isso mantém o `audit` fiel à sua identidade — apontar fato, não hipótese — e estende a única sondagem de dado que o produto já faz (validação por contagem) a um segundo uso que respeita as mesmas cinco regras invioláveis.

## What Changes

- `internal/sqlprobe`: o extrator passa a emitir, além da igualdade entre duas colunas qualificadas, **evidência de predicado** — uma referência qualificada, o operador e a classe do lado direito (literal, parâmetro, referência). Nova saída, sem tocar no contrato da evidência de junção existente. Degradação preservada: predicado não reconhecido é ignorado sem erro.
- `internal/catalog`: leitura da lista de extensões instaladas (`pg_extension`) para dentro do modelo, para que a recomendação de tipo de índice só sugira `GIN gin_trgm_ops`, `HNSW` etc. quando a extensão que os suporta existir.
- `internal/model`: novos `FindingKind` — `missing_primary_key` e `unindexed_hot_column`; struct `Suggestion` opcional no `Finding`, aditiva ao contrato JSON, carregando o tipo de sugestão, as colunas envolvidas, o método de índice recomendado e o veredito da sondagem de chave.
- `internal/audit`: dois geradores novos — tabela sem PK (com o caminho de promoção quando há `UNIQUE NOT NULL`) e coluna quente sem índice (com a inferência de tipo). Ambos permanecem catálogo-puros: nenhuma linha de dado é lida aqui.
- `internal/validate`: `ProbeUniqueness` — a sondagem de unicidade de um conjunto de colunas por **contagem apenas** (`count(*)`, `count(DISTINCT ...)`, `count(*) FILTER (WHERE ... IS NULL)`). É a **única** camada que lê dado de usuário, e a sondagem respeita isso. Unicidade só é confirmada em varredura completa: amostra nunca confirma, porque a duplicata pode estar fora dela.
- `internal/cli`: o comando `audit` passa a orquestrar a evidência de predicado (via `sqlprobe.Probe`) e a sondagem de chave (via `validate.ProbeUniqueness`), com teto de tamanho configurável e cobertura para as tabelas grandes demais para sondar.
- `internal/report`: títulos e renderização dos dois achados novos; artefatos `.sql` revisáveis — `suggested_keys.sql` (`ADD CONSTRAINT ... USING INDEX`, `ADD PRIMARY KEY`) e `suggested_indexes.sql` (`CREATE INDEX CONCURRENTLY ... USING <method>`). `schema_version` do JSON incrementado.

**A superfície do `audit` deixa de ser catálogo-puro por padrão.** A sondagem de chave lê dado (só contagens) para tabelas abaixo de um teto conservador, configurável, com as maiores registradas na cobertura como não sondadas. É a mudança de contrato desta change e está isolada atrás de flag e teto.

## Capabilities

### Modified Capabilities

- `structural-audit`: ganha os achados de tabela sem PK e de coluna quente sem índice, a recomendação de tipo de índice, e a sondagem opcional de chave por contagem — com a regra de que unicidade só é confirmada em varredura completa e o teto de tamanho aparece na cobertura.
- `usage-evidence`: o extrator passa a reconhecer o operador de um predicado de coluna qualificada, alimentando a recomendação de índice. Igualdade de junção segue como está.

### New Capabilities

Nenhuma. Tudo estende capability existente, para não fragmentar o `audit` em dois comandos.

## Impact

`internal/validate` deixa de ser exclusivo do `discover` e passa a ser consumido também pelo `audit`. A fronteira "validate é a única camada que lê dado de usuário" é preservada — a sondagem de chave mora lá, não no `audit`.

Nenhuma dependência nova no binário. `pg_trgm`, `pgvector` e `btree_gin` são detectadas via `pg_extension` e usadas só quando presentes; ausência degrada para `btree` com nota, nunca para erro — mesmo contrato de `pg_stat_statements`.

Fixtures novas em `testdata/`: tabela sem PK com coluna promovível, tabela sem PK com chave composta plantada, coluna quente de view sem índice, coluna `jsonb` com contenção, e — atrás de detecção de extensão — coluna `vector`. Todas plantam valor reconhecível para a varredura de vazamento, que passa a cobrir a saída da sondagem de contagem.

`schema_version` do JSON incrementa: os campos são aditivos, mas o incremento sinaliza a um consumidor que o `Finding` pode agora carregar `Suggestion`.

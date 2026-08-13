## 1. Modelo e catálogo

- [x] 1.1 Adicionar em `internal/model/finding.go` os `FindingKind` `missing_primary_key` e `unindexed_hot_column`
- [x] 1.2 Adicionar o struct `Suggestion` (tipo de sugestão, colunas, método de índice, veredito da sondagem) como campo opcional aditivo do `Finding`, com tags JSON `omitempty`
- [x] 1.3 Adicionar em `internal/model/schema.go` os helpers `HasPrimaryKey`, `PromotableUnique` (unique cujas colunas são todas `NOT NULL`) e `IsIndexedLeading` reusado
- [x] 1.4 Definir em `internal/model` a classe de operador de predicado e o mapa operador+tipo → método de índice (`btree`/`gin`/`gin_trgm_ops`/`hnsw`) — `hash` ficou fora, ver design.md
- [x] 1.5 Adicionar ao modelo a lista de extensões instaladas e o acessor de presença por nome
- [x] 1.6 Ler `pg_extension` em `internal/catalog`, registrando a lista no resultado; ausência de leitura degrada sem erro
- [x] 1.7 Incrementar `schema_version` do JSON e ajustar o teste de contrato

## 2. Extrator de predicado

- [x] 2.1 Estender `internal/sqlprobe/extract.go` para emitir evidência de predicado: referência qualificada + operador + classe do lado direito, sem alterar a extração de junção existente
- [x] 2.2 Reconhecer os operadores `=`, `<`, `>`, `LIKE`, `ILIKE`, `@>`, `<@`, `?`, `?|`, `?&`, `@@` e os de distância de `pgvector`, classificando cada um
- [x] 2.3 Distinguir `LIKE 'prefixo%'` de `LIKE '%infixo%'` pelo literal, quando o lado direito for string constante
- [x] 2.4 Resolver a referência de predicado contra o catálogo em `internal/sqlprobe/probe.go`, adicionando `PredicateEvidence` ao `Evidence` — dedup inclui o objeto de origem, ao contrário da junção, porque a recorrência entre objetos é o sinal de que a coluna quente precisa (ver design.md)
- [x] 2.5 Garantir que predicado não reconhecido é ignorado sem erro — teste com SQL malformado e com operador desconhecido

## 3. Achados de auditoria (catálogo-puro)

- [x] 3.1 Implementar em `internal/audit` o gerador de tabela sem PK, com o caminho de promoção quando há `PromotableUnique`
- [x] 3.2 Marcar no `Suggestion` da tabela sem PK e sem unique promovível que ela precisa de sondagem de chave
- [x] 3.3 Implementar o gerador de coluna quente sem índice a partir de `JoinEvidence` + `PredicateEvidence`, cortando por limiar de recorrência configurável
- [x] 3.4 Não duplicar o achado `fk_without_index` existente: coluna já coberta por ele não vira `unindexed_hot_column`
- [x] 3.5 Inferir o método de índice pelo operador extraído e pelo tipo da coluna, respeitando a presença de extensão; sem operador reconhecido, cair em `btree`; sem recomendação honesta (ex.: contenção em tipo sem classe de operador GIN padrão), omitir o achado
- [x] 3.6 Manter `internal/audit` sem nenhuma leitura de dado — teste que falha se o pacote importar `validate`, `db` ou `pgx`

## 4. Sondagem de chave (única camada que lê dado)

- [x] 4.1 Implementar `ProbeUniqueness` em `internal/validate`, reusando `Beginner`, transação e `SET LOCAL statement_timeout`
- [x] 4.2 Consulta de contagem `count(*)`, `count(DISTINCT (cols))`, `count(*) FILTER (WHERE ... IS NULL)` — só inteiros saem
- [x] 4.3 Confirmar unicidade apenas em varredura completa; a função nunca amostra (nenhum código de amostragem existe neste caminho)
- [x] 4.4 Estouro de `statement_timeout` marca a chave `unverified` e a execução segue
- [x] 4.5 Nomear candidatos a partir do catálogo em `internal/cli`: colunas de índice não-único `NOT NULL`, com fallback para cada coluna `NOT NULL` individual, e teto de sondagens por tabela — decisão registrada no design.md: sem estimativa de `n_distinct`, a seleção é só catálogo, e a confirmação vem sempre da contagem
- [x] 4.6 Cancelamento por contexto encerra a sondagem no servidor sem deixar conexão pendente (herdado do padrão de `runQuery`)

## 5. Orquestração no comando audit

- [x] 5.1 Chamar `sqlprobe.Probe` no `runAudit` para obter junção e predicado, registrando a disponibilidade de `pg_stat_statements` na cobertura
- [x] 5.2 Chamar `validate.ProbeUniqueness` para as tabelas sem PK e sem unique promovível, abaixo do teto de tamanho
- [x] 5.3 Costurar o veredito da sondagem no `Suggestion` do achado correspondente
- [x] 5.4 Adicionar as flags `--probe-keys-max-rows`, `--no-probe-keys` e `--recurrence-min`, com padrões conservadores
- [x] 5.5 Registrar na cobertura as tabelas não sondadas por excederem o teto (`Coverage.KeyProbesSkipped`, campo novo)

## 6. Relatório e artefatos

- [x] 6.1 Adicionar os títulos dos dois achados em `internal/report/terminal.go` e renderizar o `Suggestion`
- [x] 6.2 Serializar `Suggestion` no JSON sem vazar valor de dado — só nomes de objeto, tipos e contagens
- [x] 6.3 Gerar `suggested_keys.sql`: `ADD PRIMARY KEY USING INDEX` quando há unique (live), caminho em duas etapas comentado quando cria do zero
- [x] 6.4 Gerar `suggested_indexes.sql` com `CREATE INDEX CONCURRENTLY ... USING <method> (<col> [<opclass>])` comentado; `CREATE EXTENSION IF NOT EXISTS` comentado quando o método depende de uma
- [x] 6.5 Afirmar escopo limpo quando não há achado de eficiência, sem confundir com ausência de análise (herdado de `writeNothingFound`)

## 7. Fixtures e verificação

- [x] 7.1 Fixture `missing_pk_promotable`: tabela sem PK com `UNIQUE NOT NULL` promovível
- [x] 7.2 Fixture `missing_pk_composite`: tabela sem PK cuja unicidade real é composta, com índice não-único plantado sobre as colunas — e uma segunda tabela na mesma fixture com duplicata plantada, para provar que a sondagem nunca confirma por engano
- [x] 7.3 Fixture `hot_column_unindexed`: view + função que cruzam uma coluna sem índice, recorrente
- [x] 7.4 Fixture `jsonb_containment`: coluna `jsonb` com `@>` em view e função, sem `GIN`
- [x] 7.5 Fixture `pgvector_unindexed`, atrás de detecção de extensão (`testutil.TryPostgresImageDSN`, pula se a imagem `pgvector/pgvector:pg13` não sobe): coluna `vector` sem índice de vizinhança
- [x] 7.6 Plantar valor reconhecível em todas e estender a varredura de vazamento à saída da sondagem de contagem (`PlantedValues` em `internal/testutil/leak.go` + `TestEfficiencyFindingsNeverLeakUserData`)
- [x] 7.7 Teste unitário do mapa operador+tipo → método de índice, cobrindo presença e ausência de extensão (`internal/model/evidence_test.go`)
- [x] 7.8 Teste de integração provando que a sondagem de chave confirma um caso real e nunca confirma uma duplicata plantada, e sai `unverified` no estouro de timeout (`internal/validate/keyprobe_integration_test.go`) — escrito e verificado por `go vet -tags integration`; não pôde ser executado nesta sessão por falta de acesso ao daemon Docker no sandbox
- [x] 7.9 Teste provando que `internal/audit` não lê dado (`TestPackageNeverReadsData`)
- [x] 7.10 `golangci-lint run` zerado; `go test ./...` sem Docker e sem rede; binário sem dependência nova (`go.mod`/`go.sum` inalterados)
- [x] 7.11 Revisar densidade de comentário antes de fechar
- [ ] 7.12 `openspec validate efficiency-audit` — CLI `openspec` não disponível neste sandbox; estrutura da change conferida manualmente contra as changes arquivadas (`catalog-inspection`, `fk-candidate-inference`) como substituto

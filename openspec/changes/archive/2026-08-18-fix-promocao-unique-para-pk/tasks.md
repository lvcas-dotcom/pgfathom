## 1. Texto

- [x] 1.1 `internal/model/schema.go`: comentário de `PromotableUnique` para
      de prometer "without rewriting a row" via `USING INDEX` direto
- [x] 1.2 `internal/audit/audit.go`: comentário de `missingPrimaryKeys` e
      `Finding.Detail` param de dizer "at no cost"/zero-cost

## 2. DDL

- [x] 2.1 `internal/report/sql.go`: `writePromoteUnique` deriva nome de
      índice novo via `truncateIdent("ux_" + t.Name + "_" + strings.Join(...))`,
      mesma convenção dos vizinhos
- [x] 2.2 Emite três linhas comentadas: `CREATE UNIQUE INDEX CONCURRENTLY`,
      `ALTER TABLE ... ADD PRIMARY KEY USING INDEX`, `ALTER TABLE ... DROP
      CONSTRAINT` — nessa ordem
- [x] 2.3 Comentário explicativo atualizado (por que três passos, aviso de
      truncamento de nome se aplicável), removendo "scans nothing"

## 3. Teste

- [x] 3.1 `TestSuggestedKeysArtifactPromotesLiveUnique` reescrito: extrai as
      três linhas comentadas, roda cada uma em `conn.Exec` separado (nunca
      concatenadas), mantém a verificação final de `pg_constraint`
- [x] 3.2 Não previsto no plano original: `internal/report/sql_test.go` tinha
      dois testes (`TestSuggestedKeysPromotesExistingUnique`,
      `TestSuggestedKeysConfirmedCompositeUsesTwoStepCommented`) que
      codificavam o próprio bug como requisito — um exigia explicitamente
      `ADD PRIMARY KEY USING INDEX "cadastro_cpf_key"` sem comentário, o
      outro tinha uma exceção nomeada só pra não reclamar dessa linha
      descomentada. Nenhum dos dois roda contra Postgres de verdade (são
      testes de string sobre o artefato gerado), por isso nunca pegaram o
      erro que só a suíte de integração alcança. Reescritos:
      `TestSuggestedKeysPromotesExistingUniqueViaThreeStepCommented` agora
      exige as três linhas comentadas; a exceção no teste do composto foi
      removida.

## 4. Validação

- [x] 4.1 `go build ./...` e `go build -tags=integration ./...` — limpos
- [x] 4.2 `make test` (suíte sem Docker) — verde, incluindo os dois testes
      corrigidos em `internal/report`
- [x] 4.3 `go vet ./...` e `go vet -tags=integration ./...` — limpos
- [x] 4.4 `make test-integration` — rodado em 2026-08-18 numa máquina com
      Docker. `TestSuggestedKeysArtifactPromotesLiveUnique` passa contra
      Postgres de verdade, que é o teste que motivou esta change, e a suíte
      de integração completa fica verde

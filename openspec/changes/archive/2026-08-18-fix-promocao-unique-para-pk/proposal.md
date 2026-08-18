## Why

`TestSuggestedKeysArtifactPromotesLiveUnique` (`internal/cli/audit_efficiency_integration_test.go`)
falha contra Postgres de verdade: a DDL que `writePromoteUnique` gera pra
promover uma `UNIQUE` constraint a chave primária não roda.

```
ERROR: index "cadastro_pessoa_cpf_key" is already associated with a
constraint (SQLSTATE 55000)
```

`ALTER TABLE ... ADD PRIMARY KEY USING INDEX <nome>` só aceita índice
autônomo — nunca um já associado a uma constraint. O índice de toda `UNIQUE`
declarada já nasce associado a ela, então esse caminho nunca funcionou contra
um banco real. Bug pré-existente, já em `main` antes de qualquer trabalho
desta sessão — achado rodando `make test-integration`, não relacionado à
feature de similaridade lexical.

`openspec/specs/structural-audit/spec.md:96` já diz o requisito certo — "um
caminho provado pelo catálogo e de custo baixo" — nunca prometeu custo zero.
A implementação é que prometia mais do que o spec ("without rewriting a
row", "at no cost", "scans nothing") em quatro lugares, e por isso escolheu
o único mecanismo que o Postgres rejeita para esse caso.

## What Changes

- `writePromoteUnique` (`internal/report/sql.go`) passa a gerar DDL de três
  passos, comentada — `CREATE UNIQUE INDEX CONCURRENTLY` (índice novo,
  autônomo) → `ADD PRIMARY KEY USING INDEX` (esse sim aceito, porque o
  índice é novo) → `DROP CONSTRAINT` da `UNIQUE` antiga. Mesmo padrão que os
  dois vizinhos do arquivo (`writeConfirmedPrimaryKey`,
  `writeSyntheticPrimaryKey`) já usam para `CREATE INDEX CONCURRENTLY`,
  reaproveitando os mesmos helpers de nomenclatura e truncamento.
- Texto corrigido em quatro lugares (`internal/model/schema.go`,
  `internal/report/sql.go` — comentário de código e comentário dentro do SQL
  gerado —, `internal/audit/audit.go`) para parar de prometer custo zero,
  mantendo a promessa que continua verdadeira: o catálogo já prova a chave,
  sem sondagem de dado.
- `TestSuggestedKeysArtifactPromotesLiveUnique` reescrito para rodar as três
  linhas comentadas em sequência contra o banco, em vez de extrair "linha
  executável" (que passa a não existir mais para este achado).

## Capabilities

Nenhuma mudança de requisito — `structural-audit` já promete "custo baixo",
não "custo zero". Esta é correção de implementação contra um spec que já
estava certo; `skip_specs: true` no archive.

## Impact

- Comportamento visível: o artefato `suggested_keys.sql` para este achado
  passa a vir comentado (exige revisão manual antes de rodar), igual aos
  outros dois caminhos de promoção de PK do mesmo arquivo. Hoje é o único
  que roda direto — e é por isso que quebra.
- Sem dependência nova, sem mudança de contrato em `model.Suggestion`/
  `model.Finding`.
- Sem mudança em `internal/validate`, `internal/stats`.

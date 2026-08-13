## 1. Modelo e pesos

- [x] 1.1 `internal/model/candidate.go`: adicionar `SigNameSimilarity` ao bloco de sinais
      baseados em nome
- [x] 1.2 `internal/infer/similarity.go` (novo): `TrigramSimilarity(a, b string) float64`,
      coeficiente de Dice sobre trigrama com padding de borda, case-insensitive. Exportada
      (não `trigramSimilarity`) para seguir o padrão já em uso no pacote — `generate_test.go`
      e `score_test.go` só testam por fora (`package infer_test`), nunca white-box.
- [x] 1.3 `internal/infer/similarity_test.go` (novo): idênticas → 1.0, sem sobreposição → 0.0,
      string vazia → 0.0, os pares reais do corpus com o valor calculado documentado.
      Achado na execução real: `tptramite`/`tramitetipo` = 0.545, `operador`/
      `operadorbasecalculo` = 0.552, `atorevogacao`/`ato` = 0.353 — todos abaixo do limiar de
      0.65 planejado antes de medir. Ver 4 e `design.md` para a correção.
- [x] 1.4 `internal/infer/score.go`: `weightNameSimilarityMax` (0.12), `DefaultMinNameSimilarity`
      (0.30, recalibrado — ver acima), `nameSimilarityWeight(score float64) float64`

## 2. Geração

- [x] 2.1 `internal/infer/generate.go`: `Options.MinNameSimilarity` + accessor
      `minNameSimilarity()`, mesmo padrão de `minScore()`/`smallTableRows()`
- [x] 2.2 `Generate`: `flattenTables` monta lista plana de tabelas do escopo, ao lado do
      índice existente
- [x] 2.3 Extraído `resolveKeyTarget` (filtro de aridade/chave, antes inline em `generateFor`)
      para reuso pelas duas vias
- [x] 2.4 `generateFor`: via de fallback por similaridade (`resolveBySimilarity`), ativada só
      quando `index[entity]` vem vazio. `finalizeMatches` unifica o restante do pipeline
      (compatibilidade de tipo, ambiguidade, montagem de sinais) para as duas vias.
- [x] 2.5 `buildSignals`: assinatura passa a receber o sinal de nome já pronto
      (`nameSignalFromOrigin`/`nameSignalFromSimilarity`), decisão exato/normalizado saiu da
      função para o chamador da via por afixo

## 3. Testes de não-regressão e do caso novo

- [x] 3.1 `internal/infer/generate_similarity_test.go`
      (`TestAffixMatchSuppressesSimilarityFallback`): casamento por afixo continua idêntico
      ao atual, via de similaridade nunca aciona quando o afixo já resolveu
- [x] 3.2 `TestSimilarityFallbackGeneratesWhenAffixFindsNothing`: candidato com
      `SigNameSimilarity`, sem `SigExactName`/`SigNormalizedName`
- [x] 3.3 `TestSimilarityBelowCutoffGeneratesNothing`: abaixo do limiar → nenhum candidato
- [x] 3.4 `TestSimilarityFallbackHandlesAmbiguity`: duas tabelas cruzam o limiar para a mesma
      coluna → candidato para as duas, ambas com `SigAmbiguousTarget`
- [x] 3.5 `TestSimilarityFallbackReproducesCorpusMiss`: reprodução do padrão
      `atotramite.tptramite_idkey → tramitetipo` em formato sintético — usa o sufixo `_id`
      reconhecido pelo perfil embarcado em vez de `_idkey`, porque `_idkey` só existe via
      detecção de nomenclatura por schema (fora do escopo de um teste de unidade sobre o
      perfil embarcado), não no perfil `pt-br` estático

## 4. Validação

- [x] 4.1 `make test` — suíte inteira, todos os pacotes, sem falha
- [x] 4.2 `make lint` indisponível neste ambiente (`golangci-lint` não instalado) — rodado
      `go vet ./...` como alternativa, limpo. Lint completo fica pendente de ambiente com a
      ferramenta instalada.
- [x] 4.3 `make cover` (escopo `internal/infer`, `go test -coverprofile`) — 90% no pacote, todo
      código novo (`flattenTables`, `resolveByAffix`, `resolveBySimilarity`,
      `finalizeMatches`, `nameSignalFromOrigin`, `nameSignalFromSimilarity`, `buildSignals`,
      `TrigramSimilarity`, `nameSimilarityWeight`) em 100%. As duas exceções
      (`resolveKeyTarget` 71.4%, branch de coluna de PK ausente; `generateFor` 88.9%) são
      ramos defensivos que já existiam sem cobertura antes desta change, preservados tal como
      estavam — não uma queda introduzida aqui. Comparação direta antes/depois via `git stash` não
      foi possível (hook de proteção de git bloqueia `stash` fora de `/git-commit`/`/git-merge`).

## 5. Encaminhamento (fora desta entrega)

- [ ] 5.1 `make benchmark` contra o corpus público, para medir custo real e efeito em recall
      antes de qualquer recalibração dos limiares de similaridade e de peso — não bloqueia
      esta change (ambiente pode não ter Docker disponível)

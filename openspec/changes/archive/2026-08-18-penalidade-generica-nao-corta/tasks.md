## 1. Score

- [x] 1.1 `internal/infer/score.go`: extrair `saturate` de `score`, dono único da
      regra de faixa, para que score reportado e score de corte continuem
      comparáveis contra o mesmo limiar
- [x] 1.2 `internal/infer/score.go`: `cutScore` soma os sinais do candidato
      exceto `SigGenericDomain` e satura por `saturate`, com o comentário
      explicando por que este sinal e nenhum outro

## 2. Corte

- [x] 2.1 `internal/infer/generate.go`: `finalize` compara `cutScore(c)` com o
      limiar em vez de `MetaScore`; `MetaScore` fica intacto, então a ordenação
      não muda

## 3. Teste

- [x] 3.1 `TestGenericPenaltyRanksButDoesNotCut`: alvo no **plural**, para bater
      no caminho de casamento normalizado que é onde o bug morde; verifica
      também que o `MetaScore` reportado ainda está abaixo do limiar, senão o
      teste passaria vazio no dia em que os pesos mudarem
- [x] 3.2 `TestGenericPenaltyStillSortsLast`: a isenção do corte não isenta da
      ordenação
- [x] 3.3 `TestGenericPenaltyDoesNotRescueAWeakCandidate`: quem já estava abaixo
      do limiar sem a penalidade continua descartado, com motivo
- [x] 3.4 `TestAmbiguousPenaltyStillCuts`: a isenção é de um sinal só; alvo
      ambíguo continua cortando
- [x] 3.5 Não há teste para "o score de corte satura pela mesma regra": a
      propriedade não é observável pela superfície pública — remover uma
      penalidade só pode aumentar a soma, e o teto de 1 leva à mesma decisão de
      corte com ou sem saturação. Toda a suíte do pacote é black-box
      (`package infer_test`), e abrir um arquivo white-box só para isso seria
      teste de fachada. O cenário saiu do spec delta em vez de ganhar um teste
      que não prova nada; o dono único da regra continua garantido por
      `saturate`, e pelo requisito de recomposição que já existia
- [x] 3.6 Helpers `survivor` e `discarded` em `generate_test.go`: `find` procura
      nas duas listas, que é o que a maioria dos testes quer e exatamente o que
      um teste sobre o limiar não pode fazer
- [x] 3.7 Remover o `MinScore = 0.01` de `TestGenericNameWithSmallTargetIsPenalized`,
      que era contorno do bug e escondia que o caminho exato sobrevive por
      aritmética

## 4. Validação

- [x] 4.1 `go build ./...`, `go build -tags=integration ./...`,
      `go build -tags=benchmark ./...` — limpos
- [x] 4.2 `go test ./...` — verde
- [x] 4.3 `go test -tags integration ./...` — verde
- [x] 4.4 `golangci-lint run` sob as três tags — sem achado
- [x] 4.5 Os testes novos falham sem o fix (`finalize` revertido para
      `MetaScore`): `TestGenericPenaltyRanksButDoesNotCut` e
      `TestGenericPenaltyStillSortsLast` ficam vermelhos
- [x] 4.6 Medido contra Postgres real: schema em inglês com tabelas no plural,
      perfil `en`, sai de 4 para 5 confirmados, e os dois genéricos aparecem no
      fim da lista
- [x] 4.7 `docs/DEMO.md` reproduzido: a saída publicada no README não muda
- [x] 4.8 `make benchmark` rodado: recall e contagem de candidatos idênticos aos
      publicados, porque o corpus é DDL sem linha e a penalidade nunca é emitida
      lá — registrado na proposal como limite da medição, não como custo zero

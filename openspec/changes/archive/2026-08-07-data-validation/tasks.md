## 1. Camada de validação

- [x] 1.1 Criar `internal/validate` com opções: modo completo ou amostrado, alvo de linhas, teto de tempo por validação, limite de concorrência e limiares de veredito, com padrões declarados
- [x] 1.2 Montar a query de agregação por valor distinto com identificadores citados, retornando as cinco contagens numa passada
- [x] 1.3 Seleção de modo por candidato a partir de `reltuples`: leitura direta quando cabe no alvo, `BERNOULLI` em tabela pequena, `SYSTEM` no resto, com a fração calculada do alvo
- [x] 1.4 Executar cada validação em transação somente-leitura com `SET LOCAL statement_timeout`
- [x] 1.5 Detectar estouro de teto pelo SQLSTATE de cancelamento e distinguí-lo de erro real
- [x] 1.6 Rodar as validações sob `errgroup.SetLimit` com o limite da sessão, propagando o contexto
- [x] 1.7 Atribuir vereditos na ordem da spec: fraca por evidência insuficiente, confirmada só em modo completo, quebrada com órfãos, rejeitada, zona morta como fraca com motivo

## 2. Integração no discover

- [x] 2.1 Flags `--full` e `--sample-rows`, com amostrado como padrão
- [x] 2.2 Validar apenas os candidatos sobreviventes ao limiar e ao pré-filtro
- [x] 2.3 Preencher `CandidatesValidated` e `CandidatesTimedOut` na cobertura, fechando a conta do funil
- [x] 2.4 Substituir o aviso de "candidates only" pela declaração do modo executado, com o alerta de amostragem quando aplicável
- [x] 2.5 Renderizar veredito e métricas de validação por candidato no terminal, e conferir que o JSON já os carrega pelo modelo

## 3. Verificação

- [x] 3.1 Fixture `validation` com: relação íntegra confirmável, órfãos plantados em contagem conhecida, contenção baixa para rejeição, zona morta, coluna de valor único, filha vazia e valores plantados para a varredura
- [x] 3.2 Teste de integração: modo completo produz confirmada, quebrada com contagens exatas, rejeitada, e fraca nos casos de evidência insuficiente
- [x] 3.3 Teste de integração: modo amostrado nunca produz confirmada, e órfão amostrado ainda produz quebrada
- [x] 3.4 Teste de integração: teto de tempo estoura um candidato, os demais completam, cobertura conta o estouro
- [x] 3.5 Teste de nome exótico: tabela e coluna com maiúsculas e espaço validam corretamente
- [x] 3.6 Teste unitário da atribuição de veredito por tabela de casos, sem banco
- [x] 3.7 Varredura de vazamento ponta a ponta cobrindo a saída do `discover` com validação ligada, terminal e JSON
- [x] 3.8 Rodar `golangci-lint run` e zerar os apontamentos
- [x] 3.9 Confirmar que `go test ./...` segue sem Docker e sem rede
- [x] 3.10 Rodar `openspec validate data-validation` e corrigir o que apontar

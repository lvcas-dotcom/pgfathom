## 1. Detecção de convenção de nome de PK

- [x] 1.1 Adicionar `PrimaryKeyNames []NamingEvidence` e `SinglePKTables int` a `model.NamingDetection`
- [x] 1.2 Em `internal/profile/detect.go`, tabular o nome literal da coluna em toda tabela com `HasSingleColumnPK()`, ranqueado pelos mesmos limiares de proporção de `minRefAffixShare`/`minRefAffixCount` (limiares próprios `minPKNameShare`/`minPKNameCount`, mesmo valor)
- [x] 1.3 Teste unitário: schema com convenção clara de nome de PK produz `PrimaryKeyNames` corretos; schema sem PK nenhuma produz detecção vazia sem dividir por zero

## 2. Modelo

- [x] 2.1 Adicionar `SuggestSyntheticPrimaryKey SuggestionKind = "synthesize_primary_key"` em `internal/model/finding.go`, documentando que `Columns` carrega o nome da coluna nova e `KeyProbe` fica vazio (correção não depende de dado)

## 3. Streams interativo

- [x] 3.1 Adicionar campo exportado `Interactive bool` a `cli.Streams`
- [x] 3.2 `StdStreams` calcula `Interactive` a partir de `isTerminal(os.Stdin) && isTerminal(os.Stdout)`
- [x] 3.3 Teste: `Streams` construído por literal em teste (`In`/`Out` como `bytes.Buffer`) tem `Interactive` falso por padrão, sem exigir que todo teste existente passe o campo

## 4. Resolução interativa no comando audit

Revisado depois da primeira implementação: a versão original pausava tabela a tabela e aceitava um nome digitado livremente para a coluna sintética. Corrigido para uma decisão única por execução, com o nome sempre vindo da convenção detectada — ver design.md.

- [x] 4.1 Adicionar flag `--profile` a `audit`, mesmo default e mesma ajuda de `discover`
- [x] 4.2 Carregar o profile e chamar `naming.Detect(cat.Schemas)` logo após `probeMissingKeys`, sem query nova
- [x] 4.3 Função pura `fkKeyCandidate(t model.Table, alreadyTried [][]string) ([]string, bool)`: retorna as colunas de toda FK de coluna única da tabela, quando houver duas ou mais e o conjunto ainda não estiver em `alreadyTried`
- [x] 4.4 Função pura `resolvePKName(evidence []model.NamingEvidence) string`: nome do topo do ranking (já ordenado por `profile.Detect`), ou `""` quando não há nenhum — sem limiar de confiança, sem essa distinção fazer mais sentido numa decisão global
- [x] 4.5 Orquestração `resolveUnconfirmedKeys`: chamada só quando `streams.Interactive && !opts.noProbeKeys` (gate no chamador, em `runAudit`); primeiro junta todo achado de PK ausente sem `KeyProbe == Confirmed` num `[]unresolvedKey`, sem perguntar nada; só depois imprime o resumo do cenário inteiro e pergunta uma vez
- [x] 4.6 Função pura de parsing da resposta — `a` escolhe a recomendação composta (só quando algum candidato existe), `b` escolhe a sintética (só quando há convenção), vazio pula; qualquer outra coisa devolve `ok == false`, sinal pro chamador reperguntar — testável sem I/O real
- [x] 4.7 `promptKeyResolution` reconstrói o prompt e lê de novo enquanto a resposta não for reconhecida, imprimindo "invalid answer" a cada tentativa inválida; EOF já resolve como pular porque `ReadString` devolve linha vazia junto com `io.EOF`, sem tratamento especial
- [x] 4.8 Escolha composta invoca `validate.ProbeUniqueness` na combinação de `fkKeyCandidate` de cada tabela pendente que tiver uma, aplica o resultado com `applyKeyProbeResults` (reuso do existente)
- [x] 4.9 Escolha sintética monta `model.Suggestion{Kind: SuggestSyntheticPrimaryKey, Columns: [nome da convenção], Note: proveniência com exemplos}` para toda tabela pendente, sem sondagem
- [x] 4.10 Sem terminal interativo, ou com `--no-probe-keys`: nenhuma linha de `streams.Err`/`streams.In` é tocada; saída idêntica à da change anterior — coberto por `TestNonInteractiveNeverPrompts` (integration) e pelo default de `Streams.Interactive` (unit)

## 5. Relatório

- [x] 5.1 `formatSuggestion` em `internal/report/terminal.go` renderiza `synthesize_primary_key`
- [x] 5.2 `suggestedKeysFile` em `internal/report/sql.go` ganha `writeSyntheticPrimaryKey`: cria a coluna, nota o custo de reescrita, promove em duas etapas como `writeConfirmedPrimaryKey`
- [x] 5.3 Serialização JSON do novo `SuggestionKind` coberta pelo teste de contrato existente (`json_contract.golden` regenerado para os dois campos novos de `naming_detection`)

## 6. Verificação

- [x] 6.1 `go test ./...` sem Docker e sem rede — verde
- [x] 6.2 Teste de integração cobrindo o caminho de chave composta via FKs, coluna sintética digitada, resposta vazia e a regressão não-interativa (`internal/cli/audit_interactive_integration_test.go`, fixture nova `missing_pk_fk_bridge.sql`) — escrito e verificado por `go vet -tags integration`; não pôde ser executado nesta sessão por falta de acesso ao daemon Docker no sandbox, mesma limitação registrada em `2026-08-12-efficiency-audit`
- [ ] 6.3 `golangci-lint run` zerado — binário não disponível neste sandbox; `go vet ./...` e `gofmt -l .` limpos como substituto parcial
- [x] 6.4 Revisar densidade de comentário antes de fechar

## 7. Citação de exemplos na convenção detectada (adendo)

- [x] 7.1 `model.NamingEvidence` ganha `Examples []string`, capado em `model.MaxNamingExamples` (3)
- [x] 7.2 `internal/profile/detect.go`: as quatro contagens (sufixo/prefixo de referência, prefixo de tabela, nome de PK) passam a acumular exemplos junto — capados na acumulação, não recortados depois
- [x] 7.3 `internal/cli/audit.go`: a nota de convenção aplicada sozinha e a linha de convenção fraca no prompt interativo citam os exemplos (`exampleSuffix`)
- [x] 7.4 `internal/report/discover.go`: a seção DETECTED do `discover` cita os mesmos exemplos por linha
- [x] 7.5 Contrato JSON, goldens e testes unitários/de renderização atualizados

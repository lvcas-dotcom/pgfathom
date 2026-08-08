## 1. Catálogo

- [x] 1.1 Consulta de schemas visíveis em `internal/catalog/queries.go`, filtrando sistema e exigindo `USAGE`
- [x] 1.2 Tipos `ScopeOptions` e `Scope` em `internal/catalog`, com escopo, excluídos e não analisados
- [x] 1.3 `ResolveScope` lendo o catálogo, aplicando exclusão por glob e falhando com escopo vazio
- [x] 1.4 Seleção e exclusão extraídas para função pura, testável sem banco
- [x] 1.5 `Options` aceita o escopo resolvido, preservando os chamadores que passam a lista direta

## 2. Modelo

- [x] 2.1 Dimensão de schema em `model.Coverage`: total, analisados, não analisados e excluídos
- [x] 2.2 `Complete()` passa a exigir que nenhum schema visível tenha ficado fora do escopo

## 3. CLI

- [x] 3.1 Flags `--all-schemas` e `--exclude-schema` nos dois comandos que leem catálogo
- [x] 3.2 `--schema` junto de `--all-schemas` é erro de uso, detectado pelo estado de alteração da flag
- [x] 3.3 `connect` resolve o escopo e o entrega aos comandos, mantendo o aviso de privilégio de escrita
- [x] 3.4 Escopo vazio sai como erro de uso, com mensagem distinguindo lista vazia de exclusão total

## 4. Relatório

- [x] 4.1 Bloco de cobertura declara a contagem de schemas quando há mais de um visível
- [x] 4.2 Schemas fora do escopo listados com realce e com o ponteiro para `--all-schemas`
- [x] 4.3 Schemas excluídos por filtro em linha própria, distinta dos não analisados
- [x] 4.4 Banco de schema único não ganha linha nenhuma

## 5. Documentação

- [x] 5.1 Seção de escopo no README, em inglês, com as quatro flags e o comportamento de cobertura
- [x] 5.2 Flags novas na tabela de `docs/PGFATHOM.md`, com a nota de por que o padrão continua `public`

## 6. Verificação

- [x] 6.1 Teste unitário da resolução de escopo: lista explícita, escopo total, exclusão, complemento e ordenação
- [x] 6.2 Teste unitário de escopo vazio por exclusão total e por lista vazia
- [x] 6.3 Teste de erro de uso das flags mutuamente exclusivas, nos dois comandos
- [x] 6.4 Golden de terminal com schema fora do escopo, provando o ponteiro para `--all-schemas`
- [x] 6.5 Golden do contrato JSON regravado com os quatro caminhos novos, sem mover `schema_version`
- [x] 6.6 `gofmt`, `go vet`, `go test ./...` e `golangci-lint` sem apontamento
- [x] 6.7 `go build -tags integration ./...` provando que os testes de integração ainda compilam — a suíte de integração em si não foi executada: sem Docker no ambiente de desenvolvimento
- [x] 6.8 `openspec validate schema-scope --strict` sem erro

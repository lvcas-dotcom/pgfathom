# pgfathom — contexto do projeto

Ferramenta de linha de comando em Go para sondagem de schema PostgreSQL. Descobre e valida relacionamentos que existem nos dados mas nunca foram declarados no catálogo.

A especificação completa do produto está em [`docs/PGFATHOM.md`](../docs/PGFATHOM.md). Este arquivo é o resumo operacional para quem for implementar. Em caso de divergência, `docs/PGFATHOM.md` vence.

## Regras invioláveis

Violação de qualquer uma destas é bug de severidade máxima, não questão de estilo. Toda change precisa ser avaliada contra elas.

1. **Read-only absoluto.** Nenhum código pode emitir statement que altere o banco analisado. A ferramenta gera `.sql` para o usuário revisar e executar.
2. **Dado do usuário nunca sai.** A ferramenta lê valores para comparar chaves. O que sai são contagens, proporções e nomes de objetos. Nenhum valor de tabela pode chegar a saída, log, JSON ou mensagem de erro. Atenção especial a `pg_stats`: `most_common_vals` e `histogram_bounds` **são** dados do usuário.
3. **Nenhuma afirmação sem evidência.** Toda inferência sai com veredito e a métrica que o sustenta.
4. **Silêncio nunca é ausência de problema.** Tabela pulada por privilégio, candidato que estourou timeout, schema não coberto — tudo aparece no bloco de cobertura.
5. **Nenhum falso positivo confirmado.** Falso negativo é aceitável. Na dúvida entre recuperar mais e nunca errar, nunca errar vence.

## Stack

| | |
|---|---|
| Linguagem | Go, piso 1.25, build na estável corrente |
| Driver | `github.com/jackc/pgx/v5` + `pgxpool` |
| CLI | `github.com/spf13/cobra` |
| Config | `github.com/pelletier/go-toml/v2` |
| Concorrência | `golang.org/x/sync/errgroup` com `SetLimit` |
| Log | `log/slog` (stdlib) |
| Tabela no terminal | `text/tabwriter` (stdlib) |
| Guia interativo | `bubbletea` + `lipgloss`, só no subcomando `setup` |
| Teste | `testing` + `github.com/google/go-cmp` |
| Integração | `testcontainers-go`, atrás de `//go:build integration` |
| Release | `goreleaser` |
| Lint | `golangci-lint` |

**Sem cgo.** Cross-compile precisa ser trivial. É por isso que a mineração de SQL usa extrator próprio em vez de `pg_query_go`.

**Sem viper, sem testify.** A superfície de configuração é flag, env e um TOML. `go-cmp` dá diff melhor nas structs do modelo.

A árvore de dependências pequena é requisito de produto, não gosto: o DBA que autoriza rodar isso contra produção abre o `go.mod` antes de decidir. Dependência nova no binário precisa de justificativa na proposal.

**`bubbletea` e `lipgloss` são a única exceção aberta até agora, e o custo dela está medido no binário que se publica.** Ele sai de 9 para 25 módulos e de 10,4 para 11,2 MB — 16 módulos e 800 KB. É muito, e foi aceito por uma razão específica: o `discover` tem vinte e uma flags, e a primeira execução de alguém acontece contra um banco que não é dele, com outra pessoa olhando. Um guia que pergunta escopo, modo e destino — e que **termina imprimindo o comando que compôs** — é o que separa uma ferramenta poderosa de uma ferramenta usável na primeira tentativa.

A primeira estimativa deste custo foi feita com arquivo sintético e build sem `strip`, e deu 28 módulos e 15,2 MB. Está registrado porque o erro foi na direção alarmista, e porque a lição vale: número de dependência se mede no artefato que sai, não numa aproximação.

`bubbles` foi recusado apesar de vir da mesma família e custar só 3 módulos a mais. Ele traz `atotto/clipboard` por causa do campo de texto, e "por que esta ferramenta lê minha área de transferência" é uma pergunta que não se quer responder num issue — a resposta honesta, que é dependência transitiva, não convence quem está decidindo se aponta a ferramenta para produção. Lista e campo de texto são escritos aqui, sobre os eventos de tecla que o `bubbletea` já entrega.

Nada disso vale para o resto do binário: as camadas que leem catálogo, inferem, validam e reportam continuam sem dependência de interface, e `pgfathom discover` funciona igual num terminal e num pipe.

## Arquitetura

Dependência em sentido único. Cada camada testável isolada.

```
cmd/pgfathom      entrada CLI, flags, orquestração
internal/db       conexão, pool, políticas de segurança e timeout
internal/catalog  leitura de pg_catalog e information_schema
internal/sqlprobe extração de predicados de junção de view e função
internal/model    modelo interno, tipos puros, sem I/O
internal/profile  perfis de nomenclatura, carregados de arquivo
internal/infer    geração e pontuação de candidatos (só metadados)
internal/stats    pré-filtro por estatística do planner
internal/validate validação contra dados, amostragem, anti-join
internal/report   renderização em terminal, JSON, SQL
```

`internal/model` não importa nada das outras camadas. `internal/infer`, `internal/profile` e `internal/sqlprobe` são determinísticos e não acessam banco. `internal/validate` é a **única** camada que lê dados de tabela do usuário.

## Convenções

Repositório, module path, binário, pacotes, imagem Docker: tudo minúsculo, `pgfathom`, sem underscore. Underscore com prefixo `pg_` é convenção de extensão que roda dentro do servidor; isto é binário externo.

Module path: `github.com/lvcas-dotcom/pgfathom`.

Tudo que está na árvore do repositório é em inglês — código, comentários, arquivos de build, workflows de CI e os documentos da raiz. Se um contribuidor consegue abrir, está em inglês.

Duas exceções, ambas deliberadas: as mensagens de commit e o registro de design (`docs/PGFATHOM.md` e `openspec/`) são em português. São a história do projeto e o raciocínio dele, não a interface.

Commits em Conventional Commits, em português, sem `Co-Authored-By`.

## Roadmap

As fases estão detalhadas em [`docs/ROADMAP.md`](../docs/ROADMAP.md). Uma change do OpenSpec por fase.

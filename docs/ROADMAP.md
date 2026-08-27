# Roadmap de implementação — v0.1

> **Status.** As oito fases abaixo estão fechadas e saíram na v0.1.0; a release
> corrente é a **v0.2.0**. O que mudou depois está no
> [CHANGELOG.md](../CHANGELOG.md), e o trabalho seguinte é acompanhado como
> issue, não aqui. Este documento continua sendo o registro de como o MVP foi
> ordenado e por quê — que é a parte que não envelhece.

Oito fases até o MVP. Cada fase é uma change do OpenSpec e termina em algo verificável — não em "camada pronta", mas em comportamento observável que passa em teste.

O princípio de ordenação é **empurrar o risco para a frente**. As duas primeiras fases não tocam em dado de usuário e não dependem de rede, então falham barato. A validação contra dados, que é onde mora o risco operacional, chega com o resto já testado embaixo dela.

Fases 1 a 5 entregam o `discover` funcionando. Fase 6 é o que faz ele ser melhor do que a concorrência. Fases 7 e 8 tornam o resultado publicável.

---

## Fase 1 — `bootstrap-core-model`

**Objetivo.** O esqueleto do repositório e as duas camadas puras: o modelo semântico e os perfis de nomenclatura.

**Escopo.** `go.mod`, Makefile, `golangci-lint`, GitHub Actions, `.gitignore`. `internal/model` com todos os tipos. `internal/profile` com carregamento de TOML, perfis `pt-br`/`en`/`es` embarcados via `embed`, e as funções de normalização de nome de coluna e de tabela. `cmd/pgfathom` com raiz cobra, `version`, e as convenções de saída — detecção de TTY, códigos de saída.

**Por que primeiro.** É a lógica mais sutil do projeto inteiro e a que mais vai quebrar com mudança, especialmente a despluralização. Também é a única que dá para testar exaustivamente sem infraestrutura nenhuma: sem Docker, sem banco, sem rede. Contribuidor externo consegue rodar a suíte no primeiro `git clone`.

**Entregável.** `go test ./...` verde em segundos. `pgfathom version` roda. Uma tabela de casos por perfil de idioma, cobrindo plural irregular em português.

**Critério de saída.** Cobertura alta em `internal/profile`. Nenhum teste depende de Docker.

---

## Fase 2 — `catalog-inspection`

**Objetivo.** Ler o catálogo com segurança e entregar o primeiro valor ao usuário.

**Escopo.** `internal/db` com o pool `pgx` e as políticas de segurança concentradas no `AfterConnect`: `default_transaction_read_only`, `application_name`, `lock_timeout`, `idle_in_transaction_session_timeout`. Precedência de credencial. `internal/catalog` lendo tabelas, colunas, tipos, PKs, uniques, índices, comentários, FKs declaradas com o campo `convalidated`, e estatísticas de uso com o timestamp de reset. Bloco de cobertura, incluindo tabelas puladas por falta de privilégio. Renderizador de terminal mínimo. Comando `pgfathom audit` com os dois achados que não dependem de inferência: constraint `NOT VALID` nunca validada e FK declarada sem índice do lado filho.

**Por que aqui.** O `audit` entrega valor real sem uma linha de inferência, e força a infraestrutura de teste de integração a existir cedo, quando ela ainda é barata de montar. Um banco que não tem nada a inferir ainda assim tem o que auditar.

**Entregável.** `pgfathom audit --dsn ...` roda contra um Postgres de testcontainers e reporta constraints não validadas e FKs sem índice, com bloco de cobertura.

**Critério de saída.** As políticas de segurança da sessão têm teste que prova que uma tentativa de escrita falha. Fixtures SQL versionadas em `testdata/`.

---

## Fase 3 — `fk-candidate-inference`

**Objetivo.** Gerar e pontuar candidatos a partir de metadados, sem tocar em dados.

**Escopo.** `internal/infer`: extração de nome de entidade a partir do nome da coluna usando o perfil ativo, casamento contra nomes de tabela normalizados, regras de compatibilidade de tipo, e a pontuação por sinais com pesos positivos e negativos. Corte por `--min-score`.

**Por que aqui.** É determinístico e não acessa banco, então herda o mesmo regime de teste barato da fase 1. Depende do modelo e do perfil, e nada mais.

**Entregável.** Dado um `model.Schema` de fixture, sai uma lista de candidatos ordenada por score, com os sinais que produziram cada score.

**Critério de saída.** O cenário-armadilha passa: coluna `status_id` numa base que tem tabela `status` mas onde a coluna guarda outra coisa gera candidato, e o score reflete a penalidade de nome genérico.

---

## Fase 3.5 — `naming-detection`

**Objetivo.** Derivar a convenção de nomenclatura do próprio schema, em vez de exigir que o usuário a escolha.

**Escopo.** Detecção de prefixo de tabela por frequência. Detecção do afixo de referência a partir das chaves estrangeiras já declaradas. Mesclagem do que foi detectado sobre o perfil base, sem descartar as regras dele. Relato explícito do que foi detectado, porque um perfil que muda sozinho e não avisa é pior que um perfil errado.

**Por que entrou fora do plano original.** Medição contra bancos reais de gestão pública, na fase 3:

| Banco | Tabelas | FKs | Recall |
|---|---|---|---|
| `geon_pr_assai` | 784 | 985 | 0,5% |
| `tributech_2` | 148 | 114 | ~0% |
| `sinter` (Django) | 18 | 10 | 0,0% |

Os dois primeiros usavam `_idkey` como sufixo de referência, que nenhum perfil oficial conhecia; adicionar o afixo à mão levou a 79,0% e 75,4%. O terceiro é schema em inglês com prefixo de aplicação — `auth_`, `django_` — e continuou em zero mesmo com `--profile en`.

O padrão é o mesmo nos três: **a convenção não é do idioma, é do schema.** E o modo de falha é silencioso — a ferramenta devolve quase nada e parece que o banco não tinha o que descobrir. Quem roda uma vez conclui que não serve.

Há também uma razão de método. A fase 8 mede recall no corpus de benchmark. Se o corpus rodar com perfil escolhido a mão, o número publicado não representa o que um usuário obtém na primeira execução, e a métrica principal do projeto deixa de ser honesta.

**Como é factível sem heurística nova.** As duas convenções estão no que a fase 2 já lê. Prefixo de tabela sai por frequência: se boa parte das tabelas de um schema começa com `tpl_`, isso é convenção e não coincidência. Afixo de referência sai das FKs declaradas: um banco com 470 delas está dizendo qual é a convenção, e basta comparar o nome da coluna filha com o da tabela alvo e olhar o resíduo.

**Entregável.** Rodar sem `--profile` contra os bancos medidos recupera o mesmo recall que o perfil ajustado à mão recuperou.

**Critério de saída.** O que foi detectado aparece na saída. Detecção pode ser desligada. Um afixo detectado nunca substitui os do perfil base, só acrescenta — a normalização já devolve conjunto, então afixo detectado errado custa uma forma espúria, não um falso negativo.

---

## Fase 4 — `stats-prefilter`

**Objetivo.** Matar candidato impossível usando estatística do planner, antes de qualquer I/O em tabela.

**Escopo.** `internal/stats`: leitura de `pg_stats` e `pg_class.reltuples`. Checagem de cardinalidade — mais valores distintos na filha do que linhas na pai é contenção total aritmeticamente impossível. Checagem de faixa pelos limites do histograma. Tratamento de estatística ausente ou velha: não opinar e registrar, nunca inventar rejeição.

**Por que separado da fase 3.** Porque o regime de confiança é diferente. A fase 3 opera sobre fatos do catálogo; esta opera sobre **estimativas**, que podem estar velhas. Misturar as duas na mesma camada faz a pontuação virar uma sopa onde ninguém sabe mais o que é fato e o que é palpite. Separada, a penalidade estatística é auditável e desligável.

**Entregável.** Redução mensurável no número de candidatos que chegam à validação, medida numa fixture com candidatos deliberadamente impossíveis.

**Critério de saída.** Teste que prova que nenhum valor de `most_common_vals` ou `histogram_bounds` sobrevive à camada — nem em struct exportada, nem em log, nem em erro.

---

## Fase 5 — `data-validation`

**Objetivo.** O núcleo do produto. Confirmar ou derrubar hipótese contra os dados reais.

**Escopo.** `internal/validate`: a query de agregação por valor distinto, que entrega contenção por linha e por valor na mesma passada e produz a métrica de cardinalidade. Modo amostrado com `TABLESAMPLE SYSTEM` e fallback `BERNOULLI`. `statement_timeout` por query, com candidato que estoura marcado como `unvalidated` e execução seguindo. Concorrência limitada por `errgroup.SetLimit`. Atribuição de veredito.

**Por que só agora.** É onde mora todo o risco operacional — query pesada, timeout, carga em produção. Chegar aqui com modelo, perfis, catálogo e inferência já testados significa que quando algo quebrar, dá para saber que quebrou aqui.

**Entregável.** `pgfathom discover` funciona ponta a ponta. Fixture com órfãos plantados produz veredito `broken` com contagem correta.

**Critério de saída.** Em modo amostrado, nenhum candidato sai como `confirmed` — sem exceção. Cancelamento por `context` não deixa query pendurada no servidor.

---

## Fase 6 — `sql-join-probe`

**Objetivo.** Descobrir relações que o casamento de nome não enxerga.

**Escopo.** `internal/sqlprobe`: tokenizador que trata dollar quoting (`$$` e `$tag$`), comentário de linha e de bloco, e identificador com aspas. Extração de igualdades entre referências qualificadas, com resolução de alias de `FROM`/`JOIN`. Leitura de `pg_views`/`pg_rewrite` e `pg_proc.prosrc`, com `pg_depend` como pré-filtro barato. Suporte opcional a `pg_stat_statements`. Cada junção encontrada vira sinal de peso alto e **também** cria candidato que a heurística de nome jamais geraria.

**Por que depois da validação, e não antes.** Porque é assim que dá para medir o que ele acrescenta. Com as fases 1 a 5 fechadas, existe uma linha de base de recall usando só nome. Ligar o probe e medir de novo produz a decomposição que a especificação exige no README: quanto o casamento de nome recupera sozinho e quanto a evidência de uso soma. Implementado antes, esse número não existiria.

**Entregável.** Fixture com relação descobrível **apenas** via view — coluna com nome que não se parece com a tabela alvo — passa de invisível a confirmada.

**Critério de saída.** SQL malformado ou não reconhecido é ignorado sem erro. O extrator degrada, nunca falha: junção perdida é sinal perdido, nunca resposta errada.

---

## Fase 7 — `report-outputs`

**Objetivo.** As três saídas, em qualidade publicável.

**Escopo.** `internal/report`: terminal agrupado por veredito com quebradas primeiro, cabeçalho com perfil e modo, rodapé com cobertura, e aviso destacado em modo amostrado. JSON com `schema_version` desde o primeiro release, tratado como API pública. SQL por categoria — `NOT VALID` mais `VALIDATE CONSTRAINT` separado para confirmadas, query de órfãos antes da DDL para quebradas, cabeçalho de revisão obrigatória em todo arquivo.

**Por que no fim.** Formatação regride fácil e golden file escrito cedo vira manutenção morta enquanto o conteúdo ainda muda. Com o conteúdo estável, os golden files valem o que custam.

**Entregável.** Golden files para terminal e SQL. SQL gerado executa sem edição manual num banco de teste.

**Critério de saída.** Teste que varre o JSON de todos os cenários procurando valor de dado vazado. Saída em pipe não emite ANSI.

---

## Fase 8 — `composite-keys-and-benchmark`

**Objetivo.** O número que sustenta o lançamento, e a distribuição.

**Escopo.** Chaves estrangeiras compostas, promovidas do "fora de escopo" original. Medição no banco municipal mostrou 86 tabelas de chave composta em 338 do escopo — **um quarto do banco** pulado com nota. É material demais para um MVP que se apresenta como ferramenta de banco legado, e a estrutura já existe: o modelo carrega chave de várias colunas, a cobertura registra o motivo, e a validação por anti-join generaliza para tupla sem mudança conceitual.

Harness do corpus: carregar schema público, remover todas as FKs declaradas, rodar `discover`, medir recuperação e falsos positivos. Corpus com GitLab, Odoo, Discourse, Redmine e Mastodon, mais um dump real em português anonimizado. Tabela de resultados no README, por schema, com a versão da ferramenta e a decomposição entre nome e evidência de uso. `goreleaser` com binário multiplataforma, release no GitHub, imagem Docker e tap do Homebrew. Licença.

**Por que as duas coisas na mesma fase.** O corpus existe para produzir um número defensável. Publicá-lo sabendo que um quarto de um schema alvo típico ficou fora tornaria a métrica enganosa por omissão — o recall pareceria melhor do que a cobertura real sustenta.

**Por que é fase, e não tarefa de fim de semana.** A taxa de recuperação é a métrica principal do projeto e vai no README. Ela precisa ser reproduzível por qualquer pessoa, versionada junto com o código e medida por script, não por planilha.

**Entregável.** `make benchmark` produz a tabela. `goreleaser release --snapshot` produz os artefatos.

**Critério de saída.** Zero falso positivo confirmado em todo o corpus. Se houver um, ele vira bug bloqueante antes do release, não nota de rodapé.

**Como saiu.** Em quatro changes, e a ordem entre elas foi a lição. Chave composta primeiro; o harness depois, que mediu a primeira e apontou que ela não alcançava a forma canônica de chave composta; a correção em seguida, com o número que a justificou; e a distribuição por último.

O corpus encolheu de cinco schemas para dois: só GitLab e Discourse publicam SQL carregável, e os outros três exigem subir a aplicação. O que sobrou é maior do que o previsto — o GitLab tem 1.857 chaves declaradas, quase o dobro do maior banco privado medido.

O critério de saída não é verificável no corpus público, e isso foi descoberto ao construí-lo: schema sem dados não confirma nada, e num banco real uma confirmação não declarada é indistinguível do achado que a ferramenta existe para produzir. Ele continua verificado onde é decidível, nas fixtures, e o relatório publicado diz isso em vez de deixar entender outra coisa.

---

## Depois do v0.1

`check --baseline` para CI, que é o que transforma diagnóstico pontual em infraestrutura recorrente e é a fase mais importante depois do `discover`. Depois: achados estruturais, padrões transversais, e exportação para DBML/Mermaid/PlantUML. Geração de código não está no roadmap.

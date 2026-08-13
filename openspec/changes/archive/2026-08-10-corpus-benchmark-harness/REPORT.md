# Relatório — benchmark do `pgfathom` contra o corpus de aplicações

**Data:** 2026-08-10
**Versão testada:** `fb4ddbd-dirty`
**Corpus:** Redmine, Discourse, Mastodon, Odoo, GitLab (as cinco apps do plano de benchmark de `docs/PGFATHOM.md`, "Corpus de benchmark")

## Resumo

| App | Tabelas | FKs declaradas | FKs elegíveis¹ | Recuperadas | Recall | Só nome | Via junção | Falsos positivos | Nota |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| Redmine | 54 | 0 | 0 | 0 | — | 0 | 0 | — | Schema não declara FK nenhuma no Postgres |
| Discourse | 381 | 25 | 25 | 11 | **44,0%** | 11 | 0 | 45 | Povoamento manual (plugins) |
| Mastodon | 115 | 154 | 154 | 20 | **12,99%** | 20 | 0 | 0 | Povoamento manual parcial (núcleo social, ~24/96 tabelas) |
| Odoo | 657 | 2526 | 2526 | 70 | **2,77%** | 56 | 14 | 1 | Demo data oficial, sem povoamento manual |
| GitLab | 1046 | 1846 | 1790² | 29 | **1,62%** | 29 | 0 | 4 | Povoamento manual mínimo (núcleo, tempo limitado) |

¹ FKs de coluna única — chave composta é `Skipped` pelo próprio `pgfathom` hoje, contar como falha mediria uma limitação já conhecida, não uma regressão (55 puladas no GitLab, 0 nas demais).
² 1846 FKs reais após corrigir duplicação de partição (ver GitLab abaixo) — 1790 elegíveis descontando as 55 compostas.

CSV agregado (mesmos campos de `metrics.json`, uma linha por app): `dumps_benchmarks/results/summary.csv` (fora do repositório — dado de execução local).

## Por que os números não são comparáveis entre si

O fator que mais determina o recall observado **não é** a qualidade do `discover` — é quanto da tabela envolvida em cada FK do gabarito tinha dado de verdade:

- **Redmine**: 0 FKs declaradas no banco (integridade só em ActiveRecord, nunca em `FOREIGN KEY`). Não existe gabarito pra medir recall — serviu como smoke test do harness, não como dado de recall.
- **Discourse**: gabarito inteiro (25 FKs) vivia em tabelas de plugin que o seed oficial (`dev:populate`) não toca. Povoamento manual cobriu as 25.
- **Mastodon**: gabarito grande (154 FKs, 96 tabelas envolvidas). Por tempo, povoamos só um núcleo social (~24 tabelas). Recall restrito a esse núcleo: **50% (20/40)** — bem mais alto que os 12,99% do total, porque a maior parte do gabarito nunca foi exercitada com dado.
- **Odoo**: único caso com demo data oficial rica o suficiente sem intervenção manual (24 sale orders, 92 stock moves, 61 partners, etc). O recall baixo aqui é o mais "puro" — não é falta de dado, é limite de casamento por nome (ver seção seguinte).
- **GitLab**: maior gabarito do corpus (1846 FKs reais, 1046 tabelas). Por tempo, povoamos manualmente um recorte mínimo (1 grupo, 3 projects, 6 issues, labels, milestones, notes). Recall restrito a esse recorte: **32,4% (11/34)**.

Ou seja: **Odoo é o número mais confiável pra avaliar a capacidade real do casamento por nome**, porque não tem o viés de "gabarito maior que o dado testado". Discourse e o subconjunto de Mastodon/GitLab mostram recall bem mais alto quando o dado existe.

## Por que o recall do Odoo (o número "limpo") ainda é baixo — causa raiz

Investigação no código (`internal/infer`, `internal/profile`) identificou três padrões distintos, cada um com correção diferente:

1. **Colunas de auditoria de ORM** — 34,8% do gabarito elegível do Odoo (880 de 2526 FKs) é só `write_uid`/`create_uid → res_users`, colunas que o ORM do Odoo adiciona automaticamente em quase toda tabela. Hoje não há tratamento nenhum pra esse padrão (`_by`/`uid` não é sufixo reconhecido em nenhum profile). Padrão comum entre frameworks (Rails `created_by_id`, Django `created_by`), não é peculiaridade só do Odoo.
2. **Tabela com prefixo de módulo** — `partner_id → res_partner`, `company_id → res_company` (Odoo); `target_account_id`/`from_account_id → accounts` (Mastodon). A entidade adivinhada a partir do nome da coluna é *parte* do nome da tabela, não igual a ele. A geração de candidato hoje só casa por **match exato** contra formas conhecidas (nome, prefixo removido, plural/singular) — sem fuzzy nem substring.
3. **Regra de plural faltando** — `favourites.status_id → statuses` (Mastodon) não casa porque o profile `en.toml` não tem regra pra plural em `-uses` (só cobre `-ies`, `-sses`, `-xes`, `-ches`, `-shes`, `-ices`, `-ves`, `-people`, mais o fallback genérico de tirar `s`, que reduz "statuses" a "statuse", não "status").

Auto-referência (`parent_id`, `in_reply_to_id` apontando pra própria tabela) **já é suportada** — confirmado no código e em teste dedicado (`TestSelfReferenceIsAllowed`). Não é gap.

Recomendação de prioridade pra fechar essa lacuna: (1) colunas de auditoria — mais barato, maior retorno; (2) match por sufixo/substring de tabela — resolve a família de prefixo de módulo inteira; (3) regra de plural `-uses`. Uma varredura estatística completa (todo par coluna↔tabela de tipo compatível, sem nome nenhum) resolveria o resto, mas é cara em schema grande (667–1046 tabelas neste corpus) e contraria o princípio do projeto de manter a execução default barata — melhor como modo opt-in, não default.

## Notas por app

### Redmine
Seed oficial (`redmine:load_default_data`) carrega só dado de referência (roles, trackers, status), não conteúdo (issues, projetos). Schema sem FK declarada — recall não mensurável, harness validado como smoke test (pipeline completo rodou sem erro, `discover` achou 2 relações reais por conta própria: `workflows.role_id → roles.id`, `workflows.tracker_id → trackers.id`).

### Discourse
Imagem `discourse/discourse_dev` não tem código-fonte — trocado por `discourse/discourse_test` (imagem de CI oficial). Schema exige extensão `vector` (pgvector). Perfil de nomenclatura trocado pra `en` (schema do corpus inteiro é em inglês, independente do idioma do dado — decisão que vale pras cinco apps). Nenhuma das 25 FKs do gabarito tocava tabela populada pelo seed oficial — povoamento manual via SQL direto, respeitando as FKs reais ainda ativas como validação.

### Mastodon
Imagem oficial é de produção — `RAILS_ENV=development` quebra (`annotate_rb`/`letter_opener_web` são gems de dev ausentes do bundle). Corrigido para `RAILS_ENV=production`. Precisa de `ACTIVE_RECORD_ENCRYPTION_*` geradas via `db:encryption:init`. `db:setup` só povoa 3 tabelas de config — resto (96 tabelas do gabarito) povoado manualmente só no núcleo social por tempo.

### Odoo
`-i all` **não instala todos os módulos** apesar do `--help` dizer isso — só instala `base` e dependências diretas (11 de 643 módulos disponíveis). Corrigido com lista explícita de módulos grandes (sale, purchase, stock, account, mrp, project, hr, etc.), que resolveu 123 via dependência transitiva. Único app com demo data rica o suficiente sem povoamento manual.

### GitLab
O mais espinhoso do corpus. `grafana['enable'] = false` não existe mais nesta versão do omnibus — quebrava o `reconfigure` inteiro. Imagem de produção com a mesma limitação de seed do Discourse/Mastodon — povoamento manual via `gitlab-rails runner` (ActiveRecord, não SQL cru, dado o tanto de cascata via callback do GitLab), o que exigiu descobrir a feature nova de Organizations (`organization_id` obrigatório pra criar usuário/grupo). `-U gitlab` explícito quebra autenticação (role real de peer-auth é `gitlab-psql`). Tabelas particionadas (CI) duplicavam FK por partição no gabarito (2629 brutas → 1846 reais) e quebravam o `DROP CONSTRAINT` — corrigido filtrando por `conislocal`. GitLab embarca Postgres 17, dump não restaura com `postgres:16`. Rodou sozinho, memória apertada na máquina (mínimo ~1 GiB livre) mas sem OOM.

## Comparação com um GitLab real de produção

Além do GitLab figurativo do corpus (seed mínimo manual, seção acima), rodamos o `discover` **só leitura** contra um GitLab de produção real (`gitlabhq_production`, Postgres 17.8, autorizado explicitamente pelo usuário para consulta — nenhuma escrita, nenhum `DROP CONSTRAINT`, nenhuma alteração de schema ou dado).

**Achado que muda a leitura da comparação:** `discover` pula inteiramente qualquer coluna que já faça parte de **qualquer** FK declarada, antes de gerar candidato (`eligible()` em `internal/infer/generate.go`, filtro por coluna, não por par específico). Isso significa que os 22 CONFIRMED abaixo não são "FK que já existia e foi redescoberta" — são relações **genuinamente não declaradas** que esse banco real tem hoje. Por isso não dá pra calcular um "recall" comparável ao do GitLab figurativo (que mede recuperação de FK **removida de propósito**, com gabarito conhecido): aqui nunca removemos nada, então não existe gabarito de comparação — só o que o `discover` acha por cima do que já está catalogado.

| | GitLab figurativo (benchmark) | GitLab real (produção) |
|---|---|---|
| Tabelas (schema `public`) | 1046 | 1015 |
| Tabelas analisadas pelo `discover` | 1046 | 992 tabelas no escopo · 853 analisadas (86%) |
| FKs declaradas | 1846 (após dedup de partição) | 3030 |
| Dado | seed mínimo manual (1 grupo, 3 projects, 6 issues) | produção real (dezenas de milhares de linhas em tabelas como `p_ci_builds_metadata`, `p_ci_job_artifacts`) |
| Metodologia | FK removida de propósito → `discover` → comparar contra gabarito | leitura pura, nenhuma FK tocada |
| Resultado | recall 1,62% total / 32,4% no recorte povoado (29 e 11 FKs, respectivamente) | **22 relações não-declaradas confirmadas**, 0 broken, 93 weak, 3,546s |
| Puladas | 55 compostas | 139 (25 sem privilégio de leitura, 19 chave composta, 95 particionadas) |
| Fora de escopo | — | 35 referências polimórficas (`noteable_id`+`noteable_type` etc.) — reconhecidas e deliberadamente não analisadas |

**Leitura:** o GitLab real tem quase o dobro de FK declarada (3030 vs 1846) e uma base de tabelas particionadas bem maior (95 puladas por partição, contra as que o benchmark sintético nem chegou a povoar o suficiente pra testar). As 22 relações CONFIRMED no real são achado genuíno de valor prático — 22 relações que existem nos dados de uma instância de produção, nunca formalizadas como `FOREIGN KEY`, encontradas em 3,5 segundos, sem nenhum falso positivo confirmado (`0 broken`, e nada em CONFIRMED que a validação por dado real não sustentasse). É o cenário mais próximo do uso pretendido do produto: um DBA rodando `discover` contra um banco real, sem gabarito nenhum, só pra ver o que o catálogo está deixando de declarar.

Pra virar um número de recall comparável de verdade, seria preciso repetir a metodologia destrutiva (remover as 3030 FKs numa cópia restaurada, nunca no banco original) — não autorizado nesta rodada, e não deveria ser, dado que é um banco de produção real.

## Onde estão os artefatos

- Harness (scripts): `.claude/benchmark/` — fora do repositório de propósito (scripts de execução local, não código do produto). `.claude/benchmark/RUNBOOK.md` documenta o fluxo completo, o workaround de Docker desta máquina, e todos os gotchas por app em detalhe maior que este relatório.
- Dumps, gabaritos, relatórios brutos (`report.json`, `report_no_probe.json`, `report.txt`, `run.log`, `metrics.json`) e `summary.csv`: `~/Área de trabalho/dumps_benchmarks/` — fora do repositório (dado de terceiro/execução local).
- Documentação da change: `openspec/changes/2026-08-10-corpus-benchmark-harness/` (`proposal.md`, `design.md`, `tasks.md`, este `REPORT.md`) — versionada no repositório.

## Estado da change

Tasks 1–3 completas (ambiente, harness, execução das cinco apps). Falta task 4.3: registrar no `README.md` do produto que este número é preliminar — decisão pendente de quando/como publicar, não incluída neste relatório porque é uma edição no README do produto, fora do escopo de "gerar relatório".

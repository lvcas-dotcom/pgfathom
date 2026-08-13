## Layout

Decisão revista durante a execução (3.2): os scripts NÃO ficam versionados no
repo principal — vivem em `.claude/benchmark/` (já coberto pelo `.gitignore`
existente do projeto), porque são scripts de execução local, não código do
produto, e o usuário quer poder mexer/documentar neles sem isso entrar em
nenhum commit. `.claude/benchmark/RUNBOOK.md` documenta o fluxo completo pra
retomar em sessão nova. Fora isso, o layout é o mesmo:

```
.claude/benchmark/
  apps/
    redmine/docker-compose.yml
    gitlab/docker-compose.yml
    odoo/docker-compose.yml
    discourse/docker-compose.yml
    mastodon/docker-compose.yml
  seed.sh          # sobe a app, roda o seed oficial, espera ficar pronta
  ground_truth.sh  # extrai FKs declaradas do banco seedado, antes de qualquer dump
  dump.sh          # pg_dump -Fc + derruba os contêineres da app
  run.sh           # restaura, remove FKs, roda discover 2x, calcula métricas
  compare.py       # ground_truth.json + report json (normal e --no-probe) -> metrics.json
```

Dado, fora do repo, em `/home/gabriel-arantes/Área de trabalho/dumps_benchmarks/`:

```
dumps_benchmarks/
  redmine/
    redmine.dump              # pg_dump -Fc, saída do passo "dump"
    ground_truth.json         # FKs declaradas antes da remoção (o gabarito)
  gitlab/    ...
  odoo/      ...
  discourse/ ...
  mastodon/  ...
  results/
    redmine/
      report.json              # discover --full --format json (probe ligado)
      report_no_probe.json      # discover --full --format json --no-probe
      report.txt                # discover --full --format table (leitura humana)
      run.log                   # stdout/stderr + `time -v` de cada execução
      metrics.json              # ver formato abaixo
    gitlab/    ...
    odoo/      ...
    discourse/ ...
    mastodon/  ...
    summary.csv                 # uma linha por app, agregando metrics.json de todos
```

**Onde vou olhar para a análise:** `results/<app>/metrics.json` por app (o número), `results/<app>/report.txt` quando o número for surpreendente (o detalhe — qual relação especificamente não voltou), e `results/summary.csv` para a tabela comparativa final entre os cinco.

## Seed por aplicação

Todas rodam sobre Postgres — é a única engine que o `pgfathom` suporta (`docs/PGFATHOM.md`, "PostgreSQL 13+"), então mesmo o Redmine (que também aceita MySQL) sobe com `DB_ADAPTER=postgresql`.

| App | Imagem | Seed | Observação de porte |
|---|---|---|---|
| Redmine | `redmine:5-alpine` + `postgres:16` | `bundle exec rake redmine:load_default_data REDMINE_LANG=pt-BR` | leve, minutos |
| Discourse | `discourse/discourse_test:release` (imagem de CI oficial, já vem com o repo + gems — `discourse/discourse:release` é produção, não serve; `discourse_dev` não tem o código-fonte) + `pgvector/pgvector:pg15` (schema atual exige a extensão `vector`) | `bin/rails db:create db:migrate && ALLOW_DEV_POPULATE=1 bundle exec rake dev:populate`, mais povoamento manual das tabelas de plugin com FK declarada (ver 3.2 em `tasks.md` — `dev:populate` não cobre nenhuma) | médio |
| Mastodon | `tootsuite/mastodon` + `postgres:16` | `bundle exec rails db:setup` (schema + seeds mínimos; sem gerador de povoamento fictício oficial — se o recall exigir mais volume, complementar com `faker` ad-hoc, a decidir) | médio |
| Odoo | `odoo:17` + `postgres:15` | `-i all` sem `--without-demo` (default já carrega demo pros módulos instalados; `--with-demo=all` não existe na CLI real) | médio, muitos módulos m2m |
| GitLab | `gitlab/gitlab-ce:latest` | `gitlab-rake gitlab:setup` / `db:seed_fu` conforme disponível na imagem omnibus | **pesado** — omnibus sobe Rails, Sidekiq, Redis, Postgres, Gitaly juntos; historicamente pede 4 GB+ de RAM e minutos para health check ficar `healthy` |

GitLab é o caso que a especificação do produto chama de "melhor caso disponível" (centenas de tabelas), mas também o mais caro de subir — candidato natural a rodar sozinho, sem os outros quatro em paralelo.

## Gabarito (ground truth)

Antes de tocar em qualquer FK, `ground_truth.sh` roda contra o banco recém-seedado:

```sql
select
  con.conname,
  con.conrelid::regclass::text  as child_table,
  con.confrelid::regclass::text as parent_table,
  array_agg(att.attname order by u.ord)      as child_columns,
  array_agg(attf.attname order by u.ord)     as parent_columns
from pg_constraint con
join lateral unnest(con.conkey)  with ordinality as u(attnum, ord) on true
join pg_attribute att  on att.attrelid = con.conrelid  and att.attnum = u.attnum
join lateral unnest(con.confkey) with ordinality as uf(attnum, ord) on uf.ord = u.ord
join pg_attribute attf on attf.attrelid = con.confrelid and attf.attnum = uf.attnum
where con.contype = 'f'
group by con.conname, con.conrelid, con.confrelid;
```

Cada linha vira um registro em `ground_truth.json`, com `is_composite = len(child_columns) > 1`. É esse arquivo — não o schema restaurado — que `run.sh` usa para gerar o `ALTER TABLE ... DROP CONSTRAINT` de cada FK antes de chamar o `discover`.

## Perfil de nomenclatura usado no `discover`

`run.sh` roda com `--profile en`, não o `pt-br` default do `pgfathom`. Descoberto na execução real do Discourse (3.2): todo o corpus (Redmine, Discourse, Mastodon, Odoo, GitLab) tem schema em inglês por convenção de framework (Rails/etc.) — o idioma dos dados de seed não muda os nomes de tabela/coluna. Rodar com `pt-br` contra esses schemas usa regras de plural erradas (`categories→category` é regra do inglês, não existe no perfil pt-br) e derruba candidatos por engano. Configurável via `DISCOVER_PROFILE` se algum app do corpus vier a ter schema em outro idioma.

## Cálculo de recall

Só FKs de coluna única entram no denominador — chave composta é `Skipped` pelo próprio `pgfathom` hoje (`ReasonCompositePK` / `SkipCompositeKey`), contar como falha seria medir uma limitação já conhecida e documentada, não uma regressão.

```
elegíveis   = FKs do gabarito com 1 coluna
recuperadas = elegíveis onde (child_table, child_column, parent_table, parent_column)
              aparece em report.json com veredito "confirmed" ou "broken"
recall total       = recuperadas / elegíveis
recall só-nome      = recuperadas em report_no_probe.json / elegíveis
recall via junção   = recuperadas em report.json AND NÃO em report_no_probe.json
falsos_positivos    = candidatos "confirmed" em report.json sem par no gabarito
```

Isso decompõe exatamente como `docs/PGFATHOM.md` pede: "quanto o casamento de nome recupera sozinho e quanto a evidência de uso acrescenta".

`metrics.json` por app:

```json
{
  "app": "redmine",
  "tables": 0,
  "fk_total": 0,
  "fk_composite_skipped": 0,
  "fk_eligible": 0,
  "recovered_total": 0,
  "recovered_name_only": 0,
  "recovered_via_probe": 0,
  "false_positives": 0,
  "recall_pct": 0.0,
  "duration_normal_s": 0.0,
  "duration_no_probe_s": 0.0,
  "pgfathom_version": "",
  "note": ""
}
```

`recall_pct` é `null`, não `0.0`, quando `fk_eligible = 0` — descoberto na execução real do Redmine (3.1): schema sem nenhuma FK declarada no Postgres não é "recall zero", é "sem gabarito pra medir". `note` carrega esse aviso; fica vazio nos casos normais.

## Isolamento entre seed e benchmark

Os contêineres da aplicação (Rails/Odoo/GitLab completos) só existem durante o seed — servem para gerar dados realistas, nada mais. Depois do `pg_dump`, `dump.sh` derruba tudo (`docker compose down -v`). O benchmark em si roda contra um `postgres:16` void, restaurado do dump — mais leve, reprodutível sem a stack completa da aplicação de novo, e mais próximo do cenário real do produto (um DBA com acesso só ao Postgres, não à aplicação).

## Riscos conhecidos antes de executar

- **Docker não instalado** nesta máquina — precisa de instalação (`apt`, via `sudo`) antes de qualquer passo. Ação que requer confirmação explícita, não é assumida.
- **Memória livre baixa no momento** (~1,5 GiB livres, swap já parcialmente ocupado por outros processos da sessão do usuário — IDE, `ng serve`, um servidor Spring). GitLab sozinho já é pesado; rodar mais de uma app pesada em paralelo é risco real de OOM ou de degradar o resto do trabalho do usuário na máquina. Por isso o harness roda **uma app por vez**, sequencial, com teardown completo entre uma e outra.
- **Mastodon não tem task de povoamento fictício oficial** equivalente às outras quatro — a ser resolvido na hora (schema geralmente já entrega dezenas de tabelas com relações via `db:setup`, mas o volume de linhas pode ficar baixo; se o recall não for representativo, complementar dados sintéticos fica registrado como decisão tomada durante a execução, não escondida no resultado).
- **GitLab omnibus** é a imagem mais difícil de automatizar de forma decisiva sem tentativa — health check e task exata de seed variam por versão; primeira tentativa pode exigir ajuste depois de ver o log real.

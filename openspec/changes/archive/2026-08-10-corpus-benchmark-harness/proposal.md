## Why

O README classifica o projeto como `EARLY ALPHA` até existir uma taxa de recuperação medida em schema real e reproduzível — a Fase 8 do roadmap (`composite-keys-and-benchmark`) descreve exatamente esse corpus: GitLab, Odoo, Discourse, Redmine e Mastodon, com o procedimento "carregar o schema, remover todas as FKs declaradas, rodar o `discover`, medir quantas voltaram e quantos falsos positivos apareceram" (`docs/PGFATHOM.md`, "Corpus de benchmark").

Essa change entrega só a metade "corpus + harness" da Fase 8, adiantada. As chaves compostas (`internal/infer.SkipCompositeKey`) continuam fora de escopo — não estão implementadas ainda — então o número que este harness produz **não é** o número final publicável no README. É um baseline de engenharia: mede o que o `discover` já recupera hoje, decompõe entre casamento de nome e evidência de junção, e vira o piso de comparação de quando as chaves compostas entrarem.

## What Changes

- Scripts de infraestrutura (fora do binário `pgfathom`, sem nova dependência em `go.mod`) para, por aplicação do corpus:
  1. Subir a aplicação via Docker com seu Postgres interno e popular dados via o mecanismo oficial dela (rake/seed/demo flag).
  2. Extrair o "gabarito": todas as FKs declaradas (`pg_constraint` `contype='f'`), separando as de coluna única (o que o `discover` de hoje consegue recuperar) das compostas (fora de alcance até chaves compostas existirem).
  3. Exportar o dump (`pg_dump -Fc`) e descartar os contêineres pesados da aplicação.
  4. Restaurar o dump num Postgres limpo e descartável, remover todas as FKs declaradas, rodar `pgfathom discover --full` duas vezes (uma normal, uma com `--no-probe`) para decompor nome vs. junção, e comparar o resultado contra o gabarito.
  5. Calcular recall, falsos positivos e tempo de execução, e gravar tudo em disco.
- Nenhuma linha de `internal/*` ou `cmd/*` muda. O harness só invoca o binário já existente via `discover --format json`.
- Dumps, gabaritos e resultados ficam fora do repositório, em `/home/gabriel-arantes/Área de trabalho/dumps_benchmarks/`, porque são artefatos de dados de terceiros (ainda que fictícios/demo) e de execução local, não código-fonte.
- Decisão revista durante a execução (3.2): os próprios scripts do harness também ficam fora do repositório principal, em `.claude/benchmark/` (gitignorado) — são ferramentas de execução local, não parte do produto `pgfathom`. Só esta documentação da change (`proposal.md`/`design.md`/`tasks.md`) é versionada.

## Capabilities

Nenhuma. Este change não adiciona nem modifica comportamento público do `pgfathom` — ele exercita o `discover` já existente como caixa-preta, pela CLI. Não há `specs/` novo.

## Impact

- Sem dependência nova no `go.mod`: os scripts usam `docker`, `psql`/`pg_dump`/`pg_restore` e `jq`, fora da árvore de build do Go.
- Regra de read-only do produto (`openspec/project.md`, regra 1) permanece intacta: o `pgfathom` nunca escreve no banco analisado. Quem remove as FKs do gabarito é o script do harness, com `psql` direto, fora da ferramenta — é o próprio harness preparando a fixture, não o produto mutando um banco de produção.
- Regra 2 (dado do usuário nunca sai): os bancos do corpus são de demonstração/seed fictício das próprias aplicações, não dado real de terceiro. Ainda assim os dumps não entram no repositório nem em nenhum output versionado — ficam só em `dumps_benchmarks/`, fora do controle de versão.
- Bloqueadores de ambiente identificados antes de qualquer execução (ver mensagem de acompanhamento no chat): Docker não está instalado nesta máquina, e a memória livre no momento da checagem é baixa (~1,5 GiB livres de 30 GiB, swap já em 6/8 GiB). Isso condiciona sequenciamento e possivelmente instalação de pacote via sudo — decisão do usuário, não autônoma.

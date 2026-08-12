# pgfathom

Ferramenta de linha de comando para sondagem de schema PostgreSQL. Mede a profundidade real de bancos legados: descobre e valida a estrutura que existe nos dados mas nunca foi declarada no catálogo.

*Fathom* é a braça, unidade náutica de profundidade, e é o verbo de compreender aquilo que está fundo. A ferramenta faz as duas coisas: joga a linha, mede, e devolve o que encontrou com a incerteza declarada junto.

---

## Como usar este documento

Este arquivo é a fonte de verdade do projeto e serve como contexto permanente para desenvolvimento. Antes de implementar qualquer coisa, consulte a seção correspondente aqui. Se uma decisão de implementação contradiz este documento, o documento vence, ou o documento precisa ser atualizado primeiro.

### Regras invioláveis

Estas não são preferências. Uma violação de qualquer uma delas é bug de severidade máxima, não questão de estilo.

**Nunca escreva comando que altere o banco analisado.** A ferramenta é estritamente read-only. Ela gera arquivos `.sql` para o usuário revisar e executar por conta própria.

**Nunca imprima, logue ou persista valores de dados do usuário.** A ferramenta lê dados para comparar chaves, mas o que sai dela são contagens, proporções e nomes de objetos. O caso de uso alvo inclui bancos de gestão pública com CPF, dados de saúde e cadastro de contribuintes.

Essa regra tem uma armadilha específica que precisa estar clara desde o início: `pg_stats` contém `most_common_vals` e `histogram_bounds`, que **são valores reais de dados do usuário**. A ferramenta lê essas colunas para pontuar candidatos, e isso é legítimo. Mas esses valores não podem chegar a nenhuma saída, a nenhum log, a nenhum campo do JSON, nem a mensagem de erro. Eles existem em memória, produzem um número, e morrem ali. Todo campo `Detail` de sinal, toda mensagem de diagnóstico, todo dump de debug precisa ser auditado contra isso.

**Nunca afirme uma relação como certa sem evidência nos dados.** Toda inferência sai com veredito, nível de confiança e a métrica que sustenta esse nível.

**Nunca reporte silêncio como ausência de problema.** Tabela pulada por falta de privilégio, candidato que estourou timeout, schema não analisado — tudo isso aparece no resumo. Um relatório limpo tem que significar "analisei e está limpo", nunca "não consegui olhar".

**Nunca reivindique originalidade que não existe.** Ver a seção de posicionamento. O README não pode dizer que ninguém resolve esse problema, porque não é verdade e alguém vai apontar isso no primeiro comentário do lançamento.

O README público e todo identificador em código são em inglês. Este documento e a documentação interna podem ficar em português.

---

## O problema

Bancos de dados PostgreSQL antigos e grandes carregam mais informação do que declaram. Ao longo de anos de manutenção, times diferentes e migrações mal executadas, o schema vai perdendo aquilo que dava sentido a ele.

O sintoma mais comum é a chave estrangeira ausente. A coluna `pedido.cliente_id` aponta pra `cliente.id` em toda linha da tabela, mas não existe constraint declarada. Isso acontece por vários motivos legítimos e ilegítimos: alguém desabilitou constraints pra acelerar uma carga e nunca religou, o ORM da época não criava, a migração de um banco antigo trouxe só as tabelas, ou simplesmente ninguém achou necessário.

O custo disso aparece depois. Quem chega no projeto não consegue entender o modelo, porque `\d tabela` não mostra relacionamento nenhum. Ferramenta de ERD gera um diagrama de caixas soltas. Gerador de código produz structs sem associação. E o pior: sem a constraint, o banco nunca impediu que dados órfãos entrassem, então provavelmente já existem, silenciosamente, há anos.

Existe uma variação mais traiçoeira desse caso, que quase ninguém considera. A constraint pode estar declarada e mesmo assim não valer nada, porque foi criada com `NOT VALID` e nunca validada. Nesse estado o Postgres passa a impedir violações novas, mas nunca verificou as linhas que já estavam lá. O `\d` mostra a FK, a ferramenta de ERD desenha a seta, todo mundo assume integridade — e os órfãos antigos continuam ali.

Junto disso vem uma segunda camada de conhecimento perdido. Colunas que funcionam como soft delete mas só em parte das tabelas. Colunas de tenant presentes em algumas entidades e ausentes em outras que deveriam tê-las. Campos que são enum na cabeça de todo mundo e `varchar` livre no banco, com três variações de digitação do mesmo valor. Tabelas que ninguém lê há anos e que continuam sendo mantidas, versionadas e migradas.

Nada disso está documentado em lugar nenhum. Está no banco, e dá pra extrair.

---

## O que o pgfathom faz

Conecta a uma instância PostgreSQL, lê o catálogo do sistema, cruza com as estatísticas de uso, minera as definições de view e função em busca de junções reais, e amostra os dados de forma controlada. A partir disso monta um modelo semântico do banco: as entidades, os relacionamentos reais (declarados, inferidos e evidenciados por uso), e um conjunto de achados sobre a saúde estrutural do schema.

A saída principal não é um diagrama. É um relatório acionável mais artefatos prontos pra uso: DDL sugerida, queries de diagnóstico, e um modelo em JSON versionado que outras ferramentas podem consumir.

O fluxo conceitual:

1. **Inspeção do catálogo**, para saber o que está declarado — incluindo o que está declarado mas não validado.
2. **Mineração de evidência de uso**, lendo definições de view, corpo de função e, quando disponível, o log normalizado de queries, para extrair junções que o código real executa.
3. **Geração de candidatos**, para levantar hipóteses sobre o que não está declarado.
4. **Pontuação por metadados**, para descartar hipóteses fracas antes de tocar em dados.
5. **Pré-filtro estatístico**, usando as estatísticas do planner para eliminar impossibilidades sem custo de I/O.
6. **Validação contra os dados**, para confirmar ou derrubar as hipóteses restantes.
7. **Relatório e artefatos.**

As etapas 1 e 2 leem exclusivamente catálogo. A etapa 5 lê estatística. Só a etapa 6 toca em tabela do usuário.

---

## Posicionamento

Existe muita ferramenta boa no ecossistema PostgreSQL, e o pgfathom não deve competir com nenhuma delas. Vale ser explícito sobre o que já está resolvido.

Lint de migração está coberto pelo Squawk. Detecção de drift entre ambientes está coberta pelo Atlas e pelo migra. Saúde de índice está coberta pelo pganalyze e por dezenas de coleções de query. Diagrama ER está coberto pelo SchemaSpy, Azimutt e dbdocs. Geração de código a partir de schema está coberta pelo sqlc, jOOQ, Ent e Prisma.

### O que já existe no problema específico

Isto precisa estar registrado com honestidade, porque a reivindicação errada destrói credibilidade no lançamento.

**Existe literatura acadêmica madura.** O que a ferramenta chama de contenção é, na literatura de data profiling, uma *inclusion dependency* — formalmente descrita como a parte automaticamente testável de uma chave estrangeira. É um campo com algoritmos publicados e comparados: SPIDER para dependências unárias, BINDER e MIND para n-árias, tudo implementado no framework open source Metanome, do HPI. O problema geral é NP-difícil e W[3]-completo.

Isso não invalida o projeto, mas define a postura. Metanome é framework acadêmico em Java, opera sobre arquivos exportados, não fala com PostgreSQL, não lê `pg_catalog`, não gera DDL, não conhece convenção de nomenclatura em português e nunca vai ser instalado por um time de produto. O espaço é de engenharia e de produto, não de invenção algorítmica.

Vale também a nota técnica: SPIDER resolve o problema completo com um único passe de sort-merge sobre conjuntos de valores distintos, enquanto a abordagem deste documento faz uma query por candidato sobrevivente. A escolha aqui é deliberada — a poda por metadados e por estatística reduz o espaço de candidatos a ponto de o custo por candidato deixar de importar, e uma query por candidato é infinitamente mais fácil de instrumentar, cancelar e explicar ao usuário. Se em banco real o número de candidatos sobreviventes explodir, a abordagem de passe único volta à mesa.

**Existe sobreposição parcial em ferramenta open source.** O Azimutt já tem análise de schema que sinaliza colunas terminadas em `_id` sem relação declarada, e tem heurística algorítmica para relações faltantes no roadmap. É ativo e bem feito.

**Existe solução comercial de GUI.** Hackolade e Oracle Data Modeler fazem inferência de relacionamento, sempre baseada exclusivamente em metadados.

### Onde fica a fronteira

O que separa o pgfathom não é inferir. É **validar a inferência contra os dados reais e reportar a incerteza com honestidade**.

Casar nome de coluna com nome de tabela é trivial e produz muito falso positivo. O que separa hipótese de fato é rodar o anti-join e responder: essa relação bate em quantos por cento das linhas e em quantos por cento dos valores distintos, quantos registros órfãos existem, e a distribuição de cardinalidade indica um-para-muitos ou um-para-um.

E há um segundo diferencial, que é o que quebra o teto de qualquer abordagem por nome: **evidência de uso extraída do próprio catálogo**. Uma view que faz `JOIN funcionario f ON f.id = os.resp_tecnico` prova que existe relação entre `os.resp_tecnico` e `funcionario.id`, e nenhuma heurística de nomenclatura no mundo vai descobrir isso, porque `resp_tecnico` não se parece com `funcionario`. Relações com nomenclatura divergente são o teto estrutural do casamento de nome, e minerar junção de view é como o pgfathom passa por cima dele. É catálogo puro, custo desprezível e risco zero.

A validação divide os candidatos em grupos com tratamentos diferentes:

**Relação confirmada.** Contenção total, nenhum órfão. É uma FK que alguém esqueceu de declarar. Gere a DDL e siga.

**Relação real com integridade quebrada.** Contenção alta mas não total. Existem órfãos. Esse é o achado mais valioso da ferramenta, porque é um bug de dados que está em produção há anos e ninguém sabe. Aqui não basta gerar a DDL, precisa gerar também a query que lista os órfãos, porque eles têm que ser resolvidos antes.

**Evidência insuficiente.** Coluna com um valor distinto só, proporção de nulos altíssima, tabela vazia, ou validação que não completou. Não dá pra concluir, e a ferramenta diz isso em vez de chutar.

**Coincidência de nome.** Contenção baixa. Descartar, e registrar que foi descartado, pra que o usuário não fique se perguntando por que a ferramenta ignorou uma coluna óbvia.

---

## Escopo

### v0.1 (MVP)

Dois comandos.

`pgfathom discover` faz inferência e validação de chaves estrangeiras de coluna única, com mineração de junção em view e função como sinal, e pré-filtro estatístico antes da validação.

`pgfathom audit` faz os achados que **não dependem de inferência nenhuma** e por isso são gratuitos, determinísticos e sem risco de falso positivo: constraint declarada com `NOT VALID` e nunca validada, FK declarada sem índice na coluna filha, e o relatório de cobertura da análise.

Saída em tabela no terminal, JSON e SQL.

A inclusão do `audit` no MVP é deliberada. São achados que reutilizam a mesma leitura de catálogo e o mesmo motor de anti-join, custam pouquíssimo código, e entregam valor mesmo num banco onde a inferência não encontre nada. Uma ferramenta que sempre tem algo a dizer tem uma chance de segunda execução.

### Fora do escopo do MVP

Detecção de padrões semânticos além de FK (soft delete, tenant, enum). Diagrama. Suporte a outros bancos além de PostgreSQL. Modo de escrita no banco, em qualquer fase, sob qualquer flag, jamais.

### Fases seguintes

**Fase 2 — Regressão em CI.** Comando `pgfathom check --baseline`. Compara o estado atual contra um modelo JSON versionado no repositório e falha o build quando aparece relação não declarada nova, quando a contagem de órfãos cresce numa relação já conhecida, ou quando uma relação antes confirmada regride.

Esta fase vem cedo de propósito, e a razão é estratégica. Uma ferramenta de diagnóstico se roda uma vez: o usuário roda, resolve, esquece, e o projeto morre de retenção zero. O modo de CI transforma o pgfathom de diagnóstico pontual em infraestrutura recorrente, e é o que faz o formato JSON valer a pena manter estável. É provavelmente o recurso mais importante do projeto depois do próprio `discover`.

**Fase 3 — Achados estruturais.** Expansão do `audit`. Tabelas sem leitura nem escrita desde o último reset de estatística, com todas as ressalvas da seção de armadilhas. Colunas integralmente nulas. Colunas com nome de padrão temporal ou de exclusão lógica usadas de forma inconsistente entre tabelas. `varchar` de baixa cardinalidade que deveria ser enum ou tabela de domínio, com as variações de digitação contadas — nunca exibidas.

**Fase 4 — Padrões transversais.** Detecção de coluna de tenant e identificação de tabelas que deveriam tê-la e não têm. Detecção de relacionamento polimórfico, o par `entidade_id` mais `entidade_tipo`, que a inferência simples nunca vai pegar corretamente.

**Fase 5 — Consumidores do modelo.** Exportação para DBML, Mermaid e PlantUML a partir do modelo enriquecido, para que ferramenta de diagrama consuma um schema que conhece os relacionamentos reais.

Geração de código **não** está no roadmap. Competir com sqlc, jOOQ, Ent e Prisma é briga perdida e dilui a identidade do projeto. O JSON versionado é o ponto de integração: quem quiser gerar código a partir de um modelo que conhece as relações reais tem o contrato para fazer isso, e faz melhor do que o pgfathom faria.

---

## Arquitetura

Camadas com dependência em sentido único. Cada uma testável isolada.

```
cmd/pgfathom        entrada CLI, parsing de flag, orquestração
  |
internal/db         conexão, pool, políticas de segurança e timeout
  |
internal/catalog    leitura do pg_catalog e information_schema
  |
internal/sqlprobe   extração de predicados de junção de view e função
  |
internal/model      modelo interno, tipos puros, sem I/O
  |
internal/profile    perfis de nomenclatura, carregados de arquivo
  |
internal/infer      geração e pontuação de candidatos (só metadados)
  |
internal/stats      pré-filtro por estatística do planner
  |
internal/validate   validação contra dados, amostragem, anti-join
  |
internal/report     renderização em terminal, JSON, SQL
```

`internal/model` não importa nada das outras camadas. `internal/infer`, `internal/profile` e `internal/sqlprobe` operam exclusivamente sobre o model e são determinísticos, sem acesso a banco, o que torna o teste deles trivial. `internal/validate` é a única camada que lê dados de tabela do usuário.

---

## Modelo interno

Esboço das estruturas centrais. Ajustar durante a implementação, mas manter a separação entre o que foi lido, o que foi evidenciado e o que foi inferido.

```go
type Schema struct {
    Name   string
    Tables []Table
}

type Table struct {
    Schema      string
    Name        string
    Columns     []Column
    PrimaryKey  []string        // nomes de coluna, em ordem
    Uniques     []UniqueConstraint
    ForeignKeys []ForeignKey    // apenas as DECLARADAS
    Indexes     []Index
    Stats       TableStats
    Partitioned bool
    Comment     string
}

type Column struct {
    Name      string
    Type      string            // tipo formatado, ex "bigint", "character varying(60)"
    BaseType  string            // tipo normalizado para comparação, ex "int8"
    Nullable  bool
    Default   string
    Position  int
    Comment   string
}

type ForeignKey struct {
    Name       string
    Columns    []string
    RefTable   string
    RefColumns []string
    Validated  bool             // pg_constraint.convalidated — falso = NOT VALID
    HasIndex   bool             // existe índice utilizável no lado filho
}

type TableStats struct {
    EstimatedRows int64         // reltuples
    SeqScans      int64
    IdxScans      int64
    Inserts       int64
    Updates       int64
    Deletes       int64
    TotalBytes    int64
    StatsResetAt  *time.Time    // sem isso, contador de uso não significa nada
}

// ColumnStats vem de pg_stats. NENHUM campo aqui pode chegar a saída ou log.
// Existe para pontuar e para o pré-filtro, e morre em memória.
type ColumnStats struct {
    NullFraction  float64
    NDistinct     float64       // negativo = razão sobre reltuples
    HasBounds     bool
    boundsLow     any           // NUNCA serializar
    boundsHigh    any           // NUNCA serializar
}

type SignalKind string

const (
    SigExactName        SignalKind = "exact_name"
    SigNormalizedName   SignalKind = "normalized_name"
    SigIdenticalType    SignalKind = "identical_type"
    SigCompatibleType   SignalKind = "compatible_type"
    SigUniqueTarget     SignalKind = "unique_target"
    SigAmbiguousTarget  SignalKind = "ambiguous_target"
    SigChildIndexed     SignalKind = "child_indexed"
    SigCommentMention   SignalKind = "comment_mention"
    SigNotNull          SignalKind = "not_null"
    SigJoinInView       SignalKind = "join_in_view"
    SigJoinInFunction   SignalKind = "join_in_function"
    SigJoinInStatements SignalKind = "join_in_statements"
    SigGenericDomain    SignalKind = "generic_domain_name"
    SigRangeViolation   SignalKind = "stats_range_violation"
    SigCardViolation    SignalKind = "stats_cardinality_violation"
)

type Signal struct {
    Kind   SignalKind
    Weight float64           // pode ser negativo
    Detail string            // nome de objeto apenas, NUNCA valor de dado
}

type Candidate struct {
    Child      ColumnRef
    Parent     ColumnRef
    Signals    []Signal
    MetaScore  float64        // 0..1, antes de olhar dados
    Validation *Validation    // nil se não foi validado
    Verdict    Verdict
    SkipReason string         // preenchido quando Verdict == VerdictUnvalidated
}

type Validation struct {
    Method              string        // "full" ou "sampled"
    SampledRows         int64
    NotNullRows         int64
    DistinctVals        int64
    OrphanRows          int64
    OrphanVals          int64
    ContainmentRows     float64       // (NotNullRows - OrphanRows) / NotNullRows
    ContainmentVals     float64       // (DistinctVals - OrphanVals) / DistinctVals
    MaxRowsPerValue     int64         // 1 sugere 1:1, muito acima de 1 sugere 1:N
    Duration            time.Duration
}

type Verdict string

const (
    VerdictConfirmed   Verdict = "confirmed"    // FK esquecida, íntegra
    VerdictBroken      Verdict = "broken"       // FK real com órfãos
    VerdictWeak        Verdict = "weak"         // evidência insuficiente
    VerdictRejected    Verdict = "rejected"     // coincidência de nome
    VerdictUnvalidated Verdict = "unvalidated"  // timeout, permissão, caso não suportado
)

// Achados que não dependem de inferência.
type Finding struct {
    Kind    string          // "not_valid_constraint", "fk_without_index", ...
    Object  string
    Detail  string
    Metrics map[string]int64
}

// Cobertura é obrigatória em toda saída. Silêncio nunca é ausência de problema.
type Coverage struct {
    TablesTotal        int
    TablesAnalyzed     int
    TablesNoPrivilege  []string
    TablesExcluded     []string
    TablesUnsupported  []string      // particionada, herança, sem PK
    CandidatesFound    int
    CandidatesValidated int
    CandidatesTimedOut  int
    StatsResetAt       *time.Time
    PgStatStatements   bool
}
```

---

## Algoritmo

### Etapa 1: leitura de catálogo

Tabelas, colunas, tipos, PKs, uniques, índices, comentários e as FKs declaradas — com o campo `convalidated` de `pg_constraint`, que é o que revela a constraint `NOT VALID`. Estatísticas de uso de `pg_stat_user_tables`, sempre acompanhadas do timestamp de reset.

Tudo aqui é catálogo. Nenhum dado de usuário é lido.

### Etapa 2: mineração de evidência de uso

Extrair predicados de junção das definições de view (`pg_views` / `pg_rewrite`), do corpo das funções (`pg_proc.prosrc`) e, quando a extensão estiver instalada, das queries normalizadas de `pg_stat_statements`.

O que interessa é uma única forma: igualdade entre duas referências de coluna qualificadas, `a.x = b.y`, onde ambos os lados resolvem para colunas de tabelas conhecidas. Cada ocorrência vira um sinal de peso alto no par correspondente, e — importante — **também cria candidatos que a heurística de nome jamais geraria**.

Sobre a implementação: usar um parser SQL completo exigiria `pg_query_go`, que depende de cgo e viola a regra de cross-compile trivial. A alternativa correta aqui não é um parser, é um **extrator deliberadamente estreito**, que tokeniza procurando apenas por igualdades entre referências qualificadas e resolve aliases do `FROM`/`JOIN`. Ele tem permissão explícita para falhar: SQL que ele não entende é simplesmente ignorado.

Essa permissão é o que torna a abordagem segura. A saída do extrator é *sinal*, nunca veredito. Uma junção não extraída é um sinal perdido, o que na pior hipótese devolve o candidato ao caminho normal de inferência por nome. Uma junção extraída errado gera um candidato ruim, que a validação contra dados vai rejeitar. Em nenhum dos dois casos o extrator produz resposta errada — só produz mais ou menos ajuda.

### Etapa 3: geração de candidatos

Para cada coluna que não seja chave primária da própria tabela e que não participe de FK já declarada, tentar extrair um nome de entidade alvo.

Normalização do nome da coluna. Remover sufixos de referência, na ordem: `_id`, `_codigo`, `_cod`, `_key`, `_ref`, `_fk`, `id`, `cod`. Remover prefixos: `id_`, `cod_`, `fk_`. O resultado é o candidato a nome de entidade.

Normalização do nome da tabela. Remover prefixos comuns de convenção antiga: `tb_`, `tbl_`, `sys_`, `cad_`, `mov_`. Aplicar despluralização. O resultado é um **conjunto ordenado de formas candidatas**, sempre incluindo o nome original inalterado, e o casamento tem sucesso quando qualquer forma casa — ver a seção de perfis para o porquê.

Todas essas regras — sufixos, prefixos, plural — vêm de um **perfil de nomenclatura** carregado de arquivo, nunca de constante em código. Ver a seção dedicada.

Um candidato nasce quando o nome de entidade extraído da coluna casa com o nome normalizado de alguma tabela, e essa tabela tem chave primária de coluna única, e o tipo base da coluna filha é compatível com o tipo da PK. Ou quando a etapa 2 encontrou uma junção real entre as duas colunas, caso em que o casamento de nome é dispensável.

Compatibilidade de tipo: idêntico é o caso ideal. Inteiro menor apontando pra inteiro maior é aceitável. `text` e `varchar` de qualquer tamanho são intercambiáveis entre si. `uuid` só casa com `uuid`. Numérico não casa com textual, nunca.

### Etapa 4: pontuação por metadados

Sinais que somam confiança, antes de tocar em dados:

Junção encontrada em view ou função é o sinal mais forte que existe, porque é evidência de que o código real trata aquilo como relação. Casamento exato do nome de entidade com o nome da tabela, sem precisar de normalização agressiva, é forte. Tipo base idêntico à PK alvo, forte. Apenas uma tabela candidata pra aquele nome, forte, porque ambiguidade é o principal gerador de ruído. A coluna já possuir índice, moderado, porque indica que alguém a usa em join. Comentário da coluna ou da tabela mencionando a entidade alvo, moderado. Coluna ser `NOT NULL`, fraco mas positivo.

Sinais que subtraem: a tabela alvo é muito pequena e o nome é genérico, tipo `status` ou `tipo`, porque isso costuma ser tabela de domínio onde o casamento é real mas a relação é menos interessante. Múltiplas tabelas candidatas para o mesmo nome. Tipo compatível mas não idêntico.

Candidatos abaixo de um limiar configurável são descartados sem chegar na validação.

### Etapa 5: pré-filtro estatístico

Antes de qualquer acesso a dados, consultar `pg_stats` e `pg_class.reltuples`. Custo desprezível, e elimina uma fatia grande dos sobreviventes.

**Cardinalidade.** Se o número de valores distintos estimado da coluna filha excede o número estimado de linhas da tabela pai, contenção total é aritmeticamente impossível. Uma coluna com mais valores distintos do que existem chaves possíveis não pode estar contida nelas.

**Faixa.** Para tipos ordenáveis, se os limites do histograma da coluna filha caem fora dos limites da PK do pai, a contenção alta é improvável.

Duas ressalvas de engenharia. Primeiro, são **estimativas** — `n_distinct` e histograma vêm do `ANALYZE`, podem estar velhos ou grosseiros. Por isso o pré-filtro aplica penalidade forte de score por padrão e só rejeita direto quando a violação excede uma margem de tolerância larga. Segundo, se as estatísticas estiverem ausentes porque a tabela nunca sofreu `ANALYZE`, o pré-filtro não opina, e isso é registrado. Ele nunca inventa uma rejeição a partir de dado que não tem.

E vale repetir: os valores lidos daqui são dados do usuário e não saem da memória.

### Etapa 6: validação contra dados

Para cada candidato sobrevivente, uma query de agregação. Nunca trazer linhas, só contagens.

A formulação agrega por valor distinto antes do anti-join. Isso importa por três razões: o anti-join passa a rodar sobre a cardinalidade da coluna em vez de sobre a contagem de linhas, o que em coluna de baixa cardinalidade é diferença de ordens de magnitude; entrega contenção por linha e por valor distinto na mesma passada; e produz a métrica de cardinalidade de graça.

```sql
WITH child_vals AS (
    SELECT c.<col> AS v, count(*) AS n
    FROM <child_relation> c
    WHERE c.<col> IS NOT NULL
    GROUP BY 1
),
marked AS (
    SELECT cv.v, cv.n,
           EXISTS (SELECT 1 FROM <parent> p WHERE p.<pk> = cv.v) AS matched
    FROM child_vals cv
)
SELECT
    count(*)                                            AS distinct_vals,
    coalesce(sum(n), 0)                                 AS not_null_rows,
    count(*)  FILTER (WHERE NOT matched)                AS orphan_vals,
    coalesce(sum(n) FILTER (WHERE NOT matched), 0)      AS orphan_rows,
    coalesce(max(n), 0)                                 AS max_rows_per_value
FROM marked;
```

As duas contenções contam histórias diferentes e ambas precisam sair no relatório. Um único valor inválido repetido em um milhão de linhas derruba a contenção por linha e mal arranha a por valor distinto. Muitos valores raros e inválidos fazem o contrário. Quando as duas divergem muito, isso em si é um achado.

Em modo amostrado, `<child_relation>` é a tabela filha com `TABLESAMPLE SYSTEM` calibrado para atingir aproximadamente o número de linhas alvo, com fallback para `TABLESAMPLE BERNOULLI` em tabela pequena, onde `SYSTEM` amostra por página e distorce demais. Em modo completo, é a tabela direta. A seção de armadilhas explica por que amostragem aqui é mais fraca do que parece.

Antes de cada validação, aplicar `statement_timeout`. Candidato que estourar o timeout é marcado como `unvalidated`, com o motivo registrado, e a execução continua. A ferramenta nunca deve travar num banco grande, e nunca deve deixar uma query pendurada.

### Etapa 7: veredito

Contenção por linha total, contenção por valor distinto total, e mais de um valor distinto na coluna filha, resulta em **confirmada**.

Contenção acima do limiar de quebra, por padrão noventa por cento, mas abaixo de total, resulta em **quebrada**. Registrar a contagem de órfãos por linha e por valor, que é o dado que interessa.

Contenção abaixo do limiar de rejeição, por padrão cinquenta por cento, resulta em **rejeitada**.

Coluna com um único valor distinto, com proporção de nulos muito alta, ou tabela filha vazia, resulta em **fraca** independente da contenção, porque a evidência estatística não sustenta conclusão.

Em modo amostrado, nenhum candidato pode ser marcado como confirmada. O melhor veredito alcançável por amostra é "provável, confirme com `--full`". Isso é questão de honestidade da ferramenta e não tem exceção.

---

## Armadilhas conhecidas

Estas não são casos de borda a resolver depois. São formas específicas de a ferramenta estar errada com confiança, que é o único jeito de ela perder a credibilidade de vez.

### Amostragem é mais fraca contra órfão do que contra qualquer outra coisa

Órfãos quase nunca são aleatórios. Eles entram em lote — uma carga malfeita, uma migração de fim de semana, um período em que a aplicação estava com bug — e por isso ficam **fisicamente agrupados nas mesmas páginas**. `TABLESAMPLE SYSTEM` amostra por página.

O padrão de amostragem padrão tem, portanto, viés justamente contra o padrão de dado que a ferramenta existe para encontrar. Uma relação com meio milhão de órfãos concentrados pode voltar de uma amostra limpa.

A consequência para o design: modo amostrado é **triagem**, não evidência. Ele serve para ordenar o que merece atenção num banco grande. A resposta é o `--full`. O relatório precisa dizer isso com todas as letras, não em rodapé.

### Contador de uso não sobrevive ao que as pessoas acham que sobrevive

`pg_stat_user_tables` zera em `pg_stat_reset()`, zera em `pg_upgrade`, e é **por nó**: uma tabela lida exclusivamente em réplica de leitura aparece com zero leituras na primária.

Dizer "essa tabela não é usada há anos" com base nisso, num banco de gestão pública onde alguém pode agir sobre a informação, é a pior coisa que a ferramenta pode fazer. Todo achado dessa família carrega o timestamp de reset junto e a ressalva sobre réplicas, sem exceção, e a linguagem é sempre "sem uso registrado desde X neste nó", nunca "não utilizada".

### Cobertura parcial silenciosa

`pg_stats` só expõe linhas de tabelas que o usuário corrente pode ler. Se o usuário read-only não tiver `SELECT` em todo o schema — cenário provável em órgão público, onde conseguir privilégio é processo político — a análise fica parcial e o relatório fica limpo pelo motivo errado.

Daí a struct `Coverage` ser obrigatória em toda saída, e o resumo do terminal sempre terminar com quantas tabelas foram efetivamente analisadas sobre o total.

### Casos que devem ser pulados com nota, nunca em silêncio

Relacionamento polimórfico, onde `documento_id` só faz sentido junto com `documento_tipo`. A validação vai encontrar contenção baixa e rejeitar, o que é o comportamento correto, mas a ferramenta deveria reconhecer o padrão pelas colunas vizinhas e dizer que reconheceu.

Chave estrangeira composta parcialmente correspondida: se o lado filho oferece contraparte para parte das posições da chave alvo e não para todas, nenhum candidato é gerado — a constraint parcial rejeitaria linha válida — e a fração que casou vira nota. Idem para assinatura de chave dividida por mais de uma tabela sem que nada no filho nomeie qualquer uma delas: mais de um candidato alcançaria contenção total e no máximo um é real, então nenhum é escolhido.

Tabelas particionadas, onde estatística e contagem se comportam diferente. Ler da tabela pai e não iterar partições.

Herança de tabela, raro mas existe em base antiga.

Múltiplos schemas, onde a mesma tabela pode existir em vários. O casamento de nome deve considerar o schema, e o comportamento entre schemas precisa ser configurável.

Colunas de referência que apontam para tabelas que não existem mais. Contenção zero, rejeição correta, mas vale reportar separado porque é um achado em si.

---

## Perfis de nomenclatura

As regras de afixo e de plural ficam em arquivo de configuração, não em código. O binário embarca os perfis oficiais via `embed`, e `--profile` aceita caminho para um arquivo do usuário.

```toml
# profiles/pt-br.toml
name = "pt-br"

column_suffixes = ["_id", "_codigo", "_cod", "_key", "_ref", "_fk", "id", "cod"]
column_prefixes = ["id_", "cod_", "fk_"]
table_prefixes  = ["tb_", "tbl_", "sys_", "cad_", "mov_"]

# Identificador de banco é quase sempre ASCII: `opcoes` é muito
# mais comum que `opções`. As duas formas precisam estar cobertas.

[[plural]]                      # opções → opção
suffix = "ões"
singular = "ão"

[[plural]]                      # opcoes → opcao
suffix = "oes"
singular = "ao"

[[plural]]                      # pães → pão
suffix = "ães"
singular = "ão"

[[plural]]                      # paes → pao
suffix = "aes"
singular = "ao"

[[plural]]                      # animais → animal
suffix = "ais"
singular = "al"

[[plural]]                      # papéis → papel
suffix = "éis"
singular = "el"

[[plural]]                      # responsaveis → responsavel
suffix = "eis"
singular = "el"

[[plural]]                      # lençóis → lençol
suffix = "óis"
singular = "ol"

[[plural]]                      # lencois → lencol
suffix = "ois"
singular = "ol"

[[plural]]                      # azuis → azul
suffix = "uis"
singular = "ul"

[[plural]]                      # perfis → perfil
suffix = "is"
singular = "il"

[[plural]]                      # armazens → armazem
suffix = "ns"
singular = "m"

[[plural]]                      # mulheres → mulher
suffix = "res"
singular = "r"

[[plural]]                      # meses → mes
suffix = "ses"
singular = "s"

[[plural]]                      # clientes → cliente
suffix = "s"
singular = ""
```

### A normalização devolve um conjunto, não uma string

Esta é a decisão que evita uma classe inteira de falso negativo, e ela é contra-intuitiva o suficiente para merecer o registro.

O caminho óbvio seria aplicar as regras na ordem e devolver a primeira que casa. Isso quebra em ambiguidade real. A tabela `logins` produz `logim` sob a regra `ns → m`, que está correta para `armazens`, e produz `login` sob a regra genérica de queda de `s`. Não existe informação no nome que resolva qual das duas está certa. Com primeira-regra-vence, a ordem escolhida decide arbitrariamente qual dos dois casos o projeto quebra.

Então a normalização de nome de tabela devolve um **conjunto ordenado de formas candidatas**, sempre incluindo o nome original inalterado, e o casamento tem sucesso quando qualquer forma casa. Toda regra aplicável contribui uma forma; a ordem de declaração determina a ordem de preferência dentro do conjunto, não exclusividade.

A forma que casou é reportada junto. É isso que permite à pontuação distinguir `SigExactName` de `SigNormalizedName` — casamento no nome original vale mais que casamento obtido depois de remover prefixo e despluralizar.

O custo é um conjunto pequeno por tabela em memória e a possibilidade de casamento espúrio por forma agressiva. O segundo é aceitável: a pontuação penaliza, e a validação contra dados derruba o que for coincidência. O projeto aceita ruído de candidato, não aceita falso positivo confirmado.

A regra genérica de `s` fica sempre por último, e as regras específicas vêm antes das genéricas.

Isso resolve duas coisas ao mesmo tempo. Tecnicamente, torna a lógica mais frágil do projeto — despluralização — testável por tabela de casos em vez de por código. E estrategicamente, converte o que seria um teto de adoção numa superfície de contribuição: um perfil novo é o PR mais fácil que um projeto pode oferecer a quem passa pelo repositório. Ship inicial com `pt-br`, `en` e `es`.

Detecção automática de perfil, por frequência de terminação nos nomes de tabela do schema, é desejável mas fica para depois do MVP. Até lá, o padrão é `pt-br` e o aviso de qual perfil está ativo aparece no cabeçalho da saída.

---

## Segurança operacional

A ferramenta vai rodar em banco de produção de terceiros. Uma execução que derrube ou trave um servidor encerra o projeto, independentemente da qualidade do resto.

**Sessão.** `default_transaction_read_only = on`, `application_name = 'pgfathom'` para que o DBA identifique a origem em `pg_stat_activity`, `statement_timeout` por query de validação, `lock_timeout` baixo, e `idle_in_transaction_session_timeout` para nunca segurar transação aberta.

**Privilégio.** O recomendado na documentação é um usuário exclusivo somente-leitura. A ferramenta verifica via `has_table_privilege` se o usuário corrente tem `INSERT`, `UPDATE` ou `DELETE` nas tabelas do escopo, e avisa. É cinto junto com suspensório: a sessão read-only já impede escrita, mas o aviso educa.

**Credencial.** `--dsn` no argv aparece em `ps` e no histórico do shell. A ordem de precedência documentada é `PGFATHOM_DSN`, depois as variáveis padrão do libpq (`PGHOST`, `PGUSER`, `PGDATABASE`, `PGPASSFILE`), e `--dsn` por último, com nota explícita no `--help` sobre o vazamento.

**Carga.** Concorrência limitada e configurável, com padrão baixo. Rodar quarenta anti-joins simultâneos num banco de produção é a forma mais rápida de a ferramenta ser banida da empresa. O padrão de 4 é conservador de propósito.

---

## Interface de linha de comando

```
pgfathom discover [flags]

  --dsn string           string de conexão (prefira PGFATHOM_DSN)
  --schema strings       schemas a analisar (padrão: public)
  --all-schemas          todo schema não-sistema acessível; exclui --schema
  --exclude-schema       padrões de schema a remover do escopo
  --exclude strings      padrões de tabela a ignorar
  --profile string       perfil de nomenclatura (padrão: pt-br)
  --full                 validar contra todas as linhas, sem amostragem
  --sample int           linhas alvo por amostra (padrão: 100000)
  --min-score float      limiar de metadado para validar (padrão: 0.5)
  --no-sql-probe         desativa mineração de junção em view e função
  --timeout duration     statement_timeout por query de validação (padrão: 30s)
  --concurrency int      validações simultâneas (padrão: 4)
  --format string        table | json | sql (padrão: table)
  --out string           diretório de saída para artefatos
  --include-rejected     mostrar também os candidatos descartados

pgfathom audit [flags]
  # mesmas flags de conexão e escopo
  # achados que não dependem de inferência

pgfathom check --baseline model.json [flags]   # fase 2
  # sai com código diferente de zero quando há regressão
```

### Sobre o escopo de schema

O padrão continua `public`, e ampliar escopo é opt-in. Vale a mesma razão da seção de carga: a execução que alguém faz antes de ler qualquer documentação precisa ser a mais barata, não a mais cara, porque ela acontece contra um banco de produção que não é de quem está rodando. Varrer o banco inteiro sem flag transformaria o caso silencioso no caso mais caro possível.

`--schema` e `--all-schemas` são mutuamente exclusivas. Não existe precedência defensável entre as duas: qualquer que fosse, produziria uma linha de comando cujo escopo real não é o que ela aparenta pedir. A detecção de que `--schema` foi fornecida usa o estado de alteração da flag, nunca comparação com o valor padrão — `--schema public --all-schemas` é exatamente o caso ambíguo que a regra existe para recusar.

Padrão de schema e padrão de tabela são flags separadas de propósito. Um padrão único que significasse as duas coisas faria `--exclude legacy` deixar de pular uma tabela chamada `legacy` e passar a derrubar o schema de mesmo nome, sem que ninguém digitasse nada diferente — mudança silenciosa de significado, que é a regra 4 vista de outro ângulo.

Todo schema visível que ficou fora do escopo aparece no bloco de cobertura, inclusive na execução sem flag alguma. Relatório sobre `public` que não menciona os outros onze schemas está correto e engana, e quem já sabe que precisa de `--all-schemas` não é quem está sendo enganado.

---

## Saídas

### Terminal

Tabela agrupada por veredito, quebradas primeiro porque são o achado mais urgente, depois confirmadas, depois fracas. Colunas: relação, contenção por linha, contenção por valor, órfãos, linhas analisadas, método.

Cabeçalho com o perfil de nomenclatura ativo e o modo de validação. Rodapé com contagens por veredito, tempo total, e o bloco de cobertura — tabelas analisadas sobre total, tabelas sem privilégio, candidatos que estouraram timeout. Em modo amostrado, um aviso destacado de que nenhuma confirmação é definitiva.

### JSON

O modelo completo, incluindo o que foi lido do catálogo, o que foi evidenciado por uso e o que foi inferido, com todos os sinais e métricas de validação preservados, mais o bloco de cobertura.

Esse arquivo é o contrato de integração com o `check`, com as fases futuras e com ferramentas de terceiros. Campo `schema_version` desde o primeiro release, e mudança incompatível exige incremento. É o ativo de longo prazo do projeto e deve ser tratado como API pública.

Nenhum campo do JSON contém valor de dado do usuário. Isso precisa ter teste automatizado, não só revisão.

### SQL

Um arquivo por categoria.

Para confirmadas, DDL com `NOT VALID` e o `VALIDATE CONSTRAINT` separado, comentado, porque `NOT VALID` não trava a tabela e a validação posterior pega um lock mais leve. Incluir também o `CREATE INDEX CONCURRENTLY` na coluna filha quando ela não tiver índice, porque FK sem índice do lado filho é armadilha clássica em delete.

Para quebradas, primeiro a query que lista os órfãos, depois a DDL comentada, com aviso de que ela só vai passar depois da limpeza.

Para o `audit`, o `VALIDATE CONSTRAINT` das constraints que estão `NOT VALID`, comentado, com a query que verifica se passaria.

Nenhum arquivo gerado deve ser executável sem revisão humana. Cabeçalho explícito em cada um, com a versão da ferramenta, o timestamp e o modo de validação usado.

---

## Decisões técnicas

Go. Binário único, sem runtime, distribuível por `go install`, release no GitHub, imagem Docker e tap do Homebrew. Casa com o resto do ecossistema de ferramenta de banco e com o objetivo de aprendizado do projeto.

`pgx/v5` direto, sem ORM e sem camada de abstração. A ferramenta lê catálogo, não faz CRUD.

`cobra` para CLI. É o padrão de fato e reduz atrito de contribuição.

Nenhuma dependência que exija cgo. Cross-compile precisa ser trivial. É por isso que a mineração de SQL usa extrator estreito em vez de `pg_query_go`.

Repositório, module path, binário, pacotes, imagem Docker e tap: tudo minúsculo, `pgfathom`, sem underscore. Underscore com prefixo `pg_` é convenção de extensão que roda dentro do servidor; isto é binário externo.

---

## Testes

Unitário puro para `internal/infer`, `internal/profile` e `internal/sqlprobe`, que são determinísticos. É onde mora a maior parte da lógica sutil, especialmente a despluralização, e é o que mais vai quebrar com mudança. Cobertura alta aqui é obrigatória.

Perfis de nomenclatura testados por tabela de casos, entrada e saída esperada, um arquivo por idioma. Adicionar caso é trivial e não exige entender o resto do projeto.

Integração com `testcontainers-go` subindo PostgreSQL real. Fixtures SQL em `testdata/`, cada uma representando um cenário: schema limpo com FK declarada, schema sem nenhuma FK, schema com órfãos plantados de propósito, schema com colisão de nome, schema em português com plural irregular, schema com constraint `NOT VALID` e órfãos pré-existentes, schema com relação só descobrível via view.

Golden files para as saídas de terminal e SQL, porque formatação regride fácil.

Um teste dedicado que varre todo o JSON de saída de todos os cenários procurando por valor de dado vazado. A regra de não vazar dados é a mais fácil de quebrar por acidente e a mais cara quando quebra.

Um cenário de teste deve ser explicitamente uma armadilha: coluna chamada `status_id` numa tabela onde existe uma tabela `status` mas a coluna guarda outra coisa. A ferramenta tem que rejeitar.

### Corpus de benchmark

A taxa de recuperação só convence se for medida em schema real que qualquer pessoa possa reproduzir. O corpus é montado a partir de schemas públicos grandes e completamente anotados com FK: GitLab, que tem centenas de tabelas e é o melhor caso disponível, mais Odoo, Discourse, Redmine e Mastodon. Junto com pelo menos um dump real de sistema em português, anonimizado.

O procedimento é sempre o mesmo: carregar o schema, remover todas as FKs declaradas, rodar o `discover`, medir quantas voltaram e quantos falsos positivos apareceram.

O resultado vai numa tabela no README, por schema, com a versão da ferramenta. Isso é métrica de engenharia e material de divulgação na mesma peça.

---

## Critério de aceite do MVP

O comando roda contra um banco real de porte relevante, na casa das centenas de tabelas, sem travar, sem exceder o timeout global e sem impacto perceptível na carga do servidor.

A taxa de recuperação no corpus de benchmark é publicada, por schema. É a métrica principal do projeto.

**Nenhum falso positivo confirmado.** Falso negativo é aceitável, falso positivo confirmado destrói a confiança na ferramenta. Se a escolha for entre recuperar mais e nunca errar, nunca errar vence.

A saída SQL é executável sem edição manual num banco de teste.

Duas execuções consecutivas sobre o mesmo banco produzem saída idêntica byte a byte, descontado o timestamp. Ordenação instável quebra golden file, torna ilegível qualquer diff de relatório e impede o modo de CI de distinguir mudança real de ruído.

Nenhum valor de dado do usuário aparece em qualquer saída, em qualquer formato, em qualquer cenário do corpus de teste.

O bloco de cobertura aparece em toda execução.

### Sobre a métrica, antes que alguém se decepcione

O recall vai estabilizar bem abaixo de cem por cento, e isso é esperado, não fracasso. Relações cuja nomenclatura não guarda relação nenhuma com a tabela alvo são invisíveis ao casamento de nome por construção — é exatamente por isso que a mineração de junção em view existe, e é por isso que o número reportado deve vir sempre em duas partes: quanto o casamento de nome recupera sozinho e quanto a evidência de uso acrescenta.

Essa decomposição é honesta e é também o argumento mais forte do projeto, porque mostra em número o que a abordagem por metadados pura deixa na mesa.

---

## Glossário

**Candidato.** Par de colunas que pode representar uma relação, levantado por heurística de nome, por evidência de uso, ou por ambas, antes de qualquer verificação nos dados.

**Contenção.** Proporção dos valores da coluna filha que existem na chave da tabela pai. Medida em duas variantes, por linha e por valor distinto. É a métrica central da validação.

**Cobertura.** O que foi efetivamente analisado sobre o que existe. Obrigatória em toda saída.

**Evidência de uso.** Junção real encontrada em definição de view, corpo de função ou log de queries. Prova que o código trata as duas colunas como relacionadas, independentemente de como se chamam.

**Inclusion dependency.** Nome formal da contenção na literatura de data profiling. A parte de uma chave estrangeira que é automaticamente testável a partir dos dados.

**`NOT VALID`.** Estado de constraint que impede violações novas mas nunca verificou as linhas preexistentes. Aparece como FK normal no `\d` e não garante integridade histórica.

**Órfão.** Linha da tabela filha cujo valor de referência não existe na tabela pai. Em banco com FK validada é impossível. É o que a ferramenta procura.

**Perfil de nomenclatura.** Conjunto de regras de afixo e plural, carregado de arquivo, que traduz convenção de nomenclatura de um idioma ou de uma casa.

**Relação declarada.** Chave estrangeira que existe como constraint no catálogo. Pode estar validada ou não.

**Relação inferida.** Relação que existe nos dados mas não no catálogo.

**Modo amostrado.** Validação sobre subconjunto das linhas. Rápido, indicativo, e estruturalmente fraco contra órfãos agrupados. Serve para triagem, não para conclusão.

**Modo completo.** Validação sobre todas as linhas. Lento, conclusivo.

## Why

O escopo de schema da ferramenta é `public` e não há caminho para ampliá-lo a não ser digitar cada schema à mão. Isso produz duas falhas de natureza diferente.

A primeira é de alcance. Banco legado de gestão raramente vive só em `public`: schema por módulo, por cliente, por ano de arquivo morto é a norma no ambiente-alvo. Quem aponta a ferramenta para um banco desses precisa saber a lista de schemas de antemão para passá-la em `--schema` — e essa é precisamente a informação que ele não tem, porque foi descobrir estrutura que ele veio.

A segunda é de honestidade, e é a mais grave, porque contradiz uma regra inviolável do projeto. A regra 4 diz que silêncio nunca é ausência de problema, e nomeia "schema não coberto" entre as coisas que precisam aparecer no bloco de cobertura. Hoje não aparecem. Uma execução num banco com doze schemas devolve um relatório sobre `public` que em lugar nenhum menciona os outros onze. O relatório está correto e mesmo assim engana, que é exatamente o modo de falha que a regra existe para impedir.

O que emoldura a solução é que `public` como padrão não é acidente de implementação: `docs/PGFATHOM.md` documenta `--schema strings  schemas a analisar (padrão: public)`, e a seção de carga do mesmo documento explicita a filosofia por trás disso — todo padrão é conservador, e o que for mais amplo ou mais pesado é opt-in do usuário. Ampliar o escopo é portanto uma flag, nunca uma mudança de padrão.

## What Changes

- `--all-schemas`: resolve o escopo a partir do catálogo, incluindo todo schema não-sistema sobre o qual o papel conectado tem `USAGE`. Mutuamente exclusiva com `--schema`; passar as duas é erro de uso, não precedência silenciosa.
- `--exclude-schema`: padrões glob que removem schemas do escopo antes de qualquer leitura de catálogo. É o freio de `--all-schemas` — sem ele, "todos os schemas" é uma flag sem contrapartida.
- `--exclude` permanece com o significado atual: padrão de tabela, casado contra a forma nua e contra a qualificada, sempre dentro dos schemas em escopo. Nenhuma linha de comando existente muda de sentido.
- `internal/catalog` ganha a resolução de escopo (`ResolveScope`) e a consulta de schemas visíveis. Escopo vazio depois da exclusão é erro de uso, não execução silenciosa sobre nada.
- `model.Coverage` ganha a dimensão de schema: total visível, total analisado, a lista dos que existem e ficaram fora, e a lista dos removidos por filtro — separadas pelo mesmo motivo que `TablesExcluded` já é campo distinto de `TablesNoPrivilege`.
- O bloco de cobertura passa a declarar os schemas fora do escopo em toda execução, com o ponteiro para `--all-schemas`. Vale inclusive para quem nunca usar as flags novas, e é a parte da mudança que fecha a lacuna da regra 4.
- README documenta escopo em inglês; `docs/PGFATHOM.md` registra as flags e a decisão de manter o padrão.

## Capabilities

### Modified Capabilities

- `catalog-inspection`: o requisito de escopo controlável deixa de descrever apenas lista explícita mais exclusão de tabela, e passa a cobrir a resolução a partir do catálogo, a exclusão de schema e o erro de escopo vazio. Ganha requisito próprio para a dimensão de schema na cobertura.
- `cli-foundation`: o conjunto de flags dos subcomandos que leem catálogo passa a incluir `--all-schemas` e `--exclude-schema`, com a exclusividade mútua e o escopo vazio como erros de uso.

## Impact

Nenhuma dependência nova. A consulta de schemas lê `pg_namespace`, que é catálogo puro: nenhum dado de usuário é lido, e a fronteira que a spec `catalog-inspection` declara para esta camada permanece intacta.

Uma consulta a mais por execução, inclusive para quem roda no padrão e nunca vai ligar `--all-schemas`. É deliberado: é essa consulta que alimenta a lista de schemas não analisados, e sem ela a correção da regra 4 só valeria para quem já sabia que precisava dela.

`Coverage` ganha quatro campos, o que quebra o golden do contrato JSON. Acréscimo de campo é compatível e não exige incremento de `schema_version`; o golden existe justamente para que o acréscimo seja um ato consciente que passa por revisão, e atualizá-lo é parte do trabalho.

`Complete()` fica mais estrito: uma execução com schema fora do escopo deixa de poder se declarar completa. É a consequência pretendida — um relatório não pode afirmar cobertura total enquanto existir schema que não foi olhado.

`--all-schemas` amplia a superfície de carga contra o banco analisado. As políticas que contêm essa carga — concorrência limitada, `statement_timeout`, `lock_timeout` — permanecem inalteradas e continuam valendo por candidato, então o risco cresce em duração, não em pico. Ainda assim, é a razão pela qual a flag é opt-in e o padrão não muda.

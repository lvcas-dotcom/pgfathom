## Contexto

Esta change não descobre nada e não muda nenhuma inferência. Ela mexe em duas coisas: qual conjunto de schemas entra na análise, e o que o relatório declara sobre o que ficou de fora.

O risco aqui não é errar um veredito — é o oposto do que a ferramenta existe para fazer: entregar um relatório correto que engana por omissão. Todas as decisões abaixo são sobre isso, ou sobre não estragar um contrato de linha de comando que já existe.

## Decisões

### O padrão continua `public`

`docs/PGFATHOM.md` documenta `--schema strings  schemas a analisar (padrão: public)`. Isso é decisão registrada, não lacuna de implementação, e a seção de carga do mesmo documento diz por quê: todo padrão é conservador, e o que for mais amplo ou mais pesado é opt-in explícito. O parágrafo sobre concorrência é literal quanto ao motivo — uma execução que pese num banco de produção alheio encerra o projeto, independentemente da qualidade do resto.

A alternativa considerada era inverter: sem `--schema`, varrer o banco inteiro. Descartada. Ela transforma o caso silencioso — rodar sem flag nenhuma, que é como qualquer pessoa experimenta uma ferramenta pela primeira vez — no caso mais caro possível contra um banco que não é dela. Numa ferramenta cujo requisito central é ser segura de apontar para produção de terceiro sem surpresa, o padrão precisa ser o comportamento que menos surpreende.

O documento vence sobre a implementação por regra própria dele. Mudar o padrão exigiria alterar aquela linha primeiro e justificar a inversão de postura; acrescentar uma flag não exige nada disso, porque não contradiz nada.

### `--exclude-schema` é flag nova, e não uma extensão de `--exclude`

`--exclude` casa hoje contra o nome nu da tabela e contra o qualificado, sempre dentro dos schemas em escopo. Fazer o mesmo padrão casar também contra nome de schema mudaria o significado de linhas de comando que já existem: quem hoje escreve `--exclude legacy` para pular uma tabela chamada `legacy` passaria, sem digitar nada diferente, a perder um schema inteiro de mesmo nome.

Mudança silenciosa de significado numa flag existente é a mesma falha da regra 4 vista de outro ângulo — o usuário continua acreditando numa coisa enquanto a ferramenta faz outra. Duas flags com um significado cada custam uma linha a mais no `--help` e não custam nada em ambiguidade.

Vale registrar o que já funciona, para que a flag nova não seja confundida com algo que ela não é: `--exclude 'vendas.*'` já pula toda tabela de `vendas` hoje. O que não existe é tirar o schema do **escopo** — com o filtro de tabela, o schema continua sendo consultado, entra no resultado como analisado e vazio, e as tabelas aparecem como excluídas por filtro. `--exclude-schema` remove o schema antes de qualquer leitura, e a cobertura passa a dizer "schema excluído" em vez de "847 tabelas excluídas", que é a afirmação verdadeira.

### `--schema` e `--all-schemas` são mutuamente exclusivas

Definir precedência entre as duas — qualquer que fosse — produziria uma linha de comando cujo escopo real não é o que ela aparenta pedir. `--schema vendas --all-schemas` não tem leitura óbvia: nem "só vendas" nem "tudo" é o que a linha diz.

Erro de uso, código 2, mensagem citando as duas flags. É a única resposta que não escolhe por conta própria entre duas intenções incompatíveis.

A detecção de "a flag foi fornecida" usa o estado de alteração da flag no cobra, não comparação com o valor padrão. Comparar com `["public"]` trataria `--schema public --all-schemas` como se `--schema` não tivesse sido passada, que é exatamente o caso ambíguo que a regra existe para pegar.

### O filtro de schemas exige `USAGE`, e a ausência dele não vira achado

A consulta de schemas visíveis filtra por `has_schema_privilege(..., 'USAGE')`. Sem isso, um schema que o papel enxerga no catálogo mas não consegue abrir entraria no escopo, e cada tabela dele cairia na lista de "sem privilégio de SELECT" — dezenas de linhas de ruído que se apresentam com a aparência de achado.

A distinção é entre não poder ver uma tabela num schema que se pode abrir, que é informação útil sobre um grant incompleto, e não poder abrir o schema, que é o papel simplesmente não tendo nada a ver com aquela parte do banco. A primeira é achado; a segunda é escopo.

### Schema sem tabela permanece no resultado

Um schema em escopo que não tem tabela nenhuma conta como analisado e produz entrada vazia no JSON. Filtrar seria mais limpo de ler e reintroduziria em miniatura o problema que a change corrige: o consumidor não teria como distinguir "esse schema não tem tabela" de "esse schema não foi olhado".

### A consulta de schemas roda mesmo sem `--all-schemas`

Ela custa uma ida ao catálogo por execução e, no modo padrão, não muda o escopo em nada. Roda assim mesmo porque é ela que produz a lista de schemas não analisados.

Essa lista é o que corrige a regra 4, e ela precisa aparecer justamente para quem **não** passou flag nenhuma — quem já sabe que precisa de `--all-schemas` não é quem está sendo enganado pelo relatório. Condicionar a consulta à flag entregaria a informação só a quem já não precisava dela.

### Cobertura separa "não foi pedido" de "foi excluído"

Dois campos de lista em vez de um, pelo mesmo motivo que `TablesExcluded` já é distinto de `TablesNoPrivilege`: exclusão pedida pelo usuário e ausência de pedido são fatos diferentes sobre a execução, e fundi-los faria o relatório afirmar intenção onde não houve nenhuma.

`Complete()` passa a exigir que não haja schema fora do escopo. Consequência pretendida: a única execução que pode se declarar completa é a que olhou tudo.

### Escopo vazio é erro de uso, não execução vazia

`--exclude-schema '*'`, ou excluir o único schema pedido, produz escopo vazio. A ferramenta falha com código 2 e mensagem distinguindo os dois caminhos que levam ali — lista vazia versus tudo excluído.

Executar e devolver um relatório sobre nada seria tecnicamente correto e praticamente indistinguível, para quem lê a saída, de um banco sem achado nenhum.

## Riscos

**O golden do contrato JSON quebra.** De propósito: é o mecanismo funcionando. Os quatro caminhos novos são acréscimo compatível e não movem `schema_version`, mas precisam passar por revisão consciente em vez de entrar de carona. O teste é regravado e o diff conferido campo a campo.

**`--all-schemas` amplia a carga contra o banco analisado.** Mais schemas significam mais tabelas, mais candidatos e mais anti-joins. As proteções que contêm isso — limite de concorrência, `statement_timeout`, `lock_timeout` — são por consulta e continuam valendo sem alteração, então o crescimento é em duração total, não em pico instantâneo. Ainda assim é a razão de a flag ser opt-in, e o texto de ajuda dela precisa dizer o que ela alcança.

**Schema de extensão entra no escopo.** PostGIS, pg_partman e afins criam schemas com tabelas reais, que `--all-schemas` vai analisar junto. Não é tratado por lista de exceções embutida: manter uma lista de nomes de extensão conhecidos envelheceria mal e falharia em silêncio no dia em que alguém instalasse a extensão seguinte. `--exclude-schema` é a resposta, e ela é do usuário porque só ele sabe o que no banco dele é aplicação e o que é infraestrutura.

## Fora de escopo

**Mudar o padrão para o banco inteiro.** Contradiz decisão registrada em `docs/PGFATHOM.md` e inverte a postura de padrão conservador. Se algum dia for desejável, começa por alterar o documento, não o código.

**Exclusão por padrão de coluna.** Escopo é de schema e de tabela; coluna é assunto de inferência, não de escopo.

**Perfil de nomenclatura por schema.** Um banco com schemas de origens diferentes poderia querer perfis diferentes por schema. É problema real e é outro problema — envolve detecção, precedência e apresentação, e não tem relação com resolver que conjunto de schemas entra na análise.

**Paralelizar a análise entre schemas.** `--all-schemas` aumenta o trabalho total, e a tentação é dividi-lo. Mas o limite de concorrência existe para proteger o servidor, não para ser contornado por um eixo novo de paralelismo.

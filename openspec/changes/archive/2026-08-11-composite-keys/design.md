## Context

Sete fases construíram a ferramenta em cima de uma premissa que nunca foi decidida, só herdada: uma relação liga uma coluna a uma coluna. Ela aparece na assinatura de `Candidate`, na query de validação que agrega por valor escalar, no nome de constraint que concatena tabela e coluna, no índice de alvos que exige `len(PrimaryKey) == 1`.

Em banco legado essa premissa é minoritária mais vezes do que parece. Chave composta é o que sobra de modelagem por chave natural — `(empresa, filial, numero)` em vez de um serial — e é a norma em sistema de gestão que atravessou décadas. Foi assim que 86 tabelas em 338 ficaram de fora de um único banco medido.

A restrição que domina esta change é a regra de nenhum falso positivo confirmado. Casar chave composta por nome é mais fácil e mais perigoso do que casar coluna única: `(empresa_id, filial_id)` existe em dezenas de tabelas de um ERP, e um casamento frouxo produziria relações inventadas em volume, cada uma com aparência de evidência forte.

## Goals / Non-Goals

**Goals**

Aridade atravessa a ferramenta com um caminho de código só. Alvo com chave composta vira alvo legítimo, sob regra estrita o bastante para que a aridade some confiança em vez de somar ruído. A semântica de NULL da constraint que a ferramenta emite é a mesma que a validação mede. O contrato JSON nasce na forma definitiva.

**Non-Goals**

Chave estrangeira apontando para constraint `UNIQUE` em vez da chave primária — legal em PostgreSQL, e fora daqui: a âncora continua sendo a chave primária, e ampliar as duas coisas na mesma change misturaria dois espaços de busca diferentes.

Casamento parcial promovido a candidato, sob qualquer heurística. Referência polimórfica, que continua sendo observação. Aridade como escopo do `audit`, que já lida com composta desde a fase 2.

Nenhuma calibração de peso ou limiar contra o corpus: isso é a change do benchmark, com os números na mão.

## Decisions

### `KeyRef` substitui `ColumnRef` no candidato, em vez de conviver com ele

A alternativa barata era manter `Child ColumnRef` e acrescentar `ChildColumns []string` preenchido só no caso composto. Ela é pior de todos os jeitos: dois campos que dizem a mesma coisa em regimes diferentes, dois caminhos em cada consumidor, e a garantia de que algum deles vai ler o campo escalar num candidato composto e produzir uma DDL com metade da chave.

`model.KeyRef{Schema, Table, Columns []string}` com aridade 1 como caso comum resolve por construção. Cada consumidor escreve uma vez o laço sobre `Columns`, e o cenário de coluna única passa a ser exercitado pelo mesmo código que o composto — o que significa que a suíte inteira que já existe vira teste de regressão da forma nova.

`ColumnRef` continua existindo e continua certo onde o assunto é uma coluna: estatística de planner é por coluna, e igualdade extraída de SQL liga duas colunas.

### Casamento total, uniforme e sem desempate

Para cada coluna da chave primária alvo, o lado filho precisa oferecer exatamente uma coluna correspondente, por uma de duas derivações: **espelho**, em que a coluna filha tem o nome da coluna da chave (`empresa_id` casa `empresa_id`), e **prefixada**, em que o nome da entidade alvo precede a coluna da chave (`nota_empresa_id` casa `empresa_id` na tabela `nota`). As formas do perfil ativo são o que decide se um nome é a entidade alvo, exatamente como na aridade 1.

Três regras fecham a porta ao ruído:

**Total.** Faltou uma parte, não há candidato. Uma chave estrangeira que casa em duas de três colunas não é uma chave estrangeira com uma coluna faltando — é outra coisa, e propor a DDL parcial seria propor uma constraint que rejeita linhas válidas.

**Uniforme.** A derivação escolhida vale para todas as partes. Misturar espelho numa coluna e prefixo em outra é o formato exato de uma coincidência: duas colunas com nomes comuns que por acaso existem na mesma tabela.

**Sem desempate.** Se alguma parte casa com mais de uma coluna filha, o alvo é pulado com nota em vez de resolvido por posição, por tipo ou por ordem. Escolher ali seria um palpite com aparência de decisão, e a ferramenta não tem um só lugar onde faça isso.

Falta uma quarta, que a implementação expôs: **espelho exige alvo único**. Espelho é a única derivação sem âncora de nome — nada no filho diz para onde ele aponta, a assinatura da chave é a evidência inteira. Com duas tabelas respondendo pela mesma assinatura, os dois candidatos seriam validados por conta própria e **ambos podem alcançar contenção total**, porque a mesma tupla existe nos dois lados. Confirmar os dois é confirmar um relacionamento que não existe. Não há com o que desempatar, então nada é emitido. Prefixado não cai nisso: o nome está na coluna, a ambiguidade é a comum, e segue penalizada como na aridade 1.

O parcial vira observação, com a fração que casou. É onde o recall perdido fica legível — e é o insumo de calibração da change do benchmark, que sem isso teria que adivinhar por que uma FK não voltou.

### Aridade é um sinal, não `n` sinais

A tentação é emitir os sinais existentes uma vez por coluna. Seria errado por duas razões. A soma satura no mesmo teto, então a aridade viraria uma forma indireta de fixar o score no máximo; e o score deixaria de ser explicável, porque a lista de sinais de uma chave de quatro colunas teria vinte entradas dizendo quatro fatos.

Um sinal de concordância por aridade, emitido uma vez, com peso que cresce sublinearmente. Os demais sinais continuam um por candidato, avaliados sobre a chave inteira: tipo compatível significa todas as colunas compatíveis, indexada significa índice cujas colunas iniciais são a chave na ordem dela, `NOT NULL` significa todas as colunas não nulas.

### O pré-filtro estima tupla por limite inferior

`pg_stats` tem `n_distinct` por coluna e não tem cardinalidade de tupla — isso exigiria `CREATE STATISTICS`, que é DDL, e a ferramenta não emite DDL. Mas há uma desigualdade que vale sempre: `distinto(tupla) ≥ máx distinto(coluna)`.

Isso é suficiente para a única coisa que a camada faz: rejeitar o aritmeticamente impossível. Se o limite **inferior** já excede a margem sobre as linhas do pai, nenhuma estatística melhor salvaria o candidato. O que se perde é sensibilidade — composta impossível que só se revela na combinação passa —, e perder sensibilidade nesta camada custa um anti-join a mais, enquanto ganhar sensibilidade por estimativa inventada custaria a regra de nunca rejeitar sem base.

A faixa é checada par a par e penaliza uma vez só, pela mesma razão do sinal de aridade: uma chave disjunta em três colunas é um fato, não três.

### `MATCH SIMPLE` é medido, e o que ele isenta é reportado

`FOREIGN KEY (a, b)` sem cláusula `MATCH` é `MATCH SIMPLE`: se **qualquer** coluna da chave for NULL, a linha é isenta da verificação. É o padrão, e é o que a ferramenta emite.

Então a validação filtra por todas as colunas não nulas. Contar uma linha parcialmente nula como órfã produziria veredito de quebrada que a DDL emitida não corrobora — a constraint criaria sem reclamar da linha que a ferramenta apontou como violação. Um veredito que a própria DDL desmente é a pior forma de errar disponível aqui.

Essas linhas passam a ser contadas e reportadas. `(nota_id=7, item_id=NULL)` escapa da integridade referencial por regra do padrão SQL, silenciosamente, e é a espécie de coisa que este projeto existe para dizer em voz alta. É contagem na mesma passada, sem query nova.

### O anti-join por tupla passa a ter um dono

`report.violationQuery` já monta anti-join por tupla com isenção de NULL, para as constraints `NOT VALID` do `audit`. A query de órfãos do `discover` faz o mesmo para aridade 1. Depois desta change as duas são a mesma coisa, então viram a mesma função, parametrizada pelas colunas — pelo mesmo motivo que `tableRows` deixou de existir em duas cópias na change anterior: regra duplicada é divergência agendada.

## Risks / Trade-offs

**Falso positivo confirmado por nome comum em volume** — a preocupação central. `(empresa_id, filial_id)` casa com dezenas de tabelas num ERP → mitigado pelo casamento total e uniforme, pelo alvo ambíguo continuar penalizado e não resolvido, e pela validação continuar sendo quem confirma. O corpus da change seguinte é onde isso deixa de ser argumento e vira medida: se aparecer um confirmado falso, ele é bug bloqueante antes do release.

**Explosão combinatória na geração** → o casamento é ancorado na chave do alvo, não no produto das colunas da filha: para cada alvo, cada coluna da chave resolve por consulta direta ao índice de colunas da filha. O custo é linear na aridade, não exponencial.

**Regressão silenciosa no caminho de aridade 1** → é a metade da suíte que já existe. Golden file de cenário só-coluna-única que mudar é bug, e o único golden que pode mudar é o do JSON, pela forma de `child` e `parent`.

**A validação de composta é mais cara** — agrupar por tupla é mais caro que agrupar por escalar, e a filha de chave composta costuma ser tabela de movimento → o teto de tempo por query e a amostragem já bounded o custo, e o efeito real entra na medição de tempo por etapa da change do benchmark. Se doer, dói com número.

**Nome de constraint estoura o orçamento de identificador mais cedo** — `fk_` mais tabela mais quatro colunas passa de 63 bytes com facilidade → o truncamento com sufixo de hash já existe desde a fase 7 e já avisa no arquivo quando corta.

## Migration Plan

Não aplicável: nada foi lançado. É por isso que a forma composta entra agora e `schema_version` continua `"1"`.

## Open Questions

Nenhuma bloqueante. Os pesos do sinal de aridade entram como estimativa declarada como tal, revistos com o corpus na change seguinte, pelo mesmo caminho do limiar de score e da margem do pré-filtro.

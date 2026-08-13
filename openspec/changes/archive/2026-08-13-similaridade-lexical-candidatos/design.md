## Métrica de similaridade

Coeficiente de Dice sobre conjuntos de trigrama de caractere, case-insensitive:

```
similaridade(a, b) = 2 × |trigramas(a) ∩ trigramas(b)| / (|trigramas(a)| + |trigramas(b)|)
```

Trigramas extraídos com padding de borda — duas posições à esquerda, uma à direita, mesma
convenção do `pg_trgm` — para que prefixo e sufixo curtos ainda produzam trigrama e a
similaridade não fique artificialmente baixa em nomes de poucas letras. String vazia de
qualquer lado retorna 0 diretamente, sem passar pelo cálculo.

**Por que Dice sobre trigrama, e não Jaro-Winkler ou os dois combinados:** a análise anterior
desta sessão levantou as duas opções lado a lado. Trigrama generaliza melhor para reordenação
(`idkey_operador` vs `operadorbasecalculo`, `atorevogacao` vs `ato`), que é o padrão dominante
nos três exemplos documentados do corpus pt-BR. Jaro-Winkler favorece prefixo comum e
generaliza melhor para erro de digitação, que não é o padrão observado aqui. Manter as duas e
tomar o maior adicionaria uma segunda métrica, um segundo conjunto de testes e uma segunda
constante de calibração para um ganho não demonstrado nos casos reais que motivam esta change
— contraria o princípio do projeto de não adicionar superfície sem medição que justifique.
Se o corpus revelar depois que a métrica não generaliza para outro padrão de abreviação, a
segunda métrica entra como change própria, com o número que a motivou.

**Por que não `pg_trgm` da extensão:** exigiria a extensão instalada no banco-alvo — que o
produto não controla e não pode instalar (regra 1, read-only absoluto) — para uma conta que
strings de poucos caracteres resolvem em microssegundos dentro do próprio binário. GitLab, o
maior schema do corpus, já lista `pg_trgm` entre as extensões que o **schema em si** precisa;
não é razão para o `pgfathom` depender dela para o próprio cálculo interno.

## Dois limiares, dois papéis

Este change introduz um limiar nomeado com cuidado para não colidir com o que já existe:

- **Limiar de similaridade** (`MinNameSimilarity`, padrão 0.30 — ver correção abaixo) — decide
  se a via de fallback **gera** candidato para aquele par coluna/tabela. Comparável ao papel
  que `Profile.TableForms` já cumpre na via por afixo: decide existência, não pontuação.
- **Limiar de pontuação** (`MinScore`, já existente, `DefaultMinScore = 0.5`) — decide se o
  `MetaScore` final, somando todos os sinais do candidato, sobrevive até a validação. Não
  muda nesta change, e vale por igual para candidato de qualquer origem.

**Correção feita durante a implementação, não na análise que motivou esta change:** o valor de
partida discutido antes de medir era 0.65. Rodando `TrigramSimilarity` contra os três pares
reais do corpus que motivam esta change, os três ficam abaixo disso —
`operador`/`operadorbasecalculo` = 0.552, `tptramite`/`tramitetipo` = 0.545,
`atorevogacao`/`ato` = 0.353. Um limiar de 0.65 geraria candidato para nenhum dos três casos
que a change existe para alcançar — a estimativa anterior era um chute sem medição, e a
medição chegou antes da entrega em vez de depois.

Baixar o limiar de geração para 0.30 (abaixo do menor dos três, com margem) é seguro pelo
mesmo motivo que `Generate` já documenta para o resto do pipeline: "gera liberalmente e corta
estritamente" (`internal/infer/generate.go`, doc de `Generate`). O limiar de similaridade não
precisa fazer o trabalho de proteção contra ruído sozinho — o `SigNameSimilarity` tem peso
baixo (teto 0.12) e só empurra um candidato além do limiar de pontuação (0.5) se os outros
sinais (tipo idêntico, alvo único, not null) já contribuírem a maior parte; um candidato fraco
nos dois eixos continua caindo no corte de pontuação e aparece em `Discarded`, nunca em
silêncio. Quem protege contra confirmação errada continua sendo a validação contra dado, não
este limiar.

Um candidato pode cruzar o limiar de similaridade e ainda cair no de pontuação, se os demais
sinais forem fracos (tipo só compatível, alvo ambíguo, sem índice) — comportamento idêntico ao
que já acontece hoje na via por afixo com casamento normalizado fraco.

## Peso do sinal: fixo no teto, proporcional na prática

`SigNameSimilarity` usa peso graduado, não fixo — `nameSimilarityWeight(score) =
weightNameSimilarityMax × score` —, seguindo o mesmo padrão que `arityWeight` já estabelece em
`score.go` para sinais cujo peso depende de uma medida contínua, não de um fato binário.

`weightNameSimilarityMax` fica abaixo de `weightNormalizedName` (0.15): mesmo no teto (nome
idêntico ao candidato mais lexicalmente próximo possível), a evidência de similaridade é mais
fraca que uma convenção de nomenclatura confirmada pelo perfil, porque não carrega a mesma
garantia de padrão — é proximidade de string, não regra de linguagem. Valor de partida: 0.12.
Como todo peso deste arquivo, é estimativa a recalibrar com o corpus, não medição.

## Onde a via de fallback entra em `generateFor`

`generateFor` já resolve, para a via por afixo, a sequência index → filtro de aridade/chave →
compatibilidade de tipo → ambiguidade → sinais. A via por similaridade entra **só** quando
`index[entity]` vem vazio — nunca quando vem não-vazio e todo mundo é descartado por aridade
ou ausência de chave, porque nesse caso o `Skip` já registrado explica o motivo e uma segunda
tentativa por outro caminho, mirando talvez uma tabela diferente, arrisca confundir "por que
essa tabela foi ignorada" com "por que essa outra apareceu do nada".

A parte de filtro de aridade/chave/tipo é compartilhada entre as duas vias via um helper
extraído (`resolveTarget`), para não duplicar a lógica que já existe. `buildSignals` passa a
receber o sinal de nome já pronto, montado pelo chamador de acordo com a via — exato/normalizado
para a via por afixo (lógica que sai de dentro da função, mas não muda de comportamento),
`SigNameSimilarity` para a via nova. O resto de `buildSignals` (tipo, ambiguidade, índice,
comentário, not-null, domínio genérico) é idêntico para as duas vias — a via de origem do
sinal de nome não muda nada além de qual primeiro sinal entra.

## Custo não medido, registrado como risco aceito

A via por afixo consulta um índice pré-computado (`map[string][]indexedTarget`) — custo
praticamente constante por coluna. A via de similaridade, quando ativada, compara contra
**todas** as tabelas do schema em escopo — custo linear no tamanho do schema, por coluna sem
casamento por afixo. Em schema grande com muitas colunas sem casamento (o padrão que motiva
esta change), o produto desses dois números não foi medido.

Estimativa de ordem de grandeza, não medição: GitLab (corpus, 1054 tabelas) — se algumas
centenas de colunas caírem na via de fallback, o total fica na casa de dezenas a centenas de
milhares de comparações de trigrama, cada uma da ordem de microssegundos para strings de
poucas dezenas de caracteres. Compatível com os orçamentos de tempo já registrados em
`docs/benchmark/cost.md` (candidatos gerados hoje em ~150-160ms para o schema inteiro), mas
"compatível na estimativa" não é "medido" — `make benchmark` fica como encaminhamento explícito
em `tasks.md`, a rodar antes de qualquer recalibração de limiar.

## Why

O corpus fez o que corpus serve para fazer: mediu a fase anterior e disse que ela não alcança o caso que a justificou. Das 53 chaves compostas do GitLab, **zero** voltaram. Todas têm a mesma forma:

```
ci_build_pending_states (partition_id, build_id)  →  p_ci_builds (id, partition_id)
```

Uma posição casa por espelho, `partition_id` contra `partition_id`. A outra casa pela regra de nome da aridade 1, `build_id` apontando para `id`. A change de chave composta proibiu misturar derivações, com o argumento de que mistura é o formato de uma coincidência. O argumento estava errado, e o erro tem nome: **discriminador mais referência** não é coincidência, é a forma canônica de chave composta em schema particionado ou multi-tenant. Foi a regra, não a ferramenta, que não alcançou.

Há um segundo obstáculo, e ele é de outra natureza. `entity("build_id")` dá `build`; as formas de `p_ci_builds` são `p_ci_builds` e `p_ci_build`. Falta atravessar o namespace do nome da tabela. A detecção de prefixo não resolve isso e não deveria tentar: o maior prefixo de primeiro segmento do GitLab é `ci_`, em 61 de 1.085 tabelas — 5,6%, contra os 26% que fizeram a detecção valer a pena no banco municipal. Uma detecção que dispara a 5,6% cria casamento falso nos outros 1.024 nomes.

Tabela com namespace — `<módulo>_<entidade>` — é padrão de ecossistema inteiro: `auth_user` no Django, `ci_builds` no GitLab, o que qualquer engine Rails produz. Parecia regra de casamento faltando. Medida, mostrou-se inalcançável sem palpite, e o que sobra é dizer isso.

## What Changes

- **Derivação ancorada** substitui a regra de derivação uniforme. Cada posição da chave resolve por espelho ou pela forma prefixada, e o conjunto exige **pelo menos uma âncora**: uma posição cujo nome carregue o nome do alvo. A regra fica mais simples e cobre mais: casamento todo prefixado continua valendo, e espelho puro — que não tem âncora nenhuma — continua exigindo alvo único.
- **Alvo com namespace: medido e recusado.** A regra seria casar o nome de entidade contra o segmento final do nome da tabela, sob a guarda de alvo único. Medida antes de ser escrita, ela alcança **0 das 53** chaves compostas do GitLab: as 13 tabelas alvo têm todas o segmento final disputado — `builds` por 6 tabelas, `runners` por 5, `requests` por 7 — e a guarda que a tornaria segura elimina todas. Fica registrada com o número, e não entra.
- A fixture que codificava o julgamento errado é corrigida. `frete(empresa_id, nota_numero) → nota(empresa_id, numero)` estava lá como armadilha por misturar derivações; sob a regra nova ela é exatamente o padrão, e o que continua sendo armadilha é o que nunca teve âncora.
- Medição no corpus para cada regra em separado. A que não pagasse não ficaria — e uma delas não pagou.

## Capabilities

### Modified Capabilities

- `candidate-generation`: a derivação ancorada substitui a uniforme.

## Impact

Nenhuma dependência nova, nenhuma flag nova, nenhuma mudança de contrato.

O raio das duas regras era bem diferente, e é por isso que foram medidas em separado. A derivação ancorada mexe só na geração composta, que produzia zero: não havia como piorar. O casamento com namespace mexeria na aridade 1, que é a maior parte de tudo que a ferramenta produz — e a medição disse para não mexer, poupando a mudança de maior raio do projeto em troca de um ganho de zero.

O risco continua sendo o de sempre: candidato demais é anti-join demais contra o servidor de alguém, e casamento frouxo é como um falso positivo confirmado nasce. Contra isso, as duas regras mantêm as recusas que já existiam — âncora ambígua pula, alvo compartilhado pula, posição sem correspondente derruba o conjunto — e o corpus passa a ser onde a conta é conferida em vez de argumentada.

Esta é a terceira das quatro changes da fase 8, e existe porque a segunda mediu a primeira. O saldo no corpus é modesto e honesto: 1 chave de 53, porque o que bloqueia as outras 52 não é regra de casamento, é o nome do alvo não guardar semelhança alcançável sem palpite. É a mesma fronteira que o README já declara sobre o resto do recall, agora com número em cima da chave composta também.

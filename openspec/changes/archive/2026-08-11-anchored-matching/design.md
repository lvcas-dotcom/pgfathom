## Context

Duas regras da fase de chave composta foram escritas antes de existir corpus para medi-las. O corpus mediu, e as duas erraram por motivos diferentes.

A regra de derivação uniforme errou por raciocínio. Ela partiu de um caso real — duas colunas de nome comum na mesma tabela casando por acaso — e generalizou dele para "misturar derivações é coincidência". A generalização engoliu junto o padrão mais comum de chave composta que existe.

A segunda parecia erro de omissão: o caminho de aridade 1 sempre exigiu que o nome de entidade casasse com alguma forma do nome inteiro da tabela, e schema segmentado por módulo — `auth_user`, `ci_builds` — nunca satisfaz isso. Medida, mostrou-se outra coisa, e esta change registra o quê.

## Goals / Non-Goals

**Goals**

Alcançar a forma discriminador-mais-referência, que é o que chave composta quase sempre é. Decidir sobre o namespace com número em vez de intuição. Medir cada regra em separado, no corpus, antes e depois.

**Non-Goals**

Nenhuma mudança na detecção de nomenclatura. Ela está certa como está: dispara sobre convenção de schema, e 5,6% não é convenção.

Nenhuma mudança em pontuação, limiar, validação ou saída. Nenhuma tentativa de recuperar as 727 chaves do GitLab que não voltaram por outras razões — o que não for explicado por estas duas regras fica para ser medido depois, não adivinhado agora.

## Decisions

### Âncora substitui uniformidade, e a regra fica menor

A regra antiga era: toda posição resolve pela mesma derivação. A nova é: toda posição resolve por espelho ou pela forma prefixada, com a mesma forma em todas as prefixadas, e **pelo menos uma** posição precisa ser prefixada.

A posição prefixada é a âncora — é ela que carrega o nome do alvo, e é ela que responde "por que esta tabela e não outra". As posições em espelho são discriminadores: `partition_id`, `empresa_id`, `tenant_id`, a coluna que atravessa o schema inteiro e não aponta para nada sozinha.

A regra nova é estritamente mais permissiva que a antiga em um ponto e idêntica no resto: casamento todo prefixado tem `n` âncoras e continua valendo; casamento todo espelho tem zero âncoras e continua sendo o caso sem âncora, que já exigia alvo único justamente porque não tem como dizer para onde aponta. O que muda é o meio do intervalo, que era o buraco.

O que se perde é a recusa daquela fixture: `frete(empresa_id, nota_numero) → nota(empresa_id, numero)` passa a casar. Olhando de novo, ela deveria: `nota_numero` nomeia o alvo, `empresa_id` é o discriminador, e é a mesma estrutura das 53 do GitLab. O que restava de verdadeiro no medo original — duas colunas comuns sem nada apontando para o alvo — é exatamente o caso sem âncora, e continua barrado.

### Namespace pelo segmento final: medido antes de escrito, e recusado

A regra proposta era aceitar que o nome de entidade correspondesse ao **segmento final** do nome da tabela, sob a mesma guarda do espelho: mais de uma tabela terminando naquele segmento, nenhuma é escolhida.

Ela foi medida contra a lista real de tabelas antes de existir uma linha de código, e alcança **0 das 53** chaves compostas do GitLab. As 13 tabelas alvo têm todas o segmento final disputado:

| Alvo | Chaves | Segmento | Tabelas que terminam nele |
|---|---:|---|---:|
| `p_ci_builds` | 21 | `builds` | 6 |
| `p_ci_pipelines` | 15 | `pipelines` | 5 |
| `p_ci_job_artifacts` | 3 | `artifacts` | 3 |
| `*_ci_runners` | 7 | `runners` | 5 |
| demais | 7 | vários | 2 a 8 |

Remover a guarda para alcançá-las significaria escolher `p_ci_builds` em vez de `ci_pending_builds` por decreto — o palpite que o projeto inteiro recusa. Mantendo a guarda, a regra custa complexidade na aridade 1, que é o maior raio do projeto, e paga zero.

Então ela não entra. O que resta é declarar a fronteira: `p_ci_builds` não é alcançável a partir de `build_id` por nenhuma regra de nome que não adivinhe, e isso é a mesma fronteira que o README já descreve para o resto do recall — nomenclatura que não guarda semelhança é invisível ao casamento de nome por construção.

A alternativa de ensinar a detecção a remover `ci_` foi recusada pelo mesmo caminho: `ci_` cobre 5,6% das tabelas do GitLab, contra os 26% de `tpl_` que fizeram a detecção existir. A 5,6% não se lê convenção, lê-se o maior grupo de um schema temático, e o custo cai sobre os 94% restantes.

### Cada regra medida sozinha, e uma delas morreu na medição

A ordem foi: linha de base registrada, uma regra aplicada, medida, e só então a outra. Não é cerimônia — é a primeira vez no projeto que existe onde conferir, e mexer em duas coisas ao mesmo tempo desperdiçaria exatamente isso.

Deu certo no sentido menos glamouroso possível. A ancorada entrou e valeu uma chave. A de namespace foi medida sobre a lista de tabelas antes de ser escrita e valeu zero, então não foi escrita. Dez minutos de contagem no lugar de um dia de implementação e revisão de uma mudança que mexeria no caminho de maior raio do projeto.

## Risks / Trade-offs

**Falso positivo confirmado pela derivação ancorada** — uma âncora mais discriminadores é evidência mais fraca do que a chave inteira nomeando o alvo → as recusas continuam onde estavam, e a validação continua sendo quem confirma. O critério segue medido nas fixtures, onde o gabarito é construído.

**A fixture perde uma armadilha** → ela é substituída por outra que exercita o que sobrou de verdadeiro no medo original: colunas comuns, nenhuma âncora, mais de um alvo possível.

**Duas regras numa change** → medidas em separado, com o número de cada uma registrado. Uma pagou e ficou; a outra não pagou e não foi escrita.

## Migration Plan

Não aplicável.

## Open Questions

As 727 chaves do GitLab que não voltaram por nenhuma destas duas razões continuam sem diagnóstico. Sabe-se o total, não a decomposição. Levantá-la é trabalho de medição, não de adivinhação, e o harness agora existe para isso.

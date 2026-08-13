## Why

O casamento de nome hoje (`internal/infer.generateFor`) só gera candidato quando a forma
normalizada da coluna (`Profile.EntityName`) bate, literalmente ou por plural, com uma das
formas indexadas de alguma tabela (`Profile.TableForms`). Quando não bate, a função não gera
candidato e não registra nada — o único ponto de silêncio que resta no produto, cuja regra 4
(`openspec/project.md`) é justamente que silêncio nunca é ausência de problema.

O corpus público já documenta essa lacuna com exemplos reais (`docs/PGFATHOM.md`, "O que a
gap remanescente é"):

```
atotramite.tptramite_idkey        → tramitetipo          abreviado, reordenado
basecalculo.idkey_operador        → operadorbasecalculo  nomeado pelo papel
ato.atorevogacao_idkey            → ato                  nomeado pelo papel
```

Nos três, o perfil stripa o afixo corretamente (a entidade não sai vazia), mas a forma
resultante não é literal nem plural de nenhuma tabela — é abreviação ou reordenação de
caracteres. É sobreposição lexical, não semântica: um cálculo de distância determinístico
sobre as strings resolve os três, sem precisar de modelo de linguagem, sem rede, sem
dependência nova.

Fica fora de escopo o caso genuinamente semântico, sem sobreposição de caracteres —
`os_servico.resp_tecnico → funcionario.id`, o exemplo que abre o próprio README. Esse caso já
tem via própria no produto: evidência de uso (`internal/sqlprobe`), que prova a relação lendo
o `JOIN` real em vez de adivinhar por proximidade de string. Não é reimplementado aqui.

## What Changes

- Novo sinal `SigNameSimilarity` (`internal/model`), emitido quando a coluna casa com uma
  tabela por similaridade lexical de trigrama de caractere, em vez de por afixo/plural do
  perfil.
- Nova via de geração em `internal/infer.generateFor`, ativada **só** quando o índice do
  perfil não encontra nenhuma tabela para a entidade extraída da coluna — nunca quando encontra
  e descarta por chave composta ou ausência de chave primária, caso em que o `Skip` já
  existente explica o motivo.
- Cálculo de similaridade puro, sem estado, sem I/O: coeficiente de Dice sobre conjuntos de
  trigrama de caractere, mesma família de técnica do `pg_trgm`, recalculada em Go — sem
  precisar da extensão instalada no banco-alvo.
- Candidato nascido dessa via passa pelo mesmo pipeline de validação de tipo, aridade e
  ambiguidade que a via por afixo já usa, e pelo mesmo limiar de pontuação final
  (`DefaultMinScore`) que qualquer outro candidato — nenhuma mudança na garantia de "nenhum
  falso positivo confirmado" (regra 5): quem confirma continua sendo a validação contra dado,
  não o sinal de nome.

## Capabilities

- `candidate-generation` — MODIFIED: nova via de casamento, condicionada à ausência de
  qualquer casamento por perfil.
- `candidate-scoring` — MODIFIED: novo sinal de nome e sua faixa de peso, mais fraca que o
  casamento normalizado por perfil.

## Impact

- Sem dependência nova no `go.mod` — cálculo de trigrama é string pura, biblioteca padrão.
- Custo: a via de fallback varre todas as tabelas do schema em escopo para cada coluna sem
  casamento por perfil, o que multiplica o número de comparações em relação à via por afixo
  (que consulta um índice pré-computado). Cada comparação é barata (strings curtas), mas o
  efeito agregado em schema grande (o corpus GitLab tem 1054 tabelas) não foi medido nesta
  change — `make benchmark` fica registrado como encaminhamento em `tasks.md`, não bloqueia
  esta entrega.
- Nenhuma mudança em `internal/validate`, `internal/stats`, `internal/report` — o candidato
  novo é indistinguível dos demais a partir do momento em que sai de `internal/infer`.

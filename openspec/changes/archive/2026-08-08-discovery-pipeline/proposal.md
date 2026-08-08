## Why

Sete fases depois, a execução completa do `discover` — conexão, catálogo, detecção de nomenclatura, evidência de uso, geração, pré-filtro, validação, montagem do resultado — mora dentro de uma função de comando de 131 linhas em `internal/cli`. Cada fase acrescentou um estágio ali, e cada estágio é uma linha consecutiva em vez de uma peça com fronteira.

Isso passa a custar exatamente agora. A fase 8 precisa publicar tempo por etapa, memória e número de queries de validação — são os números que sustentam o lançamento. Não existe onde instrumentar: não há costura entre os estágios. E o harness do benchmark precisa executar o pipeline **em processo**, para medir o funil de candidatos por etapa; hoje ele teria que passar pelo binário e reparsear a saída, ou reimplementar a orquestração e medir uma coisa diferente da que o usuário roda.

Junto disso, três duplicações que já divergem entre cópias. A que preocupa é `plantedValues`: a lista de valores que as fixtures plantam para a varredura de vazamento existe em quatro pacotes de teste. Uma cópia que não conhece um valor novo passa vazia e **parece limpa** — falha silenciosa exatamente na regra que proíbe dado do usuário sair.

## What Changes

- `internal/discovery`: a execução do `discover` como unidade própria, com entrada programática. Recebe pool, escopo e opções; devolve o `model.Result` pronto e os detalhes que o relatório precisa. Cada estágio vira uma etapa nomeada, o que dá à fase 8 onde medir sem reabrir esta camada.
- `internal/cli` volta ao que é: validar flags, montar opções, chamar, renderizar.
- `tableRows` deixa de existir em duas cópias; a resolução de `reltuples` com a sentinela de "nunca analisada" passa a ter um dono só, no `model`.
- Helpers de teste centralizados em `internal/testutil`: **uma** lista de valores plantados, e o verificador de que uma camada só consulta catálogo.
- `Querier` uniforme nas quatro camadas de I/O: `catalog` e `validate` passam a aceitar a mesma interface estreita que `sqlprobe` e `stats` já aceitam.

**Nenhuma mudança de comportamento.** Mesmas flags, mesma saída, mesmos vereditos, mesmos artefatos. A prova é a suíte existente inteira — incluindo os golden files, que falham ao primeiro byte diferente.

## Capabilities

### Modified Capabilities

- `cli-foundation`: acrescenta o requisito de que a execução seja invocável fora do comando, que é o que torna a medição da fase 8 possível e reproduzível por terceiros.
- `domain-model`: acrescenta o requisito de que a resolução da estimativa de linhas tenha um único dono, para que a sentinela de tabela nunca analisada não possa ser reinterpretada como zero em uma camada e como desconhecida em outra.

## Impact

Nenhuma dependência nova. `internal/discovery` importa as camadas que já existem; a direção do grafo não muda, só ganha um nível de composição acima delas.

O risco desta change é regressão silenciosa em refactor — e é contra isso que ela não escreve teste de comportamento novo: qualquer diferença tem que aparecer nos testes que já existem. Golden file que muda é bug, não atualização.

A alternativa era fazer a extração dentro da fase 8, junto do harness. Foi recusada porque misturaria mudança estrutural com a medição que ela viabiliza: se um número do benchmark saísse estranho, não haveria como saber se o problema é o harness ou a costura recém-criada embaixo dele.

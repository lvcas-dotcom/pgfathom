## Context

O `discover` acumulou vinte e uma flags em sete fases, e cada uma delas foi acrescentada por um motivo defensável. O resultado é uma ferramenta que faz o que precisa e que não se aprende por tentativa.

O contexto de uso agrava. Quem roda isto pela primeira vez está diante de um banco que não conhece, com permissão emprestada e alguém esperando resultado. O modo de falha mais comum não é erro: é apontar para o schema errado e concluir que o banco não tinha nada a descobrir. Aconteceu esta semana, com o autor do projeto, contra um banco real — e só não terminou em desistência porque o bloco de cobertura listou os 60 schemas não analisados.

## Goals / Non-Goals

**Goals**

Levar alguém do zero a uma execução correta sem ler documentação. Ensinar as flags no caminho, em vez de escondê-las. Nunca agir sem mostrar antes o que vai acontecer.

**Non-Goals**

Substituir as flags. O guia compõe um comando; ele não é um modo de operação paralelo com opções próprias.

Cobrir o `audit`, que tem superfície pequena e não precisa de guia.

Persistir configuração em arquivo. Um arquivo de config é superfície nova, com precedência para definir e migração para manter, e o comando impresso resolve o mesmo problema com um copiar e colar.

## Decisions

### Subcomando, e não o comportamento sem argumento

`pgfathom` sem argumento imprime a ajuda, e isso é requisito escrito em `cli-foundation`. Trocar por um guia seria decisão separada, com efeito sobre quem já usa o comando em script.

`pgfathom setup` é descobrível pela ajuda, e não surpreende ninguém.

### O guia termina imprimindo o comando

É a decisão que define o desenho inteiro. O objetivo não é poupar a pessoa das flags, é ensiná-las: ela vê o comando que suas respostas compuseram, copia, e da segunda vez não precisa do guia.

Isso também mantém o guia honesto. Nada acontece que a pessoa não possa reproduzir sozinha, e nenhuma opção fica escondida atrás de uma pergunta amigável.

### Mostra antes de executar, e pergunta

Executar ao final é conveniente e é o que se espera. Executar sem mostrar seria a ferramenta agindo por conta própria contra um banco de produção — o oposto do que o projeto faz em todo o resto, onde até a DDL confirmada sai comentada para revisão.

Então: monta, mostra o comando, pergunta, executa se a resposta for sim.

### A lista de schemas vem do servidor, com contagem

É o que resolve o beco real. Um servidor com 61 schemas e 6 tabelas no padrão não é caso exótico em gestão pública, é o normal — e a informação que desfaz o engano é uma linha por schema com o número de tabelas.

A leitura é do catálogo, pela mesma conexão somente leitura de sempre.

### `bubbletea` sim, `bubbles` não

O custo medido: 9 para 28 módulos, 10,4 para 15,2 MB. Aceito, registrado no `project.md`, e é a maior concessão de superfície que o projeto já fez.

`bubbles` custaria 3 módulos a mais e traria `atotto/clipboard`, por causa do campo de texto. Uma ferramenta de banco somente leitura que carrega biblioteca de área de transferência gera uma pergunta cuja resposta honesta — dependência transitiva — não tranquiliza quem está decidindo se aponta isso para produção. Lista e campo de texto são cerca de oitenta linhas sobre os eventos de tecla que o `bubbletea` já entrega.

### Exige terminal, e falha dizendo isso

Em pipe ou CI, um guia interativo espera entrada que nunca chega e trava o processo. Então ele verifica o destino na entrada e recusa com mensagem clara.

É a mesma detecção que governa realce e progresso, aplicada a um comando que não tem modo degradado: sem terminal, não há guia.

## Risks / Trade-offs

**O guia vira a única porta e as flags viram folclore** → ele imprime o comando composto, e é a última coisa que faz antes de perguntar. Quem passou uma vez não passa de novo.

**Superfície de dependência triplicada** → medida, registrada no `project.md` com a razão, e confinada ao subcomando: nenhuma camada de leitura, inferência, validação ou relatório importa interface.

**Binário 47% maior** → é o custo, e está publicado. Quem instala por pacote ou imagem não percebe; quem baixa binário vê 15 MB em vez de 10.

**Guia que executa contra produção** → mostra o comando e pergunta antes, e o que ele executa é o mesmo `discover` somente leitura de sempre.

## Migration Plan

Não aplicável: comando novo.

## Open Questions

Nenhuma.

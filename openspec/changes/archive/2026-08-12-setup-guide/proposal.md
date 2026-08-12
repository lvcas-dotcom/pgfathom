## Why

`pgfathom discover` tem vinte e uma flags. Elas existem porque cada uma resolve um problema real — escopo, modo de validação, tetos de tempo, o que desligar quando a estatística mente —, e juntas formam a superfície que separa quem já leu o README de quem acabou de baixar.

A primeira execução de alguém acontece no pior contexto possível: contra um banco que não é dele, com a pessoa que autorizou olhando por cima do ombro, sem tempo de ler documentação. E ela quase sempre começa errada. Nesta semana, apontar a ferramenta para um banco municipal real devolveu "nenhum candidato acima do limiar" — porque o schema padrão tinha 6 tabelas e as 226 que importavam estavam num dos outros 60 schemas que o bloco de cobertura listou. A ferramenta disse a verdade, e ainda assim o primeiro contato foi um beco.

Um guia que lê os schemas do servidor e mostra quantas tabelas cada um tem transforma esse beco em uma escolha de trinta segundos.

## What Changes

- Subcomando `pgfathom setup`: pergunta conexão, mostra os schemas do servidor com contagem de tabelas para escolher, pergunta modo de validação e destino dos artefatos.
- Ele **termina imprimindo o comando que compôs**, para a pessoa guardar e nunca mais precisar dele.
- Executa o `discover` ao final, e só depois de mostrar exatamente o que vai rodar e receber confirmação.
- Só funciona em terminal interativo. Em pipe ou CI, falha dizendo que precisa de um, em vez de esperar entrada que nunca chega.
- `bubbletea` e `lipgloss` entram no binário, com o custo medido e registrado no `project.md`.

## Capabilities

### New Capabilities

- `interactive-setup`: o que o guia pergunta, o que ele mostra antes de agir, e o que ele nunca faz sem confirmação.

### Modified Capabilities

- `cli-foundation`: o conjunto de subcomandos ganha o `setup`, e a regra de destino passa a valer também para um comando que exige terminal.

## Impact

**Duas dependências novas no binário**, que sai de 9 para 28 módulos e de 10,4 para 15,2 MB. É a maior mudança de superfície que este projeto já aceitou, e o `project.md` — que exige justificativa para dependência nova — foi emendado com esses números e com a razão.

`bubbles` foi recusado apesar de vir da mesma família: ele traz uma biblioteca de área de transferência por causa do campo de texto, e isso não se explica num issue de uma ferramenta de banco somente leitura. Lista e campo de texto são escritos aqui.

Nada muda para quem já usa a ferramenta. `discover` e `audit` continuam idênticos, sem dependência de interface em nenhuma camada que lê catálogo, infere, valida ou reporta.

O risco é o guia virar a única porta e as flags virarem folclore. Contra isso, ele imprime o comando composto e é a última coisa que faz antes de perguntar se pode executar — quem passou por ele uma vez não precisa passar de novo.

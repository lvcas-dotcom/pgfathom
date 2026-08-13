## Why

A nota de release da v0.1.1 é uma lista de hashes com mensagens em português, seis delas sendo `Merge pull request #NN from ...`. É a segunda coisa que alguém abre depois do README, e ela não diz o que mudou — diz o que foi tocado.

O projeto está prestes a ser divulgado, e o público é o mesmo do README: quem lê em inglês e precisa decidir se aponta isto para o banco da empresa. Uma lista de commits não ajuda essa decisão.

Há um segundo motivo, mais concreto: publicar é irreversível. Uma nota gerada não pode ser revista antes de existir, porque ela só existe depois do release. Uma nota escrita pode.

## What Changes

- **A nota de release vem de um `CHANGELOG.md` escrito à mão**, extraído pela tag e passado ao goreleaser. A geração por commits é desligada.
- **Um release sem seção no CHANGELOG falha antes de publicar.** É melhor abortar do que sair com nota vazia que não se corrige.
- O procedimento em `docs/RELEASING.md` ganha o passo, antes da tag.

## Capabilities

### Modified Capabilities

- `distribution`: a nota de release entra como parte do que o release verifica antes de publicar.

## Impact

Nenhuma dependência, nenhum código de produção. Um script de shell e um passo no workflow.

O custo real é recorrente e recai sobre quem lança: escrever a seção antes de cada tag. É o custo que se quer — a alternativa é uma nota que ninguém escreveu porque ninguém precisou.

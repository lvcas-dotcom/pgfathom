## Context

A change do corpus registrou, nas questões abertas, que a detecção era medida sem a entrada dela, e chamou isso de limitação declarada. Contra o corpus público a limitação era barata: GitLab e Discourse escrevem `_id`, que o perfil `en` já conhece, então a detecção não tinha nada a acrescentar de todo modo.

O primeiro schema em português mostrou o tamanho real do problema. Ali a convenção é `idkey_` e `_idkey`, que nenhum perfil embarcado conhece por decisão — *a convenção não é do idioma, é do schema*, e é a detecção que a lê. Sem chave declarada para ler, o recall caiu a 1,8%; com o afixo conhecido, 82,2%.

Uma limitação que muda o número por oitenta pontos não é limitação declarada, é medição errada.

## Goals / Non-Goals

**Goals**

Publicar um número que descreva a ferramenta que se entrega. Manter publicado o cenário mais difícil, que é onde a ferramenta se vende. Deixar explícito, em cada linha, qual cenário ela mede.

**Non-Goals**

Nenhuma mudança em código de produto. A detecção está certa como está, e existe teste provando que ela aprende os dois afixos das chaves declaradas.

Nenhuma tentativa de estimar o que a detecção aprenderia se pudesse ler as chaves derrubadas. Isso seria medir uma execução que ninguém faz.

## Decisions

### Duas regimes, porque são duas perguntas

**Parcial** derruba metade das chaves e mede o recall sobre essa metade. A metade que fica é evidência: a detecção a lê, exatamente como leria num banco real. Responde "quanto a ferramenta recupera do que este banco esqueceu de declarar", que é a pergunta que um usuário faz.

**Greenfield** derruba o resto e mede sobre o conjunto inteiro. Responde "quanto a ferramenta recupera de um banco que não declara integridade nenhuma", que é o caso mais difícil e o argumento de venda.

As duas são reais e nenhuma substitui a outra. Publicar só a segunda subestima a ferramenta; publicar só a primeira esconderia o pior caso.

### A metade é escolhida por ordenação, não por sorteio

O gabarito é ordenado pela identidade da relação e a divisão pega posições alternadas. Duas execuções sobre o mesmo corpus produzem a mesma divisão, e portanto o mesmo número.

Sorteio, mesmo com semente fixa, tornaria a divisão dependente de detalhe de implementação da biblioteca de aleatoriedade — e o relatório de recall é um arquivo versionado cujo diff precisa significar "mudou o comportamento", não "mudou a ordem do sorteio".

Posições alternadas em ordem de relação distribuem a divisão entre tabelas em vez de concentrar num prefixo do alfabeto, que é o que um corte pela metade da lista faria.

### Uma carga, duas medições, em sequência

Greenfield é exatamente o estado em que a parcial termina mais o resto das chaves derrubado. Então a ordem é: carregar, ler o gabarito inteiro, derrubar a metade A, medir sobre A, derrubar a metade B, medir sobre A mais B.

O ganho não é só tempo. As duas medições passam a descrever o mesmo banco carregado da mesma forma, e a diferença entre elas é atribuível ao que mudou — o que não seria verdade se cada uma tivesse a sua carga.

### O relatório nomeia o cenário, não só a taxa

Cada linha carrega a regime no rótulo, e o texto diz qual das duas descreve a primeira execução de alguém contra banco legado típico.

Duas taxas sem rótulo convidam quem cita a escolher a maior. Este projeto já resolveu esse problema uma vez, quando decidiu que amostragem nunca confirma: o número vem com o que ele não pode afirmar, colado.

## Risks / Trade-offs

**Duas taxas viram cardápio** → cada linha declara o cenário, e o texto aponta qual responde à pergunta que o leitor provavelmente tem.

**A metade escolhida pode cair de forma infeliz** — todas as chaves de uma tabela na mesma metade → posições alternadas em ordem de relação espalham por tabela; e a regime greenfield, que mede o conjunto inteiro, continua ali como controle.

**O número da regime parcial é mais alto, e alto é suspeito** → ele é medido sobre metade do gabarito, com a outra metade visível à ferramenta, e a linha diz isso. É a mesma honestidade de declarar denominador que o relatório já pratica.

**Mais uma dimensão no relatório** → o arquivo de recall cresce, e o de custo cresce mais, porque cada schema passa a ter seis execuções em vez de três. O custo continua fora do arquivo estável, onde variação não vira diff.

## Migration Plan

Não aplicável. O relatório é regenerado e o README atualizado junto, na mesma change: publicar as duas regimes com o texto antigo seria pior do que a situação atual.

## Open Questions

Nenhuma. A questão aberta que a change do corpus registrou — a detecção medida sem entrada — é justamente o que esta fecha.

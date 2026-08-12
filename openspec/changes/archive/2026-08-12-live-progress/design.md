## Context

O `discover` é uma operação de minutos contra schema grande, e hoje não diz nada até terminar. A change do benchmark instrumentou os estágios para medir custo, e com isso a informação necessária ao progresso passou a existir: nome do estágio, ordem, e o total de candidatos que a validação vai atravessar.

Falta transportá-la para fora enquanto acontece, sem violar as duas disciplinas que a saída já respeita: stdout é resultado, stderr é diagnóstico; e o destino decide a formatação, uma vez, na fronteira do processo.

## Goals / Non-Goals

**Goals**

Dizer o que está acontecendo, enquanto acontece, para quem está olhando um terminal. Não emitir nada para quem não está.

**Non-Goals**

Nenhuma dependência de interface. Uma linha que se reescreve é `\r` e um apagar de linha; biblioteca de TUI entra na change do guia interativo, onde seleção por seta e lista de schemas a justificam.

Nenhuma barra de porcentagem para estágio sem unidade mensurável. Catálogo e evidência de uso não têm denominador conhecido antes de terminarem, e inventar um seria mentir com precisão de duas casas.

Nenhuma flag para desligar. A detecção de destino já cobre pipe, arquivo, CI, `NO_COLOR` e `TERM=dumb`.

## Decisions

### O progresso é relatado, não impresso, por quem executa

A unidade de execução recebe uma função e a chama; quem desenha é o comando. É o mesmo desenho que a change do pipeline adotou para os avisos, e pela mesma razão: o harness de benchmark chama a mesma unidade e não quer nada desenhado, quer contar.

A alternativa — a camada de execução escrever em `os.Stderr` — acoplaria a orquestração ao terminal e faria o benchmark medir uma execução que imprime.

### A validação relata contagem; os outros estágios, só que começaram

A validação é onde o tempo mora: é uma query por candidato, e o total é conhecido antes de começar. Ali `312/1654` é informação real.

Catálogo, detecção, evidência de uso, geração e pré-filtro não têm denominador antes de terminarem. Para eles o progresso diz o nome do estágio e nada mais. Uma barra que anda em passos inventados dá a impressão de saber quanto falta, e não sabe.

### A decisão sobre desenhar é tomada uma vez, na fronteira

O comando decide, antes de executar, se stderr merece progresso: precisa ser character device, sem `NO_COLOR`, sem `TERM=dumb`. A decisão viaja como um valor, exatamente como o realce viaja como `Emphasis`.

`NO_COLOR` entra na conta apesar de a linha de progresso não ser cor: quem define essa variável está pedindo um stream sem escapes, e apagar linha é escape. Ser generoso na interpretação custa nada e evita sujar o log de alguém.

### Contador atômico, desenho serializado

A validação roda sob `errgroup` com limite de concorrência, então o relato vem de várias goroutines. O contador é atômico e o desenho é protegido por mutex.

Progresso que embaralha a própria linha é pior do que progresso nenhum: passa a impressão de defeito exatamente no momento em que a pessoa está avaliando se confia na ferramenta.

## Risks / Trade-offs

**Progresso escapando para stdout** → ele nunca é escrito em stdout, e o teste de disciplina de streams que já existe cobre a regra.

**Progresso em destino não interativo** → decidido na fronteira e verificado por teste: destino não interativo não recebe byte algum.

**Linha embaralhada por concorrência** → contador atômico, desenho sob mutex.

**Uma linha que se reescreve atrapalha quem lê `stderr` junto de avisos** → o aviso é escrito depois de apagar a linha de progresso, e o progresso volta na chamada seguinte.

## Migration Plan

Não aplicável.

## Open Questions

Nenhuma.

# Identidade visual

## Por que o realce virou nível e não ganhou uma dependência

`#e2483d` não existe em 4 bits. Um terminal de 16 cores recebendo `\x1b[38;2;226;72;61m` ou ignora a sequência ou imprime lixo, dependendo de quão velho ele é — e "velho" aqui inclui coisas que rodam em CI hoje.

Duas saídas: detectar capacidade, ou desistir da paleta. Detectar é o que `termenv` faz, e ele já está no binário via `lipgloss`. Ainda assim o `report` não pode usá-lo, e a razão não é custo:

> O renderizador SHALL receber essa decisão como parâmetro e MUST NOT consultar ambiente por conta própria.

Essa regra existe porque os golden files fixam a saída byte a byte. Um renderizador que detecta capacidade na inicialização produz bytes diferentes conforme a máquina — e o teste que garante que nada mudou vira o teste que falha em outra máquina.

Então a decisão continua na fronteira, onde o resto da decisão já mora, e só ganha um grau a mais: `COLORTERM` valendo `truecolor` ou `24bit` dá 24 bits, terminal sem isso dá 16 cores, sem terminal dá nada. Três estados num tipo que antes era `bool`. O renderizador recebe o nível e escolhe entre a cor da marca e a queda; nunca olha para fora.

## Por que verde-água

O relatório tem três vereditos e duas cores. `BROKEN` é vermelha, `CONFIRMED` era negrito, cabeçalho também é negrito. Quem olha rápido para uma tela de trinta candidatos distingue o vermelho do resto e mais nada.

O verde-água pastel é a única cor da paleta que podia entrar sem competir com o vermelho — cor complementar, saturação baixa o bastante para não gritar ao lado dele. E ela paga o custo certo: agora a leitura de um relatório é vermelho contra verde, com o resto quieto, que é exatamente a estrutura da informação.

`#ddd5d0` ficou de fora do relatório. Ele é claro demais para fundo claro, e cor que some em metade dos terminais não é cor — é um bug intermitente. No guia ele entra, porque lá o `lipgloss` sabe o fundo e `BrandInk` cobre o outro lado.

## Por que a marca não aparece no `discover`

Quem roda `discover` roda várias vezes por dia, quase sempre já sabendo o que a ferramenta faz. Um logo ali é ruído em cima do trabalho — e em CI, ruído em cima de um log que alguém vai ler às três da manhã.

Sobram dois momentos em que ele custa nada: `pgfathom` sem argumento, que é literalmente alguém perguntando o que isto é, e a abertura do `setup`, que é a primeira execução de alguém. Nos dois casos quem está ali está conhecendo a ferramenta.

E ele vai para stderr, pela mesma razão que o progresso e os prompts vão: stdout carrega o resultado que alguém consome. Um logo no meio de um JSON o corrompe.

## Por que a largura não é medida

O glifo tem 35 colunas. Oitenta é o piso de qualquer terminal desde o cartão perfurado, e não há terminal em uso hoje mais estreito que isso a não ser por escolha deliberada de quem o redimensionou.

Medir custaria um `ioctl` com caminho separado para Windows, ou uma dependência. A alternativa — não medir — falha, no pior caso, imprimindo um logo quebrado para quem espremeu a janela abaixo de 35 colunas. É o trade correto.

## O gradiente

A marca desce do vermelho da paleta a um vermelho mais fundo, catorze passos, sem sair do vermelho. A forma é um prumo caindo n'água, que é o que *fathom* significa e o que a ferramenta faz; o escurecimento é o que faz a forma se ler assim.

A primeira versão desbotava para `#8c7d73` na ponta. Ficou bonita e errada: uma marca que perde a própria cor no caminho não é mais a marca.

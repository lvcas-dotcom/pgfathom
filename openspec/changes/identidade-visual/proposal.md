## Why

O `pgfathom` já diz coisas graves — "esta relação está quebrada em produção", "esta aqui é sólida, e aqui está a prova" — usando exatamente dois recursos visuais: negrito e a cor que o terminal chamar de vermelha. É o vocabulário de um script, não o de uma ferramenta que alguém aponta para o banco da empresa com o chefe olhando.

Falta uma coisa concreta, não estética: **não existe cor para o que deu certo**. Quebrada é vermelha, confirmada é negrito, e negrito é também o que os cabeçalhos usam. O veredito mais importante que a ferramenta produz — o que sobreviveu à validação por dados — não tem sinal próprio.

O projeto tem paleta (`#e2483d`, `#ddd5d0`, `#8c7d73`) e tem uma marca em ASCII. Usar as duas resolve o buraco acima e, de quebra, faz a primeira execução parecer o que ela é.

## What Changes

- **O realce deixa de ser um interruptor e passa a ser um nível**: nenhum, 16 cores, 24 bits. Decidido na mesma fronteira de sempre, lendo `COLORTERM`, e entregue ao renderizador como valor.
- **Papéis semânticos ganham a paleta**: quebrada em `#e2483d`, **confirmada em verde-água `#7ec9b8` — o papel que não existia** —, atenuado em `#8c7d73`. Cada um cai para uma cor de 4 bits quando o terminal não tem mais que isso.
- **A marca aparece em dois lugares**: `pgfathom` sem argumento e a abertura do `setup`. Em stderr, só em terminal, nunca no `discover`.
- **O guia interativo é pintado na paleta**, com os mesmos papéis do relatório, e usa `lipgloss` para adaptar o tom claro ao fundo do terminal.

## Capabilities

### Modified Capabilities

- `report-terminal`: a regra de realce passa a descrever um nível com queda, e a paleta fixa os papéis.
- `cli-foundation`: a marca entra como saída diagnóstica, com as mesmas restrições de destino que o progresso já tem.
- `interactive-setup`: o guia declara que veste a mesma convenção de cor do relatório.

## Impact

**Nenhuma dependência nova.** O `report` continua sem nenhuma: as três cores são escape de 24 bits escrito à mão, com queda. O `lipgloss` já estava no binário e só é usado onde já era usado.

**Nenhum golden file muda.** Eles fixam a saída sem realce, e sem realce nada disso existe.

O risco real é `#ddd5d0` em terminal de fundo claro, onde ele é papel sobre papel. Por isso ele não entra no relatório — que precisa render igual em qualquer lugar — e no guia, que sabe o fundo, tem contraparte escura.

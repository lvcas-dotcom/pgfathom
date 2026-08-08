## Why

A ferramenta já sabe dizer que `pedido.cliente_id` aponta para `cliente.id` e que 1.284 linhas de `pedido` referenciam um cliente que não existe. O que ela ainda não faz é entregar **o que fazer com isso**. O usuário lê o achado no terminal, fecha o terminal, e o achado morre ali.

Essa é a lacuna que separa diagnóstico de ferramenta. Um DBA que descobre uma FK esquecida precisa da DDL que a declara sem travar a tabela; um que descobre uma relação quebrada precisa primeiro da query que lista os órfãos, porque a DDL não passa antes da limpeza. Escrever esse SQL à mão a partir de uma tabela no terminal é trabalho mecânico, propenso a erro de digitação em nome de constraint, e é exatamente o tipo de coisa que a ferramenta já tem todo o contexto para fazer certo.

É também o item que o critério de divulgação lista explicitamente como parte do vertical slice (`SQL / JSON artifacts`). Sem ele o projeto não sai de `EARLY ALPHA`.

As outras duas saídas chegam junto por um motivo de método: formatação regride em silêncio, e golden file escrito enquanto o conteúdo ainda muda vira manutenção morta. O conteúdo parou de mudar na fase 5. Agora os golden files valem o que custam, e é o momento certo de congelar o JSON como contrato — depois do primeiro release, mudança incompatível custa incremento de versão e migração de quem consome.

## What Changes

- `internal/report/sql.go`: geração de artefato `.sql`, um arquivo por categoria, com cabeçalho de revisão obrigatória em todos.
  - **Confirmadas** — `ADD CONSTRAINT ... NOT VALID` executável, `VALIDATE CONSTRAINT` separado e comentado, e `CREATE INDEX CONCURRENTLY` na coluna filha quando ela não tem índice em posição inicial.
  - **Quebradas** — a query que lista os órfãos primeiro, a DDL comentada depois, com o aviso de que ela só passa após a limpeza.
  - **`audit`** — o `VALIDATE CONSTRAINT` comentado de cada constraint `NOT VALID`, precedido da query que verifica se ele passaria.
- Terminal reescrito: agrupamento por veredito com **quebradas primeiro**, cabeçalho com perfil e modo, rodapé com contagem por veredito e tempo total, e o aviso de amostragem destacado. As colunas passam a ser as da especificação: relação, contenção por linha, contenção por valor, órfãos, linhas analisadas e método.
- Realce ANSI mínimo — negrito e cor apenas no aviso de amostragem e nos cabeçalhos de grupo — atrás do `Streams.Color()` que já existe. Saída em pipe continua byte-a-byte limpa.
- JSON congelado como contrato: a lista de campos de topo vira requisito com cenário, `Result` ganha a duração total da execução, e um teste de contrato quebra alto se a forma mudar sem incremento de `schema_version`.
- Flags `--out <diretório>` e `--format sql` nos dois comandos.
- Golden files para terminal e SQL, e um teste de integração que executa o SQL gerado num banco real.

## Capabilities

### New Capabilities

- `report-terminal`: a ordem de apresentação, o que o cabeçalho e o rodapé precisam declarar, o realce do aviso de amostragem, e a garantia de que o conteúdo é fixado por golden file.
- `report-json`: o JSON como API pública — campos de topo, semântica de `schema_version`, e a proibição de valor de dado do usuário verificada por varredura automatizada.
- `sql-artifacts`: o conteúdo de cada categoria de arquivo, o cabeçalho de revisão obrigatória, a citação de identificadores, a nomeação determinística de constraint, e a regra de que nada gerado é executável sem leitura.

### Modified Capabilities

- `cli-foundation`: o conjunto de formatos aceitos deixa de ser `table|json` e passa a incluir `sql`, que exige `--out`. O requisito de separação entre resultado e diagnóstico ganha o caso do artefato em arquivo, onde stdout carrega o manifesto do que foi escrito.

## Impact

Nenhuma dependência nova. `internal/report` continua sem acesso a banco e sem ler dado do usuário: ele recebe o `model.Result` pronto e o renderiza. A geração de SQL é função pura de modelo para texto, o que a torna testável sem infraestrutura — o teste de integração que executa o SQL contra um Postgres real existe para provar sintaxe, não lógica.

`--out` é a primeira escrita em disco que a ferramenta faz. A regra de somente-leitura vale para o **banco analisado** e permanece intacta; escrever arquivo no diretório que o usuário apontou é o mecanismo de entrega previsto na especificação desde o início, e é o oposto de executar DDL por conta própria.

O risco novo é de percepção, não técnico: um arquivo `.sql` gerado por máquina convida a `psql -f` sem leitura. O cabeçalho de revisão obrigatória e o fato de a DDL de categoria quebrada sair comentada existem para tornar esse caminho desconfortável de propósito.

O congelamento do JSON tem custo permanente: a partir daqui, renomear ou remover campo exige incremento de `schema_version`. É o preço de ter um contrato, e é deliberado — o `check --baseline` do pós-v0.1 consome exatamente esse arquivo.

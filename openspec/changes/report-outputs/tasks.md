## 1. Preparo

- [x] 1.1 Arquivar a change `data-validation` e commitar como primeiro commit do branch
- [x] 1.2 Acrescentar `Duration` a `model.Result` como `duration_ns`, antes do congelamento do contrato
- [x] 1.3 Acrescentar `Discarded` a `model.Result` como `discarded`, e parar de anexar descartados a `Candidates` na CLI

## 2. Terminal

- [x] 2.1 Realce mínimo em `internal/report`: negrito, cor de alerta e atenuado, com todas as funções devolvendo string vazia quando desligado
- [x] 2.2 Passar a decisão de realce de `Streams.Color()` até o renderizador, sem que ele consulte ambiente
- [x] 2.3 Reescrever `Discover` agrupando por veredito na ordem quebradas, confirmadas, fracas, não validadas, com contagem por grupo
- [x] 2.4 Colunas da especificação: relação, contenção por linha, contenção por valor, órfãos por linha e por valor, linhas examinadas, método
- [x] 2.5 Cabeçalho com versão da ferramenta, versão do servidor, perfil, limiar e modo
- [x] 2.6 Rodapé com contagem por veredito, tempo total e o bloco de cobertura completo
- [x] 2.7 Aviso de amostragem realçado no topo e repetido no rodapé
- [x] 2.8 Manter rejeitadas fora por padrão, com o ponteiro para `--include-rejected` no rodapé

## 3. Artefatos SQL

- [x] 3.1 Criar `internal/report/sql.go` com a geração como função pura de `model.Result` para conjunto de arquivos
- [x] 3.2 Cabeçalho de revisão obrigatória com versão, timestamp, versão do servidor, modo e o aviso; variante amostrada declarando que nada foi confirmado
- [x] 3.3 Nome de constraint determinístico, truncado com orçamento em bytes e corte em fronteira de rune, com o nome integral em comentário quando truncar
- [x] 3.4 Citação de identificador pelo mesmo mecanismo de `internal/validate`
- [x] 3.5 Categoria confirmada: `ADD CONSTRAINT ... NOT VALID` executável e `VALIDATE CONSTRAINT` comentado em bloco separado, com a razão escrita
- [x] 3.6 `CREATE INDEX CONCURRENTLY` comentado quando a coluna filha não tem índice em posição inicial, com os avisos de bloco de transação e de índice inválido
- [x] 3.7 Categoria quebrada: query de órfãos por anti-join primeiro, DDL comentada depois, contagem em comentário declarada como piso quando amostrado
- [x] 3.8 Categoria de `audit`: query de verificação seguida do `VALIDATE CONSTRAINT` comentado, por constraint `NOT VALID`, com suporte a chave composta
- [x] 3.9 Escrever todo arquivo mesmo com categoria vazia, com a afirmação e o motivo quando houver
- [x] 3.10 Garantir que veredito fraco, rejeitado e não validado não produzem DDL de espécie alguma

## 4. Integração na CLI

- [x] 4.1 Flag `--out` nos dois comandos, criando o diretório quando ausente
- [x] 4.2 `--format sql` aceito e exigindo `--out`, com erro de uso quando faltar
- [x] 4.3 Manifesto em stdout no modo `sql`: um caminho por artefato com a contagem da categoria
- [x] 4.4 Medir a duração da execução e preenchê-la no resultado
- [x] 4.5 Ajustar o texto de ajuda dos dois comandos ao que eles de fato fazem hoje
- [x] 4.6 Atualizar o README: invocação com `--out` e as fases 5 e 7 no roadmap

## 5. Verificação

- [x] 5.1 Golden files do terminal: quatro vereditos presentes, execução sem achado, modo amostrado, cobertura incompleta — com versão, timestamp e duração fixos na entrada
- [x] 5.2 Golden files dos três artefatos SQL, incluindo o caso de nome exótico e o de coluna sem índice
- [x] 5.3 Teste de que o renderizador com realce desligado não emite nenhum byte de escape ANSI
- [x] 5.4 Teste de contrato do JSON comparando o conjunto de caminhos de chave contra um golden
- [x] 5.5 Teste unitário de truncamento de nome de constraint: orçamento em bytes, corte em fronteira de rune, sufixo, e dois nomes longos que não colidem
- [x] 5.6 Teste unitário de determinismo: gerar duas vezes a mesma entrada produz o mesmo conteúdo fora o timestamp
- [ ] 5.7 Teste de integração que aplica o SQL gerado ao banco da fixture e confere que as constraints existem como `NOT VALID` — **escrito, não executado: sem Docker no ambiente de desenvolvimento**
- [ ] 5.8 Estender a varredura de vazamento ponta a ponta ao conteúdo dos artefatos gerados, em todas as fixtures — **escrito, não executado: sem Docker**
- [x] 5.9 Confirmar formatação dos arquivos alterados e rodar `golangci-lint run --build-tags integration` zerando os apontamentos
- [x] 5.10 Confirmar que `go test ./...` segue sem Docker e sem rede
- [x] 5.11 Rodar `openspec validate report-outputs --strict` e corrigir o que apontar

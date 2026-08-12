## 1. Relato pela unidade de execução

- [x] 1.1 Opção de progresso recebida pela unidade, com estágio e, quando houver, contagem
- [x] 1.2 Relato por estágio, na ordem de execução
- [x] 1.3 Validação relatando quantos candidatos terminaram, com contador atômico
- [x] 1.4 Sem função de progresso, nada é escrito e nada muda

## 2. Exibição pelo comando

- [x] 2.1 Decisão na fronteira: stderr é terminal, sem `NO_COLOR`, sem `TERM=dumb`
- [x] 2.2 Linha única que se reescreve, apagada ao terminar
- [x] 2.3 Desenho serializado, para que concorrência não embaralhe a linha
- [x] 2.4 Aviso apaga a linha antes de escrever, e o progresso volta depois

## 3. Verificação

- [x] 3.1 Teste de que destino não interativo não recebe byte algum
- [x] 3.2 Teste de que `NO_COLOR` suprime
- [x] 3.3 Teste de que o relato cobre os estágios na ordem
- [x] 3.4 Nenhum golden file muda: stdout não é tocado
- [x] 3.5 `go test ./...` e lint nas três etiquetas
- [x] 3.6 Rodar `openspec validate live-progress --strict`

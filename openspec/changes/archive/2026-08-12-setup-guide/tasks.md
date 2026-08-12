## 1. Fundação da interface

- [x] 1.1 `bubbletea` e `lipgloss` no `go.mod`, no mesmo commit do código que os importa
- [x] 1.2 Lista selecionável escrita aqui, sobre os eventos de tecla — sem `bubbles`
- [x] 1.3 Campo de texto de uma linha, idem
- [x] 1.4 Recusar entrada ou saída que não sejam terminal, antes de perguntar qualquer coisa

## 2. As perguntas

- [x] 2.1 Conexão, aproveitando `PGFATHOM_DSN` quando estiver definida
- [x] 2.2 Testar a conexão e mostrar servidor e aviso de privilégio de escrita
- [x] 2.3 Listar schemas do servidor com contagem de tabelas, e escolher um ou mais
- [x] 2.4 Modo de validação, dizendo que só o completo confirma
- [x] 2.5 Destino dos artefatos, opcional

## 3. Composição e execução

- [x] 3.1 Montar o comando `discover` a partir das respostas
- [x] 3.2 Imprimir o comando, com as flags explícitas
- [x] 3.3 Pedir confirmação e executar exatamente o que foi mostrado
- [x] 3.4 Recusa não executa nada, e o comando impresso continua na tela

## 4. Verificação

- [x] 4.1 Teste de que a composição do comando corresponde às respostas
- [x] 4.2 Teste de que sem terminal o comando recusa com mensagem
- [x] 4.3 Teste de que nenhuma camada fora de `internal/cli` importa interface
- [x] 4.4 `go test ./...` e lint nas três etiquetas
- [x] 4.5 `make crosscheck`: a interface não pode quebrar o build cruzado
- [x] 4.6 Conferir o tamanho do binário contra o número publicado no `project.md`

## 5. Documentação

- [x] 5.1 README apresentando o `setup` como o caminho da primeira execução
- [x] 5.2 Rodar `openspec validate setup-guide --strict`

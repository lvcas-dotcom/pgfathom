## 1. Autocompletar

- [x] 1.1 Gerar bash, zsh e fish a partir do comando, no hook que roda antes de empacotar
- [x] 1.2 Diretório dos gerados ignorado pelo git
- [x] 1.3 Incluir também nos arquivos `.tar.gz`, para quem não usa pacote

## 2. Pacotes

- [x] 2.1 `nfpms` no goreleaser produzindo `.deb` e `.rpm` para amd64 e arm64
- [x] 2.2 Binário em `/usr/bin`; licença e README sob `/usr/share/doc/pgfathom/`
- [x] 2.3 Autocompletar nos diretórios de bash, zsh e fish
- [x] 2.4 Metadados: mantenedor, licença, homepage, descrição e seção

## 3. Verificação

- [x] 3.1 `goreleaser check` limpo
- [x] 3.2 Snapshot produzindo os quatro pacotes
- [x] 3.3 Listar o conteúdo de um `.deb` e de um `.rpm` e conferir cada caminho
- [x] 3.4 Conferir que o script gerado delega ao binário, e que o binário resolve uma flag real do `discover`
- [x] 3.5 `go test ./...` e lint nas três etiquetas

## 4. Documentação

- [x] 4.1 README com a instalação por pacote, na frente das outras
- [x] 4.2 `docs/RELEASING.md`: canal na tabela e a ausência de repositório declarada
- [x] 4.3 Registrar a página de manual como ausência conhecida
- [x] 4.4 Rodar `openspec validate linux-packages --strict`

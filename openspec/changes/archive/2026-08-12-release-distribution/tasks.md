## 1. Empacotamento

- [x] 1.1 `.goreleaser.yaml` construindo `./cmd/pgfathom` sem cgo, para linux, darwin e windows, em amd64 e arm64
- [x] 1.2 Carimbo de versão, commit e data pelos mesmos nomes de variável que o Makefile usa
- [x] 1.3 Arquivos com licença e README junto do binário; checksums no release
- [x] 1.4 `goreleaser check` limpo, e alvo de Makefile que o rode
- [x] 1.5 `goreleaser release --snapshot` produzindo os artefatos localmente

## 2. Imagem de contêiner

- [x] 2.1 Imagem multiplataforma sobre base mínima com certificados raiz, executando como usuário sem privilégio
- [x] 2.2 Publicação no `ghcr.io`, sem credencial nova
- [x] 2.3 Conferir que a imagem carrega CA raiz, com a razão escrita ao lado da escolha de base

## 3. Verificação do carimbo

- [x] 3.1 Construir por snapshot e executar o binário, falhando quando versão, commit ou data saírem como desconhecidos
- [x] 3.2 Ligar essa verificação ao CI, para que rode em toda mudança e não só no release

## 4. Workflow de release

- [x] 4.1 Disparo por tag, com suíte e lint antes de qualquer publicação
- [x] 4.2 Permissões mínimas no workflow, e token do tap como opcional
- [x] 4.3 Conferir que uma falha na suíte aborta sem publicar

## 5. Procedimento e documentação

- [x] 5.1 `docs/RELEASING.md` com o passo a passo, o pré-requisito do tap na frente, e as ausências declaradas
- [x] 5.2 README com as formas de instalar, incluindo `go install` e a imagem
- [x] 5.3 README e `docs/ROADMAP.md` deixam de dizer que a fase 8 está planejada
- [x] 5.4 Rever o aviso de pré-release do README contra o que a fase 8 de fato entregou

## 6. Verificação

- [x] 6.1 `go test ./...` e lint nas três etiquetas
- [x] 6.2 `make crosscheck` verde, que é a promessa que o empacotamento assume
- [x] 6.3 Conferir que `go.mod` não ganhou dependência
- [x] 6.4 Rodar `openspec validate release-distribution --strict` e corrigir o que apontar

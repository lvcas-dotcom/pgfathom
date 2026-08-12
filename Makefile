BINARY  := pgfathom
PKG     := github.com/lvcas-dotcom/pgfathom
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X '$(PKG)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(PKG)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(PKG)/internal/buildinfo.Date=$(DATE)'

# CGO_ENABLED=0 não é otimização, é requisito: cross-compile precisa ser trivial.
export CGO_ENABLED := 0

.PHONY: build test test-integration corpus benchmark lint fmt cover crosscheck release-check image-check clean help

## build: compila o binário em ./bin
build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

## test: testes unitários, sem Docker e sem rede
test:
	go test ./...

## test-integration: testes que sobem PostgreSQL real via testcontainers
test-integration:
	go test -tags=integration -timeout=20m ./...

## corpus: baixa e confere os schemas do benchmark (única etapa que usa rede)
corpus:
	go test -tags=benchmark -timeout=15m -v -run TestFetchCorpus ./internal/bench/...

## benchmark: mede a taxa de recuperação no corpus e escreve docs/benchmark
benchmark:
	go test -tags=benchmark -timeout=90m -v -run TestCorpus ./internal/bench/...

## lint: golangci-lint em todas as etiquetas de build
lint:
	golangci-lint run
	golangci-lint run --build-tags integration
	golangci-lint run --build-tags benchmark

## fmt: formata e organiza imports
fmt:
	go fmt ./...
	go vet ./...

## cover: cobertura de testes unitários
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## crosscheck: prova que o build cruzado funciona sem cgo
crosscheck:
	@for target in linux/amd64 linux/arm64 darwin/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -o /dev/null ./cmd/$(BINARY) || exit 1; \
	done
	@echo "build cruzado ok"

## release-check: prova que o caminho de release produz binário carimbado
#
# O carimbo vem de duas configurações que precisam concordar sobre três nomes
# de variável — esta aqui e a do goreleaser. A comparação é por igualdade, e não
# por procurar "unknown", porque o binário nunca diz "unknown": buildinfo cai
# para o que o toolchain gravou e responde uma pseudo-versão do commit. É um
# bom padrão para `go install` e é exatamente o que esconderia um ldflag
# quebrado, publicando binário cuja versão não é a da tag.
release-check:
	goreleaser check
	goreleaser build --snapshot --clean --single-target --output dist/pgfathom-check
	@want=$$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' dist/metadata.json); \
	got=$$(./dist/pgfathom-check version | head -1 | awk '{print $$2}'); \
	if [ "$$got" != "$$want" ]; then \
		echo "FALHA: o release carimbaria $$want e o binário responde $$got"; \
		exit 1; \
	fi; \
	echo "carimbo ok: $$got"

## image-check: prova que a imagem constrói para as duas plataformas
#
# Reproduz o contexto que o goreleaser monta — um binário por plataforma, em
# subdiretórios — e constrói sem publicar. É a verificação que faltava: o
# Dockerfile foi escrito para o formato de contexto antigo, plano, e o erro só
# apareceu no primeiro release, no passo depois de tudo ter passado.
image-check:
	@ctx=$$(mktemp -d); \
	trap 'rm -rf "$$ctx"' EXIT; \
	for p in amd64 arm64; do \
		mkdir -p "$$ctx/linux/$$p"; \
		GOOS=linux GOARCH=$$p go build -trimpath -o "$$ctx/linux/$$p/$(BINARY)" ./cmd/$(BINARY) || exit 1; \
	done; \
	cp Dockerfile "$$ctx/"; \
	docker buildx build --platform linux/amd64,linux/arm64 --output=type=cacheonly "$$ctx"
	@echo "imagem ok"

## clean: remove artefatos de build
clean:
	rm -rf bin dist coverage.out

## help: lista os alvos
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

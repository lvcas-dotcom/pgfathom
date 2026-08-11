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

.PHONY: build test test-integration corpus benchmark lint fmt cover crosscheck clean help

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

## clean: remove artefatos de build
clean:
	rm -rf bin dist coverage.out

## help: lista os alvos
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

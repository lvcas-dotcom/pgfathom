BINARY  := pgfathom
PKG     := github.com/lvcas-dotcom/pgfathom
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X '$(PKG)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(PKG)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(PKG)/internal/buildinfo.Date=$(DATE)'

# CGO_ENABLED=0 is a requirement rather than a tuning choice: cross-compiling
# has to stay trivial.
export CGO_ENABLED := 0

.PHONY: build test test-integration corpus benchmark lint fmt cover crosscheck release-check image-check clean help

## build: compile the binary into ./bin
build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

## test: unit suite; no Docker, no network
test:
	go test ./...

## test-integration: starts real PostgreSQL through testcontainers
test-integration:
	go test -tags=integration -timeout=20m ./...

## corpus: fetch and verify the benchmark schemas (the only step that uses the network)
corpus:
	go test -tags=benchmark -timeout=15m -v -run TestFetchCorpus ./internal/bench/...

## benchmark: measure recall against the corpus and write docs/benchmark
benchmark:
	go test -tags=benchmark -timeout=90m -v -run TestCorpus ./internal/bench/...

## lint: golangci-lint under every build tag
lint:
	golangci-lint run
	golangci-lint run --build-tags integration
	golangci-lint run --build-tags benchmark

## fmt: format and vet
fmt:
	go fmt ./...
	go vet ./...

## cover: unit test coverage
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## crosscheck: prove cross-compilation works without cgo
crosscheck:
	@for target in linux/amd64 linux/arm64 darwin/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -o /dev/null ./cmd/$(BINARY) || exit 1; \
	done
	@echo "cross-compile ok"

## release-check: prove the release path produces a correctly stamped binary
#
# The stamp comes from two configurations that have to agree on three variable
# names — this one and goreleaser's. The comparison is for equality rather than
# for the string "unknown", because the binary never says "unknown": buildinfo
# falls back to what the toolchain recorded and answers a pseudo-version of the
# commit. That is a good default for `go install`, and it is exactly what would
# hide a broken ldflag and publish a binary whose version is not the tag's.
release-check:
	goreleaser check
	goreleaser build --snapshot --clean --single-target --output dist/pgfathom-check
	@want=$$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' dist/metadata.json); \
	got=$$(./dist/pgfathom-check version | head -1 | awk '{print $$2}'); \
	if [ "$$got" != "$$want" ]; then \
		echo "FAILED: the release would stamp $$want and the binary answers $$got"; \
		exit 1; \
	fi; \
	echo "stamp ok: $$got"
# The newest CHANGELOG entry has to come out whole. Extraction breaks silently
# when the heading changes shape, and the release is too late to find that out.
	@v=$$(sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' CHANGELOG.md | head -1); \
	./scripts/release-notes.sh "v$$v" > /dev/null; \
	echo "release notes ok: $$v"

## image-check: prove the image builds for both platforms
#
# Reproduces the context goreleaser assembles — one binary per platform, in
# subdirectories — and builds without publishing. This is the check that was
# missing: the Dockerfile was written for the old flat context format, and the
# error surfaced only during the first release, in the step after everything
# else had passed.
image-check:
	@ctx=$$(mktemp -d); \
	trap 'rm -rf "$$ctx"' EXIT; \
	for p in amd64 arm64; do \
		mkdir -p "$$ctx/linux/$$p"; \
		GOOS=linux GOARCH=$$p go build -trimpath -o "$$ctx/linux/$$p/$(BINARY)" ./cmd/$(BINARY) || exit 1; \
	done; \
	cp Dockerfile "$$ctx/"; \
	docker buildx build --platform linux/amd64,linux/arm64 --output=type=cacheonly "$$ctx"
	@echo "image ok"

## clean: remove build artifacts
clean:
	rm -rf bin dist coverage.out

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

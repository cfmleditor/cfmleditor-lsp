BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)
VERSION := $(shell cat VERSION)
WASI_SDK ?= /opt/wasi-sdk

.PHONY: build build-wasm test install clean docs docs-cfdocs docs-lucee generate cfparse cfparse-build update-grammar release release-dry

docs: docs-cfdocs

docs-cfdocs:
	@./scripts/fetch-docs-cfdocs.sh

docs-lucee:
	@./scripts/fetch-docs-lucee.sh

generate: docs
	go run scripts/generate_docs.go

update-grammar: generate
	npm --prefix ../tree-sitter-cfml run build
	go get github.com/cfmleditor/tree-sitter-cfml@latest
	go mod tidy
	go clean -cache

build: generate
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(OUT) ./cmd/cfmleditor-lsp

build-wasm: generate
	@mkdir -p target/release
	CC=$(WASI_SDK)/bin/clang CGO_ENABLED=1 GOOS=wasip1 GOARCH=wasm \
		go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o target/release/$(BINARY).wasm ./cmd/cfmleditor-lsp

test:
	go test ./...

visualtest:
	go test -v -run TestFormatOutput ./internal/formatter/

fmt:
	gofmt -w .
	golangci-lint run --fix ./...

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

install: build
	cp $(OUT) $(GOPATH)/bin/$(BINARY)

cfparse-build:
	@mkdir -p target/release
	go build -trimpath -o target/release/cfparse ./cmd/cfparse

cfparse: cfparse-build
	@target/release/cfparse $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

release:
	@go run scripts/release.go $(filter-out $@,$(MAKECMDGOALS))

release-dry:
	@go run scripts/release.go --dry-run $(filter-out $@,$(MAKECMDGOALS))

clean:
	rm -rf target

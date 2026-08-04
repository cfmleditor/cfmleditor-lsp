BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)
VERSION := $(shell cat VERSION)
WASI_SDK ?= /opt/wasi-sdk

.PHONY: build build-wasm test install clean docs docs-cfdocs docs-lucee docs-assemble generate cfparse cfparse-build update-grammar release release-dry

# Fetch every source, then assemble docs/data from all of them. Written as one
# sequential recipe rather than as prerequisites so `make -j` cannot start the
# assemble step before both fetches have finished.
#
# A single unreachable source must not abort the build: each fetch leaves its
# previously staged copy intact on failure, so assemble still sees it. Only a
# source that was never fetched at all goes missing, and assemble warns loudly
# for that. Releases enforce completeness separately (.github/workflows).
docs:
	@./scripts/fetch-docs-cfdocs.sh || echo "warning: cfdocs fetch failed — using previously staged copy if present" >&2
	@./scripts/fetch-docs-lucee.sh  || echo "warning: lucee fetch failed — using previously staged copy if present" >&2
	@./scripts/assemble-docs.sh

docs-cfdocs:
	@./scripts/fetch-docs-cfdocs.sh
	@./scripts/assemble-docs.sh

docs-lucee:
	@./scripts/fetch-docs-lucee.sh
	@./scripts/assemble-docs.sh

docs-assemble:
	@./scripts/assemble-docs.sh

generate: docs
	go run scripts/generate_docs.go

update-grammar: generate
	go get github.com/cfmleditor/tree-sitter-cfml@latest
	go mod tidy
	go clean -cache
	@mkdir -p internal/language/queries
	@curl -sL "https://raw.githubusercontent.com/cfmleditor/tree-sitter-cfml/$$(grep tree-sitter-cfml go.mod | awk '{print $$2}')/cfml/queries/injections.scm" -o internal/language/queries/injections.scm
	@echo "Updated injections.scm from tree-sitter-cfml"

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

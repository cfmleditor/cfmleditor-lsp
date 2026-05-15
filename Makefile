BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)
VERSION := $(shell cat VERSION)

.PHONY: build test install clean docs docs-cfdocs docs-lucee generate cfparse cfparse-build

docs: docs-cfdocs

docs-cfdocs:
	@./scripts/fetch-docs-cfdocs.sh

docs-lucee:
	@./scripts/fetch-docs-lucee.sh

generate: docs
	go run scripts/generate_docs.go

build: generate
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(OUT) ./cmd/cfmleditor-lsp

test:
	go test ./...

visualtest:
	go test -v -run TestFormatOutput ./internal/formatter/

lint:
	golangci-lint run --enable bodyclose,gocritic ./...

lint-fix:
		golangci-lint run --enable bodyclose,gocritic --fix ./...
		
install: build
	cp $(OUT) $(GOPATH)/bin/$(BINARY)

cfparse-build:
	@mkdir -p target/release
	go build -trimpath -o target/release/cfparse ./cmd/cfparse

cfparse: cfparse-build
	@target/release/cfparse $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

clean:
	rm -rf target

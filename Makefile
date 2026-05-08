BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)

.PHONY: build test install clean docs generate

docs:
	@./scripts/fetch-docs.sh

generate: docs
	go run scripts/generate_docs.go

build: generate
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w" -o $(OUT) ./cmd/cfmleditor-lsp

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

clean:
	rm -rf target

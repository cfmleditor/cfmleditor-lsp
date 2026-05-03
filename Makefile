BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)

.PHONY: build test install clean docs generate

docs:
	@./scripts/fetch-docs.sh

generate: docs
	go run scripts/gen-builtin.go

build: generate
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w" -o $(OUT) .

test:
	go test ./...

formattest:
	go test -v -run TestFormatOutput ./internal/formatter/

install: build
	cp $(OUT) $(GOPATH)/bin/$(BINARY)

clean:
	rm -rf target

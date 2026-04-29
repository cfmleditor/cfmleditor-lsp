BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)

.PHONY: build test install clean

build:
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w" -o $(OUT) .

test:
	go test ./...

install: build
	cp $(OUT) $(GOPATH)/bin/$(BINARY)

clean:
	rm -rf target

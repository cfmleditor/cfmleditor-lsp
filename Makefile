BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)
VERSION := $(shell cat VERSION)
WASI_SDK ?= /opt/wasi-sdk

.PHONY: build build-wasm test corpus shrink install clean docs docs-cfdocs docs-lucee docs-assemble generate cfparse cfparse-build update-grammar vuln release release-dry

# Pinned so a scanner change never turns an unrelated build red on its own.
# Bump deliberately; the advisory database itself is always fetched live, so a
# pinned scanner still sees newly published vulnerabilities.
GOVULNCHECK ?= golang.org/x/vuln/cmd/govulncheck@v1.7.0

# Pinned for the same reason, and built from source rather than taken from PATH.
# golangci-lint refuses to load a config whose module targets a newer Go than the
# linter itself was built with — "the Go language version (go1.25) used to build
# golangci-lint is lower than the targeted Go version (1.26.6)" — which is not a
# lint failure but a flat refusal to start, and it is what any distro or Homebrew
# binary does for a while after each Go bump. Building it here under this module's
# own toolchain makes that mismatch impossible by construction.
#
# GOTOOLCHAIN has to be forced: resolving a pkg@version otherwise picks the
# toolchain the *tool* module asks for, which is the oldest one it supports, and
# a tool built with an older Go than this module targets cannot load its packages
# at all. govulncheck fails the same way, less legibly: built with 1.25 against a
# 1.26 module it reports every package as "requires newer Go version" and scans
# nothing, while still exiting non-zero.
GOLANGCI ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

# The version is read out of go.mod directly rather than with `go list -m`.
# Inside a workspace that lists every module in go.work, one per line, and
# $(shell) folds those lines into a single space-separated value — the recipe
# then expands to `GOTOOLCHAIN=go1.26.6 1.23 go run ...` and sh tries to run
# "1.23" as a command. A developer with a go.work alongside the grammar repo
# is the normal case here, so `go list -m` cannot be used for this.
GO_VERSION = $(shell awk '/^go /{print $$2; exit}' go.mod)
GOLANGCI_RUN = GOTOOLCHAIN=go$(GO_VERSION) go run $(GOLANGCI)

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

# Formats a corpus of real-world CFML and reports what the formatter did to each
# file: clean, refused by the grammar, rejected by the whitespaceOnly guard, or not
# idempotent. Every defect in FORMATTER-ISSUES.md was found this way. The corpus is
# far too large to vendor, so CORPUS points at it and the test skips without one:
#
#   make corpus CORPUS=/src/Lucee:/src/ContentBox REPORT=/tmp/corpus.tsv
#
# REPORT is optional and names a TSV of every non-clean file to work through.
corpus:
	@test -n "$(CORPUS)" || { echo "usage: make corpus CORPUS=<dir>[:<dir>...] [REPORT=<file>]" >&2; exit 2; }
	CFML_CORPUS="$(CORPUS)" CFML_CORPUS_REPORT="$(REPORT)" \
		go test -v -count=1 -timeout 30m -run TestFormatterCorpus ./internal/formatter/

# Reduces the parse-refused and script-refused entries in a corpus report to
# the smallest contiguous fragment that still fails, so "the grammar cannot
# parse this file" becomes a construct that can be filed against
# tree-sitter-cfml. Takes the TSV `make corpus REPORT=...` writes:
#
#   make corpus CORPUS=/src/Lucee REPORT=/tmp/corpus.tsv
#   make shrink REPORT=/tmp/corpus.tsv
#
# Fragments are a starting point, not a verdict — see the comment at the top of
# internal/formatter/shrink_test.go.
# The output goes through a file rather than a pipe. Piping `go test` into
# `grep` hands the pipeline grep's exit status, so a build error or a failing
# test came back as success and `make shrink` printed nothing and exited 0.
shrink:
	@test -n "$(REPORT)" || { echo "usage: make shrink REPORT=<corpus report>" >&2; exit 2; }
	@log=$$(mktemp); \
	CFML_SHRINK_REPORT="$(REPORT)" \
		go test -v -count=1 -timeout 30m -run TestShrinkRefusals ./internal/formatter/ >$$log 2>&1; \
	status=$$?; \
	grep -vE "^(=== RUN|--- PASS|--- SKIP|PASS|ok  )" $$log || true; \
	rm -f $$log; \
	exit $$status

fmt:
	gofmt -w .
	$(GOLANGCI_RUN) run --fix ./...

lint:
	$(GOLANGCI_RUN) run ./...

# GOWORK=off so the scan resolves dependencies from go.mod rather than from a
# developer's go.work. A workspace can substitute a local checkout (e.g.
# ../tree-sitter-cfml) for a pinned module, which would report on source that
# is not what a release actually builds from.
vuln:
	GOWORK=off GOTOOLCHAIN=go$(GO_VERSION) go run $(GOVULNCHECK) ./...

lint-fix:
	$(GOLANGCI_RUN) run --fix ./...

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

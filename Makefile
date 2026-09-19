BINARY := cfmleditor-lsp
OUT := target/release/$(BINARY)
VERSION := $(shell cat VERSION)
WASI_SDK ?= /opt/wasi-sdk

# `go env GOPATH`, not $(GOPATH): the latter reads the *environment* variable,
# which is normally unset because Go computes the default itself — so it
# expanded to an empty string and `make install` wrote to /bin/cfmleditor-lsp.
GOBIN_DIR := $(shell go env GOPATH)/bin

# Where `make link` puts the symlink the Zed extension picks up. Override to
# somewhere else on PATH: make link LINK_DIR=$$HOME/.local/bin
LINK_DIR ?= $(GOBIN_DIR)
LINK := $(LINK_DIR)/$(BINARY)

.PHONY: build build-wasm test conformance conformance-summary corpus shrink install link unlink link-status clean docs docs-cfdocs docs-lucee docs-assemble generate cfparse cfparse-build update-grammar vuln release release-dry

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
#
# Building it under this module's toolchain is necessary but not sufficient: the
# linter also has to *understand* that Go version's syntax. v2.12.2 vendored
# staticcheck v0.7.0, which predates Go 1.27, and its IR builder panicked on the
# 1.27 standard library -- "buildir: package \"poll\": unexpected expr:
# *ast.KeyValueExpr" -- because 1.27 allows any valid field selector as a key in
# a struct literal and internal/poll uses it. That aborts goanalysis_metalinter
# outright, so it reads as a lint failure rather than a tool that cannot parse
# its input. v2.13.2 vendors staticcheck v0.8.1 (2026.2.1), which added Go 1.27
# support. The panic was Linux-only -- internal/poll is platform-split and the
# darwin build does not use the new form -- so a local `make lint` on macOS
# passed while CI did not. Bump this in step with the go directive.
GOLANGCI ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2

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

# Rebuilds the JavaScript the generated code-map viewer embeds. Needs Node; the
# bundle it produces is committed, which is what keeps `go build` free of that
# requirement and every generated report free of a network fetch. Run it after
# adding a D3 function to VENDOR/entry.js, or the call is undefined at runtime.
VENDOR := internal/codemap/assets/vendor

.PHONY: update-d3
update-d3:
	@command -v npm >/dev/null || { echo "npm is required to rebuild the viewer bundle"; exit 1; }
	@# npm ci when the lock file is there, so the bundle is reproducible from the
	@# exact dependency tree that was committed; npm install only to create it.
	cd $(VENDOR) && { test -f package-lock.json && npm ci --no-audit --no-fund --silent \
		|| npm install --no-audit --no-fund --silent; }
	cd $(VENDOR) && npx --yes esbuild entry.js --bundle --minify --format=iife \
		--global-name=d3 --outfile=d3.bundle.js --legal-comments=none
	@rm -rf $(VENDOR)/node_modules
	@grep -qi '</script' $(VENDOR)/d3.bundle.js \
		&& { echo "ERROR: the bundle contains </script and cannot be inlined"; exit 1; } || true
	@echo "Rebuilt $(VENDOR)/d3.bundle.js ($$(wc -c < $(VENDOR)/d3.bundle.js) bytes) - commit it"

build: generate
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(OUT) ./cmd/cfmleditor-lsp

build-wasm: generate
	@mkdir -p target/release
	CC=$(WASI_SDK)/bin/clang CGO_ENABLED=1 GOOS=wasip1 GOARCH=wasm \
		go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o target/release/$(BINARY).wasm ./cmd/cfmleditor-lsp

test:
	go test ./...
	@$(MAKE) --no-print-directory conformance-summary

# Replays the cfmleditor extension's own go-to-definition suite against this
# server and prints every case. The extension stands its own providers down
# whenever this server runs, so a case it answers and this server does not is
# something a user loses by enabling the server -- this says which.
#
# It runs under plain `go test ./...` too; the point of a target of its own is
# the report. A skipped case is a known gap and names its reason.
conformance:
	go test -v -count=1 -run 'TestDefinitionConformance|TestKnownGapsAreRealCases' ./internal/server/

# The one-line score, appended to `make test` so the gap count is visible on
# every run rather than only when someone goes looking for it.
conformance-summary:
	@out=$$(go test -v -count=1 -run TestDefinitionConformance ./internal/server/ 2>&1); \
	pass=$$(printf '%s\n' "$$out" | grep -c -- '    --- PASS' || true); \
	skip=$$(printf '%s\n' "$$out" | grep -c -- '    --- SKIP' || true); \
	fail=$$(printf '%s\n' "$$out" | grep -c -- '    --- FAIL' || true); \
	echo "definition conformance vs the extension: $$pass/$$((pass+skip+fail)) cases answered, $$skip known gaps ('make conformance' for the list)"; \
	test "$$fail" -eq 0

visualtest:
	go test -v -run TestFormatOutput ./internal/formatter/

# Formats a corpus of real-world CFML and reports what the formatter did to each
# file: clean, refused by the grammar, rejected by the whitespaceOnly guard, or not
# idempotent. Every defect in FORMATTER-ISSUES.md was found this way. The corpus is
# far too large to vendor, so CORPUS points at it and the test skips without one:
#
#   make corpus CORPUS=/src/Lucee:/src/ContentBox REPORT=/tmp/corpus.tsv
#
# BASELINE names an earlier REPORT and fails the run if any file changed verdict,
# which the totals cannot show: a change that breaks one file and fixes another
# leaves every column identical.
#
#   make corpus CORPUS=/src/Lucee REPORT=/tmp/before.tsv          # record
#   make corpus CORPUS=/src/Lucee BASELINE=/tmp/before.tsv        # compare
#
# OPTS sets formatter.Options fields by name, for sweeping a new setting through
# every mode it offers without editing the test:
#
#   make corpus CORPUS=/src/Lucee OPTS=braceStyle=next-line,paramBreakThreshold=3
#
# REPORT is optional and names a TSV of every non-clean file to work through.
corpus:
	@test -n "$(CORPUS)" || { echo "usage: make corpus CORPUS=<dir>[:<dir>...] [REPORT=<file>] [BASELINE=<file>] [OPTS=k=v,...]" >&2; exit 2; }
	CFML_CORPUS="$(CORPUS)" CFML_CORPUS_REPORT="$(REPORT)" \
	CFML_CORPUS_BASELINE="$(BASELINE)" CFML_CORPUS_OPTS="$(OPTS)" \
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
	@mkdir -p $(GOBIN_DIR)
	cp $(OUT) $(GOBIN_DIR)/$(BINARY)

# Point the zed-cfml extension at this working tree's build.
#
# The extension resolves its server in three steps (src/cfml.rs): a cached path,
# then worktree.which("cfmleditor-lsp"), and only then a GitHub release download
# into a cfmleditor-lsp-<version>/ directory. So the hook for local use is the
# PATH lookup — a symlink named cfmleditor-lsp anywhere on PATH wins and no
# download happens. Symlinking into the extension's own directory would not
# survive: that path is named after the release version, and the extension
# prunes the versions it is not using.
#
# Restart Zed after linking or unlinking; the extension caches the path it
# resolved for the life of the session.
link: build
	@mkdir -p $(LINK_DIR)
	@if [ -e "$(LINK)" ] && [ ! -L "$(LINK)" ]; then \
		echo "refusing to replace $(LINK): it is a real file, not a symlink." >&2; \
		echo 'A `make install` copy lives there. Remove it, or: make link LINK_DIR=<dir>' >&2; \
		exit 1; \
	fi
	@ln -sfn "$(CURDIR)/$(OUT)" "$(LINK)"
	@echo "linked $(LINK) -> $(CURDIR)/$(OUT)"
	@resolved=$$(command -v $(BINARY) 2>/dev/null); \
	if [ -z "$$resolved" ]; then \
		echo "warning: $(LINK_DIR) is not on this shell's PATH; Zed will not find the link either" >&2; \
	elif [ "$$resolved" != "$(LINK)" ]; then \
		echo "warning: PATH resolves $(BINARY) to $$resolved, which shadows the link just made" >&2; \
	fi

unlink:
	@if [ -L "$(LINK)" ]; then \
		target=$$(readlink "$(LINK)"); \
		rm -f "$(LINK)"; \
		echo "removed $(LINK) (was -> $$target)"; \
	elif [ -e "$(LINK)" ]; then \
		echo "leaving $(LINK) alone: it is a real file, not a symlink." >&2; \
		echo 'It is most likely a `make install` copy; remove it by hand if you meant to.' >&2; \
		exit 1; \
	else \
		echo "nothing to remove at $(LINK)"; \
	fi

# What the extension's PATH lookup would actually find, which is not always the
# link you just made: a real binary earlier in PATH shadows it, and a GUI-
# launched editor may not share this shell's PATH at all.
link-status:
	@echo "link:     $(LINK)"
	@if [ -L "$(LINK)" ]; then \
		echo "          -> $$(readlink "$(LINK)")"; \
	elif [ -e "$(LINK)" ]; then \
		echo "          (a real file, not a symlink)"; \
	else \
		echo "          (absent)"; \
	fi
	@echo "build:    $(CURDIR)/$(OUT)"
	@if [ -x "$(CURDIR)/$(OUT)" ]; then \
		echo "          (built)"; \
	else \
		echo "          (not built — run: make build)"; \
	fi
	@resolved=$$(command -v $(BINARY) 2>/dev/null); \
	if [ -n "$$resolved" ]; then \
		echo "on PATH:  $$resolved"; \
		[ "$$resolved" = "$(LINK)" ] || echo "          note: this shadows $(LINK)" >&2; \
	else \
		echo "on PATH:  not found — the extension would download a release instead"; \
	fi

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

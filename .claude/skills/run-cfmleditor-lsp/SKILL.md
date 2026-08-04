---
name: run-cfmleditor-lsp
description: build, run, smoke-test, and drive cfmleditor-lsp — the CFML LSP server and CLI tool
---

`cfmleditor-lsp` is a Go binary with two surfaces: a set of CLI subcommands (`parse`, `scan`, `format`, `unresolved`, `refs`, `deps`, `explain`, `version`, `help`) and an LSP server that communicates over stdio via JSON-RPC 2.0 with Content-Length framing. The driver is `smoke.sh`, a shell script that builds the binary and exercises both surfaces.

## Prerequisites

Go 1.26.4 (pinned in `go.mod`) with CGO enabled — the tree-sitter-cfml grammar is a cgo package. No other runtime deps; the binary is self-contained.

```bash
go version   # verify
```

## Build

```bash
make build
# → target/release/cfmleditor-lsp   (the only artifact)
```

`make build` needs network access: it depends on `generate` → `docs` → `scripts/fetch-docs-cfdocs.sh`, which git-clones the cfdocs repo into the gitignored `docs/data/`. The *generated* Go file (`internal/docs/generated_docs.go`) is committed, so when the fetch can't run, build directly instead:

```bash
go build -o target/release/cfmleditor-lsp ./cmd/cfmleditor-lsp
```

The parse-timing debug tool is a separate binary and a separate target:

```bash
make cfparse-build   # → target/release/cfparse
make cfparse <args>  # build + run in one step
```

## Run (agent path) — smoke driver

```bash
bash .claude/skills/run-cfmleditor-lsp/smoke.sh
```

Exits 0 if all 7 checks pass, 1 on any failure. Covers:

- `version` — binary responds with version string
- `parse` — reports correct function count for a sample `.cfc`
- `scan` — passes clean file, detects parse errors in broken file
- `format` — produces formatted output
- LSP `initialize` — server returns capabilities JSON over stdio
- LSP `shutdown` — server responds with `{"result":null}` and exits 0

The script starts with `make build`, so it inherits that target's network requirement (above). If the cfdocs fetch fails, build with `go build` first and comment out the `make build` line rather than treating the failure as a smoke-test failure.

**Check `git status` after a smoke run.** `make build` regenerates `internal/docs/generated_docs.go` from the assembled `docs/data/`, which needs *both* doc sources staged (cfdocs and Lucee). If `docs.lucee.org` is unreachable — some sandboxes and proxies block it — the regenerated file loses every Lucee-sourced entry, leaving a several-hundred-line pure-deletion diff. `make docs` warns when a fetch fails, but the build continues. That churn is an artifact of running the smoke test, not a change you made: `git checkout -- internal/docs/generated_docs.go`.

## Run (human path)

```bash
./target/release/cfmleditor-lsp        # blocks — waits for LSP client on stdio
./target/release/cfmleditor-lsp parse internal/parser/  # parse timing for a dir
./target/release/cfmleditor-lsp scan   internal/parser/  # report parse errors
./target/release/cfmleditor-lsp format path/to/file.cfc  # print formatted output
./target/release/cfmleditor-lsp format -w path/to/file.cfc  # format in-place

# Resolution-debugging subcommands (need a .cfmleditor.json to be useful)
./target/release/cfmleditor-lsp unresolved [--json] [--verbose] [--global-defs] <dir>
./target/release/cfmleditor-lsp refs [--mermaid] <component-or-function> <dir>
./target/release/cfmleditor-lsp deps [--mermaid] <dir-or-file>
./target/release/cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]
```

## Driving the LSP server manually

Send Content-Length–framed JSON-RPC over stdin; read the same format back on stdout:

```bash
send_msg() { local m="$1"; printf "Content-Length: %d\r\n\r\n%s" "${#m}" "$m"; }

{ send_msg '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}'; \
  sleep 0.4; \
  send_msg '{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}'; \
  sleep 0.2; \
  send_msg '{"jsonrpc":"2.0","method":"exit","params":null}'; } \
  | ./target/release/cfmleditor-lsp 2>/dev/null
```

## Gotchas

- **`timeout` is not available on macOS** — `coreutils` `gtimeout` isn't either by default. Use a background process + `sleep` + `kill` pattern instead (as `smoke.sh` does with `sleep` inside the pipe).
- **`((PASS++))` under `set -e`** — pre-increment `((var++))` returns the old value; when it's 0 that's falsy and kills the script. Use `VAR=$((VAR+1))` instead.
- **LSP stdin must stay open** — the server exits immediately if stdin closes before `exit` is sent. Always send the full `initialize` → `shutdown` → `exit` sequence, or hold stdin open with `sleep`.
- **Daemon mode** activates only when a `.cfmleditor.json` config file exists in the current directory or one level up. Without it the server runs standalone (no socket).
- **Running from this repo's root means you are testing daemon mode, not standalone.** The repo has its own `.cfmleditor.json` (`workspaceName: testdata`), so `smoke.sh` and any bare `./target/release/cfmleditor-lsp` invocation from the root become the daemon: they listen on a socket *and* serve the stdio client. To exercise the standalone path, run from a directory with no config above it (e.g. `cd /tmp && /path/to/cfmleditor-lsp`).
- **The socket file outlives the daemon process.** The path is `<socketDir>/cfmleditor-<sha256(workspaceName)[:8]>.sock` — `$XDG_RUNTIME_DIR/cfmleditor-lsp/` on Linux when set, otherwise `$TMPDIR/cfmleditor-lsp/` (`internal/daemon/socket.go`). After a smoke run the daemon exits but the file remains; a stale socket is harmless (the next start fails to `Proxy()` into it and re-listens), but don't read its presence as "a daemon is running" — check for the process instead.

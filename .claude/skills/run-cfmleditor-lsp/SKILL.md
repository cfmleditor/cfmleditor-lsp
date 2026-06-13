---
name: run-cfmleditor-lsp
description: build, run, smoke-test, and drive cfmleditor-lsp — the CFML LSP server and CLI tool
---

`cfmleditor-lsp` is a Go binary with two surfaces: a set of CLI subcommands (`parse`, `scan`, `format`, `version`) and an LSP server that communicates over stdio via JSON-RPC 2.0 with Content-Length framing. The driver is `smoke.sh`, a shell script that builds the binary and exercises both surfaces.

## Prerequisites

Go 1.22+ (matches `go.mod`). No other runtime deps — the binary is self-contained.

```bash
go version   # verify
```

## Build

```bash
make build
# → target/release/cfmleditor-lsp
```

Also builds `target/release/cfparse` (parse-timing debug tool) and `target/release/cfmlfmt` (standalone formatter).

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

## Run (human path)

```bash
./target/release/cfmleditor-lsp        # blocks — waits for LSP client on stdio
./target/release/cfmleditor-lsp parse internal/parser/  # parse timing for a dir
./target/release/cfmleditor-lsp scan   internal/parser/  # report parse errors
./target/release/cfmleditor-lsp format path/to/file.cfc  # print formatted output
./target/release/cfmleditor-lsp format -w path/to/file.cfc  # format in-place
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
- **Daemon mode** activates only when a `.cfmleditor.json` config file exists in the project root or one level up. Without it the server runs standalone (no socket).

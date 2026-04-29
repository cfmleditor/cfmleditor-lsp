# cfmleditor-lsp

A Language Server Protocol (LSP) implementation for CFML / ColdFusion, written in Go.

## Build

```sh
go build -o cfmleditor-lsp .
```

## Run

The server communicates over stdio using JSON-RPC 2.0 with LSP headers:

```sh
./cfmleditor-lsp
```

Configure your editor to launch this binary as an LSP server for `.cfm`, `.cfc`, `.cfml`, and `.cfs` files.

## Status

- [x] Initialize / Shutdown / Exit
- [x] textDocument/didOpen
- [x] textDocument/didChange (full sync)
- [x] textDocument/didClose

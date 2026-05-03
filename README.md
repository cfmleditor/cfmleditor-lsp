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

## Configuration

Place a `.cfmleditor.json` file in your project root to enable daemon mode and configure workspace indexing.

```json
{
  "workspaceName": "myproject",
  "workspacePaths": [".", "../shared-lib"]
}
```

| Field | Required | Description |
|---|---|---|
| `workspaceName` | Yes | Unique project name. Used to derive the daemon socket path so multiple projects don't collide. |
| `workspacePaths` | No | Relative paths to folders the LSP should treat as workspace roots. Resolved relative to the config file location. |

### Daemon mode

When `.cfmleditor.json` is found (in the current directory or one level up), the server starts in daemon mode:

1. The first editor session becomes the daemon, listening on a Unix socket and serving LSP over stdio.
2. Subsequent sessions connect to the existing daemon via the socket, sharing a single index.
3. The daemon shuts down automatically when all editor sessions disconnect.

Without a config file the server runs in standalone mode — a single session with its own index.

### Indexing behaviour

- If `workspaceIndexGlobs` is set, only `.cfc` files matching those patterns are indexed.
- If only `workspacePaths` is set, all `.cfc` files under those folders are indexed.
- If neither is set, the LSP falls back to indexing workspace folders reported by the editor.

## Status

- [x] Initialize / Shutdown / Exit
- [x] textDocument/didOpen
- [x] textDocument/didChange (full sync)
- [x] textDocument/didClose


## Local Development

Add cfmleditor-lsp to your path

```bash
sudo ln -sf ~/development/github/cfmleditor-lsp/target/release/cfmleditor-lsp /usr/local/bin/cfmleditor-lsp
```

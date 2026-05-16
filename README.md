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
  "workspacePaths": [".", "../shared-lib"],
  "mappings": {
    "models": "./src/models"
  },
  "componentResolvers": [
    {
      "match": "getService(\"$1\")",
      "resolve": "packages.$1.service",
      "prefix": "getService"
    }
  ]
}
```

| Field | Required | Description |
|---|---|---|
| `workspaceName` | Yes | Unique project name. Used to derive the daemon socket path so multiple projects don't collide. |
| `workspacePaths` | No | Relative paths to folders the LSP should treat as workspace roots. Resolved relative to the config file location. |
| `workspaceIndexGlobs` | No | Glob patterns to filter which `.cfc` files are indexed. |
| `mappings` | No | Component path mappings. Keys are the first segment of a dot-path, values are directory paths (absolute or relative to config). |
| `componentResolvers` | No | Custom patterns for resolving method calls to component paths. See below. |

### Mappings

Mappings let you resolve component dot-paths that use a virtual root. For example, with `"models": "./src/models"`, the dot-path `models.User` resolves to `./src/models/User.cfc`.

### Component resolvers

Component resolvers teach the LSP how to resolve custom factory patterns to specific CFCs. This enables goto-definition and dot-completion for variables assigned from those patterns.

```json
"componentResolvers": [
  {
    "match": "getService(\"$1\")",
    "resolve": "packages.$1.service",
    "prefix": "getService"
  },
  {
    "match": "_parent",
    "resolve": "packages.tass.core.kernel2",
    "prefix": "_parent"
  }
]
```

| Field | Required | Description |
|---|---|---|
| `match` | Yes | Pattern to match against the RHS of an assignment. Use `$1` as a capture placeholder. Without `$1`, acts as an exact variable name match. |
| `resolve` | Yes | Component dot-path or file path template. `$1` is replaced with the captured value. File paths (with `/` or `.cfc`) are normalised to dot-paths. |
| `prefix` | Yes | Fast-check string. Lines without this prefix are skipped entirely — avoids expensive matching on every line. |

The match is case-insensitive and works regardless of qualifiers before it. For example, `getService("$1")` matches all of:
- `getService("timetable")`
- `_parent.getService("timetable")`
- `VARIABLES._parent.getService("timetable")`

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

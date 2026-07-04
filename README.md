# cfmleditor-lsp

A Language Server Protocol (LSP) implementation for CFML / ColdFusion, written in Go.

Uses [tree-sitter-cfml](https://github.com/cfmleditor/tree-sitter-cfml).

## Build

```sh
make build
```

Or manually:

```sh
go build -trimpath -ldflags="-s -w" -o cfmleditor-lsp ./cmd/cfmleditor-lsp
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
| `formatting` | No | Formatter configuration object. See below. |
| `debug` | No | Enable debug logging (`zap.NewDevelopment`). Outputs verbose logs to stderr. |

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
| `prefix` | Yes | Fast-check string. Lines without this prefix are skipped entirely — avoids expensive matching on every line. Pipe-delimit multiple alternatives (e.g. `"createModel\|buildModel"`) to share one `match`/`resolve` pair across call-site shapes that don't start with a common substring. |

The match is case-insensitive and works regardless of qualifiers before it. For example, `getService("$1")` matches all of:
- `getService("timetable")`
- `_parent.getService("timetable")`
- `VARIABLES._parent.getService("timetable")`

### Formatting

The `formatting` object controls the built-in formatter invoked via `textDocument/formatting`.

```json
"formatting": {
  "enabled": true,
  "selfCloseTags": true,
  "whitespaceOnly": true,
  "queryFormat": false,
  "lowercaseTags": true,
  "lowercaseAttributes": true,
  "doubleQuoteAttributes": true,
  "queryUppercaseKeywords": true,
  "scopeCase": "leave",
  "commaPosition": "after",
  "queryCommaPosition": "preserve",
  "lineWidth": 100,
  "attrBreakThreshold": 4,
  "indentWidth": 4
}
```

| Field | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable the formatter. When false, formatting requests are ignored. |
| `selfCloseTags` | `true` | Convert void/implicit-end HTML tags to self-closing form (e.g. `<br>` → `<br />`). |
| `whitespaceOnly` | `true` | Reject formatting results that change non-whitespace content (safety guard). |
| `queryFormat` | `false` | Format `<cfquery>` content (SQL re-indentation, keyword casing). When false, query content is emitted verbatim. |
| `lowercaseTags` | `true` | Lowercase CF tag names (e.g. `<CFOUTPUT>` → `<cfoutput>`). |
| `lowercaseAttributes` | `true` | Lowercase attribute names. |
| `doubleQuoteAttributes` | `true` | Normalize attribute values to double quotes. |
| `queryUppercaseKeywords` | `true` | Uppercase SQL keywords inside `<cfquery>` blocks. |
| `scopeCase` | `"leave"` | Case for CFML scope names. Values: `"upper"`, `"lower"`, `"leave"`. |
| `commaPosition` | `"after"` | Comma placement in multi-line argument lists. Values: `"after"` (trailing), `"before"` (leading). |
| `queryCommaPosition` | `"preserve"` | Comma placement in SQL SELECT lists. Values: `"preserve"` (keep original position), `"after"` (trailing), `"before"` (leading). |
| `lineWidth` | `100` | Soft column limit — attributes expand to separate lines when a tag exceeds this width. |
| `attrBreakThreshold` | `4` | Number of attributes above which they are always expanded onto separate lines. |
| `indentWidth` | `4` | Spaces per indentation level. Overridden by editor `tabSize` when provided. |
| `debug` | `false` | Enable formatter debug checks. |

Note: `useTabs` and `tabSize` are taken from the editor's formatting options (sent with each formatting request), not from this config.

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

### Make commands

| Command | Description |
|---|---|
| `make build` | Update grammar, generate docs, and build the binary. |
| `make test` | Run all tests. |
| `make lint` | Run golangci-lint. |
| `make lint-fix` | Run golangci-lint with auto-fix. |
| `make update-grammar` | Regenerate docs and tree-sitter grammar, clear Go build cache. |
| `make release <version>` | Full release: validate, build, test, lint, update changelog, commit, tag, push. |
| `make install` | Build and copy binary to `$GOPATH/bin`. |
| `make clean` | Remove build artifacts. |

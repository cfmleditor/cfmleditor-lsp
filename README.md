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

The same settings can also be supplied by your editor as LSP `initializationOptions`, which is useful when you would rather not add a file to the project. `.cfmleditor.json` takes priority: it wins on every key it sets, and editor settings fill in the rest. See [Editor settings](#editor-settings) below.

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
| `completions` | No | `tagSnippets`, `functionSnippets`, `globalFunctionResolution`. All three default to `true`; set the block only to turn one off. |
| `references` | No | `textDocument/references` support, off by default. See below. |
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
| `anchored` | No (default `false`) | Require `prefix` at the *start* of the expression rather than anywhere inside it. See below. |
| `noFollow` | No (default `false`) | Accept a call through this resolver without checking that the method exists on the resolved component. Use it for dynamic factories, or Java objects whose stubs are incomplete. |

The match is case-insensitive and works regardless of qualifiers before it. For example, `getService("$1")` matches all of:
- `getService("timetable")`
- `_parent.getService("timetable")`
- `VARIABLES._parent.getService("timetable")`

#### `anchored`

By default `prefix` is searched for *anywhere* in the expression, and matching starts from
wherever it is found. That is what makes the qualifier-insensitivity above work, but it also
means a resolver can claim an expression that merely contains its prefix:

- `{"prefix": "document", "match": "document", "resolve": "app.document"}` also fires on the
  unrelated variable `domobject_document`.
- A deliberately broad catch-all such as
  `{"prefix": "get", "match": "get$1()", "resolve": "packages.tass.${1:lower}"}`, written for a
  family of bare factory calls, also fires on the `getDirectContent()` at the end of
  `VARIABLES._document.getDirectContent()` — resolving it to `packages.tass.directcontent`.

In both cases the shortened expression matches the pattern exactly, so the wrong component is
produced confidently rather than the resolver simply declining.

Setting `"anchored": true` requires the prefix at position 0, so the resolver only claims
expressions that genuinely start with it. Both examples above stop firing, while
`getPageTools()` — the bare factory call the catch-all was written for — still resolves.
Anchoring also makes the order of pipe-delimited alternatives irrelevant, since every
alternative that matches matches at the same position.

Anchoring is off by default because a resolver aimed at a *call* usually does want to match
through a receiver (`VARIABLES._parent.getService("x")`). Reach for it when a resolver is aimed
at a variable name, or when a broad catch-all is producing wrong answers — `cfmleditor-lsp
explain` will name the resolver that fired.

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
| `enabled` | `false` | Enable the formatter. When false, formatting requests are ignored. Omitting it leaves whatever the editor's settings said, rather than switching the formatter off. |
| `selfCloseTags` | `true` | Convert void/implicit-end HTML tags to self-closing form (e.g. `<br>` → `<br />`). |
| `whitespaceOnly` | `true` | Reject formatting results that change non-whitespace content (safety guard). |
| `queryFormat` | `false` | Format `<cfquery>` content (SQL re-indentation, keyword casing). When false, query content is emitted verbatim. |
| `lowercaseTags` | `true` | Lowercase CF tag names (e.g. `<CFOUTPUT>` → `<cfoutput>`). When false, each tag keeps the casing it was written with, opening and closing halves independently. |
| `lowercaseAttributes` | `true` | Lowercase attribute names. |
| `doubleQuoteAttributes` | `true` | Normalize attribute values to double quotes. |
| `queryUppercaseKeywords` | `true` | Uppercase SQL keywords inside `<cfquery>` blocks. |
| `parenSpacing` | *(unset)* | Padding inside parentheses. `"pad"` gives `if ( a )` and `foo( 1, 2 )`; `"tight"` gives `if (a)` and `foo(1, 2)`. Unset keeps the existing behaviour, which pads conditions and grouping but not argument lists — set it to get one rule in both places. |
| `scopeCase` | `"leave"` | Case for CFML scope names. Values: `"upper"`, `"lower"`, `"leave"`. |
| `commaPosition` | `"after"` | Comma placement in multi-line argument lists. Values: `"after"` (trailing), `"before"` (leading). |
| `queryCommaPosition` | `"preserve"` | Comma placement in SQL SELECT lists. Values: `"preserve"` (keep original position), `"after"` (trailing), `"before"` (leading). |
| `lineWidth` | `100` | Soft column limit — attributes expand to separate lines when a tag exceeds this width. |
| `attrBreakThreshold` | `4` | Number of attributes above which they are always expanded onto separate lines. |
| `indentWidth` | `4` | Spaces per indentation level. Overridden by editor `tabSize` when provided. |
| `debug` | `false` | Enable formatter debug checks. |

Note: `useTabs` and `tabSize` are taken from the editor's formatting options (sent with each formatting request), not from this config.

### References

`textDocument/references` — the editor's "Find All References" — is off by default and enabled per workspace:

```json
{
  "references": { "enabled": true }
}
```

The capability is advertised to the editor only when it is on, so a workspace that has not opted in does not see the command at all.

It is opt-in because of what one request costs. Answering it walks and parses every CFML file under the workspace roots, the same scan the `refs` CLI and the `cfmleditor.findRefs` command already do; there is no incremental index of call sites to answer from. On a few hundred files that is imperceptible, and on a few thousand it is a noticeable pause during which the server is busy. Whether that trade is worth making by default is the thing the flag exists to find out.

What it answers depends on what the cursor is on:

| Cursor on | Returns |
|---|---|
| A function name, declared or called | Every call site that resolves to that function, with calls to same-named functions on other components excluded |
| A component dot-path (`new models.UserDAO()`, `extends`, `<cfinvoke component>`) | Every place that path is written |

The search is scoped by the file that *declares* the function, which is resolved first by the same rules go-to-definition uses. That is what makes the request work with the cursor on a call site rather than only on the declaration.

`includeDeclaration` is honoured. A function the server cannot pin to a single declaration — several same-named functions across the workspace, none of them in the current file — falls back to scoping the search by the requesting document rather than picking one of them, so the answer is narrow rather than wrong.

### Editor settings

Every field above can be sent as LSP `initializationOptions` instead of, or alongside, `.cfmleditor.json`. The payload has exactly the same shape as the file.

In Zed, via `settings.json`:

```json
{
  "lsp": {
    "cfmleditor-lsp": {
      "initialization_options": {
        "linting": { "enabled": true },
        "mappings": { "models": "./src/models" }
      }
    }
  }
}
```

In VS Code and most other clients the equivalent key is `initializationOptions`.

Precedence, when both are present:

| | Result |
|---|---|
| Key set in `.cfmleditor.json` | The file's value wins |
| Key set only in editor settings | The editor's value applies |
| `formatting` set on both | Merged key by key — the file wins on the keys it names, the editor's other keys stand |
| `mappings`, `beanPaths`, and other maps | Merged per key; the file wins on conflicts |
| `componentResolvers`, `propertyResolvers` | Both apply, with the file's entries tried first |

Relative paths resolve against the directory of whichever source declared them — the config file's own directory, or the first workspace folder for editor settings.

Two caveats:

- Settings are read once, at `initialize`. Changing them requires restarting the language server.
- `debug` is ignored here, because the logger is constructed before the client connects. Use `.cfmleditor.json` for that one.

### Daemon mode

When `.cfmleditor.json` is found, the server starts in daemon mode. The search walks upwards from the current directory to the filesystem root, and the nearest config wins:

1. The first editor session becomes the daemon, listening on a Unix socket and serving LSP over stdio.
2. Subsequent sessions connect to the existing daemon via the socket, sharing a single index.
3. The daemon shuts down automatically when all editor sessions disconnect.

Without a config file the server runs in standalone mode — a single session with its own index. Standalone sessions look for a config the same way, walking upwards from each workspace folder the editor reports, so the same file is picked up in either mode; what they do not do is join a daemon.

That is deliberate. The socket is derived from `workspaceName`, so an index is only ever shared between sessions that named the same project. With no config there is no name to key on, and the alternative — falling back to the working directory — is not one: it groups whatever happens to share a folder name, and an editor that starts the server without setting a working directory (the IntelliJ plugin does not) gives every project on the machine the same one. A shared index means one project's symbols answering another's go-to-definition and workspace-symbol queries. Add a `.cfmleditor.json` with a `workspaceName` to get the sharing.

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

### Using a local build in an editor

`make link` builds this working tree and symlinks it onto your `PATH`:

```bash
make link          # symlink into `go env GOPATH`/bin
make link-status   # show what the link points at, and what PATH actually resolves
make unlink        # remove it
```

The link points at `target/release/cfmleditor-lsp`, so a later `make build` takes
effect without re-linking.

This is how the [zed-cfml](https://github.com/cfmleditor/zed-cfml) extension picks
up a local build. It resolves its server in three steps: a path it has already
cached, then a `cfmleditor-lsp` found on `PATH`, and only then a download of a
GitHub release. A symlink on `PATH` wins at the second step, so no release is
downloaded. Restart the editor after linking or unlinking — the resolved path is
cached for the life of the session.

Symlinking into the extension's own directory instead would not survive: that
directory is named after the release version, and the extension removes the
versions it is not using.

If `go env GOPATH`/bin is not on your `PATH`, point the link somewhere that is:

```bash
make link LINK_DIR=$HOME/.local/bin
make unlink LINK_DIR=$HOME/.local/bin   # same LINK_DIR to remove it
```

`make link` refuses to overwrite a real file at that path — normally a `make
install` copy — rather than silently replacing it, and warns when the link it
just made is shadowed by another `cfmleditor-lsp` earlier on `PATH`.

### Make commands

| Command | Description |
|---|---|
| `make build` | Update grammar, generate docs, and build the binary. |
| `make test` | Run all tests. |
| `make lint` | Run golangci-lint. Use an official v2 release binary — one built with `go install` is compiled against golangci-lint's own (older) Go toolchain and then refuses this repo's newer `go.mod` target: *"the Go language version used to build golangci-lint is lower than the targeted Go version"*. |
| `make lint-fix` | Run golangci-lint with auto-fix. |
| `make vuln` | Scan dependencies and the stdlib for known vulnerabilities (govulncheck). |
| `make update-grammar` | Regenerate docs and tree-sitter grammar, clear Go build cache. |
| `make release <version>` | Full release: validate, build, test, lint, update changelog, commit, tag, push. |
| `make install` | Build and copy binary to `go env GOPATH`/bin. |
| `make link` | Build, then symlink the binary onto `PATH` for local editor use. Override the directory with `LINK_DIR=<dir>`. |
| `make unlink` | Remove that symlink. |
| `make link-status` | Show the link, the build, and what `PATH` resolves `cfmleditor-lsp` to. |
| `make clean` | Remove build artifacts. |

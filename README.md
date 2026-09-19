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

The Go toolchain version is pinned in `go.mod`, and CGO is required (the
tree-sitter grammar is C).

### If you keep a `go.work`

A `go.work` is the normal setup for anyone working on the grammar and the server
together, and it is gitignored — so nothing in the repo updates it for you. It
carries its own `go` directive, and the build refuses to start when that is
older than `go.mod`'s:

```
go: module . listed in go.work file requires go >= 1.27.1, but go.work lists go 1.26.8
```

Match it to the version in `go.mod` whenever that is bumped:

```sh
go work use
```

### Keep gopls on the same Go release

gopls type-checks with the `go/types` of the Go release it was *built with*, not
the one on your `PATH`. A gopls built with an older Go rejects syntax that
release did not have, so after a `go.mod` bump your editor fills with errors on
code that builds and lints cleanly. Go 1.27 added `new(expr)` and allowed any
valid field selector as a struct literal key, and both are used here, so a gopls
built with 1.26 reports roughly a hundred and thirty phantom errors — mostly
`unknown field X in struct literal`.

Check what yours was built with, and rebuild it against the pinned toolchain if
it is behind:

```sh
go version -m $(command -v gopls) | head -1
GOTOOLCHAIN=go1.27.1 go install golang.org/x/tools/gopls@latest
```

Editors that manage their own copy need pointing at the rebuilt one, or their
cached binary removed so it is fetched again — Zed keeps its under
`~/Library/Application Support/Zed/languages/gopls/`, and the filename records
the Go it was built with.

This is the same trap `make lint` documents at length for golangci-lint, which
is why the Makefile builds that from source under this module's own toolchain.
gopls cannot be pinned the same way, because the build never invokes it.

## Run

The server communicates over stdio using JSON-RPC 2.0 with LSP headers:

```sh
./cfmleditor-lsp
```

Configure your editor to launch this binary as an LSP server for `.cfm`, `.cfc`, `.cfml`, and `.cfs` files.

## Code map

`cfmleditor-lsp graph` builds a map of a whole project: every function and file, and
the calls, instantiations, inheritance and includes between them. On an 11,700-file
workspace that is around 60,000 nodes and 105,000 edges in roughly ten seconds.

```sh
cfmleditor-lsp graph .                                     # summary to the terminal
cfmleditor-lsp graph --format html --out map.html .         # interactive report
cfmleditor-lsp graph --level package --format dot . | dot -Tsvg > map.svg
cfmleditor-lsp graph --db .cfmleditor/codemap.db .          # save it, and cache it
```

**Every declared function is in the map, whether or not anything calls it.** Code
nothing reaches is not dropped and not merged into the main graph: it becomes its own
*island*, with its own root, which the HTML report draws as a separate tree and
`--detached` lists on its own. A map that quietly omitted what it could not connect
would describe a tidier codebase than the one on disk.

### Levels

| `--level` | Nodes | For |
|---|---|---|
| `function` (default) | functions **and** files | everything; files carry the relationships functions cannot |
| `call` | functions, plus pages whose top-level code calls something | a strict call graph: shortest paths, real cycles, functions nobody invokes |
| `file` | one per file | module-scale dependency structure |
| `package` | one per directory | the level that actually renders as a picture of a large project |

`function` is deliberately a hybrid. Three of the four relationships a CFML codebase
has are between *files* — a component extends a component, a page includes a page,
`new Foo()` names a component it may never call a method on — so dropping file nodes
drops all of that. Use `call` when you want only "what calls what".

### Formats

`text` (default), `json`, `jsonl`, `dot`, `mermaid`, `html`.

`json` is the canonical artifact; everything else is a view of it. `mermaid` is capped
on purpose — it renders in a browser and falls over in the low thousands of nodes, so a
map big enough to need a cap should be collapsed to `--level package` first.

`html` is a single self-contained file with four views: hierarchical **edge bundling**
(grouped by island, then directory), a **force** layout that gives each island its own
centre so detached code sits apart rather than being pressed against the border, a
**dependency matrix** that has no occlusion at any size, and an **islands** view of the
disconnected pieces. Above ~1,200 nodes the force view renders to a canvas with a
quadtree for hit-testing, so it stays interactive into the tens of thousands.

The report embeds its JavaScript — a 76KB D3 bundle of just the modules these views
use, built from `internal/codemap/assets/vendor` and committed. No CDN, no network:
the file opens the same on an air-gapped machine and still renders years later.

### Scoping

`--under <path>` narrows the map to a prefix **and keeps the nodes just outside it
that an edge crosses into**, marked as boundary nodes. Cutting hard at the prefix
instead removes every caller from elsewhere — which is exactly what you scoped the
map down in order to see — and the package comes back looking uncalled rather than
narrow. `--under-strict` does the hard cut if you want it.

A map collapsed with `--level file` or `--level package` has no function nodes in
it, so searching one for a function name can only come back empty; the HTML report
says so rather than showing nothing.

### Framework routes

Convention-based routing is invisible to static analysis. A dispatcher reads a
dotted route out of a URL or an HTML attribute, builds a component path and a
method name from it and invokes them — nothing in the source names either, so
every routed controller method looks uncalled and every view unreferenced.

`routes` in `.cfmleditor.json` describes the convention, and the LSP then follows
it: route edges in the code map, ctrl-click to the controller method or the view
from go-to-definition, and a document link on each resolvable route.

```jsonc
"routes": {
  // where routes are written — all four are optional
  "attributes":  ["data-view", "data-read", "data-process"],
  "queryParams": ["do"],                        // href="x.cfm?do=a.b.c"
  "properties":  ["read", "view", "process"],   // { view: "a.b.c" }
  "functions":   ["redirect", "setPrint"],      // redirect("a.b.c")

  "aliases": { "ui.web": ["tassweb", "kiosk", "parentportal"] },
  "controllers": [
    { "component": "packages.tass.${1}-${2}", "method": "${3+:search}" },
    { "component": "packages.tass.${2}",      "method": "${3+:search}" },
    { "component": "packages.tass.${1}",      "method": "${2+:search}" }
  ],
  "views": [
    { "longestDir": true, "ext": [".cfm"] },
    { "longestDir": true, "root": "..", "ext": [".cfm"] }
  ]
}
```

**Four syntaxes, because routes are not written one way.** A URL parameter's value
sits inside the enclosing `href`'s quotes, so it runs to the next delimiter rather
than to a quote; a function argument often carries a query string or fragment after
the route, so it is cut at the first delimiter rather than rejected for holding
one. JavaScript properties are the loosest — `read` and `view` are ordinary words —
which is why a value must look like a route before it is resolved at all.

`${N}` is one segment, `${N+}` everything from N on, `${N-M}` a span. A `:concat`
suffix joins without the dots (`dialog.custom.roll` → `dialogCustomRoll`),
`:slash` with them, `:lower`/`:upper` fold the case; the default joins with dots.
A rule that reaches past the end of a route declines it rather than matching a
truncated path.

**`:search` enumerates start points**, longest first. A route's segments do not say
where the controller's name stops and the method's begins — the same shape is
spelled `studentMainStudent()` on one controller and `mainStudent()` on another —
so one rule covers both instead of one rule per start point. It is safe only
because every candidate is still checked against the component's real methods.

**Controller rules are tried in order and the method must exist**, which is what
makes the order safe: a component template built from the first segment matches
enormous numbers of routes, so without the method check it would shadow every
rule below it.

**Aliases can name several replacements.** `ui.web` means "the product serving
this page", and a view shared between products reaches whichever one is running —
which cannot be known statically. All of them are returned, the code map marks
the edge as a guess, and go-to-definition offers the choice.

`views` either takes a `path` template or `longestDir`, which finds the longest
leading run of segments that names a real directory and treats the rest as a
dotted file name (`ui.web.general.popup.lookup.filter` is
`ui/web/general/popup.lookup.filter.cfm`). No template can express that, because
the split depends on what is on disk.

The same grammar covers FW/1 — `{ "component": "controllers.${1}", "method":
"${2}" }` with `{ "path": "views/${1}/${2}" }` — which is the test that keeps it
from being one framework's rules in disguise.

`cfmleditor-lsp routes <dir>` reports what was found and what it resolved to, and
`--unresolved` groups what it could not so a missing *rule* is visible: one
unresolved route is usually noise, forty sharing a prefix is a shape the config
does not cover.

```sh
cfmleditor-lsp routes --unresolved .                        # to the terminal
cfmleditor-lsp routes --format md --out routes.md .          # a report to keep
cfmleditor-lsp routes --format json . | jq                   # for a script
```

The markdown report is the one to keep. It groups by *shape* — how many unresolved
routes have a `dialog` segment, a `popup`, an action verb — because a whole row is
usually one missing rule rather than one problem per route. It also separates the
prefixes that resolve elsewhere from the ones that never do: the first means the
controller is found and only the method name is underivable, the second means the
config cannot locate the controller at all, and those are different fixes. Every
route is cited with its file and line, so it can be committed beside the config or
handed to whoever knows the framework.

The build reports the same share. A low one means the config describes a different
convention from the one in use, and the route edges are worth correspondingly
less.

### Per-application configs, and code a runner invokes

A workspace is often several applications side by side, each with its own
`.cfmleditor.json` — and each listing the others in `workspacePaths`, so any scan
reads all of them. By default each file is now resolved under **its own** nearest
config, because applying one application's `componentResolvers` to another's source
does not fail loudly: it resolves nothing, and those files come out of the map with
no edges. `--one-config` restores the old single-config behaviour.

`--entry <glob>` marks files as entry points by path. Some code is invoked by a
runner that constructs its name, so nothing in the codebase names it and no static
analysis can see the call — release-script directories, scheduled tasks, plugin
folders. On one workspace 10,639 of 11,374 apparently-unreferenced functions were
release scripts of exactly that kind:

```sh
cfmleditor-lsp graph --entry '../prs' --entry 'tasks/*' .
```

A bare directory name matches everything beneath it. Private methods are never
marked, since a runner reaching in by a constructed name cannot reach one.

**Calls from top-level page code count.** A `.cfm` page is mostly code with no
enclosing function, so its calls are attributed to the file node. That is not a
fallback: on a real workspace, file-level callers account for 74,226 of 107,683
call sites — more than every function-level caller combined.

### Database and cache

`--db <file>` writes the map to SQLite and reuses a per-file cache on the next run.
The cache is keyed on each file's content hash *and* a fingerprint of every indexed
component, because resolving a call reads the index built from every other `.cfc` —
so editing any component invalidates the cache and the rebuild is a full one, while
editing `.cfm` pages leaves it valid. On a large workspace that is roughly 15 seconds
cold against 7 warm.

The schema is the contract, and `sqlite3` is a supported way to use it:

```sql
-- the de-facto API: what the most code depends on
SELECT id, in_degree FROM nodes ORDER BY in_degree DESC LIMIT 20;

-- functions nothing reaches, in one package
SELECT id, file, line FROM nodes
WHERE kind = 'function' AND reachable = 0 AND file LIKE 'packages/tass/%';
```

### MCP server

`cfmleditor-lsp mcp --db <file>` serves that map over the Model Context Protocol on
stdio, so an assistant can ask about structure instead of grepping for it. It is
read-only by construction: every tool is a query, and none writes a file or runs a
command.

```jsonc
{
  "mcpServers": {
    "cfmleditor-codemap": {
      "command": "cfmleditor-lsp",
      "args": ["mcp", "--db", ".cfmleditor/codemap.db"]
    }
  }
}
```

Tools: `search_symbols`, `get_symbol`, `get_callers`, `get_callees`, `find_path`,
`list_islands`, `list_orphans`, `get_stats`, and `explain_call` — which re-parses a
file and traces, step by step, how a call site's receiver was typed and which
`componentResolver` fired.

**Read `get_stats` before trusting an empty caller list.** The resolved/unresolved
ratio is the map's confidence in itself: a call the resolver could not follow is an
edge the map does not have, so on a workspace resolving around half its call sites,
"nothing calls this" means rather less than it looks like it does. That is also why
the unreferenced list is described as candidates rather than as dead code.

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
| `features` | No | Per-capability switches. `documentHighlight`, `watchedFiles` and `rangeFormatting` default to `true`; `folding` defaults to `false` and is opt-in. See below. |
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
| `braceStyle` | `"same-line"` | Where a block's opening brace goes. `"same-line"` (K&R) keeps `function f() {`; `"next-line"` (Allman) puts the brace alone on the line below, and moves `else`, `catch` and `finally` onto their own lines so their braces line up too. A `{ … }` block that is a statement in its own right is left alone — there is no header for its brace to go under. |
| `blankLinesInBlocks` | `true` | Pad a block's body with a blank line after the opening brace and before the closing one. False gives compact blocks, and collapses an empty body from three lines to two. |
| `switchCaseIndent` | `false` | Indent `case` and `default` labels one level inside the switch, with their statements one further in. False keeps the label at the `switch` keyword's own column. |
| `scopeCase` | `"leave"` | Case for CFML scope names. Values: `"upper"`, `"lower"`, `"leave"`. |
| `commaPosition` | `"after"` | Comma placement in multi-line argument lists. Values: `"after"` (trailing), `"before"` (leading). |
| `queryCommaPosition` | `"preserve"` | Comma placement in SQL SELECT lists. Values: `"preserve"` (keep original position), `"after"` (trailing), `"before"` (leading). |
| `lineWidth` | `100` | Soft column limit — attributes expand to separate lines when a tag exceeds this width. |
| `paramBreakThreshold` | `0` | Number of parameters above which a function declaration's parameter list is expanded onto separate lines. `0` expands every list that has parameters, which is what the formatter has always done; raise it to keep short signatures on one line. A list at or below the threshold still expands when it would run past `lineWidth`, or when it holds a comment or a trailing comma. |
| `attrBreakThreshold` | `4` | Number of attributes above which they are always expanded onto separate lines. |
| `indentWidth` | `4` | Spaces per indentation level. Overridden by editor `tabSize` when provided. |
| `debug` | `false` | Enable formatter debug checks. |

Note: `useTabs` and `tabSize` are taken from the editor's formatting options (sent with each formatting request), not from this config.

The editor's `insertFinalNewline` and `trimFinalNewlines` (LSP 3.15) are honoured too. The
formatter rebuilds the document rather than editing it, so left to itself it always ends its
output with exactly one newline — and VS Code's defaults for both of those settings are `false`,
which would mean adding a final newline the editor said not to add and dropping trailing blank
lines it said to keep. A client that sends neither option gets that single trailing newline, as
before.

`trimTrailingWhitespace` is deliberately **not** honoured. For the same reason — output is
rebuilt from the syntax tree, not patched — the formatter has no trailing whitespace to keep, so
there is nothing it could honestly do with `false` short of declining to format.

### Features

Most capabilities the server adds are on by default and each can be switched off
on its own, for when one misbehaves on a real workspace and the alternative is
downgrading the binary. **`folding` is the exception: it is off unless you ask
for it.**

```json
{
  "features": {
    "documentHighlight": true,
    "folding": false,
    "watchedFiles": true,
    "rangeFormatting": true
  }
}
```

| Key | Default | Controls |
|---|---|---|
| `documentHighlight` | on | Shading the other occurrences of the identifier under the cursor. |
| `folding` | **off** | Syntax-aware folding ranges. With it off the editor folds by indentation, as it did before the feature existed. |
| `watchedFiles` | on | Re-indexing files changed outside the editor. With it off the index reflects startup plus whatever you have had open, and `cfmleditor.reindex` is the way to refresh it. |
| `rangeFormatting` | on | "Format Selection". Switching it off leaves whole-document formatting and format-on-save working. |

`folding` is opt-in because it is the most expensive request here to answer. A
script-syntax component reaches the CFML grammar as one opaque region, so
folding it means parsing the whole component body with the CFScript grammar on
every request — a few milliseconds on a large component, and there is no way to
answer it more cheaply without holding a parse tree per open document. Turn it
on with `{"features": {"folding": true}}` if you want it.

Set only the keys you want to change — the block is merged key by key, so naming
one leaves the rest alone, and an editor's `initializationOptions` and a project's
`.cfmleditor.json` can each set different ones.

Switching one off *un-advertises* it rather than leaving the server to decline
the request, so the editor falls back to its own behaviour instead of offering a
command that returns nothing.

`rangeFormatting` sits under `formatting.enabled` rather than replacing it:
formatting has to be on at all for either kind to run. It has its own switch
because it is the only one of the four that writes to your buffer, and it shares
all its machinery with format-on-save — without a separate switch, stopping it
would mean giving up format-on-save too.

`linting` and `references` are the same kind of switch and keep their own
top-level keys, `references` additionally defaulting to *off*.

### Linting

CFLint diagnostics are off by default and enabled per workspace. The binary is
downloaded from the `cfmleditor/CFLint` releases on first use, unless a `cflint`
is already on `PATH`, which wins:

```json
{
  "linting": { "enabled": true, "minSeverity": "WARNING" }
}
```

`minSeverity` is the least severe CFLint level still reported, named on CFLint's
own scale — `FATAL`, `CRITICAL`, `ERROR`, `WARNING`, `CAUTION`, `INFO`,
`COSMETIC`, in that order. Anything below it is dropped rather than merely made
quiet. Unset — the default — reports everything, and an unrecognised name is
ignored with a warning in the log rather than silently filtering nothing.

It is worth setting because of how severities are mapped. CFLint's seven levels
have to fold onto the LSP's four, and every one of them lands on Error or
Warning: an editor shows neither Hint nor Information by default — VS Code draws
a Hint as a faint underline and keeps it out of the Problems panel, and hides
Information unless "Show Infos" is ticked — so a level mapped to either would be
published, logged, and then invisible, which is indistinguishable from a
diagnostic that was never produced.

The cost of that is that advisory rules (`OUTPUT_ATTR`, `IMPLICIT_SCOPE`,
`ARG_VAR_MIXED`) arrive as loud as real ones. `"minSeverity": "WARNING"` is the
way back to only the rules worth acting on, and it is a filter on CFLint's scale
rather than on the mapped severity, so it can still tell an `INFO` from a
`WARNING` after the two have folded together.

| Level | LSP severity |
|---|---|
| `FATAL`, `CRITICAL`, `ERROR` | Error |
| `WARNING`, `CAUTION` | Warning |
| `INFO`, `COSMETIC` | Warning |
| Anything else | Warning, and never filtered by `minSeverity` |

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

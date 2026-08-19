# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # generate docs + build binary to target/release/cfmleditor-lsp
make test           # go test ./...
make lint           # golangci-lint run ./...
make lint-fix       # golangci-lint run --fix ./...
make vuln           # govulncheck ./... (pinned scanner, GOWORK=off)
make fmt            # gofmt -w . && golangci-lint run --fix ./...
make install        # build and copy to $GOPATH/bin
make update-grammar # bump tree-sitter-cfml, regen docs + injections.scm, clear build cache
make cfparse        # build + run the parser-benchmark CLI (cmd/cfparse)
make visualtest     # go test -v -run TestFormatOutput ./internal/formatter/
make corpus CORPUS=<dir>[:<dir>...] [REPORT=<file>]
                    # format a real-world CFML corpus and report what the formatter did to
                    # each file (clean / grammar-refused / guard-rejected / not idempotent);
                    # skipped without CORPUS, so it never runs in CI. See FORMATTER-ISSUES.md
make build-wasm     # wasip1/wasm build (needs WASI_SDK, default /opt/wasi-sdk)
make release <ver>  # validate, build, test, lint, changelog, commit, tag, push
make release-dry <ver>
make clean          # remove target/

# Run a single test
go test ./internal/parser/... -run TestParseFunctionDefs_MixedTagAndCFScript
go test -v ./internal/formatter/ -run TestFormatOutput

# Benchmarks live in internal/parser/{benchmark,tassweb_bench}_test.go
go test ./internal/parser/ -bench . -run '^$'
```

**`make build` requires network access.** `build` depends on `generate` → `docs` →
`scripts/fetch-docs-cfdocs.sh`, which git-clones the cfdocs repo into the gitignored
`docs/data/`. The *generated* Go file (`internal/docs/generated_docs.go`) is committed, so plain
`go build ./cmd/cfmleditor-lsp`, `go test ./...`, and `golangci-lint run ./...` all work offline
— use those when the fetch step can't run.

**The docs pipeline has two sources and both must be staged.** `internal/docs/generated_docs.go`
is generated from `docs/data/*.json`, which is *assembled* from per-source staging directories:

```
scripts/fetch-docs-cfdocs.sh → docs/src/cfdocs   (cache marker docs/.sha-cfdocs)
scripts/fetch-docs-lucee.sh  → docs/src/lucee    (cache marker docs/.etag)
scripts/assemble-docs.sh     → docs/data         (cfdocs first, lucee wins on collision)
```

`make docs` runs all three in order; `make docs-cfdocs` / `make docs-lucee` refresh one source
and re-assemble. Each fetch only ever touches its own staging directory and only replaces it
after a successful download, so refreshing one source can't destroy the other and a transient
outage can't destroy the cache.

**If a source can't be fetched, the generated file loses that source's entries.** A blocked
`docs.lucee.org` (some sandboxes and proxies deny it) means no `docs/src/lucee`, and
regeneration then drops every Lucee-only entry (`cfdistributedlock`, `cfstatic`,
`cfauthenticate`, …) — a pure-deletion diff of several hundred lines in
`internal/docs/generated_docs.go`. `assemble-docs.sh` warns loudly when a source is missing, and
`make docs` warns again when a fetch fails, but the build still proceeds. **Never commit that
deletion** — `git checkout -- internal/docs/generated_docs.go`. Only commit a change to that
file when `make docs` reported both sources staged.

Go toolchain is pinned at **1.26.6** (`go.mod`). CGO is required (tree-sitter grammar).

### CLI subcommands

The binary is an LSP server by default; `os.Args[1]` selects a subcommand
(`cmd/cfmleditor-lsp/main.go`).

| Command | Usage |
|---|---|
| *(none)* | Run the LSP server over stdio (JSON-RPC 2.0, Content-Length framing) |
| `parse` | `parse <file-or-dir> [...]` — parse and report per-file timing/counts |
| `scan` | `scan <file-or-dir> [...]` — report parse errors |
| `format` | `format [-w] <file> [...]` — format to stdout, or in place with `-w` |
| `unresolved` | `unresolved [--json] [--verbose] [--global-defs] <dir> [...]` — batch scan for unresolvable component/method calls |
| `refs` | `refs [--mermaid] <component-or-function> <dir> [...]` — find references |
| `deps` | `deps [--mermaid] <dir-or-file> [...]` — component dependency graph |
| `explain` | `explain [--root <dir>] <file> <line> [call-substring]` — trace how a call site resolved |
| `version`, `help` | |

`cmd/cfparse` is a separate debug binary for parser development (timing + `-cpuprofile`).

## Architecture

An LSP server for CFML/ColdFusion written in Go, backed by the `tree-sitter-cfml` grammar.

**Two runtime modes**, selected at startup by whether `daemon.FindConfig(cwd)` finds a
`.cfmleditor.json` (current dir or one level up):

- **Daemon mode** — the first LSP client becomes the daemon: it listens on a Unix socket (path
  derived from `workspaceName`) *and* serves that first client over stdio, sharing one
  `index.Index`. Later clients `daemon.Proxy()` into the socket. A `ConnTracker` shuts the
  daemon down when the last client disconnects.
- **Standalone mode** — no config file; a single self-contained session with its own index.

Note the repo root has its own `.cfmleditor.json` (`workspaceName: testdata`), so running the
binary from the repo root enters daemon mode against `testdata/`.

**Core data pipeline:**

```
Editor document change
  → server (internal/server) receives textDocument/didChange
  → parser (internal/parser) produces ParseResult
      ├─ ClassifyRegions() splits file into Script vs Tag regions
      ├─ scriptParser extracts function defs + component refs via line scanner
      └─ tagParser extracts function defs + component refs via tag search
  → index (internal/index) stores function definitions, keyed by lowercase name
  → resolver (internal/resolve) maps dot-paths → .cfc file paths
  → cached in server per URI (parseResults, compCache, resolveCache)
```

### Package map

| Package | Responsibility |
|---|---|
| `internal/parser` | Line-scanner/tag-search parsing → `ParseResult`; resolver matching (`ast.go`) |
| `internal/server` | LSP handler wiring, completion, definition, hover, symbols, signature help, code actions, document links, formatting, on-type formatting, workspace commands, bean scanning |
| `internal/index` | Concurrency-safe store of function defs, component refs, beans, ORM entities |
| `internal/resolve` | Dot-path → `.cfc` file resolution, `CanResolveCall`/`ExplainCall`, extends chain |
| `internal/path` | Case-insensitive path resolution, mappings, globs, `Application.cfc` mapping/bean/ORM extraction, binary + CFML file detection |
| `internal/config` | `.cfmleditor.json` schema (`config.JSON`), defaults, `JavaStubResolver` |
| `internal/daemon` | Unix socket serve/proxy, connection tracking, config discovery |
| `internal/formatter` | tree-sitter CST-walking formatter (elements, cfscript, cfquery/SQL) |
| `internal/language` | tree-sitter language handles (`CFML`, `CFScript`, `CFQuery`) + injection queries |
| `internal/docs` | Built-in CFML function/tag signatures and return types (**generated — do not hand-edit**) |
| `internal/cflint` | Downloads/runs the CFLint binary, maps JSON output to LSP diagnostics |
| `internal/cache` | Per-file, per-scope completion item cache with content hashing |
| `internal/refs` | Shared reference-finding + `Trace` (multi-hop wrapper following) for the `refs` CLI and `cfmleditor.findRefs` |
| `internal/deps` | Transitive dependency graph builder for the `deps` CLI and `cfmleditor.exportDeps` |
| `internal/graph` | Graph type + Mermaid renderer |
| `internal/vfs` | `FS` interface + stdio transport, abstracted for native vs WASM builds |
| `internal/log` | zap wrapper; `debug: true` in config switches to `zap.NewDevelopment` |

### Parser design (`internal/parser`)

The parser is intentionally **not** a full tree-sitter traversal — it uses a fast line-by-line
`Scanner` for script regions and string/regex searches for tag regions. tree-sitter is used by
the *formatter*, not the parser.

- Entry points: `Parse(fileURI, content, resolvers...)` and `ParseWithOptions(fileURI, content,
  ParseOptions{...})`. `ParseOptions` gates the expensive work: `ExtractCalls`, `ExtractLinks`,
  `ScanAllScopes`, `FindCalls`, `Shallow`, plus lookup hooks (`FuncLookup`, `BeanLookup`,
  `BuiltinReturnLookup`).
- **Function bodies are parsed lazily** — `FuncVars`/`FuncRefs`/`FuncCalls`/`FuncLinks` parse
  and memoize per function, keyed `"start:end"`.
- **Incremental edits** (`incremental.go`): `ParseResult.ApplyEdit` classifies an edit as
  `EditInFunc` (shift line numbers + invalidate that one function's caches), `EditGlobal`
  (`reparseShallow()` of signatures), or `EditFull`. Parse entry points recover from panics and
  fall back to a shallow re-parse rather than crashing the daemon.
- `edit_parser.go` holds cursor-context helpers (`FindCallContext`) used by signature help and
  completion.

### Formatter (`internal/formatter`)

Walks the tree-sitter CST. All rules (tag/attribute case, indentation, quotes, comma position,
SQL keyword casing, line width, attribute break threshold) come from `formatting` config. The
`whitespaceOnly` guard rejects any result that changes non-whitespace characters. `queryFormat`
is off by default — `<cfquery>` bodies are emitted verbatim unless opted in.

## Key structural notes

- `internal/parser/result.go` — `ParseResult`, `ParseOptions`, `Parse()`/`ParseWithOptions()`,
  lazy per-function caches, `replaceExpressions`, `resolvePendingCalls`, property accessors
- `internal/parser/cfparser.go` — `ClassifyRegions()` plus the standalone
  `ParseFunctionDefs`/`ParseComponentRefs` helpers
- `internal/parser/script_parser.go` — `scriptParser` and the two parse loops (top-level
  `parse()` and `handleBodyToken`); see the scope-dispatch note below
- `internal/parser/tag_parser.go` — `tagParser` for `<cffunction>`, `<cfcomponent>`, etc.
- `internal/parser/scanner.go` — byte-level tokenizer (`isIdentStart` treats `_` and `$` as
  identifier starts)
- `internal/parser/ast.go` — resolver matching: `BuildResolverSet`, `ResolveFromCall(Full)`,
  `matchResolverWithCache`, `splitPrefix`/`findPrefixPos`, `substitutePlaceholder`
- `internal/server/server.go` — `Server` struct, capabilities, per-URI caches
- `internal/server/handler.go` — LSP method dispatch and `workspace/executeCommand`
- `internal/resolve/resolve.go` — `ComponentPath`, `CanResolveCall`, `ExplainCall`, extends walking
- `internal/config/config.go` — `.cfmleditor.json` schema and defaults
- `internal/docs/` — generated; regenerate via `make generate`, never hand-edit

**Scope-prefixed assignments:** each handled scope (`local.`, `variables.`, `this.`,
`arguments.`, `request.`, `session.`, `application.`) needs its own `case` in *both* dispatch
switches (`scriptParser.parse()` and `handleBodyToken`) routing to
`parseScopedVar`/`parseBodyScopedVar` — that handler is the only one that correctly
distinguishes `scope.name = rhs` (assignment) from `scope.name.method()` (bare call) for a
two-token-prefixed LHS. Any scope keyword *not* listed falls through to `checkAssignRef`'s
default path, which only recognizes a bare `x = ...` (single identifier directly followed by
`=`); for a scope-prefixed LHS the next token is `.` not `=`, so the statement is silently
misread as a bare-call check and any component type the RHS establishes is dropped.
`url.`/`form.`/`cookie.`/`cgi.`/`client.`/`server.` deliberately aren't listed (those scopes
hold primitive request/config data, not component instances) — add them the same way if a
project assigns components through one.

## LSP surface

Declared in `Server.capabilities()` (`internal/server/server.go`):

- Incremental text sync, completion (trigger chars `<`, `/`, `.`, `>`), definition, hover,
  signature help (`(`, `,`), document + workspace symbols, document links (with resolve), code
  actions, document formatting, on-type formatting (`>`), workspace folders.
- Diagnostics come from CFLint when `"linting": {"enabled": true}` — `internal/cflint` downloads
  the binary from `cfmleditor/CFLint` releases on first use.
- `workspace/executeCommand`: `cfmleditor.reindex`, `.format`, `.showComponentPath`,
  `.restartDaemon`, `.showResolvers`, `.showFileIndex`, `.showConnections`,
  `.openActiveApplicationFile`, `.goToMatchingTag`, `.copyPackage`, `.findRefs`, `.exportDeps`,
  `.scanWorkspace`.

## Configuration (`.cfmleditor.json`)

The authoritative schema is `config.JSON` in `internal/config/config.go`; README.md documents
the user-facing view and all `formatting` defaults.

| Field | Purpose |
|---|---|
| `workspaceName` | Required for daemon mode; derives the socket path |
| `workspacePaths`, `workspaceIndexGlobs` | Which roots / `.cfc` files to index |
| `mappings` | Virtual dot-path root → directory |
| `expressionMappings` | Runtime `#...#` expression → static substring (see below) |
| `componentResolvers` | Call expression → component dot-path (see below) |
| `propertyResolvers` | `<cfproperty>` attribute → component dot-path (`match`/`resolve`/`attribute`) |
| `servicePropertyResolvers` | `@serviceproperty <var> <kind>\|<name>` doc-comment kind → `${name}` dot-path template, for generically-typed dependencies |
| `beanPaths` | namespace → directory; `.cfc`s registered as `name@namespace`, plus a bare `name` when unique across all namespaces |
| `javaStubsPath` | Auto-synthesizes a `createObject("java", "X")` → `<javaStubsPath>.X` resolver |
| `formatting` | Formatter options |
| `linting.enabled` | Enable CFLint diagnostics |
| `completions` | `tagSnippets`, `functionSnippets`, `globalFunctionResolution` |
| `debug` | Verbose zap development logging to stderr |

## Debugging why a call site resolved (or didn't)

`cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]` prints, for every call
site on that line, the exact sequence of decisions `CanResolveCall`
(`internal/resolve/resolve.go`) walked through: which mechanism set the receiver's component
(function-scoped ref, file-level ref, `Application.cfc` ref, `<cfargument>` type, extends chain,
a `componentResolver` match on the variable name, a `componentResolver` match on the full line
text), which `FuncLookup`/componentResolver fallback fired for each hop of a chained call, and
why the final method-exists check passed or failed. `--root <dir>` picks which
`.cfmleditor.json` to load and which files to index — same semantics as `unresolved`'s directory
argument — and defaults to the target file's own directory if omitted, which matters because a
file's *own* nearest config can differ from the config a batch `unresolved` scan used.

**Reach for this before manually tracing through
script_parser.go/tag_parser.go/result.go/resolve.go.** A component path that shows up in an
unresolved-call error but doesn't match anything literal in `.cfmleditor.json` or on disk is
almost always a `componentResolver` firing on a substring you didn't expect (see "Known resolver
false-positive" below) — `explain` shows the exact resolver and match in one call instead of a
multi-file manual trace. Example: a `VARIABLES._content.createTemplate(...)` call reported "not
found in tassweb.packages.tass.directcontent" — a path absent from config entirely — because two
lines earlier, `VARIABLES._content = VARIABLES._document.getDirectContent()` had its RHS matched
by the generic catch-all resolver `{"match": "get$1()", "resolve":
"tassweb.packages.tass.${1:lower}", "prefix": "get"}` (intended for `getPageTools()`-style
factory methods): `indexFold` found the `"get"` prefix inside `"getDirectContent"`, matched the
whole call, and produced `tassweb.packages.tass.directcontent` — even though it's a genuine
iText/PdfWriter passthrough getter. `explain` surfaces this as a single `resolved "..." to "..."
via componentResolver matching the variable name` step.

## Component resolvers

Component resolvers (`componentResolvers`) teach the LSP how to map a call-site expression to a
component dot-path. They are tried in order by `ResolveFromCall` / `ResolveFromCallFull` in
`internal/parser/ast.go`.

**Component-type resolution order** (`CanResolveCall`, `internal/resolve/resolve.go`): for a
qualified call `x.method()`, the receiver's component is looked up in this order, stopping at
the first hit — (1) `call.Component`, if already set at parse time (e.g. a chained
`new`/`createObject`, or a bare-call site where the tag/script parser resolved the receiver
inline via `lookupComponentRef`); (2) a function-scoped `ComponentRef` for `x`; (3) a file-level
(global/`VARIABLES.`/`this.`) `ComponentRef`; (4) a `ComponentRef` on
`Application.cfc`/`Application.cfm`; (5) for `ARGUMENTS.x`, the `<cfargument type>` if it's a
dotted path; (6) walking the `extends` chain's own `ComponentRef`s; (7) a `componentResolver`
matched against the variable name text; (8) a `componentResolver` matched against the full line
text (handles chains like `x.method().prop.func()`). If the call is itself chained
(`call.Chain`), each hop repeats a scaled-down version of this: the hop function's declared
`ReturnComponent`/dotted `ReturnType`, falling back to a `componentResolver` matched against
`hop()`. Because steps (7)/(8) and the per-hop fallback all go through the same substring-prefix
matching described below, a broad catch-all resolver (e.g. `get$1()`) can win at *any* of these
steps, not just the ones that look like factory-method calls.

Two component values short-circuit the method-exists check unconditionally: `$any` (dynamic —
see "Expression mappings") and `$builtin.<fn>` (a built-in CFML function's return type, from
`internal/docs/builtin_returns.go`).

**How matching works** (`internal/parser/ast.go: matchResolverWithCache`):
- Each resolver has a `prefix` (fast-rejection substring) and a `match` pattern.
- `prefix` is searched for anywhere in the expression unless the resolver sets `"anchored":
  true`, which requires it at position 0 (see "Resolver false-positive" below).
- Resolvers are tried in **array order** (`ResolveFromCallFull` iterates `resolvers` and returns
  on the first successful match) — so when two resolvers could both match the same expression
  (e.g. a specific `getDirectContent()` entry and a generic `get$1()` catch-all), whichever is
  listed **earlier** wins, regardless of specificity. To make a specific case win over an
  existing broad catch-all, add it *before* the catch-all in the array, not after.
- `ResolveFromCall` finds the prefix inside the expression, takes the substring from that
  position, and tries to match the full pattern against it.
- If `match` contains no `\` escapes and no `$N` placeholders → **simple exact match** (or
  `match()` suffix).
- If `match` contains `$1` but no `\` → **simple prefix/suffix match**: the text before `$1` and
  after `$1` are checked as plain strings; the captured value replaces `$1` in `resolve`.
- If `match` contains `\` → **regex match**.

**Pipe-delimited `prefix` alternatives** (`splitPrefix`, `findPrefixPos`):

`prefix` may list alternatives separated by `|` (e.g. `"createModel|buildModel"`), letting one
resolver's `match`/`resolve` pair cover call-site shapes that don't share a common leading
substring. `findPrefixPos` tries each alternative via `indexFold` and returns the position of
whichever is found; `BuildResolverSet`'s byte-bucket index registers the resolver under every
alternative's first byte so fast rejection still finds it. Which alternative matched has no
effect on the subsequent `match`/`resolve` step — only the matched *position* is used to slice
the expression — so this is purely a fast-lookup mechanism, not a change to match semantics.

Gotcha when merging plain bare-word entries this way: `match`'s regex-vs-simple decision
(`isRegexPattern`) is triggered *only* by a literal backslash in `match` — it has nothing to do
with `prefix` or with `|` in `match`. A merged pattern like `match: "kernel|_kernel"` has no
backslash, so it is compared as one literal string (never equal to either bare word), not as
alternation — silently matching nothing. Give merged bare-word alternatives a real anchor that
needs escaping, e.g. `"^kernel(?:\\(\\))?$|^_kernel(?:\\(\\))?$"`.

**Case-folded placeholders in `resolve`** (`substitutePlaceholder`):

Both simple prefix/suffix and regex matches substitute captures through the same helper, which
supports `${N:lower}` and `${N:upper}` in addition to plain `$N`. Use `${1:lower}` to fold a
captured value: `{"match": "get$1()", "resolve": "packages.tass.${1:lower}", "prefix": "get"}`
turns `getPageTools()`, `getLockBroker()`, etc. into one resolver instead of one entry per name.
This only helps when the target path is a mechanical case-fold of the captured text — if the
target name isn't derived from it at all (e.g. an `itextObj.Foo` family mapping to unrelated
Java class names), each name still needs its own entry. Since path resolution is itself fully
case-insensitive, `${N:lower}`/`${N:upper}` are for config explicitness, not correctness.

**Case-insensitive path resolution** (`internal/path/path.go`): `match`/`prefix` matching
(`indexFold`, `EqualFold`, `(?i)`-compiled regexes) has always been case-insensitive. Turning a
resolved dot-path into an actual `.cfc` file is also fully case-insensitive at every path
segment, not just the filename: `ResolvePath` walks the path one directory at a time via
`resolveSegments`, matching each segment against a real directory listing (exact case first,
then `EqualFold`), and `mappings` keys are matched case-insensitively too (`lookupFold`). This
matters because `os.Stat` alone can't be trusted: on a case-insensitive filesystem (APFS, NTFS
defaults) `Stat` succeeds for any case variant, silently returning the *requested* case rather
than the real on-disk case — which breaks on ext4 and most Linux filesystems. Only a directory
listing reveals the true on-disk name on every platform.

**Resolver false-positive: prefix substring matching, and the `anchored` fix**

By default `ResolveFromCall` finds the resolver's `prefix` *anywhere* in the expression via
`indexFold`, not just at position 0. A resolver with prefix `"document"` will also fire when the
variable name merely contains "document" (e.g. `domobject_document`). The sub passed to the
pattern matcher starts at the prefix position, so it exactly matches the short pattern and
produces a wrong component — confidently, rather than declining.

`{"anchored": true}` on a resolver requires the prefix at position 0
(`config.Resolver.Anchored`/`parser.Resolver.Anchored`, enforced in `findPrefixPos`), so the
resolver only claims expressions that genuinely start with it. It is off by default because
unanchored matching is what makes a *call* resolver qualifier-insensitive: `getService("$1")`
is meant to fire on `VARIABLES._parent.getService("x")` too. Reach for `anchored` when the
resolver targets a variable name, or when a broad catch-all is producing wrong answers. The
older workarounds still apply where anchoring doesn't fit: rename the variable in the CFML
source, or add a more-specific resolver for the false-positive variable *earlier* in the array.

The same class of issue can appear *within* a single pipe-delimited `prefix`: unanchored,
`findPrefixPos` tries alternatives in the order written and returns the position of whichever is
found first in that order — not the earliest position in the expression, and not the alternative
that would let `match` succeed. If one alternative is a substring of another (e.g. `prefix:
"File|getFile"` against `getFile()`), the shorter one found first fixes the slice position (`sub
= "File()"`) and the longer never gets a chance. Order alternatives so a shorter potential
substring comes *after* the longer one — or set `anchored`, which makes the order irrelevant
since every alternative that matches matches at 0.

**Generic catch-all resolvers are the highest-risk case of this**, because they're deliberately
broad. `{"match": "get$1()", "resolve": "tassweb.packages.tass.${1:lower}", "prefix": "get"}` is
meant to cover a family of factory methods without one entry per name — but unanchored, `prefix:
"get"` is found inside *any* identifier containing "get", including ordinary getters unrelated
to the intended family, and including the `getDirectContent()` at the tail of
`VARIABLES._document.getDirectContent()`. A catch-all this broad will win over a
correct-but-absent answer (e.g. a generic `returntype="any"` that should fall through to `$any`)
essentially every time a same-named `getXxx()` exists anywhere in the workspace. Anchoring it
is usually the right call: the bare factory calls it was written for (`getPageTools()`) start at
position 0, and the chained getters it was never meant to claim do not. Otherwise the options
are narrowing the `match` regex to an explicit alternation of the real factory names, or
accepting the false positive and overriding it with a more specific resolver listed earlier.

`explain` names the resolver that fired (`match "...", prefix "...")`, so a wrong component can
be traced to the exact `componentResolvers` entry instead of just "a componentResolver".

## Expression mappings

`expressionMappings` is a flat `map[string]string` of runtime expression → static value
substitutions (e.g. `"#VARIABLES._core#": "packages.tass.core."`), applied to component-path
strings before resolution. Unlike `componentResolvers`, there is no `match`/regex support — each
key is matched with a plain `strings.Contains` and replaced with `strings.ReplaceAll`.

A key may list alternatives separated by `|` (e.g. `"#ROOT#|#LEGACY_ROOT#": "app."`), so several
runtime expressions collapsing to the same static value don't need separate entries. Each
alternative is checked and replaced independently — plain substring alternation, unrelated to
the regex-triggering `\` in `componentResolvers.match`. Implemented in
`internal/resolve/resolve.go: ComponentPath` and `internal/parser/result.go: replaceExpressions`.

**Unmapped `#...#` expressions become `$any`, not literal garbage.** Any component-path string
captured from CFML source — `CreateObject("component", "...")`, `<cfinvoke component="...">`,
`new "..."()` — may contain a runtime `#...#` interpolation no `expressionMappings` entry covers
(e.g. `CreateObject("component", "tools.templates.#ARGUMENTS.template#.generator")`).
`replaceExpressions` runs its substitution pass unconditionally (even with zero
`expressionMappings` configured), and if the result still contains `#`, returns `"$any"` — so
`CanResolveCall` accepts calls through it silently, the same as any other "genuinely dynamic"
case, rather than surfacing the literal text as a nonsensical "not found in" component name.
This applies to `ComponentRef`s (`pr.ComponentRefs`/`funcRefsMap`) *and* to `CallSite.Component`
(`pr.Calls`/`funcCallsMap`) — the latter matters because a bare unassigned call's
`CallSite.Component` can be baked in at parse time (`tag_parser.go`'s `lookupComponentRef`),
before this pass would otherwise run.

**Bracket-indexed chain segments** (`REQUEST['a' & b & 'c'].method()`, `arr[i].method()`): the
script parser's chain-walking (`checkVarRHS`, `parseBodyVarDecl`, `parseBodyScopedVar`,
`checkAssignRef`, `checkBareCall`, `recordBareCallAndChain` in `script_parser.go`) skips a
`[...]` group via `skipBracketIndex()` (mirrors `skipParens()`) and, critically, **poisons** the
chain text with a literal `"[]"` marker (`REQUEST['a'&b&'c'].method()` → `"REQUEST[].method"`)
instead of silently dropping the bracket and treating the bare identifier before it as the
receiver. The marker can't collide with any real identifier or resolver `match`, so it reliably
falls through to an honest `variable 'REQUEST[]' has no component ref` — the alternative would
let `REQUEST[dynamicKey].method()` resolve as if it were plain `REQUEST.method()`, which is
wrong *and* confident. To make these fully silent, add a `noFollow` resolver matching the
marker, e.g. `{"match": "^REQUEST\\[\\]$", "resolve": "nocheck", "prefix": "REQUEST[]",
"noFollow": true}` — the parser deliberately does not do this automatically, since suppressing
vs. surfacing "genuinely dynamic" is a project-level judgment call.

**Known parser limitation: CFML comments containing CFScript**

The tag parser does not fully skip `<!--- ... --->` comment blocks that contain embedded
CFScript or `<cfset>` tags. Variables declared in live code but only used inside a comment block
still generate lint errors for the commented-out lines. These are false positives; the fix
requires improving comment boundary detection in the tag parser.

## `noFollow` flag

When a resolver has `"noFollow": true`, `CanResolveCall` accepts the call immediately without
verifying the method exists in the resolved component
(`config.Resolver.NoFollow`/`parser.Resolver.NoFollow`). Use it for dynamic factory methods
where the resolved component is approximate, Java objects whose stub coverage is incomplete, or
any pattern where "this variable came from X" is enough and method checking is noise.
`CanResolveCall` checks it at three points: the primary variable resolver, the full-line-text
fallback resolver, and the altComp fallback resolver.

## Triaging "method not found" / "no component ref" lint errors

| Error form | Meaning | Fix |
|---|---|---|
| `variable 'x' has no component ref` | Parser never established what component `x` is | Add a `componentResolver` covering the RHS of the assignment or the variable name |
| `method 'f' not found in pkg.path` | Component is known but the method is missing | Add the method to the stub CFC at that path, or `"noFollow": true` on the resolver |
| `method 'f' not found in persist` | `persist` resolves correctly but the method is absent | Genuinely missing from the real CFC — implement it |
| Dynamic keys: `x[y].f`, `arr[i].f` | Type can't be tracked through runtime keys | Not fixable with resolvers; suppress with `noFollow` on the resolver that produces `x` |
| `ARGUMENTS.x` with `type="any"` | Argument has no type annotation | Add `type="pkg.path"` to `<cfargument>`, add an `ARGUMENTS.x` exact-match resolver, or document it with a `@serviceproperty` comment if `servicePropertyResolvers` is configured |
| One component, many unrelated-looking missing methods (e.g. 50+ hits all "not found in studadmin") | `CanResolveCall`'s file-level fallback picks the `ComponentRef` with the highest line number *at or before* the call site (`internal/resolve/resolve.go`), falling back to file order only for a genuine forward reference — so a scratch variable reassigned per `<cfcase>`/`<cfif>` branch is read against the assignment actually in scope. A large single-component cluster is therefore real, unless the reassignments sit inside a construct the parser doesn't line-order the way the runtime does; confirm with `explain`, which prints the line of the ref it chose. |
| A resolver with `"resolve": "nocheck", "noFollow": true` doesn't suppress the check | The resolver's `match` includes `(...)` (a call expression) rather than a bare variable name | `noFollow` only survives when the resolver re-runs live inside `CanResolveCall` (bare-word `ResolveFromCallFull(variable, ...)` lookups). A call-expression match fires once at parse time and is baked into `ComponentRef.Component`, which has no `NoFollow` field. Use `"resolve": "$any"` instead (checked unconditionally) for any resolver whose `match` contains parens. |

## Java stubs

Stubs are plain CFCs containing empty function stubs, mirroring the Java package path, so the
LSP can verify method calls without running Java. Single-line format, matching existing files:

```cfml
// Java stub for com.example.ClassName
component { function init(...) {} function methodName(required type argName) {} }
```

Set `"javaStubsPath": "<dot.path.to.stubs>"` to auto-resolve any `createObject("java",
"some.Class.Name")` to `<javaStubsPath>.some.Class.Name` without hand-writing the equivalent
regex resolver — it's synthesized and appended alongside your own `componentResolvers`
(`config.JavaStubResolver`, wired in `config.Resolve`, `daemon.Config.ComponentResolvers`, and
`cmd/cfmleditor-lsp/cliutil.go: loadResolversFromConfig`). It covers the `createObject("java",
...)` call site and Lucee's `new java:some.Class.Name()` — the latter because
`readNewComponent` recognises the `java:` type prefix and re-spells it as the equivalent
`createObject` expression before resolving, rather than a second resolver existing for it.
Chained factory calls (e.g. `someJavaObj.getInstance()` returning another instance) still need
their own resolver entry or a stub method modelling the return.

`new cfml:a.b.C()` is the sibling case and needs no configuration — the prefix is dropped and
`a.b.C` resolves as an ordinary CFC path. Both prefixes are handled in one place; the three
`new`-reading paths (`parseNewRef`, `parseStandaloneNew`, `checkReturnComponent`) all route
through `readNewComponent`.

**Two resolver shapes for struct-member Java handles.** Code that stores handles as struct keys
(`VARIABLES.itextObj.Foo = getJavaClass("Foo","itext")`) can't be tracked by the parser, so:

1. **Assignment RHS** — `var local = itextObj.Foo.init(...)`. Use `match: "itextObj.Foo.$1"`.
   The `$1` matches whatever follows the dot; since `resolve` has no `$1` it is discarded. The
   parser's `resolveCall(rhs)` picks this up and creates a `ComponentRef` for the local variable.
2. **Direct call** — `itextObj.Foo.bar(...)` where the variable *is* `itextObj.Foo`. Use
   `match: "itextObj.Foo"` (exact, no `$1`); `CanResolveCall` passes the raw variable name to
   `ResolveFromCall`, which exact-matches it.

Some handles need both shapes; others only one, depending on how the code uses them.

## Testing and lint conventions

- Test fixtures live in `testdata/` (`beans/`, `chain/`, `deps/`, `refs/`, `models/`,
  `services/`, `includes/`, plus `Application.cfc` and assorted `.cfm`/`.cfc` files). Server,
  deps, and resolve tests locate them relative to the source file via `runtime.Caller(0)`.
- Formatter golden-output tests: `make visualtest` (`TestFormatOutput`); comparison fixtures
  `testdata/comparison.cfm` / `comparison.html`.
- The formatter's whitespace-only claim is checked against external corpora, not fixtures:
  `make corpus CORPUS=<dir>` (`internal/formatter/corpus_test.go`). It skips without a corpus.
  Reach for it after any formatter change — a rule that reads as obviously safe has repeatedly
  turned out to delete code on some construct no fixture contains. `FORMATTER-ISSUES.md` records
  the current numbers and the six projects they were measured against.
- `.golangci.yml` (v2 config) enables `wsl_v5`, `nlreturn`, `revive`, `gocritic`, `gosec`,
  `errorlint`, `exhaustive`, `prealloc`, and others. **`wsl_v5` + `nlreturn` demand a blank line
  before `return` and around block statements** — this is why existing code looks the way it
  does; match it or `make lint` fails. Test files are exempted from `prealloc`, `unparam`,
  `gosec`, and `staticcheck`.
- `internal/docs/` content is generated — regenerate rather than hand-editing, but see the
  lossy-regeneration warning under Commands before committing any change to it.
- `.github/workflows/ci.yml` runs on every pull request: `build-test` (build, vet, gofmt, `go
  test -short`), `race`, `lint` (golangci-lint), `vuln` (`make vuln`), and an informational
  `perf` job that never gates a merge. None of them run `make docs`, since
  `internal/docs/generated_docs.go` is committed.

## Release

`make release <version>` (`scripts/release.go`) validates, builds, tests, lints, scans for
vulnerabilities, updates `CHANGELOG.md` and `VERSION`, commits, tags, and pushes. Use `make
release-dry <version>` first. Pushing a `v*` tag triggers `.github/workflows/release.yml`, which
cross-compiles darwin/linux/windows (amd64 + arm64) with zig as the CGO cross-compiler and
embeds the tag as `main.version`.

**`make vuln` gates the release.** Every check runs *after* `CHANGELOG.md`/`VERSION` are
rewritten but before anything is committed, so a failure leaves those two files modified and
uncommitted — same as an existing test or lint failure. A newly published advisory can therefore
block a release without any code changing; fix it by bumping the Go patch version in `go.mod`
(stdlib findings) or the offending dependency, not by skipping the step.

The `vuln` job in `.github/workflows/ci.yml` runs the same `make vuln` target, so CI, the release
gate, and a local run cannot drift apart.

## Skills

- `/add-parser-test` — patterns and pitfalls for adding tests to `internal/parser/cfparser_test.go`
- `/run-cfmleditor-lsp` — build, smoke-test, and drive the binary (CLI subcommands + LSP stdio)
- `/parser-internals` — scanner tokenisation, the two parse loops, call-site extraction, and how
  the `unresolved` command works

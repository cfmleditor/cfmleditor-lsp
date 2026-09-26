# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # generate docs + build binary to target/release/cfmleditor-lsp
make test           # go test ./...
make lint           # golangci-lint run ./... (pinned scanner, built from source)
make lint-fix       # golangci-lint run --fix ./...
make vuln           # govulncheck ./... (pinned scanner, GOWORK=off)
make fmt            # gofmt -w . && golangci-lint run --fix ./...
make install        # build and copy to `go env GOPATH`/bin
make link           # build + symlink onto PATH for local editor use (LINK_DIR=<dir> to override)
make unlink         # remove that symlink
make link-status    # show the link, the build, and what PATH resolves cfmleditor-lsp to
make update-grammar # bump tree-sitter-cfml, regen docs + injections.scm, clear build cache
make update-d3      # rebuild the code-map viewer's D3 bundle from assets/vendor/entry.js
                    # (needs Node; the bundle is committed so `go build` does not)
make cfparse        # build + run the parser-benchmark CLI (cmd/cfparse)
make visualtest     # go test -v -run TestFormatOutput ./internal/formatter/
make gapcheck [CORPUS=<dir>[:<dir>...]]
                    # diff what internal/parser extracts against what the tree-sitter
                    # grammar sees in the same file. Without CORPUS it holds the repo's
                    # fixtures to a recorded list of differences; with one it reports.
                    # 6s over 5,629 files. See PARSER-GAPS.md
make corpus CORPUS=<dir>[:<dir>...] [REPORT=<file>] [BASELINE=<file>] [OPTS=k=v,...]
                    # format a real-world CFML corpus and report what the formatter did to
                    # each file (clean / grammar-refused / guard-rejected / not idempotent /
                    # malformed); skipped without CORPUS, so it never runs in CI.
                    # BASELINE=<an earlier REPORT> fails the run if any file changed verdict —
                    # the totals cannot show that, since one file breaking and another being
                    # fixed leaves every column identical. OPTS sets formatter.Options fields
                    # by name, for sweeping a new setting through its modes without editing
                    # the test. See FORMATTER-ISSUES.md
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
`go build ./cmd/cfmleditor-lsp`, `go test ./...`, and `make lint` all work offline
— use those when the fetch step can't run. (`make lint`'s first run needs the network once, to
fetch and build the pinned linter; after that it is served from the build cache.)

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

Go toolchain is pinned at **1.27.1** (`go.mod`). CGO is required (tree-sitter grammar).

### CLI subcommands

The binary is an LSP server by default; `os.Args[1]` selects a subcommand
(`cmd/cfmleditor-lsp/main.go`).

| Command | Usage |
|---|---|
| *(none)* | Run the LSP server over stdio (JSON-RPC 2.0, Content-Length framing) |
| `parse` | `parse <file-or-dir> [...]` — parse and report per-file timing/counts |
| `scan` | `scan <file-or-dir> [...]` — report parse errors |
| `format` | `format [-w] [--allow-non-whitespace] [--root <dir>] <file> [...]` — format to stdout, or in place with `-w`. Reads `formatting` from each file's governing `.cfmleditor.json`, or from `--root`'s |
| `unresolved` | `unresolved [--json] [--verbose] [--global-defs] <dir> [...]` — batch scan for unresolvable component/method calls |
| `refs` | `refs [--mermaid] <component-or-function> <dir> [...]` — find references |
| `deps` | `deps [--mermaid] <dir-or-file> [...]` — transitive dependency graph, built through `deps.Build` so the CLI and `cfmleditor.exportDeps` answer alike |
| `graph` | `graph [--level function\|call\|file\|package] [--format text\|json\|jsonl\|dot\|mermaid\|html] [--db <file>] [--out <f>] [--live\|--detached\|--from <id>] [--under <p>] <dir> [...]` — whole-project code map |
| `mcp` | `mcp --db <file> [--root <dir>] [--no-explain]` — serve that map over MCP on stdio, read-only |
| `explain` | `explain [--root <dir>] <file> <line> [call-substring]` — trace how a call site resolved |
| `version`, `help` | |

`cmd/cfparse` is a separate debug binary for parser development (timing + `-cpuprofile`).

## Architecture

An LSP server for CFML/ColdFusion written in Go, backed by the `tree-sitter-cfml` grammar.

**Two runtime modes**, selected at startup by whether `daemon.FindConfig(cwd)` finds a
`.cfmleditor.json` (walking from the current directory up to the filesystem root):

- **Daemon mode** — the first LSP client becomes the daemon: it listens on a Unix socket (path
  derived from `workspaceName`) *and* serves that first client over stdio, sharing one
  `index.Index`. Later clients `daemon.Proxy()` into the socket. A `ConnTracker` shuts the
  daemon down when the last client disconnects.
- **Standalone mode** — no config file; a single self-contained session with its own index.
  `FindConfig` returns nil when its walk finds nothing, which is what selects this mode. It used
  to return a `Config` with an empty `Path` and the working directory's base name as its `Name`,
  making the standalone branch unreachable and keying the socket — a hash of `Name` — on that
  base name, so unrelated projects in folders with the same name shared one daemon and one
  index. Every caller already tested for nil.

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
| `internal/server` | LSP handler wiring, completion, definition, hover, symbols, signature help, code actions, document links, formatting, on-type formatting, watched-file reindexing, workspace commands, bean scanning |
| `internal/index` | Concurrency-safe store of function defs, component refs, beans, ORM entities. `HasFile` answers "indexed at all" — not the same as `FunctionsForFile` returning nothing, since a property-only bean indexes to an empty but present entry. Two views of every entry — the name buckets (`funcs`/`comprefs`) and the per-file lists (`fileFuncs`/`fileRefs`) — hold the same pointers, and **every writer must fill or clear both**; `removeFileEntries` reaches the buckets *through* the per-file lists, so a writer that updates one view alone leaves entries no removal can find. **Accessors return a copy** (`snapshot`), which is what lets writers compact and rewrite buckets in place rather than rebuild a slice that, for a name every component declares, holds one entry per file — pinned by `TestAccessorsReturnStorageWritersDoNotTouch`. On a per-keystroke path reach for `LookupPreferred`/`CountFunctions`, not `Lookup`, which pays that copy. **Bucket order is not an answer**: entries land in the order a parallel workspace scan finished, so it differs between restarts — where several files declare a name and none is the requesting file, `LookupPreferred` picks the nearest by `cfpath.URIDistance` and the lowest URI among equals, and `definition.go` orders its multi-location list the same way |
| `internal/resolve` | Dot-path → `.cfc` file resolution, `CanResolveCall`/`ExplainCall`, extends chain, cfinclude scope |
| `internal/path` | Case-insensitive path resolution, mappings, globs, `Application.cfc` mapping/bean/ORM extraction, binary + CFML file detection |
| `internal/config` | `.cfmleditor.json` schema (`config.JSON`), defaults, `JavaStubResolver` |
| `internal/daemon` | Unix socket serve/proxy, connection tracking, config discovery |
| `internal/formatter` | tree-sitter CST-walking formatter (elements, cfscript, cfquery/SQL) |
| `internal/language` | tree-sitter language handles (`CFML`, `CFScript`, `CFQuery`) + injection queries |
| `internal/docs` | Built-in CFML function/tag signatures and return types (**generated — do not hand-edit**) |
| `internal/cflint` | Downloads/runs the CFLint binary, maps JSON output to LSP diagnostics |
| `internal/cache` | Per-file, per-scope completion item cache with content hashing |
| `internal/refs` | Shared reference-finding + `Trace` (multi-hop wrapper following) for the `refs` CLI, `cfmleditor.findRefs` and `textDocument/references` |
| `internal/route` | Convention-based framework routing: the `routes` config grammar, the source scanner, and resolution to a controller method or a view |
| `internal/codemap` | Whole-project map: every function, file, and the calls/instantiations/inheritance/includes between them. The **inverse** of `internal/deps` — see the note below |
| `internal/codemap/store` | SQLite persistence + the per-file parse cache (`!wasip1`; a stub declines on wasm) |
| `internal/codemap/mcp` | Read-only MCP server over the store |
| `internal/deps` | Transitive dependency graph builder, the single implementation behind both the `deps` CLI and `cfmleditor.exportDeps`. Two traversals: file-level, which walks `Index.RefsForFile`; and function-level, which needs an `Options.LoadCalls` hook, because the index stores definitions and refs but no call sites. Without that hook the function-level graph stops after one hop |
| `internal/tsoracle` | Differential check: what `internal/parser` extracted vs what the tree-sitter grammar saw in the same file. See the note under Verification discipline |
| `internal/textdiff` | Myers line diff, for range formatting: which lines the formatter changed and what each became |
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
- **The scanner's API is `NextSkipComments`/`PeekSkipComments`, `Save`/`Restore` and
  `LastBlockComment` — nothing else.** The raw `Next`, `Peek`, `Pos`, `Line` and `Rest` are
  exported and have no caller in or out of the package. That is what makes the one-token
  lookahead safe: a peek caches the token and the position past it, and the matching next takes
  it rather than scanning the same bytes again. A peeked token carries the assignment it made to
  `LastBlockComment`, because a peek's effect on that field has always outlived the peek and
  `parseFunction` reads and clears it between the two. Adding a scanner entry point that moves
  `pos` means clearing `peeked`, as `Restore` and the raw `Next` do.
- **Don't write `switch strings.ToLower(x)` on a parse path.** `strings.ToLower` returns its
  argument untouched when there is nothing to lower, so such a switch is free on all-lowercase
  source and allocates a throwaway string for every identifier with a capital in it — which is
  `getUser`, `userDAO`, every `ARGUMENTS` and `VARIABLES`. It was 22% of everything a parse
  allocated. Use `var buf foldScratch` and `switch string(buf.lowerFold(x))` (`fold.go`): the
  compiler elides the conversion in `switch string(b)` and in `m[string(b)]`, and the array stays
  on the stack. Spell the fold inside the switch expression — binding it to a name invites
  holding it across a nested fold. `TestFoldingAnIdentifierDoesNotAllocate` parses one document
  mixed-case and lowercased and fails if the capitals cost allocations.
- **A local `var buf [N]byte` stays on the stack; a struct holding a slice of its own array does
  not.** `chainBuilder` accumulates into an array addressed by length for that reason — the
  shorter `c.rest = c.arr[:0]` defeats escape analysis and moved all eleven of its call sites'
  builders to the heap, which cost more than the buffer growth it saved. Check with
  `go build -gcflags=-m` rather than assuming.
- **Recording a call is gated on `ExtractCalls`; consuming one is not.** The gate lives in
  `addCall` alone, and every dispatch that walks a call and its argument list runs in every
  mode. It used to sit on the dispatch itself, at nine sites, and the consequence was not that
  an argument list was skipped — nothing skips it — but that it was scanned as if it were
  statements. `svc.save(force = true)` then declared a variable called `force`, and
  go-to-definition on `force` anywhere in the file landed on that argument. The same shape is
  why `{force = true}` needs `skipLiteralGroup` and a script-syntax `<cfquery>`'s attributes
  need `parseScriptTagAttrs`: a group the scan walks *into* rather than *over* is read as code.
- **An argument list is the one place a call could hide.** `skipParenBody` discarded a `(...)`
  group a token at a time, so `writeOutput(svc.getName())` recorded only `writeOutput` — on a
  realistic function body 7 of the 12 calls present. A condition, a return, an assignment's
  RHS, a concatenation, a struct or array literal and a ternary all extract correctly. The
  cost was not completeness: a method called only from inside an argument list had no edge
  into it, so `internal/codemap` read it as unreachable and `unresolved` never checked it.
  There were **three** copies of that paren loop and fixing two left `a().b(svc.c())` losing
  `svc.c`; they are one function now. The scan recurses, so `maxArgNesting` caps it at 64 —
  Go cannot `recover()` from stack exhaustion, so the guard on every parse entry point would
  not catch a runaway.
- **`new` needs its own arm wherever a chain is walked.** Treat it as an ordinary keyword and
  the scan meets `models.User(` on its own and invents a call to `User` on `models` — a made-up
  answer, which is worse than the missing one it replaced.
- **There are two variable scans and both must agree.** `scriptParser` is the full one;
  `globalScriptParser` is a much smaller one that skips every function body outright, which is
  what makes `VariablesVars`/`ThisVars` answerable per keystroke. Every rule about what does
  *not* declare a variable therefore has to hold twice — named arguments, struct-literal keys
  and script-tag attributes fooled both, and fixing one left the other reporting the same
  phantom names. `TestBothVariableScansAgree` pins them together.
- **`ident ident =` is what identifies a script-syntax CF tag**, not a list of tag names.
  Importing `internal/docs` for one would pull a generated 6,582-line file with an `init()`
  into a package kept deliberately dependency-light, and a tag it did not list would silently
  go back to declaring variables. The spelling is unambiguous in CFScript. It needs one guard:
  `parseFuncBody` runs the *script* parser over a tag function's raw text, so
  `<cffunction name="save">` reaches the same test — hence the `afterLT` flag, and why the
  attribute scan consumes **pairs** rather than everything up to the body. An earlier version
  that swallowed "up to the body" ate whole `<cffunction>` bodies and every local inside them.
- **A function assigned to a name is a method.** `this.helper = function(a) { … }` and
  `variables.helper = (a) => a` are how a component exposes a method it builds rather than
  declares, and the parser recorded only a variable — so `Funcs` held none of them and the
  component had no completion, no signature help, no go-to-definition and nothing in the index
  for any of them. `parseFunctionValue` fires only outside a function body and only for `this.`,
  `variables.` and an unscoped name: a `var`- or `local.`-scoped closure is a local value, and
  declaring one as a method would put a helper private to one function into every caller's
  completion list. The paren-less single-argument arrow (`this.x = a => a * 2`) is deliberately
  not recognised — telling it from `this.x = a` needs three tokens of lookahead on every
  assignment whose RHS is a bare identifier, which is most of them.
- **Not every binding is an assignment.** The var-decl parsers only ever looked for `=`, so
  `for (var row in qry)` declared nothing — in every for-in loop, which is how CFML iterates a
  query, an array and a struct. `catch (any e)` was the same, in every catch block there is.
  `declareVar` files both; outside a function body they land in variables scope, since there is
  no local scope to put them in, and an unscoped `for (row in qry)` lands there wherever it is,
  which is CFML's rule for any unscoped assignment.
- **Two-character operators are the scanner's job, and the two are not alike.** `?.` is folded
  into `TokDot`, so every chain walk sees `svc?.save()` as `svc.save()` without a case of its
  own — a receiver missed in one of them is recorded as a *bare* call, which resolves as an
  unqualified function: a wrong answer, not a missing one. Adjacency is required, so a ternary
  keeps its `?`. `::` is *not* folded, because its qualifier is a component rather than a
  variable holding one: `TokDoubleColon` exists so the call site carries `Component` and
  resolution looks for the method in `Foo` instead of among the file's own functions. It used
  to be two unrecognised tokens, and `Foo::bar()` was reported as
  `bar (no qualifier, not in file)` — naming the wrong problem and unresolvable even with the
  component on disk.
- **A string can hold an expression that holds a string, and only CFScript may
  assume so.** `"#DayOfWeek("{ts '2000-1-1'}")#"` is one token; taking the first
  matching quote as the end left `"#DayOfWeek("`, so the interpolation had no
  closing `#` and its call was invisible — and the swallowed terminator took the
  rest of the construct with it, 3,165 sites over 461 corpus files. `scanString`
  now steps *over* a `#...#` and over any string opened inside it. It is
  **opt-in** (`Scanner.interpStrings`, set by `scriptParser.asCFScript()`)
  because text that is not CFScript reaches the same scanner — `parseFuncBody`
  hands a tag function's raw body to the script parser — and on markup the rule
  is a *wrong* answer rather than a coarse one: `<a href="#top">x</a><a
  href="#bot">y</a>` pairs the two fragment hashes and swallows the tags between
  them. `newGlobalScriptParser` opts in for correctness, not for calls: the two
  variable scans must tokenise a file alike, and without it `variables.a =
  "#f("x=1")#"` declares a phantom `x` in variables scope. The scan is
  speculative — unclosed nesting falls back to the plain quote-to-quote scan, so
  a token is never worse than before — and capped at `maxStringNesting` for the
  reason `maxArgNesting` exists.
- **CFML's string escapes are not C's, and the scanner used C's.** A quote is
  escaped by **doubling** it (`"say ""hi"""`); a backslash is an ordinary
  character. Honouring `\` meant a string ending in one — `listLast( uri, "/\" )`,
  `"lucee-tests\" & id`, a regex class, any Windows path — did not close where it
  ends: it closed at the next quote anywhere in the file and swallowed every call
  in between. One occurrence cost 333 call sites in a 1,918-line component. The
  doubling is checked *from inside* the string, which is what keeps a bare `""`
  the empty string and `f("", "b")` two arguments. Both scanners must apply it:
  `scanString` tries `scanQuoted` first and falls back to `scanQuotedPlain`, so a
  `scanQuoted` still honouring the backslash is *hidden* by the fallback —
  `TestBothStringScannersApplyTheSameEscapeRule` asserts the token text under the
  flag because a test that only parsed a component passed with that half
  reverted.
- **A UTF-8 BOM is not a token, and 561 of the 5,629 corpus files carry one.**
  `ClassifyRegions` decides script vs tag syntax by asking the scanner for the
  first token and testing it against `component`; a scanner that stops at the BOM
  answers "not a component" and the file goes to the tag splitter. That is
  harmless until the file mentions `<script>`, which `isScriptFile` reads as an
  HTML page — ColdBox's HTMLHelperSpec.cfc mentions it only inside the string
  literals it asserts against and came apart into eight regions, each parsed from
  the middle of an expression. `NewScanner` skips it, rather than `isScriptFile`,
  because it should not be a token anywhere.
- **A `new` expression's argument list is an argument list.** The four paths that
  read one — `parseNewRef`, `parseStandaloneNew`, `checkReturnComponent`,
  `scanChainedCalls` — each kept a `skipBalancedParens` of their own, the
  discard-a-token-at-a-time loop `skipParenBody` replaced everywhere else. They
  were reached from the `new` arm rather than from the argument scan, which is
  how they survived the consolidation, and
  `new Query( datasource = getDatasource() )` recorded no call at all. They go
  through `skipParens` now and that function is gone.
- **A call made directly on a scope needs its own arm, and which receiver it gets
  is the whole question.** `this.init()`, `variables.buildCache()`,
  `request.getRemoteClients()` recorded nothing: both scoped-var handlers read
  `scope` `.` `name` and then looked only for `=` or `.`, so the shape where the
  statement *is* the call fell through. `this.` and `variables.` name a member of
  the component being parsed, so the call is recorded **unqualified** and resolves
  against the file's own functions; every other scope holds a runtime value, so
  the receiver is **`$any`** — recorded unqualified, `request.getRemote()` in a
  file that declares a `getRemote` is an edge to a function the call never
  reaches. **Read the scope from the token, not the `Scope` value**: `request`,
  `session` and `application` are all dispatched as `ScopeVariables` so an
  assignment through one keeps its right-hand side's component, and testing the
  enum put every one of them in the first group.
- **A bracket index is an expression, and `skipBracketIndex` mirrored the *old*
  `skipParens`** — it discarded its group a token at a time. `sorted[ sorted.len() ]`
  and `arr[ f() ]` recorded nothing at all, and `g( arr[ f() ] )` only `g`: the
  defect `skipParenBody` exists to have fixed, left in the one group the
  consolidation did not reach. The `[]` poison marker is unaffected — a dynamic
  key still falls through to an honest "no component ref"; only the key
  expression is read rather than thrown away.
- **A hop chained onto a scoped call needs its receiver carried.** Both
  scoped-var handlers recorded the first call and skipped its argument list, so
  the `.c()` in `variables.a.b().c()` was rediscovered by the outer loop as an
  orphaned *bare* call — in a component declaring a `c`, an edge the call never
  takes. `continueChainCalls` exists to stop that on the unscoped path, so the
  scoped path goes through the same helper rather than growing a second one; a
  call made directly on a scope carries the receiver its first call earns
  (`variables.helper().c()` walks the declared return type, `request.get().c()`
  stays `$any`). **`make gapcheck` cannot see this**: it compares line and method
  name and deliberately not the receiver, so a call against the wrong receiver
  still counts as found.
- **The parser walks a chain in five separate places** (`checkVarRHS`, `parseBodyVarDecl`,
  `parseBodyScopedVar`, `checkAssignRef`, `checkBareCall`) and a construct met mid-chain needs
  the case in all of them. Three of the five were still reporting a bare `bar` when the first
  two handled `::`. `TestStaticCallCarriesItsComponent` lists every assignment form for that
  reason; add to it rather than fixing one walk.
- **"This function calls nothing" is not "this is not a function".** `FuncCalls`
  answered both by falling back to every call in the file, so a leaf method was
  handed its siblings' calls — `deps` drew an edge out of an empty function,
  labelled with another function's line number. A whole-file scan asks
  `AllCalls` by name instead, and that one sorts by line, because its four
  callers (`unresolved`, `explain`, the code map, the MCP server) each write a
  report meant to be diffed against an earlier one and the buckets come out of a
  map.
- **`#...#` is where a computed value reaches a string, and both parsers were
  blind to it.** The scanner takes a quoted string as one token and the tag
  parser never tokenises text, so `writeOutput("id #svc.getName()#")` and
  `<cfoutput>#svc.getName()#</cfoutput>` recorded nothing. Both now hand each
  span to a scriptParser — the tag side reusing the path a `<cfscript>` body
  already takes, during the walk rather than after it so `p.inFunc` is live and
  the call lands in the right function. **A span with no `(` is rejected on a
  byte scan**: only calls are recorded, `#user.name#` and `#i#` are most of the
  interpolation in any file, and without that guard tag parsing with call
  extraction cost 21% more rather than 4.6%.
- **Six loops walk tokens and a literal reaches all of them**, so a string or a
  closing bracket goes through `handleLiteralToken` rather than each loop
  growing its own copy of the rule. `TestEveryTokenLoopHandlesLiterals` parses
  the source and fails on a loop that dispatches identifiers to
  `scanNestedCall` without a `TokString` arm beside it.
- **A member call on a literal has a receiver that is a value.** `"abc".ucase()`
  recorded a *bare* `ucase()`, indistinguishable from an unqualified call to a
  function of that name — in a component that declares one, an edge to it that
  does not exist. The component is `$any`, the existing spelling for "genuinely
  dynamic".
- **`throw` is the one keyword invoked like a function**, so it is the one that
  needs its argument list consumed: `throw(type = "x")` declared a variable
  called `type`. `if`, `while` and `switch` hold an expression, which the
  statement scan reads correctly as it is.
- **A component's or interface's attribute list is not a run of assignments**,
  and `parsePlain`'s keyword guard runs *before* its tag-attribute check — both
  names are keywords, so `component extends="models.Base"` put `extends` into
  `VariablesVars`, which is what completion offers and what the index stores.
  Each needs its own arm in both scans.
- **A nested function's body is scanned, not skipped.** An immediately-invoked
  function lost everything it called while the same closure passed as an
  argument kept it, because that path counts parentheses instead. The calls are
  attributed to the enclosing function, which is where the closure runs from.
- **A *named* nested function is a declaration; an anonymous one is a value.**
  CFML hoists `function setup(){…}` written inside another function into the
  component's variables scope, which is what lets the enclosing function call it
  above the line it is written on. The script parser recorded nothing — no index
  entry, no completion, no go-to-definition — while the tag parser had always
  recorded it, so the same code meant different things in the two syntaxes.
  `skipNestedFunction` files a `FunctionDef` and a `FuncScope` for the named
  case, and `scanNestedCall` dispatches `function` *before* the `isKeyword`
  guard so the rule holds in every one of the six token loops — a helper
  declared inside a TestBox `describe(…, function(){ … })` is the common shape.
  The calls such a function makes stay attributed to the enclosing function, per
  the bullet above; only the declaration is new. A method of an inline
  `new component { … }` becomes a method of the enclosing component, which
  over-declares by one name and matches both the grammar and the tag parser.
  **The call axis could not see this**: the calls were already recorded at the
  right lines under the wrong function, so `make gapcheck` agreed. Asking the
  grammar which *names* it declares found it in one run.
- **`import models.User;` qualifies a later bare `new User()`.** `import
  models.*;` does not: which component a bare name then means is a question
  about what is on disk, and the parser has no filesystem.

### Formatter (`internal/formatter`)

Walks the tree-sitter CST. All rules (tag/attribute case, indentation, quotes, comma position,
SQL keyword casing, line width, attribute break threshold) come from `formatting` config. The
`whitespaceOnly` guard rejects any result that changes non-whitespace characters. `queryFormat`
is off by default — `<cfquery>` bodies are emitted verbatim unless opted in.

**Two entry points format a whole file and they must not drift**: the LSP's
`textDocument/formatting` handler (`internal/server/formatting.go`) and the `format` subcommand
(`cmd/cfmleditor-lsp/main.go`). Both go through the same two shared pieces, and new behaviour
belongs in them rather than at either call site:

- `config.ResolvedFormatting.FormatterOptions()` (`internal/config/formatting.go`) maps config
  onto `formatter.Options`, starting from `formatter.DefaultOptions()`. Zero-valued int fields
  mean "unset", so a partial `formatting` block keeps defaults for the rest. It deliberately
  leaves the three parse hooks nil, since those need `internal/language`; each caller installs
  them. When the CLI built its own option struct instead, `format -w` ignored every `formatting`
  key and emitted different bytes than the editor did for the same file.
- `formatter.ParseError(tree, src)` (`internal/formatter/parse_error.go`) refuses a file the
  grammar could not parse, so a CST gap can't be written back as deleted source. The
  `whitespaceOnly` guard is a second line of defence, not a substitute — it does not catch every
  shape of loss (see `FORMATTER-ISSUES.md`).

`UseTabs` is the one setting with no config key: the LSP derives it from the editor's
`insertSpaces`, and the CLI keeps the `formatter.DefaultOptions()` value (tabs). A workspace
`indentWidth` outranks the editor's `tabSize`.

The `-w` write path (`writeFileInPlace`) writes a sibling temp file, fsyncs, restores the
replaced file's permission bits, re-checks that the file on disk still matches what was read,
and renames over the target — following a symlink to its target rather than replacing the link.
Unchanged output is not written at all, so formatting an already-clean file leaves its mtime
alone.

### Code map (`internal/codemap`)

**It is not `deps` run once per file, and the difference is node identity.**
`deps.Build` is a *seeded* BFS whose labels carry the call's line number
(`Base.cfc (line 42)`) so repeated refs stay distinct inside one tree. Run it per
file and the same method becomes a different node in every seed's graph, so tens of
thousands of definitions become hundreds of thousands of unjoinable nodes.
`codemap.Build` is the inverted shape: one streaming pass over every file emitting
canonically-identified nodes and deduplicated edges — structurally the pass
`unresolved` already makes, carrying edges instead of error records.

**Node identity is the file path, never the component dot-path.** Dot-paths are
many-to-one (mappings, per-directory resolution, expression substitution) and one
spelling can name different files from different base directories. A function is
`<relpath>::<lowercased name>`; dot-paths ride along as display labels.

**Unreachable code stays in the map.** Every declared function is a node whether or
not anything calls it. `Node.Reachable` says whether an entry point reaches it and
`Node.Island` groups it, so a renderer draws a detached subsystem as its own tree
rather than the map quietly describing a tidier codebase than the one on disk.
`Reachable(nil)` and `Detached()` are the two destructive views and neither is the
default.

**Two walks, two different edge sets, and swapping them breaks both** (each has a
test that fails if you do):
- **Islands** count `contains` (file→function). Without it every uncalled helper is
  a one-node island and the count becomes a count of uncalled functions — 103
  islands on `testdata`, against 18 with it.
- **Reachability** does *not*. Reaching a file must not reach the functions it
  declares, or every method of every live component is "reachable" and the map
  reports a codebase with no dead code at all. A `.cfm`'s top-level code needs no
  special case: `callerID` attributes a call outside any function to the file node,
  so the file node *is* that code.

**Cost.** `indexPass` reads every `.cfc` twice over (index + fingerprint) and the
scan is parallel and **streaming** — a ParseResult is turned into edges and dropped
before the next file is read, because a few thousand `ExtractCalls` trees held at
once is gigabytes. Roughly 10s for 11,769 files / 48,715 functions, 7s warm.

**`resolve.ResolveCallTarget`** is the one new resolver entry point: it runs exactly
what `CanResolveCall` runs and also reports where the call landed. `canResolveCall`
already computed the callee and threw it away. Rather than widen its return across
twenty-odd `return ""` sites — the hand-maintained parallel list this file warns
about elsewhere — each accept path calls `tr.hit(...)` beside its existing
`tr.add(...)`, and **`TestEveryAcceptPathRecordsATarget` parses the source** and
fails on a `return ""` with no preceding hit. A forgotten hit costs a graph edge,
not a wrong one.

**The HTML viewer embeds its JavaScript.** `assets/vendor/d3.bundle.js` is a 76KB
esbuild bundle of the eight D3 modules the four views use, committed so `go build`
needs no Node and a report opens with no network. `make update-d3` rebuilds it from
`entry.js` — **adding a `d3.` call to the viewer means adding its module there**, or
it is undefined at runtime. `TestEmbeddedBundleCannotCloseTheScriptElement` fails if
a re-bundle ever introduces `</script`, which cannot be escaped inside inline JS.

**The wire format is compact, and it has to be.** 60,000 nodes as JSON objects with
full string ids was a 25MB page; interning every string, addressing edge endpoints
by node position, deriving a node's id when possible, and dropping `contains`
entirely makes it 4.4MB. `compact.go` and the viewer's `decode()` are two halves of
one format — `TestCompactRoundTripsEveryNode` and `TestCompactFlagsMatchTheViewer`
pin them together.

**The store's cache has a two-part key.** Content hash *and* a fingerprint of every
indexed `.cfc`, because resolving a call site reads the index built from every other
component: keying on content alone serves a stale edge set after an unrelated file
moves a method. Editing any `.cfc` therefore invalidates everything; editing `.cfm`
pages does not. `PruneCache` keeps one generation, since older ones can never match.

**`Store.Path` is a Go BFS, not a recursive CTE.** A CTE can express the walk but not
a visited set shared across branches — SQLite can only stop a trail revisiting its
own nodes — so a search from a high-fanout node with no answer to find is
exponential in the depth limit, and "is there any path" is asked most often about
pairs that have none.

**Framework routes (`internal/route`) are how routed code stops looking dead.** A
dispatcher's target is named nowhere in the source, so every routed controller
method reads as uncalled. The convention lives in the `routes` config block, not
in Go — `TestFw1Convention` resolves FW/1 with the same machinery as TASS, and is
there to stop the grammar quietly becoming one framework's rules.

Three things carry their reasons, each with a test:
- **The method must exist** before a controller rule claims a route. A component
  template built from `${1}` matches every route starting with that segment, so
  without the check the first rule shadows all the ones below it.
- **Alias keys are sorted, longest first.** Ranging the map directly made the
  winner depend on Go's randomised map order, so the same route resolved
  differently between runs of one build.
- **An alias may have several replacements and all of them are returned.**
  `ui.web` means the product serving the page, which is a runtime fact; the map
  marks such an edge `Dynamic` and go-to-definition returns several locations.

Routes are written four ways and the scanner handles all of them: HTML attributes,
`?do=` query parameters (unquoted, so they end at a URL delimiter, `&amp;` included),
JavaScript object keys quoted or bare, and function arguments (cut at the first
delimiter, since `redirect('a.b.c&x=1')` names `a.b.c`). Two boundary rules earn
their tests: `isNamePart` counts `:` as part of a name, which is right for
`xlink:href` and exactly wrong for a JS key where the colon *ends* the name, so
properties use `isPropNamePart`; and of two overlapping matches the **shorter**
wins, or every `href` swallows the `?do=` inside it.

`${N+:search}` enumerates method start points longest-first, because a route does
not say where the controller's name stops and the method's begins. It replaced one
hand-written rule per start point and is safe only because each candidate is still
checked against the component's real methods.

`cfmleditor-lsp routes --unresolved` groups what did not resolve by leading
segments. On tassweb: 82% of 2,741 occurrences resolve, and 257 of the 292
unresolved distinct routes share their first two segments with routes that *do* —
so the controller is found and the method name is what the route does not spell.
`dialog`-segment routes are 46% of the remainder.

`longestDir` exists because the view split cannot be templated: the file name
carries dots of its own, so the split between directory and file depends on what
is on disk. Measured on a real workspace: 79% of 1,022 route occurrences resolve.

**Each file is resolved under its own `.cfmleditor.json` (`Options.ConfigFor`).**
A workspace of several applications has one config each, and every one lists the
others in `workspacePaths`, so a scan rooted anywhere reads all of them. Under a
single config the other applications' `componentResolvers` never fire — it does not
error, it just resolves nothing. `cmd/cfmleditor-lsp/graph_config.go` memoises a
`resolve.Resolver` per config file, **all sharing one `index.Index`** (signatures
are a property of the workspace, not of whose resolvers you read them under), and
mixes every config's content hash into the cache fingerprint via `ConfigExtra`,
since the resolver chain decides what an edge points at. `--one-config` opts out.

**`Options.EntryGlobs` marks code a runner invokes by a constructed name.** No
static analysis can see `createObject("component", "prs" & version)`, so those
files look unreferenced and are not. On one workspace `../prs` alone was 10,639 of
11,374 apparently-unreferenced functions — 2,600 release scripts, correctly
unreferenced and entirely wrong to read as dead code. A bare directory name matches
everything beneath it, because `path.Match` has no `**`. Private methods are never
marked.

**A file node is a caller, not just a container.** Top-level `.cfm` code has no
enclosing function, so `callerID` attributes it to the file — and that is the
larger half of the graph, not an edge case: 74,226 of 107,683 call sites on a real
workspace come from file-level callers. `TestTopLevelPageCodeIsACaller` pins it.

**`--under` keeps boundary nodes, and the strict version is the trap.**
`FilterUnder` keeps a node outside the prefix when an edge crosses into it
(`Node.Boundary`). `Filter` — keeping only edges with *both* ends inside — drops
every caller from elsewhere, which is what you scoped the map down to find: on
`packages/tass/core` that was 472 edges against 7,999. A scoped map with no callers
reads as a broken tool rather than a narrow view.

**The builtin check runs *after* resolution, not before.** About sixty names in one
real workspace (`init`, `add`, `close`, `get`, `isValid`, the `onXxx` handlers) are
both declared functions and something `isBuiltin` recognises. Testing the name
first discarded every bare call to them before anything looked for a definition.

**`collectCFMLFiles` skips every dot-directory**, not a list of named ones. The list
was `.git`/`.svn`/`node_modules`/`target`/`vendor`, and on a real workspace it let in
`.claude/worktrees` — git worktrees holding a complete second copy of the codebase,
which doubled every count and added thousands of phantom entries to the unreferenced
list.

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
  actions, document formatting, range formatting, on-type formatting (`>`), document highlight,
  folding ranges, workspace folders.
- `textDocument/documentHighlight` (`internal/server/documenthighlight.go`) shades the other
  occurrences of the identifier under the cursor. Deliberately a *textual* answer, reported as
  `DocumentHighlightKindText`: matching is whole-identifier and case-folded through the same
  `identSpan` the references handler uses. It never leaves the open buffer, which is what
  separates it from `textDocument/references` and why it is on by default.

  The protocol's Read/Write distinction is deliberately not attempted. CFML spells too many
  things with `=` — an assignment, a named argument (`f( name = 1 )`), a tag attribute
  (`<cffunction name="x">`) — so a cheap "is the next token `=`" rule would mark the attribute
  *name* in every tag as a write. A wrong Write badge is worse than none.
- `textDocument/foldingRange` (`internal/server/folding.go`) folds on the CST rather than on
  indentation, which gets tags wrong constantly. A script-syntax `.cfc` is the case that makes
  this more than a tree walk: the CFML grammar hands its whole body to the CFScript grammar as
  one opaque `cf_component_content`, so walking only the outer tree yields exactly one fold for
  the file. Each injected region is parsed with its own grammar and walked too, its rows offset
  by where the region starts.

  **The walk descends through named children only**, and every accessor on a
  tree-sitter node is a cgo call — the walk measured 71% `runtime.cgocall`. An
  anonymous node is a grammar literal and tokens are leaves, so nothing
  foldable hides under one (`TestAnonymousNodesAreLeaves`; also checked by
  running both walks over the corpus — 92,001 folds, none different). For the
  same reason `Range()` is read once per node instead of
  `StartPosition`/`EndPosition`/`EndByte`, the single-line rejection runs
  before anything else, and a `depth` counter replaces a `Parent()` call. What
  is left is dominated by the CFScript sub-parse of a script `.cfc` body,
  which is inherent to the design above: 6.5ms for a 500-line component,
  against 13ms before.

  Four rules, each with a test that fails without it. A node needs a **named child** to fold, or
  it is a run of text and folding it is gutter noise — a comment is the deliberate exception. The
  **closing line stays visible**, which takes two separate checks: the deepest last *token* is
  what closes a node (a `function_declaration`'s `}` belongs to its `statement_block`, so
  reading the immediate last child misses every wrapper), and a node ending inside the **leading
  whitespace** of its last line has no closing token of its own (`<cfelse>`'s branch ends at the
  tab before the enclosing `</cfif>`). Identical ranges are **deduped**, keeping the outermost,
  since the CST nests wrappers that add no lines. And a node whose whole extent is one opaque
  injected region is **skipped**: `component_file` wraps a script `.cfc` and would fold the file
  to nothing.
- `textDocument/rangeFormatting` (`internal/server/range_formatting.go`) formats the **whole**
  document and returns only the edits inside the requested lines. Formatting the selected text
  alone is the obvious approach and wrong twice over: a selection rarely parses standalone, and
  its indentation depends on everything enclosing it. Going through the whole document makes the
  result identical line-for-line to format-on-save, so the two commands cannot disagree.

  Lines are matched on their **trimmed** text (`internal/textdiff`, a Myers line diff). Matching
  raw lines collapses a reindented block into one hunk with nothing to anchor on; trimmed, a
  reindented line still matches itself and the diff sees only the lines the formatter genuinely
  added or removed.

  An edit is kept only when it lies **entirely** within the selection. Overlap is not enough: a
  formatter change is not always divisible — joining a five-line `<cfif …>` header onto one line
  is a single hunk covering all five — and keeping it because it reaches into the selection
  rewrites lines the user did not select. The overlap rule did exactly that in 1,745 of the
  5,509 corpus files that format. The guarantee is therefore: every change fitting inside the
  selection is applied, and one straddling its edge waits for a wider selection or a document
  format.
- `workspace/didChangeWatchedFiles` (`internal/server/watchedfiles.go`) keeps the index current
  when files change outside the editor. There is no static way to ask for this — the protocol
  offers only dynamic registration — so `initialized` sends `client/registerCapability` for
  `**/*.cfc` and `**/*.cfm` when the client advertised
  `workspace.didChangeWatchedFiles.dynamicRegistration`, and logs once when it did not. Nothing
  is needed from the extension. Before this the index was a startup snapshot: `indexWorkspace`
  ran once and only `didOpen`/`didChange`/`didSave` updated it afterwards, so a checkout or a
  second editor left the server resolving to components that no longer existed, with
  `cfmleditor.reindex` the only cure — and in daemon mode one stale snapshot served every
  client and outlived the editor.

  Three rules decide what an event does, and each has a test that fails without it. An **open
  document's buffer outranks disk** for content, since the event is either the editor's own
  save or a change the editor will reload and report itself. A **deletion is honoured whether
  or not the file is open**: content is the editor's to report, existence is not, and an entry
  pointing at a path that is gone cannot recover, while re-saving a buffer re-indexes it. A
  **read that fails is treated as a deletion**, so a file caught mid-write or created and
  removed inside one batch does not leave its previous contents indexed as current.
- `textDocument/references` (`internal/server/references.go`) is **opt-in**: off unless
  `"references": {"enabled": true}`, and `capabilities()` advertises `referencesProvider` only
  when the flag is on, so a client that has not opted in never offers the command. It is gated
  because one request walks and parses every CFML file under `searchRoots()` — the same scan
  `cfmleditor.findRefs` and the `refs` CLI do — with no incremental call-site index to answer
  from. A dot-path under the cursor searches `refs.Options.Component`; anything else is a
  function name and searches `refs.Options.FuncName`. The search is scoped by the file that
  *declares* the function (`declarationOf`, which follows go-to-definition's order of
  preference), not by the requesting document, so it works from a call site. `refs.Entry` has a
  line and no column, so `entryRange` recovers the column by finding the identifier on the line;
  component entries whose line does not name the component are dropped rather than reported as a
  whole-line match, because the parser also records the variables a component ref flows into
  (`report = myCtrl.getReport()` is a ref to myCtrl's component on a line that never names it).
- **`features`** (`config.Features`/`ResolvedFeatures`) switches off individual capabilities.
  Three default to **on** and are opt-outs; **`folding` defaults off** and is opt-in
  (`config.foldingDefault`), because a script-syntax component's body reaches the CFML grammar as
  one opaque region, so answering one request means parsing the whole body with the CFScript
  grammar — a few milliseconds on a large component, and irreducible without caching a parse tree
  per open document. The fields are `*bool` for the reason the `completions` block documents — a
  defaults-true flag as a plain bool cannot tell "turned off" from "not mentioned", so naming one
  key would switch off its siblings. `mergeFeatures` unions key by key for the same reason
  `mergeFormatting` does. `featureDefaults` in `features_chain_test.go` states every default and
  fails if a new switch is added without one.

  Two traps this shape sets, both with a test:
  - **The zero value is every switch off.** `NewServer` therefore seeds `config.ResolveFeatures(nil)`,
    exactly as it does for `completions`; a session that never reaches `applyConfig` would
    otherwise advertise none of them and look like a build without the features.
  - **A gate in the handler is one a direct call slips past.** The watched-files gate lives in
    `applyWatchedFileChanges`, not in its handler, because the first test of it called that
    function directly and passed with the switch ignored.

  Disabling a feature **un-advertises** its capability rather than declining the request, so the
  editor falls back to its own behaviour. `rangeFormatting` is a second gate *under*
  `formatting.enabled`, not an alternative to it — it is the only one of the four that writes to
  the buffer, and it shares all its machinery with format-on-save.
- **Columns are UTF-16 at the edge and bytes inside.** An LSP column counts
  UTF-16 code units, and every parser helper reads a line in bytes, so a
  handler converts once where a column arrives (`byteCol`) and once where one
  leaves (`lineCol`, or a `colMapper` for a response carrying many ranges) —
  never in between (`internal/server/position.go`). Casting
  `params.Position.Character` to an int reads the line at the wrong byte on
  any line with an `é` or an emoji before the cursor;
  `TestNoHandlerReadsAClientColumnRaw` fails on that cast. `didChange` is
  the exception by design: it hands its ranges to `parser.ApplyEdit`
  unconverted, because the parser converts them against text only it holds.
- **A large didChange batch is applied in one pass.** More than 50 changes in one
  notification (the "rapid" path) go through `parser.ApplyEdits`, which streams edits in
  document order instead of walking and copying the whole document for each. Reverting a
  reformat in Zed sends exactly that, an edit per line: 6,000 against a 110 KB file took
  435ms under the document's lock one at a time and takes about 5ms streamed. An edit out of
  order falls back to `ApplyEdit`, and `TestApplyEditsMatchesApplyEdit` holds the two to the
  same result on random input, UTF-16 columns and past-the-end positions included.
- **An edit outside a function defers its reparse; `lockDoc` pays it.** `ApplyEditResult`
  reparses every signature in the file for an edit outside a function body, and didChange
  runs on the read goroutine: 95ms per keystroke at component level in tassweb's
  `kiosk.cfc`. didChange now applies the text, marks the document with `scheduleReparse`,
  and returns (0.35 to 3ms there). `lockDoc` runs the owed reparse (`flushReparse`) before
  handing over the lock, so no handler, timer or background job ever reads a lagging
  ParseResult; the first one after a run of typing pays once. didChange itself takes
  `lockDocOnly`, as does didClose. While a reparse is owed, later edits are applied as text
  only, since their positions no longer match the parse's scopes. A timer (`reparseDelay`,
  200ms) catches up a document nobody asks about, which bounds how stale the index's view
  of it can be for requests on *other* files. The rapid-change path uses the same
  mechanism. Code that reads `s.parseResults` must hold the lock from `lockDoc`, which the
  entry points already do; `TestDeferredReparseIsCaughtUpByTheNextHandler` and its
  neighbours pin it.
- **Formatting answers with the changed runs of lines, not the document.** `lineEdits`
  diffs the source against the formatted text (a patience diff on trimmed lines, so a
  reindented line still anchors, with adjacent changed lines merged into one edit) and
  `TestLineEditsReproduceTheTarget` holds applying them to exactly the formatted text. It
  replaced a single whole-document TextEdit, which the editor diffed again itself and, on a
  65,000-line file, was 3.6MB. `internal/textdiff` (range formatting's Myers) is not used
  here: it keeps a frontier per edit distance, quadratic memory in the changed lines, and a
  first reformat changes most of them.
- **Slow handlers release the read loop.** Handlers run inline on the read goroutine, so
  one slow request held every message behind it. Formatting, range formatting,
  `explainCall`, `exportDeps` and `findRefs` call `releaseReadLoop` once they have the
  document text and settings, and do the rest concurrently. Only handlers that work from
  that captured text may: the cached ParseResult is edited in place by didChange, so a
  handler that reads it must stay inline. `TestFormattingDoesNotHoldTheReadLoop` holds the
  formatter on a channel and requires a didChange and a request sent meanwhile to be
  handled.
- **Completion hands its cached items over, it does not copy them.** The
  `inHashExpr` branch and the default branch both return
  `completionFromCache`'s slice directly when there is nothing to merge with it,
  so the response can be the process-wide `getBuiltinFuncItems` list (a
  `sync.Once` shared by every session in the process) or a document's cache
  entry. **Nothing downstream may write to `items`, and nothing may append to it
  after that point** — an append would write past the length into an array a
  concurrent request is appending to as well. `applySnippetPolicy` therefore
  copies before editing; it used to edit in place, which permanently rewrote the
  shared list for every session the first time a config turned snippets off.
  `TestCompletionDoesNotWriteToTheSharedBuiltinList` pins this, and fails if the
  in-place edit comes back. Removing the copy took a completion request from
  383KB to 3.4KB, roughly half the bytes of the whole round trip including its
  JSON marshalling.
- **Documentation and detail are resolved lazily when the client can.** A
  client listing them in `completionItem.resolveSupport.properties` is sent the
  built-in and member-function items without them (`s.builtinFuncItems()` and
  `s.memberFuncItems()`, one shape per combination, built once per process), and
  `completionItem/resolve` looks the entry up again: a built-in by its label,
  recognised by its `SortBuiltinFuncs` sort text, and a member by the entry name
  in its `data`, because member names repeat — `len` is `arrayLen`'s,
  `stringLen`'s and `structLen`'s. A client that lists neither is sent the full
  items and is not told `resolveProvider`. The deferred shapes are shared like
  the full lists and the same rule holds: nothing may write to them. Reach for
  `s.builtinFuncItems()`, not `getBuiltinFuncItems()`, anywhere a list goes to
  a client. `TestResolvedItemsMatchTheFullOnes` resolves every deferred item and
  compares it with the full one.
- Diagnostics come from CFLint when `"linting": {"enabled": true}` — `internal/cflint` downloads
  the binary from `cfmleditor/CFLint` releases on first use.

  **`mapSeverity` may return only Error or Warning.** CFLint's seven levels
  (`com.cflint.Levels`: FATAL, CRITICAL, ERROR, WARNING, CAUTION, INFO, COSMETIC,
  plus UNKNOWN) fold onto the LSP's four, and an editor shows neither Hint nor
  Information by default — VS Code draws a Hint as a faint underline and keeps it
  out of the Problems panel, and hides Information behind its "Show Infos" filter.
  A level mapped to either is published, counted in the `cflint scan complete` log
  line, and then invisible, which reads as a diagnostic that was never produced.
  That is exactly how CRITICAL and CAUTION went missing: both were absent from the
  switch, so the second-most-severe level CFLint has was reported more quietly than
  COSMETIC. `TestEveryCFLintLevelIsVisible` states the enum in full and fails on a
  level that maps to Hint or Information; the default arm returns Warning for the
  same reason, so a level added upstream surfaces rather than disappearing.

  **`linting.minSeverity` filters on CFLint's raw scale, not the mapped severity.**
  Because INFO and COSMETIC fold up onto Warning, a floor applied after the fold
  could not tell them from a real WARNING and would keep every INFO it was meant
  to drop. `meetsFloor` therefore runs in `toDiagnostics` against `issue.Severity`
  before `mapSeverity` sees it, and a severity the enum does not list is kept
  whatever the floor — a setting that never mentioned a level should not be what
  hides it.

  `Linting.Enabled` is a `*bool` for the reason `completions` documents: the block
  has two keys now, so a plain bool could not tell "turned off" from "not
  mentioned", and `mergeLinting` unions key by key — otherwise a child config
  naming only `minSeverity` would switch linting off while appearing to tune it.
- `cfmleditor.findRefs` writes its `refs-<name>.md`/`.dot` report only when its third argument is
  `true`. It used to write unconditionally, which meant the code action on an ordinary "find all
  references" gesture dropped two files beside the source file being read. The plain code actions
  pass two arguments; a separate "Export references to X to a file" action passes the third.
- `workspace/executeCommand`: `cfmleditor.reindex`, `.format`, `.showComponentPath`,
  `.restartDaemon`, `.showResolvers`, `.showFileIndex`, `.showConnections`,
  `.openActiveApplicationFile`, `.goToMatchingTag`, `.copyPackage`, `.findRefs`, `.exportDeps`,
  `.scanWorkspace`, `.generateCodeMap`, `.showCodeMapStats`, `.resolveRoute`,
  `.exportUnresolved`, `.exportCFLint`, `.explainCall`.

  **Everything a CLI subcommand did for an editor task has a server route**, because Zed
  asked the zed-cfml extension to drop its `tasks.json` tasks and reach the server instead.
  Zed has two routes: code actions (`cmd-.`, every version) and, from 1.21, an LSP command
  picker listing `executeCommandProvider`. A command missing from that list is not in the
  picker, so a new one goes in the list as well as in `handleExecuteCommand`'s switch. **Zed
  ignores a command's return value**, so anything the user must read goes through
  `window/showMessage` or a written file; returning it as well costs nothing and serves
  clients that do read it. The mapping from the removed tasks: `scan` →
  `.scanWorkspace`, `format` → `textDocument/formatting`, `explain` → `.explainCall`,
  `unresolved` → `.exportUnresolved`, `cflint` → `.exportCFLint`, `refs` → `.findRefs`
  (and `textDocument/references`), `deps` → `.exportDeps`.

  Code actions (`internal/server/codeaction.go`), in order: "Explain call resolution on line
  N" when the cursor's line holds a call; the find/export actions for the word under the
  cursor; then, wherever the cursor is, "Export dependency graph for <file>" (`.exportDeps`
  with the URI alone, the whole-file graph) and the workspace actions, "Scan workspace for
  parse errors", "Export unresolved calls report for the workspace" and "Export CFLint report
  for the workspace".

  **`explainCall` parses the buffer afresh rather than using the cached ParseResult**
  (`parseForCalls`). The cache cannot answer "what calls are on this line": an edit outside a
  function reparses it shallowly, which drops every call recorded inside one
  (`resetFuncCaches`), and an edit inside one shifts scopes and refs but not calls. One
  keystroke therefore left it reporting no calls, or calls on the wrong line.
  `TestExplainCallAfterAnEditOutsideAFunction` fails if the command goes back to the
  cache. The private parse needs no document lock. The code action does not parse at all:
  `lineMayHoldCall` looks for a name before a `(` on the cursor's line. It was a memoised
  full parse, which missed after every edit, and Zed asks for code actions each time the
  cursor settles, on the read goroutine: 11ms on a 12,000-line component and 87ms on a
  65,000-line one. Against the parser on tassweb's `kiosk.cfc` and `timesheet/persist.cfc`
  the text check misses 5 of about 23,500 call lines, each a call split over lines, and
  the command's own answer is exact regardless. The report text and the selection by line and filter are
  `resolve.WriteExplanation` and `resolve.CallsOnLine`, shared with the CLI, so the two print
  the same thing. The server's answer uses the session's resolver and index, where the CLI
  builds its own from the file's nearest config, so the two can disagree for the reason
  `--root` exists.

  `.exportDeps` had the same cache problem and now parses afresh the same way. Reading the
  cached ParseResult, a comment added above the component left it no calls, and the graph fell
  back to the index's component refs: `controller.cfc --> service.cfc (line 2)` in place of
  the functions called. `TestExportDepsAfterAnEditOutsideAFunction` pins it.

  **`resolveRoute` exists so an editor does not keep its own copy of the
  convention.** A command that takes a route by hand and go-to-definition on a
  route in source must answer alike; two resolvers reading two configs disagree
  quietly, and the wrong one still opens a file. It answers rather than errors
  when no convention is configured, because a JSON-RPC error is not something a
  command handler can show anyone.

  **`generateCodeMap` reuses the server's index instead of building one** — it is
  already current, and re-reading every `.cfc` is the more expensive half of a cold
  CLI run. It runs on `safeGo` with `context.Background()` (the handler's ctx is
  pooled and reset on return, same reasoning as `scanWorkspace`) and reports through
  `window/showMessage`, because a blocking `executeCommand` freezes the editor for
  the ten-plus seconds a large workspace takes. `codeMapOutputPath` refuses a target
  outside the workspace: the arguments come from whatever asked the editor to run
  the command.

## Capabilities the VS Code extension has and this server does not

The `cfmleditor` extension stands its own language providers down whenever this
server is running, on the rule that enabling the server hands it the language.
That rule is simpler to hold than a per-capability list, and it costs three
things the extension could answer and this server cannot. They are listed here
so the loss is deliberate and so whoever implements one knows what it has to
match.

It was four. Variable definitions were the largest, and they are closed — the
conformance suite below is what made that a measured change rather than a claim.

| Missing here | Extension's implementation | Notes |
|---|---|---|
| `textDocument/typeDefinition` | `CFMLTypeDefinitionProvider` | Go to the *type* of the symbol under the cursor, rather than its declaration. Most of the machinery exists — `CanResolveCall` already resolves a receiver to a component, which is the answer this request wants. |
| Docblock completion | `DocBlockCompletions`, triggered on `*`, `@` and `.` | `@param`, `@return` and friends inside a `/** */` block. Note the trigger characters: `capabilities()` advertises `<`, `/`, `.` and `>`, so adding this means widening that list as well as handling the context. |
| `textDocument/documentColor` | `CFMLDocumentColorProvider` | Colour swatches and the picker for colour literals. Wholly absent here; nothing in the parser records them. |

`documentColor` is the one with no foundation at all; the other three each have
most of their machinery already. Until they land, a user who enables the server
loses them — which is worth remembering when one is reported as a regression
rather than a gap.

**The list is measured, not maintained by hand.**
`internal/server/definition_conformance_test.go` replays the extension's own
`provideDefinition` suite — same cases, same cursor-marker syntax, fixtures
copied into `internal/server/testdata/conformance` with their source commit
recorded. All 41 pass. `knownGaps` is empty and kept, because **the test fails when a
known gap starts passing**, so closing
one cannot go unnoticed and the list cannot rot into decoration.
`TestKnownGapsAreRealCases` fails on a gap naming a case that no longer exists,
so a renamed case cannot hide as a fixed one.

The fixtures live in the *package's* testdata rather than the repo root's. They
are a whole second workspace, and putting them in the shared `testdata/` changed
what the repo-wide scans find — `TestReachabilityDoesNotFollowContains` failed
on the fixture `Application.cfc`'s `onRequestStart`, correctly, because a second
application had appeared in the tree it walks.

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
| `linting.minSeverity` | Least severe CFLint level reported, on CFLint's own scale (`FATAL`…`COSMETIC`); unset reports everything. See below |
| `references.enabled` | Answer `textDocument/references` (off by default; see the LSP surface above) |
| `features` | Per-capability switches: `documentHighlight`, `watchedFiles`, `rangeFormatting` default **on** (opt-outs, for when one misbehaves); `folding` defaults **off** (opt-in — it is the most expensive request to answer). See below |
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

The same trace is available from the editor: `cfmleditor.explainCall` (document URI,
0-based line, optional filter), offered as the "Explain call resolution on line N" code
action on any line holding a call. It answers with the running server's resolver and index
rather than a `--root` config, and shows the report as a message.

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

## Where `#...#` in text is CFML

A tag file's text between tags is scanned for `#...#` only inside an output
context (`internal/parser/output_context.go`), and a `.cfm` with no CF tags that
holds HTML is a tag region, not CFScript. Outside an output context a hash is a
literal character, and pairing stray ones (`href="#"`, CSS colours, jQuery
`$('#id')`, Handlebars `{{#each}}`) swallowed the markup between them and turned
every `name(` in it into a call. `features.outputContextInterpolation` (default
on) switches it off; it reaches the parser as `ParseOptions.InterpolateAllText`,
which every call-extracting `ParseOptions` site must pass.

- **The rules were measured against Adobe ColdFusion, not remembered.** One
  probe template per context in the `tass_coldfusion` container. Evaluated:
  `<cfoutput>` (HTML attributes and `<script>` in it included), `<cfquery>`,
  `<cfmail>`, `<cffunction output="true|yes">`, and the attributes of every
  `<cf...>`, `<cf_...>`, `<cfmodule>` and cfimport-prefixed tag anywhere. Not
  evaluated: plain text, HTML attributes and `<script>` outside those,
  `<cfsavecontent>`, `<cfxml>` and custom-tag bodies, functions with `output`
  unset or false, and a template `<cfinclude>`d from inside `<cfoutput>`.
  `<cfdocument*>` and `<cfcomponent output="true">` could not be exercised and are
  taken as evaluated, because a wrong guess that way costs a false positive and
  the other way a call.
- **Attributes are always scanned.** A handled tag scans its own; a declined
  `<cf...>` tag and a `<prefix:tag>` whose prefix the file `<cfimport>`s scan
  theirs in `stepOverEvaluatedTag` and are stepped over whole, so the gate never
  sees them. Leaving them in the next text gap is what the ungated walk does, and
  gating that gap lost them: tassweb has 3,296 `#fn()#` calls in custom-tag
  attributes.
- **Output ranges are file offsets.** A tag region is cut around `<cfscript>`
  blocks and a `<cfoutput>` can open before one and close after it, so
  `outputContext` runs on the whole file and each region carries `Offset`.
- **The tag walk's span stepping is gated too.** `nextTagStart` steps over a
  `#...#` span because markup inside one is an argument. Outside an output
  context that is wrong: a Handlebars `{{#` paired with a hash far below and the
  walk stepped over real `<cfif>` tags, whose calls the old reading recovered
  only by scanning the whole run as one expression. `nextTag` uses a plain `<`
  search there. Finding it took a corpus diff of every extracted call, not of
  unresolved ones: the loss showed as a resolved builtin going missing.
- **It fails open.** An unclosed output tag runs to the end of the file, and a
  close with nothing open is ignored.
- **A `<script>` block with no CF tag is a `RegionSkip`, and still read.** It is
  kept from the CFScript scanner but handed to the tag parser, which finds
  nothing in it but `#...#` spans, and those only in an output context.
- **`Caller` is filled from the line.** A sub-parser for a tag expression or a
  `#...#` span, and a region cut from a function body, name no caller of their
  own; `fillCallers` gives every call without one the function whose scope holds
  its line. Before it, 514,852 of 895,451 calls in the TASS workspace had none.

## cfinclude scope

A bare or `this.` call that neither the file nor its extends chain answers is looked up through
cfinclude (`internal/resolve/includes.go`), and lands as `TargetInclude`. An included template
runs in its includer's variables scope, so the scope is every file that includes the calling
file, transitively, plus everything each of those — and the file itself — includes, plus the
extends chain of any component among them. A template included into `api.cfc` beside forty
others reaches `api.cfc`, its base, and every sibling. A file with several includers resolves
against all of them; which one ran is not in the source.

- `parser.ExtractIncludes` reads only `<cfinclude template>`, `include "…"`, `include
  template=` and `cfinclude(template=)` naming a `.cfm`/`.cfml`, skipping `#…#` paths and
  `<!--- --->` comments. Not `ExtractLinks`: `<cfmodule template>` runs in a scope of its own.
  It runs on every indexed file, so it finds the keyword and matches only there — two
  whole-file case-insensitive regexes took tassweb's index pass from 1.5s to 5.5s.
- The index holds each file's raw include paths and an `includeGen` counter; the resolver
  resolves them (file dir, `Application.cfc` root, mappings by first segment, workspace
  folders) into a forward and reverse graph and rebuilds it only when the counter moves.
- **`removeFileEntries` must not touch includes.** Every re-index runs it first, and clearing
  them there moved the counter on every re-index of an unchanged file, rebuilding the graph —
  thousands of stats — on each lazy index during a scan and each edit in the editor. Removal
  goes through `setIncludesLocked(uri, nil)` in `RemoveFile`, which compares before it bumps.
- A writer that indexes from a parse result calls `Index.SetIncludes` beside `SetThisVars`;
  `IndexFile` sets them itself. The `unresolved` command records a template's includes without
  indexing its functions, since a page that includes a helper can call what the helper declares.

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
- Resolvers are tried in **array order** — so when two resolvers could both match the same
  expression (e.g. a specific `getDirectContent()` entry and a generic `get$1()` catch-all),
  whichever is listed **earlier** wins, regardless of specificity. To make a specific case win
  over an existing broad catch-all, add it *before* the catch-all in the array, not after.

  Two implementations honour this and must stay in agreement: `ResolveFromCallMatch` (behind
  `ResolveFromCall`/`ResolveFromCallFull`, and so behind `CanResolveCall`) walks the slice
  directly, while `ResolverSet.Resolve` (behind completion and hover) first narrows candidates
  through a first-byte index and then sorts them back into array order. The byte index decides
  only *which* resolvers are worth trying, never which one wins — it used to leak its own order
  through, so for resolvers whose prefixes start with different letters the winner was decided by
  where each prefix happened to appear in the call-site text, and the two paths could return
  different components for the same expression. `TestResolverSetMatchesArrayOrder`
  (`internal/parser/resolver_order_test.go`) pins the two paths together; add a case there rather
  than to only one path.
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

**Known parser limitation: a CFML comment inside a tag's attribute list**

This note used to say the tag parser "does not fully skip `<!--- ... --->` comment blocks that
contain embedded CFScript or `<cfset>` tags", which reads as a broad problem and is not one.
Re-probed across seventeen shapes, fifteen are handled correctly — a tag comment holding
`<cfset>` or a whole `<cfscript>`, a comment inside `<cffunction>` or `<cfoutput>`, `/* */` and
`//` in cfscript, a CFML comment inside `<cfscript>`, a use-in-comment with a live declaration,
nested comments, a comment holding an entire `<cffunction>`, an unterminated comment at EOF, an
unbalanced quote, a tag-syntax `.cfc`, a four-dash opener, and CRLF.

One shape looks like a bug and is not: a `--->` inside a quoted string ends the comment early.
tree-sitter puts the `cf_comment` end at the same place, because CFML comments are not
string-aware — an engine does the same. Do not "fix" it.

What genuinely diverges is a comment **between a tag's attributes**:

```cfml
<cffunction name="real" <!--- <cfset p = getThing()><cfset p.gone()> ---> output="false">
```

tree-sitter parses that properly (`cf_attribute`, `cf_comment`, `cf_attribute`). The tag parser
does not: roughly eight sites locate a tag's closing `>` with a bare
`strings.IndexByte(src, '>')`, and the first `>` here is the one inside the comment, so the tag
is treated as ended and the rest of the comment is scanned as live tags — reporting `p.gone()`
as an unresolved call. Quoted attribute values *are* handled (`<cfset s = "a > b">`,
`hint="returns a > b"` and `<a title="x > y">` all parse correctly); only comments are missed.

**The `>`-in-a-string half of this is fixed.** `tagEndIndex` is the shared
quote-skipping scan that note asked for, and the walk's one tag-end site goes
through it: `<cfset x = array( f( "a<br>b" ), g( "c" ) )>` no longer ends at the
`<br>`. Comments between attributes are still missed, and the other sites that
locate a `>` by hand still do.

It answers on a **two-count fast path**, because it runs over every tag in the
file: a `>` with an even number of each quote before it has every string closed,
so it is the tag's end. That is exact rather than a heuristic — CFML escapes a
quote by doubling it, which adds two — and it must count *both* kinds, or
`<cfset x = "a" & 'b>c'>` ends inside the single-quoted string. The
quote-by-quote walk it falls back to is `tagEndWalk`, a function of its own so
`TestTheTagEndFastPathAgreesWithTheWalk` can compare the two rather than restate
either's answer. Walking every tag quote by quote was 5% of a plain tag parse.

**It is close to unreachable.** One file in the 5,624-file corpus contains the shape —
`Lucee/test/jira/Jira3190/index.cfm`, a regression test whose comment holds no code — so the
corpus produces zero false positives from it. Fixing it means one shared `tagEndIndex` helper
that skips quotes *and* comments, routed through all eight sites. That consolidation is worth
doing whenever someone next works in that area, since eight independent copies of the same scan
are the parallel-list hazard this file warns about elsewhere; it is not worth doing for this
bug alone.

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
  does; match it or `make lint` fails. **Lint via `make lint`, never a `golangci-lint` on
  `PATH`.** The Makefile pins the version (`GOLANGCI`) and builds it from source under this
  module's toolchain, because golangci-lint refuses to load a config whose module targets a newer
  Go than the linter binary was built with — `can't load config: the Go language version (go1.25)
  used to build golangci-lint is lower than the targeted Go version (1.26.6)`. That is a refusal
  to start, not a finding, and it is what a distro or Homebrew binary does for weeks after each Go
  bump. Test files are exempted from `prealloc`, `unparam`,
  `gosec`, and `staticcheck`.
- `internal/docs/` content is generated — regenerate rather than hand-editing, but see the
  lossy-regeneration warning under Commands before committing any change to it.
- `.github/workflows/ci.yml` runs on every pull request: `build-test` (build, vet, gofmt, `go
  test -short`), `race`, `lint` (`make lint`), `vuln` (`make vuln`), and an informational
  `perf` job that never gates a merge. None of them run `make docs`, since
  `internal/docs/generated_docs.go` is committed.

## Verification discipline

Four habits that have each caught a real defect in this codebase, and whose
absence has each let one through:

- **Confirm every new test fails with its fix reverted.** Comment out the change,
  run the test, see it fail, put it back. Tests that passed either way have been
  written here repeatedly — a `lowercaseTags` test with an all-lowercase fixture,
  a `scanWorkspace` test asserting indexing that never happens, a bare-block test
  whose fixture did not reproduce the bug it was named for.
- **Diff the corpus per file, not by totals.** `make corpus BASELINE=<earlier report>`
  does this. A change that breaks one file and fixes another leaves every column
  identical.
- **Run `go test -short -race ./...` before pushing.** CI's race job found a
  pre-existing data race on the workspace roots that a non-race run could not.
- **Print actual output before writing an expected string.** Hand-counting the
  indentation of a nested fixture is wrong more often than right.

**Cost that scales with the workspace hides in per-file operations.** The
things that run once per file, or once per keystroke, are where an
index-sized scan turns into a quadratic: `removeFileEntries` walked every
name bucket on every index write, which made a 5,624-file workspace scan
take 34s and allocate 6.6GB (0.22s and 95MB once it went through
`fileFuncs`/`fileRefs`); `workspace/symbol` materialised all 40,000
definitions per keystroke to return a few dozen;
`LookupComponentRefInFile` — which hover, definition and completion each
ask on the keystroke — searched the variable name's bucket for the file
rather than the file's refs for the name, and the names it gets asked
about (`svc`, `dao`, `qry`) have one entry per file in the workspace.
All three were invisible in a unit test and obvious in one profile. The
general form is the index's read/write asymmetry (see the `snapshot`
comment): make an operation cost what it *uses*, not what the index
*holds*. Reach for
`go test ./internal/index/ ./internal/server/ -bench . -benchmem -run '^$'`
before assuming an LSP path is cheap — the benchmarks there load an index
the size of a real workspace, which is the axis a handler benchmark on an
empty server cannot see. Pin the shape, not the clock, when it matters:
`TestIndexFileFromResultDoesNotScaleWithIndexSize` compares allocations at
two index sizes, so it fails on the regression rather than on a busy runner.

**Search for the entries you are holding; do not sweep the bucket for them.**
Both index writers on the keystroke path — `removeFileEntries` on every
re-index, `ShiftLines` under the write lock on every edit that changes a line
count — looped over every entry filed under a name asking "is this one of
mine". For the names workspaces share, that bucket is one entry per file, so
the loop was the workspace and the answer was a handful. Worse, the per-entry
work was a *call* (a closure, then a `slices.Contains` over a one-element
group), not a compare. Inverted — `slices.Index` per entry actually touched —
the comparisons are the same and a 4,000-file shared-name workspace goes from
164µs to 28µs for a re-index and 101µs to 24µs for a shift. Neither is *flat*;
flat needs a pointer-to-position map beside the buckets, a few megabytes held
for the life of the index, and the tests say so rather than claiming otherwise.
**Measure such a thing on a file from the middle of the workspace** — the first
file's entries sit at the front of every bucket, where a search finds them at
once, so a sweep and a search look alike on it.

**A handler benchmark is not a request.** For the two responses that carry a
lot of items, encoding the answer dwarfs computing it: `workspace/symbol` on a
5,000-file index is 0.24ms of handler against 3.02ms of marshal, and completion
is 0.002ms against 0.63ms. So a saving inside either handler is rounding error,
and the only thing that makes them cheaper is sending fewer bytes.
`BenchmarkWorkspaceSymbolWithMarshal` and `BenchmarkCompletionWithMarshal`
measure them end to end; the handler-only benchmarks beside them are for
changes to the handler, not for what the request costs. `PERFORMANCE-GAPS.md`
records the three costs measured this way and not acted on, with what each
option would save and what it would cost the user. Capping `workspace/symbol`
is the obvious fix and is deferred on priority rather than overlooked; section
2 says what to know before picking it up, including that the cheap first step
is measuring a real workspace rather than building the cap.

**Benchmarks in one process contaminate each other, so isolate before believing
a regression.** A run of the whole benchmark set here reported `ScopesToFuncRanges`
69% slower, `documentSymbol` 29%, `workspace/symbol` 27% and `Keystroke` 21%
*faster* — none of which were real. The allocation-dominated ones inherit
whatever heap the benchmarks before them left live, and `benchLoadedServer`
leaves a 5,000-file index. Re-run the single benchmark on its own, both sides,
before acting on any of it. The same caution applies to wins.

**Compare against the branch point, not against your last build, and run the two
binaries alternately.** Five parser fixes in one round were each reported at
around +1% on the tag benchmarks, because each was measured against whatever
binary was left in the scratch directory rather than against `origin/main`.
Re-measured from a clean main baseline the round was **+9.4%** on the plain tag
parse — the editor's keystroke path — and the cause was a byte-at-a-time
`tagEndIndex` running over every tag in the file. No single reading had shown it.

**Then the corrected figure was wrong the other way.** The fix for that was
recorded as -0.7%, measured by running all of one binary's samples and then all
of the other's. Interleaved — one sample of each, alternately, twenty-five times
— the same pair reads **+10%**, because the machine drifts between the two runs
by more than the effect. A sequential A-then-B comparison is not a measurement,
however many samples each half has. Interleaving also settles the contamination
warning below: a `git worktree` of `origin/main` built into the scratch
directory gives a second `.test` binary to alternate with.

So: **keep a `main` build in the scratch directory, re-measure the whole branch
against it, and alternate**; treat min, p10 and median disagreeing as "not yet
measured" rather than as a result. A per-commit bisect then splits a cumulative
cost honestly — which is how the tag walk's +10% came apart into +3.2% for
bracket indexes, +5.1% for stepping over spans and +5% for the quote-aware tag
end, none of them a regression on its own, and how `tagEndIndex`'s two-count
fast path was shown to take the whole thing back to about +4%.

**A cost that scales with the document wants a scaling test, not a timing one.**
Every defect behind the three-second go-to-definition was invisible to the tests
that existed, because all of them returned the right answer. `routepkg.Scan`
lowercased the whole remainder of the file to test a four-character prefix, once
per `?` or `&` — 1.8GB and three seconds on a 64,000-line component, with the
refs correct throughout. `routeAtPosition` scanned the whole document and kept
only the cursor's line. Document links rescanned on every request.

The tests that catch these compare a **ratio**, never a clock:
`TestScanScalesLinearly` gives eight times the input and fails above three times
the input ratio (the defect was 63x, near-exactly quadratic);
`TestScanDoesNotAllocateOverContentWithNoRoutes` grows only the content that
matches nothing and fails if allocation follows it (the defect was 97x);
`TestRouteLookupDoesNotScaleWithDocumentSize` appends 460KB after the cursor and
fails if the lookup notices. A busy runner moves both numbers together, so they
fail on the regression rather than on the machine.

Two traps, both of which this file's tests fell into first. A route test needs
`Routes.Enabled()` to be *true* — it wants a source **and** a controller or view
rule, and without the second half every route path short-circuits and the test
passes whatever the code does. And a window test needs both documents to present
a **full** window, or it compares a five-line window against a fifty-line one and
fails for that instead.

**Where a tag holds an expression, hand it to the script parser.** The tag
parser matches tags and pulls attributes out with string searches; it has no
expression parser and should not grow one. A `<cfif>` condition, a `<cfelseif>`,
a `<cfreturn>`, a `<cfset>` and a `#...#` span all go to a `scriptParser` that
keeps only the calls, which is what a `<cfscript>` body has always done. One
implementation of "what is a call" then serves both syntaxes, and a fix to it
reaches tag files for free. On the six-project corpus that was 3,100 call sites
for `<cfset>` alone — a bare `<cfset arrayAppend(a, b)>`, a call after a
concatenation, a nested call in an argument list, any scope-prefixed left-hand
side — and 207 more for a script tag's attribute *value*, where the identifier
was consumed as a plain token and only its argument list was scanned, so
`array=structKeyArray(rows)` recorded what was inside the parens and never the
call. PARSER-GAPS.md has the measurement and what is still missing.

**Only `<cfset>` tops up, and that distinction is the subtle one.** Its string
paths run first and carry the refs and pending calls that decide what a variable
now holds, resolving a receiver against *this file's* refs in a way a fresh
sub-parse cannot — so the sub-parse subtracts the count already recorded for
each name on the line and adds only the excess. Counting rather than testing
presence is what keeps `<cfset x = f() + f()>` at two. Doing the same for a
`#...#` span is *wrong*: `#getColdBoxSetting("a")# #getColdBoxSetting("b")#` is
two calls on one line, and the shared merge with the tally on cost the second
one. **A unit test on one span could not have caught that; the corpus did.**

**The grammar is a second opinion on the parser, and `make gapcheck` asks it.**
`internal/parser` and the tree-sitter grammar are independent implementations of
"what is a call", so where they disagree one of them is wrong. Every call-losing
defect fixed here so far was found by hand-probing constructs one at a time;
this asks the question over a corpus instead. It found, in minutes on the repo's
own 40 fixtures, a class nobody had probed: in **tag syntax** a call in a
`<cfif>`/`<cfelseif>` condition, in a `<cfreturn>`, or chained onto an
instantiation (`<cfset d = createObject(…).init("ds")>`) is recorded nowhere,
while the same code in script syntax is. `TestKnownTagSyntaxGaps` states each
shape and fails when one starts working.

Three things about it, each of which cost a wrong conclusion first:

- **Calibrate before believing it.** The first run reported nine calls the
  parser had "invented" — every one a `queryExecute`, which the grammar gives a
  `query_expression` node of its own so SQL can be injected into it. That was
  the oracle's mistake, not the parser's.
- **Deliberate differences are not gaps.** `createObject(…)` is a `ComponentRef`
  to the parser and a call to the grammar, on purpose. `expectedDifferences`
  records both kinds with a reason each, and the test fails both on a *new*
  difference and on a listed one that no longer differs — so a gap cannot be
  closed unnoticed and the list cannot rot into decoration.
- **The declaration axis is much weaker than the call axis, and it is measured
  rather than assumed.** Of the phantom-variable shapes fixed in this parser,
  the grammar disagrees with the old behaviour on exactly one
  (`component extends=`). For the rest it agrees with the *bug*: `force` in
  `svc.save(force = true)` is an `assignment_expression` to the grammar too, and
  so is `name` in `query name="q"`. Those distinctions are semantic, not
  syntactic, so a syntax oracle cannot arbitrate them.
  `TestDeclarationAxisIsWeakerThanTheCallAxis` pins that ratio.

It cannot check resolution at all — the grammar has no idea what a dot-path
points at — and it is only as good as the corpus: the repo's fixtures contain
zero interpolated calls, which is why the size of that gap is still unmeasured.

**A `#...#` span is not a tag boundary, and the two scans that say where one
ends are deliberately different.** Markup inside a span is an argument:
`#ETH.author( content = "<strong>x</strong>" )#` was split at the `<`, losing
that call and — through the hashes left over — the call after it. The walk steps
over a span now, under three rules each measured by removing it: a span's hashes
**pair** whether or not its contents are stepped over (removing this costs 141
files, and it is the one-line mistake that made an earlier attempt take missed
sites from 1,798 to 3,025); a span with **no `(`** does not hide a tag, so a
stylesheet's `#sidebar ul {` stays markup; and a span with **unbalanced quotes**
does not either, so `$( '#search' ).typeahead(` stays a jQuery selector. Hashes
interpolate only inside `<cfoutput>` and in a tag's attributes and the walk
tracks neither, so these are heuristics — chosen so that being wrong costs a
call rather than a tag.

`interpolatedSpans` follows CFML's *nesting* instead — a string inside a span may
hold a span (`#html.elixirPath( root='#cb.themeRoot()#/inc' )#`), which is what
`Scanner.scanHashExpr` already does for CFScript. **Do not make the tag walk
share that rule**: it fixed one corpus site and broke thirty-six, because
skipping quoted strings runs a span much further in markup. A wrong span costs
the walk a tag and costs `interpolatedSpans` a call, and they are tuned for their
own error.

**A `RegionSkip` is a literal `<script>` block, and dropping it dropped its
interpolation.** The region exists so JavaScript is never fed to the CFScript
scanner, and the region was then not parsed at all — but a `<script>` body
inside `<cfoutput>` is exactly where a page writes
`var id = "#prc.oContent.getContentID()#";`. It goes to the tag parser now and
needs no mode of its own: `findScriptSkipSpans` only makes a span of a block
holding no `<cf` tag at all, so the walk can find nothing in one *but* its
interpolation. A flag restricting it to the spans was written first and removed,
because it measured identically over the whole corpus *and* against the
tag-shaped-text-in-a-JS-string case it was written for — which is not a skip
region precisely because it contains `<cf`. 124 sites over 34 files with no
invented call.

**A lone `#` in a CFML string is invalid, Lucee tolerates it, and the scanner
cannot tell it from interpolation.** `md.append( "# ColdBox Performance Report" )`
opens a span that closes at the next *real* interpolation two lines below,
swallowing every call between. Requiring a span to open and close on one line
looks obviously right and costs 45 sites net over the corpus, because a span
that genuinely wraps a line is how ContentBox writes a form field —
`#html.inputField(\n name = "authorEmail",\n …\n)#`. Both shapes cross lines,
hold quotes and hold a `(`; telling them apart needs to know the first `#` was
never interpolation, which is a fact about the enclosing expression rather than
the string. PARSER-GAPS.md §4.2 has the measurement.

**A hand-maintained parallel list wants a reflective test.** Wherever the same
names must appear in two or more places, enumerate them in a test rather than in
a comment. `mergeFormatting`'s field list had one and it caught two omissions
while the untested hops beside it stayed silent; the config and daemon chains
(`TestFormattingKeysReachTheFormatter`, `TestResolvedFormattingReadsEveryKey`) and
the parser's two scope-dispatch switches
(`TestBothDispatchSwitchesHandleEveryScope`) now have theirs. Prefer a structural
check when the rule itself is structural: a behavioural test only catches the
cases whose absence has an observable it happens to assert.

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

- `/add-formatting-setting` — the config hops a new `formatting` key crosses, the defaults
  that must not move, and how to verify one against the corpus
- `/add-parser-test` — patterns and pitfalls for adding tests to `internal/parser/cfparser_test.go`
- `/run-cfmleditor-lsp` — build, smoke-test, and drive the binary (CLI subcommands + LSP stdio)
- `/parser-internals` — scanner tokenisation, the two parse loops, call-site extraction, and how
  the `unresolved` command works

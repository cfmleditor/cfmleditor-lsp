# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # build binary to target/release/cfmleditor-lsp
make test           # go test ./...
make lint           # golangci-lint run ./...
make lint-fix       # golangci-lint run --fix ./...
make install        # build and install to $GOPATH/bin
make update-grammar # pull latest tree-sitter-cfml grammar, regen docs, clear build cache
make cfparse        # build debug CLI for parser development
make clean          # remove target/

# Run a single test
go test ./internal/parser/... -run TestParseFunctionDefs_MixedTagAndCFScript

# Explain how a specific call site's component was resolved (see "Debugging why a
# call site resolved" below) — much faster than manually tracing the parser/resolver
cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]
```

## Architecture

This is a Language Server Protocol (LSP) server for CFML/ColdFusion markup language, written in Go and backed by the `tree-sitter-cfml` grammar.

**Two runtime modes**, selected at startup:
- **Daemon mode** — triggered when a `.cfmleditor.json` config file exists. First LSP client starts a daemon that listens on a Unix socket and builds/owns the workspace index; subsequent clients attach to it. Daemon shuts down when all clients disconnect.
- **Standalone mode** — no config file; each LSP session is self-contained with its own index.

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
  → cached in server per URI
```

**Parser design** (`internal/parser`): The parser is intentionally not a full tree-sitter traversal — it uses a fast line-by-line `Scanner` for script files and string/regex searches for tag files. Function bodies are parsed lazily (only when completion or definition is requested). The entry point is `Parse(fileURI, content)` returning a `ParseResult`.

**Resolution chain** (`internal/resolve`): Converts CFML component dot-paths (e.g. `models.User`) to absolute `.cfc` file paths. Lookup order: baseDir → Application.cfc root → workspace folders → config `mappings`. Results are cached. Component resolvers in config teach the LSP about custom factory patterns (e.g. `getService("UserService")` → `packages.UserService.service`).

**Formatter** (`internal/formatter`): Walks the tree-sitter CST. All formatting rules (case, indentation, quotes, comma position, SQL keyword casing) come from config. Has a `whitespaceOnly` safety guard that rejects changes touching non-whitespace characters.

**Daemon/IPC** (`internal/daemon`): Uses Unix sockets. The config file (`.cfmleditor.json`) in the project root or one level up controls workspace paths, index globs, mappings, expression mappings, component/property resolvers, and formatting rules.

## Key structural notes

- `internal/parser/result.go` — `ParseResult` struct and `Parse()` entry point; also `ClassifyRegions()`
- `internal/parser/script_parser.go` — `scriptParser` and the `Scanner` line tokenizer. Scope-prefixed assignments (`variables.x =`, `this.x =`, `arguments.x =`, `request.x =`, `session.x =`, `application.x =`) each need their own `case` in the `handleBodyToken`/`parse()` dispatch switches routing to `parseBodyScopedVar`/`parseScopedVar` — that handler is the only one that correctly distinguishes `scope.name = rhs` (assignment) from `scope.name.method()` (bare call) for a two-token-prefixed LHS. Any scope keyword *not* listed falls through to `checkAssignRef`'s default path, which only recognizes a bare `x = ...` (single identifier directly followed by `=`); for a scope-prefixed LHS the next token is `.` not `=`, so the whole statement is silently misread as a bare-call check and any component type the RHS establishes is dropped. `url.`/`form.`/`cookie.`/`cgi.`/`client.`/`server.` deliberately aren't in this list (those scopes hold primitive request/config data in this codebase, not component instances) — add them the same way if a project's code assigns components through one of them.
- `internal/parser/tag_parser.go` — `tagParser` for `<cffunction>`, `<cfcomponent>`, etc.
- `internal/server/server.go` — LSP handler wiring; completion, definition, formatting, hover, symbols
- `internal/index/index.go` — concurrency-safe function definition store
- `internal/resolve/resolve.go` — dot-path → file resolution with caching
- `internal/config/config.go` — `.cfmleditor.json` schema and loader
- `internal/docs/` — built-in CFML function signatures (generated; do not edit manually)

The `docs/` package content is generated by `make update-grammar` / `make build` — regenerate rather than hand-editing it.

## Debugging why a call site resolved (or didn't) the way it did

`cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]` prints, for every call site on that line, the exact sequence of decisions `CanResolveCall` (`internal/resolve/resolve.go`) walked through: which mechanism set the receiver's component (function-scoped ref, file-level ref, Application.cfc ref, `<cfargument>` type, extends chain, a `componentResolver` match on the variable name, a `componentResolver` match on the full line text), which `FuncLookup`/componentResolver fallback fired for each hop of a chained call, and why the final method-exists check passed or failed. `--root <dir>` picks which `.cfmleditor.json` to load and which files to index — same semantics as `unresolved`'s directory argument — and defaults to the target file's own directory if omitted, which matters because a file's *own* nearest config can differ from the config a batch `unresolved` scan used (e.g. a file inside `tassreporting/` has its own `.cfmleditor.json`, distinct from `tassweb/.cfmleditor.json`'s resolvers/mappings).

**Reach for this before manually tracing through script_parser.go/tag_parser.go/result.go/resolve.go.** A component path that shows up in an unresolved-call error but doesn't match anything literal in `.cfmleditor.json` or on disk is almost always a `componentResolver` firing on a substring you didn't expect (see "Known resolver false-positive" below) — `explain` shows the exact resolver and match in one call instead of a multi-file manual trace. Example: `directcontent.cfc:104`'s `VARIABLES._content.createTemplate(...)` reported "not found in tassweb.packages.tass.directcontent" — a path absent from config entirely — because two lines earlier, `VARIABLES._content = VARIABLES._document.getDirectContent()` had its RHS matched by the generic catch-all resolver `{"match": "get$1()", "resolve": "tassweb.packages.tass.${1:lower}", "prefix": "get"}` (intended for `getPageTools()`-style factory methods): `indexFold` found the `"get"` prefix inside `"getDirectContent"`, matched the whole `getDirectContent()` call, and produced `tassweb.packages.tass.directcontent` — even though this call is a genuine iText/PdfWriter passthrough getter with nothing to do with that resolver's intended target. `explain` surfaces this as a single `resolved "..." to "..." via componentResolver matching the variable name` step instead of requiring a manual trace through four files.

## Component resolvers

Component resolvers (`componentResolvers` in `.cfmleditor.json`) teach the LSP how to map a call-site expression to a component dot-path. They are tried in order by `ResolveFromCall` / `ResolveFromCallFull` in `internal/parser/ast.go`.

**Component-type resolution order** (`CanResolveCall`, `internal/resolve/resolve.go`): for a qualified call `x.method()`, the receiver's component is looked up in this order, stopping at the first hit — (1) `call.Component`, if already set at parse time (e.g. a chained `new`/`createObject`, or a bare-call site where the tag/script parser resolved the receiver inline via `lookupComponentRef`); (2) a function-scoped `ComponentRef` for `x`; (3) a file-level (global/`VARIABLES.`/`this.`) `ComponentRef`; (4) a `ComponentRef` on `Application.cfc`/`Application.cfm`; (5) for `ARGUMENTS.x`, the `<cfargument type>` if it's a dotted path; (6) walking the `extends` chain's own `ComponentRef`s; (7) a `componentResolver` matched against the variable name text; (8) a `componentResolver` matched against the full line text (handles chains like `x.method().prop.func()`). If the call is itself chained (`call.Chain`), each hop then repeats a scaled-down version of this: the hop function's declared `ReturnComponent`/dotted `ReturnType`, falling back to a `componentResolver` matched against `hop()`. Because step (7)/(8) and the per-hop fallback both go through the same substring-prefix matching described below, a broad catch-all resolver (e.g. `get$1()`) can win at *any* of these steps, not just the ones that look like factory-method calls.

**How matching works** (`internal/parser/ast.go: matchResolverWithCache`):
- Each resolver has a `prefix` (fast-rejection substring) and a `match` pattern.
- Resolvers are tried in **array order** (`ResolveFromCallFull` iterates `resolvers` and returns on the first successful match) — so when two resolvers could both match the same expression (e.g. a specific `getDirectContent()` entry and a generic `get$1()` catch-all), whichever is listed **earlier** in `componentResolvers` wins, regardless of specificity. To make a specific case win over an existing broad catch-all, add the specific resolver *before* the catch-all in the config array, not after.
- `ResolveFromCall` finds the prefix inside the expression, takes the substring from that position, and tries to match the full pattern against it.
- If `match` contains no `\` escapes and no `$N` placeholders → **simple exact match** (or `match()` suffix).
- If `match` contains `$1` but no `\` → **simple prefix/suffix match**: the text before `$1` and after `$1` are checked as plain strings; the captured value replaces `$1` in `resolve`.
- If `match` contains `\` → **regex match**.

**Pipe-delimited `prefix` alternatives** (`internal/parser/ast.go: splitPrefix`, `findPrefixPos`):

`prefix` may list multiple alternatives separated by `|` (e.g. `"createModel|buildModel"`), letting one resolver's `match`/`resolve` pair cover call-site shapes that don't share a common leading substring — without this, each shape needs its own resolver entry even when `match`/`resolve` would otherwise be identical (a regex `match` with its own `|` alternation, or a `$1`-style pattern reused verbatim). `findPrefixPos` tries each alternative via `indexFold` and returns the position of whichever one is found; `BuildResolverSet`'s byte-bucket index registers the resolver under every alternative's first byte so the fast-rejection path still finds it. Which alternative matched has no effect on the subsequent `match`/`resolve` step — only the matched *position* is used to slice the expression, not the matched text — so this is purely a fast-lookup mechanism, not a change to match semantics. Compare to `${N:lower}`/`${N:upper}` above, which shares a resolver across names via a common prefix (`"get"`) rather than across genuinely different prefixes.

Gotcha when merging plain bare-word entries this way: `match`'s own regex-vs-simple decision (`isRegexPattern`, `internal/parser/ast.go`) is triggered *only* by a literal backslash appearing in `match` — it has nothing to do with `prefix` or with `|` in `match`. A merged pattern like `match: "kernel|_kernel"` has no backslash, so it is compared as one literal string (never equal to either bare word), not as alternation — silently matching nothing. Give merged bare-word alternatives a real anchor that needs escaping, e.g. `"^kernel(?:\\(\\))?$|^_kernel(?:\\(\\))?$"`, which both anchors correctly and supplies the backslash `isRegexPattern` needs.

**Case-folded placeholders in `resolve`** (`internal/parser/ast.go: substitutePlaceholder`):

Both the simple prefix/suffix match and regex match substitute captures into `resolve` through the same helper, which supports `${N:lower}` and `${N:upper}` in addition to plain `$N`. Use `${1:lower}` to fold a captured value: `{"match": "get$1()", "resolve": "packages.tass.${1:lower}", "prefix": "get"}` turns `getPageTools()`, `getLockBroker()`, etc. into one resolver instead of one entry per name. `${1:upper}` is the uppercase counterpart. This only helps when the target path is a mechanical case-fold of the captured text — if the target name isn't derived from the captured text at all (e.g. the `itextObj.Foo` family, where `Foo` maps to an unrelated Java class name), each name still needs its own resolver entry. Note: since path resolution (below) is itself fully case-insensitive, `${N:lower}`/`${N:upper}` are for explicitness/consistency in the config, not required for correctness.

**Case-insensitive path resolution** (`internal/path/path.go`): `match`/`prefix` matching (`indexFold`, `EqualFold`, `(?i)`-compiled regexes) has always been case-insensitive. The final step — turning a resolved dot-path into an actual `.cfc` file — is now also fully case-insensitive at every path segment, not just the filename: `ResolvePath` walks the path one directory at a time via `resolveSegments`, matching each segment against a real directory listing (exact case first, then `EqualFold`), and `mappings` keys are matched case-insensitively too (`lookupFold`). This matters because `os.Stat` alone can't be trusted for this: on a case-insensitive filesystem (APFS, NTFS defaults) `Stat` succeeds for any case variant of an existing name, silently returning the *requested* case rather than the real on-disk case — which breaks on a case-sensitive filesystem (ext4/most Linux) where the mismatch is never accepted at all. Only a directory listing reveals the true on-disk name on every platform.

**Known resolver false-positive: prefix substring matching**

`ResolveFromCall` finds the resolver's `prefix` *anywhere* in the expression via `indexFold`, not just at position 0. This means a resolver with prefix `"document"` will also fire when the variable name contains "document" as a substring (e.g. `domobject_document`). The sub passed to the pattern matcher starts at the prefix position, so it exactly matches the short pattern and produces a wrong component. There is no anchor mechanism in the current design. If a broad resolver causes false positives on unrelated variable names, the only workarounds are: rename the variable in the CFML source, or add a more-specific resolver for the false-positive variable that overrides with the correct component.

The same class of issue can appear *within* a single pipe-delimited `prefix`: `findPrefixPos` (see above) tries alternatives in the order written and returns the position of whichever is found first in that order — not the earliest position in the expression, and not the alternative that would actually let `match` succeed. If one alternative is a substring of another (e.g. `prefix: "File|getFile"` against `getFile()`), the shorter alternative found first fixes the slice position (`sub = "File()"`), and the longer alternative never gets a chance even though it would have matched. Order pipe-delimited alternatives so a shorter one that could be a substring of a longer one comes *after* it.

**Generic catch-all resolvers are the highest-risk case of this**, because they're deliberately broad. A pattern like `{"match": "get$1()", "resolve": "tassweb.packages.tass.${1:lower}", "prefix": "get"}` (see `${N:lower}` above) is meant to cover a family of factory methods (`getPageTools()`, `getLockBroker()`, ...) without one resolver entry per name — but `prefix: "get"` is found inside *any* identifier containing "get" anywhere, including ordinary getters that happen to be named `getXxx()` and have nothing to do with the intended factory family (e.g. a real `PdfWriter.getDirectContent()`/`itext.cfc getDirectContent()` passthrough matched this and silently produced a fabricated `tassweb.packages.tass.directcontent` component — see the `explain` example above). A catch-all this broad will win over a correct-but-absent answer (e.g. a generic `returntype="any"`/no-hint declared return that should fall through to `$any`) essentially every time a same-named `getXxx()` exists anywhere else in the workspace's naming conventions. There is no partial fix short of narrowing the `match` regex to the known name family (e.g. an explicit alternation of the real factory names) or accepting the occasional false positive and overriding it with a more specific resolver, same as the substring-prefix workaround above.

## Expression mappings

`expressionMappings` in `.cfmleditor.json` is a flat `map[string]string` of runtime expression → static value substitutions (e.g. `"#VARIABLES._core#": "packages.tass.core."`), applied to component-path strings before resolution. Unlike `componentResolvers`, there is no `match`/regex support — each key is matched with a plain `strings.Contains` and replaced with `strings.ReplaceAll`.

A key may list multiple alternatives separated by `|` (e.g. `"#ROOT#|#LEGACY_ROOT#": "app."`), so several distinct runtime expressions that should collapse to the same static value don't need separate map entries. Each pipe-delimited alternative is checked and replaced independently — this is plain substring alternation, not regex, so `|` here has no relation to the regex-triggering `\` in `componentResolvers.match`. Implemented in `internal/resolve/resolve.go: ComponentPath` and `internal/parser/result.go: replaceExpressions`.

**Unmapped `#...#` expressions become `$any`, not literal garbage.** Any component-path string captured from CFML source — `CreateObject("component", "...")`, `<cfinvoke component="...">`, `new "..."()`, etc. — may contain a runtime `#...#` interpolation that isn't covered by any `expressionMappings` entry (e.g. a genuinely dynamic factory path like `CreateObject("component", "tools.templates.#ARGUMENTS.template#.generator")`). `replaceExpressions` (`internal/parser/result.go`) runs its substitution pass unconditionally (even with zero `expressionMappings` configured), and if the result still contains `#` afterward, returns `"$any"` instead — so `CanResolveCall` accepts calls through it silently, the same as any other "genuinely dynamic, don't know" case, rather than surfacing the literal `"#ARGUMENTS.template#"` text as a nonsensical "not found in" component name. This fixup applies to `ComponentRef`s (`pr.ComponentRefs`/`funcRefsMap`) *and* to `CallSite.Component` (`pr.Calls`/`funcCallsMap`) — the latter matters because a bare unassigned call's `CallSite.Component` can be baked in directly at parse time (`tag_parser.go`'s `lookupComponentRef`), before this pass would otherwise run.

**Bracket-indexed chain segments** (`REQUEST['a' & b & 'c'].method()`, `arr[i].method()`): the script parser's chain-walking (`checkVarRHS`, `parseBodyVarDecl`, `parseBodyScopedVar`, `checkAssignRef`, `checkBareCall`, `recordBareCallAndChain` in `internal/parser/script_parser.go`) skips a `[...]` group via `skipBracketIndex()` (mirrors `skipParens()`) and, critically, **poisons** the chain text it's building with a literal `"[]"` marker (e.g. `REQUEST['a'&b&'c'].method()` → chain text `"REQUEST[].method"`) instead of silently dropping the bracket and treating the bare identifier before it as the receiver. The marker can't collide with any real identifier or resolver `match` (no CFML identifier or resolver pattern legitimately contains `[]`), so it reliably falls through to an honest `variable 'REQUEST[]' has no component ref` — the alternative (dropping the bracket) would let `REQUEST[dynamicKey].method()` silently resolve as if it were plain `REQUEST.method()`, which is wrong *and* confident if `REQUEST` bare happens to have an unrelated resolver/ref elsewhere. If a project wants these fully silent rather than merely honestly-categorized, add a `noFollow` componentResolver matching the marker, e.g. `{"match": "^REQUEST\\[\\]$", "resolve": "nocheck", "prefix": "REQUEST[]", "noFollow": true}` (same pattern as the `nocheck` example above) — the parser fix does not do this automatically, since suppressing vs. surfacing "genuinely dynamic" is a project-level judgment call.

**Known parser limitation: CFML comments containing CFScript**

The tag parser does not fully skip `<!--- ... --->` comment blocks that contain embedded CFScript or `<cfset>` tags. Variables declared in live code but only used inside a comment block will still generate lint errors for the commented-out lines. Affected examples: `tf`/`field` in `reporting/itext.cfc:createTextField`, `communityObj`/`familyObj` in `donor/persist.cfc:saveDonor`. These are false positives; the fix requires improving comment boundary detection in the tag parser.

**Two resolver shapes for `itextObj.X` struct members:**

The CFML iText code stores Java class handles as struct keys (`VARIABLES.itextObj.Foo = getJavaClass("Foo","itext")`). The parser cannot track struct-key component assignments, so two resolver shapes are needed depending on how the handle is used:

1. **Assignment RHS** — `var local = itextObj.Foo.init(...)`. Use `match: "itextObj.Foo.$1"`. The `$1` matches whatever follows the dot (method name + args); since `resolve` has no `$1` it is discarded. The parser's `resolveCall(rhs)` picks this up and creates a `ComponentRef` for the local variable.

2. **Direct call** — `itextObj.Foo.bar(...)` where the variable is `itextObj.Foo`. Use `match: "itextObj.Foo"` (exact, no `$1`). `CanResolveCall` passes the raw variable name to `ResolveFromCall`, which exact-matches it.

Some handles need both shapes; others only need one depending on how the code uses them.

**`noFollow` flag** (`config.Resolver.NoFollow`, `parser.Resolver.NoFollow`):

When a resolver has `"noFollow": true`, `CanResolveCall` accepts the call immediately without verifying that the method exists in the resolved component. Use this for:
- Dynamic factory methods where the resolved component is approximate.
- Java objects accessed via patterns where method coverage in stubs is incomplete.
- Any pattern where "this variable came from X" is enough and method checking is noise.

`CanResolveCall` checks `noFollow` at three points: the primary variable resolver, the full-line-text fallback resolver, and the altComp fallback resolver.

**Triaging "method not found" / "no component ref" lint errors:**

| Error form | Meaning | Fix |
|---|---|---|
| `variable 'x' has no component ref` | Parser never established what component `x` is | Add a `componentResolver` whose match covers the RHS of the assignment or the variable name |
| `method 'f' not found in pkg.path` | Component is known but the method is missing | Add the method to the stub CFC at that path, or add `"noFollow": true` to the resolver |
| `method 'f' not found in persist` | `persist` resolves correctly but the method is absent | The method is genuinely missing from the real CFC — implement it |
| Dynamic keys: `x[y].f`, `arr[i].f` | Type can't be tracked through runtime keys | Not fixable with resolvers; suppress with `noFollow` on the resolver that produces `x` |
| `ARGUMENTS.x` with `type="any"` | Argument has no type annotation | Add `type="pkg.path"` to `<cfargument>`, or add an `ARGUMENTS.x` exact-match resolver as a workaround |
| One component, many unrelated-looking missing methods (e.g. 50+ hits all "not found in studadmin") | **Known limitation, not (yet) fixable**: `CanResolveCall`'s global-ref fallback (`internal/resolve/resolve.go`) picks the *first* `ComponentRef` matching the variable name in file order, not the one nearest the call site — a scratch variable reassigned per `<cfcase>`/`<cfif>` branch (e.g. `service = server.kernel.GetStudAdmin()` in case A, `service = server.kernel.GetFinance()` in case B) gets every later `service.X()` call checked against case A's type, regardless of which case it's actually in. **Before triaging a large single-component cluster as N missing methods, check whether the receiver variable is reassigned more than once in the file** (`grep -n "varname\s*="`) — if so, most of the cluster is this bug, not real gaps. The genuinely-missing-method fix (row above) only applies once you've ruled this out. |
| A resolver match with `"resolve": "nocheck", "noFollow": true` doesn't suppress the check | The resolver's `match` includes `(...)` (a call expression, e.g. `foo.bar(...)`) rather than a bare variable name | `noFollow` only survives when the resolver re-runs live inside `CanResolveCall` (bare-word `ResolveFromCallFull(variable, ...)` lookups). A call-expression match fires once at parse time and gets baked into `ComponentRef.Component`, which has no `NoFollow` field — the flag is lost by check time. Use `"resolve": "$any"` instead (checked unconditionally, regardless of which code path produced it) for any resolver whose `match` contains parens. |

## Java stubs

Stubs live in `tassweb/packages/tass/javastubs/` mirroring the Java package path. They are plain CFCs containing empty function stubs so the LSP can verify method calls without running Java.

**Stub format** (single-line, matching existing files):
```cfml
// Java stub for com.example.ClassName
component { function init(...) {} function methodName(required type argName) {} }
```

**iText stub coverage** (`com.lowagie.text.*`): Anchor, Chunk, Document, Font, FontFactory, HeaderFooter, Image, Paragraph, Phrase, Rectangle; pdf sub-package: BaseFont, Barcode39, PdfContentByte, PdfPCell, PdfPRow, PdfPTable, PdfPageEventHelper, PdfReader, PdfStamper, PdfWriter.

**Existing `getJavaClass` resolver** maps `getJavaClass("pkg.ClassName","itext")` → `tassweb.packages.tass.javastubs.com.lowagie.text.pkg.ClassName`. The `itextObj.*` resolver entries (see above) are needed in addition because the struct-key assignments happen separately from the call-site expressions.

**`javaStubsPath` config option** (`internal/config/config.go: JSON.JavaStubsPath`, `config.JavaStubResolver`): set `"javaStubsPath": "tassweb.packages.tass.javastubs"` in `.cfmleditor.json` to auto-resolve any `createObject("java", "some.Class.Name")` call to `<javaStubsPath>.some.Class.Name`, without hand-writing the equivalent `componentResolver` regex. It's synthesized and appended alongside your own `componentResolvers` (`config.Resolve`, `daemon.Config.ComponentResolvers`, `cmd/cfmleditor-lsp/cliutil.go: loadResolversFromConfig` all wire it in). Only covers the `createObject("java", ...)` call site itself — it does not follow chained factory calls (e.g. `someJavaObj.getInstance(...)` returning another instance of the same Java type); those still need their own resolver entry or a stub method that models the return.

## Skills

- `/add-parser-test` — patterns and pitfalls for adding tests to `internal/parser/cfparser_test.go`
- `/run-cfmleditor-lsp` — build, smoke-test, and drive the binary (CLI subcommands + LSP stdio)
- `/parser-internals` — scanner tokenisation, the two parse loops, call-site extraction, and how the `unresolved` command works

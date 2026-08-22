# Changelog

## [Unreleased]

- Fix go-to-definition drifting further from its target the longer a file is edited. The index filed `&funcs[i]` — a pointer into the caller's slice, which for the server is the live `ParseResult` of an open document — so `ParseResult.ApplyEdit` and `Index.ShiftLines` were both moving the same structs. An in-function edit therefore shifted the index twice: once through the shared struct and again through the `ShiftLines` call in `didChange`, so a one-line insertion moved every function below it by two, compounding with each keystroke. The index copies entries on the way in now, which also makes it their only writer.
- Fix the data races in the state a daemon's sessions share. A `ParseResult` is mutated in place, and even its read accessors mutate, because each memoises lazily; the server handed the same pointer to the LSP read goroutine and to its own background timers, so a burst of typing armed the deferred reindex and 200ms later it called `ApplyFullReplace` on exactly the `ParseResult` the next keystroke was reading. A per-document lock now serialises every path that touches one. The same pass fixes an unsynchronised read of `changeCount` against the timer that deletes it, an unguarded rebuild of the shared resolver — where a rebuild racing an invalidation could hand back the nil it had just stored — and in-place compaction in the index that overwrote slices already returned to a caller. Reproduced under `go test -race` first; three regression tests fail with their fix reverted.
- Fix the deferred reindex after a large paste being silently dropped. It shared one timer slot per document with the completion-cache rebuild, so the first ordinary keystroke after a burst armed a rebuild that stopped the pending reindex and discarded it — the pasted text never reached the index at all. The two kinds of pending work have their own maps now.
- Fix `expressionMappings` and `servicePropertyResolvers` being ignored in every editor window but the first. Both are parsed and merged, and then dropped: `config.Resolve` had no field for the first, and `applyConfig` copied neither. Daemon mode configured its stdio session separately, in `main.go`, and that path did set them — so with a `.cfmleditor.json` mapping `#VARIABLES._core#`, the first editor resolved `#VARIABLES._core#Widget` to the real `.cfc` and every later one resolved it to nothing. Nothing reported it, because an unsubstituted expression is indistinguishable from a genuinely dynamic one, which `CanResolveCall` accepts. The two session kinds now share one `server.Settings` value, which drops `daemon.Serve` from twelve parameters to six — a list where an omission was invisible is what caused this.
- Fix line wrapping breaking inside a quoted attribute value. `writeWrapped` is handed whole elements verbatim, attributes included, and broke at the last space before `lineWidth` wherever it fell, so `<img src="x.png" alt="a fairly long alternative text…">` came back with newlines and indentation inside the `alt=`. The `whitespaceOnly` guard cannot catch it — only whitespace changed — but the attribute's value did change, and for a CFML tag whose attribute carries a string the runtime uses, a `cfhttpparam` value or a `cfmail` subject, the injected whitespace is in the data. Break points are computed once over the whole string now, skipping any space inside a tag's quoted value; quotes count only inside a tag, since in ordinary prose an apostrophe is a letter and treating it as a delimiter switched wrapping off from `won't` onward. 43 of the 5,504 formattable corpus files were affected, and none are now.
- Fix `componentResolvers` being tried in a different order by different features. `ResolveFromCallMatch`, behind `CanResolveCall`, walks the configured slice in order; `ResolverSet.Resolve`, behind completion and hover, collected candidates by walking the *call site's* bytes, so resolvers were tried in order of where each `prefix`'s first byte happened to appear in the text. Where two prefixes share a first byte the bucket was already in array order, which is why the documented `getDirectContent()`-before-`get$1()` case kept working; where they differ, the same expression resolved to two different components depending on which feature asked, and the documented way to override a broad catch-all worked in one half of the LSP only. Sorting the candidates back into configuration order also collapses the duplicates a pipe-delimited `prefix` contributed, which makes the path about twice as fast rather than slower.
- Finish characterising the grammar refusals in `FORMATTER-ISSUES.md`: thirteen more `tree-sitter-cfml` gaps reduced to minimal repros and re-verified standalone against a control that parses (subscripted `::` static access, `${ }` ordered-struct literals, `exit "x";`, `savecontent` as an expression, a `new` expression as a tag-in-script attribute value, colon-separated and mixed-separator tag-call attribute lists, brace-less `try`, a dot-notation numeric struct key on assignment — which parses when read — the `call():function(){}` listener form, a return type written before the access modifiers, `pageencoding` before a component, and `name: value;` colon assignment). About a third of the candidates turned out to parse on their own and are recorded so they are not re-filed. Also corrects two earlier claims: the ColdBox refusals were recorded as malformed source for omitting a comma between parameters, but that form parses in CFML and the gap is already filed, and the reducer handles all 83 refusals rather than 81.
- Fix tag-based functions being indexed under the wrong name. `getAttr` looked for an attribute name as a plain substring and only checked the text *after* the match, so a hit inside a longer attribute counted: `name` matched the tail of `displayname=`, and `<cffunction displayname="Donor Lookup" name="getDonor">` was indexed as a function called `Donor Lookup` — with its `<cfargument displayname="Donor Id" name="donorId">` indexed as `Donor Id`. Goto-definition, completion, hover and the `unresolved` scan were all wrong for any tag-based CFC that writes `displayname` before `name`, and `returntype=` shadowed `type=` the same way. A name match must now start a word.
- Fix `</cffunction>` never exiting function scope. The check was dead code twice over — `len(tag) > 13` is false for the exactly-13-byte tag, and `tag[2:13]` is `"cffunction>"`, which never equals `"cffunction"` — so `inFunc` was never cleared and everything after the first function in a tag-based component was still being attributed to it.
- Fix workspace-symbol search silently returning nothing for any query containing a character outside `a-z`. `containsFoldStr` folded only the haystack, with `|0x20`, and compared against a raw needle byte, so `_` (0x5F, folding to 0x7F) could never match itself — `get_user` found nothing. Both sides now go through an explicit A-Z fold, which also avoids the `|0x20` collisions that would make `@` match a backtick.
- Stop the formatter collapsing the body of `<pre>` and `<textarea>`. They went through the generic element path and had their content reflowed onto one line, which destroys the rendered output — and the `whitespaceOnly` guard passed it, correctly by its own definition, because nothing but whitespace changed. In these two elements the whitespace *is* the content, so no guard change can catch it; the elements are reproduced from source instead. A `<div>` is still reflowed, which a test pins in the other direction.
- Fix `appendTrailingComma` duplicating the comma and eating the newline. `bytes.Buffer.Bytes()` returns the buffer's own backing array, so `Reset()` followed by writing that slice back had `WriteByte(',')` clobber the very byte the next `Write` re-read: `"SELECT a\n"` came out as `"SELECT a,,"`. In a `<cfquery>` that corrupted the emitted SQL. The tail is copied out before the buffer is truncated now.
- Fix three defects in the reducer, found reviewing it after it landed. Its "same failure" invariant read only ERROR nodes, but tree-sitter reports a missing token as a MISSING node instead, so those failures produced an empty signature that compared equal to a fragment which parses cleanly — the invariant collapsed back into "has no ERROR node" and the reduction trimmed away the failing line. Signatures now cover both node kinds and no failing fragment can share the empty signature with a clean one. Reduced fragments also keep their newlines as escapes rather than being whitespace-collapsed, which had let a `//` comment swallow the rest of a repro, and line ranges for embedded regions are translated to file lines instead of being printed region-relative next to a file path. With those fixed the reducer handles all 83 refusals rather than 81.
- Fix `make shrink` reporting success on failure. It piped `go test` into `grep`, so the pipeline exited with grep's status and a build error or failing test came back as exit 0 with no output.

- Check the reducer in as `make shrink REPORT=<corpus report>` (`internal/formatter/shrink_test.go`). It takes a report from `make corpus` and cuts every parse-refused and script-refused file down to the smallest contiguous fragment that still fails the same way, turning "this file does not parse" into a construct that can be filed against `tree-sitter-cfml`. Two invariants keep it honest: the fragment must be a contiguous slice, because deleting interior lines invents syntax that is not in the file, and it must reproduce the *same* first ERROR node, because nearly any fragment of CFML fails to parse and reducing against "still errors" converges on a lone `}` or a stray `</cfoutput>`. It reduces all 83 refusals, 17 of them under 150 characters.
- Three more confirmed grammar gaps found with it: the tag form of `throw` in script (`throw message="x" type="y";`, where the `throw(...)` call form parses), `name:value` function annotations generally rather than just `access:` (`secured:api` fails the same way), and component-level constructs such as `static { }` inside a `<cfscript>` that sits in a *tag-based* `<cfcomponent>`, where the region has no `component { }` wrapper to make them legal.
- Skip binary files in the corpus harness. Lucee ships three GIFs named `*.gif.cfm`; parsing those produced refusals indistinguishable from grammar gaps in the report. They are counted in their own column now, which moves parse-refused from 25 to 22.

- Stop the `whitespaceOnly` guard excusing a quote the formatter dropped. The re-quoting allowance was written as "any mismatched `"` or `'` on either side", so with the default settings the guard could not see the formatter stripping the quotes off a CFML string or a SQL literal — `<cfset msg = "hello world">` coming back as `<cfset msg = hello world>` passed as whitespace-only. `normaliseAttrValue` only ever *adds* quotes to an unquoted value or *upgrades* single to double, so the allowance is now limited to those two shapes and a dropped quote is compared like any other byte. Re-running the six-project corpus moved no file between categories, confirming nothing relied on the old exception. The allowance also moved off `selfCloseTags` — which has nothing to do with quoting and was disabling the check for whole files — onto `doubleQuoteAttributes`, the option that performs the re-quoting.
- Record in `FORMATTER-ISSUES.md` what the grammar refusals actually are: eight confirmed `tree-sitter-cfml` cfscript gaps reduced to minimal repros and re-verified standalone (`access:remote` annotations, `final` members in a `static` block, `component( javasettings = {} )`, `default` interface methods, bare `param x.y;`, body-less tag-in-script, inline `java {}` classes, arrow functions with statement bodies), plus a document-grammar gap where `final component` in a `.cfc` degrades to plain text instead of an `ERROR` node — so the body is emitted unformatted, the guard passes it, and the corpus scores the file clean. Also notes which of the remaining files are Lucee's own deliberately-invalid fixtures rather than gaps.

- Force the pinned toolchain for `make vuln` too. `go run golang.org/x/vuln/...@v1.7.0` resolves the toolchain the *scanner* module asks for rather than the one this module targets — "requires go >= 1.25.0; switching to go1.25.13" — and a govulncheck built with 1.25 cannot load a 1.26 module's packages: it reports every package as "requires newer Go version" and scans nothing, while still exiting non-zero as though it had found something. CI never saw it, because `setup-go` installs the version from `go.mod` as the local toolchain and no switch happens; anyone whose own Go is older than the target hits it on the release gate. Same one-line fix as the linter got, reusing the `GO_VERSION` the linter already reads out of `go.mod`.

## [0.2.6]

- Publish `cfmleditor-lsp-windows-amd64.tar.gz` alongside the existing `.zip`. Editor extensions derive the asset name from the platform alone and ask every platform for the same extension — the Zed extension requests the tarball and failed to install with "no asset found matching cfmleditor-lsp-windows-amd64.tar.gz" (#2). Both archives are published now.
- Add `"anchored": true` to `componentResolvers`. A resolver's `prefix` is otherwise found *anywhere* in the call expression and matching starts from there, so a resolver with prefix `document` also claims the unrelated variable `domobject_document`, and a deliberately broad catch-all like `{"prefix": "get", "match": "get$1()"}` also claims the `getDirectContent()` at the end of `VARIABLES._document.getDirectContent()`. In both cases the shortened expression matches the pattern exactly, so a wrong component is produced confidently rather than the resolver declining. `anchored` requires the prefix at position 0, which leaves the bare factory calls such a catch-all was written for working while the substring matches stop; it also makes the order of pipe-delimited `prefix` alternatives irrelevant, since every alternative that matches then matches at the same position. Unanchored remains the default, because a resolver aimed at a call normally does want to match through a receiver.
- `explain` now names the `componentResolvers` entry that claimed an expression (`match "...", prefix "..."`), instead of reporting only that "a componentResolver" matched — the resolved component alone cannot distinguish a correct match from a substring false positive.
- Fix `refs`, `deps` and `parse` dropping `noFollow` when loading resolvers from `.cfmleditor.json`; those commands built their resolver list without copying the field.
- Check the formatter corpus harness in as `TestFormatterCorpus` (`make corpus CORPUS=<dir>[:<dir>...]`). Every defect in `FORMATTER-ISSUES.md` was found by formatting thousands of files from real projects, but the scanner that did it was a scratch script that was never committed, so those numbers could not be reproduced or moved without rebuilding it from the prose first. It is skipped unless `CFML_CORPUS` names a corpus, so `make test` and CI are unaffected, and it reports the grammar's failures in two buckets — documents the CFML grammar cannot parse, and documents whose embedded cfscript or cfquery the sub-grammar cannot — because the second is invisible from outside: the document parses, the formatter runs, and whatever it renders for that region is a guess.
- Fix the formatter destroying an XML declaration: `<?xml version="1.0" encoding="utf-8"?>` came back as `<?xmlversion="1.0"encoding="utf-8"?>`. Its parts are child nodes and the generic child walk joined them with no separator. Only whitespace was removed, so the `whitespaceOnly` guard passed it and the `format` CLI wrote it to disk and exited 0 — leaving a file the grammar can no longer parse.
- Fix `new component { ... }` — an anonymous component defined at the point of use — being emitted as `new ()`. The `new_expression` has neither a constructor nor an arguments node, since the class *is* its body, and rendering it from those two fields dropped the keyword and every property and method. 18 files in the corpus.
- Fix commas being inserted into a CF tag written in script syntax. `cfdirectory(directory="#dir#" action="create")` separates its attributes with spaces, but the grammar hands the list over as an `arguments` node of assignment expressions — the same shape as a call's arguments — so the formatter joined them with `", "`. 11 files.
- Across the six-project, 5,620-file corpus this takes files that format cleanly from 5,470 to 5,499 and guard rejections from 61 to 32.
- Pin golangci-lint and build it from source in `make lint`, `make lint-fix` and `make fmt`, the way `make vuln` already handles govulncheck. Any `golangci-lint` on `PATH` that was built with an older Go than `go.mod` targets refuses to start at all — `can't load config: the Go language version (go1.25) used to build golangci-lint is lower than the targeted Go version (1.26.6)` — which is what a distro or Homebrew binary does for weeks after each Go bump, and it reads as a broken repo rather than a stale tool. Building the pinned version under this module's own toolchain makes the mismatch impossible; `GOTOOLCHAIN` has to be forced, since resolving a `pkg@version` otherwise picks the toolchain the *tool* module asks for.
- Read the pinned linter's Go version out of `go.mod` rather than from `go list -m`, which is workspace-aware: inside a `go.work` it prints one line per module, `$(shell)` folds them into one value, and `make lint` died with `/bin/sh: 1.23: command not found` — `1.23` being the grammar repo's own `go` directive. A CI checkout has no `go.work`, so this only ever broke a developer machine with `tree-sitter-cfml` checked out alongside, which is the normal setup here.
- CI's lint job runs `make lint` instead of `golangci-lint-action`, so CI, the release gate and a local run share one pinned version and cannot report different findings on the same code. Findings appear in the job log rather than as inline PR annotations.
- Bump the `vuln` job's `actions/checkout` v4 → v7 and `actions/setup-go` v5 → v7. The 0.2.4 action bump covered every other job in both workflows and missed this one.

## [0.2.5]

- Fix the formatter changing non-whitespace content. Audited by formatting 5620 CFML files from Lucee, ContentBox, ColdBox, FW/1, TestBox and cfmleditor: 1671 were rejected by the `whitespaceOnly` guard, so with the default settings the LSP silently declined to format them and format-on-save did nothing. 5450 now format cleanly and 84 are still rejected. Each of the following was a separate defect, and most of them destroyed or altered code rather than merely tripping the guard:
  - `query` and `function` return types were dropped outright: `public query function GetData()` came back as `public function GetData()`. The signature prefix was gated on `IsNamed()`, but the grammar tokenises those two type names as anonymous keyword nodes. The other fourteen return types, and dotted component paths, were unaffected.
  - A `try` with more than one `catch` kept only the first and deleted the rest *along with their bodies*, because every catch clause carries the same `handler` field name. The exception type was dropped too — `catch (java.lang.Exception e)` became `catch (e)`, silently widening what the handler catches.
  - `interface {}` was rewritten as `component {}`, changing what the file declares, and the `abstract` and `final` modifiers were dropped. The component header was hardcoded to `"component"`.
  - Attributes written after the parameter list were hoisted in front of the `function` keyword, so `function setup() localmode="true" {}` became `localmode="true" function setup() {}` — output that does not compile. Seen with `localmode`, `skip`, `restpath`, `httpmethod`, `output` and `hint`.
  - A `//` comment inside an array or struct literal, or among a call's arguments, was treated as an element and joined inline with `", "`, so it commented out everything after it and destroyed the statement. Comments were also deleted outright between a block and its `else`/`catch`/`finally`, and between a chained call and its next `.hop()`.
  - Lucee and BoxLang static access `Widget::getData()` was emitted as the instance call `Widget.getData()`.
  - A leading UTF-8 BOM was stripped from every file that had one (554 in the corpus). It sits outside every CST node, so the walk never emitted it.
  - CF tags that are legal without a body — `cfmodule`, `cfhttp`, `cfinvoke`, `cffeed`, `cfadmin` — were given a closing tag they never had, which re-parented every following sibling into the tag's body.
- Refuse to format a document the grammar cannot parse, instead of emitting corrupt output. A body-less `<cfinvoke>` or `<cfhttp>` inside `<cfcomponent>` produces an `ERROR` node, which the node walk had no rendering for; it fell through to a raw emit that ran the tag name and every attribute together (`<cfinvokecomponent="..."method="..."`), dropped `</cfcomponent>` and appended a bogus `</cf>` — and returned a `nil` error while doing it.
- `cfmleditor-lsp format` no longer corrupts files. It built `DefaultOptions()`, which leaves `whitespaceOnly` off, and never checked the tree for parse errors, so it wrote that corrupt output over the source and exited 0 — 110 bytes vanished from a test fixture, leaving it unparseable. The guard is now on by default, matching the LSP, with `--allow-non-whitespace` to opt out; a file is only rewritten after formatting succeeds, and a batch run reports every failing file rather than exiting on the first.
- Let the `whitespaceOnly` guard accept the canonicalisation the formatter performs on purpose. Adding braces around a single-statement body and a semicolon to a statement written without one are both non-whitespace changes, so the guard rejected them and the formatter was in conflict with its own default — 527 real-world files, 9.4% of the corpus. The guard now skips an inserted `;`, `{` or `}` on the output side, in the same spirit as the existing self-closing-slash and quote allowances. The allowance is one-directional, so a token the formatter *dropped* still fails, and added braces must balance.
- Fix formatting not being a fixed point again, in two more places: a single-statement body gained braces emitted tight, while a real block is padded with blank lines, so the second format of the same code took the padded path; and `preformat` replaced a converted element whole, so it could not descend into its body and any void element nested there survived until a later run. Non-idempotent files dropped from 390 to 36 across the corpus.
- Fix the release workflow, which had never produced a release — 23 runs, every one failed or cancelled. `darwin` was cross-compiled from Linux with zig, but CGO there links `libresolv` and the CoreFoundation and Security frameworks, and zig ships macOS libc headers without the Apple SDK that provides them, so the link failed every time. `darwin` now builds on `macos-latest`, where CGO is native, and the amd64 slice cross-compiles on the arm64 runner with an `-arch` flag. The matrix also had `fail-fast` at its default, so that one failure cancelled every other platform mid-build — `linux/amd64` had already built and packaged successfully in the last run and still uploaded nothing.
- Other release workflow changes: zig is gone, with `linux/arm64` and `windows/amd64` cross-compiling under the stock Ubuntu toolchains (which also drops `goto-bus-stop/setup-zig` and its Node 20 deprecation warning); the CFML docs are fetched and generated once in their own job and shared as an artifact, rather than refetched on all five runners; a single tag-gated job publishes every asset instead of five racing to create the same release, and it verifies all five are present first so a partial upload cannot ship a release missing a platform; each build asserts its binary really is for the target `GOARCH`; tarballs are built with `COPYFILE_DISABLE=1` so macOS `bsdtar` does not store AppleDouble `._` entries beside the binary; permissions are `contents: read` except on the publish job; a hyphenated tag such as `v1.2.3-rc1` publishes as a prerelease and never becomes "Latest"; and `workflow_dispatch` allows the whole build to be rehearsed without cutting a tag.

## [0.2.4]

- Bump the GitHub Actions in both workflows to their current majors: `actions/checkout` v4 → v7, `actions/setup-go` v5 → v7, `golangci/golangci-lint-action` v8 → v9, `softprops/action-gh-release` v2 → v3. All four majors are Node 20 → Node 24 runtime moves; v9 of the lint action still installs golangci-lint v2, which this repo's v2 config needs.
- Bump the CFLint fallback version to 1.5.14. It only applies when the releases API cannot be reached — the normal path already downloads whatever is current — but an offline first run no longer starts four releases behind.
- `tree-sitter-cfml` grammar updates (v0.26.31 → v0.26.33). The new release makes five CFML constructs parse that previously produced an `ERROR` node, three of which the formatter then rewrote as soon as it could see them — with `whitespaceOnly` on (the default), that meant the LSP refused to format any file containing one:
  - Lucee's thin-arrow lambda `t -> t.b()` was rewritten to the fat arrow `t => t.b()`. The two are not interchangeable: `->` is a lambda and `=>` a closure, and they differ in what they capture. The source's own arrow is now preserved.
  - `new java:java.io.File(p)` and `new cfml:models.Base()` lost the type prefix entirely, becoming `new java.io.File(p)` — a different object. The prefix is now reproduced.
  - An array type suffix gained a space: `function f( string[] v )` became `string [] v`, and `User[] function getUsers()` became `User []`. It now stays attached to the type.
- Resolve `new java:...` and `new cfml:...` to a component. The script parser read the type prefix as the path itself, producing a `ComponentRef` for a component literally named `java` or `cfml`, against which every later method call reported "not found". `new java:x.y.Z()` now resolves through the same `javaStubsPath` machinery as `createObject("java", "x.y.Z")`, and `new cfml:a.b.C()` resolves to `a.b.C`. With no `javaStubsPath` configured the java form records no ref at all, which is the honest answer, rather than a wrong one.
- Format `throw()`'s named arguments like every other call's. The grammar gives `throw(type = "x")` the same `arguments` node as a call expression, but the formatter emitted that list verbatim, so `throw (type="x")` sat beside `writeLog(text = "x")` in the same file. Both now render identically. The expression form, `throw new Exception("x")`, is unchanged.
- Add CI on pull requests (`.github/workflows/ci.yml`): build, vet, gofmt, and the test suite, plus a race-detector job. Previously the only workflow ran on release tags, so pull requests had no automated checks at all.
- Skip the two wall-clock threshold tests under `-short` (`TestParsePerformance`, `TestResolverRegexNotRecompiledPerCall`). They assert microsecond budgets that a shared CI runner cannot meet; run the suite without `-short` to measure.
- Add an informational `perf` CI job that runs those threshold tests and the parser benchmarks, publishing the numbers to the run summary. It never gates a merge, so the timing coverage `-short` gives up is still visible — read it by comparing runs, not by pass/fail.
- Add `make vuln` (govulncheck), gating `make release` and running as a CI job. The scanner is pinned so a scanner release cannot fail an unrelated build; the advisory database is still fetched live.
- Bump Go toolchain to 1.26.6

- `tree-sitter-cfml` grammar updates (v0.26.30 → v0.26.31). This changes formatter output for `throw()` called with named arguments: it previously received tag-attribute spacing (`throw ( type = "x" )`) and now stays close to the source (`throw (type="x")`).
- Fix formatting not being idempotent: formatting an already-formatted file could change it again, so format-on-save produced a diff for an unchanged file. Two causes — the blank line between cfscript statements was decided from the source span rather than the emitted output, so it only appeared after `function f() {}` had already been expanded into a block; and an unclosed CF tag emitted its synthetic implicit end-tag marker as content, adding a stray blank line that disappeared once the closing tag existed.

## [0.2.3]

- Bump Go toolchain to 1.26.5
- `tree-sitter-cfml` grammar updates (v0.26.29 → v0.26.30)
- Build the generated CFML docs from both sources again — `make docs` now fetches cfdocs *and* Lucee into separate staging directories and assembles `docs/data` from them, so regenerating no longer drops the Lucee-only entries. Release builds fetched cfdocs alone, so released binaries were missing those entries.
- Accept configuration from the editor as LSP `initializationOptions`, using the same shape as `.cfmleditor.json`, so settings like `linting.enabled` no longer require a file in the project. A discovered `.cfmleditor.json` still wins on every key it sets; editor settings fill in the rest. Read once at `initialize`; `debug` is not supported this way, since the logger is built before the client connects.
- Fix standalone mode not finding `.cfmleditor.json` outside the workspace root itself. It now searches upwards from each workspace folder to the filesystem root, matching daemon mode, so opening a subdirectory of a project no longer silently drops mappings, resolvers, and linting. A malformed config no longer masks a valid one further up.
- Fix linting intermittently never starting in standalone mode. `initialize` spawned `initLinter` before loading `.cfmleditor.json`, so the goroutine could read `linting.enabled` as `false`, leave the linter uninitialised, and silently skip diagnostics for the rest of the session. Workspace config is now applied before those goroutines start, which also stops the initial workspace index from running before `mappings`, `componentResolvers`, and `beanPaths` are applied.

## [0.2.2]

- Fix cflint scan panic 
- Fix cflint issues with non [a-z0-9] characters in file names

## [0.2.1]

- Upgrade `go.lsp.dev/jsonrpc2` and `go.lsp.dev/protocol` to v1.0.1 (direct-return handler dispatch, `Optional`/`Nullable`/union-typed protocol fields)
- Switch LSP param marshaling to `github.com/go-json-experiment/json` for correct encoding of the new protocol types
- Bump Go toolchain to 1.26.4

## [0.2.00]

- Major refactor and consolidation of Parsing / Scanning
- Many testing bug fixes ( from daily use )
- `tree-sitter-cfml` grammar updates

## [0.1.22]

- Improvements to exception handling
- `tree-sitter-cfml` grammar updates
- Improvements to grammar detection for scanner
- Skip binary files during indexing and CLI scans

## [0.1.21]

- Config refactor
- Completion ordering refactor
- Fix completions and help for scoped functions
- Allow for function snippets for completion

## [0.1.20]

- Extra fixes and tests to avoid LSP daemon panics

## [0.1.19]

- Fix panic crashing the server
- Fix `.cfc` path check

## [0.1.18]

- Prevent processing of non CFML files, only process `.cfc`, `.cfm`, `.cfml`, and `.cfs` files

## [0.1.15]

- Fix zip in github workflow, use 7z instead

## [0.1.14]

- Fix CGO compile ( for mac ) for tree-sitter-cfml grammar

## [0.1.12]

- Fix CGO compile for tree-sitter-cfml grammar

## [0.1.11]

### Added

- `queryFormat` config option to control whether `<cfquery>` content is formatted (default `false`).
- `queryCommaPosition` config option with `"preserve"` (default), `"before"`, and `"after"` modes.
- Preserve mode for SQL comma placement — keeps commas in their original position by default.
- Adjacency-aware formatting — nodes without whitespace in source (e.g. `table#var#_suffix`, `<cfif>name_a<cfelse>name_b</cfif>`) stay glued together.
- Goto-definition support for component resolvers, includes, and dot-path references.
- Release tooling (`make release <version>`, `make release-dry <version>`).

### Changed

- Renamed SQL formatter options to use `query` prefix: `queryFormat`, `queryUppercaseKeywords`, `queryCommaPosition`.
- `queryCommaPosition` defaults to `"preserve"` independent of `commaPosition`.
- `queryFormat` defaults to `false` (query content emitted verbatim unless opted in).
- `make build` now runs `update-grammar` (regenerates tree-sitter grammar + clears Go cache).
- Release workflow embeds version from git tag into binary.

### Fixed

- Stale Go build cache not picking up tree-sitter grammar changes (added `go clean -cache` to `update-grammar`).
- Commas inside `<cfif>` blocks no longer incorrectly repositioned in trailing-comma mode.

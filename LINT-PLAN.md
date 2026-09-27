# Lint plan: what is enabled, what is next, and what stays off

The lint set is chosen for what it catches, measured against this codebase
rather than taken from a list. This file records the stages still to come,
the measurements behind them, and the rules deliberately left off, so the
reasons are on record rather than remembered.

Every count below was measured with the pinned golangci-lint (v2.13.2, which
carries gocritic v0.14.4, revive v1.15.0 and staticcheck) over the whole
repository, tests included, while #157 was open. Counts marked "since fixed"
dropped when #157's own fixes went in.

## How each stage is done

One pull request per stage, and the same five steps every time:

1. **Measure.** Run the candidate rules over everything, uncapped
   (`--max-issues-per-linter=0 --max-same-issues=0`), and count per rule.
2. **Enable** the ones worth keeping in `.golangci.yml`, with a comment where
   a setting or an exclusion needs its reason.
3. **Fix** what they find. A mechanical fix is still read: a value parameter
   turned into a pointer, or a loop turned into indexing, changes behaviour
   wherever the code edited its copy. See CLAUDE.md's lint conventions.
4. **Verify**: `go test -short -race ./...`, `make lint`, and, where code on
   the parse or request path changed, the benchmarks against `main`, run
   alternately.
5. **Record** the rule and its reason in CLAUDE.md, and update this file.

## Done

- **#157** — gocritic's performance checks (copy thresholds at 80 bytes),
  `whyNoLint`, `emptyStringTest`; 27 golangci-lint linters; `staticcheck`
  with `checks: ["all"]`. See its description for what each found.

## Stage 1 — free bug-catchers (done)

46 rules that found nothing, or next to nothing, and guard new code:

- **gocritic (20):** `badLock`, `badSyncOnceFunc`, `syncMapLoadAndDelete`,
  `exposedSyncMutex`, `badSorting`, `sortSlice`, `stringsCompare`,
  `truncateCmp`, `returnAfterHttpError`, `sqlQuery`, `httpNoBody`,
  `externalErrorReassign`, `uncheckedInlineErr`, `nilValReturn`, `evalOrder`,
  `dynamicFmtString`, `badRegexp`, `regexpPattern`, `deferUnlambda`,
  `sloppyReassign`.
- **govet (7):** `nilness`, `unusedwrite`, `deepequalerrors`,
  `reflectvaluecompare`, `sortslice`, `atomicalign`, `httpmux`.
- **revive (19):** `atomic`, `constant-logical-expr`, `forbidden-call-in-wg-go`,
  `identical-branches`, `identical-ifelseif-branches`,
  `identical-ifelseif-conditions`, `identical-switch-conditions`,
  `inefficient-map-lookup`, `modifies-parameter`, `modifies-value-receiver`,
  `range-val-address`, `range-val-in-closure`, `string-of-int`, `struct-tag`,
  `time-equal`, `unconditional-recursion`, `unnecessary-if`, `useless-break`,
  `waitgroup-by-value`.

Only `identical-ifelseif-branches` found anything: two `if … else if` chains
whose branches did the same thing, now one condition joined with `||`. Naming
revive rules replaces its default set, so the 23 defaults are listed in the
config too. A throwaway file breaking one rule from each tool confirmed the
new checks run, and that the defaults still do.

## Stage 2 — small fixes, mostly in tests (done)

11 rules; 96 findings measured, 36 fixed once the two settings below were in:

- `forcetypeassert` and revive's `unchecked-type-assertion`: an unchecked
  type assertion panics rather than failing with a message. 9 fixed, 3 of
  them in `server_test.go`, where one would have moved the panic to a nil
  dereference on the next line and got a real check instead. The revive rule
  runs with `acceptIgnoredAssertionResult`: the 26 `v, _ := x.(T)` it also
  reported cannot panic, since a wrong type leaves the zero value and the
  check after it fails, and the `_` is a decision rather than an omission.
- `thelper`: 1 fixed (`varCorpus`'s `testing.TB` is named `tb`). Its
  benchmark checks are off: the other 15 findings were `func(b *testing.B)`
  bodies handed to `testing.Benchmark` by scaling tests, which are not
  helpers.
- gocritic `deferInLoop` (3, now `t.Cleanup` or a close in the loop),
  `filepathJoin` (6), `importShadow` and revive `import-shadowing` (7:
  `path` and `refs` used as variable names), revive `confusing-results` (5,
  named), `redundant-import-alias` (1).
- revive `bool-literal-in-expr` (4): each was `m["k"] != true` on a `map[string]any`,
  where the literal cannot simply be dropped; rewritten as a checked `bool`
  assertion, which says what the test means.
- `unqueryvet` (3): all `SELECT *` in CFML fixtures the formatter tests feed
  in, so it is excluded in test files and on everywhere else. Production code
  has no finding.

A throwaway file breaking each rule confirmed they all run. Run it with
`--uniq-by-line=false`: several of these land on the same line as another
finding, and by default only the first is shown.

## Stage 3 — memory layout, by hand (done)

`govet`'s `fieldalignment` reports 83 structs in production code. Most only
move pointer fields to the front, which shortens what the garbage collector
scans and saves no memory. The linter is not enabled: it cannot be told to
report only the structs that would shrink, and satisfying the rest scatters
fields that are grouped for readability.

A smaller struct saves nothing unless it lands in a smaller allocator size
class, or is stored by value in a slice. Of the parser's structs:

| Struct | Before | After | Why it counts |
|---|---:|---:|---|
| `parser.CallSite` | 120 B | 112 B | held by value in every call slice |
| `scriptParser` | 392 B | 368 B | size class 416 → 384; one per parse and per sub-parse |
| `parser.Scanner` | 144 B | 128 B | size class 144 → 128; one per parser |
| `parser.ParseResult` | 640 B | (600 B) | **not changed**: still size class 640 |
| `ParseOptions`, `tagParser`, `Resolver` | | | not changed: same size class, or few of them |

Each fix moves the bools (and the `uint32` beside them) together, rather than
leaving each one padded to eight bytes between wider fields.

Parsing the six-project corpus (7,509 files, 251,313 calls) with call
extraction allocates 219.3 MB instead of 230.3 MB (−4.8%), and the call sites
it keeps take 29.8 MB instead of 31.9 MB (−6.6%); identical across three
runs. Parse time is unchanged: against `main`, alternately, the four parse
benchmarks moved −4.5% to +3.8% on four rounds, and six more rounds of the one
that read slower put it at −4% by median.

## Stage 4 — guardrails that need configuration (done)

**Complexity: limits, with today's offenders marked.** A ratchet set just
above the worst function (cognitive complexity 279, `canResolveCall`) would
have let every other function grow that far first. So the limits are set
where code gets hard to read, and each function over one today carries
`//nolint:<check> // over the limit before it existed; LINT-PLAN.md stage 4`.
`nolintlint` fails on a marker a function no longer needs, so the list only
shrinks. No test function was over any limit.

| Check | Limit | Worst | Over the limit |
|---|---:|---:|---:|
| `gocognit` | 50 | 279 | 22 |
| `nestif` | 10 | 49 | 18 (marked on the `if`) |
| `funlen` (statements; lines off, the code is commented at length) | 80 | 247 | 8 |

`cyclop` and `gocyclo` are left off: they flag the same functions as
`gocognit` without weighting nesting, which is what makes code hard to read.

**Package boundaries (`depguard`)**, each matching the import graph when it
went in:

- `internal/parser` imports no project package but `internal/log` (tests
  excepted).
- Only `internal/daemon` and `cmd` import `internal/server`.
- Only `cmd` imports `internal/codemap/store` and `internal/codemap/mcp`,
  which keeps SQLite out of the server and the wasm build.

**Banned calls (`forbidigo`):** `fmt.Print*`, `print` and `println` in
production code under `internal/`, where stdout is the LSP channel. `cmd/`
prints by design, and tests are excluded (`make visualtest` prints on
purpose). Nothing matched.

A canary per rule confirmed each fires, and raising the `gocognit` limit made
`nolintlint` report the markers it had made unnecessary.

## Stage 5 — style rules that cost nothing today (done)

29 rules, all with zero findings when enabled, so the change is config only:

- **gocritic (27):** `boolExprSimplify`, `builtinShadow`,
  `builtinShadowDecl`, `commentedOutImport`, `docStub`, `dupImport`,
  `dupOption`, `emptyDecl`, `emptyFallthrough`, `hexLiteral`, `initClause`,
  `methodExprCall`, `octalLiteral`, `preferFilepathJoin`, `ptrToRefParam`,
  `redundantSprint`, `regexpSimplify`, `stringConcatSimplify`,
  `timeExprSimplify`, `todoCommentWithoutDetail`, `tooManyResultsChecker`,
  `typeAssertChain`, `typeUnparen`, `unlabelStmt`, `unnecessaryBlock`,
  `unnecessaryDefer`, `yodaStyleExpr`.
- **revive:** `use-any`, and `enforce-slice-style` set to `literal`
  (`[]T{}` rather than `make([]T, 0)`).

Measured and left off:

| Setting | Findings | Why |
|---|---:|---|
| `enforce-map-style` `make` / `literal` | 54 / 139 | Both spellings are in use; either is that many edits that change nothing at runtime |
| `enforce-slice-style` `make` | 34 | The same churn |
| `enforce-slice-style` `nil` | 32 | Changes behaviour: a nil slice marshals to JSON `null` and an empty one to `[]`, and LSP responses and the code-map output rely on `[]` |

A canary confirmed a sample of the new checks fire (`builtinShadow`,
`yodaStyleExpr`, `unnecessaryBlock`, `use-any`, the slice style).

## Stage 6 — upkeep, on every golangci-lint bump

The procedure, each time the pin in the Makefile moves:

1. **Diff the vendored analyzers** between the two versions' `go.mod`
   (gocritic, revive, staticcheck, gosec, `golang.org/x/tools`); an analyzer
   that did not move adds nothing.
2. **Run the existing config** on the new version. A moved analyzer can find
   new things under an old name — `modernize` is one linter that grows checks.
3. **Migrate anything `[deprecated]`** in `golangci-lint linters`.
4. **Measure the checks and rules the new version adds** and sort each into a
   stage: a bug-catcher that finds nothing goes in, a style rule is a choice,
   and the rest go in the table below with their counts.
5. **Benchmark against `main`, alternately,** if any fix touched the parse or
   request path.

### v2.13.2 → v2.14.0 (done)

- **Moved:** gocritic v0.14.4 → v0.15.0 (no new checks), revive v1.15.0 →
  v1.17.0 (three new rules), gosec v2.28 → v2.29, exhaustive v0.12 → v0.13,
  `golang.org/x/tools` v0.49 → v0.50. staticcheck did not move.
- **New findings under the existing config:** 15, all `modernize`'s new
  `stringscut`, which replaces `strings.LastIndex`/`LastIndexByte` and the
  slicing around it with Go 1.27's `strings.CutLast`. 13 were on the parse
  path. Applied, then tidied by hand where the autofix invented names
  (`ok0`, `before0`) or nested one `if` inside another. The corpus extracts
  the same 251,313 calls with the same bytes per parse; the plain tag parse,
  where most of the edits are, measured 122.9µs against 122.9µs by median
  over six alternating rounds.
- **Deprecated:** unchanged — `wsl`, `gomodguard`, `exhaustruct`, none of
  them enabled. No linter was added.
- **New revive rules:** all three are enabled. `marshal-receiver` found
  nothing. `multiline-if-init` (15) and `use-slices-concat` (7) were chosen
  afterwards. `slices.Concat` returns nil where the `append` to `[]T{}` it
  replaces returned an empty slice, so each of the 7 was checked for a nil
  that could reach JSON or a nil test: the merged resolver lists, two
  component-ref lists that are only ranged over, a BOM prefix that is never
  empty, and two test helpers. None could. Each `multiline-if-init`
  finding was an `if err := f(…wrapped…); err != nil`; the statement now
  sits above the `if`, which then tests `err` alone. 12 of the 15 were in
  tests; the other three are the code map's JSONL writer and its SQLite row
  scans.

## After the stages: test-file exclusions (done)

Tests were excluded from `prealloc`, `unparam`, `gosec` and `staticcheck`.
Measured with each exclusion lifted:

- **`staticcheck`: 2, now enabled.** Both are `InitializeParams.RootURI`,
  deprecated in favour of `workspaceFolders`, in a test that pins how a
  client's `rootUri` decodes. They carry a `//nolint:staticcheck` with that
  reason, and so do the two production reads of the field, which were
  `//nolint:all` — a blanket that also hid every other linter on the line.
- **`unparam`: 4, now enabled.** Helpers whose parameters every caller passed
  the same value: `cliOpts(true)` (8 callers), `assertRef(…, 0, …)` (17,
  renamed `assertFirstRef`) and `benchLoadedServer(5000, 8)` (6, now named
  constants).
- **`prealloc` (35) and `gosec` stay excluded:** pre-sizing a test's slices
  saves nothing anyone measures, and its file paths and permissions are
  fixtures.

## gosec in production code (done)

A global exclusion hid eight `gosec` rules everywhere: 239 findings. It is now
one exclusion for `cmd/`:

- **`G115`, 173, fixed.** Every bare `int`-to-`uint32` conversion of a line or
  column (111 of them in the parser) goes through `internal/conv`, whose
  `Uint32`, `Uint32FromUint` and `Int32` clamp to the target's range instead of
  wrapping. Nothing converted a legitimately negative value; the one place one
  could arise — `ShiftLines` after a deletion — used to wrap to about four
  billion and now stops at 0. The parser's depguard rule allows the package,
  which imports nothing but `math`. `unresolved` over the corpus reports the
  same 91,405 entries before and after, and parse time is unchanged, measured
  against `main` alternately.
- **`cmd/` is excluded from `G304`, `G306` and `G703`** (38): the CLI opens,
  stats, walks and writes the files it is given on the command line.
- **The rest are fixed or carry their reason.** One was a real defect:
  `cfmleditor.findRefs` and `cfmleditor.exportDeps` built their report paths
  from the command's arguments, so a function name such as `x/../../escaped`
  (which `filepath.Join` cleans) or a document outside the workspace wrote a
  file wherever it pointed. `reportPath` now requires a plain file name inside
  a workspace root, the same confinement `codeMapOutputPath` always had, and
  `TestFindRefsReportNameCannotEscape` fails without it. The debug log is now
  `0600` in a `0700` directory, since it can hold source text; CFLint's cache
  directory is `0750`. The CFLint launch, its executable binary, reports meant
  to be shared (`0644`), and the reads the server makes by design are
  suppressed with their reasons.

A canary confirmed an unreasoned `os.WriteFile(p, b, 0o644)` is flagged under
`internal/` and not under `cmd/`.

## Suppressions that were fixable (done)

50 `//nolint` markers went, leaving the 66 that are needed: the 42 complexity
markers, 19 `gosec` on reads and writes the server makes by design, 4
`staticcheck` on the deprecated `rootUri` a client may still send, and the
Unix-socket `usetesting`.

- **`nilerr` (8) and `errcheck,gosec` on `filepath.Walk` (3).** `nilerr` flags a
  `return nil` in a branch reached with a non-nil error, but not a callback that
  collects only when `err == nil` and returns nil once. The walks skip an
  unreadable entry exactly as before. `parse` and `scan` shared a copy-pasted
  walk, now `cfmlFilesUnder`; the tree-sitter oracle test collects paths first
  and reads them after.
- **`exhaustive` (30)**, by `default-signifies-exhaustive: true`.
  - 16 markers were on switches that already had a `default:` arm.
  - The other 14 were on switches that handle a few of many values: 11 token
    loops in the parser, two scope filters and completion resolve. Each now
    ends in an empty `default:` with the marker's reason as a comment. Listing
    the other values would have meant 32 token kinds in eleven places.
  - The parser's machine code is unchanged; only debug line numbers moved.
  - Unlike a `//nolint`, the switch is still checked: deleting its `default:`
    is flagged.
- **`revive` (7).**
  - An unused `t` or `req` is now `_`.
  - `context-as-argument` allows `*testing.T` before the context.
  - The formatter's empty `else if` branch is now `touchesPrevSibling`.
- **`staticcheck` QF1012 (1)** and **`forcetypeassert` (1):** the
  `WriteString(fmt.Sprintf(…))` now writes its three parts directly, and
  `.Interface().(bool)` is now `.Bool()`.

A canary confirmed that the two configuration changes still flag what they
should.

## Left off, and why

| Rule | Findings | Why it stays off |
|---|---:|---|
| `paralleltest` | 1346 | Style; the tests are not written to run in parallel |
| `varnamelen` | 1217 | Short names are idiomatic Go in small scopes |
| `exhaustruct_v5` | 1184 | Zero values are meaningful and relied on |
| `goconst` | 553 | Mostly repeated test strings and CFML keywords |
| `lll`, `mnd` | 325, 164 | Style |
| `wrapcheck`, `err113` | 97, 57 | Would wrap errors whose context is already clear |
| `nilnil`, `gochecknoglobals`, `testpackage`, `funcorder`, `nonamedreturns` | 69, 64, 204, 41, 31 | Style |
| gocritic `unnamedResult`, `paramTypeCombine` | 27, 24 | Style |
| `contextcheck` | 10 | Flags the deliberate `context.Background()` for background work: a request context is pooled and reset when its handler returns |
| govet `shadow` | 13 | All the idiomatic `if _, err := …; err != nil` |
| gocritic `sprintfQuotedString` | 2 | Wrong here: `%q` escapes in Go syntax, and those strings are Mermaid and DOT output |
| gocritic `weakCond`, `rangeAppendAll`, `commentedOutCode` | 1, 1, 5 | False positives: a regexp index is nil or exactly two long; the snippet policy's copy is deliberate; the "code" is an explanatory comment |
| revive `data-race`, `defer` | 3, 16 | False positives: the values are written under a mutex and read after `wg.Wait()`; `CapturePanic` is correct as written |
| revive `identical-switch-branches`, `deep-exit` | 20, 40 | One case per concept on purpose; the CLI exits by design |
| `cyclop`, `gocyclo` | — | Duplicate `gocognit` without weighting nesting (stage 4) |
| `arangolint`, `clickhouselint`, `ginkgolinter`, `loggercheck`, `promlinter`, `protogetter`, `sloglint`, `spancheck`, `testifylint`, `zerologlint` | 0 | For libraries this project does not use |

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

## Stage 2 — small fixes, mostly in tests (~60 findings)

- `forcetypeassert` (8) and revive's `unchecked-type-assertion` (35): an
  unchecked type assertion panics rather than failing with a message.
- `thelper` (16): helpers call `t.Helper()`, so a failure points at the
  caller.
- `deferInLoop` (3), `filepathJoin` (6), `unqueryvet` (3): tests.
- gocritic `importShadow` and revive `import-shadowing` (13): a variable named
  after an imported package.
- revive `confusing-results` (5), `bool-literal-in-expr` (4),
  `redundant-import-alias` (1).

## Stage 3 — memory layout, by hand

`govet`'s `fieldalignment` reports 83 structs in production code. 17 would
shrink; the other 66 only move pointer fields to the front, which shortens
what the garbage collector scans and saves no memory. The linter is not
enabled: it cannot be told to report only the first kind, and satisfying the
second scatters fields that are grouped for readability.

Instead, reorder by hand the structs on the parse path, where the saving is
multiplied:

| Struct | Now | Reordered | How many |
|---|---:|---:|---|
| `parser.CallSite` | 120 B | 112 B | about a million per corpus scan |
| `scriptParser` | 416 B | 368 B | one per parse |
| `parser.Scanner` | 144 B | 128 B | one per parse |
| `parser.ParseResult` | 640 B | 600 B | one per parse, and held per open document |

Measure against `main`, alternately: the parse benchmarks, and the heap held
after indexing a corpus.

## Stage 4 — guardrails that need configuration

- **Complexity ratchet:** `gocognit`, `cyclop`, `nestif` and `funlen` with
  their thresholds set just above today's maximum. Nothing fires, but no
  function can grow past the worst one now, and the thresholds can come down
  later. The parser's dispatch functions are large on purpose; the ratchet
  does not ask them to shrink.
- **`depguard`:** package boundaries CLAUDE.md states in prose, made checked —
  for example `internal/parser` does not import `internal/docs`.
- **`forbidigo`:** banned calls — for example no `fmt.Print*` in
  `internal/server`, where stdout is the LSP channel and a stray print
  corrupts the protocol.

The thresholds and boundaries are proposed from the current code and agreed
before they go in.

## Stage 5 — style rules that cost nothing today

A choice per rule, since each fixes a style for all future code.

- **gocritic, zero findings:** `boolExprSimplify`, `builtinShadow`,
  `builtinShadowDecl`, `commentedOutImport`, `docStub`, `dupImport`,
  `dupOption`, `emptyDecl`, `emptyFallthrough`, `hexLiteral`, `initClause`,
  `methodExprCall`, `octalLiteral`, `preferFilepathJoin`, `ptrToRefParam`,
  `redundantSprint`, `regexpSimplify`, `stringConcatSimplify`,
  `timeExprSimplify`, `todoCommentWithoutDetail`, `tooManyResultsChecker`,
  `typeAssertChain`, `typeUnparen`, `unlabelStmt`, `unnecessaryBlock`,
  `unnecessaryDefer`, `yodaStyleExpr`.
- **revive:** `use-any`, and `enforce-map-style` / `enforce-slice-style`,
  which each need a style chosen.

## Stage 6 — upkeep, on every golangci-lint bump

- Run `golangci-lint linters` and migrate anything marked `[deprecated]`.
  At v2.13.2 those are `wsl` (→ `wsl_v5`, already used), `gomodguard`
  (→ `gomodguard_v2`) and `exhaustruct` (→ `exhaustruct_v5`), neither of the
  last two enabled.
- Run the linters, gocritic checks and revive rules the new version adds, as
  above, and sort them into the stages.

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
| `depguard`, `forbidigo` | — | Stage 4, once configured |
| Complexity limits | — | Stage 4, as a ratchet |
| `arangolint`, `clickhouselint`, `ginkgolinter`, `loggercheck`, `promlinter`, `protogetter`, `sloglint`, `spancheck`, `testifylint`, `zerologlint` | 0 | For libraries this project does not use |

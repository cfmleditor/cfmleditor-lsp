# Resolution gaps

What `unresolved` still reports over the six-project corpus after PR #184, what
is behind each large group, and where a fix would go. Each item says how it was
found, so it can be re-checked rather than taken on trust. Items are ordered by
how many corpus entries they account for, largest first, within each section.

## Measuring

The corpus is six public projects at these commits:

| Project | Commit | Repository |
|---|---|---|
| ContentBox | 312f182 | https://github.com/Ortus-Solutions/ContentBox |
| Lucee | 4602447 | https://github.com/lucee/Lucee |
| TestBox | af36cdd | https://github.com/Ortus-Solutions/TestBox |
| cfwheels | ef04365 | https://github.com/cfwheels/cfwheels |
| coldbox-platform | c318d8d | https://github.com/ColdBox/coldbox-platform |
| fw1 | d7fb9ad | https://github.com/framework-one/fw1 |

Each is measured twice, with a `.cfmleditor.json` at its root:

| Project | "Presets" config | "No presets" config |
|---|---|---|
| ContentBox | `{"frameworks":["coldbox","contentbox","testbox","commandbox","cfmigrations"]}` | `{}` |
| Lucee | `{}` | `{}` |
| TestBox | `{"frameworks":["testbox","commandbox"]}` | `{}` |
| cfwheels | `{"frameworks":["wheels","testbox","commandbox"]}` | `{}` |
| coldbox-platform | `{"frameworks":["coldbox","testbox","commandbox"]}` | `{}` |
| fw1 | `{"frameworks":["fw1"]}` | `{}` |

Run `cfmleditor-lsp unresolved --json <project>` for each project and
concatenate the arrays. Sort each entry's JSON with its keys sorted, so two
runs can be diffed line by line. Then compare two runs **per entry**, keyed by
file (relative to the corpus), line, function and reason. Comparing totals
hides a change that fixes one entry and breaks another.

Record the indexed and scanned file counts from stderr alongside each report.
The CLI skips nested `vendor` directories. At the pinned cfwheels commit,
the normal root scan covers 45 CFML files; the framework is under
`vendor/wheels`. For a separate report covering it explicitly, run:

```sh
cfmleditor-lsp unresolved --json <cfwheels-root> <cfwheels-root>/vendor/wheels
```

This indexes and scans 1,195 files. Keep this report separate from the default
root scan when comparing versions, and use the same root config for both.
If `workspacePaths` is configured, include both roots there as well because
it controls indexing independently of the scan arguments.

```python
import json, sys, collections

def load(path):
    out = collections.Counter()
    for line in open(path):
        if line.startswith("{"):
            d = json.loads(line)
            out[(d["file"].split("/corpus/")[-1], d["line"], d["function"], d["reason"])] += 1
    return out

a, b = load(sys.argv[1]), load(sys.argv[2])
print("added", sum((b - a).values()), "removed", sum((a - b).values()))
```

At PR #184 the totals are **9,520** with presets and **18,618** without
(18,623 when re-measured at the branch point of gap #3's fix, see "The batch
scan is not deterministic" below). After gaps #3 and #4 they are **9,476**
and **18,028**; after gap #1, **9,225** and **17,645**; after gap #2, **9,115** and
**17,532**; after gap #5, **9,020** and **17,439**; after gaps #6 and #7, **9,001**
and **17,429**. Use
`cfmleditor-lsp explain <file> <line+1> [call]` to trace any entry. The report's
lines are 0-based and `explain` takes 1-based lines. `explain` indexes only the
file's own directory unless given `--root <project>`, so pass `--root` or it
can disagree with the batch scan.

## Where the entries are

With presets, the largest groups by project and reason are:

| Entries | Project | Reason |
|---|---|---|
| 2,081 | ContentBox | variable has no component ref |
| 1,596 | coldbox-platform | variable has no component ref |
| 1,155 | Lucee | variable has no component ref |
| 863 | cfwheels | variable has no component ref |
| 557 | Lucee | no qualifier, not in file |
| 395 | cfwheels | not found in extends chain |
| 232 | fw1 | variable has no component ref |
| 181 | cfwheels | component does not exist |
| 164 | cfwheels | base component does not resolve |

"Variable has no component ref" is two thirds of the report. The variables
behind it, most frequent first:

| Project | Variables |
|---|---|
| coldbox-platform | `scheduler` 156, `iservice` 105, `arguments.mapping` 73, `oexception` 70, `now` 50, `arguments.prc.response` 47 |
| ContentBox | `content` 83, `thiscontent` 74, `comment` 65, `prc.author` 60, `c` 54, `entry` 53, `c.restrictions` 47 |
| Lucee | `field` 160, `driver` 97, `coll` 62 |
| cfwheels | `variables.helpers` 70, `details` 64, `arguments.printer` 53, `ssh` 49 |
| fw1 | `user` / `local.user` 29 each, `answer` 18, `rc.question` 17 |

The gaps below explain the largest of these.

## Gaps with a known cause

### 1. An assignment in one closure is invisible to its sibling closures — fixed

- **Was:** coldbox-platform `scheduler` (115 entries) and TestBox-spec
  variables generally. In
  `beforeEach( function(){ scheduler = asyncManager.newScheduler( "x" ); } )`
  the assignment is unscoped, so CFML puts it in variables scope, where every
  `it()` closure reads it.
- **Cause:** not the closure's line range as first thought. An unscoped,
  un-`var`'d assignment was already filed at component level
  (`forceGlobal`) when its type was known at once (`x = new X()`), but one
  typed later from a call (`x = a.b()`, a pending call) was filed with the
  enclosing function, and given the closure's lines. That held in any
  function, not only in closures.
- **Fix:** a pending call carries `forceGlobal` (`pendingCall.global`) and
  settles at component level, in both parsers. Two things came with it,
  each with a test that fails without it:
  - A `<cfscript>` block inside a `<cffunction>` is parsed as its own region
    whose parser knew only the function's arguments as locals, so
    `<cfset var conn>` above the block did not make `conn = …` inside it a
    local. The locals of the function a region split are now carried across
    it, both ways (`openLocals`). `conn = new X()` there had always been
    filed for the whole component.
  - `return x;` settled after the parse (`settleReturnVars`) looked only in
    the function's refs, while the parse-time path also read the component's.
    ColdBox's `getDateTimeHelper()` returns such a variable.
- **Measured:** presets 9,476 → 9,225 (256 removed, 5 added); no presets
  18,028 → 17,645 (389 removed, 6 added). Every addition is a call already
  reported, now against its component: `variables.mixerUtil` is a
  `MixerUtil`, whose `start()` returns its argument, and
  `variables.binder.onShutdown()` is an optional hook the Injector checks
  for with `structKeyExists`.

### 2. A MockBox decoration chain loses the mock's type — fixed

- **Was:** coldbox-platform `iService` (105 entries):
  `variables.iService = model.init( mockController ).$( "getCache", mockCache ).$property( … );`.
  `$()`, `$property()`, `$results()` and the rest return the mock itself.
- **Cause, measured:** mostly not the decoration hops. `model` is assigned in
  ColdBox's `BaseModelTest` (`createMock( annotations.model )`), not in the
  spec, so nothing the spec's parse could read typed the base of the chain.
  The hops did lose the type as well, as the doc said: a decoration made
  the rest of a chain `$any` in the parser (`walkChainRest` asked
  `FuncLookup` for `$`) and dynamic in the resolver (`missingChainHop`).
- **Fix:** the decoration list is `parser.IsMockDecoration`, one list for
  both packages. A decoration keeps its receiver's type, as `init()` does:
  in a chain's rest (`walkChainRest`, `restTypes`), for `x = m.$( … )`
  (`baseVarComponent`), and as a resolver hop (`walkHops`), so
  `m.$( "a" ).missing()` is checked against the mocked class. A chain whose
  type is otherwise unknown but decorates what it is made on is made on a
  mock, and is `$any`. An untyped hop followed by a decoration is a mock
  too, and dynamic: the stricter walk otherwise reported ColdBox's
  `controller.….getRequestService().$( "getContext", … )`. Each rule has a
  case in `internal/resolve/mock_chain_test.go` that fails without it.
- **Measured:** presets 9,225 → 9,115, no presets 17,645 → 17,532 (5 of
  those are the nondeterminism below), no entry added.

### 3. `return variables.x;` does not type the function — fixed

- **Was:** `checkReturnComponent` recorded the *first* identifier of the
  return expression, which for `return variables.print;` is the scope word,
  so `returnVar` named no ref and cfwheels' `DetailOutputService.getPrint()`
  (`print` is `inject="PrintBuffer"`) had no return type.
- **Fix:** a return whose whole expression is `variables.<name>` or
  `this.<name>` records the name with its scope (`returnsVariablesVar`,
  `returnsThisVar` in `internal/parser/result.go`). Such a name is read only
  from the component's refs, never the function's locals, and only from refs
  made through that scope (`RefScope.Admits`), in `settleReturnComponent` and
  `settleReturnVars` alike. The tag parser's `<cfreturn>` applies the same rule
  (`scopedReturnExpr`). `internal/resolve/scoped_return_test.go` has the
  `new`, `inject=`, tag-syntax, declared-before-assigned and wrong-scope cases.
- **Measured:** presets 9,520 → 9,490 (31 removed, 1 added); no presets
  18,623 → 18,518 (119 removed, 14 added). 69 of the removals are
  `getPrint`. Five of the others (cfwheels `FileSystem`) were the
  nondeterminism below, not this change. Every addition is a removed entry re-reported on the same call:
  13 now name MockBox by its declared path (`getMockBox()` is
  `return this.$mockbox`) and still stop at gap #2, and TestBox
  `BaseSpec.cfc:1683` now reports that `cbMockData`, a `box.json` dependency
  the checkout does not install, does not exist.

### 4. A function returning a resolver-matched call is untyped — fixed

- **Was:** `checkReturnComponent` typed a return only when it was a variable,
  `new`, `createObject` or `entityNew`, so CommandBox `BaseCommand.command()`
  (`return getInstance( name='CommandDSL', … );`) had no return type.
- **Fix:** a bare call returned whole — nothing after its argument list but
  the end of the statement — is offered to the componentResolvers as an
  assignment's right-hand side is (`recordBareCallAndChain` hands back the
  expression it built, and `dynamicCall` still applies). The stub generator
  types a returned bare WireBox id by file name, as it already did a variable
  holding one (`componentReturn`), and the stubs are regenerated: `command()`,
  `multiSelect()`, `watch()`, `globber()`, `propertyFile()`,
  `createSubcriteria()` and Wheels' `enableSession()` are typed, each checked
  against the source. The regex widening suggested here was not needed: a
  named first argument is already offered as `getInstance("CommandDSL")`.
- **Measured:** presets 9,490 → 9,476, no presets 18,518 → 18,028, with no
  entry added. Most of the no-preset drop is coldbox-platform `event` (383):
  its specs' `buildContext()` is `return prepareMock( new RequestContext(…) )`,
  which is now `$any` as `x = prepareMock(…)` already was. 53 are cfwheels
  `command()` chains.
- **Former limit, now fixed:** the `unresolved` scan used to index files
  without the config's resolvers (`index.IndexFile`), so a return typed by a
  *configured* resolver reached a caller in another file only through the
  stubs or the server's index. Its indexing pass now uses
  `IndexFileWithResolvers`, and a cross-file regression test covers the
  configured-resolver return.

### 5. A component path computed at run time — fixed, and re-diagnosed

- **Was listed as:** calls on Java objects reported as "no component ref".
  That shape already resolves: a chain from an unstubbed
  `createObject( "java", … )` is `$any`, and so is the variable assigned
  from it (`internal/resolve/computed_path_test.go` keeps a case).
- **What the entries are:** Lucee's admin builds its drivers with
  `createObject( "component", drivernames[ type ] )` or
  `createObject( "component", "dbdriver." & type )`. The path is whichever
  component the program picks, but only a literal path was read: a
  non-string argument recorded nothing (`driver`, 97 entries with `field`
  below), a concatenation was read as its first string
  (`component 'dbdriver.' does not exist`), and the scan stopped inside the
  argument, so a call in it lost its receiver — `arguments.mapping.getPath()`
  in ColdBox's Builder was a bare `getPath()`.
- **Fix:** a computed path is `$any`, as an unmapped `#…#` in a literal one
  already was, in both parsers. The script parser reads the rest of the
  argument for calls (`skipComputedArg`), and `parseCreateObjectRef` now
  calls `readCreateObjectComponent` rather than repeating it.
- **Measured:** presets 9,115 → 9,020 (99 removed, 4 added); no presets
  17,532 → 17,439 (98 removed, 5 added). Every addition is a call in a
  computed path now recorded against its real receiver, which is untyped.
- **Left alone:** `field` (160) is `<cfloop array="#driver.getCustomFields()#"
  index="field">`, and most `driver` refs in the other admin pages are
  `drivers[ form.class ]`, a struct element picked by a key. Those are the
  dynamic keys CLAUDE.md keeps as an honest "no component ref".

### 6. Methods assigned onto an object at run time — fixed

- **Was:** fw1 `tests/CircularTest.cfc` (8 entries), and the same shape in
  Lucee's LDEV1962 and ColdBox's specs:
  `a.getVariables = getVariables; a.getVariables();`.
- **Fix:** the script parser records `x.m = …` (`checkMemberSet`, reached
  from `checkBareCall` where no `(` follows). A call `x.m()` with no chain is
  dynamic when the same function assigned `x.m` on an earlier line
  (`ParseResult.AssignsMember`, checked in `checkMethodOn`). `a.m == b` and
  `a[ k ].m = …` are not recorded.
- **How it is carried:** as a `pendingCall` marked `memberSet`, which is
  already keyed by function, rekeyed and merged per region, and
  `resolvePendingCalls` files it in `ParseResult.memberSets`. A slice of its
  own on `scriptParser` failed `TestParserStructsKeepTheirSize` (368 → 392
  bytes). Recording it as a `$any` ref for `a.m` was tried and dropped: refs
  reach other files through the index, and ContentBox's `prc.author` came
  out untyped in five places. The list is dropped when an edit moves lines,
  since a call is only checked on a fresh parse.
- **Not done:** tag syntax (`<cfset a.m = f>`). No corpus entry needs it.
- **Measured:** presets 9,020 → 9,001, no presets 17,439 → 17,429, no entry
  added.

### 7. `this.x()` detection reads the source line — fixed

`thisCallAnswered` (`internal/resolve/missing_method.go`) accepts `this.x()`
on a component with `onMissingMethod`, and the parser records `this.x()` and
`x()` as the same bare call, so it searched the call's line for `this.x(`.
`CallSite.This` now says which was written. It is set in `CallSite.onScope`,
which the three paths that apply `scopeReceiver` to a call go through (a
statement, an argument list, a `return`), and it fits in `CallSite`'s padding.
`lineAt` and the text scan are gone. The line scan also accepted a bare `f()`
on a line that held a `this.f()`; `TestThisCallsAreKnownWhereverTheyAreWritten`
has that case and the tag-syntax ones. The corpus report is unchanged.

### 8. Colon-named factory arguments lose their assigned type — fixed

`getInstance(name: "coldbox.system.web.tasks.ColdBoxScheduler", ...)`
was untyped in an assignment even though `name = "..."` worked. The
assignment resolver's argument reader now accepts both separators, as do
the framework id and DSL patterns. It also rejects a computed first argument
whose string literal is only a prefix of the expression. ColdBox's scheduler
and task specs alone lose 113 findings with presets from this change.
`TestColonNamedArgumentTypesResolverAssignment` covers local, variables,
this and dotted factory assignments; the computed-id regression keeps a
concatenation from acquiring its prefix's component type.

### 9. CFML argument type annotations are ignored — fixed

ColdBox's `@mapping.doc_generic coldbox.system.ioc.config.Mapping` says what
an otherwise untyped argument holds. Both parsers now recognize this form
alongside JSDoc `@param`. Only a dotted component type is promoted, and only
for an untyped, `any` or `struct` argument. Explicit component and array
declarations stay intact; prose and array-element annotations are ignored.
This types mapping, invocation, injector and cache-provider arguments from
their source documentation rather than their variable names.

Script argument refs are now stored with their function, as tag arguments
already were. Previously a typed parameter leaked into every other method
using the same name. A whole `arguments.name` assignment now preserves its
declared component when a constructor stores it in a field or local variable;
member reads and concatenations are not treated as the argument itself.
The parser and resolver regressions verify valid methods, missing methods,
explicit-type precedence, scope isolation and stored dependencies. They fail
against the PR #189 merge without these fixes.

**Measured against the PR #189 merge**, with the same pinned projects and
default CLI roots used above:

| Configuration | Before | After | Removed | Added | Net reduction |
|---|---:|---:|---:|---:|---:|
| Presets | 7,203 | 6,969 | 239 | 5 | 234 |
| No presets | 15,138 | 14,872 | 309 | 43 | 266 |

ColdBox accounts for most of the improvement: 1,673 → 1,444 with presets,
3,737 → 3,458 without. ContentBox loses five findings in either mode. The
separate Wheels scan including `vendor/wheels` changes 6,910 → 6,908 with
presets (two removed), and stays at 13,262 without (12 removed, 12 added).
Every comparison is per entry, including reason changes.

The five preset additions now reach CacheFactory's untyped `getTaskScheduler`
getter instead of accepting the receiver as dynamic. Without presets, 18
TestBox additions are undocumented `testResults` arguments that previously
borrowed a sibling method's type; the other 25 are ColdBox return-type and
runtime-mixin gaps exposed by correctly typed receivers. Wheels' 12 no-preset
additions are the same undocumented TestBox arguments in its bundled runner.
These remain visible rather than being suppressed to reduce the totals.

### 10. Generated getters ignore constructor field types — fixed

A property without a declared type may still hold a known component:
`variables.stats = new coldbox.system.cache.util.CacheStats()` types the field,
but the generated `getStats()` previously returned nothing known. Getters now
use existing variables-scope field refs when no property metadata types them.
Local variables and `this.name` do not type the getter; explicit getter methods
and primitive property types retain their declarations. Conflicting field
types and unresolved call chains stay dynamic. Field types are collected once
rather than scanning every ref for every property.

Properties also recognize `doc_generic="models.Component"` for untyped,
`any` and `struct` declarations, in both script and tag syntax. Array element
annotations and prose are ignored, and explicit types win. This metadata
types both the field and its generated getter; it alone changes no entries
in the pinned corpus.

ColdBox and TestBox stubs were regenerated from the same pinned commits.
Generation now follows dotted argument component types as well as returns,
so scoping argument refs correctly does not drop their dependencies, such as
LogEvent. Interfaces retain an `interface` declaration and bodyless methods:
emitting ICacheProvider as a concrete component falsely rejected provider
methods such as `getOrSet` that its implementations add.

Regression tests cover workspace and bundled getter chains, valid and missing
methods, field scope, conflicting types, primitive declarations, explicit
getters, property metadata and script/tag interfaces. Property/getter tests
fail against the PR #189 merge. Generator tests fail without its changes with
the corrected parser in place. Regenerating the 95 ColdBox/TestBox stubs again
produces identical files and coverage. Build, vet, full short tests, full short
race tests, pinned lint and diff checks pass.

**Incremental comparison against PR #190's first commit (`f784d26`):**

| Configuration | Before | After | Removed | Added | Net reduction |
|---|---:|---:|---:|---:|---:|
| Presets | 6,969 | 6,900 | 70 | 1 | 69 |
| No presets | 14,872 | 14,801 | 76 | 5 | 71 |

ContentBox improves 2,719 → 2,709 with presets and 7,862 → 7,849 without;
ColdBox 1,444 → 1,395 and 3,458 → 3,401; TestBox 298 → 288 and 713 → 712.
Lucee, FW/1 and the default Wheels root are unchanged per entry. The separate
Wheels vendor scan improves 6,908 → 6,901 with presets (seven removed), and
stays at 13,262 without (one removed, one added). Indexed/scanned file counts
match in every comparison.

The added preset finding reaches Injector's untyped `registerNewInstance`
return. Five no-preset additions reach Controller's service getters,
Injector's `getInstance`, and LogBox's `getConfig`, whose return components
remain unknown. The Wheels no-preset addition now types `oMockGenerator` from
its getter and exposes a component path that does not resolve in that corpus.
CacheFactory's `getTaskScheduler` remains untyped; this change does not infer
types from its `@see` links or conditional factory assignments.

**Whole PR #190 against the PR #189 merge:** presets 7,203 → 6,900
(309 removed, six added); no presets 15,138 → 14,801 (371 removed, 34 added).
The separate Wheels vendor reports are 6,910 → 6,901 with presets (nine
removed), and 13,262 → 13,262 without (13 removed, 13 added).

## Hand-maintained lists that could be generated

- **`moduleHelpers`** (`internal/resolve/modules.go`): the cbi18n, cbfs and
  HTMLHelper helper names, copied from each module's `helpers/Mixins.cfm` at the
  commits noted there.
  - `cmd/cfstubgen` could fetch those modules like the framework sources and
    emit the list, so a new module or a renamed helper is one `make
    framework-stubs` away.
  - Other ColdBox modules with an `applicationHelper` (cbmessagebox,
    cbsecurity, cbauth, …) are not covered.
- **The rule's scope:** it only fires where the component's extends chain
  reaches `coldbox.system.`. A `.cfm` view calling `$r()` without the `coldbox`
  preset is not covered. With the preset, `HelperScope` covers views when the
  module is installed.

## Version skew in the bundled stubs

Each framework is stubbed at one pinned commit
(`internal/frameworkapi/sources.go`). A project written against another version
sees the other version's API.

- **cborm:** pinned to 4.12.1 because ContentBox declares `^4.10.0`, and 5.x
  dropped `getBeanPopulator`. A cborm 5 project would now see 4.12's API.
- **CommandBox:** `ServerService.getServerInfoJSON` (cfwheels'
  `benchmark.cfc`) is absent from the pinned CommandBox. It was not checked
  whether it exists in another version or is a genuine error.
- **Direction:** pick the stub version from the project's `box.json`
  dependency range when it names one. That means stubs for several versions of
  a framework, which is a size question: the stubs are about 1.8MB now.

## Untyped, but correctly so for now

These appear in the added-entries diff of #184 and were left as they are:

| Entry | Why it is not a bug |
|---|---|
| `getBeanPopulator()` has no return type (16 entries, ContentBox) | cborm 4.12 declares none and documents none. A fix belongs in the stub generator (infer from its body) or nowhere. |
| cborm's `getWireBox()` has no return type (13, no presets) | The property is assigned `application.wirebox`, which nothing types. |
| `DetailOutputService.error()` has no return type (13, cfwheels) | `error()` returns nothing, so `.output()` chained on it is a genuine error in wheels-cli. |
| `getRootLogger()`, `site()` and similar have no return type | They declare and document no type. Constructor-backed property getters such as `getStats()` are now inferred (gap #10). |

## Genuine findings the new rules exposed

These were hidden while the component didn't resolve, and each was checked
against the source:

- `addPermission` / `removePermission` on `cbRole` (70 entries), in
  ContentBox's old `build/patches/1-0-x` upgrade scripts. The Role entity's
  `permissions` property has no `singularname`, so CFML generates
  `addPermissions`.
- `generatePasswordResetToken` (ContentBox `authSpec.cfc`), `humanize` on
  wheels-cli's `helpers`, and `isInstanceCheck` / `getAppStartHandlerFired`
  (coldbox-platform tests) exist nowhere in the corpus.
- `evictEntity` (ContentBox `CommentService`) is not in cborm 4.12 or 5.x.

## Small loose ends

- **FW/1 stub guess — no longer present:** at the PR #188 merge, the committed
  and freshly regenerated `framework.one` stubs declare the private
  `getCachedController` / `getController` methods as `any`. Regeneration from
  the pinned FW/1 source produces no diff. The earlier first-return claim
  does not describe the current output.
- **`entityLoad` — fixed:** `entityLoad( "name", … )` and
  `entityLoadByPK( "name", … )`, in script and tag assignments, are tested to
  find an entity by its `entityname`, as `entityNew` already was.
- **`<cfreturn x.y>` is read as `<cfreturn x>` — fixed:** the tag parser now,
  like the script parser, types only a name that stands alone. A member return
  stays untyped because the current parser does not represent member types;
  that is preferable to claiming it returns the receiver's component.
- **The batch scan is not deterministic:** the same binary reports 2,535 or
  2,540 entries for cfwheels without presets from run to run; the five are
  `fileSystemUtil.resolvePath()` in `cli/src/commands/wheels/cache/clear.cfc`
  (`inject="FileSystem"`), reported as "component 'FileSystem' does not
  exist" in some runs only. `explain` reports it every time. Six runs at the
  PR #185 merge showed it, so it predates gaps #3 and #4; the parallel
  scan's lazy indexing is the first suspect. Compare runs with that in mind.
  - **Fix:** `EnsureIndexed` adds namespaced
    framework stubs to the shared index. The bare-name fallback subsequently
    finds those virtual files through `FindFilesByBasename`, so loading
    `commandbox.system.util.FileSystem` in another file can make the bare
    `FileSystem` id resolve without a CommandBox preset. The path cache then
    preserves whichever answer was reached first. Exclude virtual stubs from
    the workspace candidates before choosing the nearest file; configured
    CommandBox ids still resolve through `IDPackages`.
  - `TestBareComponentLookupDoesNotDependOnLoadedStubs` covers both lookup
    orders, a fresh path cache after loading, explicit namespaced lookup,
    configured ids, and precedence of workspace source over a loaded stub.
    Confirmed to fail against the pre-fix code.
  - **Measured on the pinned commits with the current CLI:** presets
    7,203 before and after; no presets 15,138 before and after, with no
    entries added or removed in either configuration. Six cfwheels scans
    without presets produced identical findings (61 each). These counts
    differ from the earlier handoff measurements: the current CLI skips
    `vendor`, where this cfwheels checkout keeps its framework source, and
    does not scan the CLI file named above. The deterministic regression
    reproduces the stub-loading cause directly; the repeated corpus scans
    alone do not reproduce the original five fluctuating entries.
- **A stale stub — fixed:** regenerating CommandBox from its pinned source
  incorrectly typed `ArtifactService.getPackagePath()` as
  `commandbox.system.services.ConfigService`. Its `getArtifactsDirectory()`
  dependency declares `string`, but a plain parse inferred ConfigService
  from `var path = configService.getSetting(...)` and propagated that
  component through the same-file call. Declared primitive return types now
  discard concrete component guesses, including deferred variable returns
  in both syntaxes. Runtime-created components keep their dynamic `$any`
  marker; untyped and generic declarations still infer component types.
  The parser and generator regressions fail without the fix. Regenerating
  CommandBox and FW/1 now produces no changes to the committed stubs.
  - **Measured against the PR #188 merge:** all twelve default corpus
    reports are identical per entry (7,203 findings with presets, 15,138
    without). The separate Wheels report including `vendor/wheels` is also
    identical: 6,910 findings with presets and 13,262 without, over 1,195
    scanned files. This fixes incorrect inference and regeneration drift;
    it does not reduce the measured unresolved-call counts.
- **Merge commit attribution:** the merge commit of `origin/main` on this
  branch lacks the attribution lines. Fixing it would need a force-push.

## Masa CMS: literal startup bean registrations (5d136ab)

Additional application corpus: [MasaCMS/MasaCMS](https://github.com/MasaCMS/MasaCMS),
commit `696383140578f8dea3ece26f80cd7bfb370ddf0f` (7.6.1). Copy
[`scripts/corpus/masacms.json`](scripts/corpus/masacms.json) to the checkout's
`.cfmleditor.json`, then run `cfmleditor-lsp unresolved --json <checkout>`.
Restore the checkout's original configuration afterward. The supplied mappings
mirror applicationSettings.cfm; beanPaths and startupFiles expose the application's
factory setup without adding hand-written alias rules. Development dependencies
TestBox 2.3 and DocBox 2 are absent at this pin; this measurement adds no stubs.

The default root scan indexes and scans **897 files**, excluding ten shipped CFML
files under core/vendor. With identical configuration, literal registration
inference reduces **18,539 → 16,860** findings: **1,820 removed, 141 added**.
The additions mostly expose unknown return types further along MuraScope and bean
call chains; they are retained rather than hidden by treating aliases as dynamic.
The total equals the prior diagnostic experiment with 57 hand-written aliases.
Its reason labels used dotted component names; automatically discovered absolute
paths display component basenames, so 138 reason labels differ between those
experiments despite identical unresolved call sites.

Source examples at core/appcfc/onApplicationStart_include.cfm: `declareBean` at
335, `content → contentBean`, `user → userBean`, `$ → MuraScope`, and chained
bundle aliases at 337–389. The Adobe/Lucee-specific contentGateway registration
has two distinct targets and remains untyped. Both the editor and CLI discover
registrations before indexing return types, and refresh with resolver invalidation.
Tests preserve missing-method diagnostics and ordinary component basename lookup.
The existing six-project corpus and separate Wheels vendor scan are unchanged
per entry, with matching file coverage, both with and without presets.

The follow-up below handles setter injection on managed components. Remaining
steps include FW/1 controller wiring, struct member assignments such as
rc.contentBean, and shared-service loops/wrapper return types.
The earlier ColdBox fluent-return and scheduler cases remain separate regressions
to investigate. These semantic gaps should be measured under the supplied mappings
rather than counted together with missing runtime mapping configuration.


## Masa CMS: managed setter arguments and field types

With the same configuration and **897 indexed/scanned files**, the follow-up
reduces **16,860 → 16,557** findings: **392 removed, 89 added**, a net reduction
of **303**. Relative to the scan before either Masa batch, the combined change
is **18,539 → 16,557** (2,210 removed, 228 added). No corpus configuration or
source files were changed.

A CFC inside beanPaths can obtain a dependency through a public, single-argument
`setName(Name)` method. Generic arguments receive a separate inferred component;
declared signatures remain unchanged, and primitive/documented/explicit component
types keep priority. Whole argument assignments propagate into fields and generated
getters. Script, tag, and mixed tag/script functions preserve that type throughout
region boundaries. Private, multi-argument, mismatched-name and unmanaged setters,
computed values and argument members do not acquire the dependency's type.

Removed groups include 193 calls on variables.configBean, 59 on
variables.settingsManager, and 42 on variables.contentManager. Source examples
include core/mura/bean/beanExtendable.cfc's setConfigBean at 103 and tag setters in
core/mura/extend/extendManager.cfc at 294 and core/mura/client/httpSession.cfc at 90.
Bean aliases also govern property/getter lookup: the user alias names userBean,
rather than the same-basename SOAP user CFC. Real missing methods remain reported.

The added findings expose untyped return chains on getSite/getClassExtensionManager
and untyped parameters that previously borrowed a component field's type. In
particular, a function argument shadows a field even when its type is unknown.
The six-project corpus adds three findings with presets and four without: Lucee's
_Mail.cfc getMails(smtpServer), ColdBox's MethodInvocationTest invokeMethod and
invokeMethod2(invocation), and without presets an InterceptorStateTest event
parameter. Source confirms these are generic parameters, not the same-named fields.
ContentBox, TestBox, FW/1 and both Wheels scans remain unchanged per entry.

Whole PR relative to merged PR #189: presets **7,203 → 6,903** (309 removed,
9 added); no presets **15,138 → 14,805** (371 removed, 38 added), with matching
coverage. ColdBox is now 1,397 / 3,404 and Lucee 1,985 in both modes. The separate
Wheels vendor report remains 6,901 / 13,262.

Editor indexing gives closed managed files the same getter/field types as open
files and the CLI; lazy dependency indexing carries the same lookup. Bean-map
cache ownership follows the resolver, so changed or removed bean roots replace
stale entries. Regressions fail with setter inference disabled and cover missing
methods, parameter shadowing, alias lookup, scope boundaries, declared signatures,
closed-file indexing, configuration refresh, and script/tag/mixed syntax.

FW/1's admin controllers sit outside the supplied bean roots: admin/Application.cfc
sets its bean factory at 197, and admin/framework.cfc autowires controllers/services
at 1249–1270 and 1401. The following batch models that managed scope separately.
Struct members, shared-service loops, wrapper returns and runtime factory values
remain subsequent work.

## Masa CMS: source-backed FW/1 controller wiring

The next batch recognizes the nearest Application.cfc extending a real FW/1
implementation, with an explicit setBeanFactory call in setupApplication or global
scope. The framework source must declare its factory/controller/autowire methods
and call autowire from getCachedComponent (older controllers/services) or
getCachedController (newer controllers). Preset stubs and a matching extends
basename alone do not establish injection. The application scope is cached with
the resolver and shared by editor, closed-file indexing and CLI parsing.

Only direct controller/service files at the application's conventional base are
managed; older explicit usingSubsystems=true permits one subsystem directory.
Models, views, templates, deeper directories and nested applications are excluded.
Conflicting literals, computed subsystem flags, struct-form settings, relocated
bases, custom directories, subsystem factories and overridden setBeanFactory
methods remain unsupported. A literal base default can be followed by a runtime
expression, as in Masa's setFrameWorkBaseDir; only the static default is modeled.
FW/1 3.5+'s subsystem-directory convention and automatic DI configuration remain
separate work. Dependency identities still come from the configured workspace
bean map and alias rules; runtime factory overrides are not modeled.

With unchanged configuration and the same **897 indexed/scanned files**, Masa
changes **16,557 → 16,355**: **223 removed, 21 added**, net **202**. The full PR
changes **18,539 → 16,355**: **2,433 removed, 249 added**, net **2,184**. Removed
calls include controller dependencies on permUtility, settingsManager,
contentManager, contentUtility and utility. Every addition is a later untyped
return-chain diagnostic: twenty on settingsManager.getSite(), one on
trashManager.getTrashItem(). Real missing methods remain checked. All fourteen
existing corpus comparisons, including both Wheels vendor scans, are unchanged
per entry with matching indexed/scanned coverage.

Scope, lazy-indexed getter, editor/closed-file and CLI regressions fail with the
FW/1 scope hook disabled. Source inspection identifies the next dependencies:
settingsManager.getSite() returns variables.sites[key], populated through a
separate builder struct in setSites(); rc.contentBean assignments chain through
contentBean.loadBy() and contentManager.read(). These need collection/struct
member types and verified wrapper returns rather than a global rc component type.

## FW/1 documentation review and revised follow-up

The official [Developing Applications guide](https://framework-one.github.io/documentation/4.3/developing-applications/),
[DI/1 guide](https://framework-one.github.io/documentation/4.3/using-di-one/), and
[Subsystems guide](https://framework-one.github.io/documentation/4.3/using-subsystems/)
give a broader contract than the Masa-specific source pattern above. Masa's
embedded admin/framework.cfc identifies itself as **FW/1 1.2** at line 1202;
the separate pinned FW/1 corpus is **4.3.2**. Treat their factory and loader
conventions separately.

- Modern FW/1 creates DI/1 automatically by default (`diEngine="di1"`), scanning
  `model` and `controllers` unless diLocations changes them. An application need
  not call setBeanFactory. Manual factory management uses `diEngine="none"`;
  custom/AOP/WireBox engines and diComponent have distinct contracts.
- Struct-form variables.framework configuration is documented application syntax.
  The framework can also be constructed and delegated to without Application.cfc
  extending it. Custom base/controller directories are documented settings.
- DI/1 resolves constructors, explicit setters and implicit property setters.
  Constructors can receive singletons or transients, while setters and properties
  receive **singletons only**. Typed/defaulted properties are omitted by default
  in current DI/1, subject to omitTypedProperties/omitDefaultedProperties.
  Explicit constructor overrides and configured constants can replace bean values.
- Subsystems 2.0 arrived in **3.5**, alongside legacy top-level subsystems; it did
  not replace them in 4.0. Default subsystem locations are subsystems/name, and
  each automatically managed subsystem factory inherits from the top-level
  factory. Subsystem-local beans are not visible in the parent. An explicitly
  supplied subsystem factory inherits only if a parent is actually installed.

The current bean-root setter lookup has no lifetime metadata. A minimal configured
bean-root fixture containing model/beans/User.cfc and a Consumer.setUser(user)
resolves variables.user.run(), even though DI/1 would skip that transient setter.
This is a correctness gap, not a measured corpus reduction. Do not apply a blanket
transient exclusion to Masa's FW/1 1.2 controller loader: its separate autowire
loop checks containsBean, not isSingleton. Factory-owned injection and framework
fallback injection need distinct policies.

Revised order for further resolution work:

1. Model factory identity, DI mode/configuration and bean lifetime; preserve
   aliases, transient rules, constants and explicit overrides. Add regressions
   for dependencies that must remain uninjected before broadening scope.
2. Support documented literal struct configuration and automatic DI locations,
   then constructor injection and implicit property eligibility under that policy.
3. Resolve legacy/new subsystem layouts and local/parent factory precedence;
   cover custom literal base/controller directories and delegated applications.
4. Return to collection/struct members and wrapper returns, including getSite()
   and rc.contentBean, with those dependency types established.

Use versioned documentation, pinned framework source, and real applications as
complementary evidence. Dynamic or conflicting configuration remains unknown.


## DI/1 factory lifetime and property eligibility

The next correctness batch separates direct bean retrieval from factory-owned
setter/property injection. Startup files and bounded literal includes recognize
literal `new` DI/1 construction through real implementation/parent method
metadata. Modern `Application.cfc extends="framework.one"` supplies the documented
default DI/1 mode or literal struct/direct configuration. Manual factory setup,
other engines and custom diComponent settings do not imply automatic DI/1.
Only supplied bean roots gain setter inference; this does not yet discover new
automatic roots or inject constructors.

Factory policies honor immediate `beans` folders, configured transient folders,
transient/singleton patterns, singular mappings, omitted directory aliases,
literal exclusions, recursion, constants, explicit declaration lifetimes,
alias chains and whole typed-value addBean registrations. Exclusions are
case-insensitive literal substrings, matching DI/1 source rather than regexes.
Conflicting factory identities/configuration, cycles and unsupported regexes
withhold injection types. Direct getBean identity remains separately available.
The older FW/1 controller fallback retains its containsBean policy.

Property inference requires accessors=true or persistent=true, excludes
setter=false and the default typed/defaulted metadata omissions, and honors
literal omission flags. An ignored implicit property blocks a matching explicit
setter; setter=false permits the separate explicit setter. Property declaration
order and full document replacement retain the same behavior. Explicit source
and documented component types remain authoritative.

With unchanged configuration and **897 indexed/scanned files**, Masa changes
**16,355 → 16,385**, **0 removed / 30 added**. This intentionally removes
unjustified type assumptions rather than reducing the count: 23 findings involve
manually populated variables.$ in contentCalendarUtilityBean, three involve an
email local that previously borrowed a defaulted property's bean identity, two
are oauth user getter chains, one is manually populated variables.content, and
one is fileBean.getSite(). The types of manually supplied setter values need
separate argument/member flow. Whole PR versus the same base is now
**18,539 → 16,385**, **2,405 removed / 251 added**, net **2,154**.

Lifetime/configuration, registration/alias, property/index, declaration-order,
editor/closed-file/configuration-refresh and CLI regressions fail with the policy
disabled and pass restored. Unquoted script property names, booleans and dotted CFC types are now parsed
as complete literals. The pinned FW/1 corpus's generated getters resolve
**508 → 409** with presets and **794 → 695** without, **99 removed / 0 added**
in each mode over the same **305 indexed/scanned files**. All removed findings
are getters on UserOneLevel/UserTwoLevel/UserThreeLevel and their Contact/Address
chains in frameworkPopulateTest. The other twelve comparisons, including Wheels
vendor coverage, preserve every per-entry finding and indexed/scanned count.
The default six-project totals become **6,804 / 14,706**; whole PR compared with
the original base is **408 removed / 9 added** with presets and **470 removed /
38 added** without. Runtime factory replacement,
computed registrations, auto-exclusion defaults, inherited property metadata,
liberal pluralization and custom factory behavior are not modeled. Constructor
injection, automatic root discovery and subsystem parent/local precedence remain
next, followed by verified struct/collection member and wrapper return types.


## DI/1 default constructor dependencies

Known DI/1 factories now supply generic argument types to public `init` methods,
including required/optional arguments and transient dependencies. This follows
Masa's pinned IOC cleanMetadata constructor selection at 421–441 and its
required/optional containsBean construction at 845–879. Inferred components stay
separate from declared argument signatures, and declared/documented types remain
authoritative. Whole arguments.name assignments type fields and generated
getters using the existing scope-aware field flow. Script, tag and mixed syntax
share the hook, including lazy indexing, editor parsing and CLI scans. The parser
reuses its managed-argument callback slot, keeping its pinned struct size, and
ordinary files do not allocate the factory callback.

Only recognized factory roots supply constructor inference; generic bean roots,
older FW/1 controller fallback scope, private init methods and custom construction
implementations do not. Literal declaration overrides and getBean constructorArgs
observed on the known factory receiver in configured startup sources suppress
matching argument/property/setter types. Unknown override bags withhold those
types; empty bags preserve the default. Registered instances/constants bypass
factory construction and setter injection. Their known component identities can
still serve as dependencies of other beans. Runtime calls outside startup
sources, fluent override builders and inherited constructor metadata remain
outside this static default-construction model.

With unchanged configuration and **897 indexed/scanned files**, Masa changes
**16,385 → 14,179**, **2,445 removed / 239 added**, net **2,206**. Removed findings
include configBean, settingsManager and pluginManager constructor fields and
subsequent values obtained through those dependencies. All additions were
reviewed: **221** are settingsManager.getSite() return chains, **15** are
configBean.getClassExtensionManager() chains, two are contentBean wrapper-return
chains, and one is a newly checked missing emailGateway.getSessionSearch() call
in dashboardManager. The actual method exists on sessionTrackingGateway, not on
the indexed emailGateway. Real missing-method checking remains enabled.

Whole PR against the same original base is now **18,539 → 14,179**,
**4,850 removed / 490 added**, net **4,360**. All fourteen other corpus comparisons
are unchanged per entry, with matching indexed/scanned coverage; six-project
presets/no-presets totals remain **6,804 / 14,706**, Wheels vendor **6,901 / 13,262**.

New constructor, lazy getter, editor/closed-file/engine-refresh and CLI
regressions fail with the constructor lookup disabled. Separate override and
registered-value regressions fail with the override gate disabled. Restored
checks pass and preserve declared contracts, private/unmanaged boundaries,
transient setter exclusion, sibling-parameter scope and real missing methods.
Build, vet, full short/race suites, pinned lint (zero issues), formatting and
diff checks pass. Collection/struct members and verified wrapper
returns, automatic root discovery and subsystem factory precedence remain next.


## Factory-backed wrapper return chains

A whole returned factory call followed by method calls now follows the actual
method return types, instead of stopping at the factory result. For example,
`return getBean('Builder').configure().build()` returns Product when build's
return contract names Product. This works in script, tag and mixed functions,
including script islands inside tag functions without typed arguments. Known
factory roots come from the configured resolver rules; this does not add
framework or Masa-specific component names.

Every recorded return expression must resolve to the same component. Unknown
methods, conflicting components, primitive/unknown branches, indexed results,
member reads and larger expressions with operators prevent concrete inference.
Declared return contracts retain priority. Closure returns do not contribute to
the enclosing function; full document replacement refreshes the inference.
Without method lookup, only whole factory results and constructor/init chains
can retain their identity. Existing shallow/lazy indexing without FuncLookup
cannot verify arbitrary method chains; this batch does not change that boundary.
Collection member flow and argument-sensitive polymorphic returns remain open.

At the pinned Masa commit and unchanged **897 indexed/scanned files**, this
batch changes **14,179 → 14,147**, **83 removed / 51 added**, net **32**. Removed
findings cover factory-backed comment, user, iterator and plugin-setting wrappers.
All 51 additions concern beanORM.loadBy(): the actual implementation at 918–924
returns a query, a beanIterator or this according to returnFormat. Its last
`return this` previously overrode the other paths. A concrete bean return is
therefore withheld; specializing by a literal/default call argument is future
work. Counts are not reduced by retaining the incorrect assumption.

Whole PR against the same original base is now **18,539 → 14,147**,
**4,927 removed / 535 added**, net **4,392**. All fourteen other corpus comparisons
are unchanged per entry with matching indexed/scanned coverage; default six
project totals remain **6,804 / 14,706**, Wheels vendor **6,901 / 13,262**.

Parser regressions cover all three syntaxes, nested/multiline arguments, same
and conflicting returns in either order, primitive and component contracts,
unknown methods, indexing/operators, closures and full replacement. A resolver
integration checks the actual final component and preserves a missing method
that belongs only to the original builder. These tests fail with the new return
settlement disabled and pass restored. The parser's pinned struct sizes remain
unchanged; return observations reuse pending-call storage, and files without
configured resolvers skip this inference.

Raw parser call extraction is unchanged per entry (60,546 records in each
version). The CLI resolved-plus-reported sum is a filtered statistic and falls
by three; builtin filtering and missing-base grouping prevent treating that sum
as call coverage. Full build, vet, short/race suites, pinned lint (zero issues),
formatting and diff checks pass.


## Uniform struct-element return contracts

Getters returning an indexed struct value now retain a component contract when
all observed element writes agree. Static dot keys and bracket keys contribute
values; whole aliases share writes while element copies form directed
dependencies. An empty
struct initializer plus a typed write can ground a copy cycle, while an empty or
uninitialized source cannot borrow a type from its reader. Whole-struct getters
keep their existing contract and never acquire the element's CFC type.

Element sources come from explicit constructors, configured factories or the
actual return contracts of methods on known receivers. No framework or Masa
component names are hardcoded. Unknown writes, conflicting component/primitive
values, whole replacement, recognized mutators and tag output bindings prevent
inference. Local/argument names cannot borrow component fields, and variables
and this remain separate. Computed writes into a scope and parent replacement
invalidate affected collections. Every explicit return must be an indexed value
with the same final component; declared return types retain priority.

Closed-file indexing retains compact component/method source contracts rather
than guessing the method receiver's identity. Resolver lookup follows current
indexed definitions, so a producer edit changes an existing getter immediately.
Compaction detaches strings and nested method slices from parse storage. Source
unions are limited to sixteen distinct contracts; deferred lookup is bounded to
eight recursive levels and a shared budget of 128 source/method steps. Cycles,
unknown dependencies and exceeded limits withhold a concrete type.

This models observed static writes, not arbitrary reflective/runtime mutations
or mutations through escaped structs. Closure bindings are not yet modeled by
this collection analysis: files containing script closures withhold its new
inference. Arrays, object-guarded lazy members, argument-sensitive polymorphic
returns and arbitrary wrapper chains without FuncLookup remain open. Masa's
getClassExtensionManager() is not safe to infer from isObject alone: configBean
also permits computed instance-field replacement through setValue().

At the same Masa commit and unchanged **897 indexed/scanned files**, this batch
changes **14,147 → 13,749**, **451 removed / 53 added**, net **398**. Removed
findings include settingsManager.getSite() and resource-bundle lookups. All 53
additions are downstream settingsBean contracts now reached through getSite:
getRBFactory (22), getContentRenderer (21), getApi (6), getCacheFactory (4).
The first two involve lazy/externally supplied object members; getApi constructs
computed component names, and cacheFactories is nested under a replaceable
instance struct. These remain unknown rather than assuming receiver identity.

Whole PR against the original base is now **18,539 → 13,749**,
**5,101 removed / 311 added**, net **4,790**. All fourteen other corpus
comparisons remain unchanged per entry, with matching indexed/scanned coverage.
Six-project presets/no-presets totals stay **6,804 / 14,706**, Wheels vendor
**6,901 / 13,262**. Raw Masa parser call extraction is identical per entry:
**60,546 records** in each version. The CLI resolved-plus-reported statistic is
filtered and is not raw call coverage.

Parser cases cover script, tag and mixed functions, agreeing/conflicting writes
and returns, grounded/ungrounded copies, shadowing, dynamic scope and parent
writes, operators, closures, mutators, tag output bindings and declared types.
Resolver integration covers closed indexing, canonical agreement, conflicting
and primitive producer returns, recursive getters, dependency replacement and
real missing methods on the final CFC. Regressions fail with collection
settlement disabled; the closed-file regression separately fails with deferred
lookup disabled. Restored checks, build, vet, full short/race suites, pinned lint,
formatting and diff checks pass.


## Automatic mappings, literal bootstrap services and explicit record members

Mapping discovery now reads the governing Application's literal includes and
relative parent constructors, including dot assignments and whole mapping
structs. Known path variables, current/base template paths and literal
left/right length arithmetic preserve the declaring template's context. Whole
struct RHS expressions read the previous struct before replacement, so map
iteration order cannot create a false dependency. Comments, computed paths and
cyclic/deep include traversal do not execute CFML; traversal is bounded to 64
files and 16 levels. Repeated includes and mapped parent aliases are not yet
modeled as distinct execution contexts.

Physical non-root CFConfig mappings are defaults, loaded from the nearest
.cfconfig.json or a static server.json cfconfig.file selection (up to 32 ancestor
directories). Relative physical paths are based on the selected config file.
Archive-primary entries and environment/runtime placeholders remain unknown.
Application declarations override defaults and explicit editor configuration
wins case-insensitively. Watched includes, parent CFCs and JSON changes invalidate
mapping and resolver caches; the known-issues reload path retains priority.

Finite script listToArray loops now model shared-scope service installation.
The list must be literal (at most 64 simple IDs), key and bean interpolation must
use the same iterator, and nested loops, closures and iterator mutations are
excluded. The actual factory must expose getBean(beanName) and
declareBean(beanName,dottedPath); configured/registered bean identities supply
the component, never a service-name guess. Scalar variables-scope factory writes
and shared aliases are followed with bounded recursion. Unknown factory
replacements, absent beans and conflicting/unknown service writes withhold the
new loop inference.

Explicit static record writes such as rc.$=getBean('$') or
variables.instance.gateway=new Gateway() type that exact member without typing
the container. Constructors, configured factories and verified return contracts
supply identities. Unknown/conflicting writes, dynamic keys, parent replacement,
recognized mutators, escaping container aliases and closure-containing source
withhold new inference. Record paths retain their full identity: arguments.data.$
cannot borrow an unrelated $. Local/argument roots shadow component record fields,
and variables/this remain distinct. Body edits invalidate ref/link caches and
recompute member bindings from current content. Struct-literal members, arbitrary
record aliases, object property contracts and generic keyed event values remain
open; getValue('MuraScope') is not typed from its key alone because externally
supplied data and arguments can replace that slot.

At the pinned Masa commit, with the existing configuration and the same **897
indexed/scanned files**, this batch changes **13,749 → 12,288**, **1,490 removed /
29 added**, net **1,461**. The removed groups include 673 application.settingsManager,
133 application.serviceFactory, 122 application.contentManager, 106
application.permUtility and 95 application.pluginManager findings. All 29
additions were reviewed: fifteen arguments.data record findings, five rc.$,
one arguments.rc.userBean, six newly reached return-chain gaps, and two checked
missing methods (emailDAO.getSubject and userManager.readByEmail). Whole PR
against the original base is **18,539 → 12,288**, **6,576 removed / 325 added**,
net **6,251**.

Removing only the manual mappings, while retaining bean/startup/resolver config,
changes **17,488 → 13,484**, **4,189 removed / 185 added**, net **4,004** over the
same 897 files. All eleven root mappings are discovered from source. This is
not identical to the configured run: nested access-restriction Applications
under core/mura and other directories do not declare those mappings. Library
analysis needs the calling application's mapping context to bridge that gap;
blindly inheriting parent mappings would break application isolation.

All fourteen other corpus comparisons retain indexed/scanned coverage.
ContentBox changes 2,709→2,705 / 7,849→7,845 (five removed, one added per mode).
TestBox changes 288→291 / 712→715 (three thread-attribute findings per mode).
Default Wheels changes 16→18 / 61→63 (two argument-record findings per mode).
ColdBox changes 1,397→1,404 / 3,404→3,411 (seven record findings per mode).
Lucee and FW/1 are unchanged per entry. Six-project totals are **6,812 / 14,714**.
Wheels vendor coverage changes **6,901→6,915** (five removed, nineteen added)
and **13,262→13,264** (three removed, five added). New findings involve thread
attributes, struct literals, returned records, dynamic object properties and
mock members previously accepted by the incorrect last-segment fallback.
Every new entry has a source review; reductions are not obtained by losing files.

A fresh raw-call capture immediately after AllCalls in the actual CLI scan
preserves all **60,921 records per entry** in both versions. The earlier 60,546
capture used a different probe path; it is not directly comparable. Filtered
resolved-plus-reported totals remain unsuitable as raw coverage measurements.
Regressions fail independently when source walking, CFConfig loading, startup
loop inference, member settlement or preserved record identity is disabled,
then pass restored. Build, vet, full short/race suites, pinned lint, formatting,
diff and parser performance checks pass. CI must be checked on the published
commit, as for earlier batches.

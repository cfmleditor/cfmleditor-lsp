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
**17,532**. Use
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
- **Limit:** the `unresolved` scan indexes files without the config's
  resolvers (`index.IndexFile`), so a return typed by a *configured* resolver
  reaches a caller in another file only through the stubs or the server's
  index, which parses with them. Same-file calls always see it.

### 5. Calls on Java objects are "no component ref" rather than dynamic

- **Evidence:** Lucee `field` (160), `driver` (97), and most of the rest of
  Lucee's no-component-ref entries.
- **Shape:**

  ```cfml
  var field = createObject( "java", "…QueryImpl" ).getClass().getDeclaredField( "x" );
  ```

- **Cause:** a chain that starts at an unstubbed Java object is dynamic when
  called, but the variable it is *assigned* to gets no ref. A later
  `field.setAccessible( true )` is then reported.
- **Fix direction:** a pending call whose chain base is dynamic (`$any`,
  including a Java `createObject` with no stub) should give the variable
  `$any`.
- **Where:** `resolvePendingCalls` / `baseVarComponent` in
  `internal/parser/result.go`. `returnedComponent` (added in #184) already
  applies this rule to returns.
- **Caveat:** check it doesn't swallow calls that a `javaStubsPath` would type.

### 6. Methods assigned onto an object at run time

- **Evidence:** fw1 `tests/CircularTest.cfc`, 8 entries.
- **Shape:**

  ```cfml
  a.getVariables = getVariables;
  a.getVariables();
  ```

- **Fix direction:** a call `x.m()` where the same function assigns `x.m = …`
  on an earlier line is dynamic. The parser would need to record member
  assignments on locals, as `HasScopedAssignment` does for the variables scope.
- **Priority:** small, and only common in tests.

### 7. `this.x()` detection reads the source line

`thisCallAnswered` (`internal/resolve/missing_method.go`) accepts `this.x()` on
a component with `onMissingMethod`. Only a `this.`-qualified call reaches
`onMissingMethod`, and the parser records `this.x()` and `x()` as the same
bare call. So the resolver re-reads the line text, only on the failure path.

- **Cleaner fix:** add a `This bool` to `parser.CallSite`, set where
  `scopeReceiver` returns the `this` case.
- **Size:** CallSite's small fields are grouped at the end, and a bool fits in
  the existing padding. `TestParserStructsKeepTheirSize` will say if it
  doesn't.
- **What to check:** every path that records such a call goes through
  `scopeReceiver` (see CLAUDE.md), so that is the one place to set it. Then
  delete `lineAt` and the text scan.

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
| `getStats()`, `getRootLogger()`, `site()` and similar have no return type | They declare and document no type. |

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

- **FW/1 stub guess:** `framework.one`'s private `getCachedController` /
  `getController` are stubbed from their first `return this;`. The stub
  generator takes a function's first return, and these functions return `this`
  in one special case. They are private, so no application code calls them.
- **`entityLoad`:** `entityNew( "name" )` is tested to find an entity by its
  `entityname`. `entityLoad` and `entityLoadByPK` were not checked.
- **`<cfreturn x.y>` is read as `<cfreturn x>`:** the tag parser takes the
  first identifier of a return expression that holds no `(`, so a function
  returning a property of `x` is typed as `x`'s component. The script parser
  types only a name that stands alone. Found while fixing gap #3; left as it
  is until its effect on the corpus is measured.
- **The batch scan is not deterministic:** the same binary reports 2,535 or
  2,540 entries for cfwheels without presets from run to run; the five are
  `fileSystemUtil.resolvePath()` in `cli/src/commands/wheels/cache/clear.cfc`
  (`inject="FileSystem"`), reported as "component 'FileSystem' does not
  exist" in some runs only. `explain` reports it every time. Six runs at the
  PR #185 merge showed it, so it predates gaps #3 and #4; the parallel
  scan's lazy indexing is the first suspect. Compare runs with that in mind.
- **A stale stub:** regenerating the stubs at the PR #185 merge, with nothing
  changed, rewrites `ArtifactService.getPackagePath()` to return
  `commandbox.system.services.ConfigService`. It returns a string (`var path
  = getArtifactsDirectory() & …`), so the committed stub, with no return
  type, is kept. A reduced copy of the function does not reproduce it, so the
  cause is elsewhere in the file.
- **Merge commit attribution:** the merge commit of `origin/main` on this
  branch lacks the attribution lines. Fixing it would need a force-push.

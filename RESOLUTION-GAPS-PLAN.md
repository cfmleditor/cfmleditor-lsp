# Remaining resolver gaps: coverage and proposed fixes

Updated 2026-10-01, after merged [PR #190](https://github.com/cfmleditor/cfmleditor-lsp/pull/190).

## Baseline and measurement boundaries

The merged baseline is `8ad2bba2e6314383c23c3aff71a1660ab5df70c1`. Its tracked
tree is identical to the validated PR head `c8fee8e`. Both Masa configurations
were rescanned with a binary built from the merge commit: findings match the
previous reports **per entry**, and indexed/scanned/accepted counts are unchanged.
Other project figures below are the final PR-head corpus captures, reused because
the merge has an identical tree; they were not rescanned for this document.

Masa is pinned to `696383140578f8dea3ece26f80cd7bfb370ddf0f` (7.6.1).
“Configured” uses [masacms.json](scripts/corpus/masacms.json). “Automatic mappings”
removes **only** its manual mappings: bean paths, startup files and component
resolver configuration remain supplied. This is not a zero-configuration scan.
Wheels vendor coverage uses the previously pinned `ef04365` source.

| Project / scan | Indexed and scanned files | Unresolved | Accepted |
|---|---:|---:|---:|
| Masa, configured | 897 | 12,288 | 24,227 |
| Masa, automatic mappings | 897 | 13,450 | 19,674 |
| Wheels, presets, vendor included | 1,195 | 3,275 | 46,148 |
| Wheels, no presets, vendor included | 1,195 | 8,929 | 39,574 |
| ContentBox, presets | 724 | 2,705 | 14,364 |
| Lucee | 3,786 | 1,976 | 37,134 |
| TestBox, presets | 146 | 286 | 3,853 |
| ColdBox, presets | 664 | 1,403 | 19,966 |
| FW/1, presets | 305 | 409 | 1,643 |

The default six-project scans total **6,796 / 14,690 unresolved** with / without
presets. Default Wheels covers only 45 files (17 / 62 unresolved); do not substitute
it for the vendor-inclusive measurement or add both into the same total.

These are diagnostic-entry counts. Broken-inheritance entries can summarize
multiple unchecked calls. Accepted plus unresolved counts are filtered CLI totals,
not all extracted calls or a resolution-success percentage. The prior raw captures
were 60,921 for Masa and 67,931 / 67,484 for Wheels; no new raw capture was made
for this classification. Masa excludes ten shipped vendor CFML files; absent
TestBox/DocBox dependencies are not replaced with stubs.

## What already resolves

The merged work covers literal Application mappings through bounded includes and
parent constructors, physical non-root CFConfig defaults and explicit editor
precedence. Known callers retain their application mappings while traversing
library source; separate applications remain isolated.

It also covers literal bean registrations and aliases, supplied-root DI/1
lifetime/property/constructor policy, source-backed legacy FW/1 controller
wiring, finite startup service loops, explicit record-member assignments,
uniform collection-element returns and supported factory/fluent wrappers.
Wheels Mapper/Controller/view/Global integration and observed super aliases use
the real loader, original definitions and signatures. Literal root factories and
public property hops resolve when their source contracts are provable.

These features are partial contracts, not blanket typing by framework name.
Ordinary saves now retain immutable bean/DI discovery while relevant dependency
changes refresh it. That review work changes performance, not corpus findings.
See [RESOLUTION-GAPS.md](RESOLUTION-GAPS.md) for implementation history and proofs.

## Measured diagnostic classification

Each finding belongs to exactly one row. “Unknown return” is the explicit
`has no component return type` reason; “missing chain origin” means the first
method in a chain was not found. Component-chain failures include summaries
beginning `base component`, `extends ... chain breaks` or `calls a component
whose chain breaks`. A missing method is a lookup result, not proof of a runtime
defect. These labels describe where resolution stopped, not its established cause.

| Failure stage | Masa configured | Masa automatic mappings | Wheels presets, vendor | Wheels no presets, vendor |
|---|---:|---:|---:|---:|
| Unknown receiver | 10,982 | 11,039 | 2,280 | 7,675 |
| Unknown return | 628 | 363 | 101 | 297 |
| Missing chain origin | 20 | 16 | 15 | 55 |
| Missing component | 124 | 1,277 | 8 | 2 |
| Broken component/inheritance chain | 0 | 247 | 14 | 14 |
| Unqualified lookup | 498 | 507 | 828 | 864 |
| Missing inherited method | 23 | 1 | 26 | 21 |
| Missing method on known component | 12 | 0 | 3 | 1 |
| Other | 1 | 0 | 0 | 0 |
| **Total** | **12,288** | **13,450** | **3,275** | **8,929** |

Masa's one “other” entry is `not found in parent component`. Its unknown-receiver
group is **89.4%** of configured findings. Mapping discovery matters, especially
without manual mappings, but does not explain most of the configured remainder.
The drop in unknown-return counts without mappings is not an improvement: some
calls stop earlier at missing components instead.

The largest configured Masa unknown-receiver groups are:

| Receiver | Findings |
|---|---:|
| `$` | 1,484 |
| `variables.$` | 1,149 |
| `rc.$` | 729 |
| `rc.contentBean` | 472 |
| `arguments.event` | 408 |
| `arguments.bean` | 399 |
| `arguments.contentBean` | 396 |
| `arguments.feedBean` | 267 |
| `dbUtility` | 248 |

These counts are exact receiver slices, not guaranteed fix yields. A receiver
name can combine conditional assignments, missing request context, arbitrary
arguments and genuine dynamic values. The following semantic categories may
overlap; their counts must not be added to the stage table.

## Gap categories and proposed fixes

### 1. Application context and automatic mappings — priority 1

**Evidence:** Removing Masa's manual mappings raises missing-component entries
from 124 to 1,277 and creates 247 component-chain summaries. The CLI identifies
`mura.cfobject` as an unresolved base in 100 files, covering 1,435 unchecked
inherited calls; these calls are not 1,435 additional diagnostic entries.
The real mapping exists in `core/appcfc/applicationSettings.cfm:220`.
Request-blocking Applications inside libraries can govern direct scans even
when a runtime caller would use the main application's mappings.

**Proposed fix:** Trace representative failures through the actual Application
and include chain before broadening discovery. Add an explicit executing-application
context for standalone library scans if source cannot establish it. Extend only
proven static path/parent/include forms. Model a physical CFConfig `/` entry as
a default lookup root with documented precedence, rather than a named namespace;
`internal/path/server_mappings.go` currently excludes it. Root CFConfig support
is a confirmed implementation boundary, not a measured cause of Masa findings.

**Validation:** Two applications sharing a library must keep different mappings;
direct scans must not silently inherit an unrelated parent. Test root/default
precedence, mapped parent declarations, cycles, conflicting paths and config
creation/deletion. Compare target URIs as well as findings.

### 2. Automatic DI roots and subsystem ownership — priority 1

**Evidence:** The existing DI policy models supplied bean roots and documented
lifetimes, but automatic root discovery and subsystem local/parent precedence
remain open. Modern FW/1 defaults to DI/1 over model/controllers; Masa's embedded
FW/1 is 1.2 and the separate framework corpus is 4.3.2. No count in the stage table
has yet been attributed specifically to automatic DI-root discovery.

**Proposed fix:** Discover literal default/custom diLocations for a verified
FW/1/DI/1 factory. Associate roots with their owning application/subsystem and
implement local-before-parent lookup. Follow documented delegated Application
layouts and literal base/controller directories. Preserve manual-factory mode,
custom engines, constants, exclusions, explicit overrides and transient policy.

**Validation:** Parent factories cannot see subsystem-local beans; unrelated
factories must not share aliases. Setter/property injection remains singleton-only
where DI/1 requires it; the legacy containsBean controller fallback stays distinct.
Support literal layouts without inferring computed configuration.

### 3. MuraScope and request/view propagation — priority 1

**Evidence:** `$`, `variables.$` and `rc.$` account for 3,362 unknown-receiver
findings. Sources include a conditional getBean/init assignment in
`admin/assets/js/frontendtools.js.cfm:13` and the request-context assignment
from `request.event.getValue('MuraScope')` in `admin/Application.cfc:415`.
`MuraScope.init` delegates to `super.init`; its parent returns itself.
Many views retrieve the scope from an externally mutable event slot.

**Proposed fix:** Separate verified factory/init self-return chains from
request-event data propagation. Follow source-proven scope creation through
bounded controller/view/include edges with request ownership. Propagate an event
slot only from proven writes/defaults with no conflicting replacements. Improve
inherited init self-return binding where a minimal regression establishes a gap.

**Validation:** Conditional initialization without a proven incoming value,
arbitrary setValue writes, caller-supplied event data and request replacements
must remain unknown. Never assign MuraScope solely because the variable is
`$` or the string key is `MuraScope`. Measure each source shape separately.

### 4. Record members, argument aliases and branch flow — priority 1

**Evidence:** All 472 `rc.contentBean` findings lack a receiver identity.
In `admin/core/controllers/carch.cfc`, assignments come from getcontentVersion,
getActiveContent and getBean/loadBy/set chains; the update branches write
`arguments.rc.contentBean` and later read `rc.contentBean`.
Explicit static writes already work, but branch merges, wrapper returns,
aliases and container replacement can prevent settlement.

**Proposed fix:** Normalize shorthand references only to a proven argument/local
root. Preserve full member paths and infer the member at a use site when all
reaching assignments have the same component contract. Add bounded struct-literal
member and nonescaping alias support. Resolve producer returns before treating
the consumer member as a separate problem.

**Validation:** Conflicting branch types, missing writes, root shadowing, alias
escape, parent replacement, dynamic keys and mutators must withhold inference.
A container must not acquire the type of one of its members.

### 5. Collection, lazy and argument-sensitive returns — priority 1

**Evidence:** Masa has 628 explicit unknown-return findings. Examples include
24 getBean → loadBy chains, 23 globalConfig → getFileDelim chains and
19 getRBFactory → getKey chains. `settingsManager.getSite` returns keyed
`variables.sites` entries through try/catch, lazy initialization and a default
fallback (`core/mura/settings/settingsManager.cfc:537–564`).
Uniform collection inference already exists; this richer source shape needs
separate proof. loadBy and event getters can return different kinds of values.

**Proposed fix:** Extend collection-element proof to guarded/lazy initialization
only when every reachable write and return agrees. Follow verified wrapper
contracts and literal factory arguments, retaining bounded recursion. Keep
component, primitive, record and collection-element contracts distinct.

**Validation:** Mixed elements, unknown writes, externally supplied collections,
replacement values and polymorphic loadBy options remain unknown. Recursive
wrappers need a concrete return anchor. Do not apply a universal self-return
contract to loadBy or a keyed getter.

### 6. Untyped parameters and callback arguments — priority 2

**Evidence:** Masa has 408 unknown `arguments.event`, 399 `arguments.bean`,
396 `arguments.contentBean` and 267 `arguments.feedBean` findings.
Some event callbacks receive a framework-created event; others are generic APIs.
An example is `core/modules/v1/deprecation/model/handlers/handler.cfc:onLogDeprecation`.

**Proposed fix:** Prefer declared contracts. For verified callback registration
and dispatch, derive argument identities from the source producer. Consider
bounded call-site specialization for private/local helpers with consistent callers;
keep shared indexed signatures independent of one caller's inferred arguments.

**Validation:** Different callers, overridden dispatch, argumentCollection,
generic beans and unknown callback registration must not force one type.
Source-defined callback contracts take priority over parameter-name guesses.

### 7. Include scope and unqualified methods — priority 2

**Evidence:** Masa has 498 unqualified lookup findings and 248 unknown
`dbUtility` receivers, including `core/mura/dbUpdates/5.0.594.cfm`.
Wheels has 828 unqualified findings with presets. Some are include-provided
helpers; others need runtime loaders, closures or application state.

**Proposed fix:** Classify representative sites by their actual includer/loader.
Carry source-proven helper and variable scope through literal includes and verified
public mixin plans. Investigate closure-local binding separately from file scope.

**Validation:** Computed includes, conditional loaders, overwritten helpers,
inaccessible methods and conflicting contexts cannot create blanket definitions.
Confirm original source targets and required arguments.

### 8. Wheels runtime model/controller factories — priority 2

**Evidence:** The vendor scan has 1,000 unknown `_controller` findings with
presets. SecurityDefaultsSpec creates it using
`application.wo.controller("dummy")`; other specs use g.controller with literal
names. No-preset scans also expose model → create/findOne/where chains after
the real Global method becomes visible.

**Proposed fix:** Follow argument-sensitive model/controller names through verified
runtime path configuration and literal factory wrappers. Select actual application
components, then preserve their inheritance and loader contracts. Model plugin
and package override priority only from a proven active configuration.

**Validation:** Different names produce different target components; literal
missing names still report errors. Computed names, competing packages and unknown
plugin activation stay unresolved. Do not type every controller as the generic
base or suppress newly exposed model-return gaps.

### 9. Missing dependencies, ORM methods and genuine missing methods — priority 3

**Evidence:** Masa's 124 missing-component entries include testWidget (64),
contentGateway (31) and cfide.adminapi.datasource (10). contentGateway startup
registration has engine-dependent targets. ContentBox has 103 known-component
missing-method findings; addPermission on cbRole accounts for 66.
Some generated ORM relationship methods need schema/source proof, while missing
modules, engine APIs and historical fixture errors may be outside scan coverage.

**Proposed fix:** Audit component existence, configured roots, versions and
engine-dependent registrations before adding inference. Add ORM relationship
methods only from persistent relationship metadata and the engine's documented
contract. Record unavailable dependencies and intentional invalid fixtures.

**Validation:** Preserve genuine missing-method diagnostics, version boundaries
and relationship ambiguity. Adding a runtime dependency changes coverage and
must be reported as a separate comparison, not a resolver-only reduction.

### 10. Builtin/Java/engine adapters and arbitrary runtime mutation — priority 3

**Evidence:** Lucee has 49 explicit builtin-member findings, including query and
file handles. Wheels retains computed database/engine adapters and Java returns;
`migration.adapter` is no longer incorrectly treated as Migration.
Masa's event.getValue can return supplied defaults or an empty string as well as
stored values (`core/mura/event.cfc:113`).

**Proposed fix:** Expand versioned builtin/Java return metadata where a stable
contract exists. Use literal engine/adapter configuration to select a concrete
implementation only when justified. Offer explicit user contracts for irreducibly
dynamic values rather than inventing a component type.

**Validation:** Different engines/versions, primitive defaults, unknown adapter
selection and arbitrary runtime copying remain distinct. A lower diagnostic count
is not success if it accepts calls on the wrong component.

## Other corpus priorities

| Project | Largest measured signals | First investigation |
|---|---|---|
| ContentBox | 2,048 unknown receivers; 119 missing chain origins; 177 missing inherited methods | Audit ORM/member flow and missing module/helper contracts, including cbMessageBox; separate historical patches from current application code. |
| ColdBox | 1,014 unknown receivers; 213 unknown returns | Inspect fluent/cache/provider contracts, interceptor/callback records and source-declared runtime mixins. |
| FW/1 | 224 unknown receivers; 93 unknown returns; 36 broken-base summaries | Automatic DI ownership, request-record flow and fluent IOC declaration wrappers; absent MXUnit fixtures remain separate. |
| Lucee | 1,063 unknown receivers; 557 unqualified lookups; 49 builtin-member gaps | Include-provided admin helpers, collection-element identities and versioned builtin/Java contracts; retain intentionally invalid tests. |
| TestBox | 176 unknown receivers; 73 unqualified lookups; 16 unknown returns | Generic runners/callbacks, helper dispatch and engine response handles; preserve arbitrary actual/target values. |

## Five-step implementation sequence

1. **Baseline and classification:** retain this merged baseline, both Masa modes,
   the fourteen other scans, source examples and explicit coverage boundaries.
2. **Prove the first shared cause:** trace representative automatic mapping
   failures and MuraScope/request-member producers using explain and minimal
   fixtures. Choose a fix from evidence, not receiver-name frequency alone.
3. **Implement configuration/DI ownership fixes:** handle confirmed static
   discovery gaps and automatic/subsystem roots in bounded, independently
   reviewable batches. Reuse existing DI policies and application isolation.
4. **Implement producer-to-consumer inference:** address self/wrapper returns,
   lazy collection elements and branch/member flow, then tackle Wheels factories.
   Callback and include work follows where the measured examples justify it.
5. **Validate and publish each batch:** prove regressions fail without their fix;
   check source targets/signatures and negative cases; run build, vet, short tests,
   race tests and lint; compare every finding per entry, accepted counts and
   indexed/scanned coverage. Capture raw calls when parser extraction changes.
   Audit all additions and check CI on the published commit.

The first implementation candidate is a source-proven MuraScope init/request
propagation path alongside the automatic-mapping investigation. Its exact yield
is unknown until the individual failing source shapes are reproduced. Automatic
DI roots/subsystems remain a separate cross-framework feature, not an assumed
explanation for every Masa receiver.

## Reproduction and evidence

Build the CLI from the baseline. Copy masacms.json to the Masa checkout's
.cfmleditor.json, preserving the original, and run:

```sh
cfmleditor-lsp unresolved --json /path/to/MasaCMS > masa-configured.json
```

Repeat after removing only the mappings property; restore the original config.
For Wheels, use both checkout and vendor/wheels as roots and repeat with/without
the same framework presets used by the corpus capture.

Classify each reason in this order: no component ref; no component return type;
chained on; component does not exist; base/component-chain summary; no qualifier;
not found in extends chain; method not found; remaining reasons. Count findings,
not the number of calls mentioned inside a grouped reason.

Session evidence is under `/workspace/gap-classification` (fresh Masa reports,
logs and scan script) and `/workspace/pr-review-validation` (fourteen archived
corpus reports, coverage.json and prior validation). These workspace artifacts
are not portable repository dependencies. The tables and source paths above
retain the planning evidence in this document.

## First implementation batch (PR #192)

The tables above remain the merged PR #190 baseline. The first fixes now follow
configured tag factory chains to concrete final types and propagate exact record
member producer/self-return contracts. Configured Masa is **11,881 unresolved /
24,669 accepted** (net 407 fewer findings); automatic mappings are **13,051 /
20,099** (net 399 fewer), with 897 indexed/scanned files in each mode. Wheels
vendor/presets improves by twelve findings to 3,263; thirteen other comparisons
remain identical. Configured Masa preserves 60,921 raw calls per entry.

Categories 3 and 4 gain these specific source shapes. Request/event propagation,
untyped callbacks and polymorphic producers remain unresolved; this is not a
blanket MuraScope or contentBean contract. See the final batch in
[RESOLUTION-GAPS.md](RESOLUTION-GAPS.md) for boundaries and evidence. Session
validation is in `/workspace/receiver-validation`.

## Second implementation batch (PR #192)

Category 4 now recognizes a bounded no-argument optional-getter path to a
startup-proven shared bean. Masa's `globalConfig()` gains a receiver without
assigning its property-getter calls an unconditional component return.
Configured Masa is **11,842 unresolved / 24,709 accepted**; automatic mappings
are **13,020 / 20,131**, retaining 897 indexed/scanned files. This batch removes
39 findings in each mode, exposing eight missing component paths in automatic
mode. Fourteen other comparisons remain identical per entry, and Masa's 60,921
raw calls are unchanged. See [RESOLUTION-GAPS.md](RESOLUTION-GAPS.md) for the
contract, rejection cases and remaining producers. Session evidence is under
`/workspace/optional-return-validation`.

## Wrap-up and loose ends (follow-up to merged PR #192)

PR #192 merged at its second-batch head while this batch was unpublished.
The producer/root fixes and this wrap-up belong to the follow-up PR.

This section supersedes the implementation status above; the original tables
remain historical measurements. The current batch adds call-specific producer
proof, scalar-call return correction, argument-preserving collection chains,
owned array element inference, mixed tag/cfscript bodies, and literal root `/`
mappings from Application.cfc and CFConfig. It does not close every category.

### Remaining work

| Area | Current coverage | Loose end and next proof |
|---|---|---|
| Application mappings | Literal root defaults, named/relative precedence and application overrides now resolve. Unknown root writes suppress stale defaults, including struct-literal replacement. | Automatic mode still misses source/runtime aliases and external mappings. Reproduce each missing base in its nearest Application context; do not map `mura` from a folder-name guess. |
| DI ownership | Previously supported registrations, aliases and caller isolation remain. | Automatic DI roots and subsystem ownership need explicit source/configuration proof and separate application fixtures. This batch does not add them. |
| Request/view propagation | Proven member producers and self-updates carry their concrete returns. | Controller-to-view `rc.$`, event slots, callback records and cross-request values still lack a proven origin. Track documented framework boundaries before propagating them. |
| Argument-sensitive producers | Omitted defaults, literal/supplied constructors, `this`, finite argument forwarding, object/presence guards and supported branch flow now specialize a call. | Arbitrary caller identifiers, dynamic argument bags, computed defaults of unknown value, conflicting returns and escaped scopes remain unknown. Add call-site reaching-definition proof before carrying caller variables. |
| Lazy/shared caches | Existing proven collection contracts remain supported; owned arrays and structs share element checks. | Rich keyed/lazy getters still need all writer/initialization paths to agree. `settingsManager.getSite`'s try/catch cache is proven (see below); `settingsBean.getRazunaSettings`'s shared-field cache is the remaining concrete fixture. Do not restore a receiver-class guess to silence these findings. |
| Tags/control flow | Tag and mixed cfscript method plans now remain connected. Query output names can invalidate a returned local through finite attribute bags. | Unsupported controls/tags, computed output names, record aliases, uncertain mutation and exhausted bounds withhold inference. Extend one source shape at a time with negative tests. |
| Callback/parameter contracts | Declared component types and existing framework contracts continue to work. | Untyped bean/feed/event parameters require verified registration/call-site contracts; arbitrary TestBox actual/target values should remain dynamic. |
| Includes and unqualified calls | Existing static include/helper discovery remains. | Scope ownership for runtime includes and helpers needs a provenance fixture; a matching method name alone is insufficient. |
| Wheels factories | Existing literal root factories, associations, model finders and loader integration remain. | Runtime model/controller path construction needs documented configuration/plugin root selection. This batch adds no blanket factory contract. |
| External APIs/dependencies | Existing builtin and Java dynamic contracts are preserved. Known concrete receivers still report missing methods. | Missing dependencies, versioned builtin members, ORM additions and intentional invalid fixtures need separate classification. Arbitrary reflection/runtime mutation cannot yield a static CFC identity without a contract. |

### Validation and operational follow-up

Producer plans contain lexical source, not cached inferred caller types. Source
bytes refresh their plans, and supplied constructors use the caller's directory.
Inference is bounded: 4,096 body tokens/tags, eight nested calls, shared work
budgets and sixteen loop/settlement iterations. Hitting a boundary loses proof;
it must not manufacture a receiver.

All nine targeted disabled-fix checks must fail behaviorally and pass restored.
Corpus findings are compared per entry, with additions retained in the evidence
rather than hidden by a net total. Raw parser call records are checked separately
from accepted/unresolved CLI totals.

The existing route/document-link performance assertions are timing-sensitive.
Run corpus scans separately from tests, with `GOMAXPROCS=2` and package
parallelism `-p 1` for this environment's full short/race checks. Those settings
make local validation reproducible; they do not change the tests or establish a
source fix for timing variability. Source-plan interpretation adds work, so
representative cold/warm latency and memory measurements remain a follow-up
before broadening inference. CI is checked on the published commit separately.

For future sessions, run `bash scripts/dev-env.sh check`, use setup if needed,
and source `target/dev-env/env.sh`. Sandbox proxy reachability must be checked in
the approved execution context, as described in AGENTS.md; cached local tests
and exact-head CI remain separate evidence.

The next focused batch should start with the two lazy-cache fixtures above and
call-site variable provenance. Those are concrete producer gaps that can unblock
existing member flow. Request/view and callback propagation follows once its
framework handoff is proven; receiver-frequency totals alone are not a fix plan.

### Final measurements for this batch

| Scan | Files | Previous unresolved | Current unresolved | Accepted now | Batch removed / added |
|---|---:|---:|---:|---:|---:|
| Masa configured | 897 | 11,842 | 11,701 | 24,868 | 208 / 67 |
| Masa automatic | 897 | 13,020 | 12,886 | 20,270 | 201 / 67 |
| Wheels presets, vendor included | 1,195 | 3,263 | 3,263 | 46,162 | 0 / 0 |
| Wheels no-presets, vendor included | 1,195 | 8,929 | 8,929 | 39,574 | 0 / 0 |

These are filtered diagnostic/accepted counts, not a raw call success rate.
Both Masa modes retain 897 files; all fourteen other corpus comparisons retain
their indexed/scanned coverage. Cumulative Masa reductions against merged PR #190
are 587 configured and 564 automatic.

### Known corpus coverage losses to revisit

ColdBox with presets has **55 removed / 55 added** findings, so its net total
stays 1,403. The new receiver-stage findings affect `event1`/`event2`, `e` and
`this.event` in integration/context fixtures. Its BaseTestCase
`getMockRequestContext` accepts an optional decorator and constructs mocks through
MockBox; the producer interpreter does not prove those `isNull`/mock factory
branches. The related `execute()` wrapper therefore loses its receiver at some
uses. This is a known inference limitation at these calls, not evidence that
those application methods are missing. A focused no-argument/default-decorator
contract must prove both the original class and the supplied-decorator boundary.

Without presets, ColdBox removes 43 and adds five findings (net 38 fewer). The
five additions are Controller `getRenderer` chains in FrameworkSupertype/Renderer:
the lazy WireBox-backed field is not proven by this interpreter. Preserve the
source-backed documented/DI receiver when that initialization contract is proven;
do not substitute Controller itself as the returned class. These cases join the
lazy-cache follow-up above. Per-entry additions are retained even when a project
improves overall.

## The getSite cache, and Mura's service loop (follow-up to merged PR #195)

`settingsManager.getSite`'s lazy try/catch cache needed no new inference. The
existing collection contract already proves it: every write to `variables.sites`
is the `cfparam` empty struct or the `builtSites` struct `setSites` rebuilds,
whose elements are each a copy of a cached element or `variables.DAO.read(...)`,
so `getSite` returns what `read` does. `TestALazyCacheReturnsItsElementType`
pins the shape. On MuraCMS (the corpus's `MSU-NatSci_MuraCMS`, scanned with
`scripts/corpus/masacms.json`) it returns `settingsBean`. A call on that bean is
then accepted as dynamic, correctly, because `mura.bean.bean` defines
`OnMissingMethod`.

What blocked `getSite` in Mura was its receiver. Mura appends a legacy service
under a condition, `variables.serviceList=listAppend(variables.serviceList,'advertiserManager')`,
before the startup service loop. The non-literal assignment dropped the list, so
none of the 25 `application.*` services was typed. A `listAppend` of a literal
onto the same known list now keeps it, with the appended name included: a
binding says what the loop assigns a name, not that it does. Masa's startup
template evidently lacks the append, which is why its configured baseline
already typed these services.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| MuraCMS, configured | 12,575 | 11,290 | 1,293 / 8 |
| Six other corpus projects | | | 0 / 0 each |

Seven of the additions are deeper unknown returns the typed receiver now reaches
(`contentManager.getActiveContent`, `userManager.getCurrentUser`,
`settingsManager.save`). One is a genuine missing method:
`client/api/soap/v1/user.cfc:70` calls `userManager.readByEmail`, which Mura's
user package does not declare.

`explain` now builds its resolver and parse through the unresolved scan's own
`unresolved.NewResolver` and `unresolved.Parse`. It used to build its own
without `beanPaths` or the setter and constructor policies, and reported these
receivers unresolved while the scan accepted them.

## MuraScope: why `$` stays unknown

Measured on MuraCMS with `scripts/corpus/masacms.json`, after the service-loop
fix above. `variables.$`, `$` and `rc.$` are 2,910 of 10,070 unknown-receiver
findings: 1,344 in display modules under `core/modules/v1`, 889 in admin views,
and most of the rest in `standardEventsHandler.cfc` and
`contentRendererUtility.cfc`. Typing them soundly needs three links, and none
is provable from source:

1. **The value.** Both bulk origins read an event slot:
   `request.context.$=request.event.getValue('MuraScope')`
   (`admin/Application.cfc:384`) and
   `variables.$=variables.event.getValue("muraScope")` (`contentRenderer.init`).
   `servletEvent.getValue` reads the global `request` scope and returns `""`
   for an unset key, and any dynamic-key write to `request` can replace the
   slot. The provable origin, `getBean('$').init()` (`$` is a literal alias of
   `MuraScope`), covers a handful of locals.
2. **Into the display modules.** The renderer includes them through computed
   paths (`#filePath#themes/#theme#/…`, `#theIncludePath#/modules/…`), so there
   is no static include edge to carry `variables.$` along.
3. **Into the admin views.** FW/1 renders views through computed includes too,
   so `rc.$` never reaches a view statically.

Two project contracts were measured on a scratch copy of the config, not committed:

| Contract | Findings | Removed / added |
|---|---|---|
| `getValue("muraScope")` is `mura.MuraScope` | 11,290 → 11,251 | 113 / 74 |
| A receiver named `$`, `variables.$` or `rc.$` is `mura.MuraScope` | 11,290 → 8,365 | 3,126 / 201 |

The name contract's drop is mostly not proof. `MuraScope` defines
`onMissingMethod`, so most calls on it are accepted as dynamic once `$` is
typed, and the additions are deeper unknown returns (`$.currentUser()` 86,
`$.content()` 34). It is also what this plan's rule forbids as inference.
Either contract is a project's own decision to make in its config; the
resolver does not make it.

## Untyped arguments: what the caller rule can reach

Measured on MuraCMS with `scripts/corpus/masacms.json`. 2,772 unknown receivers
are `arguments.*`. The plan's rule (type a parameter from consistent callers
only in a private or local helper) reaches 6 of them: 2,621 are in public or
default-access methods and 145 are outside any function.

| Group | Findings | Example | Provable |
|---|---:|---|---|
| DAO, gateway and manager methods | 1,416 | `contentDAO.create(contentBean)`, `settingsDAO.create`/`update(bean)`, `feedGateway.getFeed` | Only under a closed-workspace assumption: each tends to have one in-workspace caller (`settingsManager` calling `variables.DAO.create(bean)`), but a public method can be called from outside it |
| Framework callback arguments | 596 | `standardEventsHandler.doAction(event, $)` | No: Mura invokes them by dynamic dispatch, and `$` is the MuraScope case above |
| Other | 760 | `renderer`, `item`, `bundle` | Mixed |

Typing the first group means letting the agreement of a public method's
workspace callers decide its parameter's type, which this plan's rule keeps
shared signatures independent of. If it is done, it belongs behind an explicit,
default-off closed-workspace setting, measured on its own.

## Unknown returns on MuraCMS

Measured with `scripts/corpus/masacms.json`: 552 findings say a method has no
component return type. Two shapes were provable and are fixed here:

- **A getter of a startup-typed shared variable.** `getPluginManager()` and
  `getServiceFactory()` in `mura.cfobject` are `return application.pluginManager`
  and `return application.serviceFactory`. A function whose whole body is a
  return of a shared-scope variable now returns what the startup templates
  assign it, the same lookup and policy that already type the variable as a
  receiver. Only that shape: a call anywhere in the body can write the scope,
  so a guarded or computed body (`TestAbsentOptionalArgumentBoundaries`) still
  has no answer. Reading shared variables anywhere in a body was tried first
  and broke those boundaries.
- **A parenthesised return.** `return( this );` is how `mura.jsonSerializer`
  ends every fluent definer, so `asString()`, `asInteger()` and the rest had no
  return type. A fully parenthesised expression is now read as the expression.

Together: 11,290 → 11,186 (104 removed, none added); the six other corpus
projects are unchanged.

What remains is mostly unprovable from source:

| Group | Findings | Why it stays |
|---|---:|---|
| Lazy getters on a bean's `variables.instance` struct: `configBean.getClassExtensionManager` (47), `settingsBean.getContentRenderer` (22), `settingsBean.getRBFactory` (22) | ~90 | The bean's generic `setValue` writes `variables.instance["#arguments.property#"]`, and any caller can name the field |
| `servletEvent.getValue` and `MuraScope.event` | ~52 | An event slot, as in the MuraScope section above |
| `getCurrentUser()` | ~54 | Lazily fills `request.currentUser`; any `request` write can replace it |
| `getBean(...)` with a computed name | ~46 | The bean is chosen at run time |
| `loadBy` on beans | ~38 | Polymorphic by its options, as category 5 says |

## Unqualified lookups: directory-listing includes

489 findings on MuraCMS (with `scripts/corpus/masacms.json`) are bare calls
that nothing in reach declares. About 114 were `getDbType`, `dbTableColumns`
and `dbCreateIndex` in the `dbUpdates/*.cfm` scripts. `configBean.applyDbUpdates`
lists the directory beside itself
(`<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#dbUpdates" filter="*.cfm" name="rsUpdates">`)
and includes each file it finds (`<cfinclude template="dbUpdates/#rsUpdates.name#">`),
so every update script runs in `configBean`'s variables scope. That shape is now
an include edge to each listed template (`parser.directoryIncludes`,
`Resolver.includeTargets`). A recursive listing, a computed directory or filter,
a parent directory, an include prefix naming another directory, and a listing
inside a comment are not.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| MuraCMS, configured | 11,171 | 11,032 | 141 / 2 |
| MuraCMS, no config | 20,745 | 20,626 | 121 / 2 |
| Five other corpus projects | | | 0 / 0 each |

The additions are `getClassExtensionManager()` chains in `dbUpdates/5.2.2655.cfm`,
the lazy-field group under unknown returns.

Most of what remains is `rbKey` (253) and `buildURL` (37) in the admin views.
Mura's admin embeds FW/1 1.x: `framework.view()` includes each view through a
computed path inside the framework object, and `admin/Application.cfc` is that
object (it extends `framework`), so a view calls its functions unqualified.
Typing it needs the FW/1 preset to take a view's implicit base as the governing
`Application.cfc` rather than the `framework.one` stub, and the project config
to opt into `fw1`; the second shifts the documented Masa baseline.

## FW/1 views run inside the Application

FW/1 includes a view or layout inside the framework object, and an
`Application.cfc` that extends the framework is that object, so a view calls
the Application's own functions unqualified. The `fw1` preset gave views the
`framework.one` stub as their base, which has FW/1's functions but none of the
Application's. It now marks the base `parser.ApplicationBase` + `framework.one`.
The resolver (`applicationBase`) takes the governing `Application.cfc` when that
file's chain declares `view()` and `buildURL()`, the sign it is the framework
instance, and the stub otherwise.

Mura's admin embeds FW/1 1.x as `admin/framework.cfc`, and every admin view
calls `rbKey()`, which `admin/Application.cfc` declares. MuraCMS with
`scripts/corpus/masacms.json`, which now opts into `"frameworks": ["fw1"]`:

| Build | Findings | Removed / added against the config without the preset |
|---|---:|---:|
| Preset as it was | 10,905 | 380 / 253: every `rbKey` became "not found in extends chain" |
| Preset with the Application base | 10,652 | 380 / 0 |

The removals are `rbKey` (253) and `buildURL` (37) in admin views, and 88
`variables.fw` calls in controllers (`redirect`, `setView`) that the preset's
existing `fw` resolver types; both methods exist in Mura's own FW/1. FW/1's
repository with the preset is unchanged (409), and no project changes without
the preset.

**The configured baseline moves with this.** `masacms.json` opting into `fw1`
means a configured scan of Masa or MuraCMS now runs the FW/1 preset: compare a
later configured measurement against 10,652 on MuraCMS, not 11,032, and rescan
Masa's configured baseline before comparing to the figures earlier in this
plan.

## Lazy-field getters: why they stay unknown

`configBean.getClassExtensionManager()`, `settingsBean.getRBFactory()` and
`settingsBean.getContentRenderer()` each fill a `variables.instance` field when
it is not yet an object and return it. Every one of those fields can be written
with any value from outside the getter, so the field's type is not in the
source:

- `configBean.setValue` writes `variables.instance[property]` directly; `init`
  copies the whole config struct through it (`setValue(prop, arguments.config[prop])`,
  line 261), so field names come from the site's settings, and its
  `onMissingMethod` turns any `setX(value)` into `setValue("x", value)`.
- `settingsBean.set()` copies every column of a query or key of a struct
  through `setValue`, and `setRBFactory()` is a public setter.

This is the plan's rule for dynamic writes, and they stay unknown.

## Missing components on MuraCMS

With `scripts/corpus/masacms.json`, 125 findings name a component that does
not exist. Two causes were fixable:

- **A registration that depends on the engine.** Mura aliases `contentGateway`
  to `contentGatewayAdobe` on Adobe ColdFusion and to `contentGatewayLucee`,
  which extends it, otherwise. Registrations of one id that disagree used to
  cancel out; when one target is extended by every other, the id now has that
  common base's type (`Resolver.commonBase`). A method the base declares exists
  under either engine, and one only a subclass declares is still reported.
  Unrelated targets still give no type.
- **Adobe's administrator API.** `cfide.adminapi.*` ships in the engine's CFIDE
  directory, never in a project, so it is treated as an engine component, as
  `com.adobe.coldfusion.*` already was: accepted as dynamic once nothing on
  disk resolves it.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| MuraCMS, configured | 10,652 | 10,587 | 65 / 0 |
| MuraCMS, no config | 20,626 | 20,615 | 11 / 0 |
| Five other corpus projects | | | 0 / 0 each |

The configured removals are the 31 `contentGateway` findings, 23 calls on
`application.contentGateway` (which the service loop types now that the alias
resolves) and 11 `cfide.adminapi` ones. Most of what remains is `testWidget`
(64): `core/tests/specs/mura/core/entities.cfc` registers a model directory at
run time (`configBean.registerModelDir(dir="/muraWRM/core/tests/resources/model")`)
and Mura registers each bean there by its `entityName`. That is one test
spec's run-time registration, not modelled.

## ColdBox: the BaseTestCase loose end

Measured on ColdBox with the `coldbox` and `testbox` presets and the checkout
mapped as `coldbox` (a scratch config; without the mapping `coldbox.system.*`
resolves to the bundled stubs and none of this is exercised): 1,469 findings,
the scale of the 1,403 recorded above.

The receivers lost in the producer batch (`event1`, `event2`, `e`, `this.event`)
come from `BaseTestCase`'s `get()` → `request()` → `execute()` and
`getMockRequestContext()`. Reading those functions turned up four ways a plan
misread its source, each fixed:

- **A second `catch` clause** became an expression statement that ran on into
  the statement after it, so `execute()` lost `requestContext = getRequestContext();`
  and the interpreter kept a stale value. Every catch is now a handler.
- **A missing semicolon** made `request()`'s whole body one statement and lost
  its return. A line break now ends a statement when the next line starts with
  a word that is not an operator, after a token that can end an expression.
- **`this.f()` and `variables.f()`** inside a body read their receiver as a
  variable, unknown without a receiver; they now call the component's own `f()`,
  as a bare `f()` does.
- **A bare chain on a self-returning method** (`set(data).setValidations()`) now
  takes the calling subclass, as a qualified call already did.

Two precedence rules came with them, each needed to keep the corpus from
regressing once plans read more source:

- **A documented `@return` applies** unless the body guards on its arguments.
  `RequestService.getContext()` now has a plan, which hides its returns in a
  `lock` block; the doc type had answered before, while the plan was invalid.
- **A value given members before it is returned is not its component.**
  `execute()` attaches `getRenderedContent()`, `getHandlerResults()` and
  `getRenderData()` to the context it returns; typed as a plain
  `RequestContext`, 13 calls to them were reported missing.

Result: 1,469 → 1,453 (16 removed, none added), every one a `this.event` call in
`InterceptorStateTest`; every other corpus scan is unchanged per entry.
`execute()`'s callers (`event1`, `event2`, `e`) stay unknown, as before: typing
them needs a component type that carries the members `execute()` attaches. The
five no-preset `getRenderer` additions are the lazy WireBox field and remain.

Found on the way, and since fixed: the parser typed a returned local by its
*first* assignment, so `var x = new A(); x = new B(); return x;` was declared to
return `A`, deciding before the interpreter ran. The return now takes the ref
reaching its line (`refReaching`: the latest at or before it, as
`funcScopedRef` reads a receiver), and `catches()`/`noSemicolons()` are back in
`TestProducerPlansReadTheSourceAsWritten`. Three neighbours had hidden the same
bias and came with it:

- **`hasRefFor` skipped any pending call whose variable already had a ref**, so
  `var x = new A(); x = make();` never recorded the second assignment. It now
  skips only a ref for the same line, which is what `appendResolverRefs`
  duplicates.
- **A qualified call took the file's own function of that name**:
  `x = arguments.binder.init()` was the Injector's `init()`, which returns
  `this`. Only a bare, `this.`, `variables.` or `super.` call does now
  (`callsOwnFunction`), and `arguments.x` is not read from the component's
  refs (`baseArgs`).
- **A return settled in the later pass never typed the calls on it**:
  `variables.binder = buildBinder()` had been looked at first. Untyped calls
  are looked at again while returns keep settling (`maxReturnRounds`).

Corpus, per entry: ColdBox 19 removed / 1 added, Mura 20 / 0, ContentBox 2 / 0,
cfwheels 3 / 0, fw1 and Lucee unchanged. The addition, `Future.cfc:66`
(`variables.executor = variables.executor.getNative()`), had been accepted
because the file's own `getNative()` typed it. Two of Mura's removals
(`contentManager.cfc:966`, `trashManager.cfc:14`, both
`pluginEvent = pluginEvent.init(…).getEvent()`) are the receiver reading the
ref its own assignment makes, since `funcScopedRef` admits a ref on the call's
line: pre-existing, and the same effect `Future.cfc:66` relied on before.
Since fixed: such a ref carries `Rebinds` and does not reach a call on its own
line. Mura, against the branch above: 25 removed, 2 added. The additions are
those two lines, reported again against the `MuraScope` they are called on;
the removals are calls after a reassignment that the tag parser had fixed to
the type before it while parsing. Every other corpus scan is unchanged.

The lazy body parse `FuncRefs` falls back to now types its pending calls as
the full parse does, so the editor, after an edit, and a scan agree. With it,
an unscoped argument or local receiver is read only from the function's refs.
Corpus against the commit above: ColdBox 0 removed / 10 added, every other
scan unchanged. All ten are `oBean.getFname()`/`getLname()` in
`FrameworkSuperTypeTest.cfc`: `oBean = target.populate( … )` where `target` is
the closure's own `var target = getInterceptor( "Test1" )`, which nothing
types. They had passed because the component-level `target`, a `$any` mock,
was read in its place.

Not done: assignments on different branches are not compared, so the one
reaching the return by line wins even where another branch assigns something
else. `buildBinder()` is `$any` only because its later branch is.

## ContentBox: measurement and relationship getters

**Measure ContentBox with `cfmigrations` on.** The checkout has no root
`box.json`, so nothing suggests presets. With `coldbox`, `contentbox` and
`testbox` it reports 4,055 findings, 884 of them `table` in
`modules/contentbox/migrations`: cfmigrations' schema builder hands its callback
a `Blueprint`, which the `cfmigrations` preset types by name. With the preset
added it reports 2,919, the scale of the 2,705 recorded earlier, which evidently
had it on. `cborm` is not a preset name.

**A single-valued ORM relationship's getter returns its entity.** A
`many-to-one` or `one-to-one` property names the entity it holds in its `cfc`
attribute, so the generated getter returns one: ContentBox's content items reach
their site through `property name="site" fieldtype="many-to-one"
cfc="contentbox.models.system.Site"`, and `getSite()` had no return type. Only
a persistent component counts. Mura's own beans declare relationships the same
way, but their `cfc` is a bean id (`site`), and typing it as a component made 13
calls on Mura's `site` beans report a component that does not exist.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| ContentBox, `coldbox` + `contentbox` + `testbox` + `cfmigrations` | 2,919 | 2,872 | 47 / 0 |
| ContentBox, no config | 7,807 | 7,766 | 41 / 0 |
| MuraCMS (both), ColdBox (both), cfwheels, fw1, Lucee | | | 0 / 0 each |

**What is left of ContentBox's unknown receivers**, largest first:

- `print` (204): CommandBox task runners (`build/patches/*/Updater.cfc`, the
  archive seeder). CommandBox makes a task runner extend `BaseTask`; nothing in
  the source says which files are run as tasks.
- Loop variables over relationship collections: `for ( var thisContent in
  aRelatedContent )`, `<cfloop array="#prc.comments#" index="comment">`.
  `thisContent`, `comment`, `oRule`, `entry`, `page` and `author` are largely
  this shape, a few hundred findings. A `*-to-many` property holds an array of
  its `cfc`, so the elements are typed, but there is no element type on a
  collection or a loop variable to carry it: the next feature here.
- Closure parameters such as `c` in `newCriteria().…( function( c ) { … } )` and
  arguments typed only by their callers, as on MuraCMS.

## ContentBox: loop variables over entity collections

590 of ContentBox's unknown receivers are loop variables. 421 iterate a `prc.*`
collection in an admin view, which a handler fills: that is the request/view
handoff, unprovable without the framework's routing, and left alone. The rest
iterate a local collection, and a loop variable now holds the collection's
element when the source states it (`Resolver.loopElement`):

- a persistent entity's `one-to-many` or `many-to-many` property, whose
  generated getter carries the element entity (`FunctionDef.ElementComponent`),
  whether the loop calls the getter or reads the property;
- a cborm service bound to an entity, whose `getAll()` returns an array of it,
  except with `properties`, when it returns structs;
- a local variable assigned from either, by its nearest preceding assignment.

Both `for ( [var] x in collection ) { … }` and `<cfloop array="#collection#"
index|item="x">` count, and only for a call inside the loop's body. The element
is the entity *and every component extending it*, as alternatives: an ORM
collection holds subclasses, and ContentBox's subscriber calls
`getRelatedContent()`, a `CommentSubscription` method, on the comment ones.
Loops are found once per file version and cached, since the lookup runs for
every untyped receiver; the scan's cost is within noise.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| ContentBox, presets | 2,870 | 2,821 | 62 / 13 |
| ContentBox, no config | 7,764 | 7,715 | 62 / 13 |
| MuraCMS (both), ColdBox (both), cfwheels, fw1, Lucee | | | 0 / 0 each |

All 13 additions are calls that were already findings, now naming the method
the entity lacks instead of an untyped variable: `build/patches/3.7.0`–`4.2.1`
call `Author.getAPIToken()`/`generateAPIToken()` and `1-0-4` calls
`Page.getRecursiveSlug()`, none of which today's model declares.

## ContentBox: the prc handoff from handler to view

A ColdBox view reads the `prc` its handler action filled, and names neither the
handler nor a type. The handoff is the `setView` call: for a view at
`<module>/views/<name>.cfm`, every action in `<module>/handlers` that calls
`event.setView( "<name>" )` renders it, and `prc.X` holds what each of those
actions last assigned before the call, typed in the handler with the handler's
own rules (`Resolver.viewPrc`, and `viewPrcElement` for a loop over it). It is
an answer only when every rendering action types it and they agree: an action
that does not assign `prc.X` leaves it to a pre-handler, an interceptor or a
layout. A view no `setView` names, such as a partial rendered by `renderView`,
has no handoff.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| ContentBox, presets | 2,821 | 2,750 | 71 / 0 |
| ContentBox, no config | 7,715 | 7,644 | 71 / 0 |
| MuraCMS (both), ColdBox (both), cfwheels, fw1, Lucee | | | 0 / 0 each |

The handler index and the handler parses are built once for the life of the
resolver, as `startupCache` is. Keyed by index generation they were rebuilt all
the time, since lazy indexing moves the generation during a scan, and the scan
was 60% slower; cached, it is within noise.

What the handoff cannot yet reach, of the 421 loop and 226 direct `prc`
findings in views:

- **Partials** (202 of the loop findings): sidebars, pagers and editors that
  another view renders or includes. Their `prc` is their parent view's.
- **Struct fields of a search result** (about 95): `prc.comments =
  commentResults.comments`, where `search()` returns a struct holding a criteria
  `list()`.
- **`xService.list(…)`** (about 54): whether it returns entities or a query
  depends on cborm's configuration.
- **A prc member assigned from an injected service's entity method** (fixed):
  `prc.author = authorService.get( rc.authorID )` was untyped even in the
  handler, while the same call assigned to a local was typed `cbAuthor`. The
  cause was not the WireBox id: any closure in the file (`function(` or `=>`,
  here a `.each()` callback in another action) withheld every record member in
  the file, and the member's ref, recorded with no component, then kept the
  pending call from typing it. A closure now withholds only the members it can
  write (`withholdForClosure`): its own function's and the component's.

  | Scan | Before | After | Removed / added |
  |---|---:|---:|---:|
  | ContentBox, presets | 2,751 | 2,670 | 81 / 0 |
  | ContentBox, no config | 7,643 | 7,576 | 67 / 0 |
  | MuraCMS, ColdBox, cfwheels, fw1, Lucee | | | 0 / 0 each |

  Measured on top of the partials handoff below, which the fix unblocks:
  `prc.author` went from 61 to 19. The same file-wide closure wipe remains in
  `applyCollectionReturns` (`collection_returns.go`), unmeasured.

- **A service bound through its init's argument default** (fixed):
  `ContentService` declares `init( entityName = "cbContent" )` and calls
  `super.init( entityName = arguments.entityName )`, which the ORM rule, reading
  only a literal, did not follow, so `contentService.get()` was `$any` and a
  `prc` member assigned from it, which withholds a `$` component, was reported.
  `boundEntity` now takes the argument's literal default from the service's own
  init(), in script or tag syntax; one with no default binds nothing. A subclass
  passing its own literal (`EntryService`, `cbEntry`) still wins, since the walk
  starts at the service asked about.

  | Scan | Before | After | Removed / added |
  |---|---:|---:|---:|
  | ContentBox, presets | 2,670 | 2,637 | 33 / 0 |
  | ContentBox, no config | 7,576 | 7,543 | 33 / 0 |
  | MuraCMS, ColdBox, cfwheels, fw1, Lucee | | | 0 / 0 each |

  `prc.content` went from 49 to 30. The 30 left are all `content/quickLook.cfm`,
  a separate gap: see "ContentBox: a base handler's variable its subclasses
  inject" below.

## ContentBox: partials

The prc handoff now follows `view()` and `renderView()` as well as `setView`
(ColdBox 7 renamed the second the first). A handler action that returns a
viewlet (`return view( view = "comments/pager", module = "contentbox-admin" )`)
renders that view as setView would, and a partial a view renders
(`#view( view = "authors/editor/sidebar" )#`) reads the prc of whatever renders
its parent, since prc is the request's. A parent view that assigns the member
itself does not hand it on, and a `view()` naming another module renders that
module's view. Computed names (`cbAdminComponent( "editor/sidebar/…" )`, which
wraps `view( view = "_components/#arguments.component#" )`, and the
`*/indexTable` views) are not followed.

This links the partials but resolves only one more finding today
(`prc.widgetService` in `widgets/widgetList.cfm`): what most partials inherit
is a prc member the handler itself leaves untyped. `prc.commentPager_oPaging =
getInstance( "Paging@contentbox" )` and `prc.author = authorService.get( … )`
were untyped in the handler even though the same right-hand sides assigned to a
local were typed: the record-member gap under the handoff section above, which
also blocked the pagers and the author editor's partials. That gap is now
fixed; see there.

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| ContentBox, presets | 2,752 | 2,751 | 1 / 0 |
| ContentBox, no config | 7,644 | 7,643 | 1 / 0 |
| MuraCMS (both), ColdBox (both), cfwheels, fw1, Lucee | | | 0 / 0 each |

## ContentBox: a base handler's variable its subclasses inject

Open; measured after PR #216. Known for short as **the quickLook ormService
gap**, after the view it empties. The largest single cause left in
`contentbox-admin`'s handlers. Listed as gap 11 in RESOLUTION-GAPS.md.

**To pick it up:** read this section, rebuild the ContentBox scratch copy
(`~/corpus/Ortus-Solutions_ContentBox` copied to a scratch directory with
`.cfmleditor.json` `{"workspaceName":"cbox","frameworks":["coldbox","contentbox","testbox","cfmigrations"]}`;
never write into `~/corpus`), run the repro below, write the failing test in
`internal/resolve` (a base component calling `variables.svc.get()`, two or
three subclasses injecting different services), then follow CLAUDE.md's
verification discipline.

**Shape.** `handlers/baseContentHandler.cfc` is an abstract handler. It calls
`variables.ormService` throughout and never declares it; each concrete handler
that extends it injects its own:

```cfml
// pages.cfc
component extends="baseContentHandler" {
	property name="ormService" inject="pageService@contentbox";
// entries.cfc        inject="entryService@contentbox"
// contentStore.cfc   inject="contentStoreService@contentbox"
```

The base file's own header says so ("These properties must be set by the
concrete content type handler"). The three services all extend
`ContentService`, which since PR #216 binds `cbContent`; their own entities
(`cbPage`, `cbEntry`, `cbContentStore`) all extend `BaseContent`
(`cbContent`).

**What it costs.** `receiverComponent` looks a `variables.x` receiver up in the
file's refs, then up its extends chain (step 6), and never down: nothing in
`baseContentHandler.cfc` or `baseHandler.cfc` types `ormService`, so every call
on it is `variable 'variables.ormService' has no component ref`, and every value
read from it is untyped. On the ContentBox scratch copy (presets, 2,637 entries):

- about 50 entries in `baseContentHandler.cfc` itself: the calls on
  `variables.ormService` (`save`, `saveAll`, `populate`, `getAllForExport`,
  `importFromFile`, …) and on what it returned (`oContent`, `original`,
  `prc.oContent`, `prc.oParent`);
- all 31 of `views/content/quickLook.cfm`: its only renderer is
  `baseContentHandler.quickLook()`, which assigns
  `prc.content = variables.ormService.get( … )`, so the prc handoff
  (`Resolver.viewPrc`) has nothing to hand on.

Reproduce: `cfmleditor-lsp explain --root <scratch>/cbox
<scratch>/cbox/modules/contentbox/modules/contentbox-admin/handlers/baseContentHandler.cfc 181 ormService`,
which prints "no ref found in this file — checking extends chain (baseHandler)"
and then "has no component ref".

**Where the fix goes.** A new step in `receiverComponent`
(`internal/resolve/resolve.go`), after the extends chain and before the
`componentResolver` fallbacks, for a receiver the file and its bases never
assign: the components that extend this file (`Index.FilesExtendingName` +
`descendsFrom`, as `withSubclasses` in `loop_element.go` already does for ORM
collections) are asked for their own file-level ref for the name, injected
property included. That needs a subclass's parse (or its indexed property
refs), not just its function index.

**Decisions to make, each with a test that fails the wrong way:**

- **Every subclass or some?** Answer only when every concrete subclass types the
  name. One that does not leaves it to a pre-handler, a mixin or a runtime set,
  and guessing from the others would invent a type. This matches the handoff's
  own rule: an answer only when every renderer types it and they agree.
- **Agreeing or not.** Three different services do not agree. Two answers are
  defensible: the alternatives (`pageService@contentbox|entryService@contentbox|contentStoreService@contentbox`,
  the spelling `withSubclasses` uses, where a method any alternative declares is
  found), or their nearest common base (`ContentService`). Alternatives are
  more permissive (a method only `EntryService` has is accepted in the base
  code, which may be right: such code is often under a type check). The common
  base is stricter and would report it. Prefer alternatives, for consistency
  with `withSubclasses`, and check that `FuncLookup`/`walkChainRest` and
  `boundEntity` cope with an alternatives string on a chain hop
  (`variables.ormService.get( id ).getSlug()` must become
  `cbPage|cbEntry|cbContentStore`, or at least `cbContent`). That is untested
  today and is the likeliest place for the work to grow.
- **Scope.** Only a receiver the file never assigns, and only `variables.` or
  unscoped (a `this.` member is public and the same rule would hold, but nothing
  in the corpus needs it). Only when the file is extended within the
  workspace; a framework base class extended by every app would otherwise ask
  every handler in the workspace. Cap the subclass walk as `withSubclasses` does
  (16).
- **Not ColdBox-specific.** The shape is any abstract component whose
  subclasses inject or assign what it uses, so it belongs in the resolver, not
  in a preset. Measure MuraCMS, ColdBox, cfwheels, fw1 and Lucee too: a
  0 / 0 there is the expected result, and an addition is the thing to explain.

**Measure** with the per-entry sorted diff over the scratch copy and the six
corpus projects, as for the sections above. Expected: about 80 removed in
ContentBox, and `quickLook.cfm` to 0 if the handoff picks up the alternatives;
any added entry is a method one subclass's entity lacks, which is either a real
finding or a reason to prefer the common base.


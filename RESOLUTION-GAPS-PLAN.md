# Remaining resolver gaps: coverage and proposed fixes

Updated 2026-10-04 through the current resolution/performance branch. Historical
measurements below retain the commit or PR against which they were made.

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
| Argument-sensitive producers | Omitted defaults, literal/supplied constructors, `this`, finite argument forwarding, straight-line caller aliases, object/presence guards and supported branch flow now specialize a call. Literal default modes may return the receiver when the source proves the final dispatch. | Dynamic argument bags, branch-dependent caller aliases, computed defaults of unknown value, conflicting returns and escaped scopes remain unknown. The next proof needs reaching definitions across control flow where every path agrees. |
| Lazy/shared caches | Existing collection contracts, guarded `variables`/`this` fields and the Masa `settingsBean.getRazunaSettings` shape now resolve when every lexical write agrees. Presence, primitive-sentinel and null guards are covered. | Rich keyed caches, aliases, external mutation and unsupported control flow still need all writer/initialization paths to agree. Do not restore a receiver-class guess to silence these findings. |
| Relationship getters | Persistent ORM getters and factory-proven nonpersistent bean relationships now carry single-valued targets and collection element contracts into script and tag loops. | Unknown bean ids, broad id-echoing factories, dynamic relationship metadata and collection operations other than proven iteration remain unknown. Add each consumer only with exact factory and cardinality evidence. |
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

The next focused work should target a current corpus entry rather than another
synthetic generalization. The highest-priority open proofs are same-type branch
flow for record members, verified request/view handoffs, automatic application
mapping and DI ownership, and framework callback registration/dispatch. Runtime
includes and Wheels factories follow. Receiver-frequency totals alone are not a
fix plan; every batch still needs a source fixture and per-entry comparison.

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

Assignments on different branches are compared now (`flow.go`): those that
may reach a return must agree, a dynamic one makes the return dynamic, and
otherwise the function has no return type. `buildBinder()` is `$any` because
one of its branches is. Corpus: ColdBox 0 removed / 1 added, every other scan
unchanged. The addition is `Util.cfc:30`: `getClassMappingHelper()` assigns a
`BoxLangMappingHelper`, a `LuceeMappingHelper` or a `CFMappingHelper` by
engine, three components with no common base that each declare
`addCustomTagPath()`. It had been typed as the last. Answering it needs a
return type that is a set of components, which nothing holds yet.

### Braceless bodies are not blocks

Partly fixed: a braceless body that is one line ending in a semicolon is now a
block (`openBracelessBody`, `TestBracelessBodiesAreBlocks`). Still open: a body
over several lines or without a semicolon, which needs the folding pass's
statement-end rule, and braceless bodies inside closures. The text below is the
original analysis. Not corpus-measured (none available when it was written);
the parse benchmark alternated against the old binary showed no difference
beyond noise, on a fixture with few braceless bodies.

Originally open. `flowBlocks` sees a block only where a brace opens one, so a body
written without braces runs "always" as far as a return is concerned:

```cfml
var x = new models.A();
if ( c ) x = new models.B();
return x;            // typed models.B; should be no type (A or B)
```

The same holds for a braceless `else` (including `if ( c ) { … } else x = …`),
`for` and `while`. A braceless branch followed by an assignment that always
runs is already right (`if ( c ) x = new B(); x = new C();` returns `C`).
`TestKnownBracelessBodyGaps` in `internal/parser/return_branches_test.go`
lists each shape with today's answer and the wanted one, and fails when a case
starts giving the wanted one: move it to
`TestReturnTypeComparesTheBranchesReachingIt` then.

Not measured on the corpus: the branch comparison changed one entry there, so
the braceless share of it is likely small, but nothing has counted it. Count it
first (a scan for `if`/`else`/`for`/`while` heads not followed by `{` in
functions with a variable return) before deciding the fix is worth its cost.

Where the fix goes: the function-body loop in `scriptParser.parseFunction`
(`internal/parser/script_parser.go`, the `for depth > 0` loop that opens and
closes a block on each brace) and `handleBodyToken`. After the head of an
`if (…)`, `else`, `for (…)` or `while (…)` that is not followed by `{`, open
a block for the next statement and close it when that statement ends.
`stampFlow` already stamps per handled token, so a block opened and closed
around one statement gets its assignment stamped correctly.

What makes it more than a token check:

- **The statement's end.** CFScript does not require the semicolon, and
  cfwheels omits it throughout. The rule the folding pass uses
  (`parser.StructureSpans`: a newline ends a statement when the last token
  could end an expression and the next is a word that is not an operator) is
  the one to reuse, not a second copy of it.
- **Chains.** `else if ( d ) x = …` is a braceless `else` holding a braceless
  `if`; both blocks end with the same statement. `if ( a ) if ( b ) x = …`
  nests the same way.
- **The head's own parentheses.** The condition is consumed by the loop as
  ordinary tokens today, and the block must open after its closing `)`, not at
  the keyword.
- **Tag syntax has no braceless form**, so `tagParser.trackBlock` needs
  nothing.

Verify as for the branch comparison: the known-gaps test, the corpus diff per
entry against the commit before, and the parse benchmarks alternated against
the old binary, since the loop runs on every token of a full parse.

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

Implemented (`subclass_refs.go`) and measured, see the end of the struct-field
section. Originally open; measured after PR #216. Known for short as **the quickLook ormService
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

## Search-result struct fields (implemented and measured)

The largest group of loop findings left in ContentBox's admin views, ready to
pick up. Measured on `main` at 6e3771f (after #212, #214–#217) with the
ContentBox scratch config from "ContentBox: measurement and relationship
getters" (presets `coldbox`, `contentbox`, `testbox`, `cfmigrations`): 2,637
findings, of which about **250** are loop variables over a `prc` collection
that a handler fills from a field of the struct a service's `search()`
returns, and about 60 more from a cborm `list()`.

**The shape.** Handler, service and view, as ContentBox writes them:

```cfml
// handlers/comments.cfc, action index
var commentResults = variables.commentService.search( search = rc.searchComments, … );
prc.comments       = commentResults.comments;
event.setView( "comments/index" );

// models/comments/CommentService.cfc
struct function search( … ){
	var results = { "count": 0, "comments": [] };
	var c       = newCriteria();
	…
	results.count    = c.count();
	results.comments = c.list( offset = …, max = …, sortOrder = …, asQuery = false );
	return results;
}
```
```cfml
<!--- views/comments/index.cfm --->
<cfloop array="#prc.comments#" index="comment"> #comment.getCommentID()# </cfloop>
```

| Handler right-hand side | Loop findings |
|---|---:|
| `contentResults.content`, `contentResults[ variables.entityPlural ]` (`baseContentHandler`, shared by entries/pages/contentStore) | ~95 |
| `commentResults.comments` (index and pager) | ~52 |
| `results.authors` (`AuthorService.search`) | ~40 |
| `results.versions` | ~22 |
| `results.settings` | ~19 |
| `results.menus` | ~12 |
| `modules.modules` (`ModuleService.findModules`) | ~11 |

**The links to prove**, each by source:

1. **Handler → view.** Done: `Resolver.viewPrcElement` (#212, #214) types a loop
   over `prc.X` from the rendering action's last `prc.X = <rhs>`.
2. **`<rhs>` is a struct field of a local holding a call's result**
   (`commentResults.comments`). `elementOf` (`internal/resolve/loop_element.go`)
   understands a call and a bare or `local.`/`variables.` name; it needs a case
   for `name.field` where `name` is a local assigned from a call: the element of
   that call's returned struct's `field`.
3. **What a function's returned struct holds in each field.** The producer
   interpreter (`internal/resolve/producer_flow.go`) already models struct
   literals and member writes: `producerValue.fields`, and `assign` writing
   `results.comments`. It has no collection value: a `producerValue` is a set of
   components, so `c.list(…)` has nowhere to put "array of cbComment". Add an
   element type to `producerValue` (for example `elements []string`), merged as
   `components` are, and a way to ask for one field's element.
4. **A cborm criteria `list()` returns its entity's array.** `c = newCriteria()`
   on a service bound to an entity (`Resolver.boundEntity`, which #216 extended
   to an init argument's default) is a `CriteriaBuilder` over that entity, and
   `c.list( …, asQuery = false )` returns an array of it. Only `asQuery = false`
   written literally counts at first. `AuthorService.search` passes
   `asQuery = arguments.asQuery` with a default of `false`, which needs the
   call-site specialisation the interpreter already does for defaults. **Verify
   `CriteriaBuilder.list`'s own `asQuery` default against cborm 4.12.1's source**
   (the version the stubs pin; the stub in
   `internal/frameworkapi/stubs/cborm/cborm/models/criterion/CriteriaBuilder.cfc`
   drops defaults) before treating an omitted `asQuery` as an array.
   `c.resultTransformer( c.DISTINCT_ROOT_ENTITY ).list( … )` should be the same
   builder (resultTransformer returns it).

**Rules to keep it sound:**

- A field has an element type only when every write to it that reaches the
  return agrees: the initial `[]` in the literal is compatible with any element
  type, but a field also written with a query or a computed value is unknown.
- A computed key (`results[ variables.entityPlural ]`, `contentResults[ … ]`) names
  no field; leave it unknown. `baseContentHandler`'s `entityPlural` is set per
  subclass, so its ~95 findings may stay out, or need the subclass's literal;
  they also depend on "a base handler's variable its subclasses inject" above,
  since that handler's `ormService` is untyped in the base.
- `xService.list( … )` without `asQuery = false` stays unknown: cborm's
  configuration decides whether it returns a query.
- Collection elements are the entity and its subclasses, as in #209
  (`Resolver.withSubclasses`).

**Validation.** Tests in `internal/resolve` modelled on
`TestALoopVariableHoldsTheCollectionsElement` and
`TestAViewReadsThePrcItsHandlerActionAssigns`: a struct literal with a field
written from a criteria `list( asQuery = false )`; the same through a handler
and a view; and negatives for `asQuery = true`, an omitted `asQuery`, a computed
key, a field written twice with different entities, and a field written with a
query. Each must fail with its piece removed. Measure per entry (sorted diff)
on the ContentBox scratch copy, both modes, and on MuraCMS (both), ColdBox
(presets with the checkout mapped as `coldbox`, and default), cfwheels, fw1 and
Lucee, against `main`, and check scan cost with alternating builds. Run
`unresolved` with absolute directories: a relative one breaks the producer
plans' file reads.

**Also open in the same views**, recorded rather than planned: `cache.get( … )`
for `results.settings` in one action (a cache entry is dynamic), and the
partials rendered through `cbAdminComponent( … )`'s computed view name.

**Status.** Links 2 to 4 are implemented in `internal/resolve/struct_field_element.go`,
hooked into `elementOf` for `name.field` (a local assigned from a call). It is
a token analysis of the producing function, **not** the producer-interpreter
extension sketched above: the interpreter's `assign` discards a struct literal's
fields once a member is written, and widening it to carry element types was
not attempted without a corpus to measure against. The analysis is
deliberately narrower: one script function; every return returns one local
struct; that local is used only as `x.field` (passed, indexed or rebuilt, it is
declined); every value of the field is `[]` or a criteria `list()`; the builder
is a local whose every assignment is `newCriteria()`; and `asQuery = false` is
written literally, or is a parameter defaulting to `false` that the call site
does not override. An omitted `asQuery` is declined (the stubs drop cborm's
default, still to be verified against 4.12.1). Tag-syntax producers are
declined. `TestAStructFieldHoldsTheCriteriaListItsFunctionAssignsIt` has the
positive and each negative; each fails with its piece removed.

**Measured** (ContentBox at 312f182, config `coldbox`, `contentbox`, `testbox`,
`cfmigrations`; per-entry diff against the commit before this work, from
`unresolved --json` on a scratch copy): 2,637 -> 2,486, **153 removed, 2 added**.
Without config: 7,543 -> 7,394, 151 removed, 2 added. The two added are
`baseContentHandler.cfc:424` `addJoinedExpiredTime` / `addJoinedPublishedtime`,
"variable has no component ref" before and now "method 'populate' in
contentStoreService@contentbox|entryService@contentbox|pageService@contentbox has
no component return type": the same calls with a more specific reason, from the
subclass step (gap 11, below). TestBox, cfwheels, coldbox-platform, fw1 and Lucee,
each with presets and without: 0 removed, 0 added. Largest removals:
`comments/index.cfm` 27, `authors/indexTable.cfm` 25, `comments/pager.cfm` 25,
`versions/pager.cfm` 22. That is 153 where about 250 were expected; the
`results[ entityPlural ]` content views and the `cache.get( … )` settings are
among what is left.

Two things the first version missed, found only by running it on the corpus:
ContentBox writes `var c = newCriteria().isEq( … )` (a chain of builder methods,
`builderMethods`), and its handlers write the `search(` call with its arguments one
to a line, which `localAssignment` now joins (up to 24 lines), for loops and struct
fields alike. The first run removed 26; these two took it to 153.

**Gap 11 (`subclass_refs.go`), measured in the same runs:** 14 of the removals are
calls on `variables.ormService` in `baseContentHandler.cfc`. **Not fixed:** a value
read from it (`oContent = variables.ormService.get( … )`, `prc.content = …`, about 50
more in the base handler) is still untyped, so `views/content/quickLook.cfm` is
unchanged at 31. The step answers a receiver lookup; the type an assignment gives its
variable is decided at parse time by a different path (pending calls typed from the
file's own refs and its bases), which does not ask the subclasses. That is the next
piece. Still open from the struct-field section: `results[ variables.entityPlural ]`
(computed key) and `cache.get( … )`.

## ContentBox: what is left after the struct-field work (2,486 findings)

Measured at ContentBox 312f182 with presets `coldbox`, `contentbox`, `testbox`,
`cfmigrations`, after the work above. Each group names the definition that settles
it, found in the corpus or in a pinned dependency; none is implemented yet.

| Findings | Group | The definition that is missing, and the fix |
|---:|---|---|
| 275 | **cbmessagebox is not installed.** `cbMessageBox()` bare (157) and `.error()`, `.warn()`, `.setMessage()`, `.renderit()` chained on it | `coldbox-modules/cbmessagebox` @ 4bbbf8c: `ModuleConfig.cfc` has `this.applicationHelper = [ "helpers/mixins.cfm" ]`, and `mixins.cfm` declares `cbMessageBox()` returning `wirebox.getInstance( "messagebox@cbmessagebox" )`, i.e. `models/MessageBox.cfc`. `helpers.go` already finds a module's helper when the module is in the workspace; ContentBox lists the module in its box.json but ships it under `contentbox-deps` (not in the checkout). Fix: a `cbmessagebox` entry in `frameworkapi.Sources` (stubs for `models.MessageBox`, plus the helper template served from the stub root) and `applicationHelpers()` offering the stub helper last, after the workspace's own |
| 204 | `print` (CommandBox task runners, `build/patches/*/Updater.cfc`) | known: nothing in the source says a file is a task. `BaseTask` is in the commandbox stubs; the missing part is a rule that a component in `build/patches` run by CommandBox extends it |
| 116 | **cborm builder members and closure parameters.** `c.restrictions` (47), the `c` of `.when( test, function( c ){ … } )` (54), `arguments.c` (15) | `cborm/models/criterion/BaseBuilder.cfc` @ a888246: `when( required boolean test, required target )` hands its closure the current builder (`@target … receives the current criteria as the argument`), and a builder's `this.restrictions` is `cborm.models.criterion.Restrictions` (CriteriaBuilder.cfc header). Neither is in the stub: `restrictions` is assigned from an argument, and a closure parameter has no declared type. Fix: a rule typing `x.restrictions` on a builder component, and typing a function literal's first parameter from the callee's documented closure argument (`when`, `list( criteria = function( c ) )`) |
| 80 | `new coldbox.system.orm.hibernate.util.ORMUtilFactory()` in `build/patches/*` | the class moved: it is `cborm/models/util/ORMUtilFactory.cfc`. The patches name ColdBox's old path, so this is a **genuine finding** unless a legacy alias is wanted |
| 70 | `addPermission`/`removePermission` not found in `cbRole` | genuine (the entity's property has no singular name, so CFML generates `addPermissions`); already listed under "Genuine findings" |
| 365 | loop and prc variables in admin views (`thisContent`, `entry`, `page`, `content`, `item`, `author`, `thisPerm`, …) | mostly `results[ variables.entityPlural ]` (computed key, set per subclass) feeding `contentViewlet`, `pager` and the `*/indexTable` views, and `cbAdminComponent( … )`'s computed view name; plus the `oContent = variables.ormService.get( … )` assignment typing gap 11 left |
| 43 | `getBeanPopulator()` / `site()` have no return type | cborm and ContentBox declare and document none (see "Untyped, but correctly so") |
| 41 | bare `getInstance( … )` in `email_templates/*.cfm`, `command( … )`, `getCWD()`, `getSystemSetting()` | email templates are rendered by a ContentBox service, so they have no base; `command()` and the others are CommandBox task helpers |
| 26 | `new dbinfo( … ).columns()` | `dbinfo` is an engine component; `columns()` is a Lucee member the engine rule does not know |

Order by cost and value: cbmessagebox (275, one new source and one helper
hook), then the builder rules (116), then gap 11's assignment typing. The 204
`print` group needs a decision on whether a directory convention may imply a
base, since nothing in the source says so.

### cbmessagebox: done (275 -> 2)

`frameworkapi.Sources` has a `cbmessagebox` entry (pinned 4bbbf8c, the commit
the clone's HEAD was at), `frameworkapi.Helpers` names the template to stub
(`helpers/mixins.cfm`, with `cbMessageBox()` returning
`cbmessagebox.models.MessageBox`, which its source states only in the body), and
`cmd/cfstubgen` writes it as `stubs/cbmessagebox/cbmessagebox/helpers/mixins.cfc`
beside `models/MessageBox.cfc`. The contentbox preset implies the stubs
(`implied`), and `helperTemplates` offers the stub helper **last**, after the
workspace's own, so a checkout of the module outranks it.
`TestAModulesHelperComesFromItsStubWhenTheModuleIsAbsent`, which fails without
the hook. Measured on ContentBox with presets: 2,486 -> 2,213, **273 removed, 0
added**. Only the contentbox preset brings it, so the other projects are not
affected.

### cborm builder members: done in part (116 -> 43)

`builder_members.go`: a closure written as an argument of `when( test, target )`
on a builder chain (`newCriteria()` followed by builder methods, or a local
assigned one) has the builder as its first parameter, and `<builder>.restrictions`
is a `cborm.models.criterion.Restrictions`. A method is a builder method when it is
in `builderMethods`, when the Restrictions stub declares it (the builder forwards
each to Restrictions and returns itself), or when the builder declares it to
return a builder; `isNewCriteriaChain` and the struct-field analysis now share
that rule. `TestACriteriaBuildersClosureAndRestrictionsAreTyped` has the
negatives (a chain that is not a builder, a closure that is not `when`'s, a
typed non-builder with a `restrictions` member); each rule fails without its
piece. The first version removed 18; the early record-member branch in
`receiverComponent` returned before the rule ran, and the chains use many more
restriction methods than a fixed list. Measured on ContentBox with presets:
2,213 -> 2,140, **73 removed, 0 added**; the other five projects 0 / 0.
Left (43): a local `c` assigned from an untyped call, `variables.ormService.
newCriteria()` in an abstract base (gap 11's assignment typing), and
`arguments.criteria.when( … )` where the builder is a parameter.

### Gap 11's assignment typing: done (2,140 -> 2,046)

`assigned_call.go`: a variable the parse left untyped is typed at lookup from its
last assignment, `x = receiver.method( … )` (one line, a bare or `local.` name or
`prc.name`), with the receiver typed as any receiver is (the subclass step
among them) and the method's return taken **per alternative**; every alternative
must return a component. It is the last step of `receiverComponent`, so it
never overrides one, and `receiverComponentD` bounds `x = y.f()` through
`y = z.g()` at three. The parse-time path was left alone: threading a hook
through the seven sites that build `ParseOptions` was not needed to answer a
lookup. `view_handoff.go` kept only the first alternative of `prc.x` (it
passed the list to `ComponentPath`); `pathsOf` resolves each.
`TestAVariableAssignedFromASubclassHeldReceiverIsTyped`; each piece fails
without it.

Measured on ContentBox with presets: 2,140 -> 2,046, **97 removed, 3 added**
(`quickLook.cfm` 30 removed, `baseContentHandler.cfc` 28, `sites/editor.cfm` 23).
The 3 added are genuine and were hidden by the untyped receiver:
`prc.content.getDisplayExpiredDate()` (declared nowhere in ContentBox's models)
and two `getActiveContent().getChangelog()` chains, where `getActiveContent()`
is declared `any`. The other five projects 0 / 0.

**Cumulative since this work began** (ContentBox 312f182, presets): 2,637 ->
2,046, 596 removed, 5 added (2 reason changes, 3 genuine); the other five
projects, with and without presets, unchanged.

### Computed view names and keys, read per subclass (2,046 -> 1,958)

ContentBox's base handler renders `"#variables.handler#/indexTable"` and hands
the view `results[ variables.entityPlural ]`; each subclass sets both to its own
literals (`pages`, `entries`, `content`) and holds its own service, whose
`search()` struct names its field the same way. Three pieces, one per
subclass:

- `moduleActions` expands a `#variables.x#` view name for each leaf subclass
  (`leafLiteral` reads the literal the leaf, or a component between it and the
  base, assigns), and the action carries that `leaf`;
- the handoff evaluates the action on the leaf's behalf (`lookupCtx.leaf`,
  threaded through `receiverComponentD`, `elementOf` and `fieldElement`), so
  `variables.ormService` is that subclass's service and not the union;
- `elementOf` reads `name[ variables.x ]` as the field the leaf's literal names.

`TestAComputedViewAndKeyAreReadPerSubclass`; each piece fails without it.
Measured on ContentBox with presets: **88 removed, 0 added** (`pages/indexTable.cfm`
31, `contentStore/indexTable.cfm` 24, `entries/indexTable.cfm` 21, and the
three `index.cfm`); the other five projects 0 / 0.

**Cumulative since this work began** (ContentBox 312f182, presets): 2,637 ->
1,958, **684 removed, 5 added** (2 reason changes, 3 genuine); the other five
projects, with and without presets, unchanged at every step.

### A literal-named view its base handler renders, read per leaf (1,958 -> 1,927)

`perLeaf`: a leaf-less action of a handler that has subclasses (a literal
`view( "content/pager" )` its base renders) is asked as it is, and when that
answers nothing, once per leaf subclass; the answer is the union as
alternatives, and only when **every** leaf gives one. Measured on ContentBox
with presets: **32 removed, 1 added** (`content/pager.cfm` 19,
`editorSelectorEntries.cfm` 13); the added is genuine, `getActiveContent()` being
declared `any`, so the chain from it was hidden by the untyped receiver. The
other five projects 0 / 0. `TestAComputedViewAndKeyAreReadPerSubclass` has the
union and the leaf that sets no literal.

**Cumulative** (ContentBox 312f182, presets): 2,637 -> 1,927, **716 removed, 6
added** (2 reason changes, 4 genuine).

### What is left, and why it stays (1,927)

- **`print` 204, `getCWD`/`command`/`getSystemSetting` 25**: the files are run
  as CommandBox tasks, but nothing in them says so. Only `BuildDocs.cfc` is named
  (by `box.json` scripts, 4 findings); the patch updaters and archived seeds are
  run from outside the source. Not derivable; it would be a convention.
- **`oRole` `addPermission`/`removePermission` 70**: genuine.
- **`coldbox.system.orm.hibernate.util.ORMUtilFactory` 80**: genuine, the class
  moved to `cborm.models.util`.
- **`new dbinfo(…).columns()` 26**: a name collision decided at run time. The
  non-Lucee branch means Adobe's built-in component; a `DBInfo.cfc` beside the
  file shadows it for the resolver and, on a case-insensitive file system, for the
  engine.
- **`getBeanPopulator()`, `site()` etc. 43**: declared and documented nowhere.
- **Arguments typed only by their callers** (`arguments.content` 48,
  `arguments.site` 32, `arguments.setup` 31, `arguments.original` 27, …) and
  the loop variables over what they hold: a feature of its own (infer an
  argument from every caller when they agree), not a gap in a rule.

### Arguments typed by their callers (1,927 -> 1,890)

`arg_callers.go`. An argument with no type is the component every call of its
function passes, as alternatives when they differ. For each caller found in the
workspace: the call must be the function's own (a bare, `this.` or `variables.`
call in its file or a subclass, a `super.` call from a descendant, or a call on a
receiver whose function of that name *is* it, compared by definition, not by
name); the argument's expression is read from the **tokens**, so a call split
over lines reads, by position or by name; and it is typed as a receiver is
(`new X()`, a name or dotted name — through the receiver lookup and then the
configured resolvers — an argument passed on, typed by its own callers, or a
single call). The argument is typed only when **every** caller that passes it
gets one. A caller that omits it says nothing; a receiver nothing can place, or
an expression nothing can type, leaves it untyped (fail closed: the others'
answer would be a guess). Callers the workspace does not hold are unseen, so
this is an inference from the code present.

Three decisions, each measured:

- **Opt-in.** `Resolver.InferArgsFiles` holds the files to search; only a batch
  scan sets it (`unresolved.Options.InferArgs`, on by default in `unresolved` and
  `explain`, `--no-infer-args` off), once its index is complete. The caller index
  (name -> files whose text calls it) is built once from those files. The editor
  never pays for it.
- **Last.** The step is the end of `canResolveCall`, after every other answer.
  Placed inside `receiverComponent`, it ran ahead of the dynamic rules and the
  name resolvers and turned three accepted calls in coldbox-platform into
  findings.
- **Fail closed on an unplaced receiver.** Treating it as "not this function's"
  would type `clone( original )` from one caller and ignore the others.

`TestAnUntypedArgumentHoldsWhatEveryCallerPasses` has agreeing callers, a union,
a multi-line call, a caller that omits it, an unplaced caller, a `super.` call
and the feature off; each piece fails without it.

Measured against the same build with `--no-infer-args` (the whole effect of this
step): ContentBox with presets **37 removed, 0 added**. The other five projects,
with and without presets, **no call is newly a finding**: every difference is a
removal or a changed reason on a call already reported (TestBox `exposeMixin`
on `makePublic`'s argument became "not found in test1", true of the fixture
callers pass and false of the MockBox-decorated object it is at run time;
Lucee's `MailSpool` argument became "component 'GreenMail' does not exist").
Cost: +0.1 to +0.4s per scan (Lucee, 22s, +0.3s).

What stays: functions nothing calls (handler actions, migrations: the framework
invokes them), and callers whose own receiver is untyped (`newChild.clone(…)`) or
whose expression is a framework result (`populate( "Setup@cbi" )`). The
`createSite( arguments.setup )` chain ends there.

**Cumulative** (ContentBox 312f182, presets): 2,637 -> 1,890, **747 removed, 6
added** (2 reason changes, 4 genuine).

### Straight-line caller aliases and guarded shared fields

Caller inference now follows a plain local through its last assignment when
the assignment precedes the call at the same lexical brace depth. Conditional,
nested, self-referential and after-call assignments remain unknown.

Producer flow also recognizes builtin `structKeyExists`, `isSimpleValue` and
`isNull` guards around `variables`/`this` fields. A guarded field is concrete
only when every lexical write in the component agrees on one component;
primitive sentinels are accepted only for `isSimpleValue`. Conflicting object
writes, including component-body initialization, primitive replacements,
whole-scope replacements and component-defined overrides of the guard builtins
fail closed.

`TestAnUntypedArgumentHoldsWhatEveryCallerPasses` and
`TestGuardedLazyFieldsReturnTheirInitializedType` cover the positive paths;
`TestGuardedLazyFieldsFailClosed` covers the rejection boundaries. No corpus
count was initially claimed because the external corpus checkout was
unavailable in that environment.

The pinned projects were subsequently checked out and scanned at PR head
`d7ee340`, with `85f330b` as the pre-PR baseline. The reports are identical per
entry in every mode measured: **0 removed, 0 added**. The changes therefore add
coverage for the regression fixtures without changing these pinned corpus
findings.

| Scan | Files | Unresolved | Accepted | Removed / added vs `85f330b` |
|---|---:|---:|---:|---:|
| ContentBox, presets | 724 | 1,628 | 15,519 | 0 / 0 |
| ContentBox, no presets | 724 | 7,049 | 9,721 | 0 / 0 |
| Lucee | 3,786 | 1,975 | 37,134 | 0 / 0 |
| TestBox, presets | 146 | 291 | 3,848 | 0 / 0 |
| TestBox, no presets | 146 | 751 | 3,364 | 0 / 0 |
| ColdBox, presets | 664 | 1,364 | 20,019 | 0 / 0 |
| ColdBox, no presets | 664 | 3,365 | 17,700 | 0 / 0 |
| FW/1, presets | 305 | 410 | 1,642 | 0 / 0 |
| FW/1, no presets | 305 | 682 | 1,362 | 0 / 0 |
| cfwheels, presets, explicit vendor root | 1,882 | 4,916 | 69,559 | 0 / 0 |
| cfwheels, no presets, explicit vendor root | 1,882 | 11,241 | 60,382 | 0 / 0 |
| Masa, configured | 897 | 11,981 | 26,791 | 0 / 0 |
| Masa, automatic mappings | 897 | 13,498 | 21,979 | 0 / 0 |

The cfwheels invocation indexed 1,882 files in this checkout, rather than the
older canonical report's 1,195, so it is recorded separately and is not folded
into the historical six-project totals above. All comparisons used the same
roots and configuration for the baseline and PR-head binaries.

### Literal default modes that return the receiver

Masa's `beanORM.loadBy(returnFormat="self")` finishes with a literal mode
dispatch: `query` and `iterator` return other values, while the final `else`
returns `this`. Its earlier SQL construction is intentionally outside producer
flow's supported language, so interpreting the whole method withheld the
default self contract used by `settingsBean.getRazunaSettings` and many other
ORM beans.

A source-backed method contract now recognizes only that final dispatch when
the selected parameter has the literal default `self`. An omitted argument or
an explicit literal `self` returns the actual receiver, including a subclass;
another literal or an unknown value does not. An interpolated mode, a branch
for `self`, work after the dispatch, or a final return other than `this`
withholds the contract. This is argument-sensitive rather than a blanket
`loadBy` rule.

Measured per entry against `d799add` on Masa 7.6.1:

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| Masa, configured | 11,981 | 11,784 | 199 / 2 |
| Masa, automatic mappings | 13,498 | 13,377 | 121 / 0 |

The two configured additions are newly exposed downstream contracts:
`beanEntity.getCurrentUser()` and `oauthClientBean.getUser()` have no component
return type before their following `isSuperUser()` / `login()` calls. All six
other pinned projects, with and without presets, are unchanged per entry.
`TestAGuardedLazyFieldAcceptsAnInheritedArgumentSensitiveSelfReturn` covers the
Masa shape and rejects an explicit non-self mode.

### Bean-backed relationship getters outside CFML ORM

Masa declares bean-ORM relationships with CFML's `fieldtype` and `cfc`
metadata but does not mark those components `persistent`. A single-valued
relationship getter now uses the `cfc` value only when an exact configured or
source-discovered `getBean()` resolver proves that bean id's component. A broad
resolver that merely echoes the id provides no evidence, and ordinary
nonpersistent properties remain untyped.

Measured per entry against `7dee48d` on Masa 7.6.1: configured **11,784 ->
11,782** (2 removed, 0 added), automatic mappings **13,377 -> 13,376** (1
removed, 0 added). The configured removals are `oauthClient.getUser().login()`
and `file.getSite().getWebPath()`; the latter also resolves in automatic mode.
`TestANonPersistentRelationshipGetterUsesAProvenBean` pins the required factory
evidence.

### Bean-backed collection relationships outside CFML ORM

Nonpersistent bean-ORM `one-to-many` and `many-to-many` relationships now use
the same exact bean-factory evidence as single-valued relationships, but retain
that component as the generated getter's element contract rather than typing
the collection itself as one entity. This covers both cardinalities in script
loops and tag `<cfloop>` consumers. Unknown bean ids and broad resolvers remain
untyped.

`TestANonPersistentCollectionRelationshipUsesAProvenBean` covers both
cardinalities and both loop syntaxes. No corpus delta is claimed for this
batch because the pinned external corpus checkout is unavailable here.

### Review corrections for aliases and relationship fallback

Straight-line alias provenance now compares the complete lexical brace path,
not only brace depth. Assignments and calls in sibling conditional blocks have
the same depth but no reaching relationship, so they remain unknown.

A nonpersistent single-valued relationship whose `cfc` bean id has no exact
factory match now continues through ordinary property injection evidence. An
unmatched relationship id no longer suppresses a separately proven `inject`
target. `TestAnUntypedArgumentHoldsWhatEveryCallerPasses` and
`TestRelationshipWithoutFactoryMatchFallsThroughToPropertyBean` pin both
review findings.

### PR 222 review: fail closed when provenance is incomplete

The default-self final-dispatch shortcut now rejects earlier returns, uses of
its mode parameter or `arguments` scope, and a dispatch nested in preceding
control flow. Unsupported and oversized producer methods remain represented
as unsafe plans, so a possible shared-field write cannot disappear from the
all-writes-agree check. Unrelated unsupported methods without a reference to
the field or its scope do not invalidate the field contract.

Caller alias inference now rejects unbraced control bodies and explicitly
withholds inference when the lexical token bound is exhausted. Equal empty
brace paths from failed scans are no longer treated as scope evidence.

Regression fixtures cover early returns, parameter mutation/scope escape,
script and tag switch writers, oversized writers, unbraced conditionals/loops,
and sibling branches in an oversized file. The existing positive fixtures
continue to pin supported straight-line aliases and guarded cache getters.
No new corpus delta is claimed for these review corrections.

### Regression from PR 222: a member's presence counted as an argument guard

PR 222 raised configured Masa from **10,645 to 11,980** findings. It was not
caught because the corpus comparison recorded under "Straight-line caller
aliases" used a Masa run that already included the regression, so it read as
0 / 0. Each commit was measured again on a fresh checkout of the pinned corpus:

| Commit | Masa configured | `variables.configBean` |
|---|---:|---:|
| `85f330b` (before PR 222) | 10,645 | 86 |
| `10b0a2c` Infer guarded lazy shared-field returns | 11,981 | 1,128 |
| `87dcf90` Infer literal default self-return modes | 11,784 | 1,128 |
| `ffd6a37` Withhold resolver inference … | 11,980 | 1,128 |

**`10b0a2c` (+1,336), fixed.** `producerGuardSensitive` replaced the check for
`structKeyExists(arguments, …)` and dropped its test for the comma after the
scope, so `structKeyExists(arguments.config, "assetDir")` counted as a guard
too. A guard marks a method as needing a call-specific plan, and that plan
outranks the method's declared or inferred return. `configBean.set()` has
such a test and a body the tag plan cannot follow, so its plan is one
unsafe node and answers unknown. The `return this` the parser had found was
discarded, `application.configBean = new mura.configBean().set(…)` became
`$any`, and the DI/1 registration `addBean("configBean",
application.configBean)` lost its component. Every managed bean taking
`configBean` in its constructor then had an untyped field. The interpreter's
`structKeyCondition` reads only a bare scope, so the wider trigger could never
succeed. The comma test is back. `TestAMembersPresenceIsNotAnArgumentGuard`
fails without it.

Measured per entry against `ee888f6`:

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| Masa, configured | 11,980 | 10,651 | 1,363 / 34 |
| Masa, automatic mappings | 13,497 | 11,981 | 1,565 / 49 |
| The other thirteen modes | | | 0 / 0 each |

The additions are the loose ends that a typed `configBean` exposes, already
recorded above: `getClassExtensionManager()` chains (a lazy `variables.instance`
getter), two `contentBean` wrapper returns, and in automatic mode chains that
break at an unmapped `mura.bean.*` base.

**`ffd6a37` (+198), left as it is, for a decision.** It made
`selfDispatchPrefixSafe` reject any `arguments` token before
`loadBy(returnFormat="self")`'s final dispatch. Most of Masa's uses are reads,
which are safe, but one is not: `set(arguments)` passes the whole scope by
reference to `set()`, which hands it to `super.set(argumentCollection=arguments)`.
Accepting it means proving no callee rewrites `returnFormat`, which needs
analysis across calls that does not exist. Allowing reads alone recovers none
of the 198, because `set(arguments)` is in every `loadBy` prefix. Choose between
the fail-closed rule as it stands and an explicit exception for passing the
scope to the component's own `set`.

### Wheels `controller( "name" )` (gap 8, controller half): done (4,934 -> 3,879)

Wheels' own test specs build their controller under test with
`application.wo.controller( "dummy", params )`, and every call on it was
`variable '_controller' has no component ref`: 1,000 of the vendor-included
scan's findings. Three pieces, each with a test that fails without it:

- **The rule** (`wheels_controller_paths.go`, reached from `wheelsFactoryReturn`
  once the call is Global's own `controller`): the class
  `$createControllerClass` instantiates, checked against its pinned body —
  the first path in the `controllerPath` list holding `<name>.cfc`, or the
  last path's `Controller.cfc`. The list is the literal written by the nearest
  directory above the calling file (`vendor/wheels/tests/runner.cfm`'s
  `set( controllerPath = AssetPath & "controllers" )` for its specs), else the
  framework default (`events/init/views.cfm`). A computed write there, a
  computed name, or a candidate with no file withholds the type; several
  literal writes in one place give each candidate as an alternative.
  The conditional browser-fixture path `$lockedLoadRoutes` appends is not read
  (the runner switches it off). `TestAWheelsControllerIsTheClassItsPathHolds`,
  `TestAWheelsControllerNeedsThePinnedClassLookup`.
- **Assignment typing passes the call's arguments.** `typeCallExpr` asked
  `FuncLookup` for the method by name only, which no argument-sensitive return
  can answer; it now asks again with the call (`parser.CallHop`), as the parse
  does. It also reads a receiver only a configured resolver names (the
  preset's `application.wo`), as `ComponentOf` does.
- **A variables-scope assignment in another function** (`variablesAssignment`):
  TestBox's `beforeAll()` assigns what `run()` reads. When the enclosing
  function has no assignment, the nearest one above it counts — the parse's
  own rule for such a name (`fileLevelRef`) — unless the function declares the
  name as a local or argument, or the assignment found is a `var`/`local.` one.
  `TestAVariableAssignedInAnotherFunctionIsTypedByThatAssignment`.

Measured per entry against `a6b92a5`:

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| cfwheels, presets, vendor root | 4,934 | 3,879 | 1,058 / 3 |
| Masa, configured | 10,651 | 10,590 | 61 / 0 |
| Masa, automatic mappings | 11,981 | 11,924 | 58 / 1 |
| The other ten modes | | | 0 / 0 each |

All 1,000 `_controller` findings resolve. The three Wheels additions are
gaps the typed controller exposes: two plugin mixins (`$helper01`,
`$$pluginOnlyMethod`, injected from a test plugin at run time) and an
untyped `policyScope()` return. Masa's removals come from the receiver
fallback: `application.changesetManager` / `application.feedManager`, which
the startup templates type, read through `x = application.y.read( … )`
assignments; the one addition is the known automatic-mode `mura.bean.beanFeed`
mapping gap. Cost, alternating binaries: the Wheels scan 12.2s -> 13.0s,
Masa and Lucee unchanged. Without presets nothing types `application.wo`, so
the no-preset Wheels scan is unchanged; the model half of gap 8 is not done.

### A template reads what its includer holds at the include (Masa 10,590 -> 10,324)

An included template runs inside its includer, and inside the includer's function when the
`<cfinclude>` is written in one, so an unscoped name the template reads but never sets is
whatever the includer holds by that name at that line. Masa's `configBean.applyDbUpdates`
declares `var dbUtility = getBean("dbUtility")` and includes every `dbUpdates/*.cfm`, and all
248 `dbUtility` findings there were that name. TestBox's reporters include `assets/*.cfm` inside
`runReport( results, testbox )`, and `CoverageService.renderStats` includes `coverageStats.cfm`
after `var codeBrowser = new browser.CodeBrowser(…)`.

`includerHeld` (`included_locals.go`) asks each include site that reaches the template
(`parser.IncludeSites`, which is `ExtractIncludes` keeping each statement's offset; a
directory listing's glob sits at its `<cfinclude>`) what the includer's receiver lookup gives
for the name at that line. Every site must type it, and the answer is their union. A template
that assigns or declares the name itself (`setsName`) is not asked about. The answer is cached
per template, name, depth and include generation; computed per call, it cost the Masa scan 20%.
`TestATemplateReadsWhatItsIncluderHoldsAtTheInclude` (each guard fails without it),
`TestIncludeSitesSayWhereEachIncludeIs`.

Measured per entry against `2cd21b6`:

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| Masa, configured | 10,590 | 10,324 | 266 / 0 |
| Masa, automatic mappings | 11,924 | 11,658 | 266 / 0 |
| TestBox, no presets | 759 | 652 | 107 / 0 |
| TestBox, presets | 299 | 296 | 3 / 0 |
| The other nine modes | | | 0 / 0 each |

Masa's removals are `dbUtility` (248), `contentRendererUtility` in the legacy object-class views
(16) and two locals of admin pages. Cost, alternating binaries: Masa 14.5s -> 14.8s, Lucee
unchanged. Templates FW/1 or Mura include through a computed path (the admin views' `rc.$`, the
display modules' `$`) have no include edge, so this does not reach them.

### Masa's DAO arguments: callers the parse placed, and members kept through `.update()` (10,324 -> 9,677)

Masa's DAOs take their bean as an untyped argument (`settingsDAO.update( bean )`,
`contentDAO.create( contentBean )`, …), 2,372 untyped-argument findings in all. Logging why
`argumentFromCallers` withheld each showed every DAO blocked by the same two callers, each a
defect of its own rather than a limit of the rule:

- **A caller the parse had already placed was read from its variable alone.**
  `$.getBean( "userManager" ).update( … )` (`jsonApiUtility.cfc`) carries `Component`
  `userManager` from the configured getBean resolver, but `callIsTo` typed only `$`, could not,
  and failed every function named `update` in the workspace. It now reads `call.Component`
  first, as `canResolveCall` does; that also stops dismissing a call like
  `application.serviceFactory.getBean( "contentUtility" ).setUniqueFilename( … )` because its
  bare variable is the bean factory. `TestACallerPlacedByTheParseIsACaller`.
- **A member a method named like a collection mutator was called on lost its type.**
  `x.update( … )`, `x.append( … )` and the rest were recorded as an unknown write *of* `x`, so
  `variables.instance.DAO.update( reminderBean )` withheld the member `reminderManager` sets from
  its DI constructor argument — and with it every DAO's caller list. Such a call changes what `x`
  holds, never which object it is, so it is now a content write: `x`'s own members are
  invalidated as before, `x` keeps its type. `TestExplicitMemberBindings` (two new cases).

Measured per entry against `b8a47f1`:

| Scan | Before | After | Removed / added |
|---|---:|---:|---:|
| Masa, configured | 10,324 | 9,677 | 647 / 0 |
| Masa, automatic mappings | 11,658 | 11,577 | 105 / 24 |
| The other eleven modes | | | 0 / 0 each |

Configured removals are the DAOs' arguments: `contentDAO` 161, `settingsDAO` 149, `feedDAO` 75,
`userDAO` 100, `categoryDAO` 39, `emailDAO` 29, plus `contentUtility` 55 and the member/mailing
list/reminder DAOs. In automatic mode most of those arguments now name beans whose chain breaks
at the unmapped `mura.bean.beanExtendable`, so they collapse into that per-file summary; the
additions are those summaries (2) and 21 `contentUtility` `arguments.contentBean` findings the
rule had typed while dismissing the `getBean( "contentUtility" )` caller above, which passes an
untyped `variables.item`. Cost, alternating binaries: Masa 14.4s -> 15.1s, from the calls on
arguments that are now typed and checked.

### Wheels mixins call what their Controller holds (cfwheels 3,856 -> 3,224)

The largest group left in cfwheels was "no qualifier, not in file" — 910 with presets, 821 of
them in `vendor/wheels`, and 632 of those in `wheels/view/*.cfc` and `wheels/controller/*.cfc`:
`$get` 130, `$args` 83, `model` 52, `$element`, `$tag`, … Those components are never
instantiated. `Controller.init` runs `$integrateComponents( "wheels.controller" )` and
`( "wheels.view" )`, which copies their public methods into every controller, so a bare call in
one runs on the controller. The rule already resolved such methods *from* a controller
(`wheelsControllerFunc`); this is the reverse direction.

`wheelsMixinHostFunc` answers a bare call in a file with no extends whose directory is
`controller` or `view`, when that file is the component `wheels.<package>.<name>` names, the
`Controller.cfc` beside the package is `wheels.Controller`, and it integrates both packages as
pinned (`wheelsControllerIntegrates`, split out of `wheelsControllerFunc` with
`wheelsControllerExtendsGlobal`). The name is then looked up on the Controller and its chain
(`lookupFunc`, so Global's includes count), then among the other packages' public methods. It
runs after cfinclude in `resolveBareCall` and in `bareFunc`, so a chain headed by such a call
continues. A private method of another package is still not found, since it is never copied.
`TestAWheelsMixinsBareCallsAreTheControllers`, with a Controller that does not integrate
`wheels.view` as the negative case; both the rule and the integration check fail it when removed.

Measured per entry against `b98fb2d` (v0.4.1): cfwheels with presets 3,856 -> 3,224 and without
11,259 -> 10,627, 637 removed and 5 added in each; every other mode 0 / 0. The five additions are
`$engineAdapter().isBoxLang()` and two siblings, previously "chained on '$engineAdapter', which
is not found": the head is now found in `global/request.cfm`, which declares `any`, so the chain
stops at the true break. No bare call in the two package directories is left unresolved; the 56
findings remaining there are untyped variables.

### An operator word can name a receiver (cfwheels 3,224 -> 2,971)

Most of the 427 cfwheels "not found in extends chain" findings were in `cli/lucli/tests/specs`,
which hold their module as `variables.mod = new cli.lucli.Module( … )` and call
`mod.generate( … )`. `mod` is CFML's modulus operator, so `isKeyword` turned the parser away from
it and the call was recorded as a bare `generate()`, looked up on the spec's own BaseSpec chain.
`operatorWordIsName` reads a word operator (`mod`, `and`, `eq`, … ) followed by a dot as a name
— no operand starts with a dot except a number, and a chain needs a name after it — in the
statement dispatch (`checkAssignRef`), the argument scan (`scanNestedCall`), the component-level
loop and the tag parser's `<cfset>` string path. The last mattered only for two calls on one
line: `<cfset x = mod.g(1)><cfset mod.g(p)>` lost the second to the sub-parse's per-line tally.
`TestAnOperatorWordCanNameAReceiver`; removing any of the four guards fails it.

Measured per entry: cfwheels with and without presets, 253 removed and 0 added each; every other
mode 0 / 0. `make gapcheck` unchanged.

### A page included by a computed name (Lucee 1,975 -> 1,507)

Lucee's admin `web.cfm` includes `web_functions.cfm` and then `#current.action#.cfm`, where the
action comes from the URL with any `/` refused, so every page beside it runs inside it and calls
its helpers bare: `toArrayFromForm` 158, `printError` 55, `renderCodingTip` 54, … The include
scan dropped any path holding `#`. `computedNameGlob` reads one whose file name is a single
`#...#` span with a literal `.cfm` extension, under a relative literal directory, as that
directory's `*.cfm` — the glob a directory listing already gives. A name with a literal part,
two spans, a computed or mapped directory, or `.cfml` is still not read. The scanners take it
through `includePathAt`, and the reference expressions in `include_scan_test.go` carry the same
alternative, with a sample per refused shape.

A page then sees what every page beside it declares, which is the include scope's rule for
siblings. That is right for `ext.applications.detail.cfm`, which `ext.applications.cfm`
includes beside `ext.functions.cfm`, and wrong once: `messaging.cfm`'s `toFile()` is declared
only in `services.schedule.edit.cfm`. The 8 cfwheels removals are `public/views` dispatchers
(`../docs/#type#.cfm`, `../tests/#format#.cfm`, `layouts/#docFormat#.cfm`).
`TestAPageADispatcherIncludesByNameSeesTheDispatchersHelpers`.

A scope of a hundred templates made `findThroughIncludes` ten times dearer, since each template
went through the whole component lookup; `scopeFunc` reads a template's own functions only,
which is all a template contributes (its includes are in the scope already), and the scan time
is unchanged. Measured per entry: Lucee 468 removed, cfwheels 8 removed in each mode, 0 added
anywhere.

### A LuCLI module's own name, and a mapping built by a regex replace (cfwheels 2,963 -> 2,870, coldbox-platform 1,345 -> 1,327)

Two "component does not exist" groups were paths a project spells for itself:

- **cfwheels' CLI** (`cli/lucli`) writes `new modules.wheels.services.deploy.config.ConfigLoader()`:
  LuCLI installs a module at `modules/<name>`, and the module's `module.json` names it `wheels`.
  `lucliModuleRoot` answers `modules.<name>.rest` from the nearest directory above the caller
  whose `module.json` gives that name and a `main` component, after every mapping and the
  `box.json` slug, as `slugRoot` does for CommandBox. `TestALuCLIModuleNamesItselfUnderModules`.
- **coldbox-platform's tests** map the harness from the repository root, which their
  `Application.cfc` finds as `REReplaceNoCase( this.mappings[ "/tests" ], "tests(\\|/)", "" )`.
  `replacedMappingPath` evaluates `reReplace`/`reReplaceNoCase` of a path by a literal pattern Go
  compiles, with a literal replacement holding no backreference and an optional literal scope;
  anything else is declined, as every other term is. Thirteen corpus `Application.cfc` files
  build a mapping this way. `TestAMappingBuiltByARegexReplaceIsEvaluated`.

Measured per entry: cfwheels 93 removed in each mode, coldbox-platform 18 in each, 0 added
anywhere. Every method called on the now-resolving components exists.

### Wheels mapper mixins call what their Mapper holds (cfwheels 2,870 -> 2,815)

The controller rule above, for `wheels/mapper/*.cfc`: `Mapper.init` copies Global's public
methods and then the mapper package's into itself, so a bare call in a mapper component is the
Mapper's. `wheelsMapperMixinFunc` requires the file to be `wheels.mapper.<name>`, the Mapper
beside it to be `wheels.Mapper`, and that Mapper to run the pinned loader (`wheelsMapperSetup`,
split out of `wheelsMapperFunc`), then looks the name up exactly as a call on a Mapper does.
`TestAWheelsMapperMixinsBareCallsAreTheMappers`, with a Mapper whose init returns before
loading as the negative case.

Measured per entry: cfwheels 57 removed and 2 added in each mode, every other mode 0 / 0. The
additions are `$engineAdapter().globRegex()` and a sibling, whose head is now found and declares
`any`, as in the controller case.

### Templates Wheels includes through Global's wrappers; `$`-named callers (cfwheels 2,815 -> 2,718)

- `public/Application.cfc` runs `application.wo.$includeAndOutput( template =
  "/wheels/events/onrequestend/debug.cfm" )`, and the template calls Global's methods bare
  (`$get`, `urlFor`, `capitalize`). `wheelsWrappedTemplateFunc` finds every call to `$include`,
  `$includeAndOutput` or `$includeAndReturnOutput` naming the template by a literal
  mapping-absolute path, takes the component it runs in (Global for `application.wo`, the
  calling component for a bare call), checks the wrapper and the two methods it calls against
  Global's pinned bodies, and looks the name up there; every such include must agree. Batch
  only, through the caller index. `TestATemplateAWheelsWrapperIncludesIsItsIncluders`.
- The caller index read `$build(` as a call to `build`, so no `$`-named function's argument was
  ever typed from its callers; `callerWord` now takes `$`. `TestADollarNamedFunctionsCallersAreFound`.

Measured per entry on cfwheels with presets: 97 removed, 0 added (55 through the wrappers, 42
`$`-named functions' arguments).

### A caller whose receiver cannot be the component (Masa 9,586 -> 9,219)

Caller inference gives up on an argument when any call of the function's name cannot be placed,
and Mura's DAOs name their writers `update` and `create`: `settingsDAO.update( bean )` shared its
name with `cryptUtility`'s `md.update()` on a `java.security.MessageDigest` and with
`pluginManager`'s `pluginCFC.update()` on `createObject("component",
"plugins.#dir#.plugin.plugin")`. Neither receiver types, so no DAO's `arguments.bean` ever did.
`cannotHold` places such a call as "not this function" when the receiver's assignment reaching
the call makes a Java object (`createObject("java", …)`, `new java:`), or a component whose
computed path ends in a literal file name other than the declaring file's — unless a workspace
file of that name extends the declaring component. `TestACallOnWhatCannotBeTheComponentIsNotACaller`,
which fails without each of the three.

Measured per entry: Masa configured 367 removed, 0 added (`arguments.bean` 133, `userBean` 75,
`feedBean` 72, `categoryBean` 35, …); every other mode unchanged.

### `this` as an argument, and a receiver's guessed type is not a return type (Masa 9,219 -> 9,201)

- An argument passed as `this` from a component is that component or one extending it
  (`withSubclasses`): Mura's contentRenderer hands itself to contentRendererUtility, and a
  theme's renderer extends it. `TestAnArgumentPassedAsThisIsTheCallersComponent`.
- The index parses without looking methods up, and there `x = base.m()` gives x base's own
  type, the fluent guess `baseVarComponent` makes. Two readers took that guess as a statement:
  return inference (`settleReturnVars`), and the resolver's body evaluation reading a
  component variable from the index (`producerEvaluation.read`). So contentRenderer's
  `getMuraScope()`, returning `variables.$ = variables.event.getValue("muraScope")`, returned
  the event, and every call chained on it was "not found in event". The ref now carries
  `ComponentRef.BaseGuess`, and both readers decline it; the resolver then evaluates the body
  with lookups. `TestAReceiversGuessedTypeIsNotAReturnType` fails without either guard.
  `TestThisAndVariablesAreSeparateStores` asserted the guess as a return type; it now states
  `getFromScope`'s return through a lookup and checks the same two stores.

Measured per entry: Masa configured 21 removed, 3 added; automatic 21 removed, 4 added;
coldbox-platform 2 removed; TestBox 4 removed. The additions are honest: `getMuraScope` now has
no return type ("has no component return type" instead of "not found in event"), and
`arguments.content` typed as contentBean exposes `getDisplayInterval()` with none.

### A Mura preset, and a lazy request field's only writer (Masa 9,201 -> 6,072; automatic 11,452 -> 8,436)

- **`frameworks: ["mura"]`** types the Mura scope, which Mura's documentation spells `$`, `m`
  and `mura` and which an admin view reads as `rc.$`: those names in any scope,
  `getMuraScope()` and an event's `getValue( "muraScope" )` are `mura.MuraScope`. Every resolver
  is `dynamicIfMissing`, as presets are, and the stubs (`MuraScope`, `MasaScope`, `cfobject`,
  `event`, `sessionUserFacade`) are generated from MasaCMS at the commit the corpus pins. The
  scope's `OnMissingMethod` forwards to the content renderer, so calls it does not declare are
  accepted as the runtime answers them. The source never states what `$` holds in a view — the
  admin framework copies `rc.$` into a local before a computed include, and `rc.$` is
  `request.event.getValue('MuraScope')` — which is what makes it a preset and not a rule.
  `scripts/corpus/masacms.json` now enables it, so both Masa baselines move with this commit.
  `TestEveryPresetResolverMatchesItsOwnNames` has its cases.
- **A lazily created request field** is what its only writer stores: `getCurrentUser()` creates
  `request.currentUser` when absent and returns it, and no other file writes the key, so it
  returns `sessionUserFacade` — which types `$.currentUser()` and the rest of the chains the
  preset exposed. `sharedFieldContract` reads it for `request.` as for `variables.`, in a batch
  scan only (`onlyFileWritesRequest` checks every file for an assignment, a bracketed write,
  `structInsert` or `cfparam`), and the file's own writes counted in its text must equal the
  ones the plan read, since a statement the planner cannot read may hold one.
  `TestALazyRequestFieldIsWhatItsOnlyWriterStores`.

Measured per entry against the previous commit: Masa configured 3,326 removed and 197 added,
automatic 3,474 removed and 458 added; every other mode unchanged. The additions are calls
chained on the scope's dual-mode accessors (`$.event()`, `$.content()`, `$.getFeed()`), now
reported as "has no component return type" where the receiver was untyped before.

### An FW/1 view's rc, and a partial reads its view's members (Masa 6,072 -> 5,401)

- **`rc.X` in an FW/1 view** is what the controller action rendering it leaves there
  (`fw1ViewRc`): `views/<section>/<item>.cfm` runs after `controllers/<section>.cfc`'s
  `<item>( rc )`, which runs after `before( rc )`, and an action calling
  `setView( "section.item" )` renders it too. The item action's assignment at its end, else
  `before()`'s, typed with the controller's own rules; every action must agree, and a view no
  action renders gives nothing. Applied only where a preset gives views a base (the fw1
  preset), so the convention is not guessed elsewhere. Mura's admin views read
  `rc.contentBean`, `rc.siteBean`, `rc.feedBean` and the rest this way.
- **A partial reads its includer's members** (`includerHeld`): `rc.contentBean` in
  `views/carch/form/*.cfm` is what the view including it holds. `includerHeld` took plain names
  only, and refused `rc` and `prc` with the rest of `isScopeWord`'s list; it now takes one member
  of a variable that is not a CFML scope (`isCFMLScope`), and the member branch of
  `receiverComponentD` asks it, and the FW/1 rule, last.
  `TestAnFW1ViewsRcIsWhatItsControllerAssigns`, failing without each of the five pieces.

Measured per entry: Masa configured and automatic 673 removed and 2 added each; every other mode
unchanged. The additions are calls chained on `contentBean` methods that declare no type.

### Wheels view helpers (cfwheels 2,718 -> 2,600)

Worked back from the method definitions: 116 cfwheels findings were bare calls in views to
functions declared in `app/views/helpers.cfm`. `Controller.cfc` includes
`#application.wheels.viewPath#/helpers.cfm` into every controller, and `$initControllerObject`
includes `<viewPath>/<controller>/helpers.cfm` into the one it starts, so a view (which runs in
its controller) and the controller call both bare. `wheelsViewHelpers` adds them, with what they
include beside them, to the preset's helper templates: for a view the controller is its folder,
for a controller file its own name. `TestWheelsViewHelpersReachTheViewsAndControllers`.

Measured with `make resolution-report` against the previous run: cfwheels 118 removed, 0 added;
the other six configured scans unchanged.

### An unquoted `extends` (TestBox 292 -> 271, fw1 409 -> 387, Lucee -7, cfwheels -6)

Working back from `describe()`, defined in TestBox's BaseSpec: the specs calling it bare declared
`component extends=testbox.system.BaseSpec {` — an unquoted attribute value, which CFML allows.
The script parser read `extends` only from a quoted string, so the spec had no base. It now
takes an unquoted dotted name (`dottedRest`), as does the `Application.cfc` mapping reader; tag
syntax already did. `TestAnUnquotedExtendsIsRead`.

Measured against the previous run: TestBox 21 removed, fw1 23 removed and 1 added (a spec whose
unquoted base is `mxunit.framework.TestCase`, not installed, now reports that once), Lucee 7 and
cfwheels 6 removed.

### Module helpers ContentBox depends on but does not check out

cbvalidation (`validate`, `validateOrFail`, `getValidationManager`,
`validateModel`, `validateHasValue`, `validateIsNullOrEmpty`, `assert`),
cbsecurity (`jwtAuth`, `cbSecure`), cbauth (`auth`) and cbmessagebox
(`cbMessageBox`) join `moduleHelpers`, each read from the module's
`helpers/Mixins.cfm` at the commit noted in `modules.go`. As before they apply
only to a component whose extends chain reaches `coldbox.system.`. Measured:
cb-p −12, nothing added; the other scans unchanged.
`TestAModuleHelperIsAColdBoxComponents`.

### A stub keeps a return type documented in another framework's namespace

cborm documents `getObjectPopulator()` as returning
`coldbox.system.core.dynamic.ObjectPopulator`, a class cborm's source does not
hold, so `docReturn` dropped it; and its deprecated `getBeanPopulator()`, which
ContentBox still calls, is `return getObjectPopulator();`. `cmd/cfstubgen` now
takes a documented path in another framework's namespace
(`frameworkapi.NamespaceOf`) when that framework's stubs, written earlier in
the same run, hold it (`foreignStub`; ColdBox comes first in `Sources`). It
also gives a function whose whole body is `return f();` the stub type of `f`
in the same file (`delegatedReturn`), and `funcLookup` falls back to a
documented type. New types: cborm `getBeanPopulator`/`getObjectPopulator`,
ColdBox `Controller.getDataMarshaller`/`getRequestContext`. Regeneration is
reproducible (the same diff twice).

Regenerating also picked up drift from this branch's parser changes, each
checked against the source. Gains: entity relationship variables
(`variables.site`, `creator`, `role`…) and `buildProviderMenu`'s `Menu` argument.
Losses, all corrections: `getClassMappingHelper`/`getEngineMappingHelper` assign
one of three helpers by engine; `buildBinder` returns either a new `Binder` or
`arguments.binder.init()`; and MediaService's `variables.provider` does not
exist. Measured: cb-p −22/+2, where the two added are the same chains one hop
further on (`populateFromStruct` returns the target it is passed). Other scans
unchanged. `TestAStubReturnsAnotherFrameworksDocumentedClass`.

Not fixed, recorded: cb-p's `build/patches` (193 findings) are upgrade scripts
written against old ContentBox and ColdBox APIs. `addPermission` (66) is the
ORM method of an older `Role` that had `singularName="permission"`, and
`coldbox.system.orm.hibernate.util.ORMUtilFactory` (54) is ColdBox 3/4.

### A struct of closures answers its members

DI/1's `declare()` returns a local struct built from literals whose members are
closures, each returning the struct again, and FW/1 applications chain them:
`declare( "x" ).instanceOf( "y" ).asSingleton()`. A chain hop whose function
declares no component now reads its body (`closure_struct.go`). When every
top-level return returns one local, and that local is assigned a struct literal
or `structAppend()`ed one, the closure members of those literals are the methods
the next hops may call. A member returning the local keeps the chain on it; one
returning anything else ends what is checked. A name that is not a member is
reported as such, and a function returning a struct with no closures is
reported as before. Measured: fw-p −73, nothing added; other scans unchanged.
`TestAStructOfClosuresAnswersItsMembers`.

### A function returning a built-in's value returns something dynamic

`getPageContextResponse()` in TestBox, ColdBox's Bootstrap and BaseTestCase,
and the cfwheels copies, returns `getPageContext().getResponse()`. Depending on
the engine it may instead return a struct standing in for that, or a ternary
of two such chains. A call chained directly on a built-in is already dynamic
where it is written, but one reached through a user function was reported as
"has no component return type". `engineValueReturn` (`closure_struct.go`) now
reads the function's own top-level returns. When every one is a literal, or a
chain headed by a built-in the file does not declare, and at least one is the
latter, the value is dynamic. A ternary qualifies when both branches do. Both
the qualified-hop and the bare-chain paths ask, and the bare path asks the
closure-struct rule too. Measured: tb-p −13, cx-p −9, cw-p −6, nothing added.
`TestAReturnOfABuiltInsValueIsDynamic`.

### A Mura display object runs inside the content renderer

contentRenderer.cfc includes a display object's template by the path
`siteConfig().lookupDisplayObjectFilePath()` finds under a `modules` or
`display_objects` directory (`core/modules/v1` among them). The mura preset now
gives such a `.cfm` the implied base `mura.content.contentRenderer`, the
mechanism FW/1 views already use, so a bare `showItemMeta()`, `getURLStem()` or
`dspObject()` in one is the renderer's. Measured: masa-c −74/+20.

Of the 20 added, 12 are earlier findings one step further on. A bare `getSite()`
is now found on the renderer, but it has no return type. `dspTopNav()` and
`variables.siteConfig()` are declared nowhere on the renderer, so they move from
"no qualifier" to "not found in extends chain". The other 6 are
`event.getContentBean()` reported "not found in event": a template's `event` is
the renderer's `variables.event`, which DI/1's constructor autowiring types as
the `event` bean (`mura.event`), while Mura passes a `servletEvent` at render
time. That is the same question as the held-back `event` preset
(`resolution-candidates/held-back/mura-event-preset.patch`), so it is left for
that decision. `TestAMuraDisplayObjectRunsInTheContentRenderer`.

### A Wheels global template reads Global's functions

`app/global/*.cfm` is mixed into every Wheels controller, model and view, and
`vendor/wheels/global/*.cfm` into `Global`. The wheels preset gives a `.cfm` under
a `global` directory the implied base `wheels.Global`, which all of those hosts
have, so `model()` in `install.cfm` is found while a view-only helper there would
still be reported. Measured: cw-p −9/+1, where the added one is `$getDBType`
moving from "no qualifier" to "not found in extends chain".
`TestAWheelsGlobalTemplateRunsInGlobal`.

### Wheels seed files run in the Seeder

`wheels.Seeder` includes `app/db/seeds.cfm` and `app/db/seeds/<env>.cfm`
through a computed path, so `seedOnce()` there is the seeder's. The wheels
preset now gives `seeds.cfm` (by name) and `.cfm` files under a `seeds`
directory the implied base `wheels.Seeder`, and `wheels.Seeder` is added to
the Wheels stubs' `Extra` list for apps without the framework checked out.
Regenerating also stubbed Mura's `contentRenderer`, which the mura preset now
names as a base. Measured: cw-p −9, nothing added; masa-c unchanged, since its
own source outranks the stub.

### An inline component with attributes is dynamic

Lucee's tests write `new component accessors=true { … }` and
`new component javaSettings='…' { … }`. The parser handled `new component { … }`
as `$any` only when the brace came straight after, so these were read as a
component literally named `component`, and every call on them was "not found in
component". `readNewComponent` now consumes `name` and `name=value` attributes
up to the body's `{` (`skipInlineComponentAttrs`) and gives `$any`, restoring
the scanner when no body follows. Measured: lucee −47, nothing added; gapcheck
unchanged. `TestAnInlineComponentWithAttributesIsDynamic`.

### What Wheels' $createObjectFromRoot builds, through a spec's wrapper

`assignedFromCall` reads `x = receiver.method( … )` at lookup time, with a regex
that did not allow `$` in names. So every
`d = application.wo.$createObjectFromRoot( path = "wheels", fileName = "Dispatch", method = "$init" )`
in cfwheels' specs was untyped, although the factory with literal arguments was
already answered (`wheelsConstructedFactory`). It now uses `assignedCallRe`,
which allows `$`. A bare `x = f( args )` is read too (`typeBareCallExpr`): when
`f` declares no component, its argument-dependent return is asked
(`expressionReturn`), and a local holding a literal struct is written into the
call in its place (`inlineStructArg`: the last `name = { … }` in the function,
with later `name.key = …` applied, keeping string literals only).
`wheelsFactoryReturn` recognises one more wrapper shape,
`return g.$createObjectFromRoot( argumentCollection = arguments.config )`
(`wheelsArgCollectionParam`), and hands the factory the struct's fields.
Measured: cw-p −195 (`d` 85, `PluginObj`/`pluginObj` 94, `_dispatch` 16),
nothing added. `TestAWrapperHandsTheFactoryItsArgumentsStruct`, whose
computed-`fileName` case stays untyped.

### MXUnit is TestBox's compatibility layer

TestBox documents running MXUnit tests by mapping `/mxunit` to
`testbox/system/compat`, and its stubs already hold
`testbox.system.compat.framework.TestCase`. `frameworkapi.Namespaced` now reads
`mxunit.` as `testbox.system.compat.`, so no MXUnit source is stubbed.
Measured: fw-p −36/+5. The 5 are methods FW/1's tests inject into the framework
object (`selectLayoutTwo()` calling `setLayout()`/`view()`), previously hidden
behind the unresolved base. `TestMXUnitIsTestBoxsCompatibilityLayer`.

cbvalidation is stubbed; see the next section.

### cbvalidation is stubbed

ContentBox depends on cbvalidation and does not ship it. cbvalidation joins
`frameworkapi.Sources` at b700fab0, stubbing `ValidationManager` and
`ValidationResult` (and what they reach), and the `cbvalidation.models.`
namespace. As with cbmessagebox, its `helpers/Mixins.cfm` is stubbed as a
helper, and the contentbox preset implies it. `validate()` and
`validateModel()` are stated to return `ValidationResult`: their doc names the
`IValidationResult` interface, which `docReturn` skips. `getValidationManager()`
returns the manager.

The parse cannot see a helper stub, so `var vResults = validate( … )` is typed
at lookup. `typeBareCallExpr` used to decline a bare call whose function
declares a return, on the assumption that the parse had already typed it; it
now returns that declared component. Measured: cb-p −23 (`vResults` 22, plus one
argument typed by caller inference), cw-p −2 (`local.bridge = $cliBridge()`, a
`CliBridge`), nothing added. `TestAValidationResultIsTypedFromTheStubbedHelper`.
cbauth remains unstubbed: its `auth()` helper is accepted (`moduleHelpers`),
and nothing in the corpus chains on it.

### cbsecurity is stubbed

The same pattern, at a890b0cb: `CBSecurity` and `JwtService` (and what they
reach) are stubbed, along with the `cbsecurity.models.` namespace and
`helpers/mixins.cfm`. `jwtAuth()` and `cbSecure()` return `JwtService` and
`CBSecurity`; both are `wirebox.getInstance( "<id>@cbSecurity" )`. The contentbox
preset implies it. Measured: cb-p −2/+1. `jwtAuth().fromUser()` resolves, and
the contentbox-api auth handler's missing-base summary goes, since its
`jwtAuth()` calls were the inherited calls it counted. The added finding was
hidden before: `jwtAuth().getUser().getMemento()`, where `JwtService.getUser()`
returns `any`. `TestCBSecurityHelpersComeFromTheirStubs`.

### A constructor argument is what every construction passes

Caller-argument inference never covered `init`. It looked for calls by the
function's name, which for `init` is every file, and `new X( … )` is not a call
named init. `inferInitArgument` (`init_args.go`) finds an `init` argument's
callers by the component's file name instead. It reads every `new a.b.X( … )`
whose path resolves to the component (`constructions`, using the call's own
token, since `callArgument`'s name-and-line lookup met `function stats()`
before `new Stats( this )`), and every `init()` call the existing check places
on it. Any other place the name ends a quoted string, such as
`getInstance( "X" )` or a `createObject` with no init on its line, is a
construction that cannot be read, and leaves the argument untyped. The caller
index records names ending a quoted string for that purpose
(`quotedCallerKey`). `initArgMember` then types a `variables.x` whose only
assignments are `variables.x = arguments.p` inside init.

Measured: cw-p −3, tb-p −1 (`CollectionExpectation`'s `variables.spec`, a
`SshPoolTask`'s pool), nothing added. Scan time is unchanged within noise
(cw-p 9.4s, masa-c 12.5s). The case it was written for, cfwheels' CLI
`Templates`, stays untyped for a real reason: its one construction passes
`helpers = getService( "helpers" )`, a service locator keyed by name.
`TestAConstructorArgumentIsWhatEveryConstructionPasses`.

### A generated getter returns the init argument its setter stored

ColdBox's `BoxLangStats` stores its provider through accessors:
`setCacheProvider( arguments.cacheProvider )` in init, read back as
`getCacheProvider().getCache()`. `initArgGetter` answers a chain hop on a
generated getter when every write to the property in the file is
`variables.x = arguments.p` or `setX( arguments.p )` inside init
(`initStoredArg`). A setter called anywhere else leaves it untyped.
`initArgType` answers with the argument's declared component type when it is
dotted (BoxLangStats documents `ICacheProvider` through `@cacheProvider.doc_generic`),
and otherwise with what every construction passes. Both the qualified-hop and
the bare-chain paths ask; the qualified one moved into `untypedHop` to keep
`walkHops` under the complexity limit. Measured: cx-p −29, nothing added.
`TestAGeneratedGetterReturnsTheInitArgumentItsSetterStored`.

### A ColdBox error template reads processException's locals

ColdBox renders an application's error page by including the template its
config names as `customErrorTemplate`. The include is computed
(`include "#bugReportRelativePath#"`) from inside Bootstrap's
`processException()`, where `var oException = new ExceptionBean( … )`, so every
`oException.x()` in ColdBox's own Whoops.cfm, BugReport.cfm and
BugReport-Public.cfm was "has no component ref". `coldboxErrorHosts`
(`coldbox_error_template.go`) gives such a template that site as an includer,
for `includerHeld`. The template must be named by some `config/ColdBox.cfc`.
The site is checked by its text and by the function it is in, and ColdBox's
source must be present, since a stub has no body. For coldbox-platform's own
checkout, an include path's first segment may also be the `box.json` slug
above the file (`includePathUncached`, as `slugRoot` already reads a dot-path),
so `/coldbox/system/exceptions/Whoops.cfm` resolves without a `/coldbox`
mapping. Measured: cx-p −70, nothing added.
`TestAColdBoxErrorTemplateReadsProcessExceptionsLocals`, which fails without
either half.

### A ColdBox model test's model is the class its attribute names

ColdBox's `BaseModelTest` runs `variables.model = mockBox.createMock(
annotations.model )`, and `BaseInterceptorTest` does the same with
`interceptor`. A test written
`component extends="coldbox.system.testing.BaseModelTest" model="coldbox.system.core.events.EventPool"`
therefore holds a mock of `EventPool` in `model`. The base's own ref made
`model` `$any`, so nothing on it was checked, and `pool = model.init( … )`
typed nothing. `coldboxTestSubject` answers `model`/`interceptor` from the
attribute when the extends chain reaches one of those bases and the file
assigns the name nowhere else. It is asked before the extends-chain refs, in
`inheritedReceiver` (its own file, outside the accept-path test's scope,
since its `""` means no answer rather than accepted).

The specs assign `variables.pool = model.init( … )` in a `beforeEach` closure
and read `pool` unscoped in an `it()`. The lookup-time assignment reader
(`localAssignRe`) now accepts a `variables.` prefix, as it accepted `local.`.
An unscoped read reaches a variables-scope name when no local hides it.

Measured: cx-p −100, masa-c −6 (Masa's SSRF spec,
`variables.apiUtility = …getApi( … )` in setup), nothing added. With `model`
typed, its calls are now checked, and none was missing.
`TestAColdBoxModelTestsModelIsItsAttributesClass`, which fails without either
half.

### prc.response is ColdBox's Response

`RequestContext.getResponse()` stores a `coldbox.system.web.context.Response`
in the private collection ("The response object lives in `prc.response`"), and
ColdBox's RestHandler calls it before reading `arguments.prc.response`.
`coldboxPrcResponse` answers `prc.response`/`arguments.prc.response` with
that Response in a component whose chain reaches ColdBox's EventHandler, by
name or by resolving to its file (RestHandler's bare `extends="EventHandler"`).
It does not apply when the file assigns `prc.response` itself; no application
in the corpus does. It is asked on both receiver paths: `inheritedReceiver`,
and the member path that `arguments.prc.response` takes. cborm's
`resources.BaseHandler`, which ContentBox's API handlers extend (it extends
RestHandler), joins the cborm stubs.

Measured: cx-p −58, cb-p −26/+4. The ContentBox removals include the 12 API
missing-base summaries, now that cborm's BaseHandler resolves. The 4 added
were behind that broken base: the API `baseHandler` reads
`variables.ormService`, which only its subclasses set and not all of them
type. `TestPrcResponseIsColdBoxsResponse`.

### Each nested FW/1 application has its own DI/1 beans

DI/1 discovery read only the `Application.cfc` at each workspace root. FW/1's
examples are applications side by side, each extending `framework.one` with
DI/1 over its own `model` and `controllers`, and none got a policy, so
`property userService;` was untyped. `diSources` now also takes every
`Application.cfc` under the workspace folders that extends `framework.one`
(`nestedFW1Apps`). They are found by a bounded walk (`applicationFiles`),
because the policies are built while the index is still being filled.
Each such application then gets the automatic policy its root would have.

Several applications also share bean names (each has a `model/services/user.cfc`),
and the workspace bean map names one file per bean, so a policy found its
candidate in another application or not at all. Each policy now indexes its
own folders (`indexBeans`: a component by its file name, and by file name plus
its folder's singular, DI/1's alias). It uses that when the workspace map's
answer is not under it (`beanIn`).

Measured: fw-p −22, nothing added; Masa CMS scan time unchanged (12.8s).
`TestEachNestedFW1AppHasItsOwnDI1Beans`, which fails without either half.

### FW/1 injects its controllers; a property names a singleton

qBall sets `diLocations = "./model/services"`, so its DI/1 policy covered only
that folder and its controllers' properties were untyped, although FW/1
autowires its controllers from the bean factory whatever `diLocations` says.
An automatic FW/1 policy now also injects into the application's
`controllers` folder (`diPolicy.injected`, `serves`), without discovering beans
there. When a policy's own folders hold two beans of a name (qBall's
`beans/question.cfc` and `services/question.cfc`), a property or setter, which
DI/1 fills only with a singleton, takes the one that is not transient
(`beanIn(name, singletonOnly)`, `transient`).

Measured: fw-p −17/+2. The 2 added were a resolution error that was already
there, now visible because the services are typed; the next section fixes it.
`TestFW1AutowiresItsControllersWhateverDILocationsSays`.

### CommandBox's getInstance() takes WireBox ids

CommandBox is built on WireBox, and a command's `getInstance()` takes the same
ids and DSL as a ColdBox handler's: cfwheels' CLI writes
`application.wirebox.getInstance( "DetailOutputService@wheels-cli" )`. Only the
coldbox preset registered `idResolver("getInstance")` and the DSL resolvers,
so under the commandbox preset alone the id typed nothing. The commandbox
preset now carries both. Measured: cw-p −67, tb-p −1, nothing added.
`TestEveryPresetResolverMatchesItsOwnNames` has the case.

### An entity name is an entity, not the file beside the caller

`entityNew( "question" )` in FW/1 qBall's `services/question.cfc` was read as a
path, which found that service itself rather than the persistent
`beans/question.cfc`. The parser now marks a name passed to `entityNew()`,
`entityLoad()` or `entityLoadByPk()` as an entity name (`parser.EntityPrefix`,
`entity:Name`), in both syntaxes. The resolver looks such a name up as an
entity first and falls back to reading it as a path (`componentPathUncached`,
`nearestEntity`); `displayComponent` drops the prefix. The batch index now also
keeps persistent components that name no `entityname`, whose entity is called
after the file, as candidates per name (`Index.EntityCandidates`), as the editor
already registered them. An explicit `entityname` wins; otherwise the candidate
nearest the caller does, the lowest path among equals, since several
applications in one workspace each have one.

A first attempt preferred an entity for any bare name; it changed nothing in
fw-p and added 8 wrong findings in lucee, and was dropped. Measured with the
marked name: fw-p −12, nothing added; the full short test suite and
gapcheck pass. `TestAnEntityNameIsTheEntityNotTheFileBesideTheCaller`.

### A forwarded argument is what the caller holds

Mura's contentRenderer sets `arguments.renderer = this` and calls its utility
with `dspObject( argumentCollection = arguments )`. Caller-argument inference
saw no `renderer` passed at that call. `callArgumentAt` now reads a call
passing `argumentCollection = arguments` as handing over the caller's own
`arguments.<name>`, which the receiver lookup types in the caller's file. A
forwarded argument that cannot be typed is skipped (`forwardedArg`), as a call
that does not pass the argument always was. Without that, cfwheels'
`SpyTenantMigrator`, which forwards its own untyped migrator to `super`, left
`TenantMigrator`'s `arguments.migrator` untyped (+8). The constructor
inference applies the same rule; `inferInitArgument`'s per-file scan moved
into `initArgSites` to stay under the complexity limit.

Measured: masa-c −56/+2, where the 2 added are the same chains one step further
on (`getEvent()` on the renderer, untyped); other scans unchanged.
`TestAForwardedArgumentIsWhatTheCallerHolds`.

### An alias is typed as the name it copies

`assignedFromCall` read only `x = receiver.call( … )`. Masa's form builder
writes `var mmRBF = application.rbFactory`, a value a startup template assigns
and only a lookup types. A right-hand side that is a dotted name and nothing
else (`aliasRe`) is now typed as that name is at the line, one assignment
deeper (`maxAssignedDepth`). Measured: masa-c −13, cw-p −12, cb-p −6, nothing
added. `TestAnAliasIsTypedAsTheNameItCopies`. The form-builder templates
themselves (144 findings) read `mmRBF` from functions that include them by a
computed path, and are not reached yet.

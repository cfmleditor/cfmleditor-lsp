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
| Lazy/shared caches | Existing proven collection contracts remain supported; owned arrays and structs share element checks. | Rich keyed/lazy getters still need all writer/initialization paths to agree. Masa's `settingsManager.getSite` try/catch cache and `settingsBean.getRazunaSettings` shared-field cache are concrete follow-up fixtures. Do not restore a receiver-class guess to silence these findings. |
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

# Unresolved findings and their candidates

What `unresolved` still reports over the pinned corpus, arranged for a person to evaluate.
**Nothing here resolves a finding.** Every one is still reported; the lists say which link
each finding is missing, whether the method exists anywhere, and the liberal matches the scan
can offer for it, each with its evidence and a confidence level.

Regenerate with `make resolution-report`, which builds the binary, runs each named scan with
`--candidates` (four at a time, `JOBS` to change it), keeps the JSON in `RUNS` (default
`target/resolution/latest`) and rewrites these lists:

```sh
make resolution-report CORPUS="masa-c=/src/MasaCMS cw-p=/src/cfwheels,/src/cfwheels/vendor/wheels ..."
```

Each scan resolves under the `.cfmleditor.json` governing its first directory. The lists are
for milestones, not every change. After a change, compare with an earlier run instead, without
rewriting them:

```sh
make resolution-report CORPUS="..." RUNS=target/resolution/after BASELINE=target/resolution/before LISTS=
```

which prints each scan's findings removed and added, finding by finding. Candidates are
computed after the scan, against a fully built index, so the same workspace gives the same
lists, and they never change the findings.

`summary.md` links one report per corpus (each in its configured, presets-on mode):
ContentBox (`cb-p`), cfwheels (`cw-p`), coldbox-platform (`cx-p`), fw1 (`fw-p`), Lucee
(`lucee`), Masa CMS (`masa-c`) and TestBox (`tb-p`).

## The four categories

| Category | The missing link | Reason text |
|---|---|---|
| **Object reference** | A component path names no file, or a chain breaks at one | `component 'X' does not exist`, `extends chain breaks at …`, `chained on 'f', which is not found` |
| **Variable definition** | A receiver whose component is unknown | `variable 'x' has no component ref` |
| **Return type** | A call chained on a method that declares no component | `method 'f' has no component return type (chain to 'g')` |
| **Method definition** | A method not found where it was looked for | `method 'f' not found in X`, `no qualifier, not in file`, `not found in extends chain` |

## Missing, or unconnected

`--global-defs` accepts any call whose method name some indexed file defines. Comparing a scan
with and without it splits every category in two: findings whose method is **defined nowhere**
in the workspace (*missing*), and findings whose method exists but the resolver cannot connect
the call to it (*unconnected*). Most of every corpus is unconnected — the object is real and
the method is there; what is absent is the link between them.

Each cell is *all findings / findings whose method is defined nowhere*:

| Scan | Total | Object | Variable | Return type | Method |
|---|---:|---:|---:|---:|---:|
| ContentBox, presets | 1,576 | 97 / 82 | 1,228 / 78 | 84 / 20 | 167 / 41 |
| ContentBox, none | 7,049 | 297 / 104 | 5,864 / 1,463 | 109 / 27 | 779 / 343 |
| cfwheels, presets | 2,718 | 218 / 4 | 1,995 / 307 | 154 / 16 | 351 / 109 |
| cfwheels, none | 10,121 | 463 / 62 | 8,540 / 482 | 420 / 42 | 698 / 145 |
| coldbox-platform, presets | 1,325 | 51 / 15 | 1,016 / 119 | 166 / 41 | 92 / 22 |
| coldbox-platform, none | 3,345 | 149 / 29 | 2,583 / 172 | 405 / 41 | 208 / 38 |
| fw1, presets | 409 | 40 / 37 | 224 / 19 | 93 / 80 | 52 / 29 |
| fw1, none | 682 | 40 / 36 | 446 / 25 | 88 / 74 | 108 / 29 |
| Lucee | 1,507 | 74 / 38 | 1,061 / 636 | 17 / 12 | 353 / 171 |
| Masa CMS, configured | 5,401 | 100 / 12 | 4,561 / 90 | 637 / 2 | 102 / 31 |
| Masa CMS, automatic mappings | 7,765 | 1,294 / 47 | 5,636 / 132 | 729 / 14 | 105 / 25 |
| TestBox, presets | 292 | 10 / 8 | 158 / 68 | 40 / 15 | 84 / 11 |
| TestBox, none | 648 | 34 / 18 | 425 / 97 | 98 / 16 | 91 / 18 |

(Object and return-type findings count as "defined" when the *called* method exists somewhere,
which says little about the missing object; read those two columns for their totals.)

## How candidates are found

Candidates are deliberately the least reliable matching there is, and so they are only ever
annotations, ranked by what else the code around the finding says:

- **A receiver** (variable or return type): the components that declare **every** method the
  same function calls on that receiver — what the receiver is used for is the evidence. The
  pool is the files declaring the rarest of those methods, each checked with its extends chain.
  A component **named like the receiver** (`contentBean` and `contentBean.cfc`, `oUser` and
  `User.cfc`) ranks first, then the nearest by directory.
- **A method**: every definition of it in the index. One in a subclass of the component the
  call was checked against ranks first, then the nearest.
- **An object**: the files named like the last segment of the path that names nothing.

Confidence: **high** is the only component declaring every method called on the receiver when
there are several methods, or the only one and named like it; **medium** is a sole match on
weaker evidence, or the one named like the receiver among several; **low** is one of several.
Where there are several, the report lists them all.

On Masa CMS, configured, the top high-confidence groups are what the code holds:
`rc.contentBean` → `contentBean.cfc` (six methods, and the name), `arguments.renderer` →
`contentRenderer.cfc`, `variables.configBean` → `configBean.cfc`, `attributeBean` →
`extendAttribute.cfc` (eight methods, no name match).

## Open questions — where your knowledge of these projects helps

1. **Framework conventions a preset could state.** Several large groups are values a
   framework hands its code by convention, which the source never types:
   - Mura event handlers' `event` argument (Masa: 407 `arguments.event`, 219 `event`). A preset
     entry typing it as `mura.servletEvent|mura.event` was measured at 634 removed and 114 added
     (the added are honest "no return type" and 6 "not found"), and is **held back**, not
     committed, pending your view.
   - Mura's dual-mode accessors `$.event()`, `$.content()`, `$.currentUser()`: an object with no
     arguments, a value with one. Typing them needs a per-call reading of the argument count.
   - Lucee admin `driver`/`field` (Lucee: 160 + 58): components listed from packages at run time.
2. **Real defects the lists contain.** TestBox's `getStringName()` (41) is called bare in
   `Expectation.cfc` but declared only on `Assertion`; CFML never routes an unscoped call to
   `onMissingMethod`, so those failure-message paths would throw. Lucee's `messaging.cfm` calls
   `toFile()`, declared only in a sibling page.
3. **Config, not code.** Lucee's specs include `/admin/...`, a mapping only Lucee's build script
   creates; ContentBox's old upgrade patches name `coldbox.system.orm.hibernate.util.ORMUtilFactory`,
   removed from ColdBox. Each is one `mappings` entry or a known-dead path.
4. **The pending `loadBy(returnFormat="self")` decision** (Masa) still governs whether
   `getBean('x').loadBy(…)` chains are typed.

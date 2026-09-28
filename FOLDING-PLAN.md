# Folding: what is missing and how to add it

`textDocument/foldingRange` folds from the LSP's own parser, not from tree-sitter
(`internal/server/folding.go`). Today it covers every function and every comment
spanning more than one line: the scopes from the cached `ParseResult`, and
`parser.CommentSpans`. That is deliberately a first step. This file records what
tree-sitter folded that the parser does not, measured, and how each part can be
added from the parser.

`folding` stays off by default until enough of this lands (see
[§5](#5-when-to-switch-the-default-on)). An editor that is given folding ranges
uses them *instead of* its own indentation folding, so switching folding on
today trades a fold for every indented block for fewer, exact ones.

## 1. What is missing, measured

The six-project corpus (ColdBox, ContentBox, TestBox, Lucee, cfwheels, FW/1:
7,509 files) folded by the tree-sitter implementation this replaced, with each
fold labelled by the grammar node that produced it, then matched against the
parser's folds:

| Group | Tree-sitter folds | Share | Missing now |
|---|---:|---:|---:|
| Script control blocks: `if`/`else`, `try`/`catch`/`finally`, loops, `switch`/`case` | 44,701 | 28.6% | 44,700 |
| Closures and multi-line calls | 32,976 | 21.1% | 32,932 |
| Functions | 20,947 | 13.4% | 776 |
| Comments | 15,994 | 10.2% | 3,486 |
| Multi-line expressions and literals | 13,196 | 8.4% | 13,164 |
| HTML elements | 9,315 | 6.0% | 9,311 |
| CF tags in tag files | 8,916 | 5.7% | 8,067 |
| Component body (`component { … }`) | 5,297 | 3.4% | 5,297 |
| Script-syntax tags (`transaction { }`, `lock { }`, `query`) | 3,066 | 2.0% | 3,066 |
| `#...#` spans | 571 | 0.4% | 571 |
| Other | 1,262 | 0.8% | 1,262 |
| **Total** | **156,241** | | **122,632** |

Four things in that table need reading correctly:

- **"Closures" is mostly TestBox.** 28,894 of the group are multi-line
  `expression_statement`s, and about 18,700 of those are spec blocks —
  `it(…, function(){…})` alone is 12,961, then `describe`, `beforeEach`,
  `afterEach`, `given`/`then`, `story`. In a spec file these *are* the
  functions, so a spec currently folds to almost nothing.
- **The 3,486 missing comments are not missing.** Tree-sitter's comment node
  starts at the end of the line *before* the comment, so it folded a one-line
  comment together with the line above it (about 3,100), or a multi-line one a
  line early (363). The parser folds the comment's own lines. A few dozen real
  differences remain, the largest a `/** */` above a tag-syntax `<cfinterface>`
  (2 files), which a tag region does not look for.
- **The 776 missing functions were a parser bug, not a folding gap.** See
  §2.1; 8 remain.
- **"Other" is noise worth leaving**: multi-line `<cfset>`, `<cfsavecontent>`
  bodies, destructuring patterns, `static { }` initialisers.

## 2. The plan, in order

Ordered by what each step is worth against what it costs. Each is a separate
change with its own corpus comparison (§4).

### 2.1 Fix the scope of a function with trailing attributes — done

```cfml
public void function testAbort() localmode=true skip=true {
remote function getName() restPath="name" httpMethod="GET" {
private function cached(id) cachedwithin=createTimeSpan(0,1,0,0) {
```

Every reader of a script function declaration expected the `{` straight after
the `)`. Finding an attribute there, it recorded a scope ending on the
function's own first line and read the body as component-level code.
`skipFunctionAttrs` (`cfparser.go`) now steps over the attributes for the parse,
for `findScriptFuncScopes` (behind `ParseVars`) and for a named nested
function. The parse reads each value as the expression it is, so the
`createTimeSpan` in either spelling is still recorded as a call.

Measured over the corpus, 241 files changed:

- **Scopes.** Those ending on their own first line went from 728 to 10, and the
  10 left are genuine one-line functions (`function setUp(){}`).
- **Variables.** 794 function locals are no longer reported as variables of the
  component, and none were added.
- **Callers.** Calls with no caller went from 5,511 to 200, because the calls
  are now attributed to their function. 22 duplicate calls, recorded once from
  the function and once from the component-level read, are gone.
- **`unresolved`.** About 2,100 entries changed, almost all only in their
  `caller`, and 19 changed in substance. The largest group is TestBox's
  `runRemote(…) output=true`: its local `var runner = new
  testbox.system.TestBox(…)` had been filed as a component-wide reference, so
  every other function's untyped `runner` argument resolved through it.
- **Folds.** Function folds missing against tree-sitter went from 776 to 8.

Reading these bodies as function bodies exposed two existing gaps, which every
ordinary function had as well, since fixed on their own — CLAUDE.md's parser
notes have the rules:

- `return variables.a.b().c()` recorded `c` as a bare call with no receiver,
  where the same chain assigned or written as a statement kept it.
- `x = variables.f()` on an assignment's right-hand side recorded `variables` as
  the receiver object, where a statement `variables.f()` is recorded
  unqualified.

### 2.2 A bracket pass over script — done

`parser.StructureSpans` replaced `CommentSpans`: one pass over each script
region with the parse's scanner, a stack of open brackets, and the rules below.
Measured against the tree-sitter folds over the corpus:

| Group | Before | After |
|---|---:|---:|
| Script control blocks | 0% | 99.5% |
| Closures and multi-line calls | 0.1% | 98.6% |
| Component body | 0% | 99.9% |
| Script-syntax tags | 0% | 99.2% |
| Expressions and literals | 0% | 78.8% |
| **All folds** | **22.0%** | **83.5%** |

937 of its 131,420 folds are ones tree-sitter did not make. The plan below
predicted most of the rules; four were found by the corpus comparison:

- **CFScript without semicolons.** cfwheels omits them throughout, and without a
  rule every fold in such code started on some earlier statement's line. A
  newline ends a statement in a block when the last token could end an
  expression and the next is a word that is not an operator.
- **Attribute statements.** `component`, `property` and script-syntax tags
  (`admin action="x"` over several lines) broke that rule, one attribute per
  line. Inside one, a word on a new line is another attribute when `=` follows
  it — `default="x"` included, though `default` is a keyword — or a valueless
  one when a `{` or the next line does. A keyword or anything else on its own
  line begins a new statement. One ending in `;` folds as a statement, as
  tree-sitter folded a tag statement.
- **Whole chains.** Tree-sitter folds an `if … else …` and a `try … catch …` as
  one statement as well as branch by branch, and an `else if` to the end of the
  chain. VS Code keeps one fold per line, the outermost first, so the chain is
  what a user of the tree-sitter version saw on an `if` line.
- **A bracket not on its statement's first line** folds from its own line too,
  as tree-sitter folded a statement block and an argument list.

Left as they are: multi-line binary expressions and conditions (about 1,500,
noise), and method chains and concatenations inside declarations (about 420).
The design notes that follow are kept for the reasoning; §2.3 is next.

#### Design as planned

One pass over each script region with the parse's scanner, in CFScript mode —
the same tokenisation `CommentSpans` already does — keeping a stack of open
`{`, `(` and `[`. Folding every pair that spans lines covers, in one piece of
work, the script control blocks, closures, struct and array literals, the
component body and script-syntax tags: more than half of everything
tree-sitter folded.

It should replace `CommentSpans` rather than sit beside it, so a request
tokenises the document once. Call the result `StructureSpans`, returning spans
with a kind (comment, block, list), and let the server decide the fold.

The rules that make this match tree-sitter, rather than merely resemble it:

- **A block folds from its statement's first line, not from its `{`.** When an
  `if` condition wraps, tree-sitter's `if_statement` starts at the `if`, and the
  fold arrow belongs there. Track the start of the current statement (the first
  token after `;`, `{` or `}`) and start a `{` block's fold on that line.
- **`} else {` and `} catch (e) {` split correctly without a special case.** The
  `if` block ends on the line before `} else {`, and the `else` block starts
  there, which is what tree-sitter's `if_statement` and `else_clause` produce.
- **The closing line stays visible**: fold to the line before the one holding
  the closing bracket, as functions already do. A closing bracket that shares
  its line with code (`return x; }`) still folds to the line before; that is
  what the tree-sitter version did.
- **Parentheses and brackets fold only when their content spans lines**, and
  should come in a second step after braces. Multi-line argument lists and
  parameter lists (about 4,800 folds) are useful. A multi-line binary
  expression, a `return` of one or a parenthesised condition (about 4,000) are
  noise, so leave those out even though tree-sitter folded them.
- **`case` labels have no brackets.** A `switch` case folds from its `case` line
  to the line before the next `case`, `default` or closing `}`: 1,638 folds, a
  small addition once the stack exists.
- **Variable declarations and assignments mostly come for free.** Tree-sitter
  folded 4,786 multi-line declarations: 4,265 `var x = …` and 521 plain
  assignments. About 4,400 of them hold a bracketed right-hand side, and the
  statement-start rule gives each the same range tree-sitter gave the whole
  declaration:

  | Right-hand side | Folds |
  |---|---:|
  | A call with multi-line arguments, including `new`, `createObject` and `queryExecute` | 2,332 |
  | Struct literal | 1,594 |
  | Array literal | 440 |
  | String concatenation | 209 |
  | Method chain split across lines | 211 |

  The last two rows, about 420, have no bracket spanning the statement:
  a chain such as `var app = builder( x )` followed by `.authority( … )` and
  `.build();` on lines of their own, and strings joined with a trailing `&`.
  To cover them, a statement spanning lines with no bracket pair over it would
  fold from its first line to the line before its last. That is
  cheap, since the pass tracks where a statement starts, but it is a separate,
  optional step. Measure it before taking it: 44% of all declaration folds are
  two or three lines long, and folding a three-line chain is close to noise.
- **Folds on the same lines are one fold.** The component body and the file
  can coincide (a script `.cfc` with nothing outside `component { }`), and a
  closure passed to a call can share both lines with the call. Dedupe as the
  handler already does.

### 2.3 A tag pass over markup — done

`tagStructure` (`internal/parser/tagstructure.go`) walks the markup with a stack
of open tags. A closing tag pops to its opener, and whatever is left open above it
is discarded. Measured against the tree-sitter folds over the corpus:

| Group | Before | After |
|---|---:|---:|
| HTML elements | 0% | 89.2% |
| CF tags in tag files | 9.5% | 91.2% |
| Other (multi-line `<cfset>`, `<cfsavecontent>`, …) | 34.5% | 80.6% |
| **All folds** | **83.5%** | **93.9%** |

`<cfif>` is at 99.1% and `<cfscript>` at 1,374 of 1,375. `<cfquery>`,
`<cffunction>`, `<cfsavecontent>`, `<script>` and `<style>` match exactly.

Where it differs from the plan above, and why:

- **One walk over the whole file**, stepping over the script regions, not one
  per tag region. ClassifyRegions cuts a region at `<cfscript>` and resumes after
  `</cfscript>`, leaving both tags between regions. A walk per region never sees
  the pair.
- **A `<cfif>`'s branches all run to the last content before `</cfif>`**, not to
  the next branch. Tree-sitter reads a `<cfelseif>` as holding every branch after
  it. A branch has no closing tag of its own to keep on screen, so, like a switch
  case, it keeps its last line.
- **A `<script>` holding a CF tag is walked**, not skipped, as ClassifyRegions
  treats it. Only CF-free blocks are raw text, where `a<b` and a `"</div>"`
  string are not tags.
- **A CF tag inside an HTML tag ends it**: `<input <cfif a>checked</cfif>>` and
  a `<tr` whose attributes are wrapped in `<cfif>`. Swallowing the CF tag as an
  attribute left its `</cfif>` unpaired.
- **An opening tag whose attributes wrap folds by itself**, bodiless or not.
  That covers a multi-line `<cfset>` and tree-sitter's 523 `start_tag` folds in
  one rule.
- **A page of plain HTML is walked as markup.** ClassifyRegions reads a file
  with no CF tag as CFScript, which is the parse's rule. A file whose first
  token is `<` cannot be CFScript.
- **Not done:** a test holding this walk and `FindMatchingTag` to the same
  pairing. They answer different questions — every pair in one forward walk,
  against one tag's partner found by searching outward — and they disagree
  wherever one of them meets an implied close.

What remains is mostly where tree-sitter's own reading is doubtful. It closes
`<td>` and `<li>` implicitly. It gives a lone `<hr>` a fold. It lets an
unclosed custom tag such as `<cfinputClassic>` or `<cfchartdata>` hold
everything up to its parent's close. 758 of the 1,040 element misses are in
Lucee's admin pages, which are written that way.

### 2.4 Leave out

- `#...#` spans (571) and the "other" group (1,262): multi-line interpolation
  and destructuring are rare, and folding them is noise.
- Multi-line binary expressions, returns and parenthesised conditions — see
  §2.2.

## 3. Cost

`CommentSpans` already tokenises every script region on each request. On the
largest corpus files that is 1.5ms for a 4,965-line component and 2.0ms for a
10,751-line one; the bracket pass is the same tokenisation with a stack beside
it, so it should cost about the same. A 65,000-line file would be around 10ms.

If that becomes a problem, cache the spans per document and clear them in
didChange, as the parse is. That would be the first cache folding needs. It does
not need one today: the 60-function benchmark is 48µs.

## 4. How to verify each step: keep tree-sitter as the oracle

The tree-sitter implementation is independent of the parser, so where the two
disagree one of them is wrong — the same argument as `make gapcheck`. Each step
above should be measured against it rather than against fixtures:

- **Restore the tree-sitter fold walk as a test-only oracle**, from
  `internal/server/folding.go` at `d8dffc1` into `internal/tsoracle`, which
  already depends on tree-sitter. It should label each fold with its node kind,
  as the measurement in §1 did.
- **Add `make foldcheck CORPUS=<dir>`**, skipped without a corpus like
  `make corpus`. It should report, per group, the folds the parser now
  produces, the ones still missing, and any the parser produces that tree-sitter
  did not. It should also list the known tree-sitter artefacts separately (the
  early-starting comments) so they do not read as misses.
- **Diff per file, not by totals.** A step that fixes one file's folds and
  breaks another's leaves every total the same; `make corpus BASELINE=` exists
  for exactly this reason.
- **Confirm each new test fails with its rule removed.** The tree-sitter
  version's tests did, and three of its rules were found only that way.

## 5. When to switch the default on

§2.1 to §2.3 have landed. The parser covers 93.9% of what tree-sitter folded,
including every block an indentation fold would have found in a script file and
the tag structure of a page. Switching folding on is now an improvement rather
than a trade, and `config.foldingDefault` can become `true`.
`featureDefaults` in `features_chain_test.go` states every default and has to
change with it. That is a change to what every user sees, so it is left for its
own decision.

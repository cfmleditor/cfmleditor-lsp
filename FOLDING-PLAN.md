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
- **The 776 missing functions are a parser bug, not a folding gap.** See §2.1.
- **"Other" is noise worth leaving**: multi-line `<cfset>`, `<cfsavecontent>`
  bodies, destructuring patterns, `static { }` initialisers.

## 2. The plan, in order

Ordered by what each step is worth against what it costs. Each is a separate
change with its own corpus comparison (§4).

### 2.1 Fix the scope of a function with trailing attributes — a parser bug

```cfml
public void function testAbort() localmode=true skip=true {
remote function getName() restPath="name" httpMethod="GET" {
```

A script function with attributes after its parameter list gets a `FuncScope`
that **ends on its own first line** (`Start == End`). The `FunctionDef` is
recorded, so the function still appears in document symbols and the index.
But everything that maps a line to its enclosing function through `pr.Scopes`
reads that function's body as *outside* it: `callerAtLine` (behind
`fillCallers`, which names the caller of every call), `findFuncScope`, and
`ApplyEdit`'s `funcContaining`, which decides whether an edit is inside a
function.

739 of the 776 missing function folds are this shape, nearly all in Lucee's
test suite (`skip=`, `localmode=`, `restPath=`). Fix it in the parser first,
with a test in `cfparser_test.go` that asserts the scope's end line; the fold
follows for free. Measure the other consequences on the corpus as well — call
attribution will change for every call inside such a function — since that is
the larger effect.

### 2.2 A bracket pass over script: blocks, closures, literals, the component body

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
- **Folds on the same lines are one fold.** The component body and the file
  can coincide (a script `.cfc` with nothing outside `component { }`), and a
  closure passed to a call can share both lines with the call. Dedupe as the
  handler already does.

### 2.3 A tag pass over tag regions: CF tags, branches, HTML elements

About 18,000 folds (12%), and nearly all of what a `.cfm` page has: today a page
folds only its comments and its `<cffunction>`s.

One pass over each tag region, keeping a stack of open tags:

- Push on `<name …>`; on `</name>` pop to the matching entry and fold it.
  Unclosed tags between the two are discarded — that is how `<cfset>`,
  `<cfreturn>`, `<cfargument>`, `<br>`, `<img>` and `<input>` fall out without a
  list of which tags have bodies, and how an unclosed `<p>` does no harm.
  Self-closing `<… />` never pushes.
- Find a tag's end with `tagEndIndex`, which steps over quoted attribute
  values, not with a bare search for `>`. Step over `<!--- --->` and `<!-- -->`
  so a tag inside a comment is not read.
- **`<cfelse>` and `<cfelseif>` start a branch** that ends on the line before
  the next branch or the closing `</cfif>`; the `<cfif>` itself folds to the
  line before its first branch. This is the case the tree-sitter version
  handled with its leading-whitespace rule, and it had a test
  (`TestFoldingRangesInATagDocument`, as of commit `d8dffc1`). `<cfcase>` and
  `<cfdefaultcase>` inside `<cfswitch>` are the same shape.
- **A `<script>` or `<style>` element folds as one element** and is not looked
  inside: its content is a `RegionSkip`, JavaScript or CSS, not CFML.
- **Match tag names case-insensitively**, CFML being case-insensitive, with the
  allocation-free fold the parser uses elsewhere (`fold.go`).

`internal/parser/tags.go` already pairs tags for go-to-matching-tag
(`FindMatchingTag`, `findOpenTagBefore`, `findCloseTagAfter`), but it answers one
position at a time by searching outward. The pass here is a single forward walk.
The two should agree on what pairs with what, and a test should hold them to it.

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

When §2.1, §2.2 (braces) and §2.3 have landed, the parser will cover about 85%
of what tree-sitter folded, including every block an indentation fold would
have found in a script file and the tag structure of a page. At that point
switching folding on is an improvement for everyone rather than a trade, and
`config.foldingDefault` can become `true`. `featureDefaults` in
`features_chain_test.go` states every default and has to change with it.

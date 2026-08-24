# Formatter: non-whitespace change audit

Findings from formatting a corpus of real-world CFML through
`internal/formatter` and checking each result against the `whitespaceOnly`
guard (`checkWhitespaceOnly`, `internal/formatter/formatter.go`).

The formatter is meant to be whitespace-only: `formatting.whitespaceOnly`
defaults to `true` (`internal/config/config.go`), and `Format` rejects its own
output when the non-whitespace character stream changed. Everything below is a
case where the formatter *did* change non-whitespace content, or where the
guard failed to notice that it had.

Most of these are fixed. Section 4 lists what is still outstanding.

## 1. Corpus and method

Two corpora. The repository's own fixtures, and 5,620 files from six
open-source CFML projects:

| Project | Files |
|---|---|
| Lucee (`lucee/Lucee`) | 3,775 |
| ContentBox (`Ortus-Solutions/ContentBox`) | 724 |
| ColdBox (`ColdBox/coldbox-platform`) | 655 |
| FW/1 (`framework-one/fw1`) | 305 |
| TestBox (`ortus-solutions/testbox`) | 145 |
| cfmleditor (`cfmleditor/cfmleditor`) | 16 |

Each file was parsed with `language.CFML`, formatted with
`formatter.DefaultOptions()` plus the three sub-parsers and
`WhitespaceOnly: true`, and compared against its input. Files that formatted
cleanly were then formatted a second time to check idempotency.

### Results

| | Before | After the audit | Current |
|---|---|---|---|
| Formatted cleanly | 3,863 | 5,450 | **5,508** |
| Rejected by the guard | 1,671 | 84 | **23** |
| Refused: grammar cannot parse | 86 | 86 | **83** |
| Not idempotent | 390 † | 36 | **3** |
| Panics | 0 | 0 | **0** |

† measured at the post-fix corpus size; the pre-fix figure of 50 covered a much
smaller pool, since a file the guard rejects never reaches the idempotency
check. Comparing like for like, the same 5,450 files went from 390 unstable to
36.

The "current" column is what `make corpus` prints today (section 5), against the
same six projects at their current HEAD. It counts the grammar's 83 refusals in
two buckets rather than one — 22 documents the CFML grammar cannot parse, and 61
that parse as documents but whose embedded cfscript or cfquery the sub-grammar
cannot — because the two are different work and the second is invisible from the
outside: the document parses, the formatter runs, and whatever it renders for
that region is a guess.

Per project, current:

| Project | Files | Clean | Parse-refused | Script-refused | Guard-rejected | Unstable |
|---|---|---|---|---|---|---|
| Lucee | 3,775 | 3,682 | 23 | 54 | 15 | 1 |
| ContentBox | 724 | 719 | 2 | 1 | 2 | 0 |
| ColdBox | 655 | 644 | 0 | 5 | 5 | 1 |
| FW/1 | 305 | 304 | 0 | 0 | 1 | 0 |
| TestBox | 145 | 143 | 0 | 1 | 0 | 1 |
| cfmleditor | 16 | 16 | 0 | 0 | 0 | 0 |

The repository's own `testdata/` went from 30/39 clean to 38/39, the last being
`DefinitionTestTag.cfc`, which the grammar cannot parse (see 2.1).

### Why the entry point matters

- The **LSP path** (`internal/server/formatting.go`) refuses a document whose
  tree has an `ERROR` node before calling `Format`, and passes `WhitespaceOnly`
  through from config. Symptom of a bug there is "format-on-save silently does
  nothing".
- The **`format` CLI** had neither check: it built `formatter.DefaultOptions()`,
  which leaves `WhitespaceOnly` at `false`, and never inspected the tree.
  Symptom there was silent file corruption. Fixed — see 2.2.

## 2. Fixed

### 2.1 Corrupt output from unparseable trees

`Format` walked `ERROR` nodes with no rendering for them and fell through to a
raw emit that concatenated their children without separators. A body-less
`<cfinvoke>` or `<cfhttp>` inside `<cfcomponent>` — valid CFML the grammar
cannot parse — came back as:

```
<cfinvokecomponent="models.Widget"method="render"returnvariable="r"></cf>
```

Tag name and attributes run together, `</cfcomponent>` dropped, a bogus `</cf>`
appended. `Format` returned a `nil` error; only the guard caught it, and only
because the streams ended up different lengths.

`Format` now refuses any tree containing an `ERROR` or `MISSING` node, naming
the offending line. The construct remains unformattable, but it can no longer
produce garbage.

### 2.2 The `format` CLI wrote corrupt output and reported success

```console
$ wc -c victim.cfc
1521 victim.cfc
$ cfmleditor-lsp format -w victim.cfc
formatted victim.cfc          # exit 0
$ wc -c victim.cfc
1411 victim.cfc               # 110 bytes gone, no longer parses
```

`cmdFormat` now defaults `WhitespaceOnly` to `true` (matching
`config.Resolve`), with `--allow-non-whitespace` to opt out, and only rewrites
a file after `Format` succeeds. A batch run reports every failing file instead
of exiting on the first.

### 2.3 UTF-8 BOM stripped — 554 files

A leading BOM sits outside every CST node, so the walk never emitted it. Every
BOM-prefixed file silently lost its encoding preamble. Now carried across
verbatim, and never invented for files without one.

### 2.4 Function attributes hoisted before `function` — 108 files

Attributes written after the parameter list are siblings of the parameters, but
`scriptFunction` appended every non-field child to the signature *prefix*:

```cfml
function setup() localmode="true" {}   ->   localmode="true" function setup() {}
```

The output does not compile. Seen with `localmode`, `skip`, `restpath`,
`httpmethod`, `output` and `hint`. Attributes are now emitted between the
parameter list and the body.

### 2.5 `query` and `function` return types dropped

The signature prefix was gated on `IsNamed()`, but the grammar tokenises some
type and modifier keywords as *anonymous* nodes:

```
function_declaration
  access_type  named=true   "public"
  query        named=false  "query"     <-- dropped
  function     named=false  "function"
```

`public query function f()` became `public function f()`. Of the fourteen CFML
return types plus dotted component paths, only `query` and `function` were
affected — the rest arrive as named `identifier` nodes. Anonymous children other
than the `function` keyword are now kept, and only the first `function` token is
treated as the keyword so `function function f()` survives.

### 2.6 Catch clauses and catch types dropped — 194 files

Two defects in `scriptTry`:

- Every catch clause carries the same `handler` field name, so
  `ChildByFieldName` returned only the first. A `try` with two catches lost the
  second **along with its body**.
- The exception type is a separate `type` field. Rendering only the `parameter`
  turned `catch (java.lang.Exception e)` into `catch (e)`, silently widening
  what the handler catches.

Clauses are now walked as children and rendered as `catch (<type> <param>)`.

### 2.7 `interface` rewritten as `component`; `abstract`/`final` dropped

`scriptComponent` hardcoded its header to `"component"`, so `interface {}`
became `component {}` — changing what the file declares — and the modifiers,
also anonymous nodes, vanished. Declaration keywords are now emitted in source
order from an explicit keyword set.

### 2.8 `::` static access rewritten to `.` — 24 files

`member_expression` hardcoded `"."`, so Lucee/BoxLang static access
`Widget::getData()` became the instance call `Widget.getData()`. The accessor
now comes from the node; `::` arrives as a named `static_chain` child rather
than an anonymous token.

### 2.9 Comments commenting out the code around them — 108 files

`exprArray`/`exprObject` walked `NamedChild`, which includes comments, treated
them as elements, and joined everything with `", "`:

```cfml
var routes = [                 var routes = [// leading comment, { pattern: "/",
    // leading comment    ->   handler: "home" }, { pattern: "/x", handler: "x" }];
    { pattern: "/", ... },
    { pattern: "/x", ... }
];
```

The whole statement is inside a line comment. `exprArgs` had the same defect
for call arguments, via a comment test that only recognised `cf_comment` and
not the cfscript `comment` kind.

Literal and argument children are now classified as elements or comments; a
line comment forces the construct onto several lines and never takes a
trailing comma. Block comments still inline.

### 2.10 Comments deleted in "between" positions

A comment belonging to no field was skipped past and lost:

- between a block and its `else` / `catch` / `finally`
- between a chained call and its next `.hop()`

Both are now emitted, with the continuation keyword or chain hop moving to its
own line. With no comment present, `} else {` still sits on one line.

### 2.11 Invented closing tags — ~100 files

`formatCFTag` closed every `cf_tag`, so a tag legal without a body gained a
closing tag it never had and every following sibling was re-parented into it:

```cfml
<cfmodule template="a.cfm">        <cfmodule template="a.cfm">
<p>after</p>                  ->       <p>after</p>
                                   </cfmodule>
```

Affected `cfmodule`, `cfhttp`, `cfinvoke`, `cffeed` and `cfadmin`; the other
fifteen void-ish CF tags were already correct. `hasRealCFEndTag` now checks for
an actual `cf_end_tag` child — an unclosed tag has either none or only the
grammar's synthetic `implicit_cf_end_tag` marker.

### 2.12 The guard vetoing the formatter's own canonicalisation — 527 files

The formatter deliberately adds braces around single-statement bodies and
semicolons to statements written without them. Both are non-whitespace changes,
so `whitespaceOnly` rejected them and the LSP silently declined to format 9.4%
of real-world files with no indication why.

`checkWhitespaceOnly` now skips an inserted `;`, `{` or `}` on the output side,
in the same spirit as the existing self-closing-slash and quote allowances. Two
things keep it narrow: the allowance is one-directional, so a token the
formatter *dropped* still fails; and added braces are counted and must balance.

`guard_test.go` pins this down with the twelve real defects above — all still
rejected.

### 2.13 Formatting not a fixed point — 390 files

Two cases where one pass left work the next pass performed, so format-on-save
kept producing a fresh diff for an unchanged file:

- `scriptBlockOf2` wrapped a single-statement body in braces *tightly*, while
  `scriptBlock` pads the inside of a real block with blank lines. On the second
  format the braces were in the source, so the same code took the padded path.
- `preformat` replaces a converted element whole, so `collectEdits` could not
  descend into its body and any void element nested there survived the pass.
  `<p>text<br>more</p>` inside a converted parent kept its `<br>` until a later
  run. `preformat` now repeats until the source stops changing.

## 3. Guard coverage gaps

Cases the `whitespaceOnly` guard passed and should not have. The first two were
latent — nothing in the corpus triggered either — but they meant the "clean"
figures were an upper bound rather than a proof. The last two were not latent:
each was destroying real files while the guard reported success, because a
change can be whitespace-only and still change what the file means. All four
are closed.

### 3.1 CFML comments were skipped entirely — fixed

`skipWSAndComments` advanced past `<!--- … --->` on both sides before
comparing, so comment *content* never entered the compared stream: rewriting,
deleting or injecting a whole CFML comment was invisible.

It now collects each comment body as it steps over it and compares the two
sinks once both sides are exhausted (`compareCommentBodies`), which is why a
comment cannot be compared in line — the formatter is allowed to *move* one.
Rewriting, deleting and injecting are all rejected.

### 3.2 `selfCloseTags` disabled quote checking across the whole file — fixed

The allowance was written as "any mismatched `"` or `'` on either side", gated
on `allowSelfClose`. Two things were wrong with that.

The check was unanchored, so it applied to string literals and SQL, not just
attribute values. With `selfCloseTags` at its default of `true`:

| Source | Output | Verdict (before) |
|---|---|---|
| `<cfset msg = "hello world">` | `<cfset msg = hello world>` | passed |
| `<cfquery>SELECT 'a' FROM t</cfquery>` | `<cfquery>SELECT a FROM t</cfquery>` | passed |

The formatter stripping every quote out of a `<cfset>` was invisible to the
guard by default.

The fix follows from what the formatter actually does. `normaliseAttrValue`
produces exactly two shapes: an unquoted value *gains* quotes, and a
single-quoted one is *upgraded* to double. Neither removes a quote. So the
allowance is now:

- a quote on the output side where the source has none — an addition;
- a quote on each side that differ — a substitution, consumed on both sides.

A quote the formatter dropped is compared like any other byte. The allowance
also moved off `selfCloseTags` onto `doubleQuoteAttributes`, the option that
performs the re-quoting; `selfCloseTags` still governs the `/>` rule alone.

Re-running the corpus after the change moved no file between categories —
nothing in 5,620 real files relied on the removal allowance, confirming it was
pure blind spot rather than a load-bearing exception. Covered by
`TestGuardRejectsDroppedQuotes`, `TestGuardAllowsAttributeRequoting` and
`TestGuardRequoteGatedOnItsOwnOption`.

### 3.3 Whitespace-only is not a sufficient invariant for `<pre>` — fixed

The two gaps above were the guard failing to notice a change. This one is the
opposite: the guard worked exactly as specified, and the specification was
wrong.

`<pre>` and `<textarea>` went through the generic element path and had their
bodies collapsed onto one line:

```
<pre>              ->  <pre>
line one                   line one indented line three
    indented           </pre>
line three
</pre>
```

Nothing but whitespace changed, so `checkWhitespaceOnly` passed it — correctly,
by its own definition. But in these two elements the whitespace *is* the
content, and the rendered page is destroyed. No amount of guard work can catch
this, because the guard's entire premise is that whitespace is free.

The fix is a carve-out rather than a guard change: an element whose tag is in
`htmlPreformattedElements` is reproduced from source instead of walked
(`isPreformattedElement`, `internal/formatter/element_formatter.go`). Covered by
`TestPreformattedElementsKeepTheirWhitespace` and, in the other direction,
`TestOrdinaryElementStillCollapses` — a `<div>` must still be reflowed or the
carve-out is too wide.

Worth remembering as a class: "the guard passed" means "no non-whitespace
character changed", which is only equivalent to "nothing was destroyed" where
whitespace carries no meaning. `<pre>` is the case where that does not hold;
another would be any construct the grammar exposes as text but a runtime treats
as significant.

### 3.4 Line wrapping broke inside quoted attribute values — 43 files

`writeWrapped` reflows a long line by breaking at the last space before
`lineWidth`. It is handed whole elements *verbatim* — the "emit this element
as-is" path in `formatElement` passes `f.text(n)`, markup and attributes
included — so the space it picked was often inside an attribute value:

| Source | Output (before) |
|---|---|
| `<img src="x.png" alt="a fairly long alternative text describing the picture">` | `alt="a fairly long`<br>`alternative text describing the`<br>`picture" />` |

The guard cannot see this: only whitespace changed, which is exactly what the
guard permits. But the attribute's *value* changed, and for a CFML tag whose
attribute carries a string the runtime uses — a `cfhttpparam` value, a `cfmail`
subject — the injected newline and indentation are in the data.

Break points are now computed once over the whole string (`safeBreaks`),
skipping any space inside a tag's quoted value. Two details matter:

- **Once, not per line.** The offsets depend on tag and quote state a per-line
  scan cannot reconstruct: slicing the first line off `<img src="a" alt="b c">`
  leaves `alt="b c">`, which no longer starts inside a tag. The first version of
  the fix did it per line and kept breaking inside values.
- **Quotes only count inside a tag.** The same text stream carries ordinary
  prose, where an apostrophe is a letter. Tracking quotes everywhere made
  `I won't display because…` unbreakable from the apostrophe onward — wrapping
  silently switching off for ordinary English. That regression is pinned by
  `TestWrapStillWrapsProseContainingApostrophes`.

Measured by formatting all 5,504 formattable corpus files and looking for a
quoted attribute value that gained a newline: 43 before, 0 after. Per-file
corpus verdicts are byte-identical to the baseline, so nothing moved category.
Covered by `internal/formatter/wrap_test.go`.

## 4. Outstanding

Counts from the current `make corpus` run (section 5).

| Issue | Files | Notes |
|---|---|---|
| Grammar cannot parse the document | 22 | Refused safely rather than corrupted. Needs grammar work in `tree-sitter-cfml`, not the formatter. |
| Grammar cannot parse embedded cfscript/cfquery | 61 | The document parses, so the formatter runs and renders those regions blind. Also grammar work, but the failure mode is worse: some of these files are also guard-rejected, and the rest are formatted from a tree with an `ERROR` node in it. |
| Guard-rejected, long tail | 23 | 11 in Lucee's `test/` directory. No bucket larger than three files left; the remainder are one- and two-file causes, five of them comment-text changes and one a content-length mismatch. |
| Not idempotent | 3 | One file whose formatted output no longer parses (`jquery.blockUI.js.cfm` — JavaScript in a `.cfm`), and two whose second pass is refused by the cfscript sub-parser. |
| `final component` body not formatted | — | Not a formatter bug: the *document* grammar does not accept `final` on a component at the top of a `.cfc`, in any position or case, and degrades to `html_text` + `text` rather than an `ERROR` node. The formatter therefore emits the body verbatim, the change is whitespace-only, the guard passes it, and the corpus counts the file **clean**. `component` and `abstract component` parse normally. See 6.2. |

Fixed since the audit table above, all found by re-running the harness:

- `final susi = "foo";` (a Lucee/BoxLang immutable declaration) came back as
  `var susi = "foo";`, and `var final y = 2;` came back as `var y = 2;` — the
  keyword silently replaced rather than dropped, in the second case. The
  grammar's `variable_declaration` accepts `var`, `final`, or the combined
  `final var`/`var final` as its leading keyword, each its own anonymous
  child, but the renderer's keyword-detection loop only recognised `var` (and
  a dead `local`, which the grammar has never produced here), so it walked
  past `final` every time and fell back to its `"var"` default. Shared between
  the statement-level renderer and the `for (...)` inline-declaration
  renderer, which had the identical bug. 2 files.
- `required timeUnit = "milliseconds"` — a `required` parameter with no type
  annotation — lost the `required`. `required_parameter`/`optional_parameter`,
  the wrapper node types the non-flat parameter path was written to handle,
  do not exist anywhere in the current grammar; every parameter is flat
  (`[required] [type] name [= default]` as direct siblings of
  `formal_parameters`). `hasFlatParams` only checked for a `parameter_type`
  sibling, so a parameter list with no typed member at all took the
  non-flat path, which walks named children only and silently dropped the
  anonymous `required` token beside each of them. A typed `required`
  parameter was unaffected, since its `parameter_type` sibling already routed
  the whole list through the flat path, which already handled `required`
  correctly. 3 files.
- `a?.b?.c?.d` (Lucee/BoxLang's null-safe member access) came back as `a.b.c.d`
  — the `?` silently dropped, turning a chain that tolerates a nil receiver into
  one that throws on it. The grammar wraps `?.` in a named `optional_chain`
  node, exactly as it wraps `::` in a named `static_chain` node, but
  `memberOperator` only special-cased the latter; its fallback loop walks only
  *anonymous* children, so the operator fell through to the default `"."`.
  4 files.
- `<?xml version="1.0" encoding="utf-8"?>` came back as
  `<?xmlversion="1.0"encoding="utf-8"?>`. The declaration's parts are children
  (`<?`, `xml`, `tag_attributes`, `?>`) and the generic child walk joined them
  with nothing between. **The guard cannot catch this** — only whitespace was
  removed — so the CLI wrote it to disk and exited 0, leaving a file the grammar
  can no longer parse. Same class as the doctype bug in 2.1, opposite cause.
- `new component { ... }`, an anonymous component defined at the point of use,
  was emitted as `new ()`: the `new_expression` has neither a constructor nor an
  arguments node, and rendering it from those two fields deleted the keyword and
  the entire body. 18 files.
- A CF tag written in script syntax separates its attributes with spaces
  (`cfdirectory(directory="#dir#" action="create")`), but the grammar hands the
  list over as an `arguments` node of assignment_expressions — the same shape as
  a call's arguments — and the formatter joined them with `", "`, inserting
  commas that were never in the source. 11 files.

## 5. Reproducing

The corpus scanner is checked in as `TestFormatterCorpus`
(`internal/formatter/corpus_test.go`). It is skipped unless `CFML_CORPUS` names
the corpus, so `make test` and CI are unaffected:

```console
$ make corpus CORPUS=/src/Lucee:/src/ContentBox REPORT=/tmp/corpus.tsv
    formatting 4499 files from 2 root(s)
    root                files  clean  parse script  guard unstab  panic   skip
    Lucee                3775   3677     20     54     20      1      0      3
    ContentBox            724    719      2      1      2      0      0      0
    TOTAL                4499   4396     22     55     22      1      0      3
```

`CORPUS` is a `PATH`-style list of source trees; each is reported separately so a
regression can be attributed to a project rather than to the pile. `REPORT` is
optional and writes a TSV of every non-clean file — verdict, path, and the reason
— to work through individually.

The six projects in the table above are:

```console
$ git clone --depth 1 https://github.com/lucee/Lucee
$ git clone --depth 1 https://github.com/Ortus-Solutions/ContentBox
$ git clone --depth 1 https://github.com/ColdBox/coldbox-platform
$ git clone --depth 1 https://github.com/framework-one/fw1
$ git clone --depth 1 https://github.com/ortus-solutions/testbox
$ git clone --depth 1 https://github.com/cfmleditor/cfmleditor
```

The harness formats each file exactly as `internal/server/formatting.go` does —
default options, `WhitespaceOnly: true`, all three sub-parsers wired up — then
formats its own output again to check idempotency. A panic fails the test; guard
rejections and instability are reported but do not, since they are the thing
being measured. Runtime is a few seconds for all 5,620 files.

Individual cases reproduce through the CLI. It applies the guard by default now,
so `--allow-non-whitespace` is what shows you the damage a bug would do:

```console
$ go build -o target/release/cfmleditor-lsp ./cmd/cfmleditor-lsp
$ printf 'component {\n\tpublic query function A() {}\n}\n' > /tmp/r.cfc
$ target/release/cfmleditor-lsp format /tmp/r.cfc
$ target/release/cfmleditor-lsp format --allow-non-whitespace /tmp/r.cfc
```

Regression coverage for everything in section 2 lives in
`internal/formatter/parse_error_test.go`, `internal/formatter/guard_test.go`,
`internal/formatter/idempotency_test.go` and
`cmd/cfmleditor-lsp/format_test.go`; for the three fixes in section 4, in
`internal/formatter/doctype_test.go` and
`internal/formatter/script_tag_call_test.go`.

## 6. Grammar gaps behind the refused counts

The 83 refusals in section 4 are the largest bucket left, and "the grammar
cannot parse it" is not something anyone can act on. This section reduces them
to constructs. All of it is `tree-sitter-cfml` work, not formatter work.

### 6.1 Confirmed cfscript gaps

Each was reduced from a failing file and then re-checked **standalone** against
`language.CFScript`, because the ERROR node marks where the parser gave up
rather than what defeated it — most constructs the raw error text pointed at
turned out to parse fine on their own.

| Construct | Minimal repro | Seen in |
|---|---|---|
| `name:value` function annotations | `component { function f(String x) access:remote { } }` — also `access:"remote"` and `secured:api`, so the gap is the annotation form, not one keyword | Lucee LDEV3963, LDEV5763 |
| `final` member in a `static` block | `component { static { public final MEMBER = "v"; } }` | Lucee LDEV0600 |
| Component with parenthesised settings | `component( javasettings = { } ) { public function test() { } }` | Lucee LDEV5763 |
| `default` method in an `interface` | `interface { public default any function f(any obj){ } }` | Lucee LDEV1835 |
| Bare `param` statement | `param url.number;` | Lucee Jira2605 |
| Body-less tag-in-script | `query name="local.q2" dbtype="query";` | Lucee LDEV1750 |
| Inline Java class | `classInstance = java { public class C { } };` | Lucee LDEV4001 |
| Arrow function with a statement body | `list.each((value) => if (value < 0) throw(message = "x"));` | Lucee LDEV1819 |
| Tag-form `throw` in script | `throw message="Access Denied" type="MyCustomError";` (the `throw(...)` call form parses) | Lucee LDEV1819 |
| Component-level constructs in a `<cfscript>` inside a **tag-based** component | `static { static3 = 3; }` as a whole `<cfscript>` body. `static { }` parses inside `component { }`, but a tag-based `<cfcomponent>` gives the region no such wrapper | Lucee Issue0275 |

Two neighbouring constructs do parse, and are recorded here so they are not
re-filed by mistake: a plain `static { }` block, and the ordered-struct literal
`$[ key : "value" ]`. `param name="url.x" type="numeric";` also parses — it is
only the bare `param url.number;` form that fails.

### 6.2 A document-grammar gap that does not produce an ERROR

`final component { … }` at the top of a `.cfc` is not recognised by the CFML
document grammar — not in any position (`final abstract component`) and not in
any case (`FINAL component`). Rather than producing an `ERROR` node it degrades
to `html_text` + `text`, so nothing downstream can tell that parsing failed:
the formatter emits the body verbatim, the change is whitespace-only, the guard
passes it, and the corpus scores the file **clean**. `component` and
`abstract component` are accepted.

This is worth separating from the ERROR-node cases: a refusal is visible and
safe, while a silent degradation to text is neither.

### 6.3 What the remaining files are

Not all 61 script-refused files are grammar gaps, and this matters before any
of them is filed:

- **Deliberately invalid fixtures.** Lucee's suite includes negative tests that
  are *meant* not to parse — `test/general/Struct/invalid1.cfm` through
  `invalid3.cfm` (`var x = {susi.sorglos, peter};`),
  `LDEV3060/invalidcomponent.cfc`, `LDEV4062` (`testLambda = () => ;`).
- **Fixture junk.** `LDEV4157.cfm` contains literal ``` ``` ``` markdown fences
  inside the CFML.
- **Not CFML at all.** Lucee ships three files whose extension claims CFML and
  whose bytes are a GIF (`arrow-down.gif.cfm`). The corpus harness skips binary
  content now and counts it in its own column, so it cannot be read as a
  grammar gap; that alone moved parse-refused from 25 to 22.
- **Comma-less function parameters.** `coldbox-platform/system/web/Controller.cfc`
  and `MockController.cfc` omit a comma between two arguments in a `relocate()`
  signature. This was recorded here as malformed source; it is not — the form
  parses in CFML and the gap is already filed as tree-sitter-cfml #49.
  `function f(string a, string b boolean c)` fails while the comma-separated
  version parses.

The reduction is automated now: `make shrink REPORT=<corpus report>`
(`internal/formatter/shrink_test.go`) takes a report written by `make corpus`
and reduces every parse-refused and script-refused entry to the smallest
contiguous fragment that still fails the same way. It reduces all 83 refusals;
17 come out under 150 characters and 30 under 400, which is where the entries
above came from. The rest stay large because the reduction is deliberately
conservative — see 6.4 for how those were finished.

### 6.4 The rest of the refusals, characterised

Working through the fragments the reducer left large. As in 6.1, every
construct below was lifted from a failing file and then **re-parsed standalone**
against `language.CFScript`, with a control that does parse — the ERROR node
marks where the parser gave up, not what defeated it, and roughly a third of
the candidates turned out to parse fine on their own.

| Construct | Minimal repro | Control that parses | Filed |
|---|---|---|---|
| Subscripted static access | `x = Test::["m"]()` (and `::[m]()`) | `x = Test::m()` | #79 |
| `${ }` ordered-struct literal | `animals = ${ a: "x" }` | `animals = $[ a: "x" ]` | #80 |
| `exit` with a string argument | `exit "exitTemplate";` | `exit;` / `exit method="t";` | #81 |
| `savecontent` as an expression | `g = savecontent { … };` | `savecontent variable="g" { … }` | #82 |
| `new` as a tag-in-script attribute value | `query name="q" listener=new Foo() { … }` | `listener=makeIt()` / `listener=someVar` | #83 |
| Colon-separated tag-call attributes | `cfparam (name:"d" default:"D");` | all-comma or all-space list | #84 |
| Commas and spaces mixed in one attribute list | `cfimap( a="1", b="2" c="3" )` | either separator used uniformly | #84 |
| Brace-less `try` | `try x = y; catch (any e) { }` | `try { x = y; } catch (any e) { }` | #85 |
| Numeric struct key by dot, **assigned** | `myNumb.4 = "4";` | `x = myNumb.4;` and `myNumb[4] = "4";` | #86 |
| `call():function(…){ }` listener form | `var t = mySuccess():function(r, e) { };` | — | #87 |
| Return type before the access modifiers | `struct public function f() { }` | `public struct function f() { }` | #88 |
| `pageencoding` before a component | `pageencoding "utf-8"; component { }` | — | #89 |
| `name: value;` colon assignment | `msSQL.class: 'org.x.Driver';` | `msSQL.class = 'org.x.Driver';` | #90 |

Constructs that were candidates and **do** parse, recorded so they are not
re-filed: an array literal with keys (`[ cow: [1,2] ]`), a bare `include "x.cfm";`,
a CFML comment inside cfscript, nested tag-in-script bodies
(`cfchart(…) { cfchartseries(…) { … } }`), a dotted named argument
(`g( formstruct.name="test" )`), `savecontent` in statement form, and a
tag-in-script statement with a body and only literal attributes.

Two more went to existing issues rather than new ones: `() => return r` is the
same gap as #75 (a statement as an arrow-function body), and `param url.n 45;`
is a fourth `param` spelling noted on #70.

Two invariants are what make the output trustworthy, and both were learned the
hard way:

- **Contiguity.** Deleting interior lines reduces harder but invents syntax. A
  ColdBox signature reduced that way read `function href( target ="" struct
  data = {} )` — an apparent missing comma between parameters that is not in
  the file, just two unrelated lines pushed together.
- **The same failure, not any failure.** Nearly every fragment of CFML fails to
  parse, so reducing against "still errors" converges on whatever scrap is
  left: a lone `}`, a stray `</cfoutput>`, a line of backticks. The tool
  requires the first ERROR node's text to match the one the whole region
  produced.

Even with both, a fragment is a starting point rather than a verdict — every
construct in 6.1 was re-checked standalone before being written down, and that
check is what caught `static { }` (fails alone, parses inside `component { }`)
being a subtler gap than it first appeared.

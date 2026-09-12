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
| Formatted cleanly | 3,863 | 5,450 | **5,563** |
| Rejected by the guard | 1,671 | 84 | **2** |
| Refused: grammar cannot parse | 86 | 86 | **54** |
| Not idempotent | 390 † | 36 | **2** |
| Malformed output | — † | — † | **0** |
| Panics | 0 | 0 | **0** |

The malformed row has no earlier figure because the check that produces it did
not exist: nothing here asked whether output the guard accepted and the
idempotency check settled on was *well formed*. See section 4.3.

† measured at the post-fix corpus size; the pre-fix figure of 50 covered a much
smaller pool, since a file the guard rejects never reaches the idempotency
check. Comparing like for like, the same 5,450 files went from 390 unstable to
36.

The "current" column is what `make corpus` prints today (section 5), against the
same six projects at their current HEAD. It counts the grammar's 54 refusals in
two buckets rather than one — 22 documents the CFML grammar cannot parse, and 32
that parse as documents but whose embedded cfscript or cfquery the sub-grammar
cannot — because the two are different work and the second is invisible from the
outside: the document parses, the formatter runs, and whatever it renders for
that region is a guess.

The corpus is six upstream repositories at *their* HEAD, not a pinned snapshot,
so the "current" column moves when they do and is not a like-for-like comparison
with the two columns beside it. Re-measured at 5,624 files on
tree-sitter-cfml v0.26.34, the run before the fixes in section 4 stood at 5,527
clean and 29 guard-rejected, with script-refused already down from 61 to 40 on
grammar and upstream changes alone. The v0.26.35 bump then took script-refused to
32 — and moved five of those files into the formatter's own defect columns, since
a construct the grammar starts parsing is one the formatter starts rendering.

Per project, current:

| Project | Files | Clean | Parse-refused | Script-refused | Guard-rejected | Unstable | Skipped |
|---|---|---|---|---|---|---|---|
| Lucee | 3,776 | 3,721 | 20 | 30 | 1 | 1 | 3 |
| ContentBox | 724 | 720 | 2 | 1 | 0 | 1 | 0 |
| ColdBox | 657 | 655 | 0 | 1 | 1 | 0 | 0 |
| FW/1 | 305 | 305 | 0 | 0 | 0 | 0 | 0 |
| TestBox | 146 | 146 | 0 | 0 | 0 | 0 | 0 |
| cfmleditor | 16 | 16 | 0 | 0 | 0 | 0 | 0 |

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

Cases the `whitespaceOnly` guard got wrong. The first two were latent — nothing
in the corpus triggered either — but they meant the "clean" figures were an
upper bound rather than a proof. The next two were not latent: each was
destroying real files while the guard reported success, because a change can be
whitespace-only and still change what the file means. 3.5 and 3.6 are the mirror
image, and the only ones where the guard was too strict rather than too lax —
each refused correct output. 3.7 is the second instance of 3.3's class — the
guard working as specified against a premise that does not hold — and is the
only entry not fully closed: its main defect is fixed and a narrow residue is
described there.

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

### 3.5 A string literal's `/*` opened a comment — 7 files

The four cases above are the guard failing to notice a change, or noticing one
it should have allowed. This one is the guard refusing a *correct* format, and
it is worth separating because the symptom points nowhere near the cause.

`skipWSAndComments` decides where a comment starts by looking at the bytes, and
a string literal is allowed to hold the bytes that open one. CFML code is full
of globs that do — ColdBox's own build script has
`path = "/#libBuildDir#/**"`, and `"#target#/*.zip"` a few lines later. The `/*`
inside the quotes was taken as a block-comment open, and everything to the next
`*/` — some forty lines of code below — was collected as comment body.

The swallowed code is still compared, which is why this is a false rejection
rather than a blind spot. But it is compared as *comment text*, and that
comparison is the stricter of the two: `compareCommentBodies` folds whitespace
and case exactly as the main loop does, and has none of the main loop's
allowances for the canonicalisation the formatter performs on purpose. So a
semicolon deliberately added to a `.run()` forty lines further down landed
inside a "comment body", the two sinks diverged, and a correct format was
refused — reported as changed comment text, naming neither the string that
caused it nor the statement that tripped it. Four of the seven files reported
exactly that, with "comment bodies" that were plainly code.

`stringSpansOf` now locates the string literals in each script region up front,
and no comment may open inside one. Both forms of embedded quote are stepped
over, since ending a literal early would leave its remainder looking like code
and reopen the same hole. Quotes are tracked only in script regions: in markup
the same bytes are attribute delimiters and ordinary prose. Covered by
`internal/formatter/guard_string_test.go`, in both directions — the deliberate
insertion is accepted, and a statement deleted in the region that used to be
swallowed is still caught.

### 3.6 Two ways an ordinary `.cfc` had no script region at all — 5 files

Everything the guard does with a `//` comment depends on knowing which parts of
the file are script: `//` opens a comment there and is ordinary content in
markup. A script-syntax `.cfc` has no `<cfscript>` tag to key off, so
`scriptRegionsOf` asks `isScriptSyntaxComponent`, and that one answer decides
comment handling for the whole file. Two ways of writing a perfectly ordinary
component defeated it, and both produced the same symptom as 3.5 — a correct
format refused, reported somewhere unhelpful.

**A leading UTF-8 BOM.** It is not whitespace, so the probe stopped on it and
the keyword check then failed on a file that is plainly `component { … }`. With
no script region, comment text was compared as though it were code: a
leading-comma struct holding a commented-out entry —
`//, bundleVersion: '3.2.2.54'` — came back as a non-whitespace change reported
against the entry *before* it. 554 files in the corpus carry a BOM, and 2 were
rejected this way.

**Classic Mac line endings.** A bare `\r` with no `\n` anywhere. The
line-comment scan looked for `\n` alone, so the first `//` ran to end of file
and collected every remaining line as its body. This one was latent until the
BOM fix above: TestBox's fixture has both, and giving it a script region is what
gave the scan somewhere to run away in. CRLF was never affected — stopping at
the `\r` leaves the `\n` as the whitespace it is.

Fixing the two together exposed a third defect they had been hiding, which is
the point of recording them as one entry: with comments finally recognised, the
guard could see that a function declaration's annotations were being folded onto
one line, so a `//` comment among them swallowed every annotation after it and
the brace opening the body (section 4). Three files were being formatted into
code that no longer parsed, and the guard had had no way to say so.

### 3.7 Template text that is JavaScript — mostly fixed

3.3 is the case where the guard's premise does not hold: whitespace is not free
in a `<pre>`, so a whitespace-only change destroyed the content and the guard
passed it, correctly, by its own definition. This is a second instance of the
same class, and until this change it was the only entry in this document where a
file was **silently destroyed** rather than refused.

A `.cfm` may be JavaScript. Lucee ships one:

```cfml
<cfcontent type="text/javascript"><cfsetting showdebugoutput="no">/*!
 * jQuery blockUI plugin
 …
```

To the CFML grammar that body is template text, so it went through
`collapseWhitespace` and `writeWrapped` and was reflowed as prose. JavaScript's
`//` comment means nothing to CFML, so nothing stopped a following line being
folded up onto one:

```js
    msg = msg === undefined ? opts.message : msg;      // source

    // remove the current block (if there is one)
    if (full && pageBlock)
        remove(window, {fadeOut:0});
```

```js
    msg = msg ===                                      // before the fix
    undefined ? opts.message : msg; // remove the current block (if there is one) if (full
    && pageBlock) remove(window,
```

The `if` is inside the comment, and the file is no longer the program it was.
The guard passes — only whitespace changed — so the formatter writes it.

#### The fix

`writeText` (`element_formatter.go`) replaces the
`writeWrapped(collapseWhitespace(…))` pair at all four text call sites. A run
holding a `//` comment keeps the line structure it was written with; everything
else is collapsed and reflowed as before. It is the carve-out 3.3 took for
`<pre>`, for the same reason — the guard's premise is that whitespace is free,
and in text whose line breaks terminate comments it is not.

Reflowing is given up for such a run rather than taught to break safely. Doing
the latter needs two invariants, not one: `collapseWhitespace` has to keep the
newline that ends a comment, *and* `safeBreaks` has to stop offering break
positions after a `//` on its line — otherwise wrapping splits the comment and
the tail becomes code again. A run that is JavaScript wants its own line
structure kept regardless, so the simpler rule is also the better one here.

`isLineCommentStart` already declines to read the `//` of a URL scheme as a
comment, so a link in prose does not pin a line.

Measured on `jquery.blockUI.js.cfm`: source comments still ending their own line
in the output go from **4 of 81 to 78 of 81**.

#### What is still wrong — the remaining 3 of 81

The three that stay broken are a different mechanism, and the carve-out cannot
reach them. Each is a comment whose *text* the CFML grammar tokenises, so it
arrives as several CST nodes rather than one, and each node's run is emitted on
its own line — putting the tail of the comment on a new line, as code:

| Source | Output |
|---|---|
| `centerX: true, // <-- only effects element blocking (…)` | `centerX: true, //` ⏎ `<-- only effects element blocking (…)` |
| `// $.blockUI.defaults.css = {};` | `// $.blockUI.defaults.css =` ⏎ `{};` |
| `// … browse_thread/thread/36640a8730503595/2f6a79a77a78e493#2f6a79a77a78e493` | `// …/2f6a79a77a78e493` ⏎ `#2f6a79a77a78e493` |

`<--` is read as markup, and `#…#` as an interpolation. Both are reasonable
readings of a `.cfm`; they are only wrong because the surrounding text is a
JavaScript comment, which CFML has no concept of.

Fixing these means keeping a run together across node boundaries whenever a `//`
is open — and that is where it stops being safe. `//` is **not** a comment in
HTML, so a rule that absorbs following nodes into a verbatim run would stop
formatting real elements whenever a `//` appears in adjacent prose:
`<p>a // b <b>bold</b></p>` would emit the `<b>` verbatim. That is a genuine
over-reach for the common case in order to serve the rare one, so it is left
undone deliberately rather than overlooked.

The file therefore stays on the not-idempotent list. What changed is the
severity: it is no longer a file the formatter destroys, but one it leaves three
comments wrong in.


## 4. Outstanding

Counts from the current `make corpus` run (section 5).

| Issue | Files | Notes |
|---|---|---|
| Grammar cannot parse the document | 22 | Refused safely rather than corrupted. Needs grammar work in `tree-sitter-cfml`, not the formatter. |
| Grammar cannot parse embedded cfscript/cfquery | 32 | The document parses, so the formatter runs and renders those regions blind. Also grammar work, but the failure mode is worse: some of these files are also guard-rejected, and the rest are formatted from a tree with an `ERROR` node in it. |
| Guard-rejected, long tail | 2 | Both characterised in 4.1. One is a grammar gap that produces no ERROR node rather than a formatter defect; the other is the last unreduced comment-text case. |
| Not idempotent | 2 | Both are files whose formatted output the grammar can no longer read, and in both the guard confirmed the output is whitespace-only. `filelisting.cfm` is unharmed, so only a re-format is refused (4.2). `jquery.blockUI.js.cfm` is JavaScript in a `.cfm`; the formatter no longer reflows it as prose (3.7), which took its comments from 4 of 81 intact to 78 of 81, but three whose text the grammar tokenises still have their tails split onto the next line as code. The two whose second pass was refused by the cfscript sub-parser are fixed — both were the comment defects in 3.6. |
| `final component` body not formatted | — | Not a formatter bug: the *document* grammar does not accept `final` on a component at the top of a `.cfc`, in any position or case, and degrades to `html_text` + `text` rather than an `ERROR` node. The formatter therefore emits the body verbatim, the change is whitespace-only, the guard passes it, and the corpus counts the file **clean**. `component` and `abstract component` parse normally. See 6.2. |

Fixed since the audit table above, all found by re-running the harness:

- `import a.b.C;` between the doc block and the component defeated
  `isScriptSyntaxComponent`, which stepped over `abstract` and `final` but not
  over an import. The file then had no script region at all, and with no script
  region the guard stops recognising `//` as a comment anywhere in it and
  compares comment text as though it were code — the same failure the UTF-8 BOM
  caused, with a different token in the way. Five corpus files have an import
  before their component, and on all five the guard ran weakened in every mode.

  It surfaced as a rejection in exactly one place: under
  `commaPosition: "before"` a comma legitimately moves across a `//` comment,
  and with the comment being read as code that looks like a reordering
  (`coldbox-platform/.../ColdBoxScheduledTask.cfc`, line 193). Under the default
  `commaPosition: "after"` nothing moves, which is why the corpus showed two
  guard rejections in that mode and three in this one. Both now show two.

  The probe steps over any number of imports, ending each at a semicolon or a
  line break — Lucee accepts `import a.b.C` without the semicolon, and stopping
  at the line end also keeps a file that ends mid-import from swallowing the
  rest of the source looking for one.

- Two mistakes in how the guard's string scanner reads a CFML literal, both of
  which ran a literal past its own closing quote so that the span covered code —
  where no comment could then be recognised. Each cost one file, and each is a
  general defect rather than a quirk of the file that surfaced it:
  - **A backslash is an ordinary character in CFML.** A quote is escaped by
    doubling it and in no other way, so `"\"` is a string holding one
    backslash — ContentBox has `replace( inPath, "\", "/", "all" )`, and every
    Windows path written `"C:\dir\"` ends the same way. The scanner treated
    `\"` as an escape, the way most C-family languages would.
  - **An interpolation may hold strings of its own.** Lucee's admin has
    `"timezone:'#replace(ds.timezone,"'","''","all")#' // …"` — a double-quoted
    string whose `#…#` contains three more of them. The scanner ended the outer
    literal at the first, leaving the rest of the line outside any string, where
    its `//` was read as a comment. `##` is a literal hash and is stepped over
    rather than read as an empty interpolation.

- Three comment and separator defects in the tail, each a single file:
  - `<cfset x = /* why */ f()>` lost the comment, while the identical one
    written *after* the value survived — that one is a child of the tag rather
    than of the assignment. `delimitedComments` existed for exactly this but
    matched on node kind, and the document grammar gives a `/* … */` in this
    position the plain `comment` kind, the same one it gives `//`. The text is
    what tells the two apart; only the line form cannot be re-emitted inline.
  - `cfparam (name:"local.d" default:"DDD")` — a CF tag in script separating
    attribute from value with a colon, which Lucee accepts — came back with
    every colon rewritten to `=`. The grammar gives both spellings the same
    node with the operator as an anonymous child, and the helper asked for `=`
    returned it whether or not the node had one.
  - `var colTypes = [ "a", "b" ]// note`, a declaration with no semicolon of its
    own, gained a comma after the array and put its semicolon *after* the
    comment, where the comment swallowed it: the comment is a named child of the
    variable_declaration and every named child went into the declarator list.
    Comments are carried separately now, each on a line of its own — trailing
    the semicolon is not a fixed point, because once the semicolon is emitted a
    second pass parses the comment as a statement-level comment and moves it
    down anyway.

- A function declaration's annotations were joined with a space, so a `//`
  comment among them — how ColdBox's own test handlers say what each cache
  setting is for — swallowed every annotation after it *and* the brace opening
  the body, leaving code that no longer parses. A comment now ends its line and
  the annotations that follow continue on the next, with the brace given a line
  of its own when a comment is last. Found only after 3.6, which is what let the
  guard see these files at all. 3 files.
- The check behind those comment fixes now covers position as well as presence.
  A comment can survive a rendering and still be broken by it: the operand that
  followed it in the source gets folded up onto its line, where the comment
  swallows it. Only whitespace changed, so a character-level comparison cannot
  see it — TestBox's `MockBox.cfc`, where a nested parenthesised operand folded
  and a flat one did not. `keptLineComments` now requires each comment to be
  both present and still the last thing on its line.

- Five constructs that v0.26.35 brought into view, each of which deleted
  something the source had. The grammar refused all five before the bump, so
  every one is a defect the release created rather than revealed a fix for:
  - `Test::["f"]()`, the subscripted form of static access (#79), came back as
    `Test["f"]()` — the `::` dropped, turning a static call into an instance
    call. The grammar reports it as a named `static_chain` field on the
    `subscript_expression`, exactly as it does on a `member_expression`, and
    only the latter was special-cased.
  - `throw message="Access Denied" type="MyCustomError";` — the tag form in
    script — lost every attribute but the first, deleting the `type` a catch
    block dispatches on. Each attribute is its own `parameter_attribute` child
    and every rendering path read `NamedChild(0)` alone.
  - `component( output=false, javasettings={…} )` lost the commas between its
    attributes. The parenthesised form separates them with commas while the
    bare form separates them with spaces; the commas are anonymous children, so
    joining everything with a space dropped them. Which attribute carried one is
    recorded now rather than inferred from the form, so neither is imposed on
    the other.
  - `describe("x", function() labels="query" { … })` lost the annotation — the
    way TestBox and Lucee's suite label a spec. It is a child with no field
    name, and the function-*expression* renderer built its output from name,
    parameters and body alone, so the same annotation survived on a declaration
    and vanished inside an argument list.
  - A `//` comment parked before a ternary's `:` was dropped outright. Same root
    cause as the `&&` condition above, and the same fix.

- Two function parameters with no comma between them —
  `f(struct s = structNew()\n  boolean ssl)`, valid CFML that tree-sitter-cfml
  has parsed since v0.26.35 (#49, recorded as malformed source in 6.3 until
  then) — were merged into a single parameter and rejoined with a space. The
  grammar accepts a newline between such a pair but not a space, so the
  formatter's own output no longer parsed and the file came back **unstable**
  rather than guard-rejected: the guard sees only whitespace change, because
  that is all it is. The parameter walk now ends a parameter where the next
  one's first token begins, comma or not, and records whether a comma followed
  so the renderers reproduce the source's separator instead of assuming one.
  The single-line renderer cannot express the newline the pair needs and
  reproduces such a list verbatim. 4 files, all ColdBox and TestBox.
- A `//` comment on each operand of a condition joined with `&&` — every one
  but the last dropped, and that survivor left in front of the closing paren
  where it comments out the rest of the line. A comment between two operands is
  neither the `left` nor the `right` field of the binary_expression holding
  them, so rebuilding the condition from those fields loses it; the same
  condition written with `or` keeps its comments, which is why this survived the
  audit. Rather than enumerate the safe shapes, the rendered condition is now
  checked against the source's own comments and reproduced as written when any
  went missing, and a short condition carrying a line comment is never collapsed
  onto one line. 2 files, exposed by the same v0.26.35 bump.

- A trailing comma — `[1, 2, ]`, `{ a: 1, }`, `f(1, 2, )`,
  `function init(required wirebox, )` — was silently deleted. Legal in Lucee,
  Adobe CF and BoxLang, and common in hand-maintained lists because adding an
  entry then touches one line rather than two. Five renderers (`exprArray`,
  `exprObject`, `exprArgs`, `exprParams`/`flatParams`, `exprFuncDefParams`) each
  collect the elements and rejoin them with `", "`, reconstructing the
  separators from scratch, so the source's final comma had nowhere to come back
  from. The guard caught it, so nothing was corrupted — the effect was that
  format-on-save silently did nothing to any file containing one. The two
  parameter renderers held byte-identical copies of the same walk and now share
  it (`flatParamParts`). 4 files: two `Application.cfc` cache configurations,
  Lucee's own `<cfdump>` tag library, and ColdBox's test harness.
  - The comma has to go after the last *parameter*, not at the end of the
    rendered list: a comment can sit anywhere a parameter can, including after
    the list's final comma, and a separator written past it lands inside the
    comment. The first version of the fix did exactly that to Lucee's
    `LDEV0285/App4.cfc` — `//<cfargument stuff>` came back as
    `//<cfargument stuff>,` — turning a fixed file into a broken one, which is
    why the corpus is re-run against the per-file report rather than the totals.
- `</cfcomponent>` with no opening tag before it crashed the formatter:
  `strings: negative Repeat count`. The open and close tags are siblings rather
  than parent and child, so one increments the indentation level and the other
  decrements it, and the grammar accepts an unmatched close without an `ERROR`
  node — leaving the level at −1 and `strings.Repeat` with a negative count.
  `Format` recovers its own panics, so this surfaced as a refusal rather than a
  crashed daemon, but the file could never be formatted. `indent` now treats a
  negative level as column zero, which makes it total for all thirty-odd sites
  that move the level, and the close tag no longer decrements past zero.
  Reachable in an editor by deleting a component's opening line, and hit by
  Lucee's `Jira2828.cfc`.

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

### 4.1 The remaining guard rejections, characterised

Reduced the same way section 6 reduces the refusals — the smallest contiguous
line range that still fails *with the same verdict*, which matters because
cutting a component in half turns a guard rejection into a parse refusal and
reads as a much smaller repro than it is.

**One is a grammar gap that produces no ERROR node**, the class 6.2 describes.
`<cfcomponent output="false" javasettings={ maven: [...] }>` — an unquoted
struct as a tag attribute — is not parsed as one value. The grammar shreds it
into a run of bogus attributes and ends the tag with a
`cf_selfclose_void_tag_end` it never had:

```
(cf_component_open_tag
  (cf_tag_attributes (cf_attribute (cf_attribute_name) (quoted_cf_attribute_value …)))
  (cf_tag_attributes (cf_attribute (cf_attribute_name) (cf_attribute_value …)))   ; javasettings={
  (cf_tag_attributes (cf_attribute (cf_attribute_name)))                          ; maven:
  …
  (cf_selfclose_void_tag_end))
```

The formatter renders that faithfully and the result is garbage —
`javasettings="{"` followed by `maven:` and `[` as separate attributes, with the
array's contents dropped. There is nothing to fix downstream: any reconstruction
is a reconstruction of a wrong parse. `tree-sitter-cfml` work, and worth filing
with the tree above, since the bogus self-close marker is the part that makes
the failure invisible from the outside.
`Lucee/test/tickets/LDEV5763/LDEV5763_tag_unquoted_struct.cfc`.

**The remaining two** are single-file causes:

| Cause | File |
|---|---|
| Grammar gap producing no ERROR node, described above | `LDEV5763_tag_unquoted_struct.cfc` |
| Comment text, not yet reduced — the last of the bucket 3.5 emptied | `Router.cfc` |

### 4.2 The two files whose output the grammar cannot re-read

Counted as **not idempotent**, and worth separating from the rejections: in both
the guard passed, so the output differs from the source in whitespace only and
the file itself is unharmed. What fails is the *second* parse — tree-sitter
cannot read back a file it could read before, so a re-format is refused.

`jquery.blockUI.js.cfm` is JavaScript in a `.cfm` and has been on this list
since the audit. The second parse failing was always a symptom rather than the
problem: the cause was the formatter reflowing JavaScript as prose and folding
code into `//` comments, which the guard cannot see. That is fixed in 3.7, bar a
residue of three comments described there, and the file stays counted here
because the residue still leaves the output unparseable.

`filelisting.cfm` reduces to two lines, and the cause is a literal `<-` used as
a back-arrow glyph in body text:

```cfml
<a href="x"> <- back</a>    <!--- parses --->

<a href="x">
    <- back                 <!--- does not: MISSING ">" --->
</a>
```

The `<` is read as opening a tag wherever it starts a line. ContentBox writes
the glyph on the same line as the `<a>` tag, so the source parses; the formatter
puts body text on its own line, and the result does not. Grammar work — the
formatter's output is correct CFML, and re-indenting body text is not something
it can reasonably avoid.

### 4.3 Malformed output: the class the harness could not see

Every check above this one asks whether the formatter *destroyed* something or
failed to *settle*. None asks whether what it settled on is well formed. A
defect that is whitespace-only and idempotent therefore passes the guard, passes
the second pass, and is counted **clean** — which is how a braced `case` body
came out as

```
		switch ( k ) {
		case 1:
 {                    <- brace in column one
				a();
			}}            <- the switch's closing brace folded onto the block's
```

across every run recorded in this file, invisible. `malformedShape`
(`internal/formatter/corpus_test.go`) closes that gap with two rules:

- a line that is nothing but closing braces has more than one on it;
- a lone `{` sits in column one.

Both ignore string literals, where a brace is text the formatter cannot
re-indent without changing what it says.

Both rules were chosen by measuring candidates against the corpus and keeping
only those that accused no healthy file, on the grounds that a shape check that
cries wolf is one nobody reads. Four others were tried and dropped; they are
listed in the comment on `malformedShape` with their false-positive counts so
they are not tried again. What is left is narrow on purpose — it does not claim
to find every malformed output, only to stop this class being counted clean.

It reported one file on its first run, `Lucee/test/tickets/LDEV1576/test.cfm`,
and it was a real defect rather than a rule misfiring:

```
	local.qInsert = queryExecute(
" insert into LDEV1576
...
(:requestID,:passThumbnail)",
{                                    <- column one, and so is everything below
requestID: {value: 8, CFSQLType: 'CF_SQL_INTEGER'},
},
);
```

The cause was not lost indentation but indentation never applied. The grammar
gives `queryExecute` a `query_expression` node of its own — the SQL arriving as
a `query_text` child between two quote tokens — rather than the
`call_expression`/`arguments` shape every other call has, and with no case for
it the expression fell to `expr`'s default arm, `return f.text(n)`. The whole
call was emitted verbatim, so it kept whatever indentation the source had, which
here was none. 137 corpus files contain a `queryExecute`; in script-syntax CFML
it is the replacement for `<cfquery>`, and none of them was being formatted.

Fixed by `exprQuery`. The SQL is still re-emitted exactly as written, quotes
included: it is a string literal, so its interior — including the line breaks of
a multi-line query — is content rather than layout, and re-indenting it would
change what the query says. That is the one thing `queryFormat` cannot reach.
The corpus now stands at 0 malformed.

Causes that were in this list and are now fixed are recorded above rather than
here: the outright comment *deletions* (a `//` on the operands of an `&&`
condition, and one parked before a ternary's `:`), the folded signature
annotations, the three comment and separator defects, and the two string-scanner
mistakes in section 4; the string-literal misread that accounted for seven
files in 3.5; and the two missing-script-region defects in 3.6.

## 5. Reproducing

The corpus scanner is checked in as `TestFormatterCorpus`
(`internal/formatter/corpus_test.go`). It is skipped unless `CFML_CORPUS` names
the corpus, so `make test` and CI are unaffected:

```console
$ make corpus CORPUS=/src/Lucee:/src/ContentBox REPORT=/tmp/corpus.tsv
    formatting 4499 files from 2 root(s)
    root                files  clean  parse script  guard unstab  shape  panic   skip
    Lucee                3775   3677     20     54     20      1      0      0      3
    ContentBox            724    719      2      1      2      0      0      0      0
    TOTAL                4499   4396     22     55     22      1      0      0      3
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
`cmd/cfmleditor-lsp/format_test.go`; for the fixes in section 4, in
`internal/formatter/doctype_test.go`,
`internal/formatter/script_tag_call_test.go`,
`internal/formatter/trailing_comma_test.go` and
`internal/formatter/grammar_v26_35_test.go`; for 3.5, in
`internal/formatter/guard_string_test.go`; and for 3.6, in
`internal/formatter/guard_bom_test.go`.

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
  parses in CFML, and the gap was filed as tree-sitter-cfml #49. **Fixed in
  v0.26.35**: `function f(string a, string b boolean c)` now parses, which moved
  these four files out of script-refused and straight into the formatter's own
  defect column — see the comma-less parameter entry in section 4.

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

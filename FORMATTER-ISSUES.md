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

| | Before | After |
|---|---|---|
| Formatted cleanly | 3,863 | **5,450** |
| Rejected by the guard | 1,671 | **84** |
| Refused: grammar cannot parse | 86 | 86 |
| Not idempotent | 390 † | **36** |
| Panics | 0 | 0 |

† measured at the post-fix corpus size; the pre-fix figure of 50 covered a much
smaller pool, since a file the guard rejects never reaches the idempotency
check. Comparing like for like, the same 5,450 files went from 390 unstable to
36.

Per project, after:

| Project | Clean | Parse-refused | Guard-rejected |
|---|---|---|---|
| Lucee | 3,626 | 77 | 72 |
| ContentBox | 720 | 3 | 1 |
| ColdBox | 642 | 5 | 8 |
| FW/1 | 303 | 0 | 2 |
| TestBox | 143 | 1 | 1 |
| cfmleditor | 16 | 0 | 0 |

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

Both are latent — nothing in the corpus triggers them — but they mean the
"clean" figures are an upper bound, not a proof.

### 3.1 CFML comments are skipped entirely

`skipWSAndComments` advances past `<!--- … --->` on both sides before
comparing, so comment *content* is never in the compared stream:

| Source | Output | Verdict |
|---|---|---|
| `<cfset a = 1><!--- keep this --->` | `<cfset a = 1><!--- TOTALLY DIFFERENT --->` | passes |
| `<cfset a = 1><!--- keep --->` | `<cfset a = 1>` | passes |
| `<cfset a = 1>` | `<cfset a = 1><!--- injected --->` | passes |
| `<cfset a = 1>// line comment` | `<cfset a = 1>// CHANGED` | **rejected** |

Rewriting, deleting or injecting a whole CFML comment is invisible. Script
`//` comments are compared normally.

### 3.2 `selfCloseTags` disables quote checking across the whole file

When `allowSelfClose` is true the guard skips *any* mismatched `"` or `'` on
either side. The comment says "around attribute values", but the check is
unanchored and applies to string literals and SQL too:

| Source | Output | `selfCloseTags: true` | `false` |
|---|---|---|---|
| `<cfset msg = "hello world">` | `<cfset msg = hello world>` | passes | rejected |
| `<cfquery>SELECT 'a' FROM t</cfquery>` | `<cfquery>SELECT a FROM t</cfquery>` | passes | rejected |

`selfCloseTags` defaults to `true`, so by default the guard cannot detect the
formatter stripping every quote out of a `<cfset>`.

## 4. Outstanding

| Issue | Files | Notes |
|---|---|---|
| Grammar cannot parse | 86 | 61 cfscript, 25 document. Now refused safely rather than corrupted. Needs grammar work in `tree-sitter-cfml`, not the formatter. |
| Guard-rejected, long tail | 84 | 72 in Lucee's test suite. Includes a residual leading-comma case in some literals, and assorted one- and two-file buckets. |
| Not idempotent | 36 | Blank-line and self-close edge cases in tag-based CFCs. |
| Guard gaps 3.1 / 3.2 | — | Comment content uncompared; quote checking disabled by a style option. |
| `final component` body not formatted | — | Pre-existing: emitted verbatim as `{ function a() {} }`. Whitespace-only, so it passes the guard. `abstract component` and `interface` go through the normal path. |

## 5. Reproducing

The corpus scanner is not checked in — it is a scratch harness, and pointing it
at a large tree is a manual step. To rebuild it, format each file exactly as
`internal/server/formatting.go` does:

```go
o := formatter.DefaultOptions()
o.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
o.ParseQuery  = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
o.ParseCFML   = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }
o.WhitespaceOnly = true

tree := language.Parse(language.CFML, src, nil)
out, err := formatter.Format(src, tree, o)   // err != nil => refused
```

Then format `out` again and compare, to catch non-idempotency.

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
`cmd/cfmleditor-lsp/format_test.go`.

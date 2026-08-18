# Formatter: non-whitespace change audit

Findings from formatting every `.cfm`/`.cfc` file in `testdata/` with
`internal/formatter` and checking the result against the `whitespaceOnly`
guard (`checkWhitespaceOnly`, `internal/formatter/formatter.go:199`).

The formatter is supposed to be whitespace-only: `formatting.whitespaceOnly`
defaults to `true` (`internal/config/config.go:202`), and `Format` rejects its
own output when the non-whitespace character stream changed
(`internal/formatter/formatter.go:186`). Everything below is a case where the
formatter *does* change non-whitespace content, or where the guard fails to
notice that it did.

## Corpus and method

39 files under `testdata/` (`.cfm` + `.cfc`). Each was parsed with
`language.CFML`, formatted with `formatter.DefaultOptions()` plus the three
sub-parsers and `WhitespaceOnly: true`, and the output compared to the input.
Files that passed the guard were then formatted a second time to check
idempotency.

| Result | Count |
|---|---|
| Formatted cleanly (whitespace-only, and idempotent) | 30 |
| **Rejected by the `whitespaceOnly` guard** | **9** |
| Non-idempotent (differs between pass 1 and pass 2) | 0 |

The 9 rejected files, and which issue below causes each:

| File | Issue |
|---|---|
| `testdata/UserService.cfc` | 1 |
| `testdata/deps/persist.cfc` | 1 |
| `testdata/deps/service.cfc` | 1 |
| `testdata/refs/controller.cfc` | 1 |
| `testdata/refs/persist.cfc` | 1 |
| `testdata/refs/service.cfc` | 1 |
| `testdata/beans/dao/OrderDAO.cfc` | 4 |
| `testdata/filepath_test.cfm` | 3 |
| `testdata/DefinitionTestTag.cfc` | 2 |

Note the asymmetry that governs how much each issue actually hurts:

- The **LSP path** (`internal/server/formatting.go`) is protected twice — it
  refuses to format a document whose tree has an `ERROR` node
  (`formatting.go:91`) *before* calling `Format`, and it passes
  `WhitespaceOnly` through from config (`formatting.go:115`). Symptom there is
  "format-on-save silently does nothing".
- The **`format` CLI subcommand** (`cmd/cfmleditor-lsp/main.go:294`) has
  **neither** check. It calls `formatter.DefaultOptions()`
  (`main.go:312`), which leaves `WhitespaceOnly` at its zero value `false`, and
  never inspects the tree for errors. Symptom there is silent file corruption
  (issue 5).

---

## 1. `query` and `function` return types are silently dropped

**Severity: high — silent semantic change.** Affects 6 of the 9 failing files.

```cfml
component {
    public query function GetData(required string id) {
        return 1;
    }
}
```

formats to:

```cfml
component {

	public function GetData(
		required string id
	) {

		return 1;

	}

}
```

The `query` return type is gone. Surveying every CFML return type, two are
dropped and the rest survive:

| Return type | Result |
|---|---|
| `any` `array` `binary` `boolean` `component` `date` `guid` `numeric` `string` `struct` `uuid` `variablename` `void` `xml`, and dotted component paths (`models.User`) | preserved |
| **`query`** | **dropped** |
| **`function`** | **dropped** |

It is not tied to the access modifier — `query function A()`,
`private query function B()`, `remote query function I()` and
`package query function J()` all lose it. Parameter types are unaffected
(`function a(query q)` is fine); only the return type is lost.

**Root cause.** The CFML grammar tokenises these two type names as *anonymous*
keyword nodes rather than as named `identifier` nodes. CST for
`public query function A(...)`:

```
function_declaration
  access_type      named=true   "public"
  query            named=false  "query"     <-- anonymous
  function         named=false  "function"
  identifier       named=true   "A"
```

versus `public struct function A(...)`, where the type arrives as
`identifier named=true "struct"`.

`scriptFunction` (`internal/formatter/cfscript_formatter.go:1067`) collects
signature prefix tokens behind a named-node gate:

```go
if !c.IsNamed() {
    t := c.Kind()
    if t == "function" {
        // keyword
        continue
    }
}

if c.IsNamed() {
    ...
    prefix = append(prefix, f.text(c))     // cfscript_formatter.go:1103
}
```

An anonymous child that is not the `function` keyword falls through both
blocks and is dropped with no diagnostic. `query` is exactly that case.

`function` as a return type is lost to the sibling problem: the two
`function` tokens (return type and keyword) are both anonymous and both
skipped by the `t == "function"` branch at `cfscript_formatter.go:1088`, and
the signature builder then emits a single literal `"function "` at
`cfscript_formatter.go:1118`.

## 2. `<cfcomponent>` containing a body-less `<cfinvoke>`/`<cfhttp>` produces corrupt output

**Severity: critical — destroys the file via the CLI.** Affects
`testdata/DefinitionTestTag.cfc`.

```cfml
<cfcomponent>
	<cfinvoke component="models.Widget" method="render" returnvariable="r">
</cfcomponent>
```

formats to:

```
<cfcomponent>

<cfinvokecomponent="models.Widget"method="render"returnvariable="r"></cf>
```

Three separate corruptions in one output: the tag name and every attribute are
concatenated with no separating whitespace (`<cfinvokecomponent="…"method="…"`,
which is not parseable CFML), the `</cfcomponent>` closing tag is discarded,
and a bogus `</cf>` is emitted.

The same input with `<cfhttp>` instead of `<cfinvoke>` corrupts identically.
Closing the tag (`<cfinvoke …></cfinvoke>`) formats correctly, as does the same
body-less `<cfinvoke>` inside `<cfif>` or at the top level of a `.cfm`.

**Root cause.** The grammar cannot parse a body-less `<cfinvoke>` inside
`<cfcomponent>` and emits an `ERROR` node wrapping it:

```
program
  cf_component_open_tag  "<cfcomponent>"
  ERROR                                     <-- parse failure
    cf_start_tag  "<cfinvoke component=... >"
    </cf
    >
```

`Format` has no handling for `ERROR` nodes in the top-level CFML tree — the
`parseErr` machinery (`formatter.go:349`) only covers *sub*-parses of script
and query blocks — so the walk falls through to a raw-emit path that
concatenates the ERROR node's children without separators.

Note `Format` returns `err == nil` here. Only the `whitespaceOnly` guard
catches it, and only because the character streams end up different lengths.

**Downstream symptom.** The same mis-nesting cascades into surrounding HTML.
`<div>\n<cfinvoke component="a" method="b">\n</div>` formats to:

```
<div />
<cfinvoke component="a" method="b">
</div>
</cfinvoke>
```

The `<div>` is rewritten as a self-closing `<div />` while its `</div>` is left
in place, and a `</cfinvoke>` is emitted after the `</div>`.

## 3. Body-less optional-body tags gain a synthesised closing tag

**Severity: medium — re-parents following content.** Affects
`testdata/filepath_test.cfm`.

```cfml
<cfmodule template="includes/header.cfm">
<p>after</p>
```

formats to:

```cfml
<cfmodule template="includes/header.cfm">
	<p>after</p>
</cfmodule>
```

`<p>after</p>` was a sibling of the `<cfmodule>` and is now its body. Surveying
CF tags commonly written without a body:

| Tag | Behaviour |
|---|---|
| `cfinclude` `cfset` `cfparam` `cfabort` `cflocation` `cfheader` `cfcookie` `cfdump` `cflog` `cfimport` `cfobject` `cfthrow` `cfexit` `cfflush` `cfrethrow` | correctly treated as void → self-closed, following content untouched |
| **`cfmodule`** `cfhttp` `cfinvoke` | treated as containers → all following content swallowed, closing tag synthesised |

`cfmodule`, `cfhttp` and `cfinvoke` genuinely *can* take a body
(`<cfhttpparam>`, `<cfinvokeargument>`, a custom-tag body), so they are not
void — but they are legal and common without one, and the formatter has no
handling for that shape.

## 4. Missing statement semicolons are inserted

**Severity: low — benign in CFML, but blocks formatting entirely.** Affects
`testdata/beans/dao/OrderDAO.cfc`.

```cfml
component {
	function a() {
		return []
	}
	function b() {
		var x = 1
		return x
	}
	function c() {
		writeOutput("hi")
	}
}
```

Every statement comes back with a `;` appended. Semicolons are optional in
CFML, so this is semantically harmless and arguably desirable normalisation —
but it inserts non-whitespace characters, so the guard rejects the whole file.
With the default `whitespaceOnly: true`, one missing semicolon anywhere makes
the LSP refuse to format the entire document, with no visible reason.

## 5. The `format` CLI subcommand has no safety net and will corrupt files

**Severity: critical — silent data loss.**

`cmdFormat` (`cmd/cfmleditor-lsp/main.go:294`) uses
`formatter.DefaultOptions()`, which does not set `WhitespaceOnly`
(`internal/formatter/formatter.go:104`), and unlike the server path it never
checks `tree.RootNode().HasError()`. So every issue above is written straight
to disk by `format -w`:

```console
$ wc -c victim.cfc
1521 victim.cfc
$ cfmleditor-lsp format -w victim.cfc
formatted victim.cfc
$ echo $?
0
$ wc -c victim.cfc
1411 victim.cfc
$ tail -2 victim.cfc
<cfinvokecomponent="models.Widget"method="render"returnvariable="invokeResult">
<cfinvokecomponent="services.UserService"method="listUsers"returnvariable="users"></cf>
```

110 bytes gone, the file no longer parses, exit status 0, success message
printed. `internal/server/formatting.go` refuses this same document; the CLI
does not.

## 6. Guard blind spot: CFML comments are skipped entirely

**Severity: medium — the guard under-reports.**

`skipWSAndComments` (`internal/formatter/formatter.go:291`) advances past
`<!--- … --->` blocks on both sides before comparing, so comment *content* is
never part of the compared stream. Every one of these passes the guard:

| Source | Output | Guard verdict |
|---|---|---|
| `<cfset a = 1><!--- keep this --->` | `<cfset a = 1><!--- TOTALLY DIFFERENT --->` | passes |
| `<cfset a = 1><!--- keep --->` | `<cfset a = 1>` | passes |
| `<cfset a = 1>` | `<cfset a = 1><!--- injected --->` | passes |
| `<cfset a = 1>// line comment` | `<cfset a = 1>// CHANGED` | **rejected** |

Rewriting, deleting or injecting an entire CFML comment is invisible. Script
`//` comments are compared normally, so the exposure is specific to `<!--- --->`.

## 7. Guard blind spot: `selfCloseTags` disables quote checking everywhere

**Severity: medium — a style option silently widens the guard.**

When `allowSelfClose` is true, `checkWhitespaceOnly` skips any mismatched `"`
or `'` on either side (`internal/formatter/formatter.go:228-251`). The intent
per the comment is "added/removed quotes around attribute values", but the
check is unanchored — it applies to every quote in the file, including string
literals and SQL:

| Source | Output | `selfCloseTags: true` | `selfCloseTags: false` |
|---|---|---|---|
| `<cfset msg = "hello world">` | `<cfset msg = hello world>` | passes | rejected |
| `<cfset a = "x" & "y">` | `<cfset a = x & y>` | passes | rejected |
| `<cfquery>SELECT 'a' FROM t</cfquery>` | `<cfquery>SELECT a FROM t</cfquery>` | passes | rejected |

`selfCloseTags` defaults to `true`, so by default the guard cannot detect the
formatter stripping every quote out of a `<cfset>`. This is a latent gap — no
file in the corpus triggers it — but it means the "30 clean files" figure above
is an upper bound, not a proof.

---

## Reproducing

The corpus scan is not checked in (it would fail CI while these bugs stand).
To rebuild it, format each file with `WhitespaceOnly: true` and the three
sub-parsers wired up, exactly as `internal/server/formatting.go` does:

```go
o := formatter.DefaultOptions()
o.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
o.ParseQuery  = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
o.ParseCFML   = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }
o.WhitespaceOnly = true

tree := language.Parse(language.CFML, src, nil)
out, err := formatter.Format(src, tree, o)   // err != nil => non-whitespace change
```

Individual cases reproduce through the CLI, which applies no guard:

```console
$ go build -o target/release/cfmleditor-lsp ./cmd/cfmleditor-lsp
$ printf 'component {\n\tpublic query function A() {}\n}\n' > /tmp/r.cfc
$ target/release/cfmleditor-lsp format /tmp/r.cfc
```

## Suggested order of work

1. **Issue 5** — give `cmdFormat` the same two gates the server has
   (`HasError()` refusal, `WhitespaceOnly` on). One-line-ish, and it turns
   every other bug here from data loss into a refusal to format.
2. **Issue 2** — make `Format` refuse a tree containing an `ERROR` node
   instead of raw-emitting it, so corrupt output cannot be produced at all.
3. **Issue 1** — in `scriptFunction`, accept anonymous prefix children that
   are not the `function` keyword, and track the return-type `function` token
   separately from the keyword.
4. **Issue 3** — treat `cfmodule`/`cfhttp`/`cfinvoke` as void when the source
   has no matching close tag.
5. **Issues 6 and 7** — tighten the guard: compare comment content, and anchor
   the quote allowance to attribute values rather than the whole file.
6. **Issue 4** — decide whether semicolon insertion is intended. If it is, it
   needs to be exempted from the guard the way self-closing already is;
   if not, preserve the source's omission.

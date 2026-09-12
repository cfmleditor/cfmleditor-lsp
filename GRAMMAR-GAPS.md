# Grammar gaps: constructs `tree-sitter-cfml` cannot parse

`FORMATTER-ISSUES.md` counts files the grammar refuses — currently 22 documents
and 32 embedded cfscript/cfquery regions. A count is not something a grammar
maintainer can act on. This file turns the ones that are worth acting on into
constructs, and says which are not worth acting on at all.

## Method

Two parsers over the same six-project corpus (5,624 files):

- **`tree-sitter-cfml`**, through `make corpus`.
- **[RustCFML](https://github.com/RustCFML/RustCFML)**, a CFML interpreter in
  Rust. Its parser is a useful oracle precisely because it is part of a working
  engine: it has to accept what a real engine accepts. Driven through a harness
  over its `cfml-compiler` crate that mirrors `cfml-vm`'s own pipeline —
  `has_cfml_tags` → `tags_to_script_checked` → `Parser::parse`.

|  | RustCFML parses | RustCFML refuses |
|---|---|---|
| **tree-sitter parses** | 5320 | 247 |
| **tree-sitter refuses** | **32** | 25 |

The 32 are the interesting cell. Each was reduced with `make shrink` to the
smallest contiguous fragment that still fails, and each fragment was then run
through *both* parsers again — a fragment that loses the context that made
RustCFML accept the file is not a repro. 24 of the 32 survive that.

**RustCFML is the more permissive parser, so "RustCFML accepts it" is evidence,
not proof.** Six of the 24 are Lucee's own *negative* fixtures — files whose
names say they are meant to fail — and there tree-sitter is right and RustCFML
is wrong. They are listed below so nobody files them. The remaining eighteen
first read as six constructs; isolating each by hand cut that to four, filed as
tree-sitter-cfml
[#116](https://github.com/cfmleditor/tree-sitter-cfml/issues/116)–[#119](https://github.com/cfmleditor/tree-sitter-cfml/issues/119).

The 247 in the other direction are RustCFML's gaps on a Lucee-heavy corpus,
not ours.

## Not gaps: negative fixtures

tree-sitter correctly refuses these. RustCFML accepting them is RustCFML being
lax about errors it would raise at runtime instead.

| File | Fragment |
|---|---|
| `Lucee/test/tickets/LDEV5900/bad1.cfm` | `<cfset x = >` |
| `Lucee/test/tickets/LDEV5900/bad2.cfm` | `<cfset unclosed string` |
| `Lucee/test/general/Struct/invalid1.cfm` | `var x={susi.sorglos,peter};` |
| `Lucee/test/general/Struct/invalid2.cfm` | `var x={variables.susi,peter};` |
| `Lucee/test/general/Struct/invalid3.cfm` | `var x={susi(),peter};` |
| `Lucee/test/tickets/LDEV3060/invalidcomponent.cfc` | `domain_id :: oDnsDomain.getId()` — `::` where `:` was meant |

## Verified against the grammar, one construct at a time

A fragment is a *file*, and a file that fails is not yet a construct. Each of the
six candidates was therefore re-cut into the smallest standalone source that
still reproduces, run against `tree-sitter-cfml` **HEAD** (`a4b5b72`, `v0.26.35`
— the version this repo pins, and the newest published), and checked against the
grammar's own `LIMITATIONS.md` and issue tracker before being called a gap.

That step changed five of the six. `component { package final whatever function
f() {} }` parses on its own, and so does every one of `final component`, `final
function f()`, `final public function f()`, a `thread { … }` statement, a tag
island, and a `<cfif>` spanning whole tag attributes. What fails is narrower than
the file made it look, and in two cases what fails is already settled upstream.

### Already known upstream — do not file

| Construct | Status |
|---|---|
| `final` on a parameter — `function f( final required s )` | In `LIMITATIONS.md`. Implemented, measured, rejected: `( final (` is ambiguous at every parameter list in the language. |
| `final component`, `final function f()`, `final public function f()` | All parse. Fixed by [#77](https://github.com/cfmleditor/tree-sitter-cfml/issues/77) / [#69](https://github.com/cfmleditor/tree-sitter-cfml/issues/69). |

So Lucee's `test3671.cfc` is one gap, not three, and it is a closed question.

## Filed: four constructs

Each reproduces standalone on HEAD and RustCFML parses each. All four are now
open against `tree-sitter-cfml`:
[#116](https://github.com/cfmleditor/tree-sitter-cfml/issues/116),
[#117](https://github.com/cfmleditor/tree-sitter-cfml/issues/117),
[#118](https://github.com/cfmleditor/tree-sitter-cfml/issues/118),
[#119](https://github.com/cfmleditor/tree-sitter-cfml/issues/119).

### 1. An arrow function with an empty body ([#116](https://github.com/cfmleditor/tree-sitter-cfml/issues/116))

`Lucee/test/tickets/LDEV4062/LDEV4062.cfm`. Fails in `cfscript/grammar.js` and,
reached through `<cfset f = function() { … }>`, in `common/define-grammar.js`.
`=>` and `->` both fail; the parameter list may be empty or not.

```cfml
x = () => ;
y = 1;
```

Not [#75](https://github.com/cfmleditor/tree-sitter-cfml/issues/75), which is a
*statement* as the body. The empty body is worse than an error: at end of file
`x = () => ;` yields an `arrow_function` whose body is a `number` holding a
MISSING token, and with a following statement the parser takes `y = 1` as the
lambda body.

### 2. A return type between two access modifiers ([#117](https://github.com/cfmleditor/tree-sitter-cfml/issues/117))

`Lucee/test/general/modifiers/All.cfc`

```cfml
component { public struct static function f() {} }
```

`struct public static function f()` parses ([#88](https://github.com/cfmleditor/tree-sitter-cfml/issues/88))
and so does `public static struct function f()`. Only the interleaved spelling
fails — the return type is accepted before the modifier run or after it, not
inside it. Lucee's file writes four such members with a user-defined type name.

### 3. A `thread { … }` statement followed by a tag island ([#118](https://github.com/cfmleditor/tree-sitter-cfml/issues/118))

`Lucee/test/tickets/LDEV4157/LDEV4157.cfm` and `test4157.cfc`

````cfml
thread name="t" {
	thread.test = "thread";
}

```
	<cfset res = "works">
```
````

The statement alone parses and the island alone parses; together the grammar
reports a MISSING `;` at the closing brace. Already recorded in the grammar's
`LIMITATIONS.md`, but never filed, so nothing tracks it.

### 4. A start tag split across `<cfif>` branches ([#119](https://github.com/cfmleditor/tree-sitter-cfml/issues/119))

Two shapes, both from ContentBox, both ordinary CFML. The closing `>` inside the
branches (`views/authors/editor.cfm`):

```cfml
<a
	title="Back"
	<cfif x>
		href="a">
	<cfelse>
		href="b">
	</cfif>
	Back</a>
```

and the whole tag *opening* inside them, closed after the `</cfif>`
(`views/settings/rawSettingsTable.cfm`):

```cfml
<cfif x>
	<a
		disabled="disabled"
<cfelse>
	<a
		class="confirmIt"
</cfif>
	title="Delete"
>
```

A `<cfif>` that spans only whole *attributes* — `<a <cfif x>href="a"<cfelse>href="b"</cfif>>`
— parses. It is putting the `>` on the branch side that fails, in both HTML and
`<cf…>` tags.

## Shrank poorly: twelve files still to reduce

These reproduce, but `make shrink` could not isolate a small fragment — the
failure needs context the contiguous-fragment search cannot keep. They need
reducing by hand or a smarter shrinker.

| Chars | Refusal | File |
|---|---|---|
| 842 | script | `Lucee/test/tickets/LDEV5389.cfc` |
| 899 | script | `Lucee/test/tickets/_LDEV3623.cfc` |
| 1202 | document | `Lucee/test/tickets/LDEV1750.cfc` |
| 1236 | script | `Lucee/test/tickets/LDEV3113.cfc` |
| 2411 | document | `Lucee/test/tickets/LDEV0869.cfc` |
| 2829 | document | `Lucee/core/src/main/cfml/context/wddx.cfm` |
| 6863 | document | `Lucee/core/.../admin/debugging.templates.create.cfm` |
| 10028 | document | `Lucee/core/.../admin/services.gateway.create.cfm` |
| 21520 | document | `Lucee/core/src/main/cfml/context/formtag-form.cfm` |
| 21558 | document | `Lucee/core/src/main/cfml/context/form.cfm` |
| 25684 | script | `Lucee/core/src/main/cfml/context/admin/Jira.cfc` |
| 78998 | script | `ContentBox/modules/contentbox/models/system/CBHelper.cfc` |

## Reproducing

```bash
CORPUS=/src/Lucee:/src/ContentBox:/src/coldbox-platform:/src/fw1:/src/testbox:/src/cfmleditor
make corpus CORPUS=$CORPUS REPORT=/tmp/corpus.tsv
make shrink REPORT=/tmp/corpus.tsv
```

For the RustCFML side, build a harness against its compiler crate — three
dependencies, about 35 seconds — that mirrors `crates/cfml-vm/src/lib.rs`'s
parse pipeline, and run it over the same file list. It parses the whole corpus
in under a second, so the comparison is cheap enough to repeat whenever the
grammar is bumped.

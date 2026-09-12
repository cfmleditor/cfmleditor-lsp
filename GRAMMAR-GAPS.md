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
is wrong. They are listed below so nobody files them.

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

## Filable: six constructs

Each reproduces on its own, and RustCFML parses each.

### 1. A lambda with no body

`Lucee/test/tickets/LDEV4062/LDEV4062.cfm`

```cfml
testLambda=()=>;
```

### 2. `final` on a component, a function, and a parameter

`Lucee/test/tickets/LDEV3671/test3671.cfc`. `FORMATTER-ISSUES.md` § 4 already
records `final component` degrading to `html_text` rather than an `ERROR` node;
this is the same keyword in three positions.

```cfml
final component {
	final public function testFunc(final required s) {
	}
}
```

### 3. Modifier ordering, and contextual keywords as function names

`Lucee/test/general/modifiers/All.cfc`

```cfml
component {
	package final whatever function final() {}
	final whatever package function public() {}
	whatever final package function package() {}
	package final function function private() {}
}
```

### 4. A tag island after a `thread` statement

`Lucee/test/tickets/LDEV4157/LDEV4157.cfm`

````cfml
thread name="LDEV4157" {
	thread.test = "thread";
}

```
	<cfset res = "tag-island after the thread statement works">
```
````

### 5. The same inside a component, with `thread action="join"`

`Lucee/test/tickets/LDEV4157/test4157.cfc`

````cfml
component {
	function foo() {
		thread name="LDEV4157cfc" {
			thread.test = "thread";
		}

		```
			<cfset var res = "tag-island after the thread statement in cfc works">
		```
		thread action="join" name="LDEV4157cfc";

		return cfthread.LDEV4157cfc.test & " and " & res;
	}
}
````

### 6. A tag whose attributes are split across `<cfif>` branches

`ContentBox/.../views/authors/editor.cfm`, and the same shape in
`.../views/settings/rawSettingsTable.cfm`. The opening `<a` and its closing `>`
are in different branches, which is ordinary CFML and common in real templates.

```cfml
<a
	title="Back"
	class="btn btn-sm btn-back mt5"
	<cfif prc.oCurrentAuthor.hasPermission( "AUTHOR_ADMIN" )>
		href="#event.buildLink( prc.xehAuthors )#">
	<cfelse>
		href="#event.buildLink( prc.xehDashboard )#">
	</cfif>
```

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

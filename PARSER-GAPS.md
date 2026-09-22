# Parser gaps: what the grammar sees that `internal/parser` does not

`internal/parser` and the `tree-sitter-cfml` grammar are two independent
implementations of "what is a call" — a hand-written line scanner and tag search
against a real parse. Where they disagree about one, one of them is wrong.

`make gapcheck CORPUS=<dir>[:<dir>...]` diffs them over a corpus.
`internal/tsoracle` holds the comparison; without `CORPUS` it holds the repo's
own fixtures to a recorded list of differences and fails on a new one.

This file records what the six-project corpus says, because a number nobody
writes down gets rediscovered. `GRAMMAR-GAPS.md` is the mirror of it: constructs
the *grammar* cannot parse.

## 1. Corpus and method

The same six repositories `FORMATTER-ISSUES.md` uses, at their HEAD:

| Project | Files |
|---|---|
| Lucee (`lucee/Lucee`) | 3,777 |
| ContentBox (`Ortus-Solutions/ContentBox`) | 724 |
| ColdBox (`ColdBox/coldbox-platform`) | 661 |
| FW/1 (`framework-one/fw1`) | 305 |
| TestBox (`ortus-solutions/testbox`) | 146 |
| cfmleditor (`cfmleditor/cfmleditor`) | 16 |
| **Total** | **5,629** |

Each file is parsed twice — `GrammarCalls` walks the CFML tree and every
injected region, `parserCalls` runs `ParseWithOptions{ExtractCalls: true}` — and
both are reduced to `(line, method)`. The whole run takes **6 seconds**.

**Receivers are deliberately not compared.** The two spell a chained receiver
differently: `a().b()` is a nested `member_expression` to the grammar and a base
plus a `Chain` to the parser. A difference there is a naming difference, not a
missing call.

**The corpus is not neutral.** Lucee is 3,777 of the 5,629 files and most of
that is its test suite, which is why TestBox-style specs and one particular
string-quoting shape loom large below. Read the classes, not the ratios.

## 2. Results

| | Before | After §3.1 | After §3.2 | After §3.3 |
|---|---:|---:|---:|---:|
| Files where the two disagree | 1,506 (26.8%) | 1,340 | 1,297 (23.0%) | **837 (14.9%)** |
| Call sites the grammar sees and the parser misses | 10,451 | 8,707 | 8,500 | **5,357** |
| Sites the parser records and the grammar does not | 1,065 | 1,070 | 1,067 | 1,098 |

Keyed on the method name alone — ignoring which line it landed on — the missed
figure is 9,726 before, 7,982 after §3.1, 7,775 after §3.2 and **4,602** after §3.3,
and the parser-only figure falls to 343. The difference between the two keyings,
around 750 sites, is **line skew**: a multi-line `<cfset>` or `<cfif>` records
its calls at the tag's starting line while the grammar puts each on its own.
Cosmetic, but it is why a report of this keyed on lines cannot be diffed
cleanly against another.

## 3. Fixed

### 3.1 Expressions a `<cfset>` holds

About 3,100 of the missed sites were the largest single class, and all of one
shape: `parseCFSet` matched a few ways an expression can hold a call with string
searches, and everything else fell through.

```cfml
<cfset arrayAppend(ret, prefix & "." & key) />     <!-- a bare call         -->
<cfset var name="test"&createuniqueid()>          <!-- after a concatenation -->
<cfset attributes.req.list=cfc.listApplications()><!-- scope-prefixed LHS    -->
<cfset k = evaluate(fileread(v.indexFile)) />     <!-- nested: evaluate only -->
<cfset x = svc.y()>                               <!-- the shape it handled  -->
```

A `<cfset>` holds an expression, so it now goes to a `scriptParser` and keeps
the calls — the same thing `<cfif>`, `<cfelseif>`, `<cfreturn>` and a `#...#`
span already do, and the same thing a `<cfscript>` body has always done. One
implementation of "what is a call" serves both syntaxes.

Two things this needed:

- **The string paths stay.** They carry the refs and pending calls that decide
  what a variable now holds, and they resolve a receiver's component against
  *this file's* refs, which a fresh sub-parse cannot. So the sub-parse tops the
  line up: the count already recorded for each name is subtracted and only the
  excess is added. Counting rather than testing presence is what keeps
  `<cfset x = f() + f()>` at two calls.
- **Only `<cfset>` tops up.** Deduping a `#...#` span is actively wrong, and the
  corpus is what caught it: `#getColdBoxSetting("a")# #getColdBoxSetting("b")#`
  is two calls on one line, and running the shared merge with the tally on cost
  the second one. A unit test on a single span could not have found that.

It also made `scanChainedInstantiation` redundant —
`<cfset d = createObject("component","x").init("ds")>` now reaches the same
sub-parse, which resolves the instantiation itself — so that string scan and its
two helpers are gone.

#### Cost

Measured interleaved, alternating builds, minimum of nine, one benchmark per
process:

| | before | after |
|---|---:|---:|
| `BenchmarkParse_TagCFC` | 93.4µs / 201 allocs | **90.0µs / 201 allocs** |
| `BenchmarkParse_TagCFC_ExtractCalls` | 105.3µs / 255 allocs | 122.1µs / 335 allocs |

The plain tag parse got *faster*, because the string scan it replaced ran on
every `<cfset>`. With call extraction it is **+15.9%**, which is one sub-parse
per `<cfset>` holding a `(` — ten of them in that fixture. That cost lands on
`unresolved`, the code map and `deps`, not on the editor's keystroke path, which
parses without `ExtractCalls`.

### 3.2 Calls in a script-tag attribute value

```cfml
loop array=structKeyArray(rowData) item="local.col" { … }
http url="u" result=serializeJson(body.data) {}
query name="q" datasource=getDatasource() { }
```

`skipTagAttrValue` consumed the value's identifier as a plain token and handed
only its *argument list* to the scan that finds calls — so it recorded whatever
was inside the parens and never the call itself. It dispatches the identifier
through `scanNestedCall` now, and a quoted value through `handleLiteralToken`,
which also picks up `datasource="#svc.ds()#"`.

207 sites, 43 files. Allocations are identical and the timings are within noise
on all three parse benchmarks, so this one is free.

`TestEveryTokenLoopHandlesLiterals` grew to cover it: the loop peeks at its
terminator rather than consuming it, so its dispatch reads the token first, and
the test's anchor now accepts that shape. There are six such loops.

### 3.3 A same-quoted string opened inside `#...#`

```cfml
assertEquals("7", "#DayOfWeek("{ts '2000-1-1 17:26:03'}")#");
name = "logMessage_#replace( createUUID(), "-", "", "all" )#"
writeOutput( "#cb.menu( slug = arguments.slug, type = "html" )#" );
```

`scanString` took the first matching quote as the end, so the token was
`"#DayOfWeek("` and the interpolation never had a closing `#` to find:

```
"#DayOfWeek("   {   ts   '2000-1-1'   }   ")#"
```

It is a **scanner** gap rather than an interpolation one — `'…'` inside `"…"`
already worked, and so did `"#DayOfWeek(d1)#"`. `scanString` now steps *over* a
`#...#` expression, and over any string opened inside it, through a pair of
mutually recursive helpers (`scanQuoted`/`scanHashExpr`).

Three things it needed:

- **It is opt-in, via `scriptParser.asCFScript()`.** Text that is not CFScript
  reaches the same scanner — `parseFuncBody` hands a tag function's raw body to
  the script parser — and applied to markup the rule is actively *wrong*:

  ```html
  <a href="#top">x</a><a href="#bot">y</a>
  ```

  pairs the two fragment hashes and swallows the markup between them into one
  string token. That is a wrong answer where the old behaviour was merely a
  coarse one. The callers that know they hold CFScript opt in — the six
  region-guarded construction sites, the interpolated-span sub-parser, and
  `newGlobalScriptParser`.
- **`newGlobalScriptParser` opts in for correctness, not for calls.** The two
  variable scans must tokenise a file the same way. Without the flag
  `variables.a = "#f("x=1")#"` tokenises as `"#f("`, `x`, `=`, `1`, `")#"`, and
  a scan that reads a bare `ident =` as a declaration files `x` as a
  variables-scope variable — which is what completion offers and what the index
  stores.
- **The scan is speculative.** When the nesting does not close before EOF the
  attempt is abandoned and the plain quote-to-quote scan runs from the same
  place, so a token can never come out worse than it was. The recursion is
  capped at `maxStringNesting`, for the reason `maxArgNesting` is: the depth
  comes from the source and Go cannot `recover()` from stack exhaustion.

**3,165 sites across 461 files**, which is far more than the 888 the shape
itself accounts for: a string that swallows its terminator takes the rest of
the line — often the rest of the construct — with it, so the recovery reaches
calls that have nothing to do with interpolation. 442 of those files are
Lucee's date tests, but the other 19 are the idiom appearing in ordinary code
(the ColdBox and ContentBox lines above), which is exactly the condition §5
named for revisiting this.

#### Cost

Measured interleaved, alternating builds, minimum of nine, one benchmark per
process:

| | before | after |
|---|---:|---:|
| `BenchmarkParse_GlobalVars` | 42.3µs | **47.8µs (+12.8%)** |
| `BenchmarkParse_ScriptCFC` | | +1.3% |
| `BenchmarkParse_ScriptCFC_ExtractCalls` | | −2.9% |
| `BenchmarkParse_TagCFC` | | −0.4% |
| `BenchmarkParse_TagCFC_ExtractCalls` | | −1.6% |

Allocations are identical everywhere — the extra work is a second pass over the
bytes of a string that contains a `#`, not a new object. `GlobalVars` is the one
that moves because it is the smallest benchmark and is almost entirely string
scanning; the whole-file parses absorb it into noise.

## 4. Known and not fixed

### 4.1 `createObject` — 435 sites, deliberate

The parser records a `ComponentRef`, not a `CallSite`. This is the one entry in
`expectedDifferences` that is a design difference rather than a gap.

### 4.2 The parser records 1,098 sites the grammar does not

Two causes, both pre-existing and neither a wrong answer about code that runs:

- **Line skew**, as in §2.
- **Tag-shaped text inside a string.** `coldbox-platform/system/aop/Mixer.cfc`
  builds CFML source as a string literal; the tag parser scans `<cfset structDelete(…)>`
  inside it as a live tag, and the grammar knows it is a string. That is the
  same class as the comment-in-an-attribute-list note in CLAUDE.md: the tag
  parser matches tags wherever they look like tags.

## 5. What would change these decisions

- **§3.1's cost** — if `unresolved` or the code map ever shows the sub-parse in a
  profile, the answer is to reuse one `scriptParser` per file rather than
  building one per `<cfset>`.
- **§3.3's opt-in** — it is a flag on the scanner because one caller
  (`parseFuncBody`) feeds it markup. If that path ever stops reusing the script
  parser for a tag function's body, the flag stops earning its keep and the rule
  can be unconditional.

Re-run `make gapcheck CORPUS=…` after anything here; it is 6 seconds, and the
`expectedDifferences` list in `internal/tsoracle` fails on a difference that is
neither recorded nor fixed.

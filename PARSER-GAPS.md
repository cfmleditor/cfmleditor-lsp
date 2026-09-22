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

| | Before | After §3 |
|---|---:|---:|
| Files where the two disagree | 1,506 (26.8%) | **1,340 (23.8%)** |
| Call sites the grammar sees and the parser misses | 10,451 | **8,707** |
| Sites the parser records and the grammar does not | 1,065 | 1,070 |

Keyed on the method name alone — ignoring which line it landed on — the missed
figure is 9,726 before and 7,982 after. The difference between the two keyings,
around 700 sites, is **line skew**: a multi-line `<cfset>` or `<cfif>` records
its calls at the tag's starting line while the grammar puts each on its own.
Cosmetic, but it is why a report of this keyed on lines cannot be diffed
cleanly against another.

## 3. Fixed: expressions a `<cfset>` holds

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

### Cost

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

## 4. Known and not fixed

### 4.1 A same-quoted string opened inside `#...#` — 888 sites, 56 files

```cfml
assertEquals("7", "#DayOfWeek("{ts '2000-1-1 17:26:03'}")#");
```

The scanner closes the outer string at the inner quote, so the token is
`"#DayOfWeek("` and the interpolation never has a closing `#` to find:

```
"#DayOfWeek("   {   ts   '2000-1-1'   }   ")#"
```

This is a **scanner** limitation, not an interpolation one — `'…'` inside
`"…"` works, and so does `"#DayOfWeek(d1)#"`. Fixing it means making
`scanString` aware that a `#` opens an expression in which a string may nest,
which is a real change to the one function every parse path runs through.

Nearly all of it is Lucee's test suite writing dates this way. Worth revisiting
if it shows up outside that idiom.

### 4.2 Calls in a script-tag attribute value

```cfml
loop array=structKeyArray(rowData) item="local.col" { … }
http url="u" result=serializeJson(body.data) {}
```

`skipTagAttrValue` consumes the value and hands nested parens to the scan that
finds calls inside them, but the value's own outer call is consumed as a plain
token. The fix is the same shape as §3.

### 4.3 `createObject` — 435 sites, deliberate

The parser records a `ComponentRef`, not a `CallSite`. This is the one entry in
`expectedDifferences` that is a design difference rather than a gap.

### 4.4 The parser records 1,070 sites the grammar does not

Two causes, both pre-existing and neither a wrong answer about code that runs:

- **Line skew**, as in §2.
- **Tag-shaped text inside a string.** `coldbox-platform/system/aop/Mixer.cfc`
  builds CFML source as a string literal; the tag parser scans `<cfset structDelete(…)>`
  inside it as a live tag, and the grammar knows it is a string. That is the
  same class as the comment-in-an-attribute-list note in CLAUDE.md: the tag
  parser matches tags wherever they look like tags.

## 5. What would change these decisions

- **§4.1** — the shape appearing outside Lucee's date tests. It is 56 files
  today, and the fix touches the scanner every parse path runs through.
- **§4.2** — it is small, and the fix is the one §3 already demonstrates, so it
  is worth doing the next time someone is in `parseScriptTagAttrs`.
- **§3's cost** — if `unresolved` or the code map ever shows the sub-parse in a
  profile, the answer is to reuse one `scriptParser` per file rather than
  building one per `<cfset>`.

Re-run `make gapcheck CORPUS=…` after anything here; it is 6 seconds, and the
`expectedDifferences` list in `internal/tsoracle` fails on a difference that is
neither recorded nor fixed.

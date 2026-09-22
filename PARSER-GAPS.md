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

| | Before | After §3.3 | After §3.7 | After §3.11 |
|---|---:|---:|---:|---:|
| Files where the two disagree | 1,506 (26.8%) | 837 (14.9%) | 693 (12.3%) | **556 (9.9%)** |
| Call sites the grammar sees and the parser misses | 10,451 | 5,357 | 1,798 | **1,144** |
| Sites the parser records and the grammar does not | 1,065 | 1,098 | 903 | 652 |

Keyed on the method name alone — ignoring which line it landed on — the missed
figure is 9,726 before, 4,602 after §3.3, 1,188 after §3.7 and **787** after
§3.11, and the parser-only figure falls from 343 to 295. Of the 787 that remain,
**545 are the deliberate differences in §4.1**, so about 240 are real.

The per-step figures are:

| After | files | missed | name-keyed | parser-only |
|---|---:|---:|---:|---:|
| §3.1 `<cfset>` expressions | 1,340 | 8,707 | 7,982 | 1,070 |
| §3.2 script-tag attribute values | 1,297 | 8,500 | 7,775 | 1,067 |
| §3.3 nested strings in `#...#` | 837 | 5,357 | 4,602 | 1,098 |
| §3.4 CFML string escapes | 781 | 2,736 | 2,122 | 908 |
| §3.5 a leading BOM | 780 | 2,254 | 1,641 | 905 |
| §3.6 constructor arguments | 746 | 2,102 | 1,489 | 906 |
| §3.7 a call on a scope | 693 | 1,798 | 1,188 | 903 |
| §3.8 a bracket index | 666 | 1,661 | 1,051 | 903 |
| §3.9 a scoped chain's receiver | 639 | 1,408 | 1,051 | 650 |
| §3.10 a span is not a tag boundary | 599 | 1,267 | 910 | 650 |
| §3.11 a span may hold a nested span | 556 | 1,144 | 787 | 652 |

The difference between the two keyings is **line skew**: a multi-line `<cfset>`
or `<cfif>` records its calls at the tag's starting line while the grammar puts
each on its own. Cosmetic, but it is why a report of this keyed on lines cannot
be diffed cleanly against another.

**Note what §3.9 does and does not move.** Its name-keyed figure is unchanged,
because the oracle compares line and method name and deliberately *not* the
receiver — a call recorded against the wrong receiver still counts as found.
What it fixed was a wrong answer, and the only trace of that in the tally is the
line: 251 sites where a rediscovered bare call landed somewhere the grammar did
not put it.

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

### 3.4 CFML's string escapes are not C's

```cfml
listLast( uri, "/\" )                   <!-- ends in a backslash -->
getTempDirectory() & "lucee-tests\" & id
x = "say ""hi"""                        <!-- CFML's real escape -->
```

The scanner honoured `\` as a string escape. CFML has none: a quote is escaped
by **doubling** it, and a backslash is an ordinary character. So a string ending
in a backslash did not close where it ends — it closed at the next quote
anywhere in the file, swallowing every call in between.

One occurrence of `listLast( uri, "/\" )` in ContentBox's `BaseContent.cfc`
cost **333 call sites**: everything from line 774 to the end of a 1,918-line
component. The same two characters run through Lucee's test suite and ColdBox's
`Builder.cfc`.

Doubling is checked from *inside* the string, which is what keeps a bare `""` the
empty string and makes `""""` a string holding one quote. Applied to the opening
quote instead, `f("", "b")` becomes one argument.

A `""`-escaped string is also how CFML writes markup across several lines, so a
string token spanning lines is ordinary once the rule is right.
`scanInterpolation` therefore places each `#...#` span at the token's start plus
the newlines before it, rather than reporting every call in a multi-line string
at the line the string opened.

**2,480 name-keyed sites.** Free: within noise on all five parse benchmarks and
slightly faster on three, since the hot loop loses a branch.

### 3.5 A leading UTF-8 BOM

`ClassifyRegions` decides script vs tag syntax by asking the scanner for the
first token and testing it against `component`. A BOM is invisible in an editor,
**561 of the 5,629 corpus files carry one**, and the scanner stopped at it — so
those files answered "not a component" and went to the tag splitter.

Harmless until the file also mentions `<script>`, which `isScriptFile` reads as
evidence of an HTML page. ColdBox's `HTMLHelperSpec.cfc` only mentions it inside
the string literals the spec asserts against, and came apart into eight regions
each parsed from the middle of an expression: no function found in 800 lines.

**481 name-keyed sites.** The skip lives in `NewScanner` rather than in
`isScriptFile`, because a BOM should not be a token anywhere.

### 3.6 A constructor's argument list

```cfml
var q = new Query( datasource = getDatasource() );
var c = new X().g( svc.f() );
```

`skipParenBody` replaced the discard-a-token-at-a-time paren loop everywhere a
call could hide — except in the four paths that read a `new` expression, which
each kept a `skipBalancedParens` of their own: `parseNewRef`,
`parseStandaloneNew`, `checkReturnComponent` and `scanChainedCalls`. They were
reached from the `new` arm rather than from the argument scan, which is how they
survived the consolidation.

`new Query( datasource = getDatasource() )` is how ContentBox's migrations reach
a datasource: **118 sites under that one method name, 152 in all.** The
instantiation itself stays a `ComponentRef` — the deliberate difference in §4.1
— so only the arguments change.

It also produced a difference in the *other* direction where the parser is
right: testbox's `compat-directory-runner.cfm` has
`directory="#expandPath( '…' )#"` inside a `<cfset>`, which the parser now
records and the grammar does not see as a call at all.

### 3.7 A call made directly on a scope

```cfml
this.init();
variables.buildCache();
request.getRemoteClients();
```

Both scoped-var handlers read `scope` `.` `name` and then looked only for `=`
(an assignment) or `.` (a longer chain). The shape where the statement *is* the
call had no arm, so it recorded nothing. `x = request.getRemote()` and
`request.a.getRemote()` both worked, which is why it survived — and the bare
statement form is how a component calls its own method with an explicit scope.

Where the call lands depends on the scope, and getting that wrong is worse than
the miss:

- `this.` and `variables.` name a member of the component being parsed, so the
  call is recorded **unqualified** and resolves against the file's own functions
  — including the ones `parseFunctionValue` files from
  `this.helper = function(){}`.
- Every other scope holds a value put there at runtime, so the receiver is
  **`$any`**: the call site is recorded and the method-exists check is skipped,
  as it already is for a literal receiver. Recorded unqualified instead,
  `request.getRemote()` in a file that declares a `getRemote` would be an edge to
  a function the call never reaches.

The scope is read from the **token**, not from the `Scope` value the dispatch
passes: `request`, `session` and `application` are all dispatched as
`ScopeVariables`, deliberately, so an assignment through one keeps the component
its right-hand side establishes. Testing the enum put every one of them in the
first group.

**301 name-keyed sites**, 109 of them under `getRemoteClients` alone.

### 3.8 A call inside a bracket index

```cfml
max : sorted[ sorted.len() ],
avg : round( total / times.len() )
```

`skipBracketIndex` mirrored the *old* `skipParens`: it discarded its group a
token at a time. An index is an expression like any other, so
`sorted[ sorted.len() ]`, `arr[ f() ]` and `a.b[ f() ]` recorded nothing at all,
and `g( arr[ f() ] )` recorded only `g` — the same defect `skipParenBody` exists
to have fixed, left in the one group the consolidation did not reach.

The chain text callers build is unchanged: a bracket still poisons it with the
literal `[]` marker, so `REQUEST[key].method()` still falls through to an honest
"no component ref" rather than resolving as `REQUEST.method()`. Only the key
expression is read rather than thrown away. **137 name-keyed sites.**

### 3.9 The receiver of a hop chained onto a scoped call

```cfml
variables.a.b().c();
variables.helper().c();
request.get().c();
```

Both scoped-var handlers recorded the first call and then merely skipped its
argument list, so the `.c()` was left for the outer loop to rediscover as an
orphaned *bare* call — in a component that declares a `c`, an edge the call never
takes. `continueChainCalls` exists to stop exactly this on the unscoped path, so
the scoped path goes through the same helper rather than growing a second one.
A call made directly on a scope needed the same, with the receiver its first call
already earns: `variables.helper().c()` walks the chain through the member's
declared return type, `request.get().c()` stays dynamic all the way down.

**This is the fix `make gapcheck` can least see** — see the note under §2.

### 3.10 A `#...#` span is not a tag boundary

```cfml
#ETH.author( content = "<strong>@name@</strong>" )#

#ETH.divider()#
```

Markup written inside a span is an argument, not a tag. The walk found the next
`<` without regard for the span, so `scanInterpolatedText` got a chunk with an
unterminated `#` — losing that call — and the hashes left over mis-paired with
the next span, losing the call after it too.

This is the §4.2 entry a previous round recorded as known-and-not-fixed after an
attempt took missed sites from 1,798 to 3,025. **That attempt was wrong in one
line**: it advanced only past the *opening* hash of a span it declined, so the
closing hash became the next opening one and every pairing after it was
inverted. Two plain interpolations on one line start the cascade —
`<li><b>#myKey#</b>: #prc.info[ k ]#</li>` swallowed the `<cfif>` below it, which
is 141 files and 773 sites on its own.

Three rules, each measured by removing it and re-running the corpus:

| Rule | cost of removing it |
|---|---|
| A span's hashes pair whether or not its contents are stepped over | 141 files, 773 sites |
| A span with no `(` does not hide a tag (a stylesheet rule holds none) | 5 files |
| A span with unbalanced quotes does not either (`$( '#search' )`) | 7 files |

Hashes interpolate inside `<cfoutput>` and in a tag's attributes and this walk
tracks neither, so these are heuristics rather than a parse. They are chosen so
that being wrong costs a call rather than a tag, and the corpus says they are
right everywhere in 5,629 files. **141 name-keyed sites**, and the bulk of the
scan stays an `IndexByte`: a byte-at-a-time version cost +6.9% on
`Parse_TagCFC_ExtractCalls`, this one is flat.

### 3.11 A span may hold a string that holds a span

```cfml
<link href="#cb.themeRoot()#/#html.elixirPath( root='#cb.themeRoot()#/inc' )#">
```

`interpolatedSpans` took the first `#` it met as the close, so the *inner*
opening hash terminated the outer span and every pairing after it on the line
was inverted. `Scanner.scanHashExpr` already reads CFScript this way;
`matchingHash` is the same rule for the text a tag parser hands over, and the two
were parallel implementations of it.

**`nextTagStart` deliberately does not share that rule.** The two scans optimise
different errors: a wrong span in the tag walk costs a *tag*, so it stays
conservative and pairs hashes as they come; a wrong span here costs at most a
call. Making the walk share `matchingHash` fixed one corpus site and broke
thirty-six, because skipping quoted strings runs a span much further in markup.

**123 name-keyed sites**, against one lost in `services.updateold.cfm` where the
walk's conservative pairing splits a span this scan would read whole.
`Parse_TagCFC` +1.4% on the minimum of 25 interleaved samples, which is the one
cost in this round that lands on the editor's keystroke path.

## 4. Known and not fixed

### 4.1 `createObject`, `entityNew`, `entityLoad` — 545 sites, deliberate

The parser records a `ComponentRef`, not a `CallSite`. These are the three
entries in `expectedDifferences` that are design differences rather than gaps,
and they are now **69% of what is left**.

`entityNew` and `entityLoad` were recorded only after a second corpus run
re-investigated them from scratch, which is the cost of a deliberate difference
nobody writes down. Their fixture lines sit at the end of
`testdata/DefinitionTest.cfc` because `definition_testdata_test.go` addresses
that file by line number.

### 4.2 The parser records 652 sites the grammar does not

Three causes, none of them a wrong answer about code that runs:

- **Line skew**, as in §2.
- **Tag-shaped text inside a string.** `coldbox-platform/system/aop/Mixer.cfc`
  builds CFML source as a string literal; the tag parser scans
  `<cfset structDelete(…)>` inside it as a live tag, and the grammar knows it is
  a string. That is the same class as the comment-in-an-attribute-list note in
  CLAUDE.md: the tag parser matches tags wherever they look like tags.
- **The grammar's own gaps**, as in §3.6's `expandPath` case.

## 5. What would change these decisions

- **§3.10 and §3.11's heuristics** — the honest fix is the tag walk knowing where
  interpolation is *live*, which is `<cfoutput>` nesting plus attribute context.
  Until then the two scans stay deliberately different, and the table in §3.10 is
  what any replacement has to beat.
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

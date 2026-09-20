# Performance gaps: costs identified and not acted on

Three costs found while profiling the per-keystroke paths for
[#101](https://github.com/cfmleditor/cfmleditor-lsp/pull/101), none of them
acted on. Two change what a client receives and so are not tune-ups; the third
is not worth what it costs to write. Section 4 is settled; sections 2 and 3 are
deferred, and section 2 records what a reader should know before picking it up.

This file exists so they are not rediscovered from scratch, and so the reasons
are on record rather than remembered. Every number below is measured, and the
method is in section 1 — including the mistake that made one of them look
eighty times smaller than it is.

## 1. Method, and why the earlier numbers were wrong

Measured on a synthetic index of 5,000 files declaring 8 methods each — 40,000
definitions — built by `benchLoadedServer` in
`internal/server/perf_bench_test.go`. Each figure is the minimum of several
runs of a single benchmark run **on its own**; see the contamination note in
CLAUDE.md for why that matters.

**The handler is not the request.** A handler benchmark measures the answer
being computed and not the answer being sent, and for these two the sending is
almost all of it:

| | handler | marshal | handler's share |
|---|---:|---:|---:|
| `workspace/symbol`, query `method3` | 0.24ms | 3.02ms | 7% |
| `workspace/symbol`, query `m` | 1.6ms | 24.4ms | 6% |
| `textDocument/completion` | 0.002ms | 0.63ms | 0.3% |

`BenchmarkWorkspaceSymbol` measures only the handler, which is how the
`workspace/symbol` cost below was first recorded as 296µs when the request
costs 3.3ms. `BenchmarkWorkspaceSymbolWithMarshal` now measures it end to end,
next to the `BenchmarkCompletionWithMarshal` that already did.

The consequence for anything on this page: **the only way to make these
requests cheaper is to send fewer bytes.** Work saved inside the handler is
rounding error against the encode.

Two smaller notes from the same measurements:

- The wire already uses `encoding/json/v2` — `go.lsp.dev/jsonrpc2`'s default
  codec. There is no encoder swap left to make. On this payload v2 is in fact
  *slower* than v1 (748µs against 625µs for the completion list), so the
  benchmarks, which marshal with v1, understate the real cost slightly rather
  than overstating it.
- The completion payload has no redundant fields to drop. `sortText` carries a
  sort-priority prefix (`8abs`), so it is not the label; `insertText` is a
  snippet (`abs(${1:number})`), so it is not the label either; `filterText` is
  already omitted on every item. There is no free saving here — only the two
  fields in section 3.

**Synthetic-index caveat.** Every definition in the bench index is named
`method0`–`method7`, so the query `m` matches all 40,000. A real workspace has
varied names and a one-character query would match some fraction of them. The
shape is what carries over, not the multiplier: matching is a case-folded
*substring* test over every distinct name, so short queries match a large share
of a real workspace too, and short queries are the ones always sent.

## 2. `workspace/symbol` sends the whole match set — deferred

**This is the largest cost on this page by an order of magnitude, and it is
not being worked on: other things come first.** That is the whole of the
reason, and it is worth stating plainly rather than dressing as a technical
judgement — nothing below argues the cap is wrong, only what a reader should
know before picking it up.

The measurements are kept in full because the cost is real and has not gone
anywhere.

`handleWorkspaceSymbol` (`internal/server/symbol.go`) turns every matching
definition into a `protocol.SymbolInformation` and returns all of them. The
picker issues a request per keystroke, and the first keystrokes are the short
queries that match the most:

| query | matches | JSON | total |
|---|---:|---:|---:|
| `m` | 40,000 | 6.2MB | **26ms** |
| `me` | 40,000 | 6.2MB | 26ms |
| `method` | 40,000 | 6.2MB | 26ms |
| `method3` | 5,000 | 776KB | 3.3ms |
| `zzz` | 0 | 2 B | 0.003ms |

A symbol is 151 bytes on the wire:

```json
{"name":"method3","kind":12,"location":{"uri":"file:///ws/pkg0/File0.cfc",
 "range":{"start":{"line":25,"character":0},"end":{"line":25,"character":0}}}}
```

### Options

**(a) Cap the result count.** The standard answer — gopls caps at 100 by
default (`symbolLimit`), rust-analyzer at 128 for inexact queries. Measured
against the same index:

| cap | matches sent | JSON | marshal |
|---|---:|---:|---:|
| none | 40,000 | 6.2MB | 24.4ms |
| 1,000 | 1,000 | 154KB | 0.60ms |
| 100 | 100 | 15KB | 0.06ms |

A cap of 100 is roughly **13x** on the `method3` query and **400x** on `m`.

The cost is real: a symbol past the cap cannot be found, and with an arbitrary
cut there is no reason the ones kept are the ones wanted. If this is done, the
cap should be applied to a *ranked* set — the same `cfpath.URIDistance` and
lowest-URI ordering `LookupPreferred` and `definition.go` already use — so the
survivors are the near ones rather than whichever bucket the parallel scan
filled first. Ranking 40,000 entries costs something, but far less than
encoding them.

**(b) LSP 3.17 `WorkspaceSymbol` with a URI-only location**, resolved through
`workspaceSymbol/resolve`, gated on the client's
`workspace.symbol.resolveSupport`. Drops the range from each entry — 151 bytes
to about 75 — so roughly half, with **no result lost**. It does not solve the
problem on its own (6.2MB becomes ~3MB, 26ms becomes ~13ms) and it needs a new
handler, but it composes with (a) and it costs the user nothing.

**(c) Require a minimum query length.** One or two characters return nothing
until the query is worth answering. Cheap, but it makes the picker feel broken
rather than fast.

### What to know before picking this up

The cost is real, the fix is known, and it is a priority call rather than a
technical one. Three things about the shape of it, so whoever gets to it is not
starting from the top:

- **A cap is the only option that touches the real cost, and it is not free.**
  It buys latency in a feature whose entire job is *finding things*, by making
  some things unfindable — and a symbol search that is fast and silently
  incomplete is a harder failure to notice than one that is slow, because the
  user cannot tell "no such symbol" from "past the cut". Ranking before the cut
  makes the survivors plausible rather than arbitrary, which is the difference
  between a defensible cap and a careless one, but it does not make the cut
  lossless. That is what makes the default a judgement rather than a constant:
  gopls settled on 100, rust-analyzer on 128, and the repo's convention would
  be a documented default with a config key over it.
- **The headline number is synthetic.** Every definition in the bench index is
  named `method0`–`method7`, so the query `m` matches all 40,000 and the 26ms
  is a worst case built to be one. A real workspace has varied names; the shape
  carries over (case-folded substring matching over every distinct name, so
  short queries match a large share) but the multiplier does not, and nobody
  has measured a real one. Measuring that first would say how urgent this
  actually is, and is much cheaper than building the cap.
- **The picker is used deliberately, not continuously**, which is the main
  reason this can wait while the parse and index paths could not. It is a
  per-keystroke cost inside a burst the user chose to start; those run while
  someone is simply typing code. A slow symbol search is felt once per search.

Option (b) — the LSP 3.17 `WorkspaceSymbol` with a URI-only location — is the
one piece with no downside to the user, and is the place to start if this is
picked up: it drops the range from every entry with no result lost. On its own
it halves a number that needs an order of magnitude, and it wants a client
advertising `workspace.symbol.resolveSupport` to be worth the handler, which
is why it is not a fix by itself.

## 3. Completion sends documentation and detail for every item

A completion response is 977 items and 331KB, and the handler is 2µs of the
633µs it takes. Two fields are two thirds of the payload:

| variant | JSON | marshal |
|---|---:|---:|
| as sent today | 331KB | 633µs |
| without `documentation` | 218KB (−34%) | 446µs (−30%) |
| without `detail` | 264KB (−20%) | 519µs (−18%) |
| without either | **151KB (−54%)** | **351µs (−45%)** |

`documentation` averages 100 bytes an item and `detail` 57, and both are for
the *one* item the user eventually looks at.

### Options

**(a) `completionItem/resolve`.** Send the list without those two fields and
fill them in when the client asks about a highlighted item. This is what the
request exists for. It needs `completionProvider.resolveProvider: true`, a
`completionItem/resolve` handler, and enough in `CompletionItem.Data` to find
the item again.

It must be gated on the client's advertised
`textDocument.completion.completionItem.resolveSupport` — a client that does
not resolve would show a popup with no documentation at all, which is a
regression, not an optimisation. The server already reads client capabilities
in `handleInitialize` (see `clientWatchesFiles`), so the gate is cheap; the
handler is the work.

**(b) Defer only `documentation`, keep `detail`.** `detail` is the signature
line, shown inline in the popup list rather than in the expanded panel, so it
is the one a user reads without asking. Two thirds of the saving for a much
smaller behavioural change — but the same resolve machinery, so it is not
cheaper to build.

**(c) `CompletionList.itemDefaults` (LSP 3.17)** hoists fields shared by every
item out of the list. It covers `insertTextFormat`, which is `2` on every
snippet item — about 21KB, 6% — and does not cover `documentation` or
`detail`. Worth doing alongside (a), not instead of it.

Not done here because it is a feature with capability negotiation and a
resolve handler, not a tune-up, and because it is the second-largest item on
this page rather than the first.

## 4. `LookupPreferred` scans the name's bucket — measured, and not worth it

When a bare function name is not declared in the requesting file,
`nearestTo` (`internal/index/index.go`) calls `cfpath.URIDistance` against
every definition of that name to pick the closest. On a shared name across
5,000 files that is ~150µs, on hover, signature help and argument completion.

The only saving available is that the reference URI is fixed across the loop,
so the part of `URIDistance` that depends only on it — the backward scan to the
last separator, and the count of separators after it — can be computed once.
Prototyped and measured over 5,000 entries: **205µs to 168µs, −18.4%**, with
the prepared version agreeing with the current one on every URI.

That is under a fifth of a cost that is already a fraction of the parse it
accompanies, in exchange for a second implementation of a function whose
comment explains at length why its answer must be exactly what it is. A
smaller constant does not change the shape either — the minimum of N distances
needs all N.

Skipping the scan entirely would need the buckets indexed by directory, which
is the same few-megabytes-per-index trade rejected for the bucket search in
#101 and rejected again here for less benefit.

**Not worth doing.** Unlike sections 2 and 3, this one is closed rather than
deferred: if the 150µs ever matters, the answer is to stop asking — cache the
preferred definition per (name, file) and invalidate it on an index write —
not to make the scan 18% faster.

## 5. What would change these decisions

- **Section 2** — room to do it. It is deferred on priority, not on evidence,
  so nothing has to happen first. What would move it up the list: a report of
  the symbol picker being felt as slow, or a measurement on a real workspace
  showing what a one- or two-character query matches there — the cheap step
  that would say whether the synthetic 26ms is anywhere near the real one.
- **Section 3** — a client in use that advertises `resolveSupport`, or a
  workspace where completion latency is being felt. The saving is known and
  large; only the build cost is holding it.
- **Section 4** — a profile showing `nearestTo` mattering on a real workspace
  rather than on a bucket built to be worst-case. Then cache the answer rather
  than speed up the scan.

Anything here that gets done should re-measure end to end, with the marshal,
on the benchmark named in section 1 — not on a handler benchmark.

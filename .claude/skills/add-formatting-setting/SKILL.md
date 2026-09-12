---
name: add-formatting-setting
description: add a formatting option to cfmleditor-lsp — the config hops a new `formatting` key crosses, the defaults that must not move, and how to verify it against the corpus
---

Add a new key under `formatting` in `.cfmleditor.json` and wire it through to the
formatter.

## The rule that governs everything else

**Unset must reproduce today's output byte for byte.** Every project already
formatted by this tool depends on it. A setting whose default changes output
reformats files nobody asked to touch, and the diff turns up in someone's next
commit rather than in this one.

That constrains the default value: it is whatever the formatter already does,
even when that is the less tidy option. `paramBreakThreshold` defaults to `0`
("always expand") because that is what the code did, not because it is a good
default.

## The hops

A key crosses eight hand-written lists. Miss one and it resolves to its default
and silently does nothing — which reads exactly like the setting not working.

| # | File | What to add |
|---|---|---|
| 1 | `internal/config/config.go` — `Formatting` | the field + `json` tag. Bools and ints are **pointers** (`*bool`, `*int`) so Merge can tell "set false" from "not mentioned"; strings are plain |
| 2 | `internal/config/config.go` — `ResolvedFormatting` | the same field, non-pointer |
| 3 | `internal/config/config.go` — `Resolve` | `BoolDefault(f.X, <default>)` / `IntDefault(f.X, <default>)` / `f.X` |
| 4 | `internal/config/config.go` — `mergeFormatting` | an entry in the `*bool`, `*int` or `*string` list |
| 5 | `internal/config/formatting.go` — `FormatterOptions` | copy onto `formatter.Options` |
| 6 | `internal/config/formatting.go` — `DefaultResolvedFormatting` | only if the default is not the zero value |
| 7 | `internal/daemon/config.go` | a `FormattingX()` accessor **and** a line in `ResolvedFormatting()` |
| 8 | `internal/formatter/formatter.go` | the `Options` field, plus `DefaultOptions()` if the default is not the zero value |

Then `README.md` (the settings table) and `CHANGELOG.md`.

Three reflective tests already cover hops 3, 4, 5 and 7 and will fail with a
precise message if you miss one — run `go test ./internal/config/ ./internal/daemon/`
early rather than discovering it from the corpus:

- `TestMergeFormattingCoversEveryField` (hop 4)
- `TestFormattingKeysReachTheFormatter` (hops 3 and 5)
- `TestResolvedFormattingReadsEveryKey` (hop 7)

## Two traps in the defaults

**Ints: `FormatterOptions` only copies when `> 0`.** A zero means "unset", so the
real default has to live in `DefaultOptions()`. This works out when the wanted
default *is* zero, but check both places agree.

**Bools: `Resolve` applies a per-field default.** A field you forgot returns that
default, which usually looks plausible. This is why the reflective tests set
bools both ways — a single run cannot tell a dropped field from a working one.

## Verify against the corpus

Not optional. A rule that reads as obviously safe has repeatedly turned out to
move output on some construct no fixture contains.

```bash
CORPUS=/src/Lucee:/src/ContentBox:/src/coldbox-platform:/src/fw1:/src/testbox:/src/cfmleditor

# 1. baseline, from a clean tree (git stash your change first)
make corpus CORPUS=$CORPUS REPORT=/tmp/before.tsv

# 2. defaults, with the change applied — must be identical
make corpus CORPUS=$CORPUS BASELINE=/tmp/before.tsv

# 3. every mode the setting offers
make corpus CORPUS=$CORPUS OPTS=myKey=value
make corpus CORPUS=$CORPUS OPTS=myKey=otherValue
```

`BASELINE` compares **per file**, which the totals cannot: a change that breaks
one file and fixes another leaves every column identical. `OPTS` sets
`formatter.Options` fields by name, so a mode sweep needs no edit to the test.

Watch the `shape` column as well as `unstab` — see `malformedShape` in
`internal/formatter/corpus_test.go`. A setting that moves braces can trip it, and
that is a finding either way: either the output is wrong, or the rule needs
scoping to the styles it applies to (as the column-one rule is scoped to
same-line braces).

## Tests

Write them in `internal/formatter/`, following `paren_spacing_test.go` or
`block_padding_test.go`:

- unset reproduces today's output — the constraint above, stated as a test;
- each mode does what it claims;
- **idempotency in every mode**. This is where these settings fail. A rule that
  moves a line break makes the second pass read a different source, and an
  untouched file then produces a fresh diff on every save;
- the boundaries — an empty list, a comment, whatever the rule must *not* touch.

**Confirm each test fails with its half of the change neutered.** Comment out the
new branch, run the test, see it fail, put it back. Tests that pass either way
have been written here more than once.

**Print the actual output before writing an expected string.** Hand-counting
indentation for a nested fixture is wrong more often than it is right; format the
fixture, look at it, then assert on what should be there.

## Pitfall: two block renderers

`scriptBlock` and `scriptBlockOf2` (`internal/formatter/cfscript_formatter.go`)
both emit a block — the second gives braces to a single-statement body. Their
padding must match **exactly**, because on a second format the braces are in the
source and the first renderer runs. Anything touching block layout goes through
the shared helpers (`blockPadOpen`, `blockPadClose`, `writeOpenBrace`), and
`TestBlankLinesInBlocksAgreeAcrossBothBlockRenderers` checks they still agree.

## When to withhold

If the corpus shows guard rejections or new unstable files in a mode, do not ship
that mode. `parenPosition: hanging` was withdrawn for exactly this. Report the
numbers and let the user decide.

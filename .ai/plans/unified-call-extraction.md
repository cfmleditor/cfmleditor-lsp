# Unified Call Site Extraction in Parsers

## Goal

Move all `variable.method()` call site detection into the script/tag parsers, eliminating the separate line-based `FuncCalls` scanner. This means one pass through the content extracts everything: refs, links, resolvers, AND call sites.

## Current State

Three separate call-scanning mechanisms:
1. **Script/tag parsers** — only detect calls in assignment context (`x = obj.method()`)
2. **`scanLineForCalls`** — line-based, searches for specific function names (FindCalls option)
3. **`FuncCalls`** — line-based, finds ALL `variable.method(` patterns by scanning for `(` and walking backwards

`FuncCalls` is used by:
- `exportDeps` command (dependency graphs)
- `unresolved` CLI command (validates all calls)
- `findRefs` command (traces call chains)

## Plan

### 0. Make call extraction opt-in

Call extraction adds overhead that isn't needed for all parse contexts. Function signature parsing (e.g. hover, completions for parameter names) only needs refs/links, not call sites.

Add an option to the parse request:

```go
type ParseOptions struct {
    ExtractCalls bool // when true, record all call sites during parsing
}
```

The parsers check this flag before recording calls. Callers that need call data (exportDeps, unresolved, findRefs) pass `ExtractCalls: true`. Callers that only need signatures/refs leave it false (the default).

### 1. Add `calls` slice to scriptParser and tagParser

```go
type scriptParser struct {
    ...
    calls []CallSite // all variable.method() calls found (only populated when ExtractCalls is true)
}
```

### 2. Capture calls in the script parser's body processing

In `handleBodyToken` → `default` case, when we see `ident.ident(`:
- Currently only enters `checkAssignRef` which requires a preceding `=`
- Add detection for bare calls: when a dot chain ends with `(` but there's no `=` before it, record a CallSite

The key insight: the parser already walks dot chains in the `default` case of `handleBodyToken` via `checkAssignRef`. We need a sibling path that doesn't require an assignment.

### 3. Track calls in the script parser's main loop

For global-scope calls (outside functions), detect `obj.method(` patterns in the top-level parse loop.

### 4. Capture calls in the tag parser

For tag-based code, calls appear in:
- `<cfset obj.method(...)>` without assignment
- `<cfinvoke>` tags (already handled for refs)
- Inline expressions in attributes

The primary case is `<cfset>` without `=` — the tag parser already strips the tag wrapper; if there's no `=`, the whole inner content is a call expression.

### 5. Add `funcCalls` map to parsers (per-scope)

```go
funcCalls map[string][]CallSite // keyed by "start:end" scope
```

Route calls to the appropriate scope (global or per-function).

### 6. Merge calls in buildParseResult

Like `funcRefs` and `funcLinks`, merge parser calls into the ParseResult:
```go
pr.Calls       []CallSite           // global scope calls
pr.funcCalls   map[string][]CallSite // per-function calls
```

### 7. Rewrite FuncCalls to use cached data

```go
func (pr *ParseResult) FuncCalls(funcStart, funcEnd int) []CallSite {
    key := funcKey(funcStart, funcEnd)
    return pr.funcCallsMap[key]
}
```

### 8. Remove the line-based FuncCalls scanner

Delete the current `FuncCalls` implementation (the `for len(body) > 0` loop that scans for `(` and walks backwards).

### 9. Migrate scanLineForCalls into the parser

The `FindCalls` feature (used by exportDeps) searches for calls TO specific functions. This can be a filter on the recorded call sites rather than a separate scan.

## Files to Modify

- `internal/parser/script_parser.go` — Add call detection in body processing
- `internal/parser/tag_parser.go` — Add call detection for `<cfset>` without assignment
- `internal/parser/result.go` — Add call storage, merge in buildParseResult, rewrite FuncCalls
- `internal/parser/ast.go` — Possibly extend CallSite if needed

## Key Considerations

- **Opt-in**: Call extraction is gated behind `ExtractCalls`. Signature-only parses (hover, completions) skip it entirely — no wasted work.
- **Performance**: The parser already processes every token — adding call detection is O(1) per call found, no extra pass needed
- **Accuracy**: The parser-based approach won't be fooled by strings containing `.method(` patterns (the current line scanner uses `maskStrings` as a workaround)
- **Completeness**: Need to handle: bare calls, chained calls (`a.b().c()`), calls in return statements, calls as arguments

## Execution Order

0. Add `ParseOptions` with `ExtractCalls` flag; thread it through to parsers
1. Add `calls`/`funcCalls` fields to script parser
2. Detect bare `obj.method()` calls in `handleBodyToken` (non-assignment context), guarded by `ExtractCalls`
3. Detect calls in main parse loop (global scope)
4. Add same to tag parser for `<cfset>` without `=`
5. Merge calls in buildParseResult
6. Rewrite `FuncCalls` to return cached data
7. Remove old line-based scanner
8. Migrate `scanLineForCalls`/`FindCalls` to filter on recorded calls
9. Update callers: exportDeps/unresolved/findRefs pass `ExtractCalls: true`
10. Tests — verify unresolved/exportDeps/findRefs still work; verify signature-only parses don't populate calls

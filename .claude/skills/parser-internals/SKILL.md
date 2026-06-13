---
name: parser-internals
description: understand, debug, or extend the cfmleditor-lsp parser — scanner tokenisation, the two parse loops, call-site extraction, and how the unresolved command works
---

Use this skill when debugging unexpected parser output, tracing how a CFML construct is tokenised, adding call-site extraction features, or investigating why a variable/ref is wrong in the `unresolved` command output.

## Scanner (`internal/parser/scanner.go`)

The scanner is byte-level. Two predicates control what counts as an identifier:

```go
func isIdentStart(ch byte) bool {
    return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}
func isIdentPart(ch byte) bool { return isIdentStart(ch) || isDigit(ch) }
```

`$` is a valid identifier start — `$assert` tokenises as a single `TokIdent("$assert")`. If you see a `$`-prefixed variable being stripped in output, the scanner was the first place to check (it lacked `$` before the fix in the current working tree).

`charToKind` maps single characters to token kinds. Anything not matched there falls through as `TokOther` and is silently discarded by the parsers' `TokIdent`-only loops.

## Two parse loops

### 1. Top-level loop — `scriptParser.parse()` (`script_parser.go:475`)

Runs over global/component scope. The `default:` arm handles dot-chains:

```
tok (ident) → peek == TokDot → walk chain → peek == TokLParen
  → if extractCalls: addCall{Variable: chain[:lastDot], FuncName: lastIdent}
```

### 2. Function body loop — `scriptParser.parseBody()` → `handleBodyToken()` (`script_parser.go:866, 989`)

`parseBody` drives `handleBodyToken` for every `TokIdent` inside `{ }`. The `default:` arm:

```
tok (ident) → peek == TokLParen  → recordBareCallAndChain (bare call)
tok (ident) → peek == TokDot     → checkAssignRef → checkBareCall
```

`checkBareCall` (`script_parser.go:1623`) walks the dot chain, checks the next peek is `TokLParen`, then records:

```go
chain := "$assert.isEqual"   // fullChain.String()
dotIdx := 7
varName = chain[:dotIdx]     // "$assert"
// CallSite{FuncName: "isEqual", Variable: "$assert", ...}
```

**Key difference from the top-level loop**: `handleBodyToken` only dispatches on the immediate next token. A `$foo.bar()` call goes through `checkAssignRef` → `checkBareCall`, NOT through the dot-chain arm of `parse()`.

## Call-site extraction pipeline

Enable with `ParseOptions{ExtractCalls: true}`.

| Location | Stored in |
|---|---|
| Global / component scope | `pr.Calls` |
| Inside a function | `pr.funcCallsMap["start:end"]` |

`pr.FuncCalls(0, lastLine)` aggregates both when `funcKey(0, lastLine)` is not a real function key — use this in the `unresolved` command to get all calls across the file.

## ComponentRef vs funcRef

| Kind | API |
|---|---|
| Component-scope (`variables.x = new Foo()`) | `pr.ComponentRefs` |
| Function-local (`var x = new Foo()` inside a func) | `pr.FuncComponentRefs(scope.Start, scope.End)` |

`CanResolveCall` in `internal/resolve/resolve.go:277` checks function-scoped refs first, then falls back to `pr.ComponentRefs`. The reason string `"variable 'X' has no component ref"` means neither lookup found a `ComponentRef.Variable` matching `X`.

## The `unresolved` command (`cmd/cfmleditor-lsp/unresolved.go`)

Parse options used:

```go
parser.ParseOptions{
    Resolvers:          cfResolvers,
    ExpressionMappings: expressionMappings,
    ExtractCalls:       true,
    ScanAllScopes:      true,
}
```

Then calls `pr.FuncCalls(0, lastLine)` and feeds each call to `resolver.CanResolveCall`.

`ScanAllScopes` is stored on `ParseResult` but currently has no additional effect beyond `ExtractCalls` — all scopes are already scanned when `ExtractCalls: true`.

## Quick debug checklist

1. **Wrong variable name in output** (e.g. `assert` instead of `$assert`) → check `isIdentStart` in `scanner.go`; the leading character may not be recognised as ident.
2. **Call not captured at all** → confirm `extractCalls: true` is set; check whether the call is at top-level (`parse()`) or inside a function (`handleBodyToken`); trace through `checkBareCall`.
3. **ComponentRef not resolving** → check `pr.ComponentRefs` vs `pr.FuncComponentRefs`; `CanResolveCall` tries function-scoped first.
4. **Rebuild after scanner changes** → `make build` (or `make cfparse` for the debug CLI); the installed binary won't pick up changes automatically.

# Type Inference for Variable Member Functions

## Goal

Infer the type of variables at parse time so the LSP can:
1. Provide accurate member function completions after `.`
2. Validate member function calls in the unresolved scanner
3. Map builtin function return values to their correct type

## Current State

- `VarDef` has `Name`, `Scope`, `Line` — no type info
- Member functions are defined in `internal/docs/member.go` grouped by type (string, array, struct, query, etc.)
- `builtin_returns.go` maps specific functions (fileOpen, imageRead) to method lists
- The parser tracks assignments but doesn't infer types from RHS literals/expressions

## Plan

### 1. Add `Type` field to `VarDef`

```go
type VarDef struct {
    Name  string
    Scope Scope
    Type  string // inferred type: "string", "array", "struct", "query", "numeric", "boolean", "xml", "image", "file", ""
    Line  uint32
}
```

### 2. Infer types from RHS patterns in the script parser

In `parseVarDecl`, `parseBodyVarDecl`, `parseScopedVar`, `parseBodyScopedVar`, and `checkAssignRef` — after consuming `=`, peek at the RHS to determine type:

| RHS Pattern | Inferred Type |
|---|---|
| `""` or `''` (TokString) | string |
| `[]` (TokLBracket) | array |
| `{}` (TokLBrace) | struct |
| Numeric literal | numeric |
| `true` / `false` | boolean |
| `arrayNew(...)` / `arrayAppend(...)` | array |
| `structNew()` / `structCopy(...)` | struct |
| `queryNew(...)` / `queryExecute(...)` | query |
| `xmlParse(...)` / `xmlNew(...)` | xml |
| `imageRead(...)` / `imageNew(...)` | image |
| `fileOpen(...)` | file |
| `listToArray(...)` / `listFind(...)` | (return type from docs) |

### 3. Add type inference to tag parser

In `parseCFSet` / `checkSetRHSStr`, infer type from the RHS string using similar heuristics:
- Starts with `"` or `'` → string
- Starts with `ArrayNew` / `[]` → array
- Starts with `StructNew` / `{}` → struct
- Starts with `QueryNew` / `QueryExecute` → query

### 4. Create a builtin function → return type map

In `internal/docs/builtin_returns.go`, add:

```go
var builtinReturnTypes = map[string]string{
    "arraynew": "array", "arrayappend": "array", "arraymerge": "array",
    "structnew": "struct", "structcopy": "struct",
    "querynew": "query", "queryexecute": "query",
    "xmlparse": "xml", "xmlnew": "xml",
    "imageread": "image", "imagenew": "image",
    "fileopen": "file",
    "listtoarray": "array",
    "serializejson": "string", "deserializejson": "struct",
    // ... etc
}

func LookupBuiltinReturnType(funcName string) string {
    return builtinReturnTypes[strings.ToLower(funcName)]
}
```

### 5. Use inferred type for member function completion

In `internal/server/completion.go` where dot-completion is handled:
- Look up the variable's `VarDef.Type` 
- Filter `docs.AllMemberFunctions()` to only those matching the type
- Fall back to showing all member functions if type is unknown

### 6. Use inferred type in unresolved scanner

In `resolve.go` `CanResolveCall` — when a variable has no component ref:
- Check if variable has an inferred type
- If yes, check if the called method is a valid member function for that type
- Return resolved (no error) if it is

### 7. Store inferred types in ParseResult

Add a method to retrieve the inferred type for a variable at a given line:

```go
func (pr *ParseResult) VarTypeAt(varName string, line int) string
```

This checks function-scoped vars first, then global vars.

## Files to Modify

- `internal/parser/ast.go` — Add `Type` to `VarDef`
- `internal/parser/script_parser.go` — Infer types in var/assignment parsing
- `internal/parser/tag_parser.go` — Infer types in cfset parsing
- `internal/docs/builtin_returns.go` — Add `builtinReturnTypes` map
- `internal/docs/member.go` — Ensure member functions are queryable by type
- `internal/server/completion.go` — Filter completions by inferred type
- `internal/resolve/resolve.go` — Check member functions for typed variables
- `internal/parser/result.go` — Add `VarTypeAt` accessor

## Execution Order

1. Add `Type` to `VarDef` (compile, all tests pass with empty type)
2. Add `builtinReturnTypes` map
3. Implement type inference in script parser (literal detection + builtin return types)
4. Implement type inference in tag parser
5. Add `VarTypeAt` to ParseResult
6. Wire into completion (filter member functions by type)
7. Wire into resolve (accept typed member function calls)
8. Tests

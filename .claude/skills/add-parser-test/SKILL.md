---
name: add-parser-test
description: add, write, or create a parser test in cfmleditor-lsp — patterns and pitfalls for internal/parser/cfparser_test.go
---

Add one or more tests to `internal/parser/cfparser_test.go` for the scenario described.

## Entry points

Choose based on what you need to assert:

| Goal | Call |
|---|---|
| Function names / args only | `ParseFunctionDefs(testURI, content)` |
| Full parse — refs, scopes, vars | `Parse(testURI, content)` |
| Parse with resolvers or other options | `ParseWithOptions(testURI, content, ParseOptions{...})` |

## ParseOptions pitfall

The resolver slice field is **`Resolvers`**, not `ComponentResolvers`:

```go
ParseWithOptions(testURI, content, ParseOptions{
    Resolvers: []Resolver{{Match: `getService("$1")`, Resolve: "services.$1", Prefix: "getService"}},
})
```

## Asserting on scoped data

Local variables and local component refs require going through `pr.Scopes[n]` (0-based, matches function order in file):

```go
pr := Parse(testURI, content)
scope := pr.Scopes[0]
fv   := pr.FuncVars(scope.Start, scope.End)             // []string of local var names
refs := pr.FuncComponentRefs(scope.Start, scope.End)    // local component refs
```

Global refs (component-scope `variables.x = createObject(...)`) → `pr.ComponentRefs`
Global variable names → `pr.GlobalVars()`
Function definitions → `pr.Funcs` or use `assertDefs(t, defs, []string{...})`

## After writing the test

Run it to confirm it passes:

```bash
go test ./internal/parser/... -run <TestName> -v
```

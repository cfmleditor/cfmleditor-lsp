package parser

import (
	"testing"

	"go.lsp.dev/uri"
)

// FuncRefs, FuncCalls and FuncLinks memoise per function, keyed by line range.
// reparseShallow cleared only funcVars, so after a full replace the other three
// kept serving pre-edit answers — and their keys are line ranges, which point at
// a different function once lines move. The visible symptom was hover reporting
// the component a variable held before the edit, and the server re-indexing that
// stale ref.
func TestFullReplaceClearsPerFunctionCaches(t *testing.T) {
	u := uri.File("/tmp/cache/A.cfc")

	before := "component {\n" +
		"\tpublic void function Go() {\n" +
		"\t\tvar svc = new pkg.Foo();\n" +
		"\t\tsvc.Run();\n" +
		"\t}\n" +
		"}\n"

	pr := ParseWithOptions(u, before, ParseOptions{ExtractCalls: true, ExtractLinks: true})

	scope := pr.Scopes[0]

	// Warm every per-function cache.
	refs, _ := pr.FuncRefs(scope.Start, scope.End)
	if len(refs) != 1 || refs[0].Component != "pkg.Foo" {
		t.Fatalf("expected the pre-edit ref, got %+v", refs)
	}

	_ = pr.FuncCalls(scope.Start, scope.End)
	_ = pr.FuncVars(scope.Start, scope.End)

	after := "component {\n" +
		"\tpublic void function Go() {\n" +
		"\t\tvar svc = new pkg.Bar();\n" +
		"\t\tsvc.Run();\n" +
		"\t}\n" +
		"}\n"

	pr.ApplyFullReplace(after)

	scope = pr.Scopes[0]

	refs, _ = pr.FuncRefs(scope.Start, scope.End)
	if len(refs) != 1 {
		t.Fatalf("expected one ref after the replace, got %d: %+v", len(refs), refs)
	}

	if refs[0].Component != "pkg.Bar" {
		t.Errorf("FuncRefs served the pre-edit component %q, want pkg.Bar", refs[0].Component)
	}
}

// The same for a function whose refs are removed entirely.
func TestFullReplaceClearsRemovedRefs(t *testing.T) {
	u := uri.File("/tmp/cache/B.cfc")

	before := "component {\n\tpublic void function Go() {\n\t\tvar svc = new pkg.Foo();\n\t}\n}\n"
	pr := ParseWithOptions(u, before, ParseOptions{ExtractCalls: true})

	scope := pr.Scopes[0]
	if refs, _ := pr.FuncRefs(scope.Start, scope.End); len(refs) != 1 {
		t.Fatalf("expected one ref to start, got %d", len(refs))
	}

	pr.ApplyFullReplace("component {\n\tpublic void function Go() {\n\t\tvar svc = 1;\n\t}\n}\n")

	scope = pr.Scopes[0]
	if refs, _ := pr.FuncRefs(scope.Start, scope.End); len(refs) != 0 {
		t.Errorf("stale refs survived a replace that removed them: %+v", refs)
	}
}

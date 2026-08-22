package index

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

func TestIndexFile_LookupRoundTrip(t *testing.T) {
	idx := New()
	idx.IndexFile(uri.URI("file:///a.cfc"), `component {
	function getUser() {}
}`)

	defs := idx.Lookup("getUser")
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	// Lookup is case-insensitive.
	if len(idx.Lookup("GETUSER")) != 1 {
		t.Error("expected case-insensitive lookup to find the function")
	}

	if len(idx.Lookup("missing")) != 0 {
		t.Error("expected no defs for unknown name")
	}
}

func TestIndexFile_ReindexReplacesNotDuplicates(t *testing.T) {
	idx := New()
	fileURI := uri.URI("file:///a.cfc")

	idx.IndexFile(fileURI, `component { function getUser() {} }`)
	idx.IndexFile(fileURI, `component { function getUser() {} function getOrder() {} }`)

	if got := len(idx.Lookup("getUser")); got != 1 {
		t.Errorf("expected re-indexing to replace, not duplicate: got %d getUser defs, want 1", got)
	}

	if got := len(idx.FunctionsForFile(fileURI)); got != 2 {
		t.Errorf("expected 2 functions for file after reindex, got %d", got)
	}
}

func TestIndexFile_MultipleFilesIsolated(t *testing.T) {
	idx := New()
	idx.IndexFile(uri.URI("file:///a.cfc"), `component { function foo() {} }`)
	idx.IndexFile(uri.URI("file:///b.cfc"), `component { function bar() {} }`)

	if got := len(idx.FunctionsForFile(uri.URI("file:///a.cfc"))); got != 1 {
		t.Errorf("expected 1 function in a.cfc, got %d", got)
	}

	if got := len(idx.FunctionsForFile(uri.URI("file:///b.cfc"))); got != 1 {
		t.Errorf("expected 1 function in b.cfc, got %d", got)
	}

	// AllFunctions sees both files.
	if got := len(idx.AllFunctions()); got != 2 {
		t.Errorf("expected 2 total functions, got %d", got)
	}
}

func TestShiftLines_OnlyAffectsMatchingFileAndLinesAfter(t *testing.T) {
	idx := New()

	target := uri.URI("file:///target.cfc")
	other := uri.URI("file:///other.cfc")

	funcs := []parser.FunctionDef{
		{Name: "before", URI: target, Line: 5},
		{Name: "after", URI: target, Line: 15},
		{Name: "exact", URI: target, Line: 10},
	}
	refs := []parser.ComponentRef{
		{Variable: "beforeRef", URI: target, Line: 5},
		{Variable: "afterRef", URI: target, Line: 15},
	}

	idx.IndexFileFromResult(target, funcs, refs)
	idx.IndexFileFromResult(other, []parser.FunctionDef{{Name: "otherFunc", URI: other, Line: 15}}, nil)

	idx.ShiftLines(target, 10, 3)

	get := func(name string) uint32 {
		defs := idx.Lookup(name)
		if len(defs) == 0 {
			t.Fatalf("expected a def named %q", name)
		}

		return defs[0].Line
	}

	if got := get("before"); got != 5 {
		t.Errorf("line at or before afterLine=10 should be untouched: got %d, want 5", got)
	}

	if got := get("exact"); got != 10 {
		t.Errorf("line exactly equal to afterLine should be untouched (only Line > afterLine shifts): got %d, want 10", got)
	}

	if got := get("after"); got != 18 {
		t.Errorf("line after afterLine should shift by delta: got %d, want 18", got)
	}

	if got := get("otherFunc"); got != 15 {
		t.Errorf("ShiftLines must not affect a different file: got %d, want 15 (unshifted)", got)
	}

	beforeRefs := idx.LookupComponentRef("beforeRef")
	if len(beforeRefs) != 1 || beforeRefs[0].Line != 5 {
		t.Errorf("beforeRef line should be untouched, got %+v", beforeRefs)
	}

	afterRefs := idx.LookupComponentRef("afterRef")
	if len(afterRefs) != 1 || afterRefs[0].Line != 18 {
		t.Errorf("afterRef line should shift to 18, got %+v", afterRefs)
	}
}

func TestRemoveFilesUnder_OnlyRemovesMatchingPrefix(t *testing.T) {
	idx := New()
	idx.IndexFileFromResult(uri.URI("file:///proj/a.cfc"),
		[]parser.FunctionDef{{Name: "fooA", URI: uri.URI("file:///proj/a.cfc"), Line: 1}}, nil)
	idx.IndexFileFromResult(uri.URI("file:///proj/sub/b.cfc"),
		[]parser.FunctionDef{{Name: "fooB", URI: uri.URI("file:///proj/sub/b.cfc"), Line: 1}}, nil)
	idx.IndexFileFromResult(uri.URI("file:///other/c.cfc"),
		[]parser.FunctionDef{{Name: "fooC", URI: uri.URI("file:///other/c.cfc"), Line: 1}}, nil)

	idx.RemoveFilesUnder("file:///proj/")

	if len(idx.Lookup("fooA")) != 0 {
		t.Error("expected fooA (under removed prefix) to be gone")
	}

	if len(idx.Lookup("fooB")) != 0 {
		t.Error("expected fooB (under removed prefix, nested) to be gone")
	}

	if len(idx.Lookup("fooC")) != 1 {
		t.Error("expected fooC (outside removed prefix) to remain")
	}
}

func TestLookupComponentRefInFile_PicksClosestPrecedingLine(t *testing.T) {
	idx := New()
	fileURI := uri.URI("file:///a.cfc")
	otherURI := uri.URI("file:///b.cfc")

	idx.SetFuncRefs(fileURI, "1:30", []parser.ComponentRef{
		{Variable: "svc", Component: "early", URI: fileURI, Line: 5},
		{Variable: "svc", Component: "late", URI: fileURI, Line: 20},
		{Variable: "svc", Component: "wrongFile", URI: otherURI, Line: 8},
	})

	// Querying at line 10 should return the ref at line 5 (closest preceding), not line 20.
	ref := idx.LookupComponentRefInFile("svc", fileURI, 10)
	if ref == nil || ref.Component != "early" {
		t.Errorf("expected closest preceding ref 'early' at line <=10, got %+v", ref)
	}

	// Querying at line 25 should now pick up the line-20 ref.
	ref = idx.LookupComponentRefInFile("svc", fileURI, 25)
	if ref == nil || ref.Component != "late" {
		t.Errorf("expected 'late' ref at line <=25, got %+v", ref)
	}

	// Querying before any ref exists should return nil.
	if ref := idx.LookupComponentRefInFile("svc", fileURI, 3); ref != nil {
		t.Errorf("expected nil when no ref precedes the queried line, got %+v", ref)
	}

	// A ref in a different file must never be returned.
	if ref := idx.LookupComponentRefInFile("svc", otherURI, 10); ref == nil || ref.Component != "wrongFile" {
		t.Errorf("expected the other file's own ref, got %+v", ref)
	}
}

func TestFindFilesByBasename(t *testing.T) {
	idx := New()
	idx.IndexFileFromResult(uri.URI("file:///proj/models/User.cfc"),
		[]parser.FunctionDef{{Name: "init", URI: uri.URI("file:///proj/models/User.cfc"), Line: 1}}, nil)

	// A file with no functions at all still needs to be findable via the fallback path.
	idx.IndexFileFromResult(uri.URI("file:///proj/models/Empty.cfc"), nil, nil)

	paths := idx.FindFilesByBasename("user")
	if len(paths) != 1 || paths[0] != "/proj/models/User.cfc" {
		t.Errorf("expected case-insensitive basename match with real casing preserved, got %v", paths)
	}

	paths = idx.FindFilesByBasename("empty")
	if len(paths) != 1 {
		t.Errorf("expected fallback match for a file with no functions, got %v", paths)
	}

	if len(idx.FindFilesByBasename("nonexistent")) != 0 {
		t.Error("expected no matches for a name that isn't indexed")
	}
}

func TestIndex_BeansAndEntities(t *testing.T) {
	idx := New()
	idx.SetBeans(map[string]string{"userservice": "app.services.UserService"})

	if got := idx.LookupBean("UserService"); got != "app.services.UserService" {
		t.Errorf("expected case-insensitive bean lookup, got %q", got)
	}

	if got := idx.LookupBean("unknown"); got != "" {
		t.Errorf("expected empty string for unknown bean, got %q", got)
	}

	idx.SetEntity("User", uri.URI("file:///models/User.cfc"))

	if got := idx.LookupEntity("user"); got != uri.URI("file:///models/User.cfc") {
		t.Errorf("expected case-insensitive entity lookup, got %q", got)
	}
}

func TestIndex_ConcurrentAccess(_ *testing.T) {
	idx := New()

	done := make(chan struct{})

	go func() {
		for i := range 100 {
			idx.IndexFileFromResult(uri.URI("file:///a.cfc"),
				[]parser.FunctionDef{{Name: "foo", URI: uri.URI("file:///a.cfc"), Line: uint32(i)}}, nil)
		}

		done <- struct{}{}
	}()

	for range 100 {
		idx.Lookup("foo")
		idx.AllFunctions()
	}

	<-done
}

// A function's refs are indexed lazily, on the first hover or definition lookup
// that lands inside it, and re-indexed after every edit to that function. The
// plain append this replaced had no way to tell those apart, so each lookup
// left another copy behind and comprefs grew for as long as the session ran.
func TestSetFuncRefs_ReplacesRatherThanAccumulates(t *testing.T) {
	idx := New()
	fileURI := uri.URI("file:///svc.cfc")

	refs := []parser.ComponentRef{
		{Variable: "svc", Component: "models.User", URI: fileURI, Line: 7},
	}

	for range 5 {
		idx.SetFuncRefs(fileURI, "3:12", refs)
	}

	if got := len(idx.LookupComponentRef("svc")); got != 1 {
		t.Errorf("re-indexing one scope five times left %d refs, want 1", got)
	}

	if got := len(idx.RefsForFile(fileURI)); got != 1 {
		t.Errorf("file refs grew to %d, want 1", got)
	}

	// A second scope in the same file is additive, not a replacement.
	idx.SetFuncRefs(fileURI, "20:30", []parser.ComponentRef{
		{Variable: "dao", Component: "models.UserDAO", URI: fileURI, Line: 22},
	})

	if got := len(idx.RefsForFile(fileURI)); got != 2 {
		t.Errorf("second scope gave %d file refs, want 2", got)
	}

	// Replacing a scope drops the refs it contributed and nothing else.
	idx.SetFuncRefs(fileURI, "3:12", []parser.ComponentRef{
		{Variable: "svc", Component: "models.Account", URI: fileURI, Line: 7},
	})

	got := idx.LookupComponentRef("svc")
	if len(got) != 1 || got[0].Component != "models.Account" {
		t.Errorf("expected the scope's single ref to be replaced, got %+v", got)
	}

	if n := len(idx.LookupComponentRef("dao")); n != 1 {
		t.Errorf("replacing one scope disturbed another: dao has %d refs, want 1", n)
	}
}

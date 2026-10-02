package resolve

import (
	"path/filepath"
	"testing"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// A cached return must not outlive the index it was read from: an edit to
// the callee, indexed and not saved, changes what the caller returns.
func TestReturnCacheFollowsTheIndex(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"A.cfc": `component {function a(){}}`,
		"B.cfc": `component {function b(){}}`,
		"G.cfc": `component {function make(){ return new A(); }}`,
		"F.cfc": `component {variables.g = new G(); function run(){ var x = variables.g.make(); return x; }}`,
	})

	r := &Resolver{}
	_ = reasonsWith(t, r, dir, "F.cfc")

	run := r.ResolveFunc("F", "run", dir)
	if run == nil {
		t.Fatal("run not found")
	}

	want := func(name string) {
		t.Helper()

		if got := r.ReturnComponentOf(run); r.ComponentPath(got, dir) != filepath.Join(dir, name+".cfc") {
			t.Fatalf("run returns %q, want %s", got, name)
		}
	}

	want("A")
	want("A")

	r.Index.IndexFile(cfpath.ToURI(filepath.Join(dir, "G.cfc")), `component {function make(){ return new B(); }}`)
	want("B")
}

// Within one generation the answer is served from the cache: a change the
// index never saw is not noticed, which is what makes the second ask cheap.
func TestReturnCacheServesRepeatedQuestions(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"A.cfc": `component {function a(){}}`,
		"F.cfc": `component {function run(){ return new A(); }}`,
	})

	r := &Resolver{}
	_ = reasonsWith(t, r, dir, "F.cfc")

	run := r.ResolveFunc("F", "run", dir)
	first := r.ReturnComponentOf(run)

	run.ReturnComponent = "nowhere.Else"
	if got := r.ReturnComponentOf(run); got != first {
		t.Fatalf("recomputed within a generation: %q, then %q", first, got)
	}
}

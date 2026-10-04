package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestReviewAppLessCallersShareView(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a/Helper.cfc": `component {function a(){}}`, "b/Helper.cfc": `component {function b(){}}`})
	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}

	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if r.forCaller(a) != r.forCaller(b) {
		t.Fatal("app-less callers allocated distinct mapping views")
	}

	if r.forCaller(a).ResolveFunc("Helper", "a", a) == nil || r.forCaller(b).ResolveFunc("Helper", "b", b) == nil {
		t.Fatal("sharing a view lost physical relative lookup")
	}
}

func TestBatchCallerIndexReusesBytesFromWorkspaceIndexing(t *testing.T) {
	cfs := newCountingFS()
	r := &Resolver{FS: cfs, InferArgsFiles: []string{"a.cfc", "b.cfc"}}

	r.IndexCallerFile("a.cfc", `component { function run(){ save(user); save(user); } }`)
	r.IndexCallerFile("b.cfc", `component { function run(){ other(user); } }`)

	files := r.callerFiles("save")
	if len(files) != 1 || files[0] != "a.cfc" {
		t.Fatalf("save callers = %v, want [a.cfc]", files)
	}

	if cfs.count("a.cfc") != 0 || cfs.count("b.cfc") != 0 {
		t.Fatal("building the caller index reread files already indexed")
	}
}

func TestBatchHandlerParseCacheDoesNotThrashAtEditorLimit(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 513)

	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("Handler%03d.cfc", i))
		if err := os.WriteFile(paths[i], []byte(`component { function run(){ work(); } }`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfs := newCountingFS()
	r := &Resolver{FS: cfs, Index: index.New(), InferArgsFiles: paths}

	for range 2 {
		for _, path := range paths {
			if r.handlerParse(path) == nil {
				t.Fatalf("failed to parse %s", path)
			}
		}
	}

	for _, path := range paths {
		if reads := cfs.count(path); reads != 1 {
			t.Fatalf("%s read %d times, want once", path, reads)
		}
	}
}

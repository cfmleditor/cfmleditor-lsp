package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func BenchmarkCallerNames(b *testing.B) {
	var content strings.Builder
	for i := range 10_000 {
		fmt.Fprintf(&content, "service.method%d (value);\n", i%100)
	}

	source := content.String()

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for b.Loop() {
		if names := callerNames(source); len(names) != 100 {
			b.Fatalf("got %d names, want 100", len(names))
		}
	}
}

func TestCallerNamesMatchesCallShapedIdentifiers(t *testing.T) {
	got := callerNames("save(); svc.Mixed_Name \t\n( value ); _private(1); 42(2); ignored.value;")

	for _, name := range []string{"save", "mixed_name", "_private", "42"} {
		if !got[name] {
			t.Errorf("caller names %v lack %q", got, name)
		}
	}

	if got["value"] || len(got) != 4 {
		t.Errorf("caller names = %v, want exactly four call-shaped identifiers", got)
	}
}

func TestCallerNamesDoesNotAllocatePerOccurrence(t *testing.T) {
	var content strings.Builder
	for range 2_000 {
		content.WriteString("service.sameName(value);\n")
	}

	source := content.String()
	allocs := testing.AllocsPerRun(10, func() {
		if names := callerNames(source); len(names) != 1 {
			t.Fatalf("got %d names, want one", len(names))
		}
	})

	if allocs > 20 {
		t.Errorf("2,000 occurrences allocated %.0f times; caller scanning is allocating per match", allocs)
	}
}

func TestBatchCallerIndexReusesBytesFromWorkspaceIndexing(t *testing.T) {
	cfs := newCountingFS()
	r := &Resolver{FS: cfs, InferArgsFiles: []string{"a.cfc", "b.cfc"}}

	r.IndexCallerFile("b.cfc", `component { function run(){ other(user); } }`)
	r.IndexCallerFile("a.cfc", `component { function run(){ save(user); save(user); } }`)
	r.IndexCallerFile("z.cfc", `component { function run(){ save(user); } }`)
	r.IndexCallerFile("a.cfc", `component { function run(){ save(user); } }`)

	files := r.callerFiles("save")
	if !slices.Equal(files, []string{"a.cfc", "z.cfc"}) {
		t.Fatalf("save callers = %v, want stable unique files [a.cfc z.cfc]", files)
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

package index

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// The index must not retain the source it was built from.
//
// Every string a parse produces is a slice of the file's content, and a Go
// substring keeps the whole backing array alive. The index outlives the parse
// results, so one retained function name holds its entire file — and an index of
// a workspace holds the workspace. On a real one that was 285MB retained for
// 322MB of source, against a few megabytes of actual data.
//
// Nothing about the index's answers is wrong when this happens, which is why it
// needs a test of its own: the defect is entirely in what stays on the heap.
func TestIndexDoesNotRetainTheSource(t *testing.T) {
	const (
		files   = 200
		padding = 64 * 1024
	)

	heap := func() uint64 {
		runtime.GC()
		runtime.GC()

		var m runtime.MemStats

		runtime.ReadMemStats(&m)

		return m.HeapAlloc
	}

	base := heap()
	idx := New()

	for i := range files {
		// A small amount of declaration in a large file. If the index keeps the
		// file, the padding comes with it.
		content := fmt.Sprintf("component {\n\tfunction getThing%d() {}\n}\n", i) +
			"// " + strings.Repeat("x", padding) + "\n"

		u := uri.URI(fmt.Sprintf("file:///tmp/retention/f%d.cfc", i))
		pr := parser.Parse(u, content)
		idx.IndexFileFromResult(u, pr.Funcs, pr.ComponentRefs)
	}

	after := heap()

	runtime.KeepAlive(idx)

	source := uint64(files) * padding
	retained := after - base

	t.Logf("%d files, %.1f MB of source, index retains %.1f MB (%.2fx)",
		files, float64(source)/1e6, float64(retained)/1e6, float64(retained)/float64(source))

	// A tenth of the source is generous: the declarations themselves are a few
	// hundred bytes per file. Keeping the files would be about 1.0x.
	if retained > source/10 {
		t.Errorf("index retains %.1f MB for %.1f MB of source (%.2fx); it is holding the files it was built from",
			float64(retained)/1e6, float64(source)/1e6, float64(retained)/float64(source))
	}
}

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
//
// Every door into the index is checked, not only IndexFileFromResult. That one
// was fixed first and was the only one tested, and three others went on storing
// parsed strings: IndexFile (the resolver's lazy indexing and the CLI scans),
// SetThisVars (called for every file on the workspace scan, so any component
// with a this-scoped variable kept its whole source) and SetFuncRefs. Each
// retained 1.1x the source it was given.
func TestIndexDoesNotRetainTheSource(t *testing.T) {
	const (
		files   = 200
		padding = 64 * 1024
	)

	// A small amount of declaration in a large file. If the index keeps the
	// file, the padding comes with it. Each kind of string a door stores is
	// present — a function, a component ref inside it, a this-scoped variable,
	// an include — so a door that forgets any one of them is caught.
	source := func(i int) string {
		return fmt.Sprintf("component {\n\tthis.thing%d = 1;\n\tinclude \"x%d.cfm\";\n"+
			"\tfunction getThing%d() { var svc = new models.Svc%d(); }\n}\n", i, i, i, i) +
			"// " + strings.Repeat("x", padding) + "\n"
	}

	doors := []struct {
		name  string
		index func(idx *Index, u uri.URI, content string)
	}{
		{"IndexFileFromResult", func(idx *Index, u uri.URI, content string) {
			pr := parser.Parse(u, content)
			idx.IndexFileFromResult(u, pr.Funcs, pr.ComponentRefs)
		}},
		{"IndexFile", func(idx *Index, u uri.URI, content string) {
			idx.IndexFile(u, content)
		}},
		{"SetThisVars", func(idx *Index, u uri.URI, content string) {
			idx.SetThisVars(u, parser.Parse(u, content).ThisVars())
		}},
		{"SetIncludes", func(idx *Index, u uri.URI, content string) {
			idx.SetIncludes(u, parser.ExtractIncludes(content))
		}},
		{"SetExtends", func(idx *Index, u uri.URI, content string) {
			idx.SetExtends(u, content[len("component"):len("component")+3])
		}},
		{"SetFuncRefs", func(idx *Index, u uri.URI, content string) {
			pr := parser.Parse(u, content)
			sc := pr.Scopes[0]
			idx.SetFuncRefs(u, "scope", pr.FuncComponentRefs(sc.Start, sc.End))
		}},
	}

	heap := func() uint64 {
		runtime.GC()
		runtime.GC()

		var m runtime.MemStats

		runtime.ReadMemStats(&m)

		return m.HeapAlloc
	}

	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			base := heap()
			idx := New()

			for i := range files {
				door.index(idx, uri.URI(fmt.Sprintf("file:///tmp/retention/f%d.cfc", i)), source(i))
			}

			after := heap()

			runtime.KeepAlive(idx)

			total := uint64(files) * padding
			retained := after - min(after, base)

			t.Logf("%d files, %.1f MB of source, index retains %.1f MB (%.2fx)",
				files, float64(total)/1e6, float64(retained)/1e6, float64(retained)/float64(total))

			// A tenth of the source is generous: the declarations themselves
			// are a few hundred bytes per file. Keeping the files would be
			// about 1.0x.
			if retained > total/10 {
				t.Errorf("index retains %.1f MB for %.1f MB of source (%.2fx); it is holding the files it was built from",
					float64(retained)/1e6, float64(total)/1e6, float64(retained)/float64(total))
			}
		})
	}
}

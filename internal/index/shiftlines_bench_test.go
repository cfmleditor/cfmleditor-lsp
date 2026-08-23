package index

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// ShiftLines runs on every line-count-changing keystroke, under the index write
// lock, so its cost is paid by every other session sharing the daemon.
//
// The sharedNames variants matter because funcs is keyed by lowercased name: a
// workspace where every component defines the same method names puts one entry
// per file in a single bucket, which is the case that still scales.
func benchIndex(files, perFile int, sharedNames bool) (*Index, uri.URI) {
	idx := New()

	var first uri.URI

	for f := range files {
		u := uri.File(fmt.Sprintf("/ws/pkg%d/File%d.cfc", f%20, f))
		if f == 0 {
			first = u
		}

		funcs := make([]parser.FunctionDef, 0, perFile)
		refs := make([]parser.ComponentRef, 0, perFile)

		for i := range perFile {
			fn := fmt.Sprintf("method%d", i)
			vn := fmt.Sprintf("svc%d", i)

			if !sharedNames {
				fn = fmt.Sprintf("f%d_method%d", f, i)
				vn = fmt.Sprintf("f%d_svc%d", f, i)
			}

			funcs = append(funcs, parser.FunctionDef{Name: fn, URI: u, Line: uint32(10 + i*5)})
			refs = append(refs, parser.ComponentRef{
				Variable: vn, Component: "models.User", URI: u, Line: uint32(12 + i*5),
			})
		}

		idx.IndexFileFromResult(u, funcs, refs)
	}

	return idx, first
}

func BenchmarkShiftLines(b *testing.B) {
	for _, sz := range []struct {
		name           string
		files, perFile int
		shared         bool
	}{
		{"5000files_sharedNames", 5000, 8, true},
		{"5000files_distinctNames", 5000, 8, false},
		{"1000files_distinctNames", 1000, 8, false},
	} {
		b.Run(sz.name, func(b *testing.B) {
			idx, u := benchIndex(sz.files, sz.perFile, sz.shared)

			b.ResetTimer()

			for i := range b.N {
				d := 1
				if i%2 == 1 {
					d = -1
				}

				idx.ShiftLines(u, 5, d)
			}
		})
	}
}

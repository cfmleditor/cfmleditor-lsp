package index

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Indexing one file is the operation a workspace scan performs once per file
// and a watched-file change performs on every save, so a cost that grows with
// the size of the index makes the startup walk quadratic.
//
// The sharedNames variants are the ones that still scale: funcs is keyed by
// lowercased name, so a workspace where every component defines init puts one
// entry per file in a single bucket.
func benchFileEntries(u uri.URI, f, perFile int, sharedNames bool) ([]parser.FunctionDef, []parser.ComponentRef) {
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

	return funcs, refs
}

func BenchmarkIndexFileFromResult(b *testing.B) {
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
			funcs, refs := benchFileEntries(u, 0, sz.perFile, sz.shared)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				idx.IndexFileFromResult(u, funcs, refs)
			}
		})
	}
}

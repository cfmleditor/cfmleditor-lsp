package index

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// LookupComponentRefInFile answers "what component is this variable, here",
// which hover, definition and completion all ask on the keystroke. The variable
// names that matter are the ordinary ones — svc, dao, qry — so the bucket it
// searches holds one entry per file in the workspace, not one entry.
func BenchmarkLookupComponentRefInFile(b *testing.B) {
	for _, files := range []int{1000, 5000} {
		b.Run(fmt.Sprintf("%dfiles", files), func(b *testing.B) {
			idx, u := benchIndex(files, 8, true)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if got := idx.LookupComponentRefInFile("svc3", u, 100); got == nil {
					b.Fatal("no ref found")
				}
			}
		})
	}
}

// The same lookup where the file holds many refs, which is the cost the
// per-file view pays in exchange.
func BenchmarkLookupComponentRefInFileWideFile(b *testing.B) {
	idx := New()
	u := uri.File("/ws/pkg/Wide.cfc")

	const perFile = 200

	refs := make([]parser.ComponentRef, 0, perFile)
	for i := range perFile {
		refs = append(refs, parser.ComponentRef{
			Variable: fmt.Sprintf("svc%d", i), Component: "models.User", URI: u, Line: uint32(i),
		})
	}

	idx.IndexFileFromResult(u, nil, refs)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if got := idx.LookupComponentRefInFile("svc199", u, 1000); got == nil {
			b.Fatal("no ref found")
		}
	}
}

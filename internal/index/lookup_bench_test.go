package index

import (
	"fmt"
	"strings"
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

// The read side of the snapshot trade. Lookup now copies the bucket it returns,
// so a name every component in the workspace declares — init, and little else —
// costs a slice of one pointer per file. It is the case where the caller is
// about to walk all of them anyway.
func BenchmarkLookup(b *testing.B) {
	for _, sz := range []struct {
		name   string
		files  int
		shared bool
		key    string
	}{
		{"sharedName_5000files", 5000, true, "method3"},
		{"distinctName_5000files", 5000, false, "f0_method3"},
		{"miss", 5000, false, "nosuchmethod"},
	} {
		b.Run(sz.name, func(b *testing.B) {
			idx, _ := benchIndex(sz.files, 8, sz.shared)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				idx.Lookup(sz.key)
			}
		})
	}
}

// LookupPreferred is what the keystroke paths use instead of Lookup, so it must
// not copy the bucket. The sharedName case is the one that matters: a name
// every component in the workspace declares.
func BenchmarkLookupPreferred(b *testing.B) {
	for _, sz := range []struct {
		name   string
		shared bool
		key    string
	}{
		{"sharedName_inFile", true, "method3"},
		{"sharedName_notInFile", true, "method3"},
		{"distinctName_inFile", false, "f0_method3"},
	} {
		b.Run(sz.name, func(b *testing.B) {
			idx, u := benchIndex(5000, 8, sz.shared)

			// The miss case: a document the index has never seen, so the
			// preference cannot be satisfied and the fallback is taken.
			if strings.HasSuffix(sz.name, "notInFile") {
				u = uri.File("/ws/elsewhere/Other.cfc")
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				idx.LookupPreferred(sz.key, u)
			}
		})
	}
}

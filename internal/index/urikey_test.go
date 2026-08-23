package index

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// The codebase builds a file URI two ways: indexing goes through uri.File,
// which percent-escapes, while several lookups concatenate "file://" and a
// path. For any workspace whose path contains a space — "/My Documents", the
// default on two of three platforms — those produce different strings, so the
// lookup missed every entry and this-scope completions silently stopped
// appearing. Both forms must reach the same entry.
func TestLookupsAgreeAcrossURIEncodings(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"space in a directory", "/My Documents/app/User.cfc"},
		{"non-ascii", "/tmp/café/User.cfc"},
		{"plain", "/tmp/plain/User.cfc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := uri.File(tt.path)           // how workspace indexing stores it
			asked := uri.URI("file://" + tt.path) // how several lookups ask for it

			idx := New()
			idx.IndexFileFromResult(stored,
				[]parser.FunctionDef{{Name: "f", URI: stored, Line: 1}},
				[]parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: stored, Line: 2}},
			)
			idx.SetThisVars(stored, []string{"greeting"})

			if got := idx.FunctionsForFile(asked); len(got) != 1 {
				t.Errorf("FunctionsForFile(%q) found %d, want 1", asked, len(got))
			}

			if got := idx.RefsForFile(asked); len(got) != 1 {
				t.Errorf("RefsForFile(%q) found %d, want 1", asked, len(got))
			}

			if got := idx.ThisVarsForFile(asked); len(got) != 1 {
				t.Errorf("ThisVarsForFile(%q) found %d, want 1", asked, len(got))
			}

			// And the reverse: stored unescaped, asked escaped.
			idx2 := New()
			idx2.SetThisVars(asked, []string{"greeting"})

			if got := idx2.ThisVarsForFile(stored); len(got) != 1 {
				t.Errorf("reverse direction: ThisVarsForFile(%q) found %d, want 1", stored, len(got))
			}
		})
	}
}

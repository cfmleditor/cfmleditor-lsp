package server

import (
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"
)

// The editor re-indexes a document on every edit that changes its signatures,
// and the re-index forgets the file's extends and delegates. The resolver's
// answer to a forgotten one is to read the file from disk and parse it: the
// saved text rather than the buffer, and on tassweb's 65,000-line kiosk.cfc
// twice per keystroke. The re-index must record both from the document's own
// parse.
func TestReindexKeepsTheBuffersExtendsAndDelegates(t *testing.T) {
	s := newTestServer()
	u := uri.File(filepath.Join(t.TempDir(), "Widget.cfc"))
	pr := s.parseContent(u, `component extends="models.Base" { property name="memory" inject delegate; }`)

	s.reindexFromParseResult(u, pr)

	if ext, ok := s.index.ExtendsForFile(u); !ok || ext != "models.Base" {
		t.Errorf("ExtendsForFile = %q, %v; want models.Base, true", ext, ok)
	}

	if ds, ok := s.index.DelegatesForFile(u); !ok || len(ds) != 1 {
		t.Errorf("DelegatesForFile = %v, %v; want one delegation", ds, ok)
	}
}

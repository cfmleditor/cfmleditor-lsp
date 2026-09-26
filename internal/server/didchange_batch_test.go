package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Reverting a reformat in Zed sends one didChange with an edit for every line
// the formatter touched, in document order. Applied one at a time each edit
// cost the whole document, so 6,000 of them against a 110 KB file took 425ms
// under the document's lock, and every request on the file waited behind it.
func TestDidChangeAppliesALargeBatchInOnePass(t *testing.T) {
	var b strings.Builder

	b.WriteString("component {\n")

	for i := range 1000 {
		fmt.Fprintf(&b, "public void function f%d(required string id) {\nvar x = foo.bar(id=arguments.id);\nif (x) {\nreturn baz(x);\n}\n}\n", i)
	}

	b.WriteString("}\n")

	content := b.String()
	lines := strings.Split(content, "\n")

	dir := t.TempDir()
	path := filepath.Join(dir, "a.cfc")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	docURI := uri.File(path)

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: content},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	// Indent every line inside the component.
	var changes []protocol.TextDocumentContentChangeEvent

	want := make([]string, len(lines))
	copy(want, lines)

	for l := 1; l < len(lines)-2; l++ {
		changes = append(changes, &protocol.TextDocumentContentChangePartial{
			Range: protocol.Range{Start: protocol.Position{Line: uint32(l)}, End: protocol.Position{Line: uint32(l)}},
			Text:  "    ",
		})
		want[l] = "    " + lines[l]
	}

	change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: changes,
	})

	start := time.Now()

	if _, err := srv.handleDidChange(context.Background(), change); err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)

	got, _ := srv.getDocument(docURI)
	if got != strings.Join(want, "\n") {
		t.Fatal("document does not match the edits applied in order")
	}

	// Timing-sensitive, so skipped under -short like TestParsePerformance: the
	// race detector's instrumentation alone takes this near the bound. About
	// 5ms without it; 435ms before the batch was streamed.
	if !testing.Short() && elapsed > 100*time.Millisecond {
		t.Errorf("%d changes against %d bytes took %v", len(changes), len(content), elapsed)
	}
}

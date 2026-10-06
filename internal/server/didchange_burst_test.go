package server

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// More than five changes inside 200ms is a burst, and a burst used to apply
// the text alone and leave a reparse of the whole file for the next request —
// even when every change was inside a function, which an incremental edit
// handles in a few milliseconds. On a 65,000-line component a held-down key
// made every sixth keystroke wait ~220ms for that reparse. Edits inside a
// function now stay incremental through a burst; one outside still defers.
func TestABurstOfEditsInsideAFunctionStaysIncremental(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///burst.cfc")
	text := "component {\n\tfunction f() {\n\t\tvar a = 1;\n\t}\n}\n"

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: text},
	})
	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	edit := func(line, char uint32, insert string) {
		t.Helper()

		at := protocol.Position{Line: line, Character: char}

		change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangePartial{Range: protocol.Range{Start: at, End: at}, Text: insert},
			},
		})
		if _, err := srv.handleDidChange(context.Background(), change); err != nil {
			t.Fatal(err)
		}
	}

	line := "\t\tvar a = 1;"

	for i := range 10 {
		edit(2, uint32(len(line)), "x")
		line += "x"

		if srv.reparseIsPending(docURI) {
			t.Fatalf("edit %d inside the function deferred a reparse of the whole file", i+1)
		}

		srv.mu.RLock()
		pr := srv.parseResults[docURI]
		srv.mu.RUnlock()

		doc, _ := srv.getDocument(docURI)
		if pr.Content != doc {
			t.Fatalf("edit %d: the parse is not the document", i+1)
		}
	}

	if want := "component {\n\tfunction f() {\n" + line + "\n\t}\n}\n"; func() string {
		d, _ := srv.getDocument(docURI)

		return d
	}() != want {
		t.Fatalf("document after the burst is not the edits applied")
	}

	// The same burst, now at component level, still defers.
	edit(0, uint32(len("component {")), " ")

	if !srv.reparseIsPending(docURI) {
		t.Error("an edit outside any function did not defer its reparse")
	}

	srv.mu.Lock()
	for _, timer := range srv.reindexTimers {
		timer.Stop()
	}

	for _, timer := range srv.cacheTimers {
		timer.Stop()
	}
	srv.mu.Unlock()

	if !strings.Contains(func() string {
		d, _ := srv.getDocument(docURI)

		return d
	}(), "component { \n") {
		t.Error("the component-level edit was not applied")
	}
}

package server

import (
	"context"
	"testing"
	"unsafe"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// An incremental edit must leave the server's document text and the open
// document's ParseResult holding one string, not two equal ones.
//
// The two were built independently — the server applied the edit to its copy
// and ParseResult.ApplyEdit applied it again to Content — so every keystroke
// allocated the whole document twice and an open document was held in memory
// twice for as long as it stayed open. Nothing about any answer changes when
// that happens, which is why the test asks where the bytes live rather than
// what they say.
func TestDidChangeSharesTheDocumentWithTheParseResult(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///shared.cfc")

	// One edit inside a function body and one outside any, since the two take
	// different paths through ApplyEdit.
	edits := []protocol.Position{{Line: 2, Character: 0}, {Line: 0, Character: 0}}

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  docURI,
			Text: "component {\n\tpublic void function go() {\n\t\tvar x = 1;\n\t}\n}\n",
		},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	for _, at := range edits {
		change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangePartial{
					Range: protocol.Range{Start: at, End: at},
					Text:  "// note\n",
				},
			},
		})

		if _, err := srv.handleDidChange(context.Background(), change); err != nil {
			t.Fatal(err)
		}

		doc, ok := srv.getDocument(docURI)
		if !ok {
			t.Fatal("document not open")
		}

		srv.mu.RLock()
		pr := srv.parseResults[docURI]
		srv.mu.RUnlock()

		if pr.Content != doc {
			t.Fatalf("edit at %v: ParseResult and document disagree:\n%q\n%q", at, pr.Content, doc)
		}

		if unsafe.StringData(pr.Content) != unsafe.StringData(doc) {
			t.Errorf("edit at %v: ParseResult holds its own copy of the document; the edit was applied twice", at)
		}
	}
}

package server

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// An edit's character offset counts UTF-16 code units, and the server applied
// it as a byte count. On a line with a non-ASCII character before the edit the
// text went in early, and the server's copy of the document disagreed with the
// editor's from then on. Both didChange paths are covered: a keystroke, and
// the burst path that only applies the text.
func TestDidChangeCountsUTF16Units(t *testing.T) {
	for _, burst := range []bool{false, true} {
		srv := newTestServer()
		docURI := uri.URI("file:///utf16.cfc")

		open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:  docURI,
				Text: "component {\n\tfunction f() {\n\t\tvar s = \"café 😀\";\n\t}\n}\n",
			},
		})

		if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
			t.Fatal(err)
		}

		if burst {
			// More than five changes inside the window takes the burst path.
			srv.mu.Lock()
			srv.changeCount[docURI] = 10
			srv.mu.Unlock()
		}

		// `\t\tvar s = "café 😀";` — the closing quote is at UTF-16 column 18:
		// two tabs, `var s = "` (9), `café` (4), a space, and 😀 (2).
		at := protocol.Position{Line: 2, Character: 18}
		change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangePartial{Range: protocol.Range{Start: at, End: at}, Text: "!"},
			},
		})

		if _, err := srv.handleDidChange(context.Background(), change); err != nil {
			t.Fatal(err)
		}

		want := "component {\n\tfunction f() {\n\t\tvar s = \"café 😀!\";\n\t}\n}\n"

		if got, _ := srv.getDocument(docURI); got != want {
			t.Errorf("burst=%v: document after the edit:\n%q\nwant\n%q", burst, got, want)
		}

		if !burst {
			srv.mu.RLock()
			pr := srv.parseResults[docURI]
			srv.mu.RUnlock()

			if pr.Content != want {
				t.Errorf("parse result after the edit:\n%q\nwant\n%q", pr.Content, want)
			}
		}

		srv.mu.Lock()
		for _, timer := range srv.reindexTimers {
			timer.Stop()
		}

		for _, timer := range srv.cacheTimers {
			timer.Stop()
		}
		srv.mu.Unlock()
	}
}

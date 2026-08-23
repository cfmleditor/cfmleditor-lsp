package server

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// The "convert to cfelseif" completion replaces from the "<" that opened the
// tag to the cursor. That "<" is not necessarily on the cursor's line —
// `<cfelse` with the cursor on the next line is an ordinary mid-edit state —
// and deriving its column by subtracting a length from the cursor's underflowed
// uint32, producing a range starting at character 4294967288, after its own end.
func TestCfelseCompletionRangeIsWellFormed(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		line     uint32
		char     uint32
		wantLine uint32
		wantChar uint32
	}{
		{
			name:     "tag and cursor on one line",
			content:  "<cfif a>\n<cfelse ",
			line:     1,
			char:     8,
			wantLine: 1,
			wantChar: 0,
		},
		{
			name:     "tag opens on an earlier line",
			content:  "<cfif a>\n<cfelse\n  x",
			line:     2,
			char:     2,
			wantLine: 1,
			wantChar: 0,
		},
		{
			name:     "tag indented on an earlier line",
			content:  "<cfif a>\n    <cfelse\n      y",
			line:     2,
			char:     6,
			wantLine: 1,
			wantChar: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer()
			docURI := uri.URI("file:///cfelse.cfm")
			srv.setDocument(docURI, tt.content)

			req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
					Position:     protocol.Position{Line: tt.line, Character: tt.char},
				},
			})

			res, err := srv.handleCompletion(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			list, ok := res.(*protocol.CompletionList)
			if !ok {
				t.Fatalf("unexpected result type %T", res)
			}

			var found bool

			for _, it := range list.Items {
				if it.Label != "if" {
					continue
				}

				found = true

				te := textEdit(t, it.TextEdit)
				if te == nil {
					t.Fatal("expected a text edit on the cfelseif completion")
				}

				start, end := te.Range.Start, te.Range.End
				if start.Line != tt.wantLine || start.Character != tt.wantChar {
					t.Errorf("range start = %d:%d, want %d:%d", start.Line, start.Character, tt.wantLine, tt.wantChar)
				}

				if start.Line > end.Line || (start.Line == end.Line && start.Character > end.Character) {
					t.Errorf("range start %d:%d is after end %d:%d", start.Line, start.Character, end.Line, end.Character)
				}
			}

			if !found {
				t.Error("no cfelseif completion offered")
			}
		})
	}
}

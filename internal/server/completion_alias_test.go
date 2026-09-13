package server

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// The handler hands the cached completion items over rather than copying them,
// which is only sound while nothing downstream writes to what it was given.
// getBuiltinFuncItems is a package-level sync.Once list shared by every session
// in the process, so a write there outlives the request and reaches every other
// client.
//
// This failed before applySnippetPolicy stopped editing in place: with
// functionSnippets off, one request blanked the insert text of the shared list
// permanently. It passed unnoticed because the edit is idempotent and the
// setting is process-wide — which is exactly why it needs a test rather than a
// comment.
func TestCompletionDoesNotWriteToTheSharedBuiltinList(t *testing.T) {
	before := snapshotBuiltins(t)

	for _, tc := range []struct {
		name              string
		tagSnip, funcSnip bool
	}{
		{"snippets on", true, true},
		{"function snippets off", true, false},
		{"tag snippets off", false, true},
		{"both off", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer()
			s.TagSnippets = tc.tagSnip
			s.FunctionSnippets = tc.funcSnip

			docURI := uri.File("/ws/Doc.cfc")

			open, err := json.Marshal(protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI: docURI, LanguageID: "cfml", Version: 1,
					// A hash expression, which is the branch that assigns the
					// cached slice straight through.
					Text: "<cfoutput>#now()#</cfoutput>",
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			if _, err := s.handleDidOpen(context.Background(), open); err != nil {
				t.Fatal(err)
			}

			req, err := json.Marshal(protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
					Position:     protocol.Position{Line: 0, Character: 16},
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			res, err := s.handleCompletion(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			list, ok := res.(*protocol.CompletionList)
			if !ok || len(list.Items) == 0 {
				t.Fatalf("no completion items returned: %#v", res)
			}

			// The policy still has to be applied to what the client receives.
			if !tc.funcSnip {
				for _, it := range list.Items {
					if it.Kind == protocol.CompletionItemKindFunction &&
						it.InsertTextFormat == protocol.InsertTextFormatSnippet {
						t.Fatalf("function snippets are off but %q came back as a snippet", it.Label)
					}
				}
			}

			if after := snapshotBuiltins(t); after != before {
				t.Error("serving a completion rewrote the shared builtin list")
			}
		})
	}
}

// snapshotBuiltins renders the shared list so a change anywhere in it shows up.
func snapshotBuiltins(t *testing.T) string {
	t.Helper()

	b, err := json.Marshal(getBuiltinFuncItems())
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

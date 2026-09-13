package server

import (
	"context"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// When go-to-definition finds the name in several files, the editor lists them
// in the order they are returned. That order used to be the index's bucket
// order — the order parallel indexing goroutines finished in — so the same
// "3 definitions found" list could come back differently after a restart.
func TestDefinitionListsNearestFirstAndStably(t *testing.T) {
	asking := uri.File("/ws/app/models/Asking.cfc")

	files := []uri.URI{
		uri.File("/ws/other/deep/Deeper.cfc"),
		uri.File("/ws/app/models/Near.cfc"),
		uri.File("/ws/other/Far.cfc"),
		uri.File("/ws/app/Mid.cfc"),
	}

	want := []uri.URI{
		uri.File("/ws/app/models/Near.cfc"),
		uri.File("/ws/app/Mid.cfc"),
		uri.File("/ws/other/Far.cfc"),
		uri.File("/ws/other/deep/Deeper.cfc"),
	}

	// Every rotation of the indexing order must produce the same list.
	for start := range files {
		s := newTestServer()
		s.GlobalFunctionResolution = true

		for i := range files {
			u := files[(start+i)%len(files)]
			s.index.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
		}

		// A qualified call the resolver cannot place, which is the branch that
		// falls through to the global lookup and returns every match.
		content := "component {\n\tfunction f() {\n\t\tunknownThing.save();\n\t}\n}\n"
		open := mustJSON(t, protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: asking, LanguageID: "cfml", Version: 1, Text: content},
		})

		if _, err := s.handleDidOpen(context.Background(), open); err != nil {
			t.Fatal(err)
		}

		res, err := s.handleDefinition(context.Background(), mustJSON(t, protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: asking},
				Position:     protocol.Position{Line: 2, Character: 16},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}

		locs, ok := res.([]protocol.Location)
		if !ok {
			t.Fatalf("rotation %d: got %#v, want a list of locations", start, res)
		}

		if len(locs) != len(want) {
			t.Fatalf("rotation %d: %d locations, want %d", start, len(locs), len(want))
		}

		for i, w := range want {
			if locs[i].URI != w {
				t.Errorf("rotation %d: position %d is %s, want %s", start, i, locs[i].URI, w)
			}
		}
	}
}

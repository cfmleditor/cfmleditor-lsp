package server

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Function-scope refs are indexed lazily, keyed by the function they came from.
// The key used to be the function's line range, which moves the moment a line
// is inserted above it — so the next indexing wrote a new entry instead of
// replacing the old one. The refs accumulated exactly as they had before
// SetFuncRefs existed, and the stale entry won LookupComponentRefInFile's
// tie-break, so hover answered with the component the variable held *before*
// the edit.
func TestFuncScopeRefsSurviveLineShifts(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///scope.cfc")

	before := "component {\n" +
		"\tpublic void function Go() {\n" +
		"\t\tvar svc = new pkg.Foo();\n" +
		"\t\tsvc.Run();\n" +
		"\t}\n" +
		"}\n"

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: before},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	srv.ensureFuncRefsIndexed(docURI, 3)

	if got := len(srv.index.LookupComponentRef("svc")); got != 1 {
		t.Fatalf("first indexing produced %d refs, want 1", got)
	}

	// An incremental edit *inside* the function body: that takes didChange's
	// EditInFunc path, which shifts line numbers without reindexing the file —
	// so nothing else clears the previous entry and the key has to do the work.
	// A whole-document replace would reindex and mask this.
	change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangePartial{
				Range: protocol.Range{
					Start: protocol.Position{Line: 3, Character: 0},
					End:   protocol.Position{Line: 3, Character: 0},
				},
				Text: "\t\t// a note\n",
			},
		},
	})

	if _, err := srv.handleDidChange(context.Background(), change); err != nil {
		t.Fatal(err)
	}

	// Hover again inside the same function, whose range has now moved.
	srv.ensureFuncRefsIndexed(docURI, 3)

	refs := srv.index.LookupComponentRef("svc")
	if len(refs) != 1 {
		t.Errorf("re-indexing one function left %d refs, want 1: %+v", len(refs), refs)
	}
}

// A function whose last component ref is deleted must lose its indexed ref too.
// The old code skipped SetFuncRefs entirely when a function had none left, so
// the previous ref stayed until a full reindex.
func TestFuncScopeRefsClearWhenEmptied(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///cleared.cfc")

	withRef := "component {\n\tpublic void function Go() {\n\t\tvar svc = new pkg.Foo();\n\t\tsvc.Run();\n\t}\n}\n"

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: withRef},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	srv.ensureFuncRefsIndexed(docURI, 2)

	if got := len(srv.index.LookupComponentRef("svc")); got != 1 {
		t.Fatalf("expected the ref to be indexed, got %d", got)
	}

	withoutRef := "component {\n\tpublic void function Go() {\n\t\tvar svc = 1;\n\t\tsvc.Run();\n\t}\n}\n"

	change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: withoutRef},
		},
	})

	if _, err := srv.handleDidChange(context.Background(), change); err != nil {
		t.Fatal(err)
	}

	srv.ensureFuncRefsIndexed(docURI, 2)

	if got := srv.index.LookupComponentRef("svc"); len(got) != 0 {
		t.Errorf("stale ref survived after its assignment was removed: %+v", got)
	}
}

package server

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func openDoc(t *testing.T, srv *Server, docURI uri.URI, text string) {
	t.Helper()

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: text},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}
}

func editDoc(t *testing.T, srv *Server, docURI uri.URI, at protocol.Position, text string) {
	t.Helper()

	change := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangePartial{Range: protocol.Range{Start: at, End: at}, Text: text},
		},
	})

	if _, err := srv.handleDidChange(context.Background(), change); err != nil {
		t.Fatal(err)
	}
}

func symbolNames(t *testing.T, srv *Server, docURI uri.URI) []string {
	t.Helper()

	req := makeCall(t, protocol.MethodTextDocumentDocumentSymbol, protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})

	res, err := srv.handleDocumentSymbol(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	symbols, _ := res.([]protocol.DocumentSymbol)

	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = fmt.Sprintf("%s@%d", sym.Name, sym.Range.Start.Line)
	}

	return names
}

const deferredDoc = "component {\n\tpublic void function a() {\n\t\tvar x = 1;\n\t}\n}\n"

// An edit outside a function body no longer reparses inline. The reparse is
// owed, and the next handler to take the document's lock pays it, so what it
// answers reflects the edit.
func TestDeferredReparseIsCaughtUpByTheNextHandler(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///deferred.cfc")
	openDoc(t, srv, docURI, deferredDoc)

	editDoc(t, srv, docURI, protocol.Position{Line: 1}, "\tpublic void function added() {}\n")

	if !srv.reparseIsPending(docURI) {
		t.Fatal("an edit outside any function was reparsed inline")
	}

	if got, want := symbolNames(t, srv, docURI), []string{"added@1", "a@2"}; !slices.Equal(got, want) {
		t.Errorf("symbols after the edit: got %v, want %v", got, want)
	}

	if srv.reparseIsPending(docURI) {
		t.Error("taking the document's lock did not run the owed reparse")
	}
}

// Edits that arrive while a reparse is owed have positions in the new text,
// and the parse's scopes are still the old ones, so applying them to the parse
// incrementally would shift and invalidate the wrong functions. They are held
// back with the rest, and the parse caught up afterwards must be the one a
// fresh parse of the final text gives.
func TestEditsWhileAReparseIsPendingStayDeferred(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///pending.cfc")
	openDoc(t, srv, docURI, deferredDoc)

	editDoc(t, srv, docURI, protocol.Position{Line: 1}, "\t// header\n\t// more\n")
	// Inside a() in the new text; in the old scopes, line 4 is a's closing line.
	editDoc(t, srv, docURI, protocol.Position{Line: 4, Character: 2}, "var y = 2;\n\t\t")

	release := srv.lockDoc(docURI)

	srv.mu.RLock()
	pr := srv.parseResults[docURI]
	doc := srv.documents[docURI]
	srv.mu.RUnlock()

	fresh := parser.ParseWithOptions(docURI, doc, parser.ParseOptions{Shallow: true})

	if pr.Content != doc {
		t.Errorf("parse content lags the document")
	}

	if !slices.Equal(scopeSpans(pr), scopeSpans(fresh)) {
		t.Errorf("scopes: got %v, a fresh parse gives %v", scopeSpans(pr), scopeSpans(fresh))
	}

	release()
}

func scopeSpans(pr *parser.ParseResult) []string {
	out := make([]string, len(pr.Scopes))
	for i, sc := range pr.Scopes {
		out[i] = fmt.Sprintf("%d-%d", sc.Start, sc.End)
	}

	return out
}

// Nobody need ask: the timer catches the parse up on its own, so the index is
// current within reparseDelay of the last edit.
func TestDeferredReparseRunsOnItsOwn(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///timer.cfc")
	openDoc(t, srv, docURI, deferredDoc)

	editDoc(t, srv, docURI, protocol.Position{Line: 0}, "// c\n")

	deadline := time.Now().Add(5 * time.Second)
	for srv.reparseIsPending(docURI) {
		if time.Now().After(deadline) {
			t.Fatal("the deferred reparse never ran")
		}

		time.Sleep(10 * time.Millisecond)
	}

	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if srv.parseResults[docURI].Content != srv.documents[docURI] {
		t.Error("the timer ran but the parse still lags the document")
	}
}

// An edit inside a function body is cheap to apply to the parse and still is,
// so nothing is deferred for it.
func TestEditInsideAFunctionIsNotDeferred(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///infunc.cfc")
	openDoc(t, srv, docURI, deferredDoc)

	editDoc(t, srv, docURI, protocol.Position{Line: 2, Character: 2}, "var z = 3;\n\t\t")

	if srv.reparseIsPending(docURI) {
		t.Error("an edit inside a function was deferred")
	}
}

// The point of it: a keystroke at component level in a very large component
// costs the text edit, not a reparse of every signature in the file.
func TestKeystrokeOutsideAFunctionIsCheapInALargeFile(t *testing.T) {
	var b strings.Builder

	b.WriteString("component {\n")

	for i := range 5000 {
		fmt.Fprintf(&b, "\tpublic void function f%d(required string id) {\n\t\tvar x = foo.bar(id = arguments.id);\n\t\treturn;\n\t}\n", i)
	}

	b.WriteString("}\n")

	srv := newTestServer()
	docURI := uri.URI("file:///large.cfc")
	openDoc(t, srv, docURI, b.String())

	// didOpen starts a completion cache build that holds the document's lock;
	// wait it out so the timing is the keystroke's own.
	time.Sleep(50 * time.Millisecond)
	srv.lockDoc(docURI)()

	start := time.Now()

	editDoc(t, srv, docURI, protocol.Position{Line: 0, Character: 0}, "x")

	elapsed := time.Since(start)

	release := srv.lockDoc(docURI)
	reparse := time.Since(start) - elapsed

	release()

	t.Logf("keystroke %v, owed reparse %v", elapsed, reparse)

	if !testing.Short() && elapsed > reparse/4 {
		t.Errorf("a keystroke outside any function took %v, against %v for the reparse it should defer", elapsed, reparse)
	}
}

package server

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Closing a document must release everything held for it.
//
// A daemon outlives the editors it serves — that is the point of it — so
// anything didClose forgets is held for as long as the daemon runs, for a buffer
// nobody has open. The completion cache was the one structure left behind, and
// the largest: an entry per function scope, each holding that scope's items.
func TestDidCloseReleasesEverythingHeldForTheDocument(t *testing.T) {
	srv := newTestServer()

	abs := testdataDir() + "/CloseRelease.cfc"
	docURI := uri.URI("file://" + abs)
	content := "component {\n\tfunction thing() { var x = 1; }\n}\n"

	open := func() {
		params, err := json.Marshal(protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "cfml", Version: 1, Text: content},
		})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := srv.handleDidOpen(context.Background(), params); err != nil {
			t.Fatal(err)
		}
	}

	open()
	srv.compCache.PutFile(docURI, []protocol.CompletionItem{{Label: "thing"}})

	if srv.compCache.GetFile(docURI) == nil {
		t.Fatal("expected a completion cache entry after opening; the test proves nothing without one")
	}

	params, err := json.Marshal(protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := srv.handleDidClose(context.Background(), params); err != nil {
		t.Fatal(err)
	}

	if items := srv.compCache.GetFile(docURI); items != nil {
		t.Errorf("completion cache still holds %d items for a closed document", len(items))
	}

	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if _, ok := srv.parseResults[docURI]; ok {
		t.Error("parse result still held for a closed document")
	}

	if _, ok := srv.funcRanges[docURI]; ok {
		t.Error("function ranges still held for a closed document")
	}
}

// The per-URI maps and what didClose releases are the same list written twice.
// One added without a matching release is memory held for the life of the
// daemon, and nothing about the server's answers changes to show it.
func TestDidCloseCoversEveryPerDocumentMap(t *testing.T) {
	// docLocks is deliberately kept; handleDidClose says why.
	keptOnPurpose := map[string]string{
		"docLocks":    "a timer goroutine may still be waiting on it; handleDidClose explains",
		"documents":   "released through removeDocument",
		"lintCancels": "released by the scan's own defer",
	}

	_, file, _, _ := runtime.Caller(0)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "handler.go"))
	if err != nil {
		t.Fatalf("reading handler.go: %v", err)
	}

	src := string(data)

	st := reflect.TypeFor[Server]()
	for i := range st.NumField() {
		f := st.Field(i)
		if f.Type.Kind() != reflect.Map || f.Type.Key().String() != "uri.URI" {
			continue
		}

		if _, ok := keptOnPurpose[f.Name]; ok {
			continue
		}

		if !strings.Contains(src, "delete(s."+f.Name+", docURI)") &&
			!strings.Contains(src, "delete(timers, docURI)") {
			t.Errorf("Server has a per-document map %q that handleDidClose does not release; "+
				"it will be held for the life of the daemon", f.Name)
		}
	}
}

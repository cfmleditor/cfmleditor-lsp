package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestSharedScopeDefinitionReadsTheOpenBuffer. An application variable is
// declared in Application.cfc, and go-to-definition reads that file to find the
// line. When the file is open with unsaved edits, the answer has to come from
// the buffer the editor will jump into, not from the copy on disk, or the
// cursor lands on whatever line the declaration used to be on.
func TestSharedScopeDefinitionReadsTheOpenBuffer(t *testing.T) {
	dir := t.TempDir()

	app := "<cfcomponent>\n" +
		"<cffunction name=\"onApplicationStart\">\n" +
		"\t<cfset application.cache = {}>\n" +
		"</cffunction>\n" +
		"</cfcomponent>\n"

	appPath := filepath.Join(dir, "Application.cfc")
	if err := os.WriteFile(appPath, []byte(app), 0o600); err != nil {
		t.Fatal(err)
	}

	page := "<cfset x = application.cache>\n"

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.Features = config.ResolveFeatures(nil)

	pageURI := uri.File(filepath.Join(dir, "page.cfm"))
	srv.setDocument(pageURI, page)

	line, char := cursorPosition(t, page, "application.|cache")

	lineOf := func() uint32 {
		t.Helper()

		switch got := definitionAt(t, srv, pageURI, line, char).(type) {
		case protocol.Location:
			return got.Range.Start.Line
		case []protocol.Location:
			if len(got) > 0 {
				return got[0].Range.Start.Line
			}
		}

		t.Fatal("no definition for application.cache")

		return 0
	}

	if got := lineOf(); got != 2 {
		t.Fatalf("from disk: line %d, want 2", got)
	}

	// Three lines added above the declaration, not saved.
	srv.setDocument(uri.File(appPath), "// a\n// b\n// c\n"+app)

	if got := lineOf(); got != 5 {
		t.Errorf("with unsaved edits open: line %d, want 5 — the answer came from disk", got)
	}
}

// TestSharedScopeDefinitionInScriptSyntax. The script parser recorded
// application., session. and request. assignments as variables scope and
// server. ones not at all, so go-to-definition from a page found nothing in a
// script-syntax Application.cfc or Server.cfc, only in tag syntax.
func TestSharedScopeDefinitionInScriptSyntax(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"Application.cfc": "component {\n" +
			"\tfunction onApplicationStart() {\n" +
			"\t\tapplication.cache = {};\n" +
			"\t}\n" +
			"\tfunction onSessionStart() {\n" +
			"\t\tsession.cart = [];\n" +
			"\t}\n" +
			"}\n",
		"Server.cfc": "component {\n" +
			"\tfunction onServerStart() {\n" +
			"\t\tserver.started = now();\n" +
			"\t}\n" +
			"}\n",
	}

	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	page := "<cfset a = application.cache>\n<cfset b = session.cart>\n<cfset c = server.started>\n"

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.Features = config.ResolveFeatures(nil)

	pageURI := uri.File(filepath.Join(dir, "page.cfm"))
	srv.setDocument(pageURI, page)

	cases := []struct {
		cursor, file string
		line         uint32
	}{
		{"application.|cache", "Application.cfc", 2},
		{"session.|cart", "Application.cfc", 5},
		{"server.|started", "Server.cfc", 2},
	}

	for _, c := range cases {
		line, char := cursorPosition(t, page, c.cursor)

		var loc *protocol.Location

		switch got := definitionAt(t, srv, pageURI, line, char).(type) {
		case protocol.Location:
			loc = &got
		case []protocol.Location:
			if len(got) > 0 {
				loc = &got[0]
			}
		}

		if loc == nil {
			t.Errorf("%s: no definition", c.cursor)

			continue
		}

		if base := filepath.Base(loc.URI.Path()); base != c.file || loc.Range.Start.Line != c.line {
			t.Errorf("%s: %s line %d, want %s line %d", c.cursor, base, loc.Range.Start.Line, c.file, c.line)
		}
	}
}

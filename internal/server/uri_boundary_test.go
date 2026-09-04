package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// appFileFixture builds a workspace whose directory name contains a space, so
// that the URI the server hands back has to be percent-encoded to be valid.
// That is the same defect as cfmleditor/cfmleditor-lsp#45, reported there for a
// Windows drive letter, in the one shape a POSIX test host can exercise.
func appFileFixture(t *testing.T) (dir, appPath, docURI string) {
	t.Helper()

	dir = filepath.Join(t.TempDir(), "My Documents")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	appPath = filepath.Join(dir, "Application.cfc")
	if err := os.WriteFile(appPath, []byte("<cfcomponent></cfcomponent>"), 0o600); err != nil {
		t.Fatal(err)
	}

	page := filepath.Join(dir, "index.cfm")
	if err := os.WriteFile(page, []byte("<cfoutput>hi</cfoutput>"), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir, appPath, string(cfpath.ToURI(page))
}

// TestOpenActiveApplicationFileReturnsAWellFormedURI is the regression for
// issue #45: the command built its window/showDocument parameter as
// "file://" + path, so the client was handed a string it could not open. On
// Windows that was "file://C:\\Users\\...", with two slashes, backslashes and a
// bare drive colon; here it is an unencoded space.
func TestOpenActiveApplicationFileReturnsAWellFormedURI(t *testing.T) {
	_, appPath, docURI := appFileFixture(t)

	srv := newTestServer()

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.openActiveApplicationFile",
		Arguments: lspAnyArgs(docURI),
	})

	result, replyErr := srv.handleExecuteCommand(t.Context(), req)
	if replyErr != nil {
		t.Fatal(replyErr)
	}

	got, ok := result.(string)
	if !ok || got == "" {
		t.Fatalf("expected the Application.cfc URI back, got %#v", result)
	}

	if want := string(cfpath.ToURI(appPath)); got != want {
		t.Errorf("URI is not canonical\n got  %s\n want %s", got, want)
	}

	if strings.Contains(got, " ") {
		t.Errorf("URI contains a raw space, so it is not a valid URI: %s", got)
	}

	// It must survive the round trip a client makes: parse the URI, open the path.
	if _, err := os.Stat(cfpath.FromURI(got)); err != nil {
		t.Errorf("the returned URI does not resolve back to a readable file: %v", err)
	}
}

// TestOpenActiveApplicationFileAcceptsAnEncodedDocumentURI covers the inbound
// half of the same boundary. The command located Application.cfc by trimming
// "file://" off the document URI, which leaves the percent-escapes in place, so
// a spec-compliant client's URI produced a directory that does not exist and
// the command answered "No Application.cfc found".
func TestOpenActiveApplicationFileAcceptsAnEncodedDocumentURI(t *testing.T) {
	_, _, docURI := appFileFixture(t)

	if !strings.Contains(docURI, "%20") {
		t.Fatalf("fixture should produce an escaped URI, got %s", docURI)
	}

	srv := newTestServer()

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.openActiveApplicationFile",
		Arguments: lspAnyArgs(docURI),
	})

	result, replyErr := srv.handleExecuteCommand(t.Context(), req)
	if replyErr != nil {
		t.Fatal(replyErr)
	}

	if result == nil {
		t.Fatal("Application.cfc was not found from an escaped document URI")
	}
}

// TestDefinitionURIsAreCanonical guards the other direction the issue calls
// out: "most likely the same for all window/showDocument commands". Every
// Location the server returns is built the same way, so the same fix has to
// hold for go-to-definition on a file path.
func TestDefinitionURIsAreCanonical(t *testing.T) {
	dir, _, docURI := appFileFixture(t)

	included := filepath.Join(dir, "header.cfm")
	if err := os.WriteFile(included, []byte("<cfoutput>head</cfoutput>"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer()

	loc := srv.resolveFilePathDef("header.cfm", uri.URI(docURI))
	if loc == nil {
		t.Fatal("expected the included file to resolve")
	}

	if want := cfpath.ToURI(included); loc.URI != want {
		t.Errorf("Location URI is not canonical\n got  %s\n want %s", loc.URI, want)
	}

	if strings.Contains(string(loc.URI), " ") {
		t.Errorf("Location URI contains a raw space: %s", loc.URI)
	}
}

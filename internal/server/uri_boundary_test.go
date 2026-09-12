package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	json "github.com/go-json-experiment/json"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
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

// initializeWithRoots drives initialize with raw URI strings, so a test can
// hand the server the exact spelling a client sends rather than one built by
// cfpath.ToURI.
func initializeWithRoots(t *testing.T, rootURI string, folderURIs ...string) *Server {
	t.Helper()

	params := map[string]any{
		"processId":    nil,
		"capabilities": map[string]any{},
	}

	if rootURI != "" {
		params["rootUri"] = rootURI
	}

	if len(folderURIs) > 0 {
		folders := make([]map[string]any, 0, len(folderURIs))
		for i, u := range folderURIs {
			folders = append(folders, map[string]any{"uri": u, "name": fmt.Sprintf("w%d", i)})
		}

		params["workspaceFolders"] = folders
	}

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshalling params: %v", err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize: %v", err)
	}

	return s
}

// TestWorkspaceRootsDecodeTheClientsURIs is issue #45 at the one boundary it
// was not fixed at. The workspace roots are what searchRoots falls back to when
// no .cfmleditor.json names any, so every workspace-wide search — findRefs,
// scanWorkspace, document links, component resolution — walks whatever lands
// here.
//
// URI.Path() returns `/c:/Users/q/proj` for the canonical percent-encoded
// Windows form, a leading slash that is not a Windows path; cfpath.FromURI,
// which every other inbound conversion already goes through, does not.
//
// The non-canonical `file://C:\Users\q\proj` some clients send is a
// different story and is not fixable here: go.lsp.dev/protocol rewrites it to
// `file://c:\users\q\proj/` while unmarshalling, reading the whole path as
// the URI's authority, and both conversions then yield "/". See
// TestUncanonicalWindowsRootIsManagledUpstream.
func TestWorkspaceRootsDecodeTheClientsURIs(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{"plain posix path", "file:///tmp/plain/src", "/tmp/plain/src"},
		{"space is decoded", "file:///tmp/uri%20demo/src", "/tmp/uri demo/src"},
		{"canonical windows drive letter", "file:///c%3A/Users/q/proj", "c:/Users/q/proj"},
	}

	for _, tc := range cases {
		t.Run("rootUri: "+tc.name, func(t *testing.T) {
			s := initializeWithRoots(t, tc.uri)

			if len(s.workspaceRoots) != 1 || s.workspaceRoots[0] != tc.want {
				t.Errorf("workspaceRoots = %q, want [%q]", s.workspaceRoots, tc.want)
			}
		})

		t.Run("workspaceFolders: "+tc.name, func(t *testing.T) {
			s := initializeWithRoots(t, "", tc.uri)

			if len(s.workspaceRoots) != 1 || s.workspaceRoots[0] != tc.want {
				t.Errorf("workspaceRoots = %q, want [%q]", s.workspaceRoots, tc.want)
			}
		})
	}
}

// TestUncanonicalWindowsRootIsMangledUpstream records a limit rather than a
// behaviour we chose, so that the next person to look does not spend the time
// twice.
//
// `file://C:\Users\q\proj` — two slashes, backslashes, a bare drive colon —
// is what several clients send, and go.lsp.dev/protocol parses the whole thing
// as the URI's authority and re-serialises it lowercased with a trailing
// slash. By the time any handler runs, the path is gone; no conversion on our
// side can recover it. The root ends up "/", which for a workspace-wide search
// means the filesystem root.
//
// If this test starts failing, the library has been fixed and the guard notes
// around workspaceRoots can go.
func TestUncanonicalWindowsRootIsMangledUpstream(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"processId":    nil,
		"capabilities": map[string]any{},
		"rootUri":      "file://C:\\Users\\q\\proj",
	})
	if err != nil {
		t.Fatal(err)
	}

	var params protocol.InitializeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshalling initialize params: %v", err)
	}

	if params.RootURI == nil {
		t.Fatal("rootUri did not survive unmarshalling at all")
	}

	decoded := string(*params.RootURI)
	if !strings.Contains(decoded, "users") {
		t.Skipf("go.lsp.dev/protocol no longer lowercases the authority (%q); recheck this boundary", decoded)
	}

	if got := cfpath.FromURI(decoded); got != "/" {
		t.Errorf("the upstream mangling changed: FromURI(%q) = %q, want \"/\" — recheck whether this is now fixable here", decoded, got)
	}
}

// TestChangedWorkspaceFoldersDecodeTheirURIs covers the same conversion on the
// other side. A root added here is indexed and then searched; one removed has
// to match what initialize stored, or it is never removed from the list.
func TestChangedWorkspaceFoldersDecodeTheirURIs(t *testing.T) {
	added := "file:///tmp/uri%20demo/added"
	wantAdded := "/tmp/uri demo/added"

	s := initializeWithRoots(t, "file:///tmp/plain/src")

	raw, err := json.Marshal(protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added: []protocol.WorkspaceFolder{{URI: uri.URI(added), Name: "a"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.handleDidChangeWorkspaceFolders(context.Background(), raw); err != nil {
		t.Fatalf("handleDidChangeWorkspaceFolders: %v", err)
	}

	if !slices.Contains(s.workspaceRoots, wantAdded) {
		t.Errorf("workspaceRoots = %q, want it to contain %q", s.workspaceRoots, wantAdded)
	}

	// Removing it again has to match the stored form, or the root stays.
	raw, err = json.Marshal(protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: uri.URI(added), Name: "a"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.handleDidChangeWorkspaceFolders(context.Background(), raw); err != nil {
		t.Fatalf("handleDidChangeWorkspaceFolders: %v", err)
	}

	if slices.Contains(s.workspaceRoots, wantAdded) {
		t.Errorf("removed folder is still a root: %q", s.workspaceRoots)
	}
}

// TestUnusableWorkspaceRootIsDeclined is the guard over the mangling above. A
// root of "/" is not a workspace, and since searchRoots hands these to
// findRefs and scanWorkspace, keeping one would have a "find all references"
// crawl the filesystem. Nothing found is the better answer, and the warning
// says why.
func TestUnusableWorkspaceRootIsDeclined(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  string
	}{
		{"a client's non-canonical windows root, which upstream collapses to /", "file://C:\\Users\\q\\proj"},
		{"the filesystem root itself", "file:///"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := initializeWithRoots(t, tc.uri)

			if len(s.workspaceRoots) != 0 {
				t.Errorf("workspaceRoots = %q, want none — a search over these walks the machine", s.workspaceRoots)
			}
		})
	}
}

// TestUsableWorkspaceRootsAreStillKept is the boundary: declining "/" must not
// decline anything real.
func TestUsableWorkspaceRootsAreStillKept(t *testing.T) {
	s := initializeWithRoots(t, "", "file:///tmp/a", "file:///tmp/uri%20demo/b")

	want := []string{"/tmp/a", "/tmp/uri demo/b"}
	if !slices.Equal(s.workspaceRoots, want) {
		t.Errorf("workspaceRoots = %q, want %q", s.workspaceRoots, want)
	}
}

// TestUnusableAddedFolderIsDeclined covers the same on the other handler, where
// the root is also handed straight to indexRoot.
func TestUnusableAddedFolderIsDeclined(t *testing.T) {
	s := initializeWithRoots(t, "file:///tmp/plain/src")

	raw, err := json.Marshal(protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added: []protocol.WorkspaceFolder{{URI: uri.URI("file://C:\\Users\\q\\proj"), Name: "a"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.handleDidChangeWorkspaceFolders(context.Background(), raw); err != nil {
		t.Fatalf("handleDidChangeWorkspaceFolders: %v", err)
	}

	if slices.Contains(s.workspaceRoots, "/") {
		t.Errorf("the filesystem root was added as a workspace root: %q", s.workspaceRoots)
	}

	if !slices.Equal(s.workspaceRoots, []string{"/tmp/plain/src"}) {
		t.Errorf("workspaceRoots = %q, want the original root untouched", s.workspaceRoots)
	}
}

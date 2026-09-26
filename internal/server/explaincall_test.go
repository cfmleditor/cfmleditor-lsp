package server

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// messageConn records every window/showMessage the server sends. The message
// is the only part of cfmleditor.explainCall's answer Zed shows anyone, since
// it ignores a command's return value.
type messageConn struct {
	fakeConn

	mu       sync.Mutex
	messages []string
}

func (c *messageConn) Notify(_ context.Context, method string, params any) error {
	if p, ok := params.(*protocol.ShowMessageParams); ok && method == protocol.MethodWindowShowMessage {
		c.mu.Lock()
		c.messages = append(c.messages, p.Message)
		c.mu.Unlock()
	}

	return nil
}

// explainWorkspace writes a caller whose line 4 (0-based) calls a method B
// defines and whose line 5 calls one it does not, and opens the caller.
func explainWorkspace(t *testing.T) (*Server, *messageConn, uri.URI, string) {
	t.Helper()

	dir := t.TempDir()

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	caller := "component {\n" +
		"\tvariables.b = new B();\n" +
		"\n" +
		"\tpublic void function go() {\n" +
		"\t\tvariables.b.run();\n" +
		"\t\tvariables.b.missing();\n" +
		"\t}\n" +
		"}\n"

	write("A.cfc", caller)
	write("B.cfc", "component {\n\tpublic void function run() {}\n}\n")

	conn := &messageConn{}
	srv := NewServer(conn, cflog.NewLogger(false))
	srv.WorkspaceFolders = []string{dir}

	aPath := filepath.Join(dir, "A.cfc")
	docURI := uri.File(aPath)

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: caller},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	return srv, conn, docURI, aPath
}

func explainCall(t *testing.T, srv *Server, args ...any) string {
	t.Helper()

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.explainCall",
		Arguments: lspAnyArgs(args...),
	})

	res, err := srv.handleExecuteCommand(context.Background(), req)
	if err != nil {
		t.Fatalf("explainCall: %v", err)
	}

	out, ok := res.(string)
	if !ok {
		t.Fatalf("expected the report as a string, got %T", res)
	}

	return out
}

// The report is the CLI's, heading and all, and it reaches the user as a
// message as well as the return value.
func TestExplainCallReportsResolvedAndUnresolved(t *testing.T) {
	srv, conn, docURI, aPath := explainWorkspace(t)

	resolved := explainCall(t, srv, string(docURI), 4)
	t.Logf("line 5:\n%s", resolved)

	if want := aPath + ":5: variables.b.run\n"; !strings.HasPrefix(resolved, want) {
		t.Errorf("heading: want prefix %q, got:\n%s", want, resolved)
	}

	if !strings.HasSuffix(resolved, "\n  => resolved") {
		t.Errorf("expected a resolved verdict, got:\n%s", resolved)
	}

	unresolved := explainCall(t, srv, string(docURI), 5)
	t.Logf("line 6:\n%s", unresolved)

	if want := aPath + ":6: variables.b.missing\n"; !strings.HasPrefix(unresolved, want) {
		t.Errorf("heading: want prefix %q, got:\n%s", want, unresolved)
	}

	if !strings.Contains(unresolved, "\n  => unresolved: ") {
		t.Errorf("expected an unresolved verdict, got:\n%s", unresolved)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !slices.Equal(conn.messages, []string{resolved, unresolved}) {
		t.Errorf("each report should be shown as a message; got %q", conn.messages)
	}
}

// The filter narrows a line to the calls whose name or receiver matches, as
// the CLI's third argument does.
func TestExplainCallFilter(t *testing.T) {
	srv, _, docURI, _ := explainWorkspace(t)

	out := explainCall(t, srv, string(docURI), 5, "RUN")
	if !strings.HasPrefix(out, "No call sites on ") || !strings.HasSuffix(out, `:6 matching "RUN"`) {
		t.Errorf("a filter matching nothing on the line should say so, got:\n%s", out)
	}

	out = explainCall(t, srv, string(docURI), 5, "MISS")
	if !strings.Contains(out, "variables.b.missing") {
		t.Errorf("a matching filter should keep the call, got:\n%s", out)
	}
}

// A line with no call is an answer, not an error: an error from a command is
// something no editor shows its user.
func TestExplainCallLineWithNoCall(t *testing.T) {
	srv, conn, docURI, aPath := explainWorkspace(t)

	out := explainCall(t, srv, string(docURI), 2)

	if want := "No call sites on " + aPath + ":3"; out != want {
		t.Errorf("want %q, got %q", want, out)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !slices.Equal(conn.messages, []string{out}) {
		t.Errorf("the no-call answer should be shown as a message; got %q", conn.messages)
	}
}

// The document's cached ParseResult loses its call sites on an edit outside a
// function, so an explain answered from it says "no call sites" about a line
// that plainly has one. Both the command and the code action must read the
// text the editor holds.
//
// The code action reads the line's text, so this also asks it about line 6
// before the edit, when that line is the closing brace, and after, when the
// call to missing() has moved onto it.
func TestExplainCallAfterAnEditOutsideAFunction(t *testing.T) {
	srv, _, docURI, aPath := explainWorkspace(t)

	if slices.Contains(codeActionCommands(t, srv, docURI, protocol.Position{Line: 6}), "cfmleditor.explainCall") {
		t.Fatal("explain offered on a closing brace")
	}

	edit := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangePartial{Range: protocol.Range{}, Text: "// added\n"},
		},
	})

	if _, err := srv.handleDidChange(context.Background(), edit); err != nil {
		t.Fatal(err)
	}

	// The call to run() is on 0-based line 5 now.
	out := explainCall(t, srv, string(docURI), 5)
	if want := aPath + ":6: variables.b.run\n"; !strings.HasPrefix(out, want) {
		t.Errorf("want prefix %q, got:\n%s", want, out)
	}

	if !slices.Contains(codeActionCommands(t, srv, docURI, protocol.Position{Line: 6}), "cfmleditor.explainCall") {
		t.Error("explain not offered on the line the call to missing() moved to")
	}
}

// A file the editor does not have open is read from disk, since the command
// picker can name any file.
func TestExplainCallOnAFileThatIsNotOpen(t *testing.T) {
	srv, _, docURI, _ := explainWorkspace(t)
	srv.removeDocument(docURI)

	if out := explainCall(t, srv, string(docURI), 4); !strings.Contains(out, "=> resolved") {
		t.Errorf("expected the call read from disk to resolve, got:\n%s", out)
	}
}

func codeActions(t *testing.T, srv *Server, docURI uri.URI, pos protocol.Position) []protocol.CodeAction {
	t.Helper()

	req := makeCall(t, protocol.MethodTextDocumentCodeAction, protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Range:        protocol.Range{Start: pos, End: pos},
	})

	res, err := srv.handleCodeAction(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCodeAction: %v", err)
	}

	actions, _ := res.([]protocol.CodeAction)

	return actions
}

func codeActionCommands(t *testing.T, srv *Server, docURI uri.URI, pos protocol.Position) []string {
	t.Helper()

	var commands []string
	for _, a := range codeActions(t, srv, docURI, pos) {
		commands = append(commands, a.Command.Command)
	}

	return commands
}

// The explain action is offered on a line holding a call, wherever the cursor
// sits on it, and on no other line.
func TestCodeActionOffersExplainOnlyOnALineWithACall(t *testing.T) {
	srv, _, docURI, _ := explainWorkspace(t)

	for _, pos := range []protocol.Position{{Line: 4, Character: 0}, {Line: 4, Character: 16}} {
		var explain *protocol.CodeAction

		for _, a := range codeActions(t, srv, docURI, pos) {
			if a.Command.Command == "cfmleditor.explainCall" {
				explain = &a
			}
		}

		if explain == nil {
			t.Fatalf("line 4 col %d: explain not offered on a line with a call", pos.Character)
		}

		if explain.Title != "Explain call resolution on line 5" {
			t.Errorf("title: %q", explain.Title)
		}

		if len(explain.Command.Arguments) != 2 || string(explain.Command.Arguments[1]) != "4" {
			t.Errorf("arguments should be the URI and the 0-based line, got %s", explain.Command.Arguments)
		}
	}

	for _, pos := range []protocol.Position{{Line: 2, Character: 0}, {Line: 3, Character: 22}} {
		if slices.Contains(codeActionCommands(t, srv, docURI, pos), "cfmleditor.explainCall") {
			t.Errorf("line %d: explain offered on a line with no call", pos.Line)
		}
	}
}

// The file-level dependency graph passes the document URI alone, which is the
// form of cfmleditor.exportDeps that graphs every function in the file.
func TestCodeActionOffersTheFileDependencyGraph(t *testing.T) {
	srv, _, docURI, _ := explainWorkspace(t)

	for _, pos := range []protocol.Position{{Line: 4, Character: 16}, {Line: 2, Character: 0}} {
		var found bool

		for _, a := range codeActions(t, srv, docURI, pos) {
			if a.Title != "Export dependency graph for A.cfc" {
				continue
			}

			found = true

			if a.Command.Command != "cfmleditor.exportDeps" || len(a.Command.Arguments) != 1 {
				t.Errorf("line %d: want exportDeps with the URI alone, got %s %s", pos.Line, a.Command.Command, a.Command.Arguments)
			}
		}

		if !found {
			t.Errorf("line %d: file dependency graph not offered", pos.Line)
		}
	}
}

// lineMayHoldCall is a look at the line's text, so it is pinned on the shapes
// it has to tell apart: a call in script, in a tag expression and chained, and
// the parens that are not calls.
func TestLineMayHoldCall(t *testing.T) {
	for text, want := range map[string]bool{
		"\tvariables.b.run();":              true,
		"x = foo.bar( id = arguments.id );": true,
		"<cfif isDefined(\"url.x\")>":       true,
		"<cfset y = obj.get().value()>":     true,
		".method( a )":                      true,
		"return helper(x);":                 true,
		"\tpublic void function go() {":     false,
		"function(a, b) {":                  false,
		"if (x) {":                          false,
		"} elseif ( y ) {":                  false,
		"for (i = 1; i <= 10; i++) {":       false,
		"<cfif (a gt b)>":                   false,
		"<cfreturn (x)>":                    false,
		"x = (1 + 2) * 3;":                  false,
		"x = 1;":                            false,
		"}":                                 false,
		"a = 1 and (b or c);":               false,
		"x = arr[1](2);":                    false,
	} {
		if got := lineMayHoldCall("first line\n"+text+"\nlast line", 1); got != want {
			t.Errorf("%q: got %v, want %v", text, got, want)
		}
	}
}

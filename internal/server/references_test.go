package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// refsWorkspace copies testdata/refs into a temp dir and returns a server whose
// workspace is that copy, with one file opened and indexed.
func refsWorkspace(t *testing.T, open string) (*Server, string, uri.URI) {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "refs")

	names, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(src, n.Name()))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, n.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer()
	srv.References = true
	srv.WorkspaceFolders = []string{dir}

	path := filepath.Join(dir, open)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	docURI := uri.File(path)

	req := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: string(data)},
	})

	if _, err := srv.handleDidOpen(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	return srv, dir, docURI
}

func referencesAt(t *testing.T, srv *Server, docURI uri.URI, line, char uint32, includeDecl bool) []protocol.Location {
	t.Helper()

	req := makeCall(t, protocol.MethodTextDocumentReferences, protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     protocol.Position{Line: line, Character: char},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: includeDecl},
	})

	res, err := srv.handleReferences(context.Background(), req)
	if err != nil {
		t.Fatalf("handleReferences: %v", err)
	}

	if res == nil {
		return nil
	}

	locs, ok := res.([]protocol.Location)
	if !ok {
		t.Fatalf("handleReferences returned %T, want []protocol.Location", res)
	}

	return locs
}

// found renders locations as "file:line" for comparison, since the temp dir
// prefix is different on every run.
func found(t *testing.T, locs []protocol.Location, dir string) []string {
	t.Helper()

	out := make([]string, 0, len(locs))

	for _, l := range locs {
		rel, err := filepath.Rel(dir, cfpath.FromURI(string(l.URI)))
		if err != nil {
			t.Fatalf("location outside the workspace: %s", l.URI)
		}

		out = append(out, rel+":"+itoa(int(l.Range.Start.Line)))
	}

	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b []byte

	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}

	return string(b)
}

func hasAll(got, want []string) bool {
	for _, w := range want {
		seen := false

		for _, g := range got {
			if g == w {
				seen = true

				break
			}
		}

		if !seen {
			return false
		}
	}

	return true
}

// TestReferencesFindsCallsToTheFunctionUnderTheCursor is the feature. The
// fixture has four components declaring a GetReport, so a result that simply
// grepped the name would return every one of them; only the two that reach
// controller.cfc's are references to it.
func TestReferencesFindsCallsToTheFunctionUnderTheCursor(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	// "public struct function GetReport() {" — the declaration.
	locs := referencesAt(t, srv, docURI, 4, 27, false)

	got := found(t, locs, dir)

	// controller.cfc's own RunReport calls it bare; view.cfm calls it through
	// a `new controller()` receiver.
	want := []string{"controller.cfc:9", "view.cfm:1"}
	if !hasAll(got, want) {
		t.Errorf("references = %v, want to contain %v", got, want)
	}

	// a.cfc, b.cfc, service.cfc and persist.cfc each have a GetReport of their
	// own, and calls to those are not references to this one.
	for _, l := range got {
		switch l {
		case "a.cfc:12", "b.cfc:12", "service.cfc:7", "persist.cfc:4":
			t.Errorf("reference to an unrelated same-named function: %s (all: %v)", l, got)
		}
	}
}

// TestReferencesFromACallSite asks from a call rather than a declaration, which
// is where the cursor usually is. It works only because the search is scoped by
// the file that *declares* the function, resolved first; scoping by the
// requesting document would make view.cfm's own call the only thing in scope.
func TestReferencesFromACallSite(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "view.cfm")

	// "<cfset report = myCtrl.GetReport()>"
	locs := referencesAt(t, srv, docURI, 1, 25, false)

	got := found(t, locs, dir)

	want := []string{"controller.cfc:9", "view.cfm:1"}
	if !hasAll(got, want) {
		t.Errorf("references = %v, want to contain %v", got, want)
	}
}

// TestReferencesIncludeDeclaration covers the request's one option. The
// declaration is not a call site, so nothing else would ever return it.
func TestReferencesIncludeDeclaration(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	without := found(t, referencesAt(t, srv, docURI, 4, 27, false), dir)
	with := found(t, referencesAt(t, srv, docURI, 4, 27, true), dir)

	if hasAll(without, []string{"controller.cfc:4"}) {
		t.Errorf("declaration returned without being asked for: %v", without)
	}

	if !hasAll(with, []string{"controller.cfc:4"}) {
		t.Errorf("includeDeclaration did not add the declaration: %v", with)
	}
}

// TestReferenceRangeSpansTheIdentifier pins the column. refs.Entry carries a
// line and no column, so returning the whole line would have been the easy
// answer — and would put the cursor at the start of the line on every jump.
func TestReferenceRangeSpansTheIdentifier(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	for _, l := range referencesAt(t, srv, docURI, 4, 27, false) {
		rel, _ := filepath.Rel(dir, cfpath.FromURI(string(l.URI)))
		if rel != "view.cfm" {
			continue
		}

		// `<cfset report = myCtrl.GetReport()>`
		//                         ^ 23
		if l.Range.Start.Character != 23 || l.Range.End.Character != 32 {
			t.Errorf("range = %d-%d, want 23-32 (the GetReport identifier)", l.Range.Start.Character, l.Range.End.Character)
		}

		return
	}

	t.Fatal("no reference in view.cfm to check the range of")
}

// TestReferencesToAComponentPath covers the other kind of target. The cursor is
// on a dot-path rather than a function name, and the search is against parsed
// component refs instead of call sites.
func TestReferencesToAComponentPath(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "view.cfm")

	// "<cfset myCtrl = new controller()>"
	locs := referencesAt(t, srv, docURI, 0, 22, false)

	got := found(t, locs, dir)

	want := []string{"view.cfm:0", "report_view.cfm:0"}
	if !hasAll(got, want) {
		t.Errorf("references = %v, want to contain %v", got, want)
	}

	// The parser also records the variables a component reference flows into,
	// so `report = myCtrl.RunReport()` on the next line is a ref to the same
	// component on a line that never names it. Those are kept, anchored on the
	// receiving variable — see TestComponentEntryAnchorsOnTheVariable for why
	// dropping them is not an option — but never as a whole-line match, which
	// would give the reader nothing to look at.
	for _, l := range locs {
		if l.Range.Start.Character == 0 && l.Range.End.Character > 20 {
			t.Errorf("whole-line range at %v: %+v", found(t, []protocol.Location{l}, dir), l.Range)
		}
	}
}

// TestComponentEntryAnchorsOnTheVariable is why a component entry whose line
// does not name the component is anchored rather than dropped. A
// componentResolver establishes a reference from an expression that never
// spells the resolved path — `svc = getPageTools()` is a reference to
// packages.tass.pagetools, and "pagetools" appears nowhere on the line — so
// dropping those would lose most instantiation sites in exactly the
// resolver-configured projects this server exists for.
func TestComponentEntryAnchorsOnTheVariable(t *testing.T) {
	srv := newTestServer()
	path := filepath.Join(t.TempDir(), "a.cfc")
	srv.setDocument(cfpath.ToURI(path), "component {\n\tsvc = getPageTools();\n}\n")

	locs := srv.entryLocations([]refs.Entry{{File: path, Variable: "svc", Line: 1}}, "pagetools", true)

	if len(locs) != 1 {
		t.Fatalf("resolver-established reference dropped: %+v", locs)
	}

	// "\tsvc = getPageTools();" — svc starts at 1.
	if locs[0].Range.Start.Character != 1 || locs[0].Range.End.Character != 4 {
		t.Errorf("range = %d-%d, want 1-4 (the receiving variable)",
			locs[0].Range.Start.Character, locs[0].Range.End.Character)
	}
}

// TestComponentEntryWithNothingToAnchorOnIsDropped is the boundary: with
// neither the component nor its variable on the line there is nothing to point
// at, and a whole-line range would send the reader to a statement they cannot
// connect to the search.
func TestComponentEntryWithNothingToAnchorOnIsDropped(t *testing.T) {
	srv := newTestServer()
	path := filepath.Join(t.TempDir(), "a.cfc")
	srv.setDocument(cfpath.ToURI(path), "component {\n\tsomethingElse();\n}\n")

	if locs := srv.entryLocations([]refs.Entry{{File: path, Variable: "svc", Line: 1}}, "pagetools", true); len(locs) != 0 {
		t.Errorf("entry kept with nothing on the line to anchor it: %+v", locs)
	}
}

// TestReferencesSeeUnsavedEdits covers the scan reading the editor's copy. It
// opens files itself, from disk, so without that a search run while the file
// being edited has unsaved changes reports line numbers from the saved text —
// and the file being edited is the likeliest one to search.
func TestReferencesSeeUnsavedEdits(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	// Insert a line above everything, shifting every line down by one.
	content, _ := srv.getDocument(docURI)
	srv.setDocument(docURI, "// an unsaved comment\n"+content)

	locs := referencesAt(t, srv, docURI, 5, 27, false)

	for _, l := range found(t, locs, dir) {
		if l == "controller.cfc:10" {
			return
		}
	}

	t.Errorf("references were read from the saved file: %v", found(t, locs, dir))
}

// TestDeclarationRangeSpansTheIdentifier is the includeDeclaration half of the
// column work: go-to-definition returns column zero, which is fine for a jump
// but puts one entry in a references list highlighted differently from the rest.
func TestDeclarationRangeSpansTheIdentifier(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	for _, l := range referencesAt(t, srv, docURI, 4, 27, true) {
		rel, _ := filepath.Rel(dir, cfpath.FromURI(string(l.URI)))
		if rel != "controller.cfc" || l.Range.Start.Line != 4 {
			continue
		}

		// `    public struct function GetReport() {`
		//                             ^ 27
		if l.Range.Start.Character != 27 || l.Range.End.Character != 36 {
			t.Errorf("declaration range = %d-%d, want 27-36", l.Range.Start.Character, l.Range.End.Character)
		}

		return
	}

	t.Fatal("declaration not among the results")
}

// TestReferencesDisabledByDefault is the flag. Both halves matter: the
// capability has to be absent, or the editor offers a command, and the handler
// has to decline, or a client that sends the request anyway triggers a full
// workspace scan nobody opted into.
func TestReferencesDisabledByDefault(t *testing.T) {
	srv, _, docURI := refsWorkspace(t, "controller.cfc")
	srv.References = false

	if p := srv.capabilities().ReferencesProvider; p != protocol.Boolean(false) {
		t.Errorf("referencesProvider = %v with the flag off, want false", p)
	}

	if locs := referencesAt(t, srv, docURI, 4, 27, false); len(locs) != 0 {
		t.Errorf("handler answered with the flag off: %v", locs)
	}

	srv.References = true

	if p := srv.capabilities().ReferencesProvider; p != protocol.Boolean(true) {
		t.Errorf("referencesProvider = %v with the flag on, want true", p)
	}
}

// TestReferencesFlagReachesTheServerFromConfig is the other half of the flag:
// applyConfig is the last hop, and a key it does not copy is one the user can
// set, the loader can read, and nothing will ever act on.
func TestReferencesFlagReachesTheServerFromConfig(t *testing.T) {
	if s := initializeWithConfig(t, `{"references":{"enabled":true}}`); !s.References {
		t.Error("references.enabled not applied to the server")
	}

	if s := initializeWithConfig(t, `{"linting":{"enabled":false}}`); s.References {
		t.Error("references enabled without being asked for")
	}
}

// TestIdentSpanMatchesWholeIdentifiers covers the column search directly. The
// substring cases are the ones that would silently point at the wrong call.
func TestIdentSpanMatchesWholeIdentifiers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		text       string
		ident      string
		start, end int
		ok         bool
	}{
		{"plain call", "    GetData(1);", "GetData", 4, 11, true},
		{"case-insensitive", "    getdata(1);", "GetData", 4, 11, true},
		{"qualified", "return VARIABLES.persist.GetData(a);", "GetData", 25, 32, true},
		{"longer name is not a match", "    GetDataSet(1);", "GetData", 0, 0, false},
		{"prefixed name is not a match", "    doGetData(1);", "GetData", 0, 0, false},
		{"skips a substring to reach the real one", "var GetDataSet = GetData();", "GetData", 17, 24, true},
		{"underscore is part of a name", "    GetData_v2(1);", "GetData", 0, 0, false},
		{"absent", "    somethingElse();", "GetData", 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start, end, ok := identSpan(tc.text, tc.ident)
			if ok != tc.ok || (ok && (start != tc.start || end != tc.end)) {
				t.Errorf("identSpan(%q, %q) = %d, %d, %v; want %d, %d, %v",
					tc.text, tc.ident, start, end, ok, tc.start, tc.end, tc.ok)
			}
		})
	}
}

// TestReferenceColumnCountsUTF16Units covers a line with a non-ASCII character
// before the match. LSP character offsets are UTF-16 code units, so counting
// bytes would shift the highlight past the identifier.
func TestReferenceColumnCountsUTF16Units(t *testing.T) {
	t.Parallel()

	// "é" is two bytes and one UTF-16 unit; the emoji is four bytes and two.
	lines := []string{`// café 🎉`, `    GetData();`}

	got := nameRange(lines, 1, "GetData")
	if got.Start.Character != 4 || got.End.Character != 11 {
		t.Errorf("plain line: %d-%d, want 4-11", got.Start.Character, got.End.Character)
	}

	// x( 0) =(2) "(4) c(5) a f é(8) space 🎉(10 and 11) "(12) ;(13) space G(15).
	got = nameRange([]string{`x = "café 🎉"; GetData();`}, 0, "GetData")
	if got.Start.Character != 15 || got.End.Character != 22 {
		t.Errorf("non-ASCII line: %d-%d, want 15-22", got.Start.Character, got.End.Character)
	}
}

// TestDeclarationOfIgnoresThisFileForAQualifiedCall pins which definition a
// qualified call is scoped by. `dao.save()` names a receiver, so it is not
// calling the save() this component happens to declare — and picking that one
// would return this component's callers instead of the DAO's.
// handleDefinition excludes the current file here for the same reason.
func TestDeclarationOfIgnoresThisFileForAQualifiedCall(t *testing.T) {
	srv := newTestServer()

	docURI := uri.URI("file:///UserService.cfc")
	other := uri.URI("file:///UserDAO.cfc")

	srv.index.IndexFileFromResult(docURI, []parser.FunctionDef{{Name: "save", URI: docURI, Line: 2}}, nil)
	srv.index.IndexFileFromResult(other, []parser.FunctionDef{{Name: "save", URI: other, Line: 7}}, nil)

	content := "component {\n\tfunction save() {}\n\tfunction run() {\n\t\tdao.save();\n\t}\n}\n"

	// The cursor is on `save` in `dao.save()`, whose receiver does not resolve.
	got := srv.declarationOf("save", content, docURI, 3, 7)

	if got == nil {
		t.Fatal("no declaration found")
	}

	if got.URI == docURI {
		t.Errorf("scoped to this file's own save() at line %d; a qualified call is not calling into itself", got.Range.Start.Line)
	}
}

// TestDeclarationOfPrefersThisFileForABareCall is the other side: with no
// receiver, this file's own definition is exactly the right answer.
func TestDeclarationOfPrefersThisFileForABareCall(t *testing.T) {
	srv := newTestServer()

	docURI := uri.URI("file:///UserService.cfc")
	other := uri.URI("file:///UserDAO.cfc")

	srv.index.IndexFileFromResult(docURI, []parser.FunctionDef{{Name: "save", URI: docURI, Line: 2}}, nil)
	srv.index.IndexFileFromResult(other, []parser.FunctionDef{{Name: "save", URI: other, Line: 7}}, nil)

	content := "component {\n\tfunction save() {}\n\tfunction run() {\n\t\tsave();\n\t}\n}\n"

	got := srv.declarationOf("save", content, docURI, 3, 4)

	if got == nil || got.URI != docURI {
		t.Errorf("bare call should resolve to this file's own definition, got %+v", got)
	}
}

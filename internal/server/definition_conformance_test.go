package server

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// The definition cases the `cfmleditor` VS Code extension has always been
// measured against, replayed here.
//
// The extension stands its own providers down whenever this server is running,
// so every case it answers and this server does not is something a user loses by
// enabling the server. That trade was being made without anyone able to say what
// it cost; this names the price. See testdata/conformance/README.md for where the
// fixtures came from and how to refresh them.
//
// A case is written the way the extension writes it — a snippet of the line with
// `|` marking the cursor — so the same case reads the same in both suites. The
// snippet must occur exactly once in the file, which is what makes it a position
// rather than a guess.

// conformanceCase is one cursor position and where it should land.
type conformanceCase struct {
	name string
	file string // relative to testdata/conformance
	at   string // line snippet with | at the cursor
	want string // relative path of the file the definition should be in
}

// conformanceCases mirrors `provideDefinition` in the extension's
// CFMLDefinitionProvider.test.ts. Cases the extension itself skips are left out:
// they assert behaviour neither implementation has.
var conformanceCases = []conformanceCase{
	// component definitions (CFML)
	{"cfml/returntype", "cfml/WidgetFactory.cfc", `name="create_with_new" returntype="|cfml.Widget`, "cfml/Widget.cfc"},
	{"cfml/new", "cfml/WidgetFactory.cfc", `var widget = new |cfml.Widget()`, "cfml/Widget.cfc"},
	{"cfml/createObject-component", "cfml/WidgetFactory.cfc", `createObject("component", "|cfml.Widget")`, "cfml/Widget.cfc"},
	{"cfml/createObject-default", "cfml/WidgetFactory.cfc", `createObject("|cfml.Widget")`, "cfml/Widget.cfc"},
	{"cfml/cfargument-type", "cfml/WidgetFactory.cfc", `cfargument type="|cfml.Widget"`, "cfml/Widget.cfc"},
	{"cfml/case-insensitive", "cfml/WidgetFactory.cfc", `new |cFML.wIDGET`, "cfml/Widget.cfc"},
	{"cfml/cfinvoke-component", "cfml/WidgetFactory.cfc", `<cfinvoke component="|cfml.Widget"`, "cfml/Widget.cfc"},
	{"cfml/isInstanceOf", "cfml/WidgetFactory.cfc", `isInstanceOf(widget, "|cfml.Widget")`, "cfml/Widget.cfc"},
	{"cfml/extends", "cfml/Widget.cfc", `extends="|cfml.Base"`, "cfml/Base.cfc"},
	{"cfml/implements", "cfml/WidgetFactory.cfc", `implements="|cfml.IFactory"`, "cfml/IFactory.cfc"},

	// component definitions (CFScript)
	{"cfscript/returntype", "cfscript/GizmoFactory.cfc", `|cfscript.Gizmo function create_with_new`, "cfscript/Gizmo.cfc"},
	{"cfscript/new", "cfscript/GizmoFactory.cfc", `var gizmo = new |cfscript.Gizmo();`, "cfscript/Gizmo.cfc"},
	{"cfscript/createObject-component", "cfscript/GizmoFactory.cfc", `createObject("component", "|cfscript.Gizmo")`, "cfscript/Gizmo.cfc"},
	{"cfscript/createObject-default", "cfscript/GizmoFactory.cfc", `createObject("|cfscript.Gizmo")`, "cfscript/Gizmo.cfc"},
	{"cfscript/cfargument-type", "cfscript/GizmoFactory.cfc", `create_from(|cfscript.Gizmo source)`, "cfscript/Gizmo.cfc"},
	{"cfscript/method-returntype", "cfscript/Gizmo.cfc", `|cfscript.Gizmo function init`, "cfscript/Gizmo.cfc"},
	{"cfscript/import", "cfscript/GizmoFactory.cfc", `import "|cfscript.Gizmo";`, "cfscript/Gizmo.cfc"},
	{"cfscript/isInstanceOf", "cfscript/GizmoFactory.cfc", `isInstanceOf(other, "|cfscript.Gizmo")`, "cfscript/Gizmo.cfc"},
	{"cfscript/extends", "cfscript/Gizmo.cfc", `extends="|cfscript.Base"`, "cfscript/Base.cfc"},
	{"cfscript/implements", "cfscript/GizmoFactory.cfc", `implements="|cfscript.IFactory"`, "cfscript/IFactory.cfc"},

	// method definitions
	{"cfml/method-of-component-var", "cfml/WidgetCallMethods.cfc", `widget.|render()`, "cfml/Widget.cfc"},
	{"cfml/method-within-component", "cfml/WidgetCallMethods.cfc", `id = |generateID()`, "cfml/WidgetCallMethods.cfc"},
	{"cfscript/method-of-component-var", "cfscript/GizmoCallMethods.cfc", `gizmo.|render()`, "cfscript/Gizmo.cfc"},
	{"cfscript/method-within-component", "cfscript/GizmoCallMethods.cfc", `id = |generateID()`, "cfscript/GizmoCallMethods.cfc"},

	// user functions
	{"cfm/user-function", "cfml/userFunctions.cfm", `<cfset |userFunctionFoo()>`, "cfml/userFunctions.cfm"},

	// variable definitions in a component
	{"var/argument", "cfml/VariableDefinitions.cfc", `ref = |argumentVariable`, "cfml/VariableDefinitions.cfc"},
	{"var/local", "cfml/VariableDefinitions.cfc", `ref = |localVariable`, "cfml/VariableDefinitions.cfc"},
	{"var/var", "cfml/VariableDefinitions.cfc", `ref = |varVariable`, "cfml/VariableDefinitions.cfc"},
	{"var/variables", "cfml/VariableDefinitions.cfc", `ref = variables.|variablesVariable`, "cfml/VariableDefinitions.cfc"},

	// variable definitions on a page
	{"cfm/variables-scoped", "cfml/VariableDefinitions.cfm", `ref = variables.|variablesVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/variables-unscoped", "cfml/VariableDefinitions.cfm", `ref = |variablesVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/url-scoped", "cfml/VariableDefinitions.cfm", `ref = url.|urlVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/url-unscoped", "cfml/VariableDefinitions.cfm", `ref = |urlVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/cfparam-scoped", "cfml/VariableDefinitions.cfm", `ref = url.|cfparamUrlVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/cfparam-unscoped", "cfml/VariableDefinitions.cfm", `ref = |cfparamUrlVariable`, "cfml/VariableDefinitions.cfm"},
	{"cfm/cfloop-index-scoped", "cfml/VariableDefinitions.cfm", `ref = variables.|loopIndex`, "cfml/VariableDefinitions.cfm"},
	{"cfm/cfloop-index-unscoped", "cfml/VariableDefinitions.cfm", `ref = |loopIndex`, "cfml/VariableDefinitions.cfm"},

	// global-scope variable definitions
	{"global/application", "cfml/GlobalVariables.cfm", `<cfset ref = application.|applicationVariable>`, "cfml/GlobalVariables.cfm"},
	{"global/request", "cfml/GlobalVariables.cfm", `<cfset ref = request.|requestVariable>`, "cfml/GlobalVariables.cfm"},
	{"global/session", "cfml/GlobalVariables.cfm", `<cfset ref = session.|sessionVariable>`, "cfml/GlobalVariables.cfm"},
	{"global/server", "cfml/GlobalVariables.cfm", `<cfset ref = server.|serverVariable>`, "cfml/GlobalVariables.cfm"},
}

// knownGaps are cases the extension answers and this server does not, each with
// the reason. They are expected to fail, and the test fails if one starts
// passing — a list of known failures that silently absorbs a fix is a list that
// stops meaning anything, and the whole point here is to be able to say what
// enabling the server costs.
//
// All sixteen are one gap: handleDefinition has no variable branch at all. It
// answers components, file paths, and function names; a cursor on a variable
// falls through to the function-name lookup, finds nothing, and returns nil.
// Closing it means resolving an identifier against the enclosing function's
// vars, then the file's, then the declaration sites a `<cfparam>` or a
// `<cfloop index>` creates.
var knownGaps = map[string]string{
	"var/argument":              "no variable branch: <cfargument> declarations are not definition targets",
	"var/local":                 "no variable branch: local.x assignments are not definition targets",
	"var/var":                   "no variable branch: var x assignments are not definition targets",
	"var/variables":             "no variable branch: variables.x assignments are not definition targets",
	"cfm/variables-scoped":      "no variable branch",
	"cfm/variables-unscoped":    "no variable branch, and unscoped needs the scope search order",
	"cfm/url-scoped":            "no variable branch",
	"cfm/url-unscoped":          "no variable branch, and unscoped needs the scope search order",
	"cfm/cfparam-scoped":        "no variable branch: <cfparam name> is not a declaration site",
	"cfm/cfparam-unscoped":      "no variable branch: <cfparam name> is not a declaration site",
	"cfm/cfloop-index-scoped":   "no variable branch: <cfloop index> is not a declaration site",
	"cfm/cfloop-index-unscoped": "no variable branch: <cfloop index> is not a declaration site",
	"global/application":        "no variable branch: application-scope assignments are not definition targets",
	"global/request":            "no variable branch: request-scope assignments are not definition targets",
	"global/session":            "no variable branch: session-scope assignments are not definition targets",
	"global/server":             "no variable branch: server-scope assignments are not definition targets",
}

// conformanceDir is the package's own testdata, not the repo-root testdata every
// other server test reads. The fixtures are a whole second workspace — a
// component tree, an Application.cfc, pages with their own scopes — and dropping
// that into the shared testdata changed what the repo-wide scans find:
// TestReachabilityDoesNotFollowContains failed on the fixture Application.cfc's
// onRequestStart, correctly, because a second application had appeared in the
// tree it walks. Package-local testdata is the Go convention for exactly this.
func conformanceDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "testdata", "conformance")
}

// newConformanceServer indexes the whole fixture workspace, as a session against
// it would. The cases resolve dot-paths like `cfml.Widget` relative to the
// workspace root, so the root is the fixture directory and there are no mappings
// — the extension's workspace has no configuration either, which is what makes
// the two comparable.
func newConformanceServer(t *testing.T) *Server {
	t.Helper()

	srv := newTestServer()
	srv.WorkspaceFolders = []string{conformanceDir()}

	for _, rel := range conformanceFiles(t) {
		openConformanceFile(t, srv, rel)
	}

	return srv
}

// conformanceFiles lists every CFML file in the fixture workspace.
func conformanceFiles(t *testing.T) []string {
	t.Helper()

	var out []string

	root := conformanceDir()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".cfc", ".cfm":
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			out = append(out, filepath.ToSlash(rel))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking fixtures: %v", err)
	}

	sort.Strings(out)

	return out
}

func openConformanceFile(t *testing.T, srv *Server, rel string) uri.URI {
	t.Helper()

	abs := filepath.Join(conformanceDir(), filepath.FromSlash(rel))

	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}

	docURI := uri.URI("file://" + abs)
	content := string(data)
	srv.setDocument(docURI, content)

	pr := parser.Parse(docURI, content, srv.cfResolvers())
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.ComponentRefs)

	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()

	return docURI
}

// cursorPosition turns a line snippet with a `|` into a position, the way the
// extension's findPosition does. The snippet must occur exactly once, so an
// ambiguous one fails loudly here rather than silently testing the wrong place.
func cursorPosition(t *testing.T, content, snippet string) (line, char uint32) {
	t.Helper()

	cursor := strings.Index(snippet, "|")
	if cursor < 0 {
		t.Fatalf("snippet %q has no cursor marker", snippet)
	}

	needle := strings.Replace(snippet, "|", "", 1)
	if n := strings.Count(content, needle); n != 1 {
		t.Fatalf("snippet %q occurs %d times, want exactly 1", needle, n)
	}

	offset := strings.Index(content, needle) + cursor
	line = uint32(strings.Count(content[:offset], "\n"))
	char = uint32(offset - (strings.LastIndex(content[:offset], "\n") + 1))

	return line, char
}

func TestDefinitionConformanceWithTheExtension(t *testing.T) {
	srv := newConformanceServer(t)
	root := conformanceDir()

	for _, c := range conformanceCases {
		t.Run(c.name, func(t *testing.T) {
			docURI := uri.URI("file://" + filepath.Join(root, filepath.FromSlash(c.file)))

			content, ok := srv.getDocument(docURI)
			if !ok {
				t.Fatalf("fixture %s was not indexed", c.file)
			}

			line, char := cursorPosition(t, content, c.at)
			got := conformanceTargets(t, definitionAt(t, srv, docURI, line, char), root)

			reason, isGap := knownGaps[c.name]
			landed := slicesContains(got, c.want)

			if isGap {
				if landed {
					t.Fatalf("known gap now passes — remove %q from knownGaps (was: %s)", c.name, reason)
				}

				t.Skipf("known gap: %s", reason)
			}

			if !landed {
				t.Errorf("want a definition in %s, got %v", c.want, got)
			}
		})
	}
}

// TestKnownGapsAreRealCases keeps the two lists honest: a gap naming a case that
// no longer exists is a line nobody will ever delete, and it would hide the case
// being renamed rather than fixed.
func TestKnownGapsAreRealCases(t *testing.T) {
	known := make(map[string]bool, len(conformanceCases))
	for _, c := range conformanceCases {
		known[c.name] = true
	}

	for name := range knownGaps {
		if !known[name] {
			t.Errorf("knownGaps names %q, which is not a conformance case", name)
		}
	}
}

func conformanceTargets(t *testing.T, result any, root string) []string {
	t.Helper()

	var locs []protocol.Location

	switch v := result.(type) {
	case nil:
		return nil
	case protocol.Location:
		locs = []protocol.Location{v}
	case []protocol.Location:
		locs = v
	default:
		t.Fatalf("unexpected definition result %T", result)
	}

	out := make([]string, 0, len(locs))

	for _, l := range locs {
		rel, err := filepath.Rel(root, l.URI.Path())
		if err != nil {
			rel = string(l.URI)
		}

		out = append(out, filepath.ToSlash(rel))
	}

	return out
}

func slicesContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}

	return false
}

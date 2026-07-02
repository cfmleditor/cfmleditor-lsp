package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// newTestdataServer creates a server configured with the testdata workspace.
func newTestdataServer() *Server {
	srv := newTestServer()
	dir := testdataDir()
	srv.WorkspaceFolders = []string{dir}
	srv.Mappings = map[string]string{
		"models":   filepath.Join(dir, "models"),
		"services": filepath.Join(dir, "services"),
	}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "services.$1", Prefix: "getService"},
		{Match: "_parent", Resolve: "models.Base", Prefix: "_parent"},
	}

	return srv
}

// openTestdataFile reads a file from testdata, sets it as an open document, and indexes it.
func openTestdataFile(t *testing.T, srv *Server, relPath string) uri.URI {
	t.Helper()

	abs := filepath.Join(testdataDir(), relPath)

	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("failed to read %s: %v", relPath, err)
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

// definitionAt calls handleDefinition and returns the result.
func definitionAt(t *testing.T, srv *Server, docURI uri.URI, line, char uint32) any {
	t.Helper()
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     protocol.Position{Line: line, Character: char},
		},
	})

	result, err := srv.handleDefinition(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func assertLocationFile(t *testing.T, result any, expectedFile string) protocol.Location {
	t.Helper()

	if result == nil {
		t.Fatal("expected definition result, got nil")
	}

	loc, ok := result.(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", result)
	}

	if !strings.Contains(string(loc.URI), expectedFile) {
		t.Errorf("expected %s, got %s", expectedFile, loc.URI)
	}

	return loc
}

func TestDefinitionTestdata_MethodViaNew(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 32: "var html = variables.widget.render();" — cursor on "render"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 32, 36), "Widget.cfc")
	if loc.Range.Start.Line != 11 { // function render() is line 11
		t.Errorf("expected line 11, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_MethodViaNewDotted(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 33: "var name = variables.widget.getName();" — cursor on "getName"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 33, 36), "Widget.cfc")
	if loc.Range.Start.Line != 15 { // function getName() is line 15
		t.Errorf("expected line 15, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_MethodViaResolver(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "services/UserService.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 36: "var user = variables.userService.getUser(1);" — cursor on "getUser"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 36, 41), "UserService.cfc")
	if loc.Range.Start.Line != 7 { // function getUser() is line 7 (0-based)
		t.Errorf("expected line 7, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_MethodViaParentResolver(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 40: "var className = variables._parent.getClassName();" — cursor on "getClassName"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 40, 42), "Base.cfc")
	if loc.Range.Start.Line != 6 { // function getClassName() is line 6 (0-based)
		t.Errorf("expected line 6, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_UnqualifiedSameFile(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 43: "var helper = helperMethod();" — cursor on "helperMethod"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 43, 21), "DefinitionTest.cfc")
	if loc.Range.Start.Line != 48 { // function helperMethod() is line 48
		t.Errorf("expected line 48, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ChainedNew(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 54: "var name = new models.Widget("x").getName();" — cursor on "getName"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 54, 42), "Widget.cfc")
	if loc.Range.Start.Line != 15 {
		t.Errorf("expected line 15, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ChainedCreateObject(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 60: 'var name = createObject("component", "models.Widget").getName();' — cursor on "getName"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 60, 62), "Widget.cfc")
	if loc.Range.Start.Line != 15 {
		t.Errorf("expected line 15, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_CfInvokeMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 37: '<cfinvoke component="models.Widget" method="render" ..>' — cursor on "render" in method
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 37, 48), "Widget.cfc")
	if loc.Range.Start.Line != 11 {
		t.Errorf("expected line 11, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_CfInvokeDottedService(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "services/UserService.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 40: '<cfinvoke component="services.UserService" method="listUsers" ..>' — cursor on "listUsers"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 40, 55), "UserService.cfc")
	if loc.Range.Start.Line != 17 { // function listUsers() is line 17 (0-based)
		t.Errorf("expected line 17, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_TagMethodOnVariable(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 20: '<cfset var html = variables.widget.render()>' — cursor on "render"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 20, 43), "Widget.cfc")
	if loc.Range.Start.Line != 11 {
		t.Errorf("expected line 11, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_TagUnqualifiedCall(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 27: '<cfset var helper = helperMethod()>' — cursor on "helperMethod"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 27, 28), "DefinitionTestTag.cfc")
	if loc.Range.Start.Line != 32 { // <cffunction name="helperMethod"> is line 32 (0-based)
		t.Errorf("expected line 32, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_MappingResolution(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/User.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 33: "var name = variables.widget.getName();" tests mapping via models.Widget
	// Let's test via user: Line 36: "var user = variables.userService.getUser(1);"
	// Actually test the mapping by checking createUser resolves to User.cfc methods
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 33, 36), "Widget.cfc")
	_ = loc // mapping resolved models.Widget correctly
}

func TestDefinitionTestdata_CfmResolverMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "services/UserService.cfc")
	docURI := openTestdataFile(t, srv, "definition_test.cfm")

	// Line 4: '<cfset users = userService.listUsers()>' — cursor on "listUsers"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 4, 31), "UserService.cfc")
	if loc.Range.Start.Line != 17 { // function listUsers() is line 17 (0-based)
		t.Errorf("expected line 17, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_CfmCreateObjectMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	docURI := openTestdataFile(t, srv, "definition_test.cfm")

	// Line 18: '<cfset className = base.getClassName()>' — cursor on "getClassName"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 18, 27), "Base.cfc")
	if loc.Range.Start.Line != 6 {
		t.Errorf("expected line 6, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaNew(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 5: "variables.widget = new models.Widget("test");" — cursor on "models.Widget"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 5, 30), "Widget.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaCreateObject(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTest.cfc")

	// Line 8: 'variables.user = createObject("component", "models.User");' — cursor on "models.User"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 8, 52), "User.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaExtends(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "models/Widget.cfc")

	// Line 0: 'component extends="models.Base" {' — cursor on "models.Base"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 0, 22), "Base.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaTagCfInvokeComponent(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 37: '<cfinvoke component="models.Widget" method="render"..>' — cursor on "models.Widget" in component attr
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 37, 28), "Widget.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaTagCreateObject(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 3: '<cfset variables.widget = createObject("component", "models.Widget")>' — cursor on "models.Widget"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 3, 57), "Widget.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_ComponentPathViaNewInTag(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 6: '<cfset variables.user = new models.User(1, "test", "test@test.com")>' — cursor on "models.User"
	loc := assertLocationFile(t, definitionAt(t, srv, docURI, 6, 35), "User.cfc")
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected line 0, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionTestdata_CfIncludeTemplate(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "filepath_test.cfm")

	// Line 0: '<cfinclude template="includes/header.cfm">' — cursor on path
	result := definitionAt(t, srv, docURI, 0, 25)
	assertLocationFile(t, result, "header.cfm")
}

func TestDefinitionTestdata_Href(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "filepath_test.cfm")

	// Line 1: '<a href="definition_test.cfm">link</a>' — cursor on path
	result := definitionAt(t, srv, docURI, 1, 14)
	assertLocationFile(t, result, "definition_test.cfm")
}

func TestDefinitionTestdata_FormAction(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "filepath_test.cfm")

	// Line 2: '<form action="includes/process.cfm">' — cursor on path
	result := definitionAt(t, srv, docURI, 2, 20)
	assertLocationFile(t, result, "process.cfm")
}

func TestDefinitionTestdata_CfModuleTemplate(t *testing.T) {
	srv := newTestdataServer()
	docURI := openTestdataFile(t, srv, "filepath_test.cfm")

	// Line 4: '<cfmodule template="includes/header.cfm">' — cursor on path
	result := definitionAt(t, srv, docURI, 4, 25)
	assertLocationFile(t, result, "header.cfm")
}

func TestDefinitionTestdata_UnqualifiedInherited(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "ExtendsTest.cfc")

	// Line 4: "var html = render();" — inherited from Widget
	assertLocationFile(t, definitionAt(t, srv, docURI, 4, 20), "Widget.cfc")
}

func TestDefinitionTestdata_UnqualifiedInheritedGrandparent(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "ExtendsTest.cfc")

	// Line 6: "var name = getClassName();" — inherited from Base (grandparent)
	assertLocationFile(t, definitionAt(t, srv, docURI, 6, 20), "Base.cfc")
}

func TestDefinitionTestdata_SuperMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "ExtendsTest.cfc")

	// Line 8: "var s = super.render();" — super resolves to Widget
	assertLocationFile(t, definitionAt(t, srv, docURI, 8, 23), "Widget.cfc")
}

func TestDefinitionTestdata_TypedArgMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "ExtendsTest.cfc")

	// Line 15: "var html = arguments.widget.render();" — typed arg models.Widget
	assertLocationFile(t, definitionAt(t, srv, docURI, 15, 37), "Widget.cfc")
}

func TestDefinitionTestdata_TypedArgInheritedMethod(t *testing.T) {
	srv := newTestdataServer()
	openTestdataFile(t, srv, "models/Base.cfc")
	openTestdataFile(t, srv, "models/Widget.cfc")
	docURI := openTestdataFile(t, srv, "ExtendsTest.cfc")

	// Line 17: "var name = arguments.widget.getClassName();" — inherited from Base
	assertLocationFile(t, definitionAt(t, srv, docURI, 17, 37), "Base.cfc")
}

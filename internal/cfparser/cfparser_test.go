package cfparser

import (
	"slices"
	"testing"

	"go.lsp.dev/uri"
)

const testURI = uri.URI("file:///test.cfc")

// --- ClassifyRegions tests ---

func TestClassifyRegions_ScriptFile(t *testing.T) {
	content := "component {\n\tpublic function init() {}\n}"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionScript {
		t.Error("expected RegionScript")
	}
}

func TestClassifyRegions_TagFile(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"test\">\n</cffunction>\n</cfcomponent>"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionTag {
		t.Error("expected RegionTag")
	}
}

func TestClassifyRegions_MixedTagWithCFScript(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"tagFunc\">\n</cffunction>\n<cfscript>\nfunction scriptFunc() {}\n</cfscript>\n</cfcomponent>"
	regions := ClassifyRegions(content)
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions, got %d", len(regions))
	}
	if regions[0].Kind != RegionTag {
		t.Errorf("region 0: expected Tag, got %v", regions[0].Kind)
	}
	if regions[1].Kind != RegionScript {
		t.Errorf("region 1: expected Script, got %v", regions[1].Kind)
	}
	if regions[2].Kind != RegionTag {
		t.Errorf("region 2: expected Tag, got %v", regions[2].Kind)
	}
}

func TestClassifyRegions_CommentBeforeComponent(t *testing.T) {
	content := "<!--- file header --->\ncomponent {\nfunction init() {}\n}"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionScript {
		t.Error("expected RegionScript when comment precedes component keyword")
	}
}

// --- ParseFunctionDefs tests ---

func TestParseFunctionDefs_ScriptCFC(t *testing.T) {
	content := "component {\n\tpublic string function getUser() {}\n\tprivate function save() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"getUser", "save"})
}

func TestParseFunctionDefs_TagCFC(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"getUser\">\n</cffunction>\n<cffunction name=\"save\">\n</cffunction>\n</cfcomponent>"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"getUser", "save"})
}

func TestParseFunctionDefs_MixedTagAndCFScript(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"tagFunc\">\n</cffunction>\n<cfscript>\nfunction scriptFunc() {}\n</cfscript>\n</cfcomponent>"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"tagFunc", "scriptFunc"})
}

func TestParseFunctionDefs_CommentedOutFunction(t *testing.T) {
	content := "component {\n<!--- function hidden() {} --->\nfunction visible() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_BlockCommentedFunction(t *testing.T) {
	content := "component {\n/* function hidden() {} */\nfunction visible() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_LineCommentedFunction(t *testing.T) {
	content := "component {\n// function hidden() {}\nfunction visible() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_LineNumbers(t *testing.T) {
	content := "component {\n\n\tfunction first() {}\n\n\tfunction second() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if defs[0].Line != 2 {
		t.Errorf("first func line = %d, want 2", defs[0].Line)
	}
	if defs[1].Line != 4 {
		t.Errorf("second func line = %d, want 4", defs[1].Line)
	}
}

func TestParseFunctionDefs_CFScriptLineNumbers(t *testing.T) {
	content := "<cfcomponent>\n<cfscript>\n\nfunction myFunc() {}\n</cfscript>\n</cfcomponent>"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Line != 3 {
		t.Errorf("func line = %d, want 3", defs[0].Line)
	}
}

func TestParseFunctionDefs_MultilineComment(t *testing.T) {
	content := "component {\n/*\nfunction hidden() {}\n*/\nfunction visible() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_InterfaceFile(t *testing.T) {
	content := "interface {\n\tpublic function getData();\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"getData"})
}

func TestParseFunctionDefs_PropertyStart(t *testing.T) {
	content := "property name=\"id\" type=\"numeric\";\ncomponent {\n\tfunction init() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"init"})
}

func TestParseFunctionDefs_EmptyFile(t *testing.T) {
	defs := ParseFunctionDefs(testURI, "")
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
}

func TestParseFunctionDefs_CommentOnlyFile(t *testing.T) {
	defs := ParseFunctionDefs(testURI, "<!--- just a comment --->")
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
}

func TestParseFunctionDefs_CommentPreservesLineNumbers(t *testing.T) {
	content := "component {\n<!--- \nmultiline\ncomment\n--->\nfunction afterComment() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Line != 5 {
		t.Errorf("func line = %d, want 5", defs[0].Line)
	}
}

func TestParseFunctionDefs_ScriptArgs(t *testing.T) {
	content := "component {\nfunction save(required string name, numeric age, flag) {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" || !args[0].Required {
		t.Errorf("arg 0 = %+v, want {name, string, required}", args[0])
	}
	if args[1].Name != "age" || args[1].Type != "numeric" || args[1].Required {
		t.Errorf("arg 1 = %+v, want {age, numeric, not required}", args[1])
	}
	if args[2].Name != "flag" || args[2].Type != "" || args[2].Required {
		t.Errorf("arg 2 = %+v, want {flag, \"\", not required}", args[2])
	}
}

func TestParseFunctionDefs_ScriptArgsWithDefaults(t *testing.T) {
	content := "component {\nfunction init(string name = \"test\", numeric count = 0) {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" {
		t.Errorf("arg 0 = %+v", args[0])
	}
	if args[1].Name != "count" || args[1].Type != "numeric" {
		t.Errorf("arg 1 = %+v", args[1])
	}
}

func TestParseFunctionDefs_ScriptNoArgs(t *testing.T) {
	content := "component {\nfunction init() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if len(defs[0].Arguments) != 0 {
		t.Errorf("expected 0 args, got %d", len(defs[0].Arguments))
	}
}

func TestParseFunctionDefs_TagArgs(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="save">
	<cfargument name="name" type="string" required="true">
	<cfargument name="age" type="numeric">
	<cfargument name="flag">
</cffunction>
</cfcomponent>`
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" || !args[0].Required {
		t.Errorf("arg 0 = %+v", args[0])
	}
	if args[1].Name != "age" || args[1].Type != "numeric" || args[1].Required {
		t.Errorf("arg 1 = %+v", args[1])
	}
	if args[2].Name != "flag" || args[2].Type != "" || args[2].Required {
		t.Errorf("arg 2 = %+v", args[2])
	}
}

func TestParseFunctionDefs_TagArgsMultipleFunctions(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="first">
	<cfargument name="a">
</cffunction>
<cffunction name="second">
	<cfargument name="b">
	<cfargument name="c">
</cffunction>
</cfcomponent>`
	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if len(defs[0].Arguments) != 1 || defs[0].Arguments[0].Name != "a" {
		t.Errorf("first func args = %+v", defs[0].Arguments)
	}
	if len(defs[1].Arguments) != 2 || defs[1].Arguments[0].Name != "b" || defs[1].Arguments[1].Name != "c" {
		t.Errorf("second func args = %+v", defs[1].Arguments)
	}
}

func TestParseFunctionDefs_NestedCFMLComment(t *testing.T) {
	content := "component {\n<!--- outer <!--- function hidden() {} ---> still comment --->\nfunction visible() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"visible"})
}

// --- ParseComponentRefs tests ---

func TestParseComponentRefs_NewWithParens(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component { myObj = new models.User() }`)
	assertRef(t, refs, 0, "myObj", "models.User")
}

func TestParseComponentRefs_NewWithoutParens(t *testing.T) {
	refs := ParseComponentRefs(testURI, "component {\nmyObj = new models.User\n}")
	assertRef(t, refs, 0, "myObj", "models.User")
}

func TestParseComponentRefs_NewQuotedPath(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component { x = new "dir.Entity"() }`)
	assertRef(t, refs, 0, "x", "dir.Entity")
}

func TestParseComponentRefs_CreateObject(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component { svc = CreateObject("component", "services.OrderService") }`)
	assertRef(t, refs, 0, "svc", "services.OrderService")
}

func TestParseComponentRefs_EntityNew(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component { user = entityNew("User") }`)
	assertRef(t, refs, 0, "user", "User")
}

func TestParseComponentRefs_CfObject(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfobject component="dir.Entity" name="obj">`)
	assertRef(t, refs, 0, "obj", "dir.Entity")
}

func TestParseComponentRefs_CfObjectReversed(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfobject name="obj" component="dir.Entity">`)
	assertRef(t, refs, 0, "obj", "dir.Entity")
}

func TestParseComponentRefs_CfInvoke(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfinvoke component="svc.Helper" method="init" returnvariable="h">`)
	assertRef(t, refs, 0, "h", "svc.Helper")
}

func TestParseComponentRefs_CfInvokeReversed(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfinvoke returnvariable="h" method="init" component="svc.Helper">`)
	assertRef(t, refs, 0, "h", "svc.Helper")
}

func TestParseComponentRefs_Multiple(t *testing.T) {
	src := "component {\na = new foo.Bar()\nb = createObject(\"component\",\"baz.Qux\")\nc = new Simple\n}"
	refs := ParseComponentRefs(testURI, src)
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
}

func TestParseComponentRefs_NoMatch(t *testing.T) {
	refs := ParseComponentRefs(testURI, "component {\nx = 42\ny = someFunction()\n}")
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %d", len(refs))
	}
}

// --- Variable definition tests ---

func TestGlobalVars_FileScope(t *testing.T) {
	src := "var x = 1\nvar y = 2\nz = x + y"
	vars := GlobalVars(src)
	assertContains(t, vars, "x", "y", "z")
}

func TestVarsInFunc_LocalOnly(t *testing.T) {
	src := "component {\nfunction doStuff() {\n\tvar localVar = 2\n\tlocal.other = 3\n}\n}"
	scopes := FindFuncScopes(src)
	if len(scopes) == 0 {
		t.Fatal("expected at least 1 func scope")
	}
	vars := VarsInFunc(src, scopes[0].Start, scopes[0].End)
	assertContains(t, vars, "localVar", "other")
}

func TestGlobalVars_VariablesAndThis(t *testing.T) {
	src := "variables.config = {}\nthis.name = \"test\"\nfunction init() {\n\tvar x = 1\n}"
	vars := GlobalVars(src)
	assertContains(t, vars, "config", "name")
}

func TestGlobalVars_PlainAssignIsVariablesScope(t *testing.T) {
	src := "result = query()\nfunction doStuff() {\n\tvar local1 = 1\n}"
	vars := GlobalVars(src)
	assertContains(t, vars, "result")
}

func TestVarsInFunc_Arguments(t *testing.T) {
	src := "component {\nfunction doStuff() {\n\targuments.id = 1\n}\n}"
	scopes := FindFuncScopes(src)
	if len(scopes) == 0 {
		t.Fatal("expected at least 1 func scope")
	}
	vars := VarsInFunc(src, scopes[0].Start, scopes[0].End)
	assertContains(t, vars, "id")
}

func TestVarsInFunc_TagFunction(t *testing.T) {
	src := "<cfset var pageVar = 1>\n<cffunction name=\"myFunc\">\n\t<cfset var localVar = 2>\n\t<cfset local.other = 3>\n</cffunction>"
	scopes := FindFuncScopes(src)
	if len(scopes) == 0 {
		t.Fatal("expected at least 1 func scope")
	}
	vars := VarsInFunc(src, scopes[0].Start, scopes[0].End)
	assertContains(t, vars, "localVar", "other")
	assertNotContains(t, vars, "pageVar")

	globals := GlobalVars(src)
	assertContains(t, globals, "pageVar")
	assertNotContains(t, globals, "localVar", "other")
}

// --- Helpers ---

func assertDefs(t *testing.T, defs []FunctionDef, want []string) {
	t.Helper()
	if len(defs) != len(want) {
		names := make([]string, len(defs))
		for i, d := range defs {
			names[i] = d.Name
		}
		t.Fatalf("got %d defs %v, want %d %v", len(defs), names, len(want), want)
	}
	for i, d := range defs {
		if d.Name != want[i] {
			t.Errorf("def[%d].Name = %q, want %q", i, d.Name, want[i])
		}
	}
}

func assertRef(t *testing.T, refs []ComponentRef, idx int, variable, component string) {
	t.Helper()
	if len(refs) <= idx {
		t.Fatalf("expected at least %d refs, got %d", idx+1, len(refs))
	}
	if refs[idx].Variable != variable || refs[idx].Component != component {
		t.Errorf("ref[%d]: got Variable=%q Component=%q, want %q %q",
			idx, refs[idx].Variable, refs[idx].Component, variable, component)
	}
}

func assertContains(t *testing.T, vars []string, expected ...string) {
	t.Helper()
	for _, e := range expected {
		if !slices.Contains(vars, e) {
			t.Errorf("expected %q in %v", e, vars)
		}
	}
}

func assertNotContains(t *testing.T, vars []string, unexpected ...string) {
	t.Helper()
	for _, u := range unexpected {
		if slices.Contains(vars, u) {
			t.Errorf("unexpected %q in %v", u, vars)
		}
	}
}

func TestParseComponentRefs_CreateObjectInit(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component {
		var persist = createObject("component", "persist").init(parent=VARIABLES._parent)
	}`)
	assertRef(t, refs, 0, "persist", "persist")
}

func TestParseComponentRefs_CreateObjectInitTag(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfset VARIABLES.persist = createObject("component","persist").init(parent=VARIABLES._parent) />`)
	assertRef(t, refs, 0, "persist", "persist")
}

func TestParseComponentRefs_CreateObjectInitScriptDot(t *testing.T) {
	refs := ParseComponentRefs(testURI, `component {
		VARIABLES.persist = createObject("component", "persist").init(parent=VARIABLES._parent)
	}`)
	assertRef(t, refs, 0, "persist", "persist")
}

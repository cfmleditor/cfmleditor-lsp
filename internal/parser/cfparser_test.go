package parser

import (
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
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

// TestClassifyRegions_ScriptTagNoCF_IsSkipped covers a plain HTML <script>
// block (not <cfscript>) with no CFML tags inside — it should be emitted as
// RegionSkip so its JavaScript is never scanned as CFML (e.g. a jQuery call
// like "$matrix.attr(...)" mistaken for an unresolved CFML method call).
func TestClassifyRegions_ScriptTagNoCF_IsSkipped(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"test\">\n</cffunction>\n<script>\nvar $matrix = $form.find('x');\n$matrix.attr('y', 'z');\n</script>\n</cfcomponent>"

	regions := ClassifyRegions(content)

	var found bool

	for _, r := range regions {
		if r.Kind == RegionSkip {
			found = true

			if !strings.Contains(r.Text, "$matrix") {
				t.Errorf("expected skip region to contain the script body, got %q", r.Text)
			}
		}
	}

	if !found {
		t.Errorf("expected a RegionSkip for the <script> block, got %+v", regions)
	}
}

// TestClassifyRegions_ScriptTagWithCF_NotSkipped covers the escape hatch: a
// <script> block that itself contains a CF tag (e.g. a dynamically generated
// <script> using <cfoutput>) must NOT be skipped, so the real CFML inside it
// still gets parsed normally.
func TestClassifyRegions_ScriptTagWithCF_NotSkipped(t *testing.T) {
	content := "<cfcomponent>\n<script>\n<cfoutput>var x = 1;</cfoutput>\n</script>\n</cfcomponent>"

	regions := ClassifyRegions(content)

	for _, r := range regions {
		if r.Kind == RegionSkip {
			t.Errorf("expected no RegionSkip when the <script> block contains a CF tag, got %+v", regions)
		}
	}
}

// TestIsScriptFile_HTMLWithScriptTag_NotTreatedAsScript covers a .cfm file
// that is pure HTML/JavaScript with zero CF tags anywhere (a plain dialog
// template, say) — isScriptFile's "no CF tags found → treat as script"
// fallback previously misclassified the whole file as CFScript, feeding raw
// JavaScript into the CFML scanner. A literal <script> tag is strong enough
// evidence to not take that fallback.
func TestIsScriptFile_HTMLWithScriptTag_NotTreatedAsScript(t *testing.T) {
	content := "<html>\n<body>\n<script>\nvar $matrix = $form.find('x');\n$matrix.attr('y', 'z');\n</script>\n</body>\n</html>"

	if isScriptFile(content) {
		t.Error("expected isScriptFile to be false for an HTML file with a <script> tag and no CF tags")
	}

	regions := ClassifyRegions(content)

	var found bool

	for _, r := range regions {
		if r.Kind == RegionSkip {
			found = true
		}

		if r.Kind == RegionScript {
			t.Errorf("expected no RegionScript (raw JS parsed as CFScript), got %+v", regions)
		}
	}

	if !found {
		t.Errorf("expected the <script> block to be a RegionSkip, got %+v", regions)
	}
}

// TestParse_ScriptTagNoCF_NoBogusCallSites is an end-to-end check that a
// jQuery-style call inside a plain <script> block never becomes a CallSite
// or ComponentRef — i.e. it won't show up as an "unresolved call" false
// positive the way "$matrix.attr(...)" did before this fix.
func TestParse_ScriptTagNoCF_NoBogusCallSites(t *testing.T) {
	content := "<html>\n<body>\n<script>\nvar $matrix = $form.find('x');\n$matrix.attr('y', 'z');\n</script>\n</body>\n</html>"

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "$matrix" {
			t.Errorf("expected no ComponentRef for $matrix, got %+v", ref)
		}
	}

	for _, call := range pr.FuncCalls(0, 10) {
		if call.Variable == "$matrix" {
			t.Errorf("expected no CallSite for $matrix, got %+v", call)
		}
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

func TestClassifyRegions_CFScriptInsideComment(t *testing.T) {
	content := "<cfcomponent>\n<!---\n<cfscript>\nvar x = obj.foo();\n</cfscript>\n--->\n</cfcomponent>"

	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region (all tag), got %d", len(regions))
	}

	if regions[0].Kind != RegionTag {
		t.Errorf("expected RegionTag, got %v — <cfscript> inside comment should not create a script region", regions[0].Kind)
	}
}

func TestClassifyRegions_CFScriptAfterComment(t *testing.T) {
	content := "<cfcomponent>\n<!---\n<cfscript>var hidden = 1;</cfscript>\n--->\n<cfscript>\nfunction realFunc() {}\n</cfscript>\n</cfcomponent>"

	regions := ClassifyRegions(content)
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions (tag, script, tag), got %d", len(regions))
	}

	if regions[0].Kind != RegionTag {
		t.Errorf("region 0: expected RegionTag, got %v", regions[0].Kind)
	}

	if regions[1].Kind != RegionScript {
		t.Errorf("region 1: expected RegionScript, got %v", regions[1].Kind)
	}

	if !strings.Contains(regions[1].Text, "realFunc") {
		t.Errorf("region 1: expected real function body, got %q", regions[1].Text)
	}

	if regions[2].Kind != RegionTag {
		t.Errorf("region 2: expected RegionTag, got %v", regions[2].Kind)
	}
}

func TestClassifyRegions_NestedCommentWithCFScript(t *testing.T) {
	content := "<cfcomponent>\n<!---\nouter comment\n<!--- <cfscript>var x=1;</cfscript> --->\nstill in outer\n--->\n</cfcomponent>"

	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region (all tag), got %d", len(regions))
	}

	if regions[0].Kind != RegionTag {
		t.Errorf("expected RegionTag, got %v — nested commented <cfscript> should not create script region", regions[0].Kind)
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

func TestParseComponentRefs_ThisAssignmentScript(t *testing.T) {
	refs := ParseComponentRefs(testURI, "component {\n\tVARIABLES.self = this;\n}")
	assertRef(t, refs, 0, "self", "/test.cfc")
}

func TestParseComponentRefs_ThisAssignmentTag(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfcomponent><cfset VARIABLES.prs = this></cfcomponent>`)
	assertRef(t, refs, 0, "prs", "/test.cfc")
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

func TestTagParser_RefClassification(t *testing.T) {
	content := `<cfcomponent>
<cfset VARIABLES.globalService = createObject("component","services.GlobalService") />
<cffunction name="init">
	<cfset VARIABLES.persist = createObject("component","persist").init() />
	<cfset var localService = createObject("component","services.LocalService") />
	<cfset localService = createObject("component","services.ReassignedLocal") />
	<cfset var result = ArrayNew(1) />
</cffunction>
<cffunction name="other">
	<cfset var otherLocal = createObject("component","services.OtherLocal") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// ComponentRefs should contain VARIABLES-scoped refs only
	var compRefVars []string
	for _, ref := range pr.ComponentRefs {
		compRefVars = append(compRefVars, ref.Variable)
	}

	if !slices.Contains(compRefVars, "globalService") {
		t.Errorf("expected globalService in ComponentRefs, got %v", compRefVars)
	}

	if !slices.Contains(compRefVars, "persist") {
		t.Errorf("expected persist in ComponentRefs (VARIABLES.persist), got %v", compRefVars)
	}

	if slices.Contains(compRefVars, "localService") {
		t.Errorf("localService should NOT be in ComponentRefs (it's var'd), got %v", compRefVars)
	}

	if slices.Contains(compRefVars, "otherLocal") {
		t.Errorf("otherLocal should NOT be in ComponentRefs, got %v", compRefVars)
	}

	// FuncComponentRefs for init should have localService
	initScope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(initScope.Start, initScope.End)

	var funcRefVars []string
	for _, ref := range funcRefs {
		funcRefVars = append(funcRefVars, ref.Variable)
	}

	if !slices.Contains(funcRefVars, "localService") {
		t.Errorf("expected localService in init FuncComponentRefs, got %v", funcRefVars)
	}

	// Reassigned local should also be in funcRefs (var'd earlier)
	if !slices.Contains(funcRefVars, "localService") {
		t.Errorf("expected reassigned localService in init FuncComponentRefs, got %v", funcRefVars)
	}

	// FuncComponentRefs for other should have otherLocal
	otherScope := pr.Scopes[1]
	otherRefs := pr.FuncComponentRefs(otherScope.Start, otherScope.End)

	var otherRefVars []string
	for _, ref := range otherRefs {
		otherRefVars = append(otherRefVars, ref.Variable)
	}

	if !slices.Contains(otherRefVars, "otherLocal") {
		t.Errorf("expected otherLocal in other FuncComponentRefs, got %v", otherRefVars)
	}

	// localService from init should NOT leak into other
	if slices.Contains(otherRefVars, "localService") {
		t.Errorf("localService should NOT be in other FuncComponentRefs, got %v", otherRefVars)
	}
}

func TestTagParser_VarShadowsComponentRef(t *testing.T) {
	content := `<cfcomponent>
<cfset VARIABLES.result = createObject("component","services.ResultService") />
<cffunction name="doStuff">
	<cfset var result = ArrayNew(1) />
	<cfset result = createObject("component","services.Overridden") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// ComponentRefs should have result -> ResultService (from VARIABLES.result)
	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "result" && ref.Component == "services.ResultService" {
			found = true
		}

		if ref.Variable == "result" && ref.Component == "services.Overridden" {
			t.Error("Overridden should NOT be in ComponentRefs (result is var'd in function)")
		}
	}

	if !found {
		t.Error("expected result -> services.ResultService in ComponentRefs")
	}

	// FuncComponentRefs should have the overridden one
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var funcFound bool

	for _, ref := range funcRefs {
		if ref.Variable == "result" && ref.Component == "services.Overridden" {
			funcFound = true
		}
	}

	if !funcFound {
		t.Error("expected result -> services.Overridden in FuncComponentRefs")
	}
}

func TestScriptParser_RefClassification(t *testing.T) {
	content := `component {
	variables.globalService = createObject("component","services.GlobalService");

	function init() {
		variables.persist = createObject("component","persist");
		var localService = createObject("component","services.LocalService");
		localService = createObject("component","services.ReassignedLocal");
		var result = ArrayNew(1);
	}

	function other() {
		var otherLocal = createObject("component","services.OtherLocal");
	}
}`
	pr := Parse(testURI, content)

	// ComponentRefs should contain VARIABLES-scoped refs only
	var compRefVars []string
	for _, ref := range pr.ComponentRefs {
		compRefVars = append(compRefVars, ref.Variable)
	}

	if !slices.Contains(compRefVars, "globalService") {
		t.Errorf("expected globalService in ComponentRefs, got %v", compRefVars)
	}

	if !slices.Contains(compRefVars, "persist") {
		t.Errorf("expected persist in ComponentRefs (variables.persist), got %v", compRefVars)
	}

	if slices.Contains(compRefVars, "localService") {
		t.Errorf("localService should NOT be in ComponentRefs (it's var'd), got %v", compRefVars)
	}

	if slices.Contains(compRefVars, "otherLocal") {
		t.Errorf("otherLocal should NOT be in ComponentRefs, got %v", compRefVars)
	}

	// FuncComponentRefs for init should have localService
	initScope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(initScope.Start, initScope.End)

	var funcRefVars []string
	for _, ref := range funcRefs {
		funcRefVars = append(funcRefVars, ref.Variable)
	}

	if !slices.Contains(funcRefVars, "localService") {
		t.Errorf("expected localService in init FuncComponentRefs, got %v", funcRefVars)
	}

	// FuncComponentRefs for other should have otherLocal
	otherScope := pr.Scopes[1]
	otherRefs := pr.FuncComponentRefs(otherScope.Start, otherScope.End)

	var otherRefVars []string
	for _, ref := range otherRefs {
		otherRefVars = append(otherRefVars, ref.Variable)
	}

	if !slices.Contains(otherRefVars, "otherLocal") {
		t.Errorf("expected otherLocal in other FuncComponentRefs, got %v", otherRefVars)
	}

	if slices.Contains(otherRefVars, "localService") {
		t.Errorf("localService should NOT be in other FuncComponentRefs, got %v", otherRefVars)
	}
}

func TestScriptParser_UnscopedAssignRouting(t *testing.T) {
	content := `component {
	function doStuff() {
		var x = new services.Local();
		y = new services.Global();
		x = new services.StillLocal();
	}
}`
	pr := Parse(testURI, content)

	// y should be in ComponentRefs (unscoped, not var'd → global)
	var compRefVars []string
	for _, ref := range pr.ComponentRefs {
		compRefVars = append(compRefVars, ref.Variable)
	}

	if !slices.Contains(compRefVars, "y") {
		t.Errorf("expected y in ComponentRefs (unscoped not var'd), got %v", compRefVars)
	}

	if slices.Contains(compRefVars, "x") {
		t.Errorf("x should NOT be in ComponentRefs (var'd), got %v", compRefVars)
	}

	// x should be in FuncComponentRefs
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var funcRefVars []string
	for _, ref := range funcRefs {
		funcRefVars = append(funcRefVars, ref.Variable)
	}

	if !slices.Contains(funcRefVars, "x") {
		t.Errorf("expected x in FuncComponentRefs, got %v", funcRefVars)
	}

	if slices.Contains(funcRefVars, "y") {
		t.Errorf("y should NOT be in FuncComponentRefs (not var'd), got %v", funcRefVars)
	}
}

func TestFunctionDef_ReturnType(t *testing.T) {
	content := `component {
	public models.User function getUser() {
		return new models.User();
	}

	function createService() {
		return createObject("component", "services.UserService");
	}

	function loadEntity() {
		return entityNew("Order");
	}

	string function getName() {
		return "hello";
	}
}`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 4 {
		t.Fatalf("expected 4 funcs, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnType != "models.User" {
		t.Errorf("expected ReturnType 'models.User', got %q", pr.Funcs[0].ReturnType)
	}

	if pr.Funcs[0].ReturnComponent != "models.User" {
		t.Errorf("expected ReturnComponent 'models.User', got %q", pr.Funcs[0].ReturnComponent)
	}

	if pr.Funcs[1].ReturnComponent != "services.UserService" {
		t.Errorf("expected ReturnComponent 'services.UserService', got %q", pr.Funcs[1].ReturnComponent)
	}

	if pr.Funcs[2].ReturnComponent != "Order" {
		t.Errorf("expected ReturnComponent 'Order', got %q", pr.Funcs[2].ReturnComponent)
	}

	if pr.Funcs[3].ReturnType != "string" {
		t.Errorf("expected ReturnType 'string', got %q", pr.Funcs[3].ReturnType)
	}

	if pr.Funcs[3].ReturnComponent != "" {
		t.Errorf("expected empty ReturnComponent, got %q", pr.Funcs[3].ReturnComponent)
	}
}

func TestFunctionDef_ReturnType_Tag(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getUser" returntype="models.User">
	<cfreturn createObject("component","models.User") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnType != "models.User" {
		t.Errorf("expected ReturnType 'models.User', got %q", pr.Funcs[0].ReturnType)
	}
}

func TestFunctionDef_ReturnVarResolution(t *testing.T) {
	content := `component {
	function getService() {
		var svc = new services.UserService();
		return svc;
	}
}`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnComponent != "services.UserService" {
		t.Errorf("expected ReturnComponent 'services.UserService', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestPendingCalls_SameFileResolution(t *testing.T) {
	content := `component {
	models.User function getUser() {
		return new models.User();
	}

	function doStuff() {
		var user = getUser();
		return user;
	}
}`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(pr.Funcs))
	}

	// doStuff's funcRefs should have user → models.User
	scope := pr.Scopes[1]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range funcRefs {
		if ref.Variable == "user" && ref.Component == "models.User" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected user → models.User in doStuff FuncComponentRefs, got %v", funcRefs)
	}

	// doStuff should also have ReturnComponent resolved via return var
	if pr.Funcs[1].ReturnComponent != "models.User" {
		t.Errorf("expected doStuff ReturnComponent 'models.User', got %q", pr.Funcs[1].ReturnComponent)
	}
}

func TestFunctionDef_ReturnType_ResolvesChain(t *testing.T) {
	// Simulates java stub pattern: getInstance() returns same component type
	content := `component {
	stubs.Signature function getInstance(required string algorithm) {}
	function initSign(required any privateKey) {}
	function sign() {}
}`
	stubURI := uri.URI("file:///stubs/Signature.cfc")
	pr := Parse(stubURI, content)

	if len(pr.Funcs) != 3 {
		t.Fatalf("expected 3 funcs, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnType != "stubs.Signature" {
		t.Errorf("expected ReturnType 'stubs.Signature', got %q", pr.Funcs[0].ReturnType)
	}
}

func TestPendingCalls_ReturnTypeFromCalledFunction(t *testing.T) {
	// When a function has a component-like return type, callers should get a ref
	content := `component {
	stubs.Signature function getInstance(required string algorithm) {}
	function initSign() {}
	function sign() {}

	function doWork() {
		var sig = getInstance("SHA256");
		sig.initSign();
		sig.sign();
	}
}`
	pr := Parse(testURI, content)

	// sig should resolve to stubs.Signature via getInstance's ReturnType
	scope := pr.Scopes[3] // doWork
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range funcRefs {
		if ref.Variable == "sig" && ref.Component == "stubs.Signature" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected sig → stubs.Signature in doWork FuncComponentRefs, got %v", funcRefs)
	}
}

// TestParseBodyVarDecl_BuiltinReturnLookup covers "var x = someBuiltinFunc(...)"
// assigned inside a function body (parseBodyVarDecl), not at component/global
// scope (checkVarRHS). Before the fix, only checkVarRHS consulted
// BuiltinReturnLookup — parseBodyVarDecl fell straight to an unresolved
// pendingCall, so e.g. "var s3Service = getCloudService(...)" inside a function
// never got typed, and a further "var s3Bucket = s3Service.root(...)" couldn't
// inherit it either.
func TestParseBodyVarDecl_BuiltinReturnLookup(t *testing.T) {
	content := `component {
	function work() {
		var s3Service = getCloudService(cred, conf);
		var s3Bucket = s3Service.root(bucket);
	}
}`

	builtinReturnLookup := func(name string) string {
		if name == "getCloudService" {
			return "$builtin.getcloudservice"
		}

		return ""
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		ExtractCalls:        true,
		ScanAllScopes:       true,
		BuiltinReturnLookup: builtinReturnLookup,
	})

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	want := map[string]string{
		"s3Service": "$builtin.getcloudservice",
		"s3Bucket":  "$builtin.getcloudservice",
	}

	for name, wantComp := range want {
		var found string

		for _, ref := range funcRefs {
			if ref.Variable == name {
				found = ref.Component
			}
		}

		if found != wantComp {
			t.Errorf("expected %s -> %s, got %q (funcRefs: %+v)", name, wantComp, found, funcRefs)
		}
	}
}

func TestPendingCalls_MethodCallFallbackToBaseVar(t *testing.T) {
	// x = variables.jss.getInstance() → x gets same component as jss
	content := `component {
	variables.jss = createObject("java","java.security.Signature");

	function doWork() {
		var instance = variables.jss.getInstance("SHA256");
	}
}`
	resolvers := []Resolver{{
		Match:   `createObject("java","java.security.Signature")`,
		Resolve: "stubs.Signature",
		Prefix:  "createObject",
	}}
	pr := Parse(testURI, content, resolvers)

	// jss should be in ComponentRefs → stubs.Signature
	var jssFound bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "jss" && ref.Component == "stubs.Signature" {
			jssFound = true
		}
	}

	if !jssFound {
		t.Fatalf("expected jss → stubs.Signature in ComponentRefs, got %v", pr.ComponentRefs)
	}

	// instance should resolve to same component as jss (fallback)
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var instanceFound bool

	for _, ref := range funcRefs {
		if ref.Variable == "instance" && ref.Component == "stubs.Signature" {
			instanceFound = true
		}
	}

	if !instanceFound {
		t.Errorf("expected instance → stubs.Signature in FuncComponentRefs, got %v", funcRefs)
	}
}

func TestResolverRefs_MultiLineCall(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="doLogin" access="public">
	<cfargument name="context" type="any" />
	<cfset var authtoken = "abc" />
	<cfset var sessionId = "123" />
	<cfset var appId = "app" />

	<cfset var user = VARIABLES._kernel.getUser(
				token=authtoken,
				sessionId=sessionId,
				isDomain=true,
				appId=appId) />

	<cfset var wasloggedin = user.isLoggedIn(
			sessionid=sessionId,
			allowDomainOnly=true) />
</cffunction>
</cfcomponent>`

	resolvers := []Resolver{{
		Match:   "getUser($1)",
		Resolve: "models.User",
		Prefix:  "getUser",
	}}
	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ScanAllScopes: true,
	})

	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var userFound bool

	for _, ref := range funcRefs {
		if ref.Variable == "user" && ref.Component == "models.User" {
			userFound = true
		}
	}

	if !userFound {
		t.Errorf("expected user → models.User in FuncComponentRefs (multi-line call), got %v", funcRefs)
	}
}

func TestExpressionMappings_ReplacesHashExpressions(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="init">
	<cfset variables.runner = CreateObject("component","#VARIABLES._core#update.run") />
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{
		ExpressionMappings: map[string]string{
			"#VARIABLES._core#": "packages.tass.core.",
		},
	})

	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "runner" && ref.Component == "packages.tass.core.update.run" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected runner → packages.tass.core.update.run, got %v", pr.ComponentRefs)
	}
}

// TestExpressionMappings_UnmappedHashExpressionBecomesAny verifies that a "#...#"
// component path with no matching expressionMappings entry (e.g. a runtime-computed
// factory path like CreateObject("component", "tools.templates.#ARGUMENTS.template#.generator"))
// collapses to "$any" instead of leaking the literal "#...#" text as a bogus component
// name — this must hold even with no expressionMappings configured at all.
func TestExpressionMappings_UnmappedHashExpressionBecomesAny(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="init">
	<cfargument name="template" required="true" type="string" />
	<cfset generator = CreateObject("component","tools.templates.#ARGUMENTS.template#.generator").init() />
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{})

	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "generator" {
			found = true

			if ref.Component != "$any" {
				t.Errorf("expected generator → $any, got %q", ref.Component)
			}
		}
	}

	if !found {
		t.Errorf("expected a ComponentRef for generator, got none: %v", pr.ComponentRefs)
	}
}

func TestTagParser_FuncRefsOffset_MixedFile(t *testing.T) {
	// Simulate a mixed tag/script file where tag region starts at a non-zero line
	content := `<cfscript>
// some script
</cfscript>
<cfcomponent>
<cffunction name="doStuff">
	<cfset var svc = CreateObject("component","services.MyService") />
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)

	// Find the doStuff scope and check funcRefs
	for _, s := range pr.Scopes {
		if s.Name == "doStuff" {
			refs := pr.FuncComponentRefs(s.Start, s.End)

			var found bool

			for _, ref := range refs {
				if ref.Variable == "svc" && ref.Component == "services.MyService" {
					found = true
				}
			}

			if !found {
				t.Errorf("expected svc → services.MyService in doStuff FuncComponentRefs, got %v", refs)
			}

			return
		}
	}

	t.Error("doStuff scope not found")
}

func TestTagParser_BaseVarMethodCall(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="test">
	<cfset var client = CreateObject("component","services.Client") />
	<cfset var response = client.doRequest("test") />
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var clientFound, responseFound bool

	for _, ref := range refs {
		if ref.Variable == "client" {
			clientFound = true
		}

		if ref.Variable == "response" && ref.Component == "services.Client" {
			responseFound = true
		}
	}

	if !clientFound {
		t.Errorf("expected client ref in FuncComponentRefs, got %v", refs)
	}

	if !responseFound {
		t.Errorf("expected response → services.Client (baseVar fallback), got %v", refs)
	}
}

func TestSimpleMatch_FirstSuffix(t *testing.T) {
	// Ensure getController("$1") captures only the first arg, not past chained calls
	resolvers := []Resolver{{
		Match:   `getController("$1")`,
		Resolve: "packages.$1.controller",
		Prefix:  "getController",
	}}

	comp := ResolveFromCall(`getController("kiosk").getFilterArray(filterType="students")`, resolvers)
	if comp != "packages.kiosk.controller" {
		t.Errorf("expected packages.kiosk.controller, got %q", comp)
	}
}

func TestTagParser_ReturnComponentInference(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="createDocument" access="public" returntype="any">
	<cfset var result = CreateObject("component","services.LuceneDocument") />
	<cfreturn result />
</cffunction>
<cffunction name="doWork" access="public">
	<cfset var doc = createDocument() />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Funcs) < 2 {
		t.Fatalf("expected 2+ funcs, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnComponent != "services.LuceneDocument" {
		t.Errorf("expected createDocument ReturnComponent 'services.LuceneDocument', got %q", pr.Funcs[0].ReturnComponent)
	}

	// doWork's doc should resolve via pending call
	for _, s := range pr.Scopes {
		if s.Name == "doWork" {
			refs := pr.FuncComponentRefs(s.Start, s.End)

			var found bool

			for _, ref := range refs {
				if ref.Variable == "doc" && ref.Component == "services.LuceneDocument" {
					found = true
				}
			}

			if !found {
				t.Errorf("expected doc → services.LuceneDocument in doWork FuncComponentRefs, got %v", refs)
			}

			return
		}
	}

	t.Error("doWork scope not found")
}

func TestTagParser_CfreturnDirectNew(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getUser" returntype="any">
	<cfreturn new models.User() />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if pr.Funcs[0].ReturnComponent != "models.User" {
		t.Errorf("expected ReturnComponent 'models.User', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestTagParser_CfreturnDirectCreateObject(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getService" returntype="any">
	<cfreturn createObject("component","services.MyService") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if pr.Funcs[0].ReturnComponent != "services.MyService" {
		t.Errorf("expected ReturnComponent 'services.MyService', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestTagParser_CfreturnVarResolution(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="build" returntype="any">
	<cfset var obj = CreateObject("component","services.Builder") />
	<cfreturn obj />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if pr.Funcs[0].ReturnComponent != "services.Builder" {
		t.Errorf("expected ReturnComponent 'services.Builder', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestTagParser_BareFunctionCallResolution(t *testing.T) {
	// x = sameFileFunc() should resolve via ReturnComponent
	content := `<cfcomponent>
<cffunction name="createHelper" returntype="any">
	<cfset var h = CreateObject("component","utils.Helper") />
	<cfreturn h />
</cffunction>
<cffunction name="doWork">
	<cfset var helper = createHelper() />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if pr.Funcs[0].ReturnComponent != "utils.Helper" {
		t.Fatalf("expected createHelper ReturnComponent 'utils.Helper', got %q", pr.Funcs[0].ReturnComponent)
	}

	scope := pr.Scopes[1]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "helper" && ref.Component == "utils.Helper" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected helper → utils.Helper in doWork FuncComponentRefs, got %v", refs)
	}
}

func TestTagParser_ChainedReturnResolution(t *testing.T) {
	// funcA returns component, funcB calls funcA and returns that
	content := `<cfcomponent>
<cffunction name="getDAO" returntype="any">
	<cfset var d = CreateObject("component","dao.UserDAO") />
	<cfreturn d />
</cffunction>
<cffunction name="doWork">
	<cfset var svc = getDAO() />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// getDAO returns dao.UserDAO
	if pr.Funcs[0].ReturnComponent != "dao.UserDAO" {
		t.Fatalf("expected getDAO ReturnComponent 'dao.UserDAO', got %q", pr.Funcs[0].ReturnComponent)
	}

	// doWork's svc should resolve to dao.UserDAO
	for _, sc := range pr.Scopes {
		if sc.Name == "doWork" {
			refs := pr.FuncComponentRefs(sc.Start, sc.End)

			var found bool

			for _, ref := range refs {
				if ref.Variable == "svc" && ref.Component == "dao.UserDAO" {
					found = true
				}
			}

			if !found {
				t.Errorf("expected svc → dao.UserDAO in doWork FuncComponentRefs, got %v", refs)
			}

			return
		}
	}

	t.Error("doWork scope not found")
}

func TestScriptParser_ReturnInsideConditional(t *testing.T) {
	// Multiple return paths — should use the last return seen
	content := `component {
	function getService(required boolean useCache) {
		if (arguments.useCache) {
			return new services.CachedService();
		}
		return new services.LiveService();
	}
}`
	pr := Parse(testURI, content)

	// Should capture one of them (last one wins)
	if pr.Funcs[0].ReturnComponent == "" {
		t.Error("expected ReturnComponent to be set from conditional return")
	}
}

func TestScriptParser_NoReturnComponent(t *testing.T) {
	// Function with no component return — should not crash or set garbage
	content := `component {
	function calculate() {
		var x = 1 + 2;
		return x;
	}
}`
	pr := Parse(testURI, content)

	if pr.Funcs[0].ReturnComponent != "" {
		t.Errorf("expected empty ReturnComponent for numeric return, got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestScriptParser_ReturnThis(t *testing.T) {
	// return this — common builder pattern, should not crash
	content := `component {
	function setName(required string name) {
		variables.name = arguments.name;
		return this;
	}
}`
	pr := Parse(testURI, content)

	// "this" is a keyword, should not set ReturnComponent
	if pr.Funcs[0].ReturnComponent != "" {
		t.Errorf("expected empty ReturnComponent for 'return this', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestPendingCalls_RecursiveDoesNotPanic(t *testing.T) {
	// Function calls itself — should not infinite loop
	content := `component {
	function recurse() {
		var result = recurse();
		return result;
	}
}`

	pr := Parse(testURI, content)
	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}
}

func TestTagParser_CommentedOutCfreturn(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="test">
	<cfset var svc = CreateObject("component","services.Real") />
	<!--- <cfreturn CreateObject("component","services.Fake") /> --->
	<cfreturn svc />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// Should resolve to Real (from svc), not Fake (commented out)
	if pr.Funcs[0].ReturnComponent != "services.Real" {
		t.Errorf("expected ReturnComponent 'services.Real', got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestPendingCalls_EmptyFunctionBody(t *testing.T) {
	content := `component {
	function empty() {}
	function caller() {
		var x = empty();
	}
}`
	pr := Parse(testURI, content)
	// Should not crash, x just won't resolve
	scope := pr.Scopes[1]

	refs := pr.FuncComponentRefs(scope.Start, scope.End)
	if len(refs) != 0 {
		t.Errorf("expected no refs for call to empty function, got %v", refs)
	}
}

func TestTagParser_MultipleReturnsLastWins(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="get">
	<cfset var a = CreateObject("component","services.First") />
	<cfset var b = CreateObject("component","services.Second") />
	<cfreturn a />
	<cfreturn b />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// First cfreturn wins (ReturnComponent is only set if empty)
	if pr.Funcs[0].ReturnComponent != "services.First" {
		t.Errorf("expected ReturnComponent 'services.First' (first cfreturn), got %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestScriptParser_DottedReturnTypeAsRef(t *testing.T) {
	// Declared dotted return type should allow pending call resolution
	content := `component {
	models.User function getUser() {
		return new models.User();
	}

	function work() {
		var u = getUser();
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[1]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "u" && ref.Component == "models.User" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected u → models.User via declared return type, got %v", refs)
	}
}

func TestPendingCalls_ScopePrefix_Variables(t *testing.T) {
	// x = VARIABLES.kernel.getService() — baseVar should be "kernel"
	content := `component {
	variables.kernel = new core.Kernel();

	function work() {
		var svc = variables.kernel.getFactory();
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "svc" && ref.Component == "core.Kernel" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected svc → core.Kernel (baseVar fallback from variables.kernel), got %v", refs)
	}
}

func TestScriptParser_AssignInsideNestedBraces(t *testing.T) {
	// Assignments inside if/for blocks should still be captured
	content := `component {
	function work() {
		var svc = new services.Foo();
		if (true) {
			var inner = new services.Bar();
		}
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var svcFound, innerFound bool

	for _, ref := range refs {
		if ref.Variable == "svc" {
			svcFound = true
		}

		if ref.Variable == "inner" {
			innerFound = true
		}
	}

	if !svcFound {
		t.Errorf("expected svc in FuncComponentRefs, got %v", refs)
	}

	if !innerFound {
		t.Errorf("expected inner in FuncComponentRefs (inside if block), got %v", refs)
	}
}

func TestScriptParser_ClosureDoesNotLeak(t *testing.T) {
	// Variable inside a closure/nested function should not appear in outer funcRefs
	content := `component {
	function outer() {
		var svc = new services.Outer();
		var fn = function() {
			var nested = new services.Nested();
		};
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var svcFound bool

	for _, ref := range refs {
		if ref.Variable == "svc" {
			svcFound = true
		}

		if ref.Variable == "nested" {
			t.Errorf("nested should NOT leak into outer FuncComponentRefs, got %v", refs)
		}
	}

	if !svcFound {
		t.Errorf("expected svc in outer FuncComponentRefs, got %v", refs)
	}
}

func TestTagParser_CfsetWithHashExpression(t *testing.T) {
	// Hash expressions in component path should not crash
	content := `<cfcomponent>
<cffunction name="init">
	<cfset variables.obj = CreateObject("component","#getPath()#.service") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	// Should have a ref with the raw # expression (expression mappings applied separately)
	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "obj" {
			found = true
		}
	}

	if !found {
		t.Error("expected obj in ComponentRefs even with # expression")
	}
}

func TestScriptParser_VarReassignmentKeepsLocal(t *testing.T) {
	// var x = A; x = B; — x should remain in funcRefs (second assign shouldn't go global)
	content := `component {
	function work() {
		var x = new services.First();
		x = new services.Second();
	}
}`
	pr := Parse(testURI, content)

	// Both should be in funcRefs (x is var'd, stays local)
	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	if len(refs) != 2 {
		t.Errorf("expected 2 refs for x (First and Second), got %v", refs)
	}

	// Should NOT be in ComponentRefs
	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "x" {
			t.Errorf("x should NOT be in ComponentRefs (it's var'd), got %v", pr.ComponentRefs)
		}
	}
}

func TestTagParser_SelfClosingCfset(t *testing.T) {
	// <cfset ... /> vs <cfset ...> both work
	content := `<cfcomponent>
<cffunction name="test">
	<cfset var a = CreateObject("component","services.A")>
	<cfset var b = CreateObject("component","services.B") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	if len(refs) != 2 {
		t.Errorf("expected 2 refs (a and b), got %v", refs)
	}
}

func TestResolverMatch_NoFalsePositiveOnSubstring(t *testing.T) {
	// Resolver with prefix "getUser" should not match "getUserDAO"
	resolvers := []Resolver{{
		Match:   "getUser($1)",
		Resolve: "models.User",
		Prefix:  "getUser",
	}}

	// Should match getUser()
	if comp := ResolveFromCall(`getUser()`, resolvers); comp != "models.User" {
		t.Errorf("expected models.User for getUser(), got %q", comp)
	}

	// Should NOT match getUserDAO() — prefix matches but pattern doesn't
	if comp := ResolveFromCall(`getUserDAO()`, resolvers); comp == "models.User" {
		t.Errorf("getUserDAO() should NOT resolve to models.User")
	}
}

func TestScriptParser_ArrowFunctionDoesNotLeak(t *testing.T) {
	// Arrow/lambda: var fn = () => { var x = new Foo(); }
	content := `component {
	function outer() {
		var svc = new services.Outer();
		var arr = [1,2,3].map(function(item) {
			var mapped = new services.Mapper();
			return mapped;
		});
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	for _, ref := range refs {
		if ref.Variable == "mapped" {
			t.Errorf("mapped should NOT leak from inline function, got %v", refs)
		}
	}
}

func TestScriptParser_ChainedNewDotInit(t *testing.T) {
	// var x = new Foo().init() — should still capture Foo
	content := `component {
	function work() {
		var svc = new services.MyService().init();
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "svc" && ref.Component == "services.MyService" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected svc → services.MyService (chained .init()), got %v", refs)
	}
}

func TestTagParser_CfsetNoValue(t *testing.T) {
	// <cfset var x> without assignment — should not crash
	content := `<cfcomponent>
<cffunction name="test">
	<cfset var x>
	<cfset var y = CreateObject("component","services.Y") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "y" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected y in FuncComponentRefs, got %v", refs)
	}
}

func TestScriptParser_CreateObjectJavaWithResolver(t *testing.T) {
	// createObject("java","com.example.Foo") with resolver
	content := `component {
	function work() {
		var obj = createObject("java", "com.example.Foo");
	}
}`
	resolvers := []Resolver{{
		Match:   `createObject("java","$1")`,
		Resolve: "stubs.$1",
		Prefix:  "createObject",
	}}

	pr := Parse(testURI, content, resolvers)

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range refs {
		if ref.Variable == "obj" && ref.Component == "stubs.com.example.Foo" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected obj → stubs.com.example.Foo, got %v", refs)
	}
}

// TestScriptParser_RequestScopedAssignment verifies that "REQUEST.x = document.createTable(1);"
// inside a cfscript function body is recognized as an assignment (not misread as a bare
// call). Before this fix, checkAssignRef's default path only recognized a bare "x = ..."
// (identifier directly followed by "="); for a scope-prefixed LHS the next token is "."
// not "=", so it fell through to checkBareCall and the RHS's component type was silently
// dropped, leaving "REQUEST.x" with no ComponentRef at all.
func TestScriptParser_RequestScopedAssignment(t *testing.T) {
	content := `component {
	function work() {
		REQUEST.objGroupTable = document.createTable(1);
		REQUEST.objGroupTable.setTableWidth('100%');
	}
}`
	resolvers := []Resolver{{
		Match:   `document.createTable($1)`,
		Resolve: "reporting.table",
		Prefix:  "document.createTable",
	}}

	pr := Parse(testURI, content, resolvers)

	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "objGroupTable" && ref.Component == "reporting.table" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected objGroupTable → reporting.table, got %v", pr.ComponentRefs)
	}
}

func TestScriptParser_EmptyComponent(t *testing.T) {
	// Empty component should not panic
	content := `component {}`

	pr := Parse(testURI, content)

	if len(pr.Funcs) != 0 {
		t.Errorf("expected 0 funcs, got %d", len(pr.Funcs))
	}
}

func TestScriptParser_FunctionWithNoBody(t *testing.T) {
	// Interface-style function with semicolon
	content := `interface {
	function doSomething();
	function doOther();
}`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 2 {
		t.Errorf("expected 2 funcs, got %d", len(pr.Funcs))
	}
}

func TestTagParser_NestedCfscriptInsideFunction(t *testing.T) {
	// A <cfscript> block inside a <cffunction> splits the file into separate
	// Tag/Script/Tag regions (ClassifyRegions). extractSignatures pre-computes
	// tag function boundaries from the whole file (findTagFuncScopes) so both
	// tagVar (before the nested script) and the content after it still resolve
	// to the function's own scope rather than leaking to global ComponentRefs.
	content := `<cfcomponent>
<cffunction name="mixed">
	<cfset var tagVar = CreateObject("component","services.TagService") />
	<cfscript>
	var scriptVar = new services.ScriptService();
	</cfscript>
	<cfset var afterVar = CreateObject("component","services.AfterService") />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d: %+v", len(pr.Scopes), pr.Scopes)
	}

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	want := map[string]string{
		"tagVar":   "services.TagService",
		"afterVar": "services.AfterService",
	}

	for name, wantComp := range want {
		var found string

		for _, ref := range funcRefs {
			if ref.Variable == name {
				found = ref.Component
			}
		}

		if found != wantComp {
			t.Errorf("expected %s -> %s in mixed's FuncComponentRefs, got %q (refs: %v)", name, wantComp, found, funcRefs)
		}
	}

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "tagVar" || ref.Variable == "afterVar" {
			t.Errorf("expected %s to be function-scoped, not global: %+v", ref.Variable, ref)
		}
	}
}

// TestTagParser_NestedCfscriptChainedCallPendingResolution covers a chained
// factory call *inside* the nested <cfscript> block itself (not just a plain
// assignment): "conn = uri.openConnection();" where uri is a tag-declared
// local whose own component was set before the script island. ClassifyRegions
// parses this <cfscript> block as its own top-level RegionScript (a separate
// newScriptParser call in extractSignatures, not tag_parser's inline cfscript
// handling), which previously had no idea it was running inside the enclosing
// <cffunction> — so "conn"'s pending call got funcKey "" and could never find
// uri's function-scoped ref via the baseVar fallback.
func TestTagParser_NestedCfscriptChainedCallPendingResolution(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="download">
	<cfset var uri = "">
	<cfset var conn = "">
	<cfset uri = createObject("java", "java.net.URL").init("x") />
	<cfscript>
		conn = uri.openConnection();
		conn.setRequestMethod("HEAD");
	</cfscript>
</cffunction>
</cfcomponent>`

	resolvers := []Resolver{
		{
			Match:   `createObject\s*\(\s*['"]java['"]\s*,\s*['"](.+?)['"]\s*\)`,
			Resolve: "stubs.$1",
			Prefix:  "createObject",
		},
	}

	funcLookup := func(component, funcName string) string {
		if component == "stubs.java.net.URL" && funcName == "openConnection" {
			return "stubs.java.net.HttpURLConnection"
		}

		return ""
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:    resolvers,
		FuncLookup:   funcLookup,
		ExtractCalls: true,
	})

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found string

	for _, ref := range funcRefs {
		if ref.Variable == "conn" {
			found = ref.Component
		}
	}

	want := "stubs.java.net.HttpURLConnection"
	if found != want {
		t.Errorf("expected conn -> %s, got %q (funcRefs: %+v)", want, found, funcRefs)
	}
}

// === Regression tests for parser refactoring / performance work ===

func TestScriptParser_AllRefTypes(t *testing.T) {
	content := `component {
	variables.a = new models.A();
	variables.b = createObject("component", "models.B");
	variables.c = entityNew("EntityC");
	variables.d = entityLoad("EntityD");

	function work() {
		var e = new models.E();
		var f = createObject("component", "models.F");
		var g = entityNew("EntityG");
		local.h = new models.H();
		variables.i = new models.I();
	}
}`
	pr := Parse(testURI, content)

	// Global refs: a, b, c, d, i (variables. inside func with forceGlobal)
	globals := map[string]string{"a": "models.A", "b": "models.B", "c": "EntityC", "d": "EntityD", "i": "models.I"}
	for _, ref := range pr.ComponentRefs {
		if expected, ok := globals[ref.Variable]; ok {
			if ref.Component != expected {
				t.Errorf("ComponentRef %s: expected %s, got %s", ref.Variable, expected, ref.Component)
			}

			delete(globals, ref.Variable)
		}
	}

	for k, v := range globals {
		t.Errorf("missing ComponentRef %s → %s", k, v)
	}

	// Function refs: e, f, g, h
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)
	locals := map[string]string{"e": "models.E", "f": "models.F", "g": "EntityG", "h": "models.H"}

	for _, ref := range funcRefs {
		if expected, ok := locals[ref.Variable]; ok {
			if ref.Component != expected {
				t.Errorf("FuncRef %s: expected %s, got %s", ref.Variable, expected, ref.Component)
			}

			delete(locals, ref.Variable)
		}
	}

	for k, v := range locals {
		t.Errorf("missing FuncRef %s → %s", k, v)
	}
}

func TestScriptParser_FuncScopes(t *testing.T) {
	content := `component {
	public void function init() {}
	private string function getName() { return ""; }
	function doWork() { var x = 1; }
}`
	pr := Parse(testURI, content)

	if len(pr.Scopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(pr.Scopes))
	}

	if pr.Scopes[0].Name != "init" {
		t.Errorf("scope[0] expected 'init', got %q", pr.Scopes[0].Name)
	}

	if pr.Scopes[1].Name != "getName" {
		t.Errorf("scope[1] expected 'getName', got %q", pr.Scopes[1].Name)
	}

	if pr.Scopes[2].Name != "doWork" {
		t.Errorf("scope[2] expected 'doWork', got %q", pr.Scopes[2].Name)
	}
}

func TestScriptParser_ReturnTypes(t *testing.T) {
	content := `component {
	public models.User function getUser() {}
	string function getName() {}
	function noType() {}
}`
	pr := Parse(testURI, content)

	cases := []struct {
		name       string
		returnType string
	}{
		{"getUser", "models.User"},
		{"getName", "string"},
		{"noType", ""},
	}

	for i, c := range cases {
		if pr.Funcs[i].ReturnType != c.returnType {
			t.Errorf("%s: expected ReturnType %q, got %q", c.name, c.returnType, pr.Funcs[i].ReturnType)
		}
	}
}

func TestScriptParser_VarsExtraction(t *testing.T) {
	content := `component {
	variables.global1 = "x";
	this.global2 = "y";

	function work() {
		var local1 = 1;
		local.local2 = 2;
		arguments.arg1 = 3;
		variables.fromFunc = "z";
		plain = "w";
	}
}`
	pr := Parse(testURI, content)

	// GlobalVars should include global1, global2, fromFunc, plain
	gv := pr.GlobalVars()

	for _, name := range []string{"global1", "global2"} {
		found := false

		for _, v := range gv {
			if v == name {
				found = true
			}
		}

		if !found {
			t.Errorf("expected %s in GlobalVars, got %v", name, gv)
		}
	}

	// FuncVars should include local1, local2, arg1
	scope := pr.Scopes[0]
	fv := pr.FuncVars(scope.Start, scope.End)

	for _, name := range []string{"local1", "local2", "arg1"} {
		found := false

		for _, v := range fv {
			if v == name {
				found = true
			}
		}

		if !found {
			t.Errorf("expected %s in FuncVars, got %v", name, fv)
		}
	}
}

func TestTagParser_AllRefTypes(t *testing.T) {
	content := `<cfcomponent>
<cfset VARIABLES.a = CreateObject("component","models.A") />
<cfset VARIABLES.b = new models.B() />
<cfset VARIABLES.c = entityNew("EntityC") />
<cffunction name="work">
	<cfset var d = CreateObject("component","models.D") />
	<cfset var e = new models.E() />
	<cfset VARIABLES.f = new models.F() />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	globals := map[string]bool{"a": false, "b": false, "c": false, "f": false}
	for _, ref := range pr.ComponentRefs {
		if _, ok := globals[ref.Variable]; ok {
			globals[ref.Variable] = true
		}
	}

	for k, found := range globals {
		if !found {
			t.Errorf("expected %s in ComponentRefs", k)
		}
	}

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)
	locals := map[string]bool{"d": false, "e": false}

	for _, ref := range funcRefs {
		if _, ok := locals[ref.Variable]; ok {
			locals[ref.Variable] = true
		}
	}

	for k, found := range locals {
		if !found {
			t.Errorf("expected %s in FuncComponentRefs", k)
		}
	}
}

// TestResolverMatch_PipeDelimitedPrefix covers a single resolver whose `Prefix` lists
// multiple pipe-delimited alternatives, sharing one match/resolve pair across call-site
// shapes that don't begin with a common substring (so a single plain Prefix can't cover both).
func TestResolverMatch_PipeDelimitedPrefix(t *testing.T) {
	resolvers := []Resolver{
		{Match: `createModel\([^)]*\)|buildModel\([^)]*\)`, Resolve: "core.contextmodel", Prefix: "createModel|buildModel"},
	}

	cases := []struct {
		expr     string
		expected string
	}{
		{`createModel()`, "core.contextmodel"},
		{`buildModel(arg1)`, "core.contextmodel"},
		{`unrelatedCall()`, ""},
	}

	for _, c := range cases {
		result := ResolveFromCall(c.expr, resolvers)
		if result != c.expected {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", c.expr, result, c.expected)
		}
	}

	// Same resolver via the pre-grouped ResolverSet path (BuildResolverSet/Resolve),
	// which indexes by first byte of each prefix alternative.
	rs := BuildResolverSet(resolvers)
	for _, c := range cases {
		result := rs.Resolve(c.expr)
		if result != c.expected {
			t.Errorf("ResolverSet.Resolve(%q) = %q, want %q", c.expr, result, c.expected)
		}
	}
}

// TestResolverMatch_PipeDelimitedPrefix_OverlappingAlternatives pins down a subtle, surprising
// behavior: findPrefixPos returns the position of whichever alternative is found first *in list
// order*, not the alternative that would actually let match succeed. When one alternative is a
// substring of another, listing the shorter one first makes the longer one unreachable — the
// short alternative's (earlier, wrong) position is used to slice expr, so match never sees the
// full text. This is the pipe-delimited-prefix variant of the pre-existing "prefix substring
// false-positive" limitation documented in CLAUDE.md, not a new bug — it's inherent to
// findPrefixPos trying alternatives in order and stopping at the first hit.
func TestResolverMatch_PipeDelimitedPrefix_OverlappingAlternatives(t *testing.T) {
	resolvers := []Resolver{
		// "File" is a substring of "getFile" (found at position 3 within "getFile()").
		{Match: `getFile\([^)]*\)`, Resolve: "customobjects.file", Prefix: "File|getFile"},
	}

	if got := ResolveFromCall(`getFile()`, resolvers); got != "" {
		t.Errorf("ResolveFromCall(getFile()) = %q, want empty (shorter alternative listed first steals the match position)", got)
	}

	// Swapping alternative order to longest-first avoids the problem.
	reordered := []Resolver{
		{Match: `getFile\([^)]*\)`, Resolve: "customobjects.file", Prefix: "getFile|File"},
	}
	if got := ResolveFromCall(`getFile()`, reordered); got != "customobjects.file" {
		t.Errorf("ResolveFromCall(getFile()) with longest-first order = %q, want customobjects.file", got)
	}
}

// TestResolverMatch_PipeDelimitedPrefix_ThreeAlternatives covers a Prefix with more than two
// pipe-delimited alternatives (the earlier tests only use two), matching the shape of the real
// merged config entries this feature was built for (e.g. the tassweb "nocheck" and
// "subservices" groups collapse 4-6 entries into one resolver each).
func TestResolverMatch_PipeDelimitedPrefix_ThreeAlternatives(t *testing.T) {
	resolvers := []Resolver{
		{
			Match:   `^_user(?:\(\))?$|^ARGUMENTS\.user$|^SESSION\.userInfo$`,
			Resolve: "core.user",
			Prefix:  "_user|ARGUMENTS.user|SESSION.userInfo",
		},
	}

	cases := []struct {
		expr     string
		expected string
	}{
		{"_user", "core.user"},
		{"ARGUMENTS.user", "core.user"},
		{"SESSION.userInfo", "core.user"},
		{"REQUEST.user", ""},
	}

	rs := BuildResolverSet(resolvers)

	for _, c := range cases {
		if got := ResolveFromCall(c.expr, resolvers); got != c.expected {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", c.expr, got, c.expected)
		}

		if got := rs.Resolve(c.expr); got != c.expected {
			t.Errorf("ResolverSet.Resolve(%q) = %q, want %q", c.expr, got, c.expected)
		}
	}
}

// TestResolverMatch_PipeDelimitedPrefix_NoFollow confirms NoFollow still propagates correctly
// when the match came via a non-first alternative of a pipe-delimited Prefix — ResolveFromCallFull
// returns NoFollow from the resolver struct itself, not from which alternative matched, but this
// pins that down explicitly since noFollow resolvers (config's "nocheck" pattern) are a real
// merge candidate (e.g. collapsing several noFollow:true entries into one).
//
// Match must contain a literal backslash to compile as regex at all — isRegexPattern's only
// trigger is "contains \\", independent of Prefix/pipe-delimiting entirely (a bare
// "a|b" Match with no backslash is compared as one literal string, not an alternation, and
// would silently fail here). The "(?:\(\))?" optional-parens tail supplies that backslash
// naturally, same as the real merged config entries do.
func TestResolverMatch_PipeDelimitedPrefix_NoFollow(t *testing.T) {
	resolvers := []Resolver{
		{Match: `^domobject_document(?:\(\))?$|^domobject_instance(?:\(\))?$`, Resolve: "nocheck", Prefix: "domobject_document|domobject_instance", NoFollow: true},
	}

	cases := []string{"domobject_document", "domobject_instance"}
	for _, expr := range cases {
		comp, noFollow := ResolveFromCallFull(expr, resolvers)
		if comp != "nocheck" || !noFollow {
			t.Errorf("ResolveFromCallFull(%q) = (%q, %v), want (\"nocheck\", true)", expr, comp, noFollow)
		}
	}

	if comp, _ := ResolveFromCallFull("unrelated", resolvers); comp != "" {
		t.Errorf("ResolveFromCallFull(unrelated) = %q, want empty", comp)
	}
}

// TestResolverMatch_PipeDelimitedPrefix_CaseInsensitive confirms each pipe-delimited alternative
// is matched case-insensitively, same as a single Prefix always has been — the split just adds
// alternatives, it must not change per-alternative matching semantics.
func TestResolverMatch_PipeDelimitedPrefix_CaseInsensitive(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getFile\([^)]*\)|objFile\([^)]*\)`, Resolve: "customobjects.file", Prefix: "getFile|objFile"},
	}

	cases := []string{"GETFILE()", "ObjFile()", "getfile(1)"}
	for _, expr := range cases {
		if got := ResolveFromCall(expr, resolvers); got != "customobjects.file" {
			t.Errorf("ResolveFromCall(%q) = %q, want customobjects.file", expr, got)
		}
	}
}

// TestPrefixEqualFold_EmptyAlternative documents the empty-alternative guard in prefixEqualFold
// (ast.go), matching the guard its siblings prefixContainsFold/findPrefixPos already had. A
// leading/trailing/doubled "|" in a Prefix (e.g. "getFile|", a plausible config typo) produces an
// empty alternative via splitPrefix; without the guard, EqualFold("", "") would wrongly report a
// match against an empty expr. No current caller passes an empty expr (both call sites already
// guard non-empty), so this calls the unexported helper directly to pin the fix down regardless
// of caller behavior.
func TestPrefixEqualFold_EmptyAlternative(t *testing.T) {
	if prefixEqualFold("", "getFile|") {
		t.Error("prefixEqualFold(\"\", \"getFile|\") = true, want false")
	}

	if !prefixEqualFold("getFile", "getFile|") {
		t.Error("prefixEqualFold(\"getFile\", \"getFile|\") = false, want true")
	}
}

// TestResolverMatch_PipeDelimitedPrefix_AssignmentCallSite guards against a regression where
// a pipe-delimited Prefix resolved correctly through ResolveFromCall/ResolverSet.Resolve but
// silently failed through scriptParser.tryResolveCall's separate quick-rejection check, which
// used to compare the whole "a|b" Prefix string as one literal substring via containsFold
// instead of checking each alternative. That check only runs for `x = foo.bar(...)`-shaped
// assignments (not the bare "_parent"-style non-call RHS covered above), so this test exercises
// that call-site shape specifically.
func TestResolverMatch_PipeDelimitedPrefix_AssignmentCallSite(t *testing.T) {
	content := `component {
	function work() {
		template = document.createTemplate(width=1);
	}
}`
	resolvers := []Resolver{
		{Match: `document\.createTemplate|document\.createStamper\(\)`, Resolve: "reporting.directcontent", Prefix: "document.createTemplate|document.createStamper"},
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers, ScanAllScopes: true})

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	found := ""

	for _, ref := range refs {
		if ref.Variable == "template" {
			found = ref.Component
		}
	}

	if found != "reporting.directcontent" {
		t.Errorf("expected template -> reporting.directcontent, got %q", found)
	}
}

// TestResolverMatch_PipeDelimitedPrefix_BareNameFallback guards against a regression where the
// "bare function name" fallback in checkSetRHSStr (tag_parser.go) and tryResolveCall
// (script_parser.go) — used for e.g. "style = document.loadStylesheet(...)" where "document"
// itself is a bare-word resolver match — only recognized r.simple resolvers via a hand-rolled
// EqualFold(callExpr, r.Match) check. Pipe-merging turns Match into a regex (r.simple=false),
// which this check silently skipped even though the resolver would otherwise match, dropping
// the ComponentRef entirely. tag_parser.go must run the bare-name candidate through
// matchResolverWithCache so regex-mode (merged) resolvers are recognized too.
func TestResolverMatch_PipeDelimitedPrefix_BareNameFallback(t *testing.T) {
	resolvers := []Resolver{
		{Match: `_kernel\.createDocument\([^)]*\)|^document(?:\(\))?$`, Resolve: "reporting.itext", Prefix: "_kernel.createDocument|document"},
	}

	content := `<cfset style = document.loadStylesheet(cssfile="x",properties="y")>`
	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers})

	found := ""

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "style" {
			found = ref.Component
		}
	}

	if found != "reporting.itext" {
		t.Errorf("expected style -> reporting.itext, got %q", found)
	}
}

// TestResolverMatch_PipeDelimitedPrefix_ScriptBareNameFallback is the script_parser.go
// counterpart to TestResolverMatch_PipeDelimitedPrefix_BareNameFallback above: tryResolveCall's
// own "bare name" fallback (script_parser.go) had the identical r.simple-only bug as
// tag_parser.go's checkSetRHSStr, just on a different call shape — a bare function call with no
// dot chain (e.g. "svc = _kernel()") rather than an obj.method() chain. The resolver's Match has
// no optional-parens tail, so the earlier "callExpr()" attempt must fail and fall through to the
// bare-name check for this test to actually exercise that code path.
func TestResolverMatch_PipeDelimitedPrefix_ScriptBareNameFallback(t *testing.T) {
	resolvers := []Resolver{
		{Match: `_kernel\.foo\([^)]*\)|^_kernel$`, Resolve: "core.kernel2", Prefix: "_kernel.foo|_kernel"},
	}

	content := `component {
	function work() {
		svc = _kernel();
	}
}`
	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers, ScanAllScopes: true})
	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	found := ""

	for _, ref := range refs {
		if ref.Variable == "svc" {
			found = ref.Component
		}
	}

	if found != "core.kernel2" {
		t.Errorf("expected svc -> core.kernel2, got %q", found)
	}
}

func TestResolverMatch_VariousPatterns(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "services.$1.service", Prefix: "getService"},
		{Match: `_parent`, Resolve: "core.kernel2", Prefix: "_parent"},
		{Match: `getController("$1")`, Resolve: "controllers.$1", Prefix: "getController"},
	}

	cases := []struct {
		expr     string
		expected string
	}{
		{`getService("user")`, "services.user.service"},
		{`getService("timetable")`, "services.timetable.service"},
		{`_parent`, "core.kernel2"},
		{`getController("kiosk")`, "controllers.kiosk"},
		{`getController("kiosk").doStuff()`, "controllers.kiosk"},
		{`unknownFunc()`, ""},
	}

	for _, c := range cases {
		result := ResolveFromCall(c.expr, resolvers)
		if result != c.expected {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", c.expr, result, c.expected)
		}
	}
}

// TestResolveFromCall_RegexPatternShapes covers the two regex-match variants a `\`-containing
// Match pattern can take: an explicit capture group whose value is substituted into Resolve
// (already exercised elsewhere via `kernel\.get([A-Za-z0-9_]+)\(\)`-style resolvers), and — the
// previously-untested shape — a pure boolean regex gate with no capture group at all, where
// Resolve is returned verbatim via the `len(m) == 1` short-circuit in matchResolverWithCache.
func TestResolveFromCall_RegexPatternShapes(t *testing.T) {
	resolvers := []Resolver{
		// No capture group — regex is just a gate; Resolve is a fixed string.
		{Match: `createModel\([^)]*\)`, Resolve: "core.contextmodel", Prefix: "createModel"},
		// Capture group inside quoted call args — captured value substituted into Resolve.
		{Match: `createObject\s*\(\s*['"]java['"]\s*,\s*['"](.+?)['"]\s*\)`, Resolve: "javastubs.$1", Prefix: "createObject"},
	}

	cases := []struct {
		expr     string
		expected string
	}{
		{`createModel()`, "core.contextmodel"},
		{`createModel(arg1, arg2)`, "core.contextmodel"},
		{`createModelSomethingElse()`, ""},
		{`createObject("java", "java.util.HashMap")`, "javastubs.java.util.HashMap"},
		{`createObject('java', 'com.example.Foo')`, "javastubs.com.example.Foo"},
		{`createObject("component", "models.User")`, ""},
	}

	for _, c := range cases {
		result := ResolveFromCall(c.expr, resolvers)
		if result != c.expected {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", c.expr, result, c.expected)
		}
	}
}

// TestResolveFromCall_DiscardedCaptureDottedChain covers the "$1 at the very end of Match"
// shape (empty suffix), used by struct-key handle patterns like `itextObj.Foo.$1` where the
// captured method call is discarded (Resolve has no $1) and the target is fixed. This exercises
// the `else` branch of simpleMatch (capture everything after prefix) that a suffixed pattern like
// `getUser($1)` never reaches, and confirms a two-level dotted RHS assignment
// (`var x = handle.Key.method(...)`) is actually recognized as a component ref during parsing,
// not just matched in isolation.
func TestResolveFromCall_DiscardedCaptureDottedChain(t *testing.T) {
	resolvers := []Resolver{
		{Match: "itextObj.PDFtableCell.$1", Resolve: "javastubs.com.lowagie.text.pdf.PdfPCell", Prefix: "itextObj.PDFtableCell"},
	}

	cases := []struct {
		expr     string
		expected string
	}{
		{`itextObj.PDFtableCell.init()`, "javastubs.com.lowagie.text.pdf.PdfPCell"},
		{`itextObj.PDFtableCell.init(1, 2)`, "javastubs.com.lowagie.text.pdf.PdfPCell"},
		{`itextObj.PDFtable.init()`, ""},
	}

	for _, c := range cases {
		result := ResolveFromCall(c.expr, resolvers)
		if result != c.expected {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", c.expr, result, c.expected)
		}
	}

	content := `component {
	function build() {
		var cell = itextObj.PDFtableCell.init(6);
	}
}`
	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ScanAllScopes: true,
	})

	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	scope := pr.Scopes[0]

	var cellComp string

	for _, ref := range pr.FuncComponentRefs(scope.Start, scope.End) {
		if ref.Variable == "cell" {
			cellComp = ref.Component
		}
	}

	if cellComp != "javastubs.com.lowagie.text.pdf.PdfPCell" {
		t.Errorf("expected cell → javastubs.com.lowagie.text.pdf.PdfPCell via dotted-chain resolver, got %q", cellComp)
	}
}

// TestResolveFromCall_CaseFoldedPlaceholder covers the ${1:lower}/${1:upper} substitution
// modifiers, which let a single templated resolver replace a family of per-name resolvers
// whose target is `packages.<lowercase-of-captured-name>` — the captured value is normally
// written in call-site case (e.g. `getPageTools()` captures "PageTools"), but CFML package
// paths are conventionally lowercase, so a plain $1 substitution would resolve the wrong,
// wrongly-cased path. Covers both the simple-match ($1 in Match, no backslash) and regex-match
// (capture group in Match) code paths, since substitutePlaceholder is shared by both.
func TestResolveFromCall_CaseFoldedPlaceholder(t *testing.T) {
	simpleResolvers := []Resolver{
		{Match: "get$1()", Resolve: "tassweb.packages.tass.${1:lower}", Prefix: "get"},
	}

	regexResolvers := []Resolver{
		{Match: `get([A-Za-z]+)\(\)`, Resolve: "tassweb.packages.tass.${1:lower}", Prefix: "get"},
	}

	upperResolvers := []Resolver{
		{Match: "get$1()", Resolve: "constants.${1:upper}", Prefix: "get"},
	}

	cases := []struct {
		name      string
		resolvers []Resolver
		expr      string
		expected  string
	}{
		{"simple lower", simpleResolvers, "getPageTools()", "tassweb.packages.tass.pagetools"},
		{"simple lower, already lowercase call", simpleResolvers, "getLockBroker()", "tassweb.packages.tass.lockbroker"},
		{"regex lower", regexResolvers, "getPageTools()", "tassweb.packages.tass.pagetools"},
		{"upper", upperResolvers, "getFoo()", "constants.FOO"},
	}

	for _, c := range cases {
		result := ResolveFromCall(c.expr, c.resolvers)
		if result != c.expected {
			t.Errorf("%s: ResolveFromCall(%q) = %q, want %q", c.name, c.expr, result, c.expected)
		}
	}
}

func TestExpressionMappings_MultipleReplacements(t *testing.T) {
	content := `component {
	variables.a = createObject("component", "#ROOT#models.A");
	variables.b = createObject("component", "#CORE#services.B");
}`
	pr := ParseWithOptions(testURI, content, ParseOptions{
		ExpressionMappings: map[string]string{
			"#ROOT#": "app.",
			"#CORE#": "app.core.",
		},
	})

	found := map[string]string{}
	for _, ref := range pr.ComponentRefs {
		found[ref.Variable] = ref.Component
	}

	if found["a"] != "app.models.A" {
		t.Errorf("expected a → app.models.A, got %q", found["a"])
	}

	if found["b"] != "app.core.services.B" {
		t.Errorf("expected b → app.core.services.B, got %q", found["b"])
	}
}

func TestExpressionMappings_PipeDelimitedAlternatives(t *testing.T) {
	content := `component {
	variables.a = createObject("component", "#ROOT#models.A");
	variables.b = createObject("component", "#LEGACY_ROOT#models.B");
}`
	pr := ParseWithOptions(testURI, content, ParseOptions{
		ExpressionMappings: map[string]string{
			"#ROOT#|#LEGACY_ROOT#": "app.",
		},
	})

	found := map[string]string{}
	for _, ref := range pr.ComponentRefs {
		found[ref.Variable] = ref.Component
	}

	if found["a"] != "app.models.A" {
		t.Errorf("expected a → app.models.A, got %q", found["a"])
	}

	if found["b"] != "app.models.B" {
		t.Errorf("expected b → app.models.B, got %q", found["b"])
	}
}

func TestPendingCalls_MultipleCallsInSameFunction(t *testing.T) {
	content := `component {
	models.User function getUser() { return new models.User(); }
	models.Order function getOrder() { return new models.Order(); }

	function work() {
		var user = getUser();
		var order = getOrder();
	}
}`
	pr := Parse(testURI, content)

	scope := pr.Scopes[2] // work
	refs := pr.FuncComponentRefs(scope.Start, scope.End)
	found := map[string]string{}

	for _, ref := range refs {
		found[ref.Variable] = ref.Component
	}

	if found["user"] != "models.User" {
		t.Errorf("expected user → models.User, got %q", found["user"])
	}

	if found["order"] != "models.Order" {
		t.Errorf("expected order → models.Order, got %q", found["order"])
	}
}

func TestPendingCalls_BaseVarNotOverrideResolver(t *testing.T) {
	// When resolver matches, baseVar fallback should not override
	content := `component {
	variables.kernel = new core.Kernel();

	function work() {
		var svc = variables.kernel.getService("user");
	}
}`
	resolvers := []Resolver{{
		Match:   `getService("$1")`,
		Resolve: "services.$1.service",
		Prefix:  "getService",
	}}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ScanAllScopes: true,
	})

	scope := pr.Scopes[0]
	refs := pr.FuncComponentRefs(scope.Start, scope.End)

	for _, ref := range refs {
		if ref.Variable == "svc" {
			if ref.Component == "core.Kernel" {
				t.Error("svc should NOT resolve to core.Kernel (baseVar fallback); resolver should win")
			}

			if ref.Component == "services.user.service" {
				return // correct
			}
		}
	}

	// If resolver matched via appendResolverRefs, check there
	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "svc" && ref.Component == "services.user.service" {
			return // correct (in componentRefs from resolver)
		}
	}

	t.Errorf("expected svc → services.user.service, got funcRefs=%v compRefs=%v", refs, pr.ComponentRefs)
}

func TestScriptParser_PropertyParsing(t *testing.T) {
	content := `component {
	property name="userDAO" type="dao.UserDAO" inject="UserDAO@dao";
	property string name;
	property age;
}`
	pr := Parse(testURI, content)

	if len(pr.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d", len(pr.Properties))
	}
}

func TestScriptParser_Extends(t *testing.T) {
	content := `component extends="base.AbstractService" {
	function init() {}
}`
	pr := Parse(testURI, content)

	if pr.Extends != "base.AbstractService" {
		t.Errorf("expected extends 'base.AbstractService', got %q", pr.Extends)
	}
}

func TestTagParser_Extends(t *testing.T) {
	content := `<cfcomponent extends="base.AbstractService">
</cfcomponent>`
	pr := Parse(testURI, content)

	if pr.Extends != "base.AbstractService" {
		t.Errorf("expected extends 'base.AbstractService', got %q", pr.Extends)
	}
}

func TestScriptParser_ArgumentTypes(t *testing.T) {
	content := `component {
	function save(required models.User user, string name, numeric id) {}
}`
	pr := Parse(testURI, content)

	if len(pr.Funcs[0].Arguments) != 3 {
		t.Fatalf("expected 3 args, got %d", len(pr.Funcs[0].Arguments))
	}

	// Component-type argument should create a ref
	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "user" && ref.Component == "models.User" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected argument ref user → models.User, got %v", pr.ComponentRefs)
	}
}

// --- Dollar-sign identifier tests ---

func TestParseFunctionDefs_DollarSignName_Script(t *testing.T) {
	content := "component {\n\tfunction $init() {}\n\tpublic string function $helper() {}\n}"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"$init", "$helper"})
}

func TestParseFunctionDefs_DollarSignName_Tag(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"$init\">\n</cffunction>\n<cffunction name=\"$helper\">\n</cffunction>\n</cfcomponent>"
	defs := ParseFunctionDefs(testURI, content)
	assertDefs(t, defs, []string{"$init", "$helper"})
}

func TestParseFunctionDefs_DollarSignArgName(t *testing.T) {
	content := "component {\nfunction save(required string $name, numeric $age) {}\n}"

	defs := ParseFunctionDefs(testURI, content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	args := defs[0].Arguments
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %+v", len(args), args)
	}

	if args[0].Name != "$name" || args[0].Type != "string" || !args[0].Required {
		t.Errorf("arg 0 = %+v, want {$name, string, required}", args[0])
	}

	if args[1].Name != "$age" || args[1].Type != "numeric" || args[1].Required {
		t.Errorf("arg 1 = %+v, want {$age, numeric, not required}", args[1])
	}
}

func TestScriptParser_DollarSignVarName(t *testing.T) {
	content := `component {
	function work() {
		var $result = 1;
		local.$temp = 2;
	}
}`
	pr := Parse(testURI, content)
	scope := pr.Scopes[0]
	fv := pr.FuncVars(scope.Start, scope.End)

	for _, name := range []string{"$result", "$temp"} {
		if !slices.Contains(fv, name) {
			t.Errorf("expected %s in FuncVars, got %v", name, fv)
		}
	}
}

func TestScriptParser_DollarSignGlobalVar(t *testing.T) {
	content := `component {
	variables.$service = createObject("component", "services.MyService");
}`
	pr := Parse(testURI, content)

	var found bool

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "$service" && ref.Component == "services.MyService" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected $service → services.MyService in ComponentRefs, got %v", pr.ComponentRefs)
	}
}

func TestScriptParser_DollarSignLocalComponentRef(t *testing.T) {
	content := `component {
	function work() {
		var $svc = createObject("component", "services.SomeService");
	}
}`
	pr := Parse(testURI, content)
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range funcRefs {
		if ref.Variable == "$svc" && ref.Component == "services.SomeService" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected $svc → services.SomeService in FuncComponentRefs, got %v", funcRefs)
	}
}

func TestScriptParser_DollarSignResolverRef(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "services.$1", Prefix: "getService"},
	}
	content := `component {
	function work() {
		var $svc = getService("user");
	}
}`
	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers})
	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range funcRefs {
		if ref.Variable == "$svc" && ref.Component == "services.user" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected $svc → services.user in FuncComponentRefs, got %v", funcRefs)
	}
}

func TestScriptParser_ChainedCallResolverRef(t *testing.T) {
	resolvers := []Resolver{
		{Match: `createMock("$1")`, Resolve: "$1", Prefix: "createMock"},
	}
	content := `component {
	function work() {
		var mock = getMockBox().createMock("models.User");
		mock.save();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ScanAllScopes: true,
		ExtractCalls:  true,
	})
	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	scope := pr.Scopes[0]
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found bool

	for _, ref := range funcRefs {
		if ref.Variable == "mock" && ref.Component == "models.User" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected mock → models.User in FuncComponentRefs (chained call resolver), got %v", funcRefs)
	}
}

func TestTagParser_DollarSignVarName(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="work">
	<cfset var $result = 1 />
	<cfset local.$temp = 2 />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)
	scope := pr.Scopes[0]
	fv := pr.FuncVars(scope.Start, scope.End)

	for _, name := range []string{"$result", "$temp"} {
		if !slices.Contains(fv, name) {
			t.Errorf("expected %s in FuncVars, got %v", name, fv)
		}
	}
}

// TestTagParser_BareCallAssignment_ExtractCalls guards against a regression where
// checkSetRHSStr (tag_parser.go) recorded a CallSite for `x = obj.method(...)` assignments
// but not for bare `x = funcName(...)` assignments (no dot) — the bare-call branch only fed
// pendingCalls (for ComponentRef resolution), never p.addCall. That silently dropped
// same-file calls like `<cfset result = getAndUpdateDisplayName(...)>` from pr.Calls, so the
// `refs` command reported zero call sites even when the call and its target function were in
// the same file.
func TestTagParser_BareCallAssignment_ExtractCalls(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getAndUpdateDisplayName">
	<cfreturn "" />
</cffunction>
<cffunction name="caller">
	<cfset local_display_name = getAndUpdateDisplayName(companyCode=ARGUMENTS.companyCode
														, person_num = ARGUMENTS.person_num)>
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	found := false

	for _, call := range pr.FuncCalls(0, len(strings.Split(content, "\n"))) {
		if call.FuncName == "getAndUpdateDisplayName" && call.Variable == "" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected bare call to getAndUpdateDisplayName in FuncCalls, got none")
	}
}

func TestScriptParser_ChainedAssignment_NoSpuriousRef(t *testing.T) {
	// variables.$assert = this.$assert = new testbox.system.Assertion()
	// The `this.$assert` in the middle is an assignment TO this, not a source;
	// only the final `new testbox.system.Assertion()` should produce a ref.
	content := `component {
	variables.$assert = this.$assert = new testbox.system.Assertion();
}`
	pr := Parse(testURI, content)

	var got []string

	for _, r := range pr.ComponentRefs {
		if strings.EqualFold(r.Variable, "$assert") {
			got = append(got, r.Component)
		}
	}

	if len(got) != 1 || got[0] != "testbox.system.Assertion" {
		t.Errorf("expected exactly one $assert ref → testbox.system.Assertion, got %v", got)
	}
}

func TestFuncVars_Cached(t *testing.T) {
	content := `component {
	function work() {
		var a = 1;
		var b = 2;
	}
}`
	pr := Parse(testURI, content)
	scope := pr.Scopes[0]

	// First call computes
	vars1 := pr.FuncVars(scope.Start, scope.End)
	// Second call should return cached
	vars2 := pr.FuncVars(scope.Start, scope.End)

	if len(vars1) != len(vars2) {
		t.Errorf("cached FuncVars mismatch: %v vs %v", vars1, vars2)
	}

	// Invalidate and re-compute
	pr.InvalidateFunc(scope.Start, scope.End)
	vars3 := pr.FuncVars(scope.Start, scope.End)

	if len(vars3) != len(vars1) {
		t.Errorf("recomputed FuncVars mismatch: %v vs %v", vars3, vars1)
	}
}

func TestTagParser_ArgumentHintRef(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="work">
	<cfargument name="document" type="any" required="true" hint="my.dotted.component" />
	<cfset var result = {} />
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)
	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	scope := pr.Scopes[0]

	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)
	for _, ref := range funcRefs {
		if ref.Variable == "document" && ref.Component == "my.dotted.component" {
			return
		}
	}

	t.Errorf("expected document → my.dotted.component in FuncComponentRefs, got %v", funcRefs)
}

func TestTagParser_ArgumentHintRef_WithResolver(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="generatePDF">
	<cfargument name="document" type="any" required="true" hint="reporting.itext" />
	<cfset var result = {} />
	<cfset var table = ARGUMENTS.document.createTable(6) />
</cffunction>
</cfcomponent>`
	resolvers := []Resolver{
		{Match: `document.createStamper()`, Resolve: "reporting.directcontent", Prefix: "document.createStamper"},
		{Match: `createTable($1)`, Resolve: "reporting.table", Prefix: "createTable"},
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ScanAllScopes: true,
	})
	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	scope := pr.Scopes[0]

	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)
	for _, ref := range funcRefs {
		if ref.Variable == "document" && ref.Component == "reporting.itext" {
			return
		}
	}

	t.Errorf("expected document → reporting.itext in FuncComponentRefs with resolvers active, got %v", funcRefs)
}

// TestTagParser_DottedCallAssignment_UsesEarlierComponentRef guards against a
// regression where a dotted call assignment `<cfset result = psService.method(...)>`
// never looked up the ComponentRef already recorded for `psService` from an earlier
// `<cfset psService = getService("person")>` resolver match. Without this, the
// CallSite's Component stayed empty, so `refs`/`findRefs` could never verify the call
// resolved to the traced target — even though goto-definition resolved `psService`
// fine, from that exact same ComponentRef.
func TestTagParser_DottedCallAssignment_UsesEarlierComponentRef(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "services.$1.service", Prefix: "getService"},
	}

	content := `<cfcomponent>
<cffunction name="updatePersonMaintenance">
	<cfset var psService = getService("person")>
	<cfset result = psService.updatePerson(argumentCollection=argsData)>
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers, ExtractCalls: true, ScanAllScopes: true})

	found := false

	for _, call := range pr.FuncCalls(0, len(strings.Split(content, "\n"))) {
		if call.FuncName == "updatePerson" && call.Variable == "psService" {
			found = true

			if call.Component != "services.person.service" || !call.Resolved {
				t.Errorf("expected updatePerson call to resolve to services.person.service (Resolved=true), got Component=%q Resolved=%v", call.Component, call.Resolved)
			}
		}
	}

	if !found {
		t.Errorf("expected a CallSite for psService.updatePerson")
	}
}

// TestTagParser_BareCallStatement_UsesEarlierComponentRef is the checkBareCallStr
// counterpart to TestTagParser_DottedCallAssignment_UsesEarlierComponentRef: the
// same lookup gap existed for a dotted call with no assignment at all, e.g.
// `<cfset psService.updatePerson(...)>`.
func TestTagParser_BareCallStatement_UsesEarlierComponentRef(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "services.$1.service", Prefix: "getService"},
	}

	content := `<cfcomponent>
<cffunction name="updatePersonMaintenance">
	<cfset var psService = getService("person")>
	<cfset psService.updatePerson(argumentCollection=argsData)>
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers, ExtractCalls: true, ScanAllScopes: true})

	found := false

	for _, call := range pr.FuncCalls(0, len(strings.Split(content, "\n"))) {
		if call.FuncName == "updatePerson" && call.Variable == "psService" {
			found = true

			if call.Component != "services.person.service" || !call.Resolved {
				t.Errorf("expected updatePerson call to resolve to services.person.service (Resolved=true), got Component=%q Resolved=%v", call.Component, call.Resolved)
			}
		}
	}

	if !found {
		t.Errorf("expected a CallSite for psService.updatePerson")
	}
}

// TestTagParser_ConcatenatedAssignment_NoSpuriousCall guards against a
// regression in the bare-call branch of checkSetRHSStr: `temp = temp & "~" &
// DateFormat(Now(), "yyyy-mm-dd")` contains a paren (from DateFormat(...)),
// but the leading identifier extractIdent grabs is "temp" — the variable
// being reassigned, not the function the paren belongs to. Before the fix,
// the branch didn't verify the paren immediately follows the extracted
// identifier, so it recorded a bogus CallSite{FuncName: "temp"}, which
// `unresolved`/`refs` then flagged as an unresolved call to a nonexistent
// function "temp".
func TestTagParser_ConcatenatedAssignment_NoSpuriousCall(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="work">
	<cfset temp = temp & "~" & DateFormat(Now(), "yyyy-mm-dd") />
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	for _, call := range pr.FuncCalls(0, len(strings.Split(content, "\n"))) {
		if strings.EqualFold(call.FuncName, "temp") {
			t.Errorf("expected no CallSite for bare variable 'temp', got %+v", call)
		}
	}
}

// TestTagParser_JavaStubResolver_ResolvesCreateObjectJava is the parser-level
// integration test for config.JavaStubResolver: confirms the synthesized
// resolver actually resolves through the parser's own regex/simple-match
// heuristics and $1 substitution, not just via a raw regexp.MatchString check.
func TestTagParser_JavaStubResolver_ResolvesCreateObjectJava(t *testing.T) {
	cr := config.JavaStubResolver("tassweb.packages.tass.javastubs")
	resolvers := []Resolver{{Match: cr.Match, Resolve: cr.Resolve, Prefix: cr.Prefix}}

	content := `<cfcomponent>
<cffunction name="work">
	<cfset variables.jss = createObject('java', 'java.security.Signature') />
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers})

	found := ""

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "jss" {
			found = ref.Component
		}
	}

	want := "tassweb.packages.tass.javastubs.java.security.Signature"
	if found != want {
		t.Errorf("expected jss -> %s, got %q", want, found)
	}
}

// TestFuncLookup_ChainedCallOverridesGenericResolver simulates the java stub
// factory pattern (Signature.getInstance() returning another Signature): a
// generic catch-all componentResolver (get(\w+)() -> packages.tass.<name>)
// would otherwise misfire on jss.getInstance(...) and resolve jssInstance to
// the wrong component. FuncLookup — modeling the stub's own declared return
// type via applyChainedReturnLookup — must take priority.
func TestFuncLookup_ChainedCallOverridesGenericResolver(t *testing.T) {
	content := `component {
	function init() {
		variables.jss = createObject('java', 'java.security.Signature');
	}

	function verify(required any tmpkey, required string tmpsignature) {
		var jssInstance = variables.jss.getInstance( algorithmMap[ ARGUMENTS.algorithm ] );
		jssInstance.initVerify( tmpkey );
		return jssInstance.verify( tmpsignature );
	}
}`

	resolvers := []Resolver{
		{
			Match:   `createObject\s*\(\s*['"]java['"]\s*,\s*['"](.+?)['"]\s*\)`,
			Resolve: "stubs.$1",
			Prefix:  "createObject",
		},
		{
			// Generic catch-all getter resolver — the false-positive source.
			Match:   `get([A-Za-z]+)\(\)`,
			Resolve: "packages.tass.${1:lower}",
			Prefix:  "get",
		},
	}

	funcLookup := func(component, funcName string) string {
		if component == "stubs.java.security.Signature" && strings.EqualFold(funcName, "getInstance") {
			return "stubs.java.security.Signature"
		}

		return ""
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:  resolvers,
		FuncLookup: funcLookup,
	})

	scope := pr.Scopes[1] // verify
	funcRefs := pr.FuncComponentRefs(scope.Start, scope.End)

	var found string

	for _, ref := range funcRefs {
		if ref.Variable == "jssInstance" {
			found = ref.Component
		}
	}

	want := "stubs.java.security.Signature"
	if found != want {
		t.Errorf("expected jssInstance -> %s, got %q (funcRefs: %+v)", want, found, funcRefs)
	}
}

// TestCheckBareCall_DeepChainTracksReceiver simulates
// "kpg.generateKeyPair().getPublic().getParams()" (a 4-level call chain assigned
// to an indexed lvalue, so it never gets a ComponentRef of its own). Before the
// fix, checkBareCall recorded only the first hop and left the scanner mid-chain,
// so the top-level dispatcher rediscovered ".getPublic(" and ".getParams(" as
// unrelated bare calls — worse, the second one had its "Variable" set to
// "getPublic" (the previous method's name, not a real variable). Every hop
// should keep Variable "kpg" and accumulate the intermediate hops in Chain.
func TestCheckBareCall_DeepChainTracksReceiver(t *testing.T) {
	content := `component {
	function work() {
		var kpg = 1;
		variables.cache[1] = kpg
			.generateKeyPair()
			.getPublic()
			.getParams();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	lastLine := len(strings.Split(content, "\n"))
	calls := pr.FuncCalls(0, lastLine)

	byFunc := make(map[string]CallSite)
	for _, c := range calls {
		byFunc[c.FuncName] = c
	}

	generateKeyPair, ok := byFunc["generateKeyPair"]
	if !ok || generateKeyPair.Variable != "kpg" || len(generateKeyPair.Chain) != 0 {
		t.Errorf("expected generateKeyPair CallSite Variable=kpg Chain=[], got %+v", generateKeyPair)
	}

	getPublic, ok := byFunc["getPublic"]
	if !ok || getPublic.Variable != "kpg" || !slices.Equal(getPublic.Chain, []string{"generateKeyPair"}) {
		t.Errorf("expected getPublic CallSite Variable=kpg Chain=[generateKeyPair], got %+v", getPublic)
	}

	getParams, ok := byFunc["getParams"]
	if !ok || getParams.Variable != "kpg" || !slices.Equal(getParams.Chain, []string{"generateKeyPair", "getPublic"}) {
		t.Errorf("expected getParams CallSite Variable=kpg Chain=[generateKeyPair getPublic], got %+v", getParams)
	}
}

// TestChainedAssignRHS_FailedFirstHopContinuesChain covers
// "subBackgnd = document.getJavaUtils().getRGBColor(r=16,g=58,b=59)" where
// "document.getJavaUtils" matches NO resolver at all (tryResolveCall and
// tryExtendChain both fail, falling to a pendingCall). Before the fix,
// tryResolveCall/tryExtendChain restored the scanner to the unconsumed "("
// on failure, but nothing then consumed the rest of the expression — so
// ".getRGBColor(...)" was rediscovered by the top-level scan loop as an
// orphaned, unqualified bare call ("no qualifier, not in file") instead of a
// call chained off "document".
func TestChainedAssignRHS_FailedFirstHopContinuesChain(t *testing.T) {
	content := `component {
	function work() {
		subBackgnd = document.getJavaUtils().getRGBColor(r=16,g=58,b=59);
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	getRGBColor, ok := byFunc["getRGBColor"]
	if !ok || getRGBColor.Variable != "document" || !slices.Equal(getRGBColor.Chain, []string{"getJavaUtils"}) {
		t.Errorf("expected getRGBColor CallSite Variable=document Chain=[getJavaUtils], got %+v", getRGBColor)
	}
}

// TestChainedAssignRHS_SucceededFirstHopContinuesChain covers the same shape
// as above, but where "document.getJavaUtils" DOES match a resolver (a
// realistic case: a broad "getjavaUtils(...)" resolver matching just that
// hop). tryResolveCall must consume through its own hop's closing ')' before
// returning success — otherwise ".getRGBColor(...)" is left dangling and,
// same as the failure case, gets rediscovered as an orphaned bare call.
func TestChainedAssignRHS_SucceededFirstHopContinuesChain(t *testing.T) {
	content := `component {
	function work() {
		subBackgnd = document.getJavaUtils().getRGBColor(r=16,g=58,b=59);
	}
}`

	resolvers := []Resolver{
		{
			Match:   `getjavaUtils\([^)]*\)`,
			Resolve: "helpers.javautils",
			Prefix:  "getjavaUtils",
		},
	}

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers:     resolvers,
		ExtractCalls:  true,
		ScanAllScopes: true,
	})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	getJavaUtils, ok := byFunc["getJavaUtils"]
	if !ok || getJavaUtils.Variable != "document" {
		t.Errorf("expected getJavaUtils CallSite Variable=document, got %+v", getJavaUtils)
	}

	getRGBColor, ok := byFunc["getRGBColor"]
	if !ok || getRGBColor.Variable != "document" || !slices.Equal(getRGBColor.Chain, []string{"getJavaUtils"}) {
		t.Errorf("expected getRGBColor CallSite Variable=document Chain=[getJavaUtils], got %+v", getRGBColor)
	}
}

// TestBracketIndexedChain_AssignmentRHS covers "x = REQUEST['a' & b & 'c'].someMethod();" —
// a dynamic bracket-indexed key between the base identifier and a chained call. Before
// the fix, the dot-walk loop in checkAssignRef only recognized TokDot to continue the
// chain; hitting "[" (not "." or "(") made it stop immediately without consuming the
// "[...]" group, leaving the scanner positioned mid-expression. The outer scan loop then
// rediscovered ".someMethod(...)" as an orphaned, unqualified bare call ("no qualifier,
// not in file") instead of a call chained off the (unknowable) REQUEST[...] receiver.
func TestBracketIndexedChain_AssignmentRHS(t *testing.T) {
	content := `component {
	function work() {
		x = REQUEST['a' & b & 'c'].someMethod(1,2);
		other();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	someMethod, ok := byFunc["someMethod"]
	if !ok || someMethod.Variable == "" {
		t.Errorf("expected someMethod CallSite to have a non-empty (poisoned) Variable, not be orphaned as a bare call, got %+v", someMethod)
	}

	other, ok := byFunc["other"]
	if !ok || other.Variable != "" {
		t.Errorf("expected other() to remain a genuine bare call (Variable=\"\"), got %+v", other)
	}
}

// TestBracketIndexedChain_BareCall covers the same shape as a bare call (no
// assignment): "REQUEST['a' & b & 'c'].someMethod();" — exercises checkBareCall's
// leading dot-chain walk.
func TestBracketIndexedChain_BareCall(t *testing.T) {
	content := `component {
	function work() {
		REQUEST['a' & b & 'c'].someMethod(1,2);
		other();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	someMethod, ok := byFunc["someMethod"]
	if !ok || someMethod.Variable == "" {
		t.Errorf("expected someMethod CallSite to have a non-empty (poisoned) Variable, not be orphaned as a bare call, got %+v", someMethod)
	}

	other, ok := byFunc["other"]
	if !ok || other.Variable != "" {
		t.Errorf("expected other() to remain a genuine bare call (Variable=\"\"), got %+v", other)
	}
}

// TestBracketIndexedChain_VarDecl covers "var x = REQUEST['a' & b & 'c'].someMethod();"
// (parseBodyVarDecl's dot-walk).
func TestBracketIndexedChain_VarDecl(t *testing.T) {
	content := `component {
	function work() {
		var x = REQUEST['a' & b & 'c'].someMethod(1,2);
		other();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	someMethod, ok := byFunc["someMethod"]
	if !ok || someMethod.Variable == "" {
		t.Errorf("expected someMethod CallSite to have a non-empty (poisoned) Variable, not be orphaned as a bare call, got %+v", someMethod)
	}

	other, ok := byFunc["other"]
	if !ok || other.Variable != "" {
		t.Errorf("expected other() to remain a genuine bare call (Variable=\"\"), got %+v", other)
	}
}

// TestBracketIndexedChain_ScopedAssignment covers "variables.x = REQUEST['a' & b &
// 'c'].someMethod();" (parseBodyScopedVar's dot-walk).
func TestBracketIndexedChain_ScopedAssignment(t *testing.T) {
	content := `component {
	function work() {
		variables.x = REQUEST['a' & b & 'c'].someMethod(1,2);
		other();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	byFunc := make(map[string]CallSite)
	for _, c := range pr.FuncCalls(0, 10) {
		byFunc[c.FuncName] = c
	}

	someMethod, ok := byFunc["someMethod"]
	if !ok || someMethod.Variable == "" {
		t.Errorf("expected someMethod CallSite to have a non-empty (poisoned) Variable, not be orphaned as a bare call, got %+v", someMethod)
	}

	other, ok := byFunc["other"]
	if !ok || other.Variable != "" {
		t.Errorf("expected other() to remain a genuine bare call (Variable=\"\"), got %+v", other)
	}
}

// TestTagParser_FunctionReturnHintPromotion covers promoting a <cffunction>'s
// hint to its ReturnType when returntype is generic (e.g. "struct") and hint
// looks like a genuine component path — the same safe mechanism already used
// for <cfargument> Type/Hint, extended to the function's own return type.
func TestTagParser_FunctionReturnHintPromotion(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getJavaUtils" access="public" output="false" returntype="struct" hint="tassreporting.packages.reporting.helpers.javautils">
	<cfreturn VARIABLES._javaUtils />
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	want := "tassreporting.packages.reporting.helpers.javautils"
	if pr.Funcs[0].ReturnType != want {
		t.Errorf("expected ReturnType %q, got %q", want, pr.Funcs[0].ReturnType)
	}
}

// TestTagParser_FunctionReturnHintPromotion_PlainProseNotPromoted covers the
// false-positive this needed isComponentType to reject: a plain descriptive
// hint that happens to end in a sentence "." (e.g. "Get Reference to Java
// Utilities.") must NOT be mistaken for a dotted component path.
func TestTagParser_FunctionReturnHintPromotion_PlainProseNotPromoted(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="getJavaUtils" access="public" output="false" returntype="struct" hint="Get Reference to Java Utilities.">
	<cfreturn VARIABLES._javaUtils />
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	if pr.Funcs[0].ReturnType != "struct" {
		t.Errorf("expected ReturnType to stay %q, got %q", "struct", pr.Funcs[0].ReturnType)
	}
}

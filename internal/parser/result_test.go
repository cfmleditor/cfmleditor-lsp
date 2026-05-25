package parser

import (
	"slices"
	"testing"
)

func TestParseResult_Funcs(t *testing.T) {
	content := "component {\n\tpublic string function getUser() {}\n\tprivate function save() {}\n}"
	pr := Parse(testURI, content)
	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(pr.Funcs))
	}
	if pr.Funcs[0].Name != "getUser" {
		t.Errorf("func 0 = %q, want getUser", pr.Funcs[0].Name)
	}
	if pr.Funcs[1].Name != "save" {
		t.Errorf("func 1 = %q, want save", pr.Funcs[1].Name)
	}
}

func TestParseResult_Refs(t *testing.T) {
	content := "component {\n\tsvc = new services.OrderService()\n}"
	pr := Parse(testURI, content)
	if len(pr.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(pr.Refs))
	}
	if pr.Refs[0].Component != "services.OrderService" {
		t.Errorf("ref component = %q", pr.Refs[0].Component)
	}
}

func TestParseResult_GlobalVars(t *testing.T) {
	content := "variables.config = {}\nthis.name = \"test\"\nfunction init() {\n\tvar x = 1\n}"
	pr := Parse(testURI, content)
	globals := pr.GlobalVars()
	if !slices.Contains(globals, "config") {
		t.Errorf("expected config in globals: %v", globals)
	}
	if !slices.Contains(globals, "name") {
		t.Errorf("expected name in globals: %v", globals)
	}
}

func TestParseResult_FuncVars(t *testing.T) {
	content := "component {\nfunction doStuff() {\n\tvar localVar = 2\n\tlocal.other = 3\n\targuments.id = 1\n}\n}"
	pr := Parse(testURI, content)
	if len(pr.Scopes) == 0 {
		t.Fatal("expected at least 1 scope")
	}
	vars := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "localVar") {
		t.Errorf("expected localVar in %v", vars)
	}
	if !slices.Contains(vars, "other") {
		t.Errorf("expected other in %v", vars)
	}
	if !slices.Contains(vars, "id") {
		t.Errorf("expected id in %v", vars)
	}
}

func TestParseResult_FuncVarsCached(t *testing.T) {
	content := "component {\nfunction doStuff() {\n\tvar x = 1\n}\n}"
	pr := Parse(testURI, content)
	vars1 := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	vars2 := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if len(vars1) != len(vars2) {
		t.Error("cached result differs")
	}
}

func TestParseResult_InvalidateFunc(t *testing.T) {
	content := "component {\nfunction doStuff() {\n\tvar x = 1\n}\n}"
	pr := Parse(testURI, content)
	_ = pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	pr.InvalidateFunc(pr.Scopes[0].Start, pr.Scopes[0].End)
	// Should re-parse without error
	vars := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "x") {
		t.Errorf("expected x after invalidate: %v", vars)
	}
}

func TestParseResult_VariablesVars(t *testing.T) {
	content := "variables.config = {}\nvariables.dsn = \"test\"\nfunction init() {\n\tvar x = 1\n}"
	pr := Parse(testURI, content)
	vars := pr.VariablesVars()
	if !slices.Contains(vars, "config") {
		t.Errorf("expected config in %v", vars)
	}
	if !slices.Contains(vars, "dsn") {
		t.Errorf("expected dsn in %v", vars)
	}
}

func TestParseResult_ThisVars(t *testing.T) {
	content := "this.name = \"app\"\nthis.datasource = \"mydb\"\nfunction init() {}"
	pr := Parse(testURI, content)
	vars := pr.ThisVars()
	if !slices.Contains(vars, "name") {
		t.Errorf("expected name in %v", vars)
	}
	if !slices.Contains(vars, "datasource") {
		t.Errorf("expected datasource in %v", vars)
	}
}

func TestParseResult_TagFile(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="save">
	<cfargument name="id" type="numeric" required="true">
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)
	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}
	if pr.Funcs[0].Name != "save" {
		t.Errorf("func name = %q", pr.Funcs[0].Name)
	}
	if len(pr.Funcs[0].Arguments) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(pr.Funcs[0].Arguments))
	}
}

func TestParseResult_InitVars(t *testing.T) {
	content := `component {
	function init() {
		variables.persist = createObject("component", "persist").init()
		variables.logger = new utils.Logger()
		this.ready = true
		var local1 = 1
	}
	function other() {
		variables.notInit = "x"
	}
}`
	pr := Parse(testURI, content)
	vars := pr.VariablesVars()
	if !slices.Contains(vars, "persist") {
		t.Errorf("expected persist in VariablesVars: %v", vars)
	}
	if !slices.Contains(vars, "logger") {
		t.Errorf("expected logger in VariablesVars: %v", vars)
	}
	if slices.Contains(vars, "local1") {
		t.Errorf("local1 should not be in VariablesVars: %v", vars)
	}
	if slices.Contains(vars, "notInit") {
		t.Errorf("notInit should not be in VariablesVars: %v", vars)
	}

	thisVars := pr.ThisVars()
	if !slices.Contains(thisVars, "ready") {
		t.Errorf("expected ready in ThisVars: %v", thisVars)
	}

	// Refs should include init() body refs
	var refComponents []string
	for _, r := range pr.Refs {
		refComponents = append(refComponents, r.Component)
	}
	if !slices.Contains(refComponents, "persist") {
		t.Errorf("expected persist ref: %v", refComponents)
	}
	if !slices.Contains(refComponents, "utils.Logger") {
		t.Errorf("expected utils.Logger ref: %v", refComponents)
	}
}

func TestParseResult_InitVarsTag(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="init">
	<cfset variables.persist = createObject("component","persist").init() />
	<cfset this.ready = true />
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)
	vars := pr.VariablesVars()
	if !slices.Contains(vars, "persist") {
		t.Errorf("expected persist in VariablesVars: %v", vars)
	}
	thisVars := pr.ThisVars()
	if !slices.Contains(thisVars, "ready") {
		t.Errorf("expected ready in ThisVars: %v", thisVars)
	}
	var found bool
	for _, r := range pr.Refs {
		if r.Component == "persist" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected persist component ref in pr.Refs")
	}
}

func TestParseResult_InitVarsNoDuplicates(t *testing.T) {
	content := `component {
	variables.persist = ""
	function init() {
		variables.persist = createObject("component", "persist").init()
		variables.extra = "x"
	}
}`
	pr := Parse(testURI, content)
	vars := pr.VariablesVars()
	count := 0
	for _, v := range vars {
		if v == "persist" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected persist once, got %d times in %v", count, vars)
	}
	if !slices.Contains(vars, "extra") {
		t.Errorf("expected extra in VariablesVars: %v", vars)
	}
}

func TestResolveFromCall(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
		{Match: "_parent", Resolve: "packages.core.base.kernel2", Prefix: "_parent"},
		{Match: `kernel\.get([A-Za-z0-9_]+)\(\)`, Resolve: "app.packages.services.$1", Prefix: "kernel.get"},
	}

	tests := []struct {
		expr string
		want string
	}{
		{`getService("timetable")`, "packages.timetable.service"},
		{`getService('teacher')`, "packages.teacher.service"},
		{`_parent.getService("general")`, "packages.general.service"},
		{`_parent.getService('general')`, "packages.general.service"},
		{`VARIABLES._parent.getService("general")`, "packages.general.service"},
		{`_parent`, "packages.core.base.kernel2"},
		{`somethingElse()`, ""},
		{`SERVER.kernel.GetFinance()`, "app.packages.services.Finance"},
		{`kernel.getUser()`, "app.packages.services.User"},
	}

	for _, tt := range tests {
		got := ResolveFromCall(tt.expr, resolvers)
		if got != tt.want {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

func TestParseResult_ResolverRefs(t *testing.T) {
	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}
	content := `<cfcomponent>
	<cffunction name="init">
		<cfset VARIABLES.persist = createObject('component','persist').init() />
		<cfset var temp2 = VARIABLES._parent.getService("general") />
	</cffunction>
	<cffunction name="other">
		<cfset var svc = _parent.getService("timetable") />
	</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content, resolvers)

	// temp2 should resolve via resolver (inside init — init is always scanned)
	var found bool
	for _, ref := range pr.Refs {
		if ref.Variable == "temp2" && ref.Component == "packages.general.service" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected resolver ref for temp2 -> packages.general.service")
	}

	// svc inside other() is extracted lazily via FuncRefs
	var otherScope FuncScope
	for _, sc := range pr.Scopes {
		for _, f := range pr.Funcs {
			if int(f.Line) == sc.Start && f.Name == "other" {
				otherScope = sc
			}
		}
	}
	funcRefs, _ := pr.FuncRefs(otherScope.Start, otherScope.End)
	found = false
	for _, ref := range funcRefs {
		if ref.Variable == "svc" && ref.Component == "packages.timetable.service" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected lazy resolver ref for svc -> packages.timetable.service")
	}
}

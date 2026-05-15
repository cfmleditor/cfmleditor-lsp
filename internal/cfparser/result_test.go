package cfparser

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

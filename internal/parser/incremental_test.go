package parser

import (
	"slices"
	"testing"
)

func TestApplyEdit_InsideFunc(t *testing.T) {
	content := "component {\nfunction doStuff() {\n\tvar x = 1\n}\nfunction other() {\n\tvar y = 2\n}\n}"
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(pr.Funcs))
	}

	// Pre-cache function vars
	vars := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "x") {
		t.Fatalf("expected x in vars: %v", vars)
	}

	// Edit inside first function: add "\tvar z = 3\n" after "var x = 1"
	kind := pr.ApplyEdit(2, 10, 2, 10, "\n\tvar z = 3")
	if kind != EditInFunc {
		t.Errorf("expected EditInFunc, got %d", kind)
	}

	// Scopes should be shifted
	if pr.Scopes[1].Start != 5 {
		t.Errorf("second scope start = %d, want 5", pr.Scopes[1].Start)
	}

	// First function's vars should be invalidated — re-parse gives new result
	vars = pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "x") {
		t.Errorf("expected x in vars after edit: %v", vars)
	}
	if !slices.Contains(vars, "z") {
		t.Errorf("expected z in vars after edit: %v", vars)
	}

	// Second function should still work
	vars2 := pr.FuncVars(pr.Scopes[1].Start, pr.Scopes[1].End)
	if !slices.Contains(vars2, "y") {
		t.Errorf("expected y in second func vars: %v", vars2)
	}
}

func TestApplyEdit_GlobalScope(t *testing.T) {
	content := "component {\nfunction first() {\n\tvar x = 1\n}\n}"
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	// Add a new function after the existing one
	// Insert "function second() {\n\tvar y = 2\n}\n" before closing }
	kind := pr.ApplyEdit(4, 0, 4, 0, "function second() {\n\tvar y = 2\n}\n")
	if kind != EditGlobal {
		t.Errorf("expected EditGlobal, got %d", kind)
	}

	// Should now have 2 functions
	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs after edit, got %d", len(pr.Funcs))
	}
	if pr.Funcs[1].Name != "second" {
		t.Errorf("second func name = %q", pr.Funcs[1].Name)
	}
}

func TestApplyEdit_FullReplace(t *testing.T) {
	content := "component {\nfunction first() {}\n}"
	pr := Parse(testURI, content)

	newContent := "component {\nfunction replaced() {}\n}"
	pr.ApplyFullReplace(newContent)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}
	if pr.Funcs[0].Name != "replaced" {
		t.Errorf("func name = %q, want replaced", pr.Funcs[0].Name)
	}
}

func TestApplyEdit_ShiftsRefs(t *testing.T) {
	content := "component {\nfunction first() {\n\tvar x = 1\n}\nsvc = new services.Foo()\n}"
	pr := Parse(testURI, content)

	if len(pr.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(pr.Refs))
	}
	origLine := pr.Refs[0].Line

	// Insert a line inside the function
	kind := pr.ApplyEdit(2, 0, 2, 0, "\tvar y = 2\n")
	if kind != EditInFunc {
		t.Errorf("expected EditInFunc, got %d", kind)
	}

	// Ref should be shifted by 1
	if pr.Refs[0].Line != origLine+1 {
		t.Errorf("ref line = %d, want %d", pr.Refs[0].Line, origLine+1)
	}
}

func TestApplyEdit_PreservesOtherFuncCache(t *testing.T) {
	content := "component {\nfunction a() {\n\tvar x = 1\n}\nfunction b() {\n\tvar y = 2\n}\n}"
	pr := Parse(testURI, content)

	// Cache both functions
	_ = pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	_ = pr.FuncVars(pr.Scopes[1].Start, pr.Scopes[1].End)

	// Edit inside function a (no line change)
	pr.ApplyEdit(2, 5, 2, 10, "newVal")

	// Function b's cache key changed due to shift (no shift here since delta=0)
	// but should still be accessible
	vars := pr.FuncVars(pr.Scopes[1].Start, pr.Scopes[1].End)
	if !slices.Contains(vars, "y") {
		t.Errorf("expected y preserved in func b: %v", vars)
	}
}

func TestApplyEdit_TagFunction(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="save">
	<cfargument name="id" type="numeric">
	<cfset var localVar = 1>
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 || pr.Funcs[0].Name != "save" {
		t.Fatalf("expected func 'save', got %+v", pr.Funcs)
	}
	if len(pr.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(pr.Scopes))
	}

	// Get vars inside the tag function
	vars := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "localVar") {
		t.Errorf("expected localVar in tag func vars: %v", vars)
	}

	// Edit inside the function: add another cfset
	kind := pr.ApplyEdit(3, 0, 3, 0, "\t<cfset var newVar = 2>\n")
	if kind != EditInFunc {
		t.Errorf("expected EditInFunc, got %d", kind)
	}

	// Re-parse the function body
	vars = pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if !slices.Contains(vars, "localVar") {
		t.Errorf("expected localVar after edit: %v", vars)
	}
	if !slices.Contains(vars, "newVar") {
		t.Errorf("expected newVar after edit: %v", vars)
	}
}

func TestApplyEdit_MixedTagAndScript(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="tagFunc">
	<cfset var x = 1>
</cffunction>
<cfscript>
function scriptFunc() {
	var y = 2;
}
</cfscript>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d: %+v", len(pr.Funcs), pr.Funcs)
	}

	// Verify both functions are found
	names := []string{pr.Funcs[0].Name, pr.Funcs[1].Name}
	if !slices.Contains(names, "tagFunc") {
		t.Errorf("expected tagFunc in %v", names)
	}
	if !slices.Contains(names, "scriptFunc") {
		t.Errorf("expected scriptFunc in %v", names)
	}

	// Edit inside the script function (line 6 is "var y = 2;")
	kind := pr.ApplyEdit(6, 0, 6, 0, "\tvar z = 3;\n")
	if kind != EditInFunc {
		t.Errorf("expected EditInFunc for script func edit, got %d", kind)
	}

	// Verify the script function's vars are updated
	for i, sc := range pr.Scopes {
		vars := pr.FuncVars(sc.Start, sc.End)
		if pr.Funcs[i].Name == "scriptFunc" {
			if !slices.Contains(vars, "y") {
				t.Errorf("expected y in scriptFunc vars: %v", vars)
			}
			if !slices.Contains(vars, "z") {
				t.Errorf("expected z in scriptFunc vars: %v", vars)
			}
		}
	}
}

func TestApplyEdit_TagGlobalScope(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="existing">
	<cfset var x = 1>
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, content)

	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(pr.Funcs))
	}

	// Add a new tag function (edit in global scope)
	kind := pr.ApplyEdit(4, 0, 4, 0, "<cffunction name=\"newFunc\">\n\t<cfset var y = 2>\n</cffunction>\n")
	if kind != EditGlobal {
		t.Errorf("expected EditGlobal, got %d", kind)
	}

	if len(pr.Funcs) != 2 {
		t.Fatalf("expected 2 funcs after edit, got %d: %+v", len(pr.Funcs), pr.Funcs)
	}
}

func TestApplyEdit_EnterAfterClosingTag(t *testing.T) {
	src := "<cfcomponent>\n\t<cffunction name=\"foo\">\n\t\t<cfset var x = 1 />\n\t</cffunction>\n\n\t<cffunction name=\"bar\">\n\t\t<cfset var a = 1 />\n\t</cffunction>\n</cfcomponent>"
	pr := Parse("file:///test.cfc", src)

	// Press Enter at end of </cffunction> (after >)
	kind := pr.ApplyEdit(3, 15, 3, 15, "\n")
	if kind == EditInFunc {
		t.Error("Enter after </cffunction> should not be EditInFunc")
	}
}

func TestApplyEdit_InsertBeforeClosingBrace(t *testing.T) {
	src := "component {\nfunction doStuff() {\n\tvar x = 1\n}\nfunction other() {\n\tvar y = 2\n}\n}"
	pr := Parse(testURI, src)

	// Insert at beginning of } line (col 0) — before the brace
	kind := pr.ApplyEdit(3, 0, 3, 0, "\tvar z = 3\n")
	if kind != EditInFunc {
		t.Errorf("Insert before } should be EditInFunc, got %d", kind)
	}
}

func TestApplyEdit_EnterAfterClosingBrace(t *testing.T) {
	src := "component {\nfunction doStuff() {\n\tvar x = 1\n}\nfunction other() {\n\tvar y = 2\n}\n}"
	pr := Parse(testURI, src)

	// Press Enter after } (col 1)
	kind := pr.ApplyEdit(3, 1, 3, 1, "\n")
	if kind == EditInFunc {
		t.Error("Enter after } should not be EditInFunc")
	}
}

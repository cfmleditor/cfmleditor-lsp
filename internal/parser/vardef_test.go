package parser

import (
	"slices"
	"testing"
)

func TestVarsAt_FileScope(t *testing.T) {
	src := "var x = 1\nvar y = 2\nz = x + y"
	vars := VarsAt(src, 2)
	assertContains(t, vars, "x", "y", "z")
}

func TestVarsAt_LocalOnlyInsideFunction(t *testing.T) {
	src := `var fileVar = 1
function doStuff() {
	var localVar = 2
	local.other = 3
	localVar + other
}`
	// Inside function — local vars + file-scope vars visible
	vars := VarsAt(src, 4)
	assertContains(t, vars, "localVar", "other", "fileVar")

	// Outside function — local vars NOT visible
	vars = VarsAt(src, 0)
	assertContains(t, vars, "fileVar")
	assertNotContains(t, vars, "localVar", "other")
}

func TestVarsAt_VariablesScopeVisibleEverywhere(t *testing.T) {
	src := `function init() {
	variables.config = {}
	this.name = "test"
}
config`
	// variables.x and this.x visible outside the function
	vars := VarsAt(src, 4)
	assertContains(t, vars, "config", "name")
}

func TestVarsAt_PlainAssignIsVariablesScope(t *testing.T) {
	src := `function doStuff() {
	result = query()
}
result`
	// Plain assignment inside function is variables scope — visible outside
	vars := VarsAt(src, 3)
	assertContains(t, vars, "result")
}

func TestVarsAt_ArgumentsOnlyInsideFunction(t *testing.T) {
	src := `function doStuff() {
	arguments.id = 1
	id
}
outside`
	// arguments.x visible inside
	vars := VarsAt(src, 2)
	assertContains(t, vars, "id")

	// NOT visible outside
	vars = VarsAt(src, 4)
	assertNotContains(t, vars, "id")
}

func TestVarsAt_OnlyBeforeLine(t *testing.T) {
	src := "var a = 1\nvar b = 2\nvar c = 3"
	vars := VarsAt(src, 1)
	assertContains(t, vars, "a", "b")
	assertNotContains(t, vars, "c")
}

func TestVarsAt_TagFunction(t *testing.T) {
	src := `<cfset var pageVar = 1>
<cffunction name="myFunc">
	<cfset var localVar = 2>
	<cfset local.other = 3>
</cffunction>
<cfset pageVar>`
	// Inside tag function — local + file-scope visible
	vars := VarsAt(src, 3)
	assertContains(t, vars, "localVar", "other", "pageVar")

	// Outside — local NOT visible
	vars = VarsAt(src, 5)
	assertContains(t, vars, "pageVar")
	assertNotContains(t, vars, "localVar", "other")
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

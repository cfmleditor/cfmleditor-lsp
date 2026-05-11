package parser

import (
	"slices"
	"testing"
)

func TestGlobalVars_FileScope(t *testing.T) {
	src := "var x = 1\nvar y = 2\nz = x + y"
	vars := GlobalVars(src)
	assertContains(t, vars, "x", "y", "z")
}

func TestVarsInFunc_LocalOnly(t *testing.T) {
	src := `var fileVar = 1
function doStuff() {
	var localVar = 2
	local.other = 3
	localVar + other
}`
	vars := VarsInFunc(src, 1, 5)
	assertContains(t, vars, "localVar", "other")
	assertNotContains(t, vars, "fileVar")
}

func TestGlobalVars_VariablesAndThis(t *testing.T) {
	src := `variables.config = {}
this.name = "test"
function init() {
	var x = 1
}`
	vars := GlobalVars(src)
	assertContains(t, vars, "config", "name")
}

func TestGlobalVars_PlainAssignIsVariablesScope(t *testing.T) {
	src := `result = query()
function doStuff() {
	var local1 = 1
}`
	vars := GlobalVars(src)
	assertContains(t, vars, "result")
}

func TestVarsInFunc_Arguments(t *testing.T) {
	src := `function doStuff() {
	arguments.id = 1
	id
}
outside`
	vars := VarsInFunc(src, 0, 3)
	assertContains(t, vars, "id")

	// Not in global
	globals := GlobalVars(src)
	assertNotContains(t, globals, "id")
}

func TestVarsInFunc_TagFunction(t *testing.T) {
	src := `<cfset var pageVar = 1>
<cffunction name="myFunc">
	<cfset var localVar = 2>
	<cfset local.other = 3>
</cffunction>
<cfset pageVar>`
	vars := VarsInFunc(src, 1, 4)
	assertContains(t, vars, "localVar", "other")
	assertNotContains(t, vars, "pageVar")

	globals := GlobalVars(src)
	assertContains(t, globals, "pageVar")
	assertNotContains(t, globals, "localVar", "other")
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

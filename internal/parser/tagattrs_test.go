package parser

import (
	"slices"
	"strings"
	"testing"
)

// varsIn returns every variable name ParseVars finds in a cfscript fragment,
// in source order.
func varsIn(t *testing.T, body string) []string {
	t.Helper()

	var names []string
	for _, v := range ParseVars("<cfscript>\n" + body + "\n</cfscript>\n") {
		names = append(names, v.Name)
	}

	return names
}

func assertVars(t *testing.T, body string, want []string) {
	t.Helper()

	got := varsIn(t, body)
	if !slices.Equal(got, want) {
		t.Errorf("%s\n  got  %v\n  want %v", body, got, want)
	}
}

// A script-syntax CF tag's attributes are not assignments. The tag name is an
// ordinary identifier to the scanner, so without recognising the shape the
// parser read `query name="q"` as a statement assigning to a variable called
// `name` — and go-to-definition on `name` anywhere in the file then jumped to
// a cfquery attribute.
func TestScriptTagAttributesAreNotVariables(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"query", `query name="q" datasource="ds" { writeOutput("x"); }`, nil},
		{"savecontent", `savecontent variable="out" { writeOutput("x"); }`, nil},
		{"lock", `lock name="l" timeout="5" { hits = 1; }`, []string{"hits"}},
		{"thread", `thread name="t" action="run" { done = 1; }`, []string{"done"}},
		{"http", `http url="u" result="r" {}`, nil},
		{"http semicolon", `http url="u" result="r";`, nil},
		{"transaction", `transaction action="begin" {}`, nil},
		{"param", `param name="form.id" default="0";`, nil},
		{"interpolated value", `cfdirectory action="list" directory="#root#" name="dirs";`, nil},
		{"attribute holding a call", `cffile action="write" output=serialize(payload) file="f";`, nil},
		{"two tags in a row", `lock name="a" { x = 1; } lock name="b" { y = 2; }`, []string{"x", "y"}},

		// Controls: shapes that look similar and must keep working.
		{"plain assignment", `total = 1;`, []string{"total"}},
		{"assignment after a tag body", `query name="q" {} total = 1;`, []string{"total"}},
		{"braceless else", `if (a) x = 1; else y = 2;`, []string{"x", "y"}},
		{"typed function", `string function f() { return ""; }`, nil},
		{"property", `property name="person" type="models.Person";`, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertVars(t, c.body, c.want) })
	}
}

// A named argument is `name = value` inside a call's parentheses, and a struct
// literal key may be written the same way. Neither declares a variable. The
// argument list leaked because the dispatch that consumes it was gated behind
// call extraction, which ParseVars does not turn on, so its contents were
// scanned as statements.
func TestNamedArgumentsAndStructKeysAreNotVariables(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"named args, dotted receiver", `svc.save(force = true, mode = "x");`, nil},
		{"named args, bare call", `save(force = true);`, nil},
		{"named args, scoped receiver", `variables.svc.save(force = true);`, nil},
		{"named args, chained", `svc.get().save(force = true);`, nil},
		{"named args, nested call", `save(inner = other(force = true));`, nil},
		{"struct literal with =", `total = {force = true, mode = "x"};`, []string{"total"}},
		{"struct literal with :", `total = {force: true};`, []string{"total"}},
		{"var struct literal", `var total = {force = true};`, []string{"total"}},
		{"scoped struct literal", `variables.total = {force = true};`, []string{"total"}},
		{"nested struct literal", `total = {a = {force = true}};`, []string{"total"}},
		{"array of structs", `total = [{force = true}];`, []string{"total"}},

		// Controls.
		{"positional args", `svc.save(1, "x");`, nil},
		{"assignment after a call", `svc.save(force = true); total = 1;`, []string{"total"}},
		{"assignment after a struct", `a = {force = true}; total = 1;`, []string{"a", "total"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertVars(t, c.body, c.want) })
	}
}

// ParseVars and VariablesVars are two separate scans of the same file —
// scriptParser's full one and globalScriptParser's cheap outside-functions-only
// one — and every rule about what does *not* declare a variable has to hold in
// both. All three shapes here fooled both, and fixing one left the other
// reporting the same phantom names to hover and completion.
func TestBothVariableScansAgree(t *testing.T) {
	// Each line is a shape, paired with the names it must not be read as
	// declaring.
	cases := []struct {
		body    string
		phantom []string
	}{
		{`query name="q" datasource="ds" { writeOutput("x"); }`, []string{"name", "datasource"}},
		{`savecontent variable="out" { writeOutput("x"); }`, []string{"variable"}},
		{`lock name="l" timeout="5" { hits = 1; }`, []string{"name", "timeout"}},
		{`http url="u" result="r";`, []string{"url", "result"}},
		{`cfdirectory action="list" directory="#root#" name="dirs";`, []string{"action", "directory", "name"}},
		{`svc.save(force = true, mode = "x");`, []string{"force", "mode"}},
		{`save(force = true);`, []string{"force"}},
		{`variables.svc.save(force = true);`, []string{"force"}},
		{`total = {force = true};`, []string{"force"}},
		{`variables.total = {force = true, mode = "x"};`, []string{"force", "mode"}},
		{`this.total = [{force = true}];`, []string{"force"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			src := "component {\n" + c.body + "\n}\n"
			pr := Parse(testURI, src)

			scans := map[string][]string{
				"VariablesVars": pr.VariablesVars(),
				"ThisVars":      pr.ThisVars(),
			}

			for _, v := range pr.AllVars() {
				scans["ParseVars"] = append(scans["ParseVars"], v.Name)
			}

			for scan, names := range scans {
				for _, name := range names {
					if slices.ContainsFunc(c.phantom, func(p string) bool {
						return strings.EqualFold(p, name)
					}) {
						t.Errorf("%s reported %q as a variable", scan, name)
					}
				}
			}
		})
	}
}

// The `<` guard: parseFuncBody runs the *script* parser over a tag function's
// raw text, so `<cffunction name="save">` reaches the tag-attribute scan as an
// identifier followed by `name=`. Reading it as a script-syntax tag consumed the
// function body with it, and every local declared inside vanished.
func TestTagSyntaxIsNotReadAsScriptTagAttributes(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="save">
	<cfargument name="id" type="numeric">
	<cfset var localVar = 1>
	<cfset local.other = 2>
</cffunction>
</cfcomponent>`

	pr := Parse(testURI, content)

	// <cfargument> names are the tag parser's to report and have never been in
	// FuncVars, which runs the script parser over the body text.
	got := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	for _, want := range []string{"localVar", "other"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in FuncVars, got %v", want, got)
		}
	}
}

package parser

import (
	"fmt"
	"slices"
	"testing"
)

// refsByName lists every ref the parse made, global and per function, as
// "name->component [where]", where is "global" or the function's name and
// ":this" marks a ref assigned through this.
func refsByName(pr *ParseResult) []string {
	var out []string

	for i := range pr.ComponentRefs {
		out = append(out, refLabel(&pr.ComponentRefs[i], "global"))
	}

	for _, s := range pr.Scopes {
		refs, _ := pr.FuncRefs(s.Start, s.End)
		for i := range refs {
			out = append(out, refLabel(&refs[i], s.Name))
		}
	}

	slices.Sort(out)

	return slices.Compact(out)
}

func refLabel(r *ComponentRef, where string) string {
	if r.This {
		where += ":this"
	}

	return fmt.Sprintf("%s->%s [%s]", r.Variable, r.Component, where)
}

// TestChainedAssignmentTypesEveryName: `a = b = rhs` gives both names what
// rhs holds, in either syntax, each filed by its own scope's rule — `var a`
// is the function's, an unscoped b in a function is the component's, and a
// this. target is marked as one. Only a scoped chain used to work, and only
// in script; `a = b == c` is a comparison and assigns b nothing.
func TestChainedAssignmentTypesEveryName(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{
			"script, unscoped",
			"component {\n a = b = new models.X();\n}",
			[]string{"a->models.X [global]", "b->models.X [global]"},
		},
		{
			"script, three names",
			"component {\n a = b = c = createObject(\"component\", \"models.X\");\n}",
			[]string{"a->models.X [global]", "b->models.X [global]", "c->models.X [global]"},
		},
		{
			"script, var and unscoped in a function",
			"component {\n function f() {\n  var a = b = new models.X();\n }\n}",
			[]string{"a->models.X [f]", "b->models.X [global]"},
		},
		{
			"script, this. inside",
			"component {\n a = this.b = new models.X();\n}",
			[]string{"a->models.X [global]", "b->models.X [global:this]"},
		},
		{
			"script, a comparison is not a chain",
			"component {\n function f() {\n  var a = b == new models.X();\n }\n}",
			nil,
		},
		{
			"tag, unscoped",
			"<cfset a = b = createObject(\"component\", \"models.X\")>",
			[]string{"a->models.X [global]", "b->models.X [global]"},
		},
		{
			"tag, scoped both ways",
			"<cfset variables.a = this.b = createObject(\"component\", \"models.X\")>",
			[]string{"a->models.X [global]", "b->models.X [global:this]"},
		},
		{
			"tag, var and unscoped in a function",
			"<cfcomponent>\n<cffunction name=\"f\">\n<cfset var a = b = createObject(\"component\", \"models.X\")>\n</cffunction>\n</cfcomponent>",
			[]string{"a->models.X [f]", "b->models.X [global]"},
		},
		{
			"script, typed later by a pending call",
			"component {\n models.X function make() {}\n a = b = make();\n}",
			[]string{"a->models.X [global]", "b->models.X [global]"},
		},
		{
			"tag, typed later by a pending call",
			"<cfcomponent>\n<cffunction name=\"make\" returntype=\"models.X\">\n</cffunction>\n<cfset a = b = make()>\n</cfcomponent>",
			[]string{"a->models.X [global]", "b->models.X [global]"},
		},
		{
			"tag, a comparison is not a chain",
			"<cfset a = b == createObject(\"component\", \"models.X\")>",
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := refsByName(ParseWithOptions(testURI, tc.src, &ParseOptions{}))
			if !slices.Equal(got, tc.want) {
				t.Errorf("refs = %q\nwant   %q", got, tc.want)
			}
		})
	}
}

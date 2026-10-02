package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestReturnedFactoryChains(t *testing.T) {
	resolvers := []Resolver{{Match: `getBean("$1")`, Resolve: "models.$1", Prefix: "getBean"}}

	cases := []struct{ name, expr, other, declaration, want string }{
		{"factory", `getBean('Builder')`, "", "", "models.Builder"},
		{"fluent", `getBean('Builder').configure(value=nested(a=1)).build()`, "", "", "models.Product"},
		{"multiline", "getBean('Builder')\n.configure(value={x: nested()})\n.build()", "", "", "models.Product"},
		{"same paths", `getBean('Builder').build()`, `getBean('Product').load()`, "", "models.Product"},
		{"different paths", `getBean('Builder')`, `getBean('Product')`, "", ""},
		{"primitive path", `getBean('Builder')`, `false`, "", ""},
		{"primitive first", `false`, `getBean('Builder')`, "", ""},
		{"unknown first", `unknown`, `getBean('Builder')`, "", ""},
		{"unknown path", `getBean('Builder')`, `unknown`, "", ""},
		{"unknown method", `getBean('Builder').missing()`, "", "", ""},
		{"member read", `getBean('Builder').value`, "", "", ""},
		{"indexed result", `getBean('Builder')[1]`, "", "", ""},
		{"concatenation", `getBean('Builder') & 'suffix'`, "", "", ""},
		{"ternary", `flag ? getBean('Builder') : false`, "", "", ""},
		{"dotted contract", `getBean('Builder')`, "", "models.Contract", ""},
		{"bare contract", `getBean('Builder')`, "", "Contract", ""},
		{"generic contract", `getBean('Builder')`, "", "any", "models.Builder"},
		{"primitive contract", `getBean('Builder')`, "", "string", ""},
	}
	for _, syntax := range []string{"script", "tag", "mixed"} {
		for _, tc := range cases {
			t.Run(syntax+"/"+tc.name, func(t *testing.T) {
				var content string
				if syntax == "script" {
					content = fmt.Sprintf("component {\n%s function wrapper() {\nreturn %s;", tc.declaration, tc.expr)
					if tc.other != "" {
						content += "\nreturn " + tc.other + ";"
					}

					content += "\n}\nfunction consumer() { var item=wrapper(); item.work(); }\n}"
				} else {
					attr := ""
					if tc.declaration != "" {
						attr = ` returntype="` + tc.declaration + `"`
					}

					content = `<cfcomponent>
<cffunction name="wrapper"` + attr + `>`
					if syntax == "mixed" {
						content += "\n<cfscript>\nreturn " + tc.expr + ";\n</cfscript>\n"
					} else {
						content += "\n<cfreturn " + tc.expr + " />\n"
					}

					if tc.other != "" {
						content += "\n<cfreturn " + tc.other + " />\n"
					}

					content += `</cffunction>
<cffunction name="consumer">
<cfset var item=wrapper()>
<cfset item.work()>
</cffunction>
</cfcomponent>`
				}

				pr := ParseWithOptions(testURI, content, &ParseOptions{Resolvers: resolvers, FuncLookup: returnedFactoryLookup, ExtractCalls: true})
				found := false

				for _, f := range pr.Funcs {
					if f.Name == "wrapper" {
						found = true

						if f.ReturnComponent != tc.want {
							t.Errorf("return=%q want=%q", f.ReturnComponent, tc.want)
						}
					}
				}

				if !found {
					t.Fatal("wrapper missing")
				}

				if tc.want != "" && refsByVar(pr)["item"] != tc.want {
					t.Errorf("consumer type=%q want=%q", refsByVar(pr)["item"], tc.want)
				}
			})
		}
	}
}

func TestReturnedFactoryChainWithoutLookup(t *testing.T) {
	pr := ParseWithOptions(testURI, `component { function wrapper() { return getBean('Builder').build(); } }`, &ParseOptions{Resolvers: []Resolver{{Match: `getBean("$1")`, Resolve: "models.$1", Prefix: "getBean"}}})
	if pr.Funcs[0].ReturnComponent != "" {
		t.Errorf("unknown method acquired return %q", pr.Funcs[0].ReturnComponent)
	}
}

func TestReturnedFactoryPathsIgnoreClosureAndRefresh(t *testing.T) {
	options := &ParseOptions{Resolvers: []Resolver{{Match: `getBean("$1")`, Resolve: "models.$1", Prefix: "getBean"}}}
	content := `component {
 function wrapper() {
  var callback=function(){ return false; };
  return getBean('Builder');
 }
}`

	pr := ParseWithOptions(testURI, content, options)
	if pr.Funcs[0].ReturnComponent != "models.Builder" {
		t.Fatalf("closure changed enclosing return: %q", pr.Funcs[0].ReturnComponent)
	}

	pr.ApplyFullReplace(strings.ReplaceAll(content, "getBean('Builder')", "getBean('Product')"))

	if pr.Funcs[0].ReturnComponent != "models.Product" {
		t.Fatalf("stale return after full replacement: %q", pr.Funcs[0].ReturnComponent)
	}

	pr.ApplyFullReplace(strings.ReplaceAll(content, "return getBean('Builder');", "return getBean('Product');\n  return false;"))

	if pr.Funcs[0].ReturnComponent != "" {
		t.Fatalf("conflicting return after full replacement: %q", pr.Funcs[0].ReturnComponent)
	}
}

func returnedFactoryLookup(component, method string) string {
	switch strings.ToLower(component + "." + method) {
	case "models.builder.configure":
		return "models.Builder"
	case "models.builder.build":
		return "models.Product"
	case "models.product.load":
		return "models.Product"
	}

	return ""
}

func TestFactoryChainDeclinesHopsTheSplitterCannotSee(t *testing.T) {
	for _, expr := range []string{
		`getFactory("x").make( /* ) */ ).build()`,
		"getFactory(\"x\").make( // don't\n ).build()",
	} {
		if root, methods := FactoryCallChain(expr); root != "" {
			t.Errorf("%q: root %q methods %v, want declined", expr, root, methods)
		}
	}

	root, methods := FactoryCallChain(`getFactory("x").make( 1 ).build()`)
	if root != `getFactory("x")` || len(methods) != 2 || methods[0] != "make" || methods[1] != "build" {
		t.Fatalf("plain chain: root %q methods %v", root, methods)
	}
}

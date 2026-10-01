package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestUniformCollectionReturns(t *testing.T) {
	cases := []struct{ name, writes, returns, want string }{
		{"factory value", `built[key]=getBean('Site');`, `return variables.items[key];`, "models.Site"},
		{"factory method return", `built[key]=getBean('DAO').read();`, `return variables.items[key];`, "models.Site"},
		{"unknown factory method", `built[key]=getBean('DAO').missing();`, `return variables.items[key];`, ""},
		{"whole alias seed", `var alias=built; alias[key]=new models.Site();`, `return variables.items[key];`, "models.Site"},
		{"unwritten alias source", `built=missing; built[key]=new models.Site();`, `return variables.items[key];`, ""},
		{"whole alias mutation", `var alias=built; built[key]=new models.Site(); alias[other]=false;`, `return variables.items[key];`, ""},
		{"new", `built[key]=new models.Site();`, `return variables.items[key];`, "models.Site"},
		{"dot key", `built.first=new models.Site();`, `return variables.items[key];`, "models.Site"},
		{"createObject", `built[key]=createObject('component','models.Site').init();`, `return variables.items[key];`, "models.Site"},
		{"grounded cycle", `built[key]=variables.items[key]; built[other]=new models.Site();`, `return variables.items[key];`, "models.Site"},
		{"unknown write", `built[key]=new models.Site(); built[other]=arguments.value;`, `return variables.items[key];`, ""},
		{"primitive write", `built[key]=new models.Site(); built[other]=false;`, `return variables.items[key];`, ""},
		{"different component", `built[key]=new models.Site(); built[other]=new models.Other();`, `return variables.items[key];`, ""},
		{"ungrounded cycle", `built[key]=variables.items[key];`, `return variables.items[key];`, ""},
		{"empty source", `var empty=structNew(); built[key]=empty[key]; built[other]=new models.Site();`, `return variables.items[key];`, ""},
		{"unknown whole alias", `built[key]=new models.Site(); built=arguments.value;`, `return variables.items[key];`, ""},
		{"larger expression", `built[key]=new models.Site() & 'suffix';`, `return variables.items[key];`, ""},
		{"ternary", `built[key]=flag ? new models.Site() : false;`, `return variables.items[key];`, ""},
		{"method dependency", `built[key]=variables.dao.read(key);`, `return variables.items[key];`, "models.Site"},
		{"unknown method", `built[key]=variables.dao.missing(key);`, `return variables.items[key];`, ""},
		{"conflicting return", `built[key]=new models.Site();`, `return variables.items[key]; return false;`, ""},
		{"primitive first", `built[key]=new models.Site();`, `return false; return variables.items[key];`, ""},
		{"agreeing returns", `built[key]=new models.Site();`, `return variables.items[key]; return variables.items['default'];`, "models.Site"},
		{"unknown mutator", `built[key]=new models.Site(); structInsert(built,'other',arguments.value);`, `return variables.items[key];`, ""},
		{"member mutator", `built[key]=new models.Site(); built.append(arguments.value);`, `return variables.items[key];`, ""},
		{"anonymous closure binding", `var callback=function() {var built=structNew();built[key]=new models.Site();};`, `return variables.items[key];`, ""},
		{"arrow closure binding", `var callback=()=>{var built=structNew();built[key]=new models.Site();};`, `return variables.items[key];`, ""},
		{"dynamic local scope write", `built[key]=new models.Site(); local[arguments.property]=arguments.value;`, `return variables.items[key];`, ""},
		{"compound assignment", `built[key]=new models.Site(); built[key]+=1;`, `return variables.items[key];`, ""},
		{"increment", `built[key]=new models.Site(); built[key]++;`, `return variables.items[key];`, ""},
		{"dynamic scope write", `built[key]=new models.Site(); variables[arguments.property]=arguments.value;`, `return variables.items[key];`, ""},
		{"separate this scope", `built[key]=new models.Site();`, `return this.items[key];`, ""},
		{"local shadows receiver", `var dao=arguments.value; built[key]=dao.read();`, `return variables.items[key];`, ""},
		{"explicit field receiver", `var dao=arguments.value; built[key]=variables.dao.read();`, `return variables.items[key];`, "models.Site"},
		{"parent overwrite", `variables.nested.items=structNew(); variables.nested.items[key]=new models.Site(); variables.nested=arguments.value;`, `return variables.nested.items[key];`, ""},
		{"argument shadows field", `built[key]=new models.Site();`, `return items[key];`, ""},
	}
	for _, syntax := range []string{"script", "tag", "mixed"} {
		for _, tc := range cases {
			t.Run(syntax+"/"+tc.name, func(t *testing.T) {
				source := collectionFixture(syntax, tc.writes, tc.returns)
				pr := ParseWithOptions(testURI, source, &ParseOptions{Resolvers: []Resolver{{Match: `getBean("$1")`, Resolve: "models.$1", Prefix: "getBean"}}, FuncLookup: collectionTestLookup, ExtractCalls: true})

				fn := collectionFunction(t, pr, "one")
				if fn.ReturnComponent != tc.want {
					t.Errorf("return=%q want=%q", fn.ReturnComponent, tc.want)
				}

				if collectionFunction(t, pr, "all").ReturnComponent != "" {
					t.Error("whole struct given element's component")
				}
			})
		}
	}
}

func collectionTestLookup(component, method string) string {
	if strings.EqualFold(component, "models.DAO") && strings.EqualFold(method, "read") {
		return "models.Site"
	}

	return ""
}

func collectionFunction(t *testing.T, pr *ParseResult, name string) *FunctionDef {
	t.Helper()

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, name) {
			return &pr.Funcs[i]
		}
	}

	t.Fatalf("function %s missing", name)

	return nil
}

func collectionFixture(syntax, writes, returns string) string {
	if syntax == "script" {
		return fmt.Sprintf(`component {
 variables.items=structNew();
 variables.dao=new models.DAO();
 function populate() {
  var built=structNew();
  %s
  variables.items=built;
 }
 function one(items) {
  %s
 }
 function all() { return variables.items; }
}`, writes, returns)
	}

	set := func(code string) string {
		var out strings.Builder

		for statement := range strings.SplitSeq(code, ";") {
			if strings.TrimSpace(statement) != "" {
				out.WriteString("<cfset\t" + statement + ">\n")
			}
		}

		return out.String()
	}
	ret := func(code string) string {
		var out strings.Builder

		for statement := range strings.SplitSeq(code, ";") {
			if strings.TrimSpace(statement) != "" {
				out.WriteString("<cfreturn " + strings.TrimPrefix(strings.TrimSpace(statement), "return ") + ">\n")
			}
		}

		return out.String()
	}

	body := set("var built=structNew();" + writes + "variables.items=built;")
	if strings.Contains(writes, "function(") || strings.Contains(writes, "=>") {
		body = "<cfscript>var built=structNew();" + writes + "variables.items=built;</cfscript>"
	}

	returned := ret(returns)

	if syntax == "mixed" {
		body = "<cfscript>\nvar built=structNew();\n" + writes + "\nvariables.items=built;\n</cfscript>"
		returned = "<cfscript>\n" + returns + "\n</cfscript>"
	}

	return "<cfcomponent>\n" + strings.ReplaceAll(set("variables.items=structNew();variables.dao=new models.DAO();"), "<cfset\t", "<cfset ") + "<cffunction name=\"populate\">\n" + body + "\n</cffunction>\n<cffunction name=\"one\">\n<cfargument name=\"items\">\n" + returned + "\n</cffunction>\n<cffunction name=\"all\">\n<cfreturn variables.items>\n</cffunction>\n</cfcomponent>"
}

func TestCollectionTagOutputRejectsType(t *testing.T) {
	for _, tag := range []string{
		`<cfquery name="variables.items">select 1</cfquery>`,
		`<cfobject name="variables.items" component="models.Other">`,
		`<cfsavecontent variable="variables.items">primitive</cfsavecontent>`,
		`<cfquery result="variables.items">select 1</cfquery>`,
		`<cfinvoke component="models.Other" method="read" returnvariable="variables.items">`,
		`<cfloop array="#arguments.values#" item="variables.items"></cfloop>`,
		`<cfparam name="variables.items[other]" default="false">`,
	} {
		t.Run(tag, func(t *testing.T) {
			source := collectionFixture("tag", `built[key]=new models.Site();`, `return variables.items[key];`)
			source = strings.Replace(source, `</cffunction>`, tag+`</cffunction>`, 1)
			pr := ParseWithOptions(testURI, source, &ParseOptions{FuncLookup: collectionTestLookup})

			f := collectionFunction(t, pr, "one")
			if f.ReturnComponent != "" || len(f.ReturnSources) != 0 {
				t.Fatalf("unsafe collection contract: %+v", f)
			}
		})
	}
}

func TestCollectionReturnDeclaredContract(t *testing.T) {
	for _, declared := range []string{"query", "models.Other"} {
		source := collectionFixture("script", `built[key]=new models.Site();`, `return variables.items[key];`)
		source = strings.Replace(source, "function one", declared+" function one", 1)
		pr := Parse(testURI, source)

		f := collectionFunction(t, pr, "one")
		if f.ReturnType != declared || f.ReturnComponent != "" || len(f.ReturnSources) != 0 {
			t.Fatalf("declaration overwritten: %+v", f)
		}
	}
}

func TestCollectionArgumentDefaultDoesNotConstrainCaller(t *testing.T) {
	pr := Parse(testURI, `component {
  function one(items=structNew()) {
   items[key]=new models.Site();
   return items[key];
  }
 }`)

	f := collectionFunction(t, pr, "one")
	if f.ReturnComponent != "" || len(f.ReturnSources) != 0 {
		t.Fatalf("caller-supplied collection inferred: %+v", f)
	}
}

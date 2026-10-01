package parser

import (
	"strings"
	"testing"
)

func TestExplicitMemberBindings(t *testing.T) {
	for _, syntax := range []string{"script", "tag"} {
		for _, tc := range []struct{ name, writes, want string }{
			{"factory member", `rc.$=getBean('$');`, "models.Scope"},
			{"constructor member", `rc.$=new models.Scope();`, "models.Scope"},
			{"scalar alias", `var bean=new models.Scope();rc.$=bean;`, "models.Scope"},
			{"unknown overwrite", `rc.$=new models.Scope();rc.$=arguments.value;`, ""},
			{"conflicting write", `rc.$=new models.Scope();rc.$=new models.Other();`, ""},
			{"dynamic member write", `rc.$=new models.Scope();rc[arguments.key]=arguments.value;`, ""},
			{"scope write", `variables.rc.$=new models.Scope();variables[arguments.key]=arguments.value;`, ""},
			{"parent overwrite", `rc.$=new models.Scope();rc=arguments.value;`, ""},
		} {
			t.Run(syntax+"/"+tc.name, func(t *testing.T) {
				source := "component {\n function run(rc,value) {\n" + tc.writes + "\nrc.$.work();\n }\n}"
				if syntax == "tag" {
					source = `<cfcomponent><cffunction name="run"><cfargument name="rc"><cfargument name="value">` + strings.ReplaceAll("<cfset "+strings.TrimSuffix(tc.writes, ";")+">", ";", "><cfset ") + `<cfset rc.$.work()></cffunction></cfcomponent>`
				}

				pr := ParseWithOptions(testURI, source, &ParseOptions{Resolvers: []Resolver{{Match: `getBean("$1")`, Resolve: "models.Scope", Prefix: "getBean"}}, ExtractCalls: true})
				found := false

				for _, s := range pr.Scopes {
					for _, ref := range append(pr.FuncComponentRefsAt(s.Start, s.End, uint32(s.End)), pr.ComponentRefs...) {
						if ref.Variable == "rc.$" {
							found = true

							if ref.Component != tc.want {
								t.Errorf("member=%q want=%q", ref.Component, tc.want)
							}
						}

						if ref.Variable == "rc" && ref.Component != "" {
							t.Error("container given member's type")
						}
					}
				}

				if !found {
					t.Fatal("member binding not recorded")
				}
			})
		}
	}
}

func TestLiteralStartupServiceLoop(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		count        int
	}{
		{"literal list", `variables.services="first,second";for(variables.i in listToArray(variables.services)){application["#variables.i#"]=variables.factory.getBean("#variables.i#");}`, 2},
		{"inline list", `for(i in listToArray("first,second")){request["#i#"]=variables.factory.getBean("#i#");}`, 2},
		{"computed list", `variables.services=arguments.services;for(i in listToArray(variables.services)){application["#i#"]=variables.factory.getBean("#i#");}`, 0},
		{"different key", `for(i in listToArray("first")){application["#other#"]=variables.factory.getBean("#i#");}`, 0},
		{"iterator replacement", `for(i in listToArray("first")){i=arguments.value;application["#i#"]=variables.factory.getBean("#i#");}`, 0},
		{"incremented iterator", `for(i in listToArray("first")){i++;application["#i#"]=variables.factory.getBean("#i#");}`, 0},
		{"nested loop", `for(i in listToArray("first")){for(j in listToArray("second")){application["#i#"]=variables.factory.getBean("#i#");}}`, 0},
		{"comment", `/*for(i in listToArray("first")){application["#i#"]=variables.factory.getBean("#i#");}*/`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := StartupBeanBindings(tc.source); len(got) != tc.count {
				t.Fatalf("bindings=%+v", got)
			}
		})
	}
}

func TestMemberBindingsRefreshAfterBodyEdit(t *testing.T) {
	source := "component {\n function run(rc) {\n rc.$=new models.Scope();\n rc.$.work();\n }\n}"
	pr := ParseWithOptions(testURI, source, &ParseOptions{ExtractCalls: true})
	check := func(want string) {
		t.Helper()

		scope := pr.Scopes[0]
		found := false

		for _, ref := range pr.FuncComponentRefsAt(scope.Start, scope.End, 3) {
			if ref.Variable == "rc.$" {
				found = true

				if ref.Component != want {
					t.Fatalf("member=%q want %q", ref.Component, want)
				}
			}
		}

		if !found && want != "" {
			t.Fatal("missing member after edit")
		}
	}
	check("models.Scope")
	pr.ApplyEdit(2, 0, 2, len(" rc.$=new models.Scope();"), " rc.$=new models.Other();")
	check("models.Other")
	pr.ApplyFullReplace(strings.ReplaceAll(source, "new models.Scope()", "arguments.value"))
	check("")
}

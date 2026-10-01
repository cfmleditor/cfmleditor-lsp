package parser

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

func TestManagedSetterTypes(t *testing.T) {
	for _, syntax := range []string{"script", "tag", "mixed"} {
		t.Run(syntax, func(t *testing.T) {
			dep := filepath.Join(t.TempDir(), "beans", "Service.cfc")

			source := `component accessors=true {
    property name="service";
    function setService(any service) { arguments.service.run(); variables.service = arguments.service; }
    function unrelated(service) { service.run(); }
    function setLabel(string label) { variables.label = arguments.label; }
    private function setPrivate(private) { variables.hidden = arguments.private; }
    function setMultiple(multiple, extra) { variables.multiple = arguments.multiple; }
    function setWrong(different) { variables.wrong = arguments.different; }
    function setMember(member) { variables.member = arguments.member.child; }
    function setConcat(concat) { variables.concat = arguments.concat & "value"; }
    function setDeclared(models.Explicit declared) { variables.declared = arguments.declared; }
    function setPublic(public) { this.public = arguments.public; }
   }`
			if syntax != "script" {
				source = `<cfcomponent accessors="true">
    <cfproperty name="service">
    <cffunction name="setService"><cfargument name="service" type="any"><cfset arguments.service.run()><cfset variables.service = arguments.service></cffunction>
    <cffunction name="unrelated"><cfargument name="service"><cfset service.run()></cffunction>
    <cffunction name="setLabel"><cfargument name="label" type="string"><cfset variables.label = arguments.label></cffunction>
    <cffunction name="setPrivate" access="private"><cfargument name="private"><cfset variables.hidden = arguments.private></cffunction>
    <cffunction name="setMultiple"><cfargument name="multiple"><cfargument name="extra"><cfset variables.multiple = arguments.multiple></cffunction>
    <cffunction name="setWrong"><cfargument name="different"><cfset variables.wrong = arguments.different></cffunction>
    <cffunction name="setMember"><cfargument name="member"><cfset variables.member = arguments.member.child></cffunction>
    <cffunction name="setConcat"><cfargument name="concat"><cfset variables.concat = arguments.concat & "value"></cffunction>
    <cffunction name="setDeclared"><cfargument name="declared" type="models.Explicit"><cfset variables.declared = arguments.declared></cffunction>
    <cffunction name="setPublic"><cfargument name="public"><cfset this.public = arguments.public></cffunction>
   </cfcomponent>`
			}

			if syntax == "mixed" {
				source = strings.Replace(source, `<cfset variables.service = arguments.service>`, `<cfscript>variables.service = arguments.service;</cfscript><cfset variables.second = arguments.service>`, 1)
			}

			pr := ParseWithOptions(uri.URI("file:///managed/Consumer.cfc"), source, &ParseOptions{SetterLookup: func(string) string { return dep }})
			found := map[string]string{}

			for _, ref := range pr.ComponentRefs {
				if ref.This {
					continue
				}

				found[strings.ToLower(ref.Variable)] = ref.Component
			}

			if found["service"] != dep {
				t.Errorf("setter field got %q, want %q", found["service"], dep)
			}

			if syntax == "mixed" && found["second"] != dep {
				t.Errorf("continued tag region lost argument type: %q", found["second"])
			}

			if found["declared"] != "models.Explicit" {
				t.Errorf("declared type overwritten: %q", found["declared"])
			}

			for _, name := range []string{"label", "hidden", "multiple", "wrong", "member", "concat", "public"} {
				if found[name] == dep {
					t.Errorf("non-dependency %s typed as service", name)
				}
			}

			checkSetterFunctions(t, pr, dep)
		})
	}
}

func checkSetterFunctions(t *testing.T, pr *ParseResult, dep string) {
	t.Helper()

	setterCount := 0

	for i := range pr.Funcs {
		fn := &pr.Funcs[i]
		if fn.Name == "setService" {
			if fn.Arguments[0].Type != "any" {
				t.Errorf("declared signature changed: %s", fn.Arguments[0].Type)
			}

			if fn.Arguments[0].Component != dep {
				t.Errorf("inferred dependency missing: %s", fn.Arguments[0].Component)
			}

			setterCount++
		}

		if fn.Name == "getService" && fn.ReturnComponent != dep {
			t.Errorf("getter type %q", fn.ReturnComponent)
		}

		if fn.Name == "unrelated" && fn.Arguments[0].Type != "" {
			t.Errorf("sibling argument leaked: %+v", fn.Arguments)
		}
	}

	if setterCount != 1 {
		t.Errorf("duplicate setter definitions: %d", setterCount)
	}
}

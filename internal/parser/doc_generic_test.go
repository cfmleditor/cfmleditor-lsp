package parser

import (
	"fmt"
	"testing"
)

func TestDocGenericArgumentType(t *testing.T) {
	for _, tc := range []struct{ annotation, declared, want string }{
		{"models.User", "", "models.User"},
		{"models.User", "any", "models.User"},
		{"models.User", "struct", "models.User"},
		{"models.User", "models.Other", "models.Other"},
		{"models.User", "array", "array"},
		{"models.User[]", "any", "any"},
		{"A prose description.", "any", "any"},
	} {
		for _, tag := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/tag=%t", tc.annotation, tc.declared, tag), func(t *testing.T) {
				content := fmt.Sprintf("component {\n/** @employee.doc_generic %s */\nfunction save(%s employee) { return employee; }\n}", tc.annotation, tc.declared)
				if tag {
					content = fmt.Sprintf("<cfcomponent>\n<!--- @employee.doc_generic %s --->\n<cffunction name=\"save\"><cfargument name=\"employee\" type=\"%s\"><cfreturn employee></cffunction>\n</cfcomponent>", tc.annotation, tc.declared)
				}

				pr := Parse(testURI, content)
				if len(pr.Funcs) != 1 || len(pr.Funcs[0].Arguments) != 1 {
					t.Fatal("expected one function with one argument")
				}

				if got := pr.Funcs[0].Arguments[0].Type; got != tc.want {
					t.Errorf("argument type = %q, want %q", got, tc.want)
				}
			})
		}
	}
}

func TestDocGenericPropertyType(t *testing.T) {
	for _, tc := range []struct{ annotation, declared, want string }{
		{"models.User", "", "models.User"},
		{"models.User", "any", "models.User"},
		{"models.User", "struct", "models.User"},
		{"models.User", "models.Other", "models.Other"},
		{"models.User", "Other", "Other"},
		{"models.User", "array", ""},
		{"models.User", "string", ""},
		{"models.User[]", "any", ""},
		{"A prose description.", "any", ""},
	} {
		for _, tag := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/tag=%t", tc.annotation, tc.declared, tag), func(t *testing.T) {
				content := fmt.Sprintf(`component { property name="employee" type="%s" doc_generic="%s"; }`, tc.declared, tc.annotation)
				if tag {
					content = fmt.Sprintf(`<cfcomponent><cfproperty name="employee" type="%s" doc_generic="%s"></cfcomponent>`, tc.declared, tc.annotation)
				}

				pr := Parse(testURI, content)
				if len(pr.Funcs) != 2 {
					t.Fatalf("expected two property accessors, got %v", pr.Funcs)
				}

				for _, f := range pr.Funcs {
					if f.Name == "getEmployee" && f.ReturnComponent != tc.want {
						t.Errorf("getter component = %q, want %q", f.ReturnComponent, tc.want)
					}
				}

				if tc.want == "" && len(pr.ComponentRefs) != 0 {
					t.Errorf("unexpected property refs: %v", pr.ComponentRefs)
				}
			})
		}
	}
}

func TestTypedArgumentDoesNotLeakToOtherFunction(t *testing.T) {
	for _, declaration := range []string{"required models.User employee", "any employee"} {
		content := fmt.Sprintf(`component {
/** @employee.doc_generic models.User */
function first(%s) { return employee; }
function second(employee) { return employee; }
}`, declaration)

		pr := Parse(testURI, content)
		if len(pr.Funcs) != 2 {
			t.Fatal("expected two functions")
		}

		if got := pr.Funcs[0].ReturnComponent; got != "models.User" {
			t.Errorf("first return = %q, want models.User", got)
		}

		if got := pr.Funcs[1].ReturnComponent; got != "" {
			t.Errorf("second return = %q, want no inferred component", got)
		}
	}
}

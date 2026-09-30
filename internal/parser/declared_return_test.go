package parser

import (
	"fmt"
	"testing"
)

func TestDeclaredPrimitiveReturnDoesNotPropagateComponent(t *testing.T) {
	for _, typ := range []string{"string", "STRING", "numeric", "boolean", "date", "struct", "array", "query", "binary", "guid", "uuid", "void", "xml"} {
		for _, tag := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tag=%t", typ, tag), func(t *testing.T) {
				content := fmt.Sprintf(`component {
	%s function value() { var result = new services.ConfigService(); return result; }
	function forwarded() { var result = value(); return result; }
}`, typ)
				if tag {
					content = fmt.Sprintf(`<cfcomponent>
<cffunction name="value" returntype="%s"><cfset var result = new services.ConfigService()><cfreturn result></cffunction>
<cffunction name="forwarded"><cfset var result = value()><cfreturn result></cffunction>
</cfcomponent>`, typ)
				}

				pr := Parse(testURI, content)
				if len(pr.Funcs) != 2 {
					t.Fatalf("got %d functions, want 2", len(pr.Funcs))
				}

				for i := range pr.Funcs {
					if f := &pr.Funcs[i]; f.ReturnComponent != "" {
						t.Errorf("%s returns component %q despite declared %s", f.Name, f.ReturnComponent, typ)
					}
				}
			})
		}
	}
}

func TestPrimitiveReturnDoesNotInferThis(t *testing.T) {
	for _, content := range []string{
		`component { string function value() { return this; } function forwarded() { var result = value(); return result; } }`,
		`<cfcomponent><cffunction name="value" returntype="string"><cfreturn this></cffunction><cffunction name="forwarded"><cfset var result = value()><cfreturn result></cffunction></cfcomponent>`,
	} {
		pr := Parse(testURI, content)
		if len(pr.Funcs) != 2 {
			t.Fatalf("got %d functions, want 2", len(pr.Funcs))
		}

		for i := range pr.Funcs {
			if f := &pr.Funcs[i]; f.ReturnComponent != "" {
				t.Errorf("%s returns component %q despite declared string", f.Name, f.ReturnComponent)
			}
		}
	}
}

func TestPrimitiveReturnKeepsRuntimeComponentDynamic(t *testing.T) {
	for _, content := range []string{
		`component { struct function value() { var result = createObject("component", "objs.#arguments.type#"); return result; } }`,
		`<cfcomponent><cffunction name="value" returntype="struct"><cfset var result = createObject("component", "objs.#arguments.type#")><cfreturn result></cffunction></cfcomponent>`,
	} {
		pr := Parse(testURI, content)
		if len(pr.Funcs) != 1 {
			t.Fatalf("got %d functions, want 1", len(pr.Funcs))
		}

		if got := pr.Funcs[0].ReturnComponent; got != "$any" {
			t.Errorf("runtime component return = %q, want dynamic $any", got)
		}
	}
}

func TestGenericReturnStillInfersComponent(t *testing.T) {
	for _, typ := range []string{"", "any", "ANY", "component", "services.ConfigService"} {
		t.Run(typ, func(t *testing.T) {
			content := fmt.Sprintf(`component {
	%s function value() { var result = new services.ConfigService(); return result; }
	function forwarded() { var result = value(); return result; }
}`, typ)
			if typ == "component" {
				content = `<cfcomponent>
<cffunction name="value" returntype="component"><cfset var result = new services.ConfigService()><cfreturn result></cffunction>
<cffunction name="forwarded"><cfset var result = value()><cfreturn result></cffunction>
</cfcomponent>`
			}

			pr := Parse(testURI, content)
			if len(pr.Funcs) != 2 {
				t.Fatalf("got %d functions, want 2", len(pr.Funcs))
			}

			for i := range pr.Funcs {
				if f := &pr.Funcs[i]; f.ReturnComponent != "services.ConfigService" {
					t.Errorf("%s returns component %q, want services.ConfigService", f.Name, f.ReturnComponent)
				}
			}
		})
	}
}

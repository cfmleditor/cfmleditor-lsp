package parser

import (
	"fmt"
	"testing"
)

func TestDeclaredPrimitiveReturnDoesNotPropagateComponent(t *testing.T) {
	for _, typ := range []string{"string", "STRING", "numeric", "boolean", "date", "array", "query", "binary", "guid", "uuid", "void", "xml"} {
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
	for _, typ := range []string{"", "any", "ANY", "component", "struct", "Struct", "services.ConfigService"} {
		t.Run(typ, func(t *testing.T) {
			content := fmt.Sprintf(`component {
	%s function value() { var result = new services.ConfigService(); return result; }
	function forwarded() { var result = value(); return result; }
}`, typ)
			if typ == "component" || typ == "Struct" {
				content = `<cfcomponent>
<cffunction name="value" returntype="` + typ + `"><cfset var result = new services.ConfigService()><cfreturn result></cffunction>
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

// A call made on another receiver is not answered by this file's function of
// the same name: `temp = fileIO.init(path)` in tassweb's javautils is init on
// a java.io.File, and this file's init — `return this`, declared struct —
// typed it as javautils.
func TestAQualifiedCallIsNotTheFilesOwnFunction(t *testing.T) {
	for name, content := range map[string]string{
		"script": `component {
	struct function init() { return this; }
	function list() { var temp = fileIO.init( "x" ); return temp; }
}`,
		"tag": `<cfcomponent>
<cffunction name="init" returntype="struct"><cfreturn this></cffunction>
<cffunction name="list"><cfset var temp = fileIO.init("x")><cfreturn temp></cffunction>
</cfcomponent>`,
	} {
		t.Run(name, func(t *testing.T) {
			pr := Parse(testURI, content)
			for i := range pr.Funcs {
				if f := &pr.Funcs[i]; f.Name == "list" && f.ReturnComponent != "" {
					t.Errorf("list returns %q; fileIO.init() is not this file's init", f.ReturnComponent)
				}
			}
		})
	}
}

// A resolver that describes the receiver of a call does not describe what the
// call returns: tassweb's `subsObj\.([a-zA-Z]+)` matches the start of
// `variables.subsObj.subsLib.startCFC("x")`, and obj is what startCFC
// returns.
func TestAResolverForTheReceiverDoesNotTypeTheCall(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="open" returntype="struct">
	<cfset var obj = variables.subsObj.subsLib.startCFC("x")>
	<cfreturn obj>
</cffunction>
<cffunction name="lib">
	<cfset var lib = variables.subsObj.subsLib>
	<cfreturn lib>
</cffunction>
</cfcomponent>`

	resolvers := []Resolver{{Match: `(?:subsObj|_objSubs)\.([a-zA-Z]+)`, Resolve: "subservices.$1.service", Prefix: "subsObj|_objSubs"}}

	for startCFC, want := range map[string]string{"pkg.Made": "pkg.Made", "": ""} {
		lookup := func(component, method string) string {
			if component == "subservices.subsLib.service" && method == "startCFC" {
				return startCFC
			}

			return ""
		}

		pr := ParseWithOptions(testURI, src, &ParseOptions{Resolvers: resolvers, FuncLookup: lookup})

		for i := range pr.Funcs {
			f := &pr.Funcs[i]

			switch f.Name {
			case "open":
				if f.ReturnComponent != want {
					t.Errorf("startCFC returning %q: open returns %q, want %q", startCFC, f.ReturnComponent, want)
				}
			case "lib":
				if f.ReturnComponent != "subservices.subsLib.service" {
					t.Errorf("lib returns %q, want the subservice itself", f.ReturnComponent)
				}
			}
		}
	}
}

// A call on this file is still answered by its own functions: on `this`, and
// on a variable holding this file (hyper's `this.clone()`, Slatwall's
// `var node = this; node = node.getParent()`).
func TestACallOnThisFileIsTheFilesOwnFunction(t *testing.T) {
	content := `component {
	models.Node function clone() { return new models.Node(); }
	models.Node function getParent() { return new models.Node(); }
	function viaThis() { var copy = this.clone(); return copy; }
	function viaSelf() { var node = this; var up = node.getParent(); return up; }
	function viaAlias() { this.self = this; var up = this.self.getParent(); return up; }
}`

	pr := Parse(testURI, content)
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if (f.Name == "viaThis" || f.Name == "viaSelf" || f.Name == "viaAlias") && f.ReturnComponent != "models.Node" {
			t.Errorf("%s returns %q, want models.Node", f.Name, f.ReturnComponent)
		}
	}
}

// A struct function that also returns a plain struct returns either, so the
// component it returns on another path does not type it.
func TestAStructFunctionReturningALiteralIsNotTyped(t *testing.T) {
	for name, content := range map[string]string{
		"script": `component { struct function pick() { if ( flag ) { return {}; } return new services.ConfigService(); } }`,
		"tag": `<cfcomponent><cffunction name="pick" returntype="struct">
<cfif flag><cfreturn StructNew()></cfif>
<cfreturn createObject("component", "services.ConfigService")>
</cffunction></cfcomponent>`,
	} {
		t.Run(name, func(t *testing.T) {
			pr := Parse(testURI, content)
			if got := pr.Funcs[0].ReturnComponent; got != "" {
				t.Errorf("pick returns %q; one path returns a plain struct", got)
			}
		})
	}
}

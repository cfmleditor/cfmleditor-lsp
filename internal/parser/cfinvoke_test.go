package parser

import "testing"

func invokeFunction(t *testing.T, pr *ParseResult, name string) *FunctionDef {
	t.Helper()

	for i := range pr.Funcs {
		if pr.Funcs[i].Name == name {
			return &pr.Funcs[i]
		}
	}

	t.Fatalf("function %s not found in %+v", name, pr.Funcs)

	return nil
}

// kernel2.getSandBox: result first holds the sandbox service, then what a
// computed method of it returns. The function returns the second, which the
// source cannot name.
func TestCfInvokeOfAComputedMethodIsDynamic(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="getSandBox" returntype="any">
	<cfargument name="name" type="string">
	<cfset var result = getService("sandbox")>
	<cfinvoke component="#result#" method="get#ARGUMENTS.name#" returnvariable="result" argumentcollection="#ARGUMENTS#" />
	<cfreturn result />
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, src, &ParseOptions{
		Resolvers: []Resolver{{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"}},
	})

	if got := invokeFunction(t, pr, "getSandBox").ReturnComponent; got != "$any" {
		t.Fatalf("getSandBox returns %q, want $any", got)
	}
}

// returnvariable holds what the method returns, not the component invoked.
func TestCfInvokeReturnVariableHoldsTheMethodsReturn(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="run">
	<cfinvoke component="models.Loader" method="load" returnvariable="thing" />
	<cfreturn thing />
</cffunction>
<cffunction name="viaObject">
	<cfset var loader = createObject("component", "models.Loader")>
	<cfinvoke component="#loader#" method="load" returnvariable="thing" />
	<cfreturn thing />
</cffunction>
</cfcomponent>`

	lookup := func(component, method string) string {
		if component == "models.Loader" && method == "load" {
			return "models.Thing"
		}

		return ""
	}

	pr := ParseWithOptions(testURI, src, &ParseOptions{FuncLookup: lookup})

	for _, name := range []string{"run", "viaObject"} {
		if got := invokeFunction(t, pr, name).ReturnComponent; got != "models.Thing" {
			t.Errorf("%s returns %q, want models.Thing", name, got)
		}
	}
}

// A function returns what its variable holds at the return, so a later
// assignment outranks an earlier one, in either syntax.
func TestReturnedVariableTakesItsLatestAssignment(t *testing.T) {
	for name, src := range map[string]string{
		"script": `component {
	function make() {
		var x = new models.First();
		x = new models.Second();
		return x;
	}
}`,
		"tag": `<cfcomponent>
<cffunction name="make">
	<cfset var x = createObject("component", "models.First")>
	<cfset x = createObject("component", "models.Second")>
	<cfreturn x>
</cffunction>
</cfcomponent>`,
	} {
		t.Run(name, func(t *testing.T) {
			pr := ParseWithOptions(testURI, src, &ParseOptions{})
			if got := invokeFunction(t, pr, "make").ReturnComponent; got != "models.Second" {
				t.Fatalf("make returns %q, want models.Second", got)
			}
		})
	}
}

func TestInvokeExpression(t *testing.T) {
	for _, c := range []struct{ component, method, want string }{
		{"models.Loader", "load", `createObject("component", "models.Loader").load()`},
		{"#loader#", "load", "loader.load()"},
		{"#VARIABLES.svc#", "get", "VARIABLES.svc.get()"},
		{"#result#", "get#name#", ""},
		{"#getSvc()#", "load", ""},
		{"models.#kind#", "load", ""},
		{"", "load", ""},
		{"models.Loader", "", ""},
	} {
		if got := invokeExpression(c.component, c.method); got != c.want {
			t.Errorf("invokeExpression(%q, %q) = %q, want %q", c.component, c.method, got, c.want)
		}
	}
}

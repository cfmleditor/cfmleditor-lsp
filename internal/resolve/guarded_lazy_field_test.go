package resolve

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func TestGuardedLazyFieldsReturnTheirInitializedType(t *testing.T) {
	tests := []struct {
		name, guard, initial string
	}{
		{"presence", `!structKeyExists(variables,"service")`, ""},
		{"primitive sentinel", `isSimpleValue(variables.service)`, `variables.service="";`},
		{"null", `isNull(variables.service)`, ""},
		{"unrelated unsupported method", `isNull(variables.service)`, `function unrelated(flag){switch(flag){case 1:var temp="x";break;}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Service.cfc": `component { function ready(){} }`,
				"Owner.cfc":   `component {` + tc.initial + ` function getService(){if(` + tc.guard + `){variables.service=new Service();}return variables.service;} }`,
				"Page.cfc":    `component {function run(){new Owner().getService().ready();}}`,
			})

			expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
				"getService.ready": "",
			})
		})
	}
}

// TestAGuardedLazyFieldAcceptsAnInheritedArgumentSensitiveSelfReturn reproduces
// settingsBean.getRazunaSettings: a bean factory returns a concrete ORM bean,
// whose inherited loadBy method returns the receiver for its default mode.
func TestAGuardedLazyFieldAcceptsAnInheritedArgumentSensitiveSelfReturn(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Base.cfc": `component {
function loadBy(returnFormat="self") {
	savecontent variable="sql" { writeOutput("select 1"); }
	if (arguments.returnFormat == "query") { return new QueryResult(); }
	else if (arguments.returnFormat == "iterator") { return new Iterator(); }
	else { return this; }
}
}`,
		"RazunaSettings.cfc": `component extends="Base" { function getAPIKey(){} }`,
		"QueryResult.cfc":    `component {}`,
		"Iterator.cfc":       `component {}`,
		"Settings.cfc": `component {
function getRazunaSettings() {
	if (!structKeyExists(variables, "razunaSettings")) {
		variables.razunaSettings = getBean("razunaSettings").loadBy(siteid=getValue("siteid"));
	}
	return variables.razunaSettings;
}
}`,
		"Page.cfc": `component {function run(){
new Settings().getRazunaSettings().getAPIKey();
new RazunaSettings().loadBy(returnFormat="self").getAPIKey();
new RazunaSettings().loadBy(returnFormat="query").getAPIKey();
}}`,
	})

	r := &Resolver{Resolvers: []parser.Resolver{{Match: `(?i)getBean\(\s*["']razunaSettings["']\s*\)`, Resolve: "RazunaSettings", Prefix: "getBean"}}}
	got := reasonsWith(t, r, dir, "Page.cfc")
	expectReasons(t, got, map[string]string{
		"getRazunaSettings.getAPIKey": "",
	})

	if got["loadBy.getAPIKey"] == "" {
		t.Fatal("an explicit non-self mode inherited the receiver type")
	}
}

func TestDefaultSelfReturnParameterRequiresALiteralFinalDispatch(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"supported", `function f(mode="self"){work();if(arguments.mode eq "query"){return q;}else{return this;}}`, "mode"},
		{"different default", `function f(mode="query"){if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"self has another return", `function f(mode="self"){if(arguments.mode eq "self"){return q;}else{return this;}}`, ""},
		{"interpolated mode", `function f(mode="self"){if(arguments.mode eq "#runtime#"){return q;}else{return this;}}`, ""},
		{"earlier return", `function f(mode="self"){return new Other();if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"conditional earlier return", `function f(mode="self"){if(runtime()){return new Other();}if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"parameter mutation", `function f(mode="self"){arguments.mode="query";if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"unscoped parameter mutation", `function f(mode="self"){mode="query";if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"arguments escape", `function f(mode="self"){mutate(arguments);if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"unbraced enclosing condition", `function f(mode="self"){if(runtime()) if(arguments.mode eq "query"){return q;}else{return this;}}`, ""},
		{"trailing work", `function f(mode="self"){if(arguments.mode eq "query"){return q;}else{return this;}work();}`, ""},
		{"different fallback", `function f(mode="self"){if(arguments.mode eq "query"){return q;}else{return other;}}`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wheelsMethods("component{" + tc.source + `}`)["f"].selfDefaultParam
			if got != tc.want {
				t.Fatalf("self-default parameter = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGuardedLazyFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name, extra, initial string
	}{
		{"conflicting component", `function replace(){variables.service=new Other();}`, ``},
		{"switch writer", `function replace(flag){switch(flag){case 1:variables.service=new Other();break;}}`, ``},
		{"oversized writer", `function replace(){` + strings.Repeat(`work();`, 1100) + `variables.service=new Other();}`, ``},
		{"primitive replacement", `function replace(){variables.service="bad";}`, ``},
		{"conflicting startup component", ``, `variables.service=new Other();`},
		{"primitive startup for null guard", ``, `variables.service="bad";`},
		{"whole scope replacement", `function replace(){variables={};}`, ``},
		{"overridden guard", `function isNull(value){return false;}`, ``},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Service.cfc": `component { function ready(){} }`,
				"Other.cfc":   `component { function ready(){} }`,
				"Owner.cfc":   `component {` + tc.initial + ` function getService(){if(isNull(variables.service)){variables.service=new Service();}return variables.service;}` + tc.extra + ` }`,
				"Page.cfc":    `component {function run(){new Owner().getService().ready();}}`,
			})

			got := reasonsWith(t, &Resolver{}, dir, "Page.cfc")
			if reason, exists := got["getService.ready"]; !exists || reason == "" {
				t.Fatalf("guarded field resolved without an all-writes-agree contract: %v", got)
			}
		})
	}
}

func TestGuardedLazyTagFieldsRejectUnsupportedWriters(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Service.cfc": `component { function ready(){} }`,
		"Other.cfc":   `component {}`,
		"Owner.cfc": `<cfcomponent>
<cffunction name="getService">
<cfif isNull(this.service)><cfset this.service=new Service()></cfif>
<cfreturn this.service>
</cffunction>
<cffunction name="replace">
<cfswitch expression="#runtime()#">
<cfcase value="1"><cfset this.service=new Other()></cfcase>
</cfswitch>
</cffunction>
</cfcomponent>`,
		"Page.cfc": `component {function run(){new Owner().getService().ready();}}`,
	})

	got := reasonsWith(t, &Resolver{}, dir, "Page.cfc")
	if reason, exists := got["getService.ready"]; !exists || reason == "" {
		t.Fatalf("unsupported tag writer did not invalidate the guarded field: %v", got)
	}
}

func TestGuardedThisFieldsRejectUnsupportedWriters(t *testing.T) {
	for _, writer := range []string{
		`function replace(flag){switch(flag){case 1:this.service=new Other();break;}}`,
		`function replace(){` + strings.Repeat(`work();`, 1100) + `this.service=new Other();}`,
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"Service.cfc": `component { function ready(){} }`,
			"Other.cfc":   `component {}`,
			"Owner.cfc":   `component {function getService(){if(isNull(this.service)){this.service=new Service();}return this.service;}` + writer + `}`,
			"Page.cfc":    `component {function run(){new Owner().getService().ready();}}`,
		})

		got := reasonsWith(t, &Resolver{}, dir, "Page.cfc")
		if reason, exists := got["getService.ready"]; !exists || reason == "" {
			t.Fatalf("unsupported writer did not invalidate the guarded this field: %v", got)
		}
	}
}

func TestDefaultSelfDispatchDoesNotHideAnotherReturn(t *testing.T) {
	for _, tc := range []struct{ name, prefix string }{
		{"earlier return", `return new Other();`},
		{"changed mode", `arguments.mode="query";`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Other.cfc": `component {}`,
				"Owner.cfc": `component {
function ready(){}
function loadBy(mode="self"){
` + tc.prefix + `
if(arguments.mode eq "query"){return new Other();}else{return this;}
}
}`,
				"Page.cfc": `component {function run(){new Owner().loadBy().ready();}}`,
			})

			got := reasonsWith(t, &Resolver{}, dir, "Page.cfc")
			if reason, exists := got["loadBy.ready"]; !exists || reason == "" {
				t.Fatalf("self shortcut hid another return: %v", got)
			}
		})
	}
}

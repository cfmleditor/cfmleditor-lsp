package resolve

import (
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

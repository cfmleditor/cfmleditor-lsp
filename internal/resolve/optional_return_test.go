package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
)

func TestAbsentOptionalArgumentReturn(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc": `component {
 application.configBean=new Config();
}`,
		"Config.cfc": `component {function work(){}}`,
		"Scope.cfc": `component {
function config(property,propertyValue){var value="";if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}}
}`,
		"Bare.cfc":      `component extends="Scope" {function run(){config().work();}}`,
		"Ambiguous.cfc": `component {function run(){var scope=new Scope();scope.config().work();scope.config('x').other();}}`,
		"Page.cfc": `component {function run(){var scope=new Scope();
scope.config().work();
scope.config('x').other();
scope.config().missing();
var settings=scope.config();
settings.work();}}`,
	})

	r := &Resolver{}
	got := reasonsWith(t, r, dir, "Page.cfc")
	expectReasons(t, got, map[string]string{"scope.config.work": "", "scope.config.other": "method 'config' in Scope has no component return type (chain to 'other')", "scope.config.missing": "method 'missing' not found in Config", "settings.work": ""})
	expectReasons(t, reasonsWith(t, r, dir, "Bare.cfc"), map[string]string{"config.work": ""})
	// Two calls to config on one line: each hop carries its own arguments,
	// so neither borrows the other's.
	expectReasons(t, reasonsWith(t, r, dir, "Ambiguous.cfc"), map[string]string{"scope.config.work": "", "scope.config.other": "method 'config' in Scope has no component return type (chain to 'other')"})

	lookup := r.FuncLookup(dir)
	if got := lookup("Scope", parser.CallHop("config()")); r.ComponentPath(got, dir) != filepath.Join(dir, "Config.cfc") {
		t.Fatalf("no-argument return %q", got)
	}

	if got := lookup("Scope", parser.CallHop("config('x')")); got != "" {
		t.Fatalf("property getter typed %q", got)
	}

	if got := lookup("Scope", "config"); got != "" {
		t.Fatalf("generic return typed %q", got)
	}
}

func TestAbsentOptionalArgumentBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, params, body, returnType string }{
		{"default", `property="x"`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"required", `required string property`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"changed guard", `property`, `if(structKeyExists(arguments,"different")){return incoming;}else{return application.configBean;}`, ""},
		{"argument mutation", `property`, `arguments.property="x";if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"scope mutation", `property`, `application.configBean=incoming;if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"computed local", `property`, `var value=mutate();if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"interpolated local", `property`, `var value="#mutate()#";if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"shadow arguments", `property`, `var arguments="";if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, ""},
		{"selected effects", `property`, `if(structKeyExists(arguments,"property")){return incoming;}else{mutate();return application.configBean;}`, ""},
		{"trailing work", `property`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}return incoming;`, ""},
		{"unknown field", `property`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.missing;}`, ""},
		{"nested field", `property`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean.nested;}`, ""},
		{"explicit primitive", `property`, `if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}`, "string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Application.cfc": "component {\napplication.configBean=new Config();\n}",
				"Config.cfc":      `component {function work(){}}`,
				"Scope.cfc":       "component {" + tc.returnType + " function config(" + tc.params + "){" + tc.body + "}}",
				"Page.cfc":        `component {function run(){var scope=new Scope();scope.config().work();}}`,
			})

			r := &Resolver{}
			reasonsWith(t, r, dir, "Page.cfc")

			if got := r.FuncLookup(dir)("Scope", parser.CallHop("config()")); got != "" {
				t.Fatalf("unsafe return %q", got)
			}
		})
	}
}

func TestAbsentOptionalArgumentSourceRefresh(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc": "component {\napplication.configBean=new Config();\n}",
		"Config.cfc":      `component {function work(){}}`,
		"Scope.cfc":       `component {function config(property){if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}}}`,
		"Page.cfc":        `component {function run(){var scope=new Scope();scope.config().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	lookup := r.FuncLookup(dir)
	if got := lookup("Scope", parser.CallHop("config()")); got == "" {
		t.Fatal("initial contract missing")
	}

	for _, expression := range []string{"config(argumentCollection=arguments)", "config('x')", "config()suffix", "config(,"} {
		if got := lookup("Scope", parser.CallHop(expression)); got != "" {
			t.Fatalf("%s typed %q", expression, got)
		}
	}

	writeFiles(t, dir, map[string]string{"Scope.cfc": `component {function config(property){if(structKeyExists(arguments,"property")){return incoming;}else{mutate();return application.configBean;}}}`})

	if got := lookup("Scope", parser.CallHop("config()")); got != "" {
		t.Fatalf("stale contract %q", got)
	}
}

func TestAbsentOptionalArgumentCallerApplication(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Scope.cfc":           `component {function config(property){if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}}}`,
		"one/Application.cfc": "component {\napplication.configBean=new One();\n}",
		"one/One.cfc":         `component {function first(){}}`,
		"one/Page.cfc":        `component {function run(){var scope=new Scope();scope.config().first();}}`,
		"two/Application.cfc": "component {\napplication.configBean=new Two();\n}",
		"two/Two.cfc":         `component {function second(){}}`,
		"two/Page.cfc":        `component {function run(){var scope=new Scope();scope.config().second();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "one/Page.cfc")
	reasonsWith(t, r, dir, "two/Page.cfc")

	for _, name := range []string{"one", "two", "one"} {
		caller := filepath.Join(dir, name)
		got := r.FuncLookup(caller)(filepath.Join(dir, "Scope.cfc"), parser.CallHop("config()"))

		want := filepath.Join(caller, strings.ToUpper(name[:1])+name[1:]+".cfc")
		if r.ComponentPath(got, caller) != want {
			t.Fatalf("%s return %q, want %s", name, got, want)
		}
	}
}

func TestAbsentOptionalArgumentGuardOverride(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc": "component {\napplication.configBean=new Config();\n}",
		"Config.cfc":      `component {function work(){}}`,
		"Scope.cfc":       `component {function config(property){if(structKeyExists(arguments,"property")){return incoming;}else{return application.configBean;}}}`,
		"Custom.cfc":      `component extends="Scope" {function structKeyExists(a,b){return true;}}`,
		"Page.cfc":        `component {function run(){var scope=new Custom();scope.config().work();}}`,
	})

	r := &Resolver{}
	got := reasonsWith(t, r, dir, "Page.cfc")
	expectReasons(t, got, map[string]string{"scope.config.work": "method 'config' in Custom has no component return type (chain to 'work')"})

	if got := r.FuncLookup(dir)("Custom", parser.CallHop("config()")); got != "" {
		t.Fatalf("overridden guard return %q", got)
	}
}

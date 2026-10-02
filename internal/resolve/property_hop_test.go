package resolve

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func TestPropertyReadText(t *testing.T) {
	for _, c := range []struct{ line, property, want string }{
		{`<cfset result = VARIABLES._parent.getSubservices().lookuptables.selectAll(cmpy_code="#a#") />`, "lookuptables", "VARIABLES._parent.getSubservices().lookuptables"},
		{`x = a.getB( f(1), "x" ).prop.c();`, "prop", `a.getB( f(1), "x" ).prop`},
		{`x = a.getB( "(" ).prop.c();`, "prop", ""},
		{`x = ( a ).prop.c();`, "prop", ""},
		{`x = svc.LookUpTables.go()`, "lookuptables", "svc.LookUpTables"},
		{`x = a.propx.go()`, "prop", ""},
		{`x = a.prop.go() & b.prop.go()`, "prop", ""},
		{`no read here`, "prop", ""},
	} {
		if got := propertyReadText(c.line, c.property); got != c.want {
			t.Errorf("propertyReadText(%q, %q) = %q, want %q", c.line, c.property, got, c.want)
		}
	}
}

func TestLineOfContent(t *testing.T) {
	for n, want := range []string{"a", "", "c", ""} {
		if got := lineOfContent("a\n\nc", n); got != want {
			t.Errorf("line %d = %q, want %q", n, got, want)
		}
	}
}

// A struct a factory fills at run time is typed by a resolver matched against
// the property read: tassweb's getSubServices() holds one component per
// subservice, created from a list, so nothing in the source types
// getSubServices().lookuptables. The call is then checked, not waved through.
func TestAPropertyHopIsTypedByAResolver(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"core/Kernel.cfc":                 `component { function getSubServices() { return createObject("component", "core.subservices"); } }`,
		"subservices/lookups/service.cfc": `component { function selectAll() {} }`,
		"pages/Page.cfc": `component {
	variables.kernel = new core.Kernel();
	function a() { return variables.kernel.getSubServices().lookups.selectAll(); }
	function b() { return variables.kernel.getSubServices().lookups.nope(); }
}`,
	})

	without := reasonsWith(t, &Resolver{}, dir, "pages/Page.cfc")
	if !strings.Contains(without["variables.kernel.getSubServices.$property:lookups.selectAll"], "has no component type") {
		t.Fatalf("without the resolver the property is untyped, got %q", without["variables.kernel.getSubServices.$property:lookups.selectAll"])
	}

	got := reasonsWith(t, &Resolver{Resolvers: []parser.Resolver{{Match: "getSubServices().$1", Resolve: "subservices.${1:lower}.service", Prefix: "getSubServices"}}}, dir, "pages/Page.cfc")
	if got["variables.kernel.getSubServices.$property:lookups.selectAll"] != "" {
		t.Errorf("selectAll on the resolved subservice did not resolve: %s", got["variables.kernel.getSubServices.$property:lookups.selectAll"])
	}

	if !strings.Contains(got["variables.kernel.getSubServices.$property:lookups.nope"], "not found in subservices.lookups.service") {
		t.Errorf("nope should be checked against the subservice, got %q", got["variables.kernel.getSubServices.$property:lookups.nope"])
	}
}

// A resolver that matches the call before the property, not the property,
// says nothing about the property: the catch-all `get$1()` on
// `kernel.getBar().baz` names getBar's component, which baz is not.
func TestAPropertyHopIgnoresAResolverForTheCallBeforeIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"core/Kernel.cfc": `component { function getBar() { return createObject("component", "core.missing"); } }`,
		"app/bar.cfc":     `component { function qux() {} }`,
		"pages/Page.cfc": `component {
	variables.kernel = new core.Kernel();
	function a() { return variables.kernel.getBar().baz.qux(); }
}`,
	})

	got := reasonsWith(t, &Resolver{Resolvers: []parser.Resolver{{Match: `get([A-Za-z]+)\(\)`, Resolve: "app.${1:lower}", Prefix: "get"}}}, dir, "pages/Page.cfc")
	if reason := got["variables.kernel.getBar.$property:baz.qux"]; !strings.Contains(reason, "has no component type") {
		t.Errorf("baz was typed by the resolver for getBar(): %q", reason)
	}
}

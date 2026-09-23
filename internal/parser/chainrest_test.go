package parser

import (
	"strings"
	"testing"
)

// chainLookup models a builder whose width/height return the builder and whose
// build returns the chart, and a factory whose builder() returns a builder.
func chainLookup(component, funcName string) string {
	switch strings.ToLower(component + "." + funcName) {
	case "stubs.builder.width", "stubs.builder.height", "stubs.builder.init":
		return "stubs.Builder"
	case "stubs.builder.build":
		return "stubs.Chart"
	case "stubs.options.builder":
		return "stubs.Options$Builder"
	case "stubs.options$builder.setcredentials":
		return "stubs.Options$Builder"
	case "stubs.options$builder.build":
		return "stubs.Options"
	}

	return ""
}

var chainResolvers = []Resolver{{
	Match:   `createObject\s*\(\s*['"]java['"]\s*,\s*['"](.+?)['"]\s*\)`,
	Resolve: "stubs.$1",
	Prefix:  "createObject",
}}

func refsByVar(pr *ParseResult) map[string]string {
	out := make(map[string]string)

	for _, r := range pr.ComponentRefs {
		out[strings.ToLower(r.Variable)] = r.Component
	}

	for _, sc := range pr.Scopes {
		for _, r := range pr.FuncComponentRefs(sc.Start, sc.End) {
			out[strings.ToLower(r.Variable)] = r.Component
		}
	}

	return out
}

// TestAssignmentIsTypedByTheLastCallOfAChain covers
// chart = b.width(1).height(2).build(), which was typed by width — the
// builder — so every method of the chart it really holds was reported
// missing; and x = createObject(...).builder().build(), typed as the
// created class. Script and tag forms, since each has its own path.
func TestAssignmentIsTypedByTheLastCallOfAChain(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"script", `component {
	function work() {
		var b = createObject("java", "Builder");
		chart = b.width(1).height(2).build();
		opts = createObject("java", "Options").builder().setCredentials(c).build();
		built = createObject("java", "Builder").init();
		lost = b.width(1).unknown().build();
	}
}`},
		{"tag", `<cfcomponent>
	<cffunction name="work">
		<cfset var b = createObject("java", "Builder") />
		<cfset chart = b.width(1).height(2).build() />
		<cfset opts = CreateObject("java","Options").builder()
			.setCredentials(c)
			.build() />
		<cfset built = createObject("java", "Builder").init() />
		<cfset lost = b.width(1).unknown().build() />
	</cffunction>
</cfcomponent>`},
	} {
		pr := ParseWithOptions(testURI, tc.content, ParseOptions{
			Resolvers:  chainResolvers,
			FuncLookup: chainLookup,
		})
		got := refsByVar(pr)

		for v, want := range map[string]string{
			"chart": "stubs.Chart",
			"opts":  "stubs.Options",
			"built": "stubs.Builder", // init() returns what it is called on
			"lost":  "$any",          // an untyped hop makes the rest unknown
		} {
			if got[v] != want {
				t.Errorf("%s: %s -> %q, want %q (refs %v)", tc.name, v, got[v], want, got)
			}
		}
	}
}

// TestChainWithoutFuncLookupIsDynamic: with nothing to type the later calls
// by, the first call's type is still the wrong answer.
func TestChainWithoutFuncLookupIsDynamic(t *testing.T) {
	content := `component {
	function work() {
		opts = createObject("java", "Options").builder().build();
		plain = createObject("java", "Options").init();
	}
}`

	got := refsByVar(ParseWithOptions(testURI, content, ParseOptions{Resolvers: chainResolvers}))

	if got["opts"] != "$any" || got["plain"] != "stubs.Options" {
		t.Errorf("opts -> %q (want $any), plain -> %q (want stubs.Options)", got["opts"], got["plain"])
	}
}

// TestCreateObjectChainHopsCarryTheirChain covers the call side of
// createObject("java", "Options").builder().setCredentials(c): every hop was
// recorded against the created class with no Chain, so setCredentials was
// checked on Options instead of on what builder() returns.
func TestCreateObjectChainHopsCarryTheirChain(t *testing.T) {
	content := `component {
	function work() {
		opts = createObject("java", "Options").builder().setCredentials(c).build();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: chainResolvers, ExtractCalls: true, ScanAllScopes: true})

	want := map[string]string{"builder": "", "setCredentials": "builder", "build": "builder,setCredentials"}

	for _, c := range pr.AllCalls() {
		w, ok := want[c.FuncName]
		if !ok {
			continue
		}

		delete(want, c.FuncName)

		if c.Component != "stubs.Options" || strings.Join(c.Chain, ",") != w {
			t.Errorf("%s: Component %q Chain %v, want stubs.Options and [%s]", c.FuncName, c.Component, c.Chain, w)
		}
	}

	if len(want) > 0 {
		t.Errorf("calls not recorded: %v", want)
	}
}

// TestTagCallDoesNotReadAnUnwalkedChainRef covers
// <cfset chart = CreateObject(...).init().width(1).build()> followed by
// <cfset chart.addSeries(...)>. The tag parser fills a call's Component from
// the nearest ref while it parses, before the chain is walked, so the call
// was checked against the builder; it now leaves Component for the resolve
// step, which reads the finished ref.
func TestTagCallDoesNotReadAnUnwalkedChainRef(t *testing.T) {
	content := `<cfcomponent>
	<cffunction name="work">
		<cfset var chart = "" />
		<cfset chart = CreateObject("java","Builder").init().width(1).build() />
		<cfset chart.addSeries("a", x, y) />
		<cfset var plain = CreateObject("java","Builder").init() />
		<cfset plain.width(1) />
	</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(testURI, content, ParseOptions{
		Resolvers: chainResolvers, FuncLookup: chainLookup, ExtractCalls: true, ScanAllScopes: true,
	})

	for _, c := range pr.AllCalls() {
		switch c.FuncName {
		case "addSeries":
			if c.Component == "stubs.Builder" {
				t.Errorf("addSeries checked against the builder: %+v", c)
			}
		case "width":
			if c.Variable == "plain" && c.Component != "stubs.Builder" {
				t.Errorf("plain.width lost its component: %+v", c)
			}
		}
	}

	if got := refsByVar(pr)["chart"]; got != "stubs.Chart" {
		t.Errorf("chart -> %q, want stubs.Chart", got)
	}
}

// TestAssignedChainKeepsItsReceiverAndHops covers two losses in an assigned
// chain. The hops were continued from the last name before the first call —
// "kernel" in REQUEST.kernel.a().b(), "c" in a.b.c.m().n() — rather than the
// receiver; and when tryExtendChain matched a resolver on `first(...).ext()`,
// ext was never recorded and the hop after it came back as a bare call.
func TestAssignedChainKeepsItsReceiverAndHops(t *testing.T) {
	resolvers := []Resolver{{Match: `get([A-Za-z]+)\(\)`, Resolve: "packages.tass.${1:lower}", Prefix: "get"}}
	content := `component {
	function work() {
		VARIABLES.x = REQUEST.kernel.getSandBox("f").getEntityObj().getSelected(a, b);
		y = a.b.c.m().n();
	}
}`

	pr := ParseWithOptions(testURI, content, ParseOptions{Resolvers: resolvers, ExtractCalls: true, ScanAllScopes: true})

	want := map[string]struct {
		recv  string
		chain string
	}{
		"getSandBox":   {"REQUEST.kernel", ""},
		"getEntityObj": {"REQUEST.kernel", "getSandBox"},
		"getSelected":  {"REQUEST.kernel", "getSandBox,getEntityObj"},
		"m":            {"a.b.c", ""},
		"n":            {"a.b.c", "m"},
	}

	for _, c := range pr.AllCalls() {
		w, ok := want[c.FuncName]
		if !ok {
			continue
		}

		delete(want, c.FuncName)

		if c.Variable != w.recv || strings.Join(c.Chain, ",") != w.chain {
			t.Errorf("%s: Variable %q Chain %v, want %q [%s]", c.FuncName, c.Variable, c.Chain, w.recv, w.chain)
		}
	}

	if len(want) > 0 {
		t.Errorf("calls not recorded: %v", want)
	}
}

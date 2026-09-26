package parser

import (
	"slices"
	"testing"

	"go.lsp.dev/uri"
)

func callNames(pr *ParseResult) []string {
	out := []string{}
	cs := pr.AllCalls()

	for i := range cs {
		c := &cs[i]

		out = append(out, c.FuncName)
	}

	slices.Sort(out)

	return out
}

func gatedCalls(src string) []string {
	return callNames(ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true}))
}

// TestTextIsScannedOnlyWhereColdFusionEvaluatesIt pins the rules in
// outputContext, one case per probe template run against Adobe ColdFusion
// 2025. A case the server evaluated wants its call; one it printed literally
// wants none.
func TestTextIsScannedOnlyWhereColdFusionEvaluatesIt(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		// Evaluated.
		{"cfoutput text", `<cfoutput>#a()#</cfoutput>`, []string{"a"}},
		{"html attribute in cfoutput", `<cfoutput><div title="#a()#"></div></cfoutput>`, []string{"a"}},
		{"script in cfoutput", "<cfoutput><script>var x = \"#a()#\";<cfif b()></cfif></script></cfoutput>", []string{"a", "b"}},
		{"cf tag attribute", `<cfparam name="x" default="#a()#">`, []string{"a"}},
		{"cf_ custom tag attribute", `<cf_echo v="#a()#">`, []string{"a"}},
		{"cfimport prefix tag attribute", `<cfimport taglib="tags" prefix="t"><t:echo v="#a()#">`, []string{"a"}},
		{"cfmodule attribute", `<cfmodule template="echo.cfm" v="#a()#">`, []string{"a"}},
		{"cfquery body", `<cfquery name="q" dbtype="query">SELECT x FROM y WHERE z = '#a()#'</cfquery>`, []string{"a"}},
		{"cfmail body", `<cfmail to="x" from="y" subject="s">#a()#</cfmail>`, []string{"a"}},
		{"cffunction output=true", `<cffunction name="f" output="true">#a()#</cffunction>`, []string{"a"}},
		{"cffunction output=yes", `<cffunction name="f" output="yes">#a()#</cffunction>`, []string{"a"}},
		{"cfoutput in a function with output=false", `<cffunction name="f" output="false"><cfoutput>#a()#</cfoutput></cffunction>`, []string{"a"}},
		{"cfoutput across a cfscript block", "<cfoutput><cfscript>x = 1;</cfscript>#a()#</cfoutput>", []string{"a"}},
		{"escaped hashes in cfoutput", `<cfoutput><a href="##" onclick="doIt()">#a()#</a></cfoutput>`, []string{"a"}},
		{"unclosed cfoutput fails open", `<cfoutput>#a()#`, []string{"a"}},

		// Printed literally.
		{"plain text", "#a()#\n<cfset x = 1>", nil},
		{"html attribute", `<div title="#a()#"></div><cfset x = 1>`, nil},
		{"script holding a cf tag, outside cfoutput", "<script>var x = \"#a()#\";<cfif b()></cfif></script>", []string{"b"}},
		{"undeclared prefix is an HTML tag", `<t:echo v="#a()#"><cfset x = 1>`, nil},
		{"custom tag body", `<cf_echo>#a()#</cf_echo>`, nil},
		{"cfsavecontent body", `<cfsavecontent variable="s">#a()#</cfsavecontent>`, nil},
		{"cfxml body", `<cfxml variable="x"><a>#a()#</a></cfxml>`, nil},
		{"cffunction output unset", `<cffunction name="f">#a()#</cffunction>`, nil},
		{"cffunction output=false", `<cffunction name="f" output="false">#a()#</cffunction>`, nil},
		{"after cfoutput closes", `<cfoutput>#a()#</cfoutput>#b()#`, []string{"a"}},

		// The shapes the report was full of: two stray hashes pairing across
		// markup and swallowing the JavaScript between them.
		{"href and a colour", `<a href="#" onclick="doIt()">x</a><span style="color:#fff">y</span><cfset x = 1>`, nil},
		{"jquery selectors", "<script>$('#a').each(function(){ run(); });<cfif true></cfif>$('#b').x();</script>", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := gatedCalls(c.src)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}

			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// TestInterpolateAllTextRestoresTheOldReading is the switch: off, every pair of
// hashes in text is scanned again, stray ones included.
func TestInterpolateAllTextRestoresTheOldReading(t *testing.T) {
	src := `<a href="#" onclick="doIt()">x</a><span style="color:#fff">y</span><cfset x = 1>`

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true, InterpolateAllText: true})
	if got := callNames(pr); !slices.Equal(got, []string{"doIt"}) {
		t.Errorf("ungated: got %v want [doIt]", got)
	}
}

// TestTagFreeTemplateIsMarkup covers a .cfm with no CF tags, which ColdFusion
// runs as literal text. It was read as CFScript, so "Rich Text (with Images)"
// was a call to Text and a JavaScript function was a CFML declaration.
func TestTagFreeTemplateIsMarkup(t *testing.T) {
	cfm := uri.URI("file:///app/template.cfm")
	html := "<div class=\"x\">\n  <label>Rich Text (with Images)</label>\n  <button onclick=\"save()\">Save</button>\n</div>\n"

	pr := ParseWithOptions(cfm, html, &ParseOptions{ExtractCalls: true})
	if got := callNames(pr); len(got) != 0 {
		t.Errorf("markup template: calls %v, want none", got)
	}

	if len(pr.Regions) == 0 || pr.Regions[0].Kind == RegionScript {
		t.Errorf("markup template classified as script: %+v", pr.Regions)
	}

	if got := callNames(ParseWithOptions(cfm, html, &ParseOptions{ExtractCalls: true, InterpolateAllText: true})); len(got) == 0 {
		t.Error("with the switch off the template should read as script again")
	}

	// A script fragment with no markup in it is still script.
	if got := callNames(ParseWithOptions(cfm, "x = foo();\nif (a<b) { bar(); }\n", &ParseOptions{ExtractCalls: true})); !slices.Equal(got, []string{"bar", "foo"}) {
		t.Errorf("script fragment: got %v want [bar foo]", got)
	}

	// A .cfc is never a template.
	cfc := uri.URI("file:///app/Thing.cfc")
	if got := callNames(ParseWithOptions(cfc, "component { function f() { return g(); } }\n</div>", &ParseOptions{ExtractCalls: true})); !slices.Contains(got, "g") {
		t.Errorf(".cfc: got %v, want g", got)
	}
}

// TestScriptBlockInterpolationFollowsTheOutputContext pins what a <script>
// block with no CF tag gets: it is a skip region, kept from the CFScript
// scanner, and its #...# spans are still read, but only where ColdFusion
// evaluates them.
func TestScriptBlockInterpolationFollowsTheOutputContext(t *testing.T) {
	src := "<cfoutput>\n<script>\nvar x = \"#inOutput()#\";\n</script>\n</cfoutput>\n" +
		"<cffunction name=\"f\" output=\"true\">\n<script>var y = \"#inOutputFunction()#\";</script>\n</cffunction>\n" +
		"<script>var z = \"#notEvaluated()#\";</script>\n"

	if got := gatedCalls(src); !slices.Equal(got, []string{"inOutput", "inOutputFunction"}) {
		t.Errorf("got %v want [inOutput inOutputFunction]", got)
	}
}

// TestCallsAfterARegionSplitKeepTheirFunction covers the calls in and after a
// <script> block or a <cfscript> island inside a tag <cffunction>. The body is
// cut into regions there, a region's parser names a caller only from functions
// it parsed itself, and the <cffunction> was in the first region: every call
// from the split on came back with no caller.
func TestCallsAfterARegionSplitKeepTheirFunction(t *testing.T) {
	src := "<cfcomponent>\n<cffunction name=\"g\" output=\"true\">\n<cfset a = before()>\n<script>\nvar x = \"#inScript()#\";\n</script>\n" +
		"<cfset b = afterScript()>\n<cfscript>\nisland();\n</cfscript>\n<cfset c = afterIsland()>\n</cffunction>\n<cfset d = outside()>\n</cfcomponent>"

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	want := map[string]string{
		"before": "g", "inScript": "g", "afterScript": "g", "island": "g", "afterIsland": "g", "outside": "",
	}

	for _, c := range pr.AllCalls() {
		w, ok := want[c.FuncName]
		if !ok {
			continue
		}

		delete(want, c.FuncName)

		if c.Caller != w {
			t.Errorf("%s: caller %q, want %q", c.FuncName, c.Caller, w)
		}
	}

	if len(want) > 0 {
		t.Errorf("calls not recorded: %v", want)
	}
}

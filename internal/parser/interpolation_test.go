package parser

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

func interpCalls(t *testing.T, src string) []string {
	t.Helper()

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	var got []string

	for _, c := range pr.AllCalls() {
		v := c.Variable
		if v == "" && c.Component != "" {
			v = "[" + c.Component + "]"
		}

		if v == "" {
			v = "?"
		}

		got = append(got, v+"."+c.FuncName)
	}

	slices.Sort(got)

	return got
}

// The scanner takes a quoted string as one token, so a call inside a #...#
// span was invisible — and interpolation is how a computed value reaches a
// string in CFML, not an edge case.
func TestCallsInsideScriptInterpolationAreFound(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`writeOutput("x #svc.getName()# y");`, []string{"?.writeOutput", "svc.getName"}},
		{`var s = "#svc.getName()#";`, []string{"svc.getName"}},
		{`var s = "#a.b()##c.d()#";`, []string{"a.b", "c.d"}},
		{`log.info("user #dao.load(id).name# ok");`, []string{"dao.load", "log.info"}},
		{`var s = {k: "#svc.v()#"};`, []string{"svc.v"}},
		{`var s = "#svc.getName()#" & other.x();`, []string{"other.x", "svc.getName"}},

		// `##` is an escaped hash and opens nothing.
		{`var s = "a ## b";`, nil},
		{`var s = "##";`, nil},

		// Controls: nothing to find, and nothing invented.
		{`var s = "no hashes";`, nil},
		{`var s = "#unclosed";`, nil},
		{`var s = "#justAName#";`, nil},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			got := interpCalls(t, "component {\n function go() {\n  "+c.body+"\n }\n}\n")
			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// The tag parser matches tags and never tokenises text, so a <cfoutput> body
// and every interpolated attribute value were invisible too. Both go through
// the same scriptParser a <cfscript> body already does.
func TestCallsInsideTagInterpolationAreFound(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{`<cfoutput>#svc.getName()#</cfoutput>`, []string{"svc.getName"}},
		{`<cfoutput>#svc.a()# and #svc.b()#</cfoutput>`, []string{"svc.a", "svc.b"}},
		{`<cfset x = "#svc.getName()#">`, []string{"svc.getName"}},
		{`<cfloop array="#svc.list()#" index="i"></cfloop>`, []string{"svc.list"}},
		{`<cfquery name="q">a=<cfqueryparam value="#svc.id()#"></cfquery>`, []string{"svc.id"}},

		// A handled tag is stepped over whole and a declined one becomes text
		// before the next tag; both routes have to reach the scan.
		{`<cfmodule template="x" attr="#svc.m()#">`, []string{"svc.m"}},

		// Controls: hashes in prose, CSS and fragments invent nothing.
		{`<div id="x">#1 seller, #2 runner</div>`, nil},
		{`<style>a { color: #fff; background: #000; }</style>`, nil},
		{`<a href="#top">x</a><a href="#bottom">y</a>`, nil},
	}

	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			if got := interpCalls(t, c.src); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// `<cfset x = #svc.y()#>` reaches both the cfset handler and the span scan.
// Recording it twice would double every edge the code map draws from it.
func TestInterpolationDoesNotDoubleRecord(t *testing.T) {
	for _, src := range []string{
		`<cfset x = #svc.y()#>`,
		`<cfset x = svc.y()>`,
	} {
		if got := interpCalls(t, src); !slices.Equal(got, []string{"svc.y"}) {
			t.Errorf("%s: got %v want [svc.y]", src, got)
		}
	}
}

// A call in a function's <cfoutput> belongs to that function, which is why the
// scan runs during the tag walk rather than after it.
func TestTagInterpolationIsFiledAgainstItsFunction(t *testing.T) {
	src := "<cfcomponent>\n<cffunction name=\"go\">\n<cfoutput>#svc.x()#</cfoutput>\n</cffunction>\n" +
		"<cffunction name=\"other\">\n<cfset y = 1>\n</cffunction>\n</cfcomponent>"

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	for _, sc := range pr.Scopes {
		n := len(pr.FuncCalls(sc.Start, sc.End))
		if (sc.Name == "go") != (n == 1) {
			t.Errorf("%s: got %d calls", sc.Name, n)
		}
	}
}

// A literal's member function is called on a value, not a variable. Walking
// past the receiver recorded a bare `ucase()`, which in a component that
// declares a function of that name is an edge to it that does not exist.
func TestMemberCallsOnLiteralsKeepAReceiver(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`return "abc".ucase();`, []string{"[$any].ucase"}},
		{`return [1,2].map(svc.fn()).toList();`, []string{"[$any].map", "[$any].toList", "svc.fn"}},

		// Controls: a bracket-indexed variable still poisons its receiver
		// rather than becoming a literal.
		{`arr[i].method();`, []string{"arr[].method"}},
		{`REQUEST['k'].method();`, []string{"REQUEST[].method"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			got := interpCalls(t, "component {\n function go() {\n  "+c.body+"\n }\n}\n")
			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// Six loops walk tokens and a literal reaches all of them, so they route
// strings and closing brackets through one place rather than each growing its
// own copy of the rule. A new loop that scans tokens itself is the regression
// this catches.
func TestEveryTokenLoopHandlesLiterals(t *testing.T) {
	src, err := os.ReadFile("script_parser.go")
	if err != nil {
		t.Fatal(err)
	}

	// Every arm that dispatches an identifier to the nested-call scan is a
	// token loop, and each must have a literal arm beside it.
	loops := regexp.MustCompile(`(?s)case TokIdent:\n\t+p\.scanNestedCall\((\w+)\)\n(.{0,160}?)\n\t+\}`)

	matches := loops.FindAllStringSubmatch(string(src), -1)
	if len(matches) < 4 {
		t.Fatalf("expected at least four nested-call dispatch sites, found %d — this test's anchor needs updating", len(matches))
	}

	for _, m := range matches {
		if !regexp.MustCompile(`case TokString`).MatchString(m[2]) {
			t.Errorf("a token loop dispatching %s to scanNestedCall has no TokString arm:\n%s", m[1], m[0])
		}
	}
}

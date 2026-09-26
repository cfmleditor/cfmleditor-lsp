package parser

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func interpCalls(t *testing.T, src string) []string {
	t.Helper()

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	var got []string

	cs := pr.AllCalls()

	for i := range cs {
		c := &cs[i]

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

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

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
	// The dispatch may read the token first, as skipTagAttrValue does, since
	// that loop peeks at its terminator rather than consuming it.
	loops := regexp.MustCompile(
		`(?s)case TokIdent:\n(?:\t+//[^\n]*\n)*(?:\t+\w+ := [^\n]+\n)?\t+p\.scanNestedCall\((\w+)\)\n(.{0,200}?)\n\t+\}`)

	matches := loops.FindAllStringSubmatch(string(src), -1)
	if len(matches) < 6 {
		t.Fatalf("expected at least six nested-call dispatch sites, found %d — this test's anchor needs updating", len(matches))
	}

	for _, m := range matches {
		if !regexp.MustCompile(`case TokString`).MatchString(m[2]) {
			t.Errorf("a token loop dispatching %s to scanNestedCall has no TokString arm:\n%s", m[1], m[0])
		}
	}
}

// A script-syntax CF tag's attribute value is an expression too. skipTagAttrValue
// consumed the identifier and handed only its *argument list* to the scan that
// finds calls, so `array=structKeyArray(rows)` recorded whatever was inside the
// parens and never structKeyArray itself.
func TestScriptTagAttributeValuesRecordTheirCalls(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`loop array=structKeyArray(rowData) item="local.col" { x(); }`,
			[]string{"?.structKeyArray", "?.x"}},
		{`http url="u" result=serializeJson(body.data) {}`, []string{"?.serializeJson"}},
		{`query name="q" datasource=getDatasource() { }`, []string{"?.getDatasource"}},
		{`lock name="l" timeout=calcTimeout(x) { }`, []string{"?.calcTimeout"}},

		// An interpolated value reaches the same arm through the string.
		{`query name="q" datasource="#svc.ds()#" { }`, []string{"svc.ds"}},

		// Controls: a plain value declares nothing and calls nothing, and the
		// body's own calls are unaffected.
		{`loop array=arr item=local.col { }`, nil},
		{`query name="q" datasource="ds" { writeOutput("x"); }`, []string{"?.writeOutput"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			src := "component {\n function go() {\n  " + c.body + "\n }\n}\n"

			if got := interpCalls(t, src); !slices.Equal(got, sorted(c.want)) {
				t.Errorf("got %v want %v", got, sorted(c.want))
			}

			// The attribute names must still not become variables.
			for _, v := range ParseVars(src) {
				t.Errorf("%s declared a variable %q", c.body, v.Name)
			}
		})
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)

	return out
}

// A `#` inside a string opens an expression, and a string may be opened inside
// *that* with the same quote character. Closing the outer string at the inner
// quote left `"#DayOfWeek("`, so the interpolation had no closing `#` and the
// call in it was invisible — 3,165 call sites over 461 files on the corpus,
// measured in PARSER-GAPS.md §3.3.
func TestStringsNestInsideInterpolation(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`assertEquals("7", "#DayOfWeek("{ts '2000-1-1'}")#");`,
			[]string{"?.assertEquals", "?.DayOfWeek"}},
		{`x = "#f("a")#";`, []string{"?.f"}},
		{`x = "#f("a")# and #g("b")#";`, []string{"?.f", "?.g"}},
		{`x = "#f('a')#";`, []string{"?.f"}},

		// Controls: an escaped hash opens nothing, a plain string is a plain
		// string, and a balanced interpolation still works.
		{`x = "a ## b";`, nil},
		{`x = "plain";`, nil},
		{`x = "#a#";`, nil},
		{`x = "#svc.get()#";`, []string{"svc.get"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			got := interpCalls(t, "component {\n function go() {\n  "+c.body+"\n }\n}\n")
			if !slices.Equal(got, sorted(c.want)) {
				t.Errorf("got %v want %v", got, sorted(c.want))
			}
		})
	}
}

// The rule is off unless the text is known to be CFScript, because text that is
// not reaches the scanner: parseFuncBody hands a tag function's raw body to the
// script parser. Applied to markup it is actively wrong — two `href="#…"`
// fragments on one line pair their hashes and swallow what is between them.
func TestMarkupHashesAreNotInterpolation(t *testing.T) {
	for _, src := range []string{
		`<a href="#top">x</a><a href="#bot">y</a>`,
		`<div id="x">#1 seller, #2 runner</div>`,
		`<style>a { color: #fff; background: #000; }</style>`,
	} {
		t.Run(src, func(t *testing.T) {
			if got := interpCalls(t, src); got != nil {
				t.Errorf("got %v, want no calls", got)
			}

			// The markup must also still tokenise into separate strings rather
			// than one that swallows the tags between them.
			sc := NewScanner(src)
			for {
				tok := sc.NextSkipComments()
				if tok.Kind == TokEOF {
					break
				}

				if tok.Kind == TokString && strings.Contains(tok.Value, "<") {
					t.Errorf("a string token swallowed markup: %q", tok.Value)
				}
			}
		})
	}
}

// The two variable scans must tokenise a file the same way, which is why
// globalScriptParser opts in too. Without it `"#f("x=1")#"` tokenises as
// `"#f("`, `x`, `=`, `1`, `")#"` — and a scan that records a bare `ident =` as
// a declaration takes `x` for a variables-scope variable, which is what
// completion offers and what the index stores.
func TestInterpolatedStringDoesNotDeclareAVariable(t *testing.T) {
	for _, src := range []string{
		"component {\n variables.a = \"#f(\"x=1\")#\";\n}\n",
		"component {\n variables.a = \"#buildLink(\"page=home\")#\";\n}\n",
	} {
		t.Run(src, func(t *testing.T) {
			pr := Parse(testURI, src)
			if got := pr.VariablesVars(); !slices.Equal(got, []string{"a"}) {
				t.Errorf("VariablesVars: got %v want [a]", got)
			}

			var names []string
			for _, v := range ParseVars(src) {
				names = append(names, v.Name)
			}

			if !slices.Equal(names, []string{"a"}) {
				t.Errorf("ParseVars: got %v want [a]", names)
			}
		})
	}
}

// The nesting is driven by the source and Go cannot recover from stack
// exhaustion, so the mutual recursion is capped — and past the cap the
// speculative scan is abandoned for the plain one rather than failing. That
// makes the cap observable: below it the whole nested string is one token,
// above it the token is the plain quote-to-quote one the scanner always
// produced.
//
// Asserting *that* is the point. A test that fed in a very deep string and
// asserted only that the scan returned passed with the cap removed as well —
// 2,000 levels of two small frames do not come close to Go's 1GB stack, so
// nothing short of a 64MB input would have exercised it.
func TestInterpolationNestingIsCapped(t *testing.T) {
	nested := func(n int) string {
		return `"` + strings.Repeat(`#f("`, n) + "a" + strings.Repeat(`")#`, n) + `"`
	}

	tokenFor := func(src string) string {
		sc := NewScanner(src)
		sc.interpStrings = true

		return sc.NextSkipComments().Value
	}

	// maxStringNesting counts both halves of the recursion, so each `#f("`
	// level costs two.
	deepest := maxStringNesting/2 - 1

	if src := nested(deepest); tokenFor(src) != src {
		t.Errorf("at depth %d the string should scan whole, got %q", deepest, tokenFor(src))
	}

	for _, n := range []int{deepest + 1, maxStringNesting, 2000} {
		if got := tokenFor(nested(n)); got != `"#f("` {
			t.Errorf("at depth %d: got %q, want the plain scan's %q", n, got, `"#f("`)
		}
	}
}

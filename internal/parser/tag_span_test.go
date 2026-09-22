package parser

import (
	"slices"
	"strings"
	"testing"
)

func tagFileCalls(t *testing.T, src string) []string {
	t.Helper()

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	out := make([]string, 0, 4)

	for _, c := range pr.AllCalls() {
		out = append(out, c.FuncName)
	}

	slices.Sort(out)

	return out
}

// Markup written inside a `#...#` span is an argument, not a tag. The walk
// found the next `<` without regard for the span, so
//
//	#ETH.author( content = "<strong>@name@</strong>" )#
//
// handed scanInterpolatedText a chunk with an unterminated `#` — losing that
// call — and left hashes that mis-paired with the next span, losing the call
// after it too. That was most of what remained of ContentBox's email templates.
func TestMarkupInsideAnInterpolatedSpanDoesNotSplitIt(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{
			"markup in a span's string argument",
			"<cfoutput>\n#ETH.author( c = \"<strong>x</strong>\" )#\n\n#ETH.divider()#\n</cfoutput>",
			[]string{"author", "divider"},
		},
		{
			"spanning lines, as the email templates write it",
			"<cfoutput>\n#ETH.author(\n  email = args.e,\n  content = \"<a href='u'>n</a>\"\n)#\n\n#ETH.heading( c = \"D\" )#\n</cfoutput>",
			[]string{"author", "heading"},
		},
		{
			"a plain span still works",
			"<cfoutput>\n#ETH.divider()#\n</cfoutput>",
			[]string{"divider"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := tagFileCalls(t, c.src); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// The rule loses tags rather than finding calls whenever it pairs hashes that
// were never a span. Each case here is reduced from a file the corpus reported
// as newly broken while that guard was missing, and each fails without it —
// the first fixtures written for these passed either way, because a guard that
// was still present rescued the shape.
func TestHashesThatAreNotASpanDoNotSwallowTags(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{
			// A first version advanced only past the *opening* hash of a span
			// it declined, so the closing hash became the next opening one and
			// every pairing after it was inverted. Two plain interpolations on
			// one line are enough to start the cascade. 141 files, 773 sites.
			"a declined span still pairs its hashes",
			"<cfoutput>\n" +
				"<li><b>#myKey#</b>: #prc.info[ myKey ]#</li>\n" +
				"<cfif structKeyExists( prc, \"img\" )>\n" +
				"#prc.img#\n" +
				"</cfif>\n" +
				"</cfoutput>",
			[]string{"structKeyExists"},
		},
		{
			// A CSS colour is a lone hash, and the span it would open runs to
			// whatever interpolation comes next — here across a `<cfif>` whose
			// condition holds a call. The quotes inside it do not balance,
			// which is what says it was never an expression. 7 files.
			"a CSS colour is not a span",
			"<div style=\"background-color: #f9f9f9; padding: 10px;\">\n" +
				"<cfoutput>\n" +
				"<cfif structKeyExists( request, 'failedAction' )>\n" +
				"<b>Action:</b> #replace( request.failedAction, \"<\", \"&lt;\", \"all\" )#<br/>\n" +
				"</cfif>\n" +
				"</cfoutput>\n</div>",
			[]string{"replace", "structKeyExists"},
		},
		{
			// A CSS id selector is the same shape with no quotes at all, so the
			// balance test cannot see it — the paren test is what does, since a
			// stylesheet rule holds none. 5 files.
			"a CSS id selector is not a span",
			"<style>\n" +
				"#sidebar ul.links li {\n  margin-bottom: 5px;\n}\n" +
				"</style>\n" +
				"<cfoutput>#view()#</cfoutput>",
			[]string{"view"},
		},
		{
			"prose hashes keep their markup",
			"<div>item #1 <b>x</b> #2</div>\n<cfif len( q )>\n</cfif>",
			[]string{"len"},
		},
		{
			"an escaped hash opens nothing",
			"<cfoutput>\n<a href=\"##top\">x</a>\n<cfif len( q )>\n</cfif>\n</cfoutput>",
			[]string{"len"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := tagFileCalls(t, c.src); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A string inside a `#...#` span may hold a span of its own — CFML nests them —
// and `interpolatedSpans` took the first `#` it met as the close. So the *inner*
// opening hash terminated the outer span, the outer call was sub-parsed from a
// fragment, and every pairing after it on the line was inverted.
//
// `Scanner.scanHashExpr` already reads CFScript this way. This is the same rule
// for the text a tag parser hands over, and the two are the parallel
// implementations CLAUDE.md warns about.
func TestAnInterpolatedSpanMayHoldAStringHoldingASpan(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{
			// ContentBox's themes write asset paths this way: 24 sites.
			"nested span in a quoted argument",
			`<link href="#cb.themeRoot()#/#html.elixirPath( root='#cb.themeRoot()#/inc' )#">`,
			[]string{"elixirPath", "themeRoot", "themeRoot"},
		},
		{
			"doubled quotes inside the nested span's argument",
			`<div>#replace("#a.b#","{u}","<a href=""h"">x</a>")#</div>`,
			[]string{"replace"},
		},
		{
			// matchingHash gives up on an unterminated quote, and the scan used
			// to give up with it — losing every span later in the same chunk.
			// An apostrophe in prose is enough to reach that.
			"an unterminated quote does not end the scan",
			"<cfoutput>Rule #1: don't #svc.load()# #svc.save()#</cfoutput>",
			[]string{"load", "save"},
		},
		{
			"a plain span is unaffected",
			`<cfoutput>#svc.load( "a" )#</cfoutput>`,
			[]string{"load"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := tagFileCalls(t, c.src); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A CF tag holds an expression, and an expression holds strings — so the `>`
// that closes the tag is not always the first one. `<cfset>` ended at the first
// `>` anywhere, so everything after it was in no tag and in no text the walk
// scans, and was recorded nowhere. Lucee's cache-driver components and its
// `<P>`-asserting tests are both written this way.
func TestATagEndsAtAQuoteAwareAngleBracket(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{"markup in a cfset's string", `<cfset x = array( f( "a<br>b" ), g( "c" ) )>`,
			[]string{"array", "f", "g"}},
		{"an entity then a bracket", `<cfset x = array( f( "p&lt;new line>" ), g( "c" ) )>`,
			[]string{"array", "f", "g"}},
		{"spanning lines", "<cfset x = array(\n f( \"a<br>b\" )\n ,g( \"c\" )\n)>",
			[]string{"array", "f", "g"}},
		{"single quotes", `<cfset x = array( f( 'a<br>b' ), g( 'c' ) )>`,
			[]string{"array", "f", "g"}},

		// A doubled quote is CFML's escape, so it does not close the string and
		// the `>` after it is still inside one.
		{"a doubled quote inside the string", `<cfset x = array( f( "a""b<br>c" ), g( "d" ) )>`,
			[]string{"array", "f", "g"}},

		// A quote that never closes falls back to the plain scan, so a
		// malformed tag cannot swallow the rest of the file.
		{"an unterminated quote does not run away", "<cfset x = f( \"a )>\n<cfset y = g()>",
			[]string{"f", "g"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := tagFileCalls(t, c.src); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A bare CR ends a line too. Five of the 5,629 corpus files use CR-only
// endings, and in one the first `//` swallowed everything after it — the whole
// of ColdBox's EventHandler.cfc, 2.5KB on what the scanner read as one line.
//
// Only the comment's end moves: line numbers still count `\n` alone, which is
// what tree-sitter does, so the two keep agreeing about where a call is.
func TestALineCommentEndsAtACarriageReturn(t *testing.T) {
	src := "component {\r" +
		"\t// Register the controller\r" +
		"\tfunction init() {\r" +
		"\t\tvariables.log = svc.getLogger();\r" +
		"\t}\r" +
		"}\r"

	if got := tagFileCalls(t, src); !slices.Equal(got, []string{"getLogger"}) {
		t.Errorf("got %v want [getLogger]", got)
	}

	// CRLF was never affected and must stay that way.
	crlf := strings.ReplaceAll(src, "\r", "\r\n")
	if got := tagFileCalls(t, crlf); !slices.Equal(got, []string{"getLogger"}) {
		t.Errorf("CRLF: got %v want [getLogger]", got)
	}
}

// A literal <script> block with no CFML tag of its own is a RegionSkip: opaque,
// so its JavaScript is never read as CFML. Its `#...#` spans are CFML all the
// same — a <script> body inside <cfoutput> is where a page writes
// `var id = "#prc.oContent.getContentID()#";` — and dropping the region dropped
// those with it. ContentBox's themes and admin panels are full of them: 124
// name-keyed sites over 34 corpus files, the largest single class left when it
// was found.
//
// The region stays opaque for everything else, which is the point: scanning the
// JavaScript would invent a call for every function it defines or calls.
func TestAScriptRegionGivesUpItsInterpolation(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{
			"interpolation in a script tag's attribute",
			`<script src="#cb.themeRoot()#/#html.elixirPath( root='#cb.themeRoot()#/inc' )#"></script>`,
			[]string{"elixirPath", "themeRoot", "themeRoot"},
		},
		{
			"interpolation in a script body",
			"<cfoutput>\n<script>\nvar x = \"#prc.o.getContentID()#\";\nvar y = \"#prc.o.getSite().getSlug()#\";\n</script>\n</cfoutput>",
			[]string{"getContentID", "getSite", "getSlug"},
		},

		// The JavaScript around the spans stays opaque. Without that, every
		// function a page defines or calls becomes a CFML call site.
		{"plain JavaScript is not CFML", `<script>var a = 1; foo(); bar();</script>`, nil},
		{"a function declaration is not CFML", `<script>function notACall(){ jsOnly(); }</script>`, nil},
		{"a jQuery id selector invents nothing", `<script>$( '#search' ).typeahead( x );</script>`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := tagFileCalls(t, c.src)
			if len(got) == 0 && c.want == nil {
				return
			}

			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}

	// A <script> holding a CFML tag is deliberately *not* a skip region, so it
	// parses as ordinary tag content. That carve-out predates this and must
	// keep working.
	src := "<cfoutput>\n<script>\nvar x = \"#prc.o.getContentID()#\";\n<cfif len( q )>a</cfif>\n</script>\n</cfoutput>"
	if got := tagFileCalls(t, src); !slices.Equal(got, []string{"getContentID", "len"}) {
		t.Errorf("script holding CFML: got %v want [getContentID len]", got)
	}
}

package parser

import (
	"fmt"
	"strings"
	"testing"
)

// spanSet renders spans as "start-end kind" so a failure reads as the source
// lines it names. Kinds: 0 comment, 1 block, 2 list, 3 case.
func spanSet(spans []Span) string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, fmt.Sprintf("%d-%d %d", s.Start, s.End, s.Kind))
	}

	return strings.Join(out, " | ")
}

// Each case is one rule of the script bracket pass, and each fails with its
// rule removed. The expected spans were checked against what the tree-sitter
// folding this replaced reported for the same source.
func TestStructureSpans(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{
			// A fold starts on its statement's first line — the `function`,
			// the `if` — and, when the bracket is further down because the
			// parameters or the condition wrapped, from the bracket's own
			// line as well, so the body can be folded with the signature on
			// screen.
			name: "wrapped parameters and condition",
			src: `component {
	function f(
		a,
		b
	) {
		if ( a &&
			b ) {
			return 1;
		}
	}
}
`,
			want: []string{"1-4 2", "5-8 1", "6-8 1", "1-9 1", "4-9 1", "0-10 1"},
		},
		{
			// Each branch folds, and so does the whole chain. An `else` that
			// is not the last folds to the end of the chain too, as the
			// statement nested in the first that it is. A catch does not.
			name: "if and try chains",
			src: `component {
	function f() {
		if ( a ) {
			x();
		} else if ( b ) {
			y();
		} else {
			z();
		}
		try {
			x();
		} catch ( any e ) {
			y();
		} finally {
			z();
		}
	}
}
`,
			want: []string{
				"2-4 1", "4-6 1", "6-8 1", "2-8 1", "4-8 1", "6-8 1",
				"9-11 1", "11-13 1", "13-15 1", "9-15 1", "1-16 1", "0-17 1",
			},
		},
		{
			// A case runs from its label to its last token; an empty one
			// falling through to the next has nothing to fold.
			name: "switch cases",
			src: `component {
	function f() {
		switch ( a ) {
			case 1:
				x();
				break;
			case 2:
			default:
				y();
				z();
		}
	}
}
`,
			want: []string{"3-5 3", "7-9 3", "2-10 1", "1-11 1", "0-12 1"},
		},
		{
			// Argument lists and array literals fold; a grouping parenthesis
			// and an index do not. A chain written a call per line folds from
			// its first line to the last call's `)`.
			name: "lists, groups, indexes and chains",
			src: `component {
	function f() {
		var r = call(
			1,
			2
		);
		var g = ( a +
			b );
		var arr = [
			1,
			2
		];
		var i = arr[
			1
		];
		return svc
			.a()
			.b(
				1
			);
	}
}
`,
			want: []string{"2-5 2", "8-11 2", "15-16 2", "15-19 2", "17-19 2", "1-20 1", "0-21 1"},
		},
		{
			// CFScript does not require semicolons, and without the newline
			// rule the struct would fold from `a = 1`.
			name: "no semicolons",
			src: `component {
	function f() {
		a = 1
		b = {
			c: 2
		}
		it( "x", function() {
			expect( 1 )
		} )
	}
}
`,
			want: []string{"3-5 1", "6-8 1", "6-8 2", "1-9 1", "0-10 1"},
		},
		{
			// A run of attributes is one statement across lines, including a
			// valueless attribute on a line of its own before the body. A
			// keyword on a new line begins a new statement, and a script tag
			// ending in `;` folds as a statement.
			name: "attribute statements",
			src: `component
	extends="base"
	singleton
	threadsafe
{
	property name="a"
	property
		name="b"
		default="string";

	function f() {
		admin
			action="update"
			type="web";
		lock name="x" {
			y();
		}
	}
}
`,
			want: []string{"6-8 1", "11-13 1", "14-16 1", "10-17 1", "0-18 1", "4-18 1"},
		},
		{
			// Mid-edit: an unclosed bracket is dropped rather than reported
			// with a guessed end.
			name: "unbalanced",
			src: `component {
	function f() {
		if ( a ) {
			x(
		}
	}
`,
			want: []string{"2-4 1", "1-5 1"},
		},
	}

	for _, c := range cases {
		if got, want := spanSet(StructureSpans(c.src)), strings.Join(c.want, " | "); got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// Each case is one rule of the tag pass, and each fails with its rule removed.
// The expected spans were checked against what the tree-sitter folding this
// replaced reported for the same source.
func TestTagStructureSpans(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{
			// Tags pair on a stack. <cfset> and <br> have no closing tag and
			// an unclosed <p> is discarded when </div> pops past it.
			name: "elements and bodiless tags",
			src:  "<div>\n\t<cfset x = 1>\n\t<br>\n\t<p>unclosed\n\t<ul>\n\t\t<li>a</li>\n\t</ul>\n</div>\n",
			want: []string{"4-6 1", "0-7 1"},
		},
		{
			// Each branch runs to the last content before </cfif>: a
			// <cfelseif> holds the branches after it, and a branch has no
			// closer of its own to keep on screen.
			name: "cfif branches",
			src:  "<cfif a>\n\tone\n<cfelseif b>\n\ttwo\n\ttwo\n<cfelse>\n\tthree\n\tthree\n</cfif>\n",
			want: []string{"0-8 1", "2-7 3", "5-7 3"},
		},
		{
			// <cfscript> pairs with </cfscript> across the script region
			// between them, which the bracket pass reads.
			name: "cfscript block",
			src:  "<cfoutput>\n<cfscript>\n\tif ( a ) {\n\t\tx();\n\t}\n</cfscript>\n</cfoutput>\n",
			want: []string{"2-4 1", "1-5 1", "0-6 1"},
		},
		{
			// ClassifyRegions makes this one script region; the tags around
			// it are still there to fold, and the code inside to read.
			name: "a page that is one cfscript block",
			src:  "<cfscript>\n\tif ( a ) {\n\t\tx();\n\t}\n</cfscript>\n",
			want: []string{"1-3 1", "0-4 1"},
		},
		{
			// JavaScript's `a<b` is not a tag, and a "</div>" in one of its
			// strings does not close the page's <div>.
			name: "raw script",
			src:  "<div>\n<script>\n\tvar s = \"</div>\";\n\tif (a<b) {\n\t}\n</script>\n</div>\n",
			want: []string{"1-5 1", "0-6 1"},
		},
		{
			// A <script> holding CF tags is walked, as ClassifyRegions
			// treats it.
			name: "script holding CF tags",
			src:  "<script>\n\t<cfif a>\n\t\tx();\n\t\ty();\n\t</cfif>\n</script>\n",
			want: []string{"1-4 1", "0-5 1"},
		},
		{
			// An opening tag whose attributes wrap folds by itself; and a
			// page of plain HTML, which ClassifyRegions reads as CFScript,
			// is walked as markup.
			name: "wrapping attributes in plain HTML",
			src:  "<button\n\ttype=\"button\"\n\tvalue=\"x\"\n>\n\tgo\n</button>\n",
			want: []string{"0-3 1", "0-5 1"},
		},
		{
			// A CF tag inside an HTML tag is read, not swallowed as part of
			// the attributes, so the <cfif> folds. (0-1 is the <tr>'s
			// opening tag up to the <cfif>, which the server drops: its fold
			// would cover no line.)
			name: "CF tag inside an HTML tag",
			src:  "<tr\n\t<cfif a>\n\t\tclass=\"x\"\n\t</cfif>\n>\n\t<td>x</td>\n</tr>\n",
			want: []string{"0-1 1", "1-3 1", "0-6 1"},
		},
		{
			name: "comments",
			src:  "<!--- a\n b --->\n<!-- c\n d -->\n",
			want: []string{"0-1 0", "2-3 0"},
		},
	}

	for _, c := range cases {
		if got, want := spanSet(StructureSpans(c.src)), strings.Join(c.want, " | "); got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// Classifying a word is on every identifier of the document, and was a fifth
// of the scan when it compared lists with EqualFold.
func TestClassifyDoesNotAllocate(t *testing.T) {
	words := []string{"Return", "ELSE", "someIdentifierLongerThanAnyKeyword", "default", "x"}

	n := testing.AllocsPerRun(100, func() {
		for _, w := range words {
			_ = classify(w)
		}
	})
	if n != 0 {
		t.Errorf("classify allocated %v times per run", n)
	}
}

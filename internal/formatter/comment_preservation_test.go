package formatter

import (
	"strings"
	"testing"
)

// TestCommentsSurviveFormatting covers comments the formatter used to delete
// outright. Each position is one where the rendering path rebuilds a construct
// from named fields, and a comment belonging to no field fell through the gap.
// Commenting a line out is how a setting or an operand gets parked without
// losing it, so dropping it is a silent loss of what the code says about itself.
func TestCommentsSurviveFormatting(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"in a block tag's attribute list",
			"<cfmail to=\"a\"\n\tsubject=\"s\"\n\t<!--- server=\"x\" --->\n\tfrom=\"b\">\nbody\n</cfmail>",
			`<!--- server="x" --->`,
		},
		{
			"in a self-closing tag's attribute list",
			`<cfparam name="a" type="string" <!--- default="740" --->default="100%" />`,
			`<!--- default="740" --->`,
		},
		{
			"between an assignment's operator and its value",
			"<cfset ok =\n\t<!---oldCall(--->\n\tnewCall(a=\"1\")>",
			`<!---oldCall(--->`,
		},
		{
			"between the operands of a concatenation",
			"<cfset cols = \"a,\" &\n\t\"b,\" &\n<!--- \"c,\" & --->\n\t\"d\">",
			`<!--- "c," & --->`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := format(t, tc.src)
			if !strings.Contains(out, tc.want) {
				t.Errorf("comment was dropped\nwant to contain: %s\ngot:\n%s", tc.want, out)
			}
		})
	}
}

// TestCommentsNotDuplicated pins the other direction. A word operator has no
// token child, so the operator is recovered from the raw source between the
// operands — which already carries any comment sitting in that gap. Adding the
// comment back on top emitted it twice.
func TestCommentsNotDuplicated(t *testing.T) {
	src := "<cfif (a GT 0 AND b LT 0)\n<!--- review --->\n\tOR (c EQ 1)>\nx\n</cfif>"

	out := format(t, src)
	if n := strings.Count(out, "<!--- review --->"); n != 1 {
		t.Errorf("comment emitted %d times, want 1\ngot:\n%s", n, out)
	}
}

// TestBodyCommentsStayInTheBody guards the fix against overreaching. A tag's
// attributes and its body can be siblings under one node, so a rule based on
// node shape alone hauled the body's comments up into the attribute list —
// where they were emitted a second time, inside the opening tag.
func TestBodyCommentsStayInTheBody(t *testing.T) {
	src := "<cfoutput>\n<!--- first --->\n<p>x</p>\n<!--- second --->\n</cfoutput>"

	out := format(t, src)
	for _, c := range []string{"<!--- first --->", "<!--- second --->"} {
		if n := strings.Count(out, c); n != 1 {
			t.Errorf("%s emitted %d times, want 1\ngot:\n%s", c, n, out)
		}
	}

	if strings.Contains(out, "<cfoutput\n") || strings.Contains(out, "<cfoutput <!---") {
		t.Errorf("body comment was hoisted into the opening tag\ngot:\n%s", out)
	}
}

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

// TestSavecontentBodyPreserved covers a body being deleted outright. Emitting
// only the children whose kind is a known cf_savecontent_body* missed content
// the grammar places elsewhere: a body made purely of comments parses as an
// *empty* cf_savecontent_body with the comments following it as siblings, so
// nothing at all was written between the tags.
func TestSavecontentBodyPreserved(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"body of only an HTML comment",
			"<cfsavecontent variable=\"d\">\n<!-- note -->\n</cfsavecontent>",
			"<!-- note -->",
		},
		{
			"body of only a CFML comment",
			"<cfsavecontent variable=\"d\">\n<!--- note --->\n</cfsavecontent>",
			"<!--- note --->",
		},
		{
			"comments around interpolated output",
			"<cfsavecontent variable=\"debug\">\n<!--=====-->\n<!-- t: <cfoutput>#x#</cfoutput> -->\n<!--=====-->\n</cfsavecontent>",
			"<!-- t: <cfoutput>#x#</cfoutput> -->",
		},
		{
			"uppercase tag name",
			"<CFSAVECONTENT VARIABLE=\"d\">\n<!-- note -->\n</CFSAVECONTENT>",
			"<!-- note -->",
		},
		{
			"ordinary text body still survives",
			"<cfsavecontent variable=\"d\">\nhello\n</cfsavecontent>",
			"hello",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := format(t, tc.src)
			if !strings.Contains(out, tc.want) {
				t.Errorf("savecontent body was dropped\nwant to contain: %s\ngot:\n%s", tc.want, out)
			}
		})
	}
}

// TestSavecontentEmptyBodyStaysEmpty checks the slice does not invent content
// for a genuinely empty tag.
func TestSavecontentEmptyBodyStaysEmpty(t *testing.T) {
	out := format(t, `<cfsavecontent variable="d"></cfsavecontent>`)
	if !strings.Contains(out, `<cfsavecontent variable="d"></cfsavecontent>`) {
		t.Errorf("empty savecontent not preserved\ngot:\n%s", out)
	}
}

// TestCFSetKeepsACommentBeforeItsValue covers `<cfset x = /* why */ f()>`. The
// comment is a child of the assignment_expression, between the left and right
// fields, so rendering the node from those fields alone dropped it — while the
// identical comment written *after* the value survived, because that one is a
// child of the tag rather than of the assignment. delimitedComments existed for
// exactly this, but matched on node kind, and the document grammar gives a
// `/* … */` in this position the plain "comment" kind rather than
// "block_comment" — the same kind it gives "//". The text is what tells the two
// apart, and only the line form is unmovable.
func TestCFSetKeepsACommentBeforeItsValue(t *testing.T) {
	t.Parallel()

	src := "<cfcomponent>\n\t<cffunction name=\"test\">\n" +
		"\t\t<cfset local.foo = /* hello world */ function(){return true;} /* bye */ >\n" +
		"\t</cffunction>\n</cfcomponent>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "/* hello world */")
	assertContains(t, out, "/* bye */")
}

// TestCFSetWithoutCommentsIsUnchanged is the boundary — nothing is inserted
// where there was no comment to carry across.
func TestCFSetWithoutCommentsIsUnchanged(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfcomponent>\n\t<cfset local.foo = 1>\n</cfcomponent>\n")

	assertContains(t, out, "<cfset local.foo = 1")
	assertNotContains(t, out, "/*")
}

// TestDeclarationCommentIsNotTreatedAsADeclarator covers a comment sitting
// inside a `var` declaration — `var colTypes = [ "a", "b" ]// note`, where the
// statement has no semicolon of its own. The comment is a named child of the
// variable_declaration, and the renderer appended every named child to the
// declarator list: the comment was comma-joined as though it were a second
// declaration, and the terminating semicolon then went after it, where the
// comment swallowed it.
func TestDeclarationCommentIsNotTreatedAsADeclarator(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tvar colTypes = [ \"varchar\", \"bigint\" ]//, \"double\"] // more\n\t\treturn 1;\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, `var colTypes = ["varchar", "bigint"];`)
	assertContains(t, out, `//, "double"] // more`)
	assertNotContains(t, out, `"bigint"],`)
	assertReparses(t, out)

	// The semicolon must not end up inside the comment.
	for _, line := range strings.Split(out, "\n") {
		if at := strings.Index(line, "//"); at >= 0 && strings.Contains(line[at:], ";") {
			t.Errorf("the terminating semicolon was folded into a comment: %q", line)
		}
	}
}

// TestDeclarationCommentPlacementIsStable pins why the comment goes on a line
// of its own rather than trailing the semicolon: once the semicolon is emitted
// the comment is no longer part of the declaration, so a second pass parses it
// as a statement-level comment and puts it on its own line. Trailing it on the
// first pass made the formatter's output differ from its output on that output.
func TestDeclarationCommentPlacementIsStable(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tvar x = [ 1, 2 ]// note\n\t\treturn 1;\n\t}\n}\n</cfscript>\n"

	once := formatGuarded(t, src)
	twice := formatGuarded(t, once)

	if once != twice {
		t.Errorf("formatting is not a fixed point:\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
}

// TestLineCommentAmongParametersIsNotInlined covers a `//` comment inside a
// function *expression*'s parameter list. The declaration path breaks the line
// so the comment ends it (joinSignatureAttrs); this path renders on one line,
// where the comment swallows the closing paren and the body's opening brace.
// An expression has nowhere to break to, so the list is reproduced as written.
func TestLineCommentAmongParametersIsNotInlined(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\nx = function( required string a, // why\n) { return 1; };\n</cfscript>\n")

	assertContains(t, out, "// why")
	assertNotContains(t, out, "// why)")
	assertReparses(t, out)
}

// TestLineCommentAmongFunctionExpressionAnnotationsIsNotInlined is the same
// defect one node up — a comment among a function expression's annotations,
// where the swallowed token is the brace opening the closure.
func TestLineCommentAmongFunctionExpressionAnnotationsIsNotInlined(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\ndescribe(\"x\", function() // note\n{ y = 1; });\n</cfscript>\n")

	assertContains(t, out, "// note")

	for _, line := range strings.Split(out, "\n") {
		if at := strings.Index(line, "// note"); at >= 0 && strings.Contains(line[at:], "{") {
			t.Errorf("the closure's opening brace was folded into a comment: %q", line)
		}
	}

	assertReparses(t, out)
}

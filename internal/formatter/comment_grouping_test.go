package formatter

import "testing"

// TestCommentsAreTransparentToGrouping covers the last blank-line oscillation.
// Whether the grammar hands a comment to a tag's body or to the siblings after
// it depends on stray trailing whitespace, and only a body is grouped — so a
// comment gained a blank line before it, the format removed the whitespace, the
// re-parse moved the comment out of the body, and the blank line went away.
//
// Comments are now transparent to grouping in both directions: they neither end
// the run they sit in nor start a new one. They already did not end one.
func TestCommentsAreTransparentToGrouping(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			"comment after a cfset in a cfelse branch, with trailing whitespace",
			"<cfif a is \"auto\">\n<cfelse>\n\t<cfset v = \"x\"><!--- paranoid --->\t\t \t\n</cfif>\n" +
				"<cfif b IS NOT true><!--- switch --->\n</cfif>",
		},
		{
			"same shape without the trailing whitespace",
			"<cfif a is \"auto\">\n<cfelse>\n\t<cfset v = \"x\"><!--- paranoid --->\n</cfif>",
		},
		{
			"comment between two different tag kinds",
			"<cfoutput>\n\t<cfset a = 1 />\n\t<!--- note --->\n\t<cfloop from=\"1\" to=\"3\" index=\"i\">\n\t\t<cfset a = 2 />\n\t</cfloop>\n</cfoutput>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			once := format(t, tc.src)

			twice := format(t, once)
			if once != twice {
				t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", once, twice)
			}
		})
	}
}

// TestHeaderCommentCountsAsASibling covers a comment carried from a construct's
// header into its body. Emitted outside the child loop it was invisible to the
// blank-line logic, so the statement after it got no blank line on the first
// format and did on the second, once the comment had become an ordinary child.
func TestHeaderCommentCountsAsASibling(t *testing.T) {
	src := "<cfscript>\n" +
		"\tif (a) {\n\t\tx();\n\t}\n" +
		"\telse //not detailed\n" +
		"\t{\n" +
		"\t\tif ( i EQ 2 ) {\n\t\t\ty();\n\t\t}\n" +
		"\t}\n</cfscript>"

	once := format(t, src)

	twice := format(t, once)
	if once != twice {
		t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
}

package formatter

import "testing"

// TestFormattingIsStableDespiteTrailingWhitespace covers a family of infinite
// oscillations. Stray whitespace in the source becomes a text node, and several
// layout decisions counted that node — so the first format inserted a blank
// line, removed the whitespace that caused it, and the second format took the
// other branch and removed the blank line again. Files never converged.
func TestFormattingIsStableDespiteTrailingWhitespace(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			// firstBodyChildIsArg saw the trailing spaces, not the cfargument.
			"trailing spaces after a cffunction tag",
			"<cfcomponent>\n\t<cffunction name=\"init\" returntype=\"struct\">    \n" +
				"\t\t<cfargument name=\"prs\" type=\"struct\" required=\"yes\">\n\t\t<cfset x = 1>\n" +
				"\t</cffunction>\n</cfcomponent>",
		},
		{
			// The grouping state was reset by the whitespace after "--->".
			"trailing tab after a comment",
			"<cfsilent>\n</cfsilent>\n<!--- records=\"#x#\"> --->\t\n" +
				"<cfimport prefix=\"container\" taglib=\"/t\">",
		},
		{
			// allSingleLine counted the whitespace node's newline.
			"trailing tab after a cfargument",
			"<cfcomponent>\n\t<cffunction name=\"g\" returntype=\"any\">\n" +
				"\t\t<cfargument name=\"id\" required=\"Yes\" type=\"string\" />\t\n" +
				"\t\t<cfreturn VARIABLES.store[ARGUMENTS.id]>\n\t</cffunction>\n</cfcomponent>",
		},
		{
			"no trailing whitespace, for contrast",
			"<cfsilent>\n</cfsilent>\n<!--- records=\"#x#\"> --->\n" +
				"<cfimport prefix=\"container\" taglib=\"/t\">",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			once := format(t, tc.src)

			twice := format(t, once)
			if once != twice {
				t.Errorf("formatting is not idempotent\n--- first pass ---\n%s\n--- second pass ---\n%s", once, twice)
			}
		})
	}
}

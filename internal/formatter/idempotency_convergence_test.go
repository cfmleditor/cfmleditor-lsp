package formatter

import "testing"

// TestGroupingIgnoresWhitespaceInEveryTagContext covers the two blank-line
// grouping sites missed the first time round — formatCFTag (a <cfsilent> body,
// say) and formatCFIfAlt. Trailing whitespace after a comment cleared the
// "previous sibling was a comment" fact, so the tag after it gained a blank
// line that the next pass removed again.
func TestGroupingIgnoresWhitespaceInEveryTagContext(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			"inside a cfsilent body",
			"<cfsilent>\n\t<cfset a = false>\n\n\t<!--- Student UD --->\t\n" +
				"\t<cfloop from=\"1\" to=\"10\" index=\"u\">\n\t\t<cfset a = true>\n\t</cfloop>\n</cfsilent>",
		},
		{
			"inside a cfelse branch",
			"<cfif y>\n\t<cfset a = 1>\n<cfelse>\n\t<!--- note --->\t\n" +
				"\t<cfloop from=\"1\" to=\"3\" index=\"u\">\n\t\t<cfset a = 2>\n\t</cfloop>\n</cfif>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			once := format(t, tc.src)

			twice := format(t, once)
			if once != twice {
				t.Errorf("formatting is not idempotent\n--- first ---\n%s\n--- second ---\n%s", once, twice)
			}
		})
	}
}

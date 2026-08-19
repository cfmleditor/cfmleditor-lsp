package formatter

import (
	"strings"
	"testing"
)

// TestNoWhitespaceOnlyLineGrowth covers a file that grew every time it was
// formatted, without limit. Content emitted verbatim — a <cfxml> body here —
// ends with the indentation that preceded its closing tag; starting a new line
// after it committed that indentation as a whitespace-only line, and the next
// format did the same to the line it had just created.
func TestNoWhitespaceOnlyLineGrowth(t *testing.T) {
	src := "<cfif x>\n\t<cfxml variable=\"headerContent\">\n<?xml version='1.0' ?>\n" +
		"\t\t<cfoutput>#ToString(d)#</cfoutput>\n\t</cfxml>\n</cfif>"

	out := format(t, src)

	for pass := 2; pass <= 4; pass++ {
		next := format(t, out)
		if next != out {
			t.Fatalf("pass %d changed the output; growth from %d to %d lines\n%s",
				pass, strings.Count(out, "\n"), strings.Count(next, "\n"), next)
		}

		out = next
	}
}

// TestNoTrailingWhitespaceOnlyLines states the rule the fix above establishes:
// the formatter never emits a line consisting only of whitespace.
func TestNoTrailingWhitespaceOnlyLines(t *testing.T) {
	cases := []string{
		"<cfif x>\n\t<cfxml variable=\"v\">\n<?xml version='1.0' ?>\n\t\t<cfoutput>#d#</cfoutput>\n\t</cfxml>\n</cfif>",
		"<cfcomponent>\n\t<cfscript>\n\t\tx = 1;\n\t</cfscript>\n</cfcomponent>",
		"<cfoutput>\n\t<p>hi</p>\n</cfoutput>",
	}

	for _, src := range cases {
		for i, line := range strings.Split(format(t, src), "\n") {
			if line != "" && strings.TrimSpace(line) == "" {
				t.Errorf("line %d is whitespace-only: %q", i+1, line)
			}
		}
	}
}

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

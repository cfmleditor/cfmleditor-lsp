package formatter

import (
	"strings"
	"testing"
)

// TestLayoutDecidedFromEmittedWidth covers a body whose source text fits inside
// LineWidth but whose emitted text does not, because the formatter adds " />" to
// tags the source wrote as ">". Measuring the source picked the inline layout,
// the first format then made the file wider, and the second format measured the
// new width and picked the other layout.
func TestLayoutDecidedFromEmittedWidth(t *testing.T) {
	src := "<cfcomponent>\n" +
		"\t<cffunction name=\"getEUDAdditionalFields\" access=\"public\" returntype=\"struct\" output=\"false\">\n\n" +
		"\t\t<cfset var result = StructNew()>\n\n" +
		"\t\t<cfset result = persist.getEUDAdditionalFields()>\n\n" +
		"\t\t<cfreturn result>\n" +
		"\t</cffunction>\n</cfcomponent>"

	once := format(t, src)

	twice := format(t, once)
	if once != twice {
		t.Errorf("layout differs between passes\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
}

// TestTrialRenderLeavesNoTrace pins the state handling. The measurement runs the
// real emitters and throws the output away, so anything they mutate — the
// buffer, the indent level, the column, queued comments — has to come back
// exactly as it was. A leak here would duplicate or drop content rather than
// merely misformat it.
func TestTrialRenderLeavesNoTrace(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"queued comment is emitted exactly once",
			"<cfscript>\nif (n eq 1)  // p ends with .cfc\n{\n\tp = 1;\n}\n</cfscript>",
			"// p ends with .cfc",
		},
		{
			"comment among attributes survives",
			"<cfparam name=\"a\" type=\"string\" <!--- default=\"740\" --->default=\"100%\" />",
			`<!--- default="740" --->`,
		},
		{
			"savecontent body survives",
			"<cfsavecontent variable=\"d\">\n<!-- note -->\n</cfsavecontent>",
			"<!-- note -->",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := format(t, tc.src)
			if n := strings.Count(out, tc.want); n != 1 {
				t.Errorf("%q appears %d times, want 1\ngot:\n%s", tc.want, n, out)
			}
		})
	}
}

// TestRendersOnOneLineRejectsWrapping guards the measurement itself. The
// emitters soft-wrap, so a body too long to fit comes back inside LineWidth by
// being split across lines — measuring width alone would call that a fit and
// take the inline branch for everything.
func TestRendersOnOneLineRejectsWrapping(t *testing.T) {
	long := strings.Repeat("word ", 60)
	out := format(t, "<div>\n"+long+"\n</div>")

	body := strings.Split(strings.TrimSpace(out), "\n")
	if len(body) < 3 {
		t.Errorf("long body was not broken up, so wrapping was mistaken for a fit\ngot:\n%s", out)
	}
}

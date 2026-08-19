package formatter

import (
	"strings"
	"testing"
)

// TestInlineComponentLiteralPreserved covers `new component { ... }`, an anonymous
// component defined at the point of use. The new_expression carries neither a
// constructor nor an arguments node — the class *is* the body — so rendering it from
// those two fields produced `new ()`, deleting the keyword and every property and
// method in the body. 16 files in the corpus, all Lucee's own tests.
func TestInlineComponentLiteralPreserved(t *testing.T) {
	src := `<cfscript>
obj = new component {
	property name="validator";
	public function getValidator() { return "mocked"; }
};
</cfscript>
`

	out := format(t, src)

	assertContains(t, out, "new component {")
	assertContains(t, out, `property name="validator";`)
	assertContains(t, out, "function getValidator()")
	assertNotContains(t, out, "new ()")
	assertReparses(t, out)
}

// TestScriptCFTagAttributesKeepSpaces covers a CF tag written in script syntax, whose
// attributes are separated by spaces rather than commas. The grammar hands the
// attribute list over as an `arguments` node full of assignment_expressions, exactly
// like a call's arguments, and the formatter joined them with ", " — inserting commas
// that were never in the source. That is a non-whitespace change, so the guard rejected
// the file and format-on-save silently did nothing on it.
func TestScriptCFTagAttributesKeepSpaces(t *testing.T) {
	tests := []struct {
		name string
		call string
		want string
	}{
		{
			name: "inline",
			call: `cfdirectory(directory="#dir#" action="create" mode="777");`,
			want: `cfdirectory(directory = "#dir#" action = "create" mode = "777");`,
		},
		{
			name: "two attributes",
			call: `cfhttp(url="http://example.com" method="get");`,
			want: `cfhttp(url = "http://example.com" method = "get");`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := format(t, "<cfscript>\n"+tt.call+"\n</cfscript>\n")

			assertContains(t, out, tt.want)
			assertNotContains(t, out, ",")
		})
	}
}

// TestScriptCFTagAttributesBrokenOntoLines checks the same when the attribute list is
// long enough to be split — the break path writes its own separators, so it needed the
// same treatment as the inline join.
func TestScriptCFTagAttributesBrokenOntoLines(t *testing.T) {
	src := `<cfscript>
cfdirectory(directory="#dir#" action="create" mode="777" recurse="true");
</cfscript>
`

	out := format(t, src)
	if strings.Contains(out, ",") {
		t.Errorf("comma inserted into a space-separated attribute list:\n%s", out)
	}

	for _, attr := range []string{"directory", "action", "mode", "recurse"} {
		assertContains(t, out, attr+" = ")
	}
}

// TestOrdinaryCallStillGetsCommas guards the other direction: an ordinary call's
// arguments must keep their commas, including when the list is broken onto lines.
func TestOrdinaryCallStillGetsCommas(t *testing.T) {
	src := `<cfscript>
writeOutput(one, two, three);
result = doSomething(alpha, beta, gamma, delta, epsilon);
</cfscript>
`

	out := format(t, src)

	assertContains(t, out, "writeOutput(one, two, three)")
	assertContains(t, out, "alpha,")
	assertContains(t, out, "epsilon")
}

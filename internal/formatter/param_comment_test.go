package formatter

import "testing"

// TestLineCommentAmongParametersKeepsTheParameter covers a `//` comment
// standing on its own line inside a parameter list — how a signature says what
// the argument below it is for.
//
// Entries in the flat parameter structure are joined with a space, and a
// standalone comment carried no boundary of its own, so it absorbed the
// parameter after it: `a, // why, b` rendered as `a,` and `// why b`, with b
// deleted into the comment. The whitespaceOnly guard caught the lost parameter
// and refused the whole file, so the only symptom was that it stopped
// responding to format-on-save.
//
// This is the same defect as the one fixed for signature annotations, one
// bracket further in.
func TestLineCommentAmongParametersKeepsTheParameter(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f(\n\t\ta,\n\t\t// why\n\t\tb\n\t) {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "// why\n")
	assertNotContains(t, out, "// why b")
	assertReparses(t, out)

	if twice := formatWithParamBreak(t, out, 0); twice != out {
		t.Errorf("not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", out, twice)
	}
}

// TestBlockCommentAmongParametersStaysWithItsParameter is the boundary. Only a
// line comment ends its line; a `/* … */` before a parameter reads as part of
// it and must keep sharing its entry, or an inline note would be pushed onto a
// line of its own.
func TestBlockCommentAmongParametersStaysWithItsParameter(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f(\n\t\ta,\n\t\t/* why */ b\n\t) {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "/* why */ b")
	assertReparses(t, out)
}

// TestCommentTrailingAParameterKeepsItsComma is the other boundary, and the
// reason the flush is conditional. A comment written after a parameter belongs
// to that parameter, and the comma separating it from the next one has not been
// read yet — ending the entry at the comment would drop it.
func TestCommentTrailingAParameterKeepsItsComma(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f(\n\t\ta /* first */,\n\t\tb\n\t) {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, ",")
	assertReparses(t, out)
}

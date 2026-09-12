package formatter

import "testing"

func formatWithParens(t *testing.T, src, spacing string) string {
	t.Helper()

	tree := parse(t, src)
	opts := testOpts()
	opts.WhitespaceOnly = true
	opts.ParenSpacing = spacing

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error (parenSpacing=%q): %v", spacing, err)
	}

	return string(out)
}

const parenSrc = "<cfscript>\n" +
	"while (i < 10) { i++; }\n" +
	"for (var i = 0; i < 3; i++) { y = i; }\n" +
	"x = (a + b) * c;\n" +
	"writeOutput(trim(s));\n" +
	"if (!isNull(q)) { z = 1; }\n" +
	"</cfscript>\n"

// TestParenSpacingUnsetKeepsTodaysMixedStyle is the setting's whole reason for
// existing, and the constraint on it. The formatter has always padded
// conditions and grouping while leaving call and parameter lists tight — two
// opposite hardcoded conventions — and unset has to reproduce that exactly, or
// adding the option reformats every file in every project that never asked for
// it.
func TestParenSpacingUnsetKeepsTodaysMixedStyle(t *testing.T) {
	out := formatWithParens(t, parenSrc, "")

	for _, want := range []string{"while ( i < 10 )", "for ( var i = 0;", "x = ( a + b ) * c", "if ( !isNull(q) )"} {
		assertContains(t, out, want)
	}

	assertContains(t, out, "writeOutput(trim(s))")
	assertReparses(t, out)
}

// TestParenSpacingPad and its tight counterpart are the two ways to ask for one
// rule in both places.
func TestParenSpacingPad(t *testing.T) {
	out := formatWithParens(t, parenSrc, "pad")

	for _, want := range []string{"while ( i < 10 )", "writeOutput( trim( s ) )", "if ( !isNull( q ) )"} {
		assertContains(t, out, want)
	}

	assertReparses(t, out)
}

func TestParenSpacingTight(t *testing.T) {
	out := formatWithParens(t, parenSrc, "tight")

	for _, want := range []string{"while (i < 10)", "for (var i = 0;", "x = (a + b) * c", "writeOutput(trim(s))", "if (!isNull(q))"} {
		assertContains(t, out, want)
	}

	assertNotContains(t, out, "( ")
	assertReparses(t, out)
}

// TestParenSpacingLeavesEmptyListsAlone covers the one shape neither padding
// applies to: "( )" is nobody's preference, in either mode.
func TestParenSpacingLeavesEmptyListsAlone(t *testing.T) {
	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\treturn g();\n\t}\n}\n</cfscript>\n"

	for _, spacing := range []string{"", "pad", "tight"} {
		out := formatWithParens(t, src, spacing)

		assertNotContains(t, out, "( )")
		assertContains(t, out, "function f()")
		assertReparses(t, out)
	}
}

// TestParenSpacingIsIdempotent is the property a formatting change is most
// likely to break: a second pass must not move the output.
func TestParenSpacingIsIdempotent(t *testing.T) {
	for _, spacing := range []string{"", "pad", "tight"} {
		once := formatWithParens(t, parenSrc, spacing)
		if twice := formatWithParens(t, once, spacing); once != twice {
			t.Errorf("parenSpacing=%q not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", spacing, once, twice)
		}
	}
}

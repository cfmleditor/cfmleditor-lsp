package formatter

import (
	"testing"
)

// Grammar v0.26.36 continues the pattern v0.26.33 through v0.26.35 established:
// a construct the grammar starts parsing is one the formatter starts rendering,
// whether or not it knows how. This release's construct is Lucee's one-word
// `elseif`.
//
// As with the earlier batches the whitespaceOnly guard caught it, so no file was
// corrupted in the default configuration — the user saw format-on-save stop
// working on a file that had formatted cleanly the release before.

// TestV2636ElseIfIsRenderedAsWritten covers `elseif`, as spelled in Lucee's own
// org/lucee/cfml/Query.cfc. The grammar gives it a clause of its own — an
// else_if_clause carrying an if_statement's condition/consequence/alternative,
// rather than an else_clause wrapping an if_statement, because there is no
// `else` token for that wrapper to match.
//
// The formatter knew the two shapes that existed before, so the new one fell
// through to the arm for "some statement after `else`": it wrote `else`, opened
// a block, and rendered the whole clause — keyword, condition and all — as the
// code inside it.
//
// The word is emitted as written rather than expanded to `else if`. Both pass
// the guard, which skips whitespace, so the choice is not forced by it:
// expanding is a rewrite no setting asked for, and contracting would break ACF,
// which has no such keyword.
func TestV2636ElseIfIsRenderedAsWritten(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tif ( a ) {\n\t\t\tx();\n\t\t} elseif ( b ) {\n\t\t\ty();\n\t\t} elseif ( c ) {\n\t\t\tz();\n\t\t} else {\n\t\t\tw();\n\t\t}\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "} elseif ( b ) {")
	assertContains(t, out, "} elseif ( c ) {")
	assertContains(t, out, "} else {")
	// The branch body belongs to the branch, not to a block the keyword was
	// buried inside.
	assertNotContains(t, out, "elseif ( b )\n")
	assertReparses(t, out)
}

// TestV2636ElseIfKeepsAPrecedingComment is the shape that first exposed the
// missing clause. A comment between the closing brace and the keyword sends
// elseLead down its other path, which writes the keyword at the start of a
// fresh line instead of attaching it to the brace.
func TestV2636ElseIfKeepsAPrecedingComment(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tif ( a ) {\n\t\t\tx();\n\t\t}\n\t\t// why\n\t\telseif ( b ) {\n\t\t\ty();\n\t\t}\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "// why\n            elseif ( b ) {")
	assertReparses(t, out)
}

// TestV2636ElseIfUnderNextLineBraces pins the clause against braceStyle, which
// has to move an `elseif`'s brace exactly as it moves an `else if`'s — and move
// the keyword off the closing brace to do it.
func TestV2636ElseIfUnderNextLineBraces(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tif ( a ) {\n\t\t\tx();\n\t\t} elseif ( b ) {\n\t\t\ty();\n\t\t} else {\n\t\t\tw();\n\t\t}\n\t}\n}\n</cfscript>\n"

	out := formatWithBraces(t, src, "next-line")

	assertContains(t, out, "\n            }\n            elseif ( b )\n            {")
	assertContains(t, out, "\n            }\n            else\n            {")
	assertReparses(t, out)
}

// TestV2636ElseIfIsIdempotent is the failure mode a clause the renderer does not
// know actually has: the first pass moves something, the second sees the new
// line break as source and moves it again.
func TestV2636ElseIfIsIdempotent(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tif ( a ) {\n\t\t\tx();\n\t\t} elseif ( b ) {\n\t\t\ty();\n\t\t} else {\n\t\t\tw();\n\t\t}\n\t}\n}\n</cfscript>\n"

	for _, style := range []string{"", "same-line", "next-line"} {
		once := formatWithBraces(t, src, style)
		if twice := formatWithBraces(t, once, style); twice != once {
			t.Errorf("elseif, braceStyle=%q, not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", style, once, twice)
		}
	}
}

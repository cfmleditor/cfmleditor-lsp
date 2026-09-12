package formatter

import (
	"strings"
	"testing"
)

// TestBracedCaseBodyIsAStatement covers `case 1: { … }` — a braced case body,
// which the grammar reports as a statement_block standing on its own rather
// than as any construct's body.
//
// The renderer for that shape wrote " {" wherever the cursor happened to be and
// left it there, so the brace landed in column one under the label and the next
// thing written — here the switch's own closing brace — shared its line as
// `}}`. Both are whitespace-only and the result was stable, so the guard passed
// it and the corpus counted the file clean; nothing in the harness could see it.
func TestBracedCaseBodyIsAStatement(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction sw(k) {\n\t\tswitch (k) {\n\t\t\tcase 1: {\n\t\t\t\ta();\n\t\t\t\tbreak;\n\t\t\t}\n\t\t}\n\t}\n}\n</cfscript>\n"

	out := formatWithBraces(t, src, "")

	// The two closing braces are separate statements and may not share a line.
	assertNotContains(t, out, "}}")

	// The brace opens its own line, indented under the label rather than
	// stranded in column one.
	assertNotContains(t, out, "case 1:\n {")
	assertContains(t, out, "case 1:\n")

	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "{" {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			t.Errorf("a block's opening brace is in column one: %q\n%s", line, out)
		}
	}

	assertReparses(t, out)

	if twice := formatWithBraces(t, out, ""); twice != out {
		t.Errorf("not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", out, twice)
	}
}

package formatter

import (
	"strings"
	"testing"
)

func formatWithBlockOpts(t *testing.T, src string, blanks, caseIndent bool) string {
	t.Helper()

	opts := testOpts()
	opts.WhitespaceOnly = true
	opts.BlankLinesInBlocks = blanks
	opts.SwitchCaseIndent = caseIndent

	out, err := Format([]byte(src), parse(t, src), opts)
	if err != nil {
		t.Fatalf("format error (blanks=%v caseIndent=%v): %v", blanks, caseIndent, err)
	}

	return string(out)
}

const padSrc = "<cfscript>\ncomponent {\n" +
	"\tfunction f(k) {\n" +
	"\t\tif (k) {\n\t\t\ta();\n\t\t}\n" +
	"\t\tswitch (k) {\n\t\t\tcase 1:\n\t\t\t\tb();\n\t\t\t\tbreak;\n\t\t\tdefault:\n\t\t\t\tc();\n\t\t}\n" +
	"\t}\n}\n</cfscript>\n"

// TestBlankLinesInBlocksDefaultPads is the constraint: true is what the
// formatter has always emitted, and every project already formatted by it
// depends on that not moving.
func TestBlankLinesInBlocksDefaultPads(t *testing.T) {
	t.Parallel()

	out := formatWithBlockOpts(t, padSrc, true, false)

	assertContains(t, out, "if ( k ) {\n\n")
	assertContains(t, out, "\n\n        }")
}

// TestBlankLinesInBlocksFalseIsCompact is the setting doing its job at both
// ends of a body.
func TestBlankLinesInBlocksFalseIsCompact(t *testing.T) {
	t.Parallel()

	out := formatWithBlockOpts(t, padSrc, false, false)

	assertNotContains(t, out, "{\n\n")

	for i, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "}" || i == 0 {
			continue
		}

		if prev := strings.Split(out, "\n")[i-1]; strings.TrimSpace(prev) == "" {
			t.Errorf("blank line left above a closing brace at line %d:\n%s", i+1, out)
		}
	}
}

// TestBlankLinesInBlocksFalseCollapsesAnEmptyBody covers the shape the padding
// reads worst on: an empty body is brace, blank, blank, brace.
func TestBlankLinesInBlocksFalseCollapsesAnEmptyBody(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction empty() {}\n}\n</cfscript>\n"

	assertContains(t, formatWithBlockOpts(t, src, false, false), "function empty() {\n")
	assertNotContains(t, formatWithBlockOpts(t, src, false, false), "{\n\n")
}

// TestBlankLinesInBlocksAgreeAcrossBothBlockRenderers is the invariant that
// makes formatting stable at all. A single-statement body gains its braces in a
// second renderer, whose padding has to match the braced one's exactly: on a
// second pass the braces are in the source, the braced renderer runs, and any
// difference in blank lines makes an untouched file produce a fresh diff on
// every save. Both now share blockPadOpen/blockPadClose, and this is what says
// so — it is the same assertion the load-bearing comment in scriptBlockOf2
// makes in prose.
func TestBlankLinesInBlocksAgreeAcrossBothBlockRenderers(t *testing.T) {
	t.Parallel()

	braceless := "<cfscript>\ncomponent {\n\tfunction f(k) {\n\t\tif (k) a();\n\t}\n}\n</cfscript>\n"
	braced := "<cfscript>\ncomponent {\n\tfunction f(k) {\n\t\tif (k) { a(); }\n\t}\n}\n</cfscript>\n"

	for _, blanks := range []bool{true, false} {
		a := formatWithBlockOpts(t, braceless, blanks, false)
		b := formatWithBlockOpts(t, braced, blanks, false)

		if a != b {
			t.Errorf("blanks=%v: the two block renderers disagree\n--- gained braces\n%s\n--- already braced\n%s", blanks, a, b)
		}
	}
}

// TestSwitchCaseIndentDefaultAlignsWithSwitch pins today's layout: the label
// sits at the `switch` keyword's own column, one back from its statements.
func TestSwitchCaseIndentDefaultAlignsWithSwitch(t *testing.T) {
	t.Parallel()

	out := formatWithBlockOpts(t, padSrc, true, false)

	assertContains(t, out, "\n            switch ( k ) {\n            case 1:\n                b();")
}

// TestSwitchCaseIndentMovesLabelAndStatementsTogether is the setting, and the
// mistake it is easy to make. Indenting the label alone leaves it level with
// the statements under it, which is neither style — the whole clause body has
// to move, so the label lands where the statements were and the statements one
// further in.
func TestSwitchCaseIndentMovesLabelAndStatementsTogether(t *testing.T) {
	t.Parallel()

	out := formatWithBlockOpts(t, padSrc, true, true)

	assertContains(t, out, "\n            switch ( k ) {\n                case 1:\n                    b();")
	assertContains(t, out, "\n                default:\n                    c();")
}

// TestBlockOptionsAreIdempotent — both settings change where lines break, which
// is what the second pass reads as the source's own layout.
func TestBlockOptionsAreIdempotent(t *testing.T) {
	t.Parallel()

	for _, blanks := range []bool{true, false} {
		for _, caseIndent := range []bool{true, false} {
			once := formatWithBlockOpts(t, padSrc, blanks, caseIndent)
			if twice := formatWithBlockOpts(t, once, blanks, caseIndent); twice != once {
				t.Errorf("blanks=%v caseIndent=%v not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s",
					blanks, caseIndent, once, twice)
			}
		}
	}
}

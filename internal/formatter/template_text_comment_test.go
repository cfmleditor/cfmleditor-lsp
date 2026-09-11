package formatter

import (
	"strings"
	"testing"
)

// A `.cfm` may be JavaScript — Lucee ships one, opening
// `<cfcontent type="text/javascript">`. To the CFML grammar that body is
// template text, so it went through collapseWhitespace and writeWrapped and was
// reflowed as prose. JavaScript's `//` means nothing to CFML, so the next line
// was folded up onto a comment and the code after it became part of the comment.
// Only whitespace changed, so the guard passed and the file was written: the one
// defect in FORMATTER-ISSUES.md that destroyed a file rather than refusing it.

// TestTemplateTextWithLineCommentsKeepsItsLines is the defect.
func TestTemplateTextWithLineCommentsKeepsItsLines(t *testing.T) {
	t.Parallel()

	src := "<cfcontent type=\"text/javascript\">\n" +
		"var a = 1;\n" +
		"\n" +
		"// remove the current block (if there is one)\n" +
		"if (full && pageBlock)\n" +
		"\tremove(window, {fadeOut:0});\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "// remove the current block (if there is one)")

	for _, line := range strings.Split(out, "\n") {
		at := strings.Index(line, "//")
		if at < 0 || (at > 0 && line[at-1] == ':') {
			continue
		}

		if rest := strings.TrimSpace(line[at:]); !strings.HasSuffix(rest, "one)") {
			t.Errorf("code was folded onto a line comment: %q", line)
		}
	}
}

// TestOrdinaryTemplateTextStillReflows is the boundary. The carve-out is keyed
// on the comment, so prose without one is collapsed and wrapped as before —
// otherwise every HTML document stops being formatted.
func TestOrdinaryTemplateTextStillReflows(t *testing.T) {
	t.Parallel()

	out := format(t, "<p>\n\tone\n\ttwo\n\tthree\n</p>\n")

	assertContains(t, out, "one two three")
}

// TestURLInTemplateTextDoesNotPinTheLine is the other boundary: the "//" of a
// scheme is not a comment, so a link in prose must not stop the text reflowing.
// isLineCommentStart already makes that distinction; this pins that the text
// path uses it rather than searching for a bare "//".
func TestURLInTemplateTextDoesNotPinTheLine(t *testing.T) {
	t.Parallel()

	out := format(t, "<p>\n\tsee http://example.com/x\n\tfor details\n</p>\n")

	assertContains(t, out, "see http://example.com/x for details")
}

// TestHoldsLineComment pins the predicate directly, since the two behaviours
// above hang off it.
func TestHoldsLineComment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		want bool
	}{
		{"plain prose", false},
		{"// a comment", true},
		{"code; // trailing", true},
		{"http://example.com", false},
		{"https://example.com and https://other.example", false},
		{"see http://example.com // then a comment", true},
		{"a / b / c", false},
	}

	for _, tc := range cases {
		if got := holdsLineComment(tc.text); got != tc.want {
			t.Errorf("holdsLineComment(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

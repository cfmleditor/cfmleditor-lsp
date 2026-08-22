package formatter

import (
	"strings"
	"testing"
)

// TestPreformattedElementsKeepTheirWhitespace covers the one place where the
// formatter's usual assumption — that whitespace can be rearranged freely — is
// false. The generic element path collapsed the body of a <pre> onto one line,
// and the whitespaceOnly guard passed it, because by its definition nothing but
// whitespace had changed. In these elements the whitespace *is* the content, so
// the guard structurally cannot catch this and the element has to be reproduced
// from source instead.
func TestPreformattedElementsKeepTheirWhitespace(t *testing.T) {
	cases := []struct {
		name string
		body string
		tag  string
	}{
		{"pre", "line one\n    indented\nline three", "pre"},
		{"textarea", "first\n\tsecond\n\n  fourth", "textarea"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "<" + tc.tag + ">\n" + tc.body + "\n</" + tc.tag + ">\n"

			out := format(t, src)
			if !strings.Contains(out, tc.body) {
				t.Errorf("body was rewritten\n src: %q\n out: %q", src, out)
			}
		})
	}
}

// TestPreformattedElementWithAttributes checks the element is still recognised
// when the tag carries attributes, since the lookup walks for the tag_name child.
func TestPreformattedElementWithAttributes(t *testing.T) {
	body := "kept   as\n        written"
	src := `<pre class="code" data-x="1">` + "\n" + body + "\n</pre>\n"

	out := format(t, src)
	if !strings.Contains(out, body) {
		t.Errorf("body was rewritten\n out: %q", out)
	}
}

// TestOrdinaryElementStillCollapses is the other direction: normal elements must
// keep being reflowed, or the carve-out is too wide.
func TestOrdinaryElementStillCollapses(t *testing.T) {
	src := "<div>\nline one\n    indented\nline three\n</div>\n"

	out := format(t, src)
	if strings.Contains(out, "    indented") {
		t.Errorf("a <div> should still be reflowed\n out: %q", out)
	}
}

// TestAppendTrailingCommaDoesNotDuplicate covers the buffer-aliasing bug in
// appendTrailingComma: Bytes() returns the buffer's own backing array, so
// Reset() followed by writing that slice back had WriteByte(',') clobber the
// byte the next Write re-read. "SELECT a\n" came out as "SELECT a,,".
func TestAppendTrailingCommaDoesNotDuplicate(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"trailing newline", "SELECT a\n", "SELECT a,\n"},
		{"trailing newline and indent", "SELECT a\n\t", "SELECT a,\n\t"},
		{"no trailing whitespace", "SELECT a", "SELECT a,"},
		{"multi line", "SELECT a\n\tFROM t\n", "SELECT a\n\tFROM t,\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := New(DefaultOptions())
			f.out.WriteString(tc.in)

			if !f.appendTrailingComma() {
				t.Fatal("appendTrailingComma reported failure")
			}

			if got := f.out.String(); got != tc.want {
				t.Errorf("appendTrailingComma(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

package formatter

import (
	"strings"
	"testing"
)

func formatAtWidth(t *testing.T, src string, width int) string {
	t.Helper()

	tree := parse(t, src)
	opts := testOpts()
	opts.LineWidth = width

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return string(out)
}

// quotedRunsIntact reports whether every quoted attribute value in s stays on
// one line. Quotes count only inside a tag — elsewhere an apostrophe is a
// letter, and treating it as a delimiter is exactly the mistake this guards.
func quotedRunsIntact(s string) bool {
	var (
		inTag bool
		quote byte
	)

	for i := 0; i < len(s); i++ {
		c := s[i]

		switch {
		case quote != 0:
			switch {
			case c == '\n':
				return false
			case c != quote:
			case i+1 < len(s) && s[i+1] == quote:
				i++
			default:
				quote = 0
			}
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case inTag && (c == '"' || c == '\''):
			quote = c
		}
	}

	return true
}

// writeWrapped is handed whole elements verbatim, attributes included, so a
// plain "break at the last space before the limit" search broke lines inside
// attribute values. The whitespace-only guard cannot catch it — only whitespace
// changed — but the value did change, and for a CFML tag whose attribute
// carries a runtime string the newline and indentation end up in the data.
func TestWrapNeverBreaksInsideAQuotedValue(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "void element with a long attribute value",
			src:  `<img src="x.png" alt="a fairly long alternative text describing the picture">` + "\n",
		},
		{
			name: "cf tag whose attribute is runtime data",
			src:  `<cfhttpparam name="body" value="hello world this value is used verbatim at runtime">` + "\n",
		},
		{
			name: "single-quoted value",
			src:  `<img src='y.png' alt='another fairly long alternative text for the picture'>` + "\n",
		},
		{
			name: "doubled quote escaping a quote inside the value",
			src:  `<cfhttpparam name="body" value="she said ""hello there"" and then walked away slowly">` + "\n",
		},
		{
			name: "several attributes, all long",
			src:  `<img src="some/rather/long/path/to/an/image.png" alt="descriptive text here" title="and a title too">` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, width := range []int{20, 30, 40, 60} {
				out := formatAtWidth(t, tt.src, width)
				if !quotedRunsIntact(out) {
					t.Errorf("width %d broke a line inside a quoted value:\n%s", width, out)
				}
			}
		})
	}
}

// The wrapping itself must still happen where it is safe: between attributes.
func TestWrapStillBreaksBetweenAttributes(t *testing.T) {
	src := `<img src="a.png" alt="one" title="two" width="10" height="20" loading="lazy">` + "\n"

	out := formatAtWidth(t, src, 30)
	if !strings.Contains(out, "\n") || len(strings.Split(strings.TrimRight(out, "\n"), "\n")) < 2 {
		t.Errorf("expected the attribute list to wrap at width 30, got:\n%s", out)
	}

	if !quotedRunsIntact(out) {
		t.Errorf("broke inside a quoted value:\n%s", out)
	}
}

// A single unbreakable token longer than the limit must be emitted whole rather
// than cut: LineWidth is a soft limit.
func TestWrapEmitsOverlongUnbreakableTextWhole(t *testing.T) {
	long := strings.Repeat("x", 120)
	src := `<img src="` + long + `.png">` + "\n"

	out := formatAtWidth(t, src, 40)
	if !strings.Contains(out, long) {
		t.Errorf("overlong token was not emitted intact:\n%s", out)
	}
}

// Tracking quotes outside tags treats an apostrophe as an opening delimiter, so
// everything after it becomes unbreakable — wrapping silently switches off for
// ordinary English. This is the regression the first version of the fix had.
func TestWrapStillWrapsProseContainingApostrophes(t *testing.T) {
	src := "<cfoutput>I won't display this because the service contains an error and it can't recover</cfoutput>\n"

	out := formatAtWidth(t, src, 40)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Errorf("prose with apostrophes did not wrap at width 40:\n%s", out)
	}

	for _, line := range lines {
		if strings.Contains(line, "won't display this because the service") {
			t.Errorf("wrapping stopped at the apostrophe:\n%s", out)
		}
	}
}

package server

import (
	"strings"
	"testing"
)

// linesAround is what keeps route detection from scanning the whole document on
// every go-to-definition. Getting its bounds wrong is not visible as an error:
// a window one line short silently stops finding routes near the edges, which
// reads as the feature being unreliable rather than broken.
func TestLinesAround(t *testing.T) {
	content := "l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\n"

	cases := []struct {
		name      string
		line, n   int
		wantFirst string
		wantLast  string
		wantBase  int
	}{
		{"middle of the file", 5, 2, "l3", "l7", 3},
		{"clamped at the start", 1, 3, "l0", "l4", 0},
		{"clamped at the end", 8, 3, "l5", "l9", 5},
		{"window larger than the file", 4, 100, "l0", "l9", 0},
		{"single line window", 5, 0, "l5", "l5", 5},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			window, base := linesAround(content, c.line, c.n)
			if base != c.wantBase {
				t.Errorf("base = %d, want %d", base, c.wantBase)
			}

			lines := strings.Split(strings.TrimSuffix(window, "\n"), "\n")
			if lines[0] != c.wantFirst {
				t.Errorf("first line = %q, want %q", lines[0], c.wantFirst)
			}

			if last := lines[len(lines)-1]; last != c.wantLast {
				t.Errorf("last line = %q, want %q", last, c.wantLast)
			}

			// The cursor's line must be findable at its offset within the window,
			// which is the whole point of returning base.
			if got := lines[c.line-base]; got != content[c.line*3:c.line*3+2] {
				t.Errorf("line %d within window = %q, want %q", c.line, got, content[c.line*3:c.line*3+2])
			}
		})
	}
}

// A line past the end has no window rather than a wrong one.
func TestLinesAroundBeyondTheEnd(t *testing.T) {
	if w, _ := linesAround("a\nb\n", 99, 2); w != "" {
		t.Errorf("expected no window past the end, got %q", w)
	}
}

func TestLinesAroundEmptyContent(t *testing.T) {
	if w, base := linesAround("", 0, 5); w != "" || base != 0 {
		t.Errorf("expected empty window for empty content, got %q base=%d", w, base)
	}
}

// The window has to be big enough for the thing it was narrowed from: an
// attribute list that opens well above the cursor and carries the route further
// down. This pins the constant rather than leaving it a number someone trims.
func TestRouteWindowCoversAMultiLineAttributeList(t *testing.T) {
	if routeWindow < 20 {
		t.Errorf("routeWindow = %d, too small for a wrapped attribute list", routeWindow)
	}
}

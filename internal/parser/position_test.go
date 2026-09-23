package parser

import "testing"

// An LSP character offset counts UTF-16 code units. é is two bytes and one
// unit; 😀 is four bytes and two units. Counting bytes put every position
// after either of them early.
func TestPositionToOffsetCountsUTF16Units(t *testing.T) {
	content := "ab\nxé|y\n😀|z\nlast"

	cases := []struct {
		name       string
		line, char int
		want       int
	}{
		{"ascii", 0, 1, 1},
		{"start of line", 1, 0, 3},
		{"after a two-byte rune", 1, 2, 3 + len("xé")},
		{"after an astral rune", 2, 2, len("ab\nxé|y\n😀")},
		{"inside a surrogate pair moves past the rune", 2, 1, len("ab\nxé|y\n😀")},
		{"past the end of the line stops at it", 0, 99, 2},
		{"last line with no newline", 3, 4, len(content)},
		{"past the last line", 9, 0, len(content)},
	}

	for _, c := range cases {
		if got := PositionToOffset(content, c.line, c.char); got != c.want {
			t.Errorf("%s: PositionToOffset(%d, %d) = %d, want %d", c.name, c.line, c.char, got, c.want)
		}
	}
}

// The closing line of a function holds the edit only when the edit is before
// its closing brace, and that column is compared against the client's. With a
// non-ASCII character earlier on the line, a byte column for the brace put an
// edit just after it inside the function.
func TestAnEditAfterTheClosingBraceIsNotInTheFunction(t *testing.T) {
	src := "component {\n\tfunction f() {\n\t\tvar s = \"é\"; }\n}\n"
	pr := Parse("file:///brace.cfc", src)

	// Line 2 is `\t\tvar s = "é"; }`: the brace is at UTF-16 column 15, so an
	// insertion at column 16 is after it.
	if kind := pr.ApplyEdit(2, 16, 2, 16, " "); kind != EditGlobal {
		t.Errorf("edit after the closing brace classified as %v, want EditGlobal", kind)
	}
}

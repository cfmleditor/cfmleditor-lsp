package server

import (
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// applyEdits applies LSP text edits the way a client does: all coordinates
// refer to the original document, so they are applied back to front.
func applyEdits(t *testing.T, content string, edits []protocol.TextEdit) string {
	t.Helper()

	lines := strings.Split(content, "\n")

	ordered := append([]protocol.TextEdit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Range.Start.Line > ordered[j].Range.Start.Line
	})

	for _, e := range ordered {
		start, end := int(e.Range.Start.Line), int(e.Range.End.Line)
		if start > len(lines) || end > len(lines) || start > end {
			t.Fatalf("edit %+v is out of bounds for a %d-line document", e.Range, len(lines))
		}

		var repl []string
		if e.NewText != "" {
			repl = strings.Split(strings.TrimSuffix(e.NewText, "\n"), "\n")
		}

		lines = append(lines[:start], append(append([]string{}, repl...), lines[end:]...)...)
	}

	return strings.Join(lines, "\n")
}

// assertNoOverlap pins the protocol requirement the coalescing step exists for.
func assertNoOverlap(t *testing.T, edits []protocol.TextEdit) {
	t.Helper()

	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1].Range, edits[i].Range
		if cur.Start.Line < prev.End.Line || (cur.Start.Line == prev.End.Line && prev.End.Line == prev.Start.Line) {
			t.Errorf("edits %d and %d overlap or share a position: %+v then %+v", i-1, i, prev, cur)
		}
	}
}

const rangeSrc = `component {
function alpha() {
x = 1;
}
    function beta( a,b,c ) {
            y = 2;
    }
function gamma() {
z = 3;
}
}
`

func formatWhole(t *testing.T, content string) string {
	t.Helper()

	out, err := formatDocument(content, protocol.FormattingOptions{InsertSpaces: false, TabSize: 4}, resolvedFormatting())
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return out
}

// TestRangeFormattingWholeDocumentMatchesDocumentFormatting is the property the
// whole design rests on. Selecting everything and formatting the selection must
// give byte-for-byte what format-on-save gives, or the two commands disagree
// and repeated use of one undoes the other.
func TestRangeFormattingWholeDocumentMatchesDocumentFormatting(t *testing.T) {
	want := formatWhole(t, rangeSrc)

	lines := strings.Split(rangeSrc, "\n")
	edits := rangeEdits(rangeSrc, want, 0, len(lines)-1)

	assertNoOverlap(t, edits)

	if got := applyEdits(t, rangeSrc, edits); got != want {
		t.Errorf("applying whole-document range edits gave:\n%q\nwant:\n%q", got, want)
	}
}

// TestRangeFormattingLeavesLinesOutsideTheRangeAlone is the other half: what is
// not selected must come back byte-identical, including its indentation.
func TestRangeFormattingLeavesLinesOutsideTheRangeAlone(t *testing.T) {
	formatted := formatWhole(t, rangeSrc)

	// Lines 4-6 are the misindented `beta` function.
	edits := rangeEdits(rangeSrc, formatted, 4, 6)
	assertNoOverlap(t, edits)

	got := strings.Split(applyEdits(t, rangeSrc, edits), "\n")
	src := strings.Split(rangeSrc, "\n")

	// Everything before the selection is untouched and still at its original
	// index, because no edit may start before it.
	for i := range 4 {
		if got[i] != src[i] {
			t.Errorf("line %d changed outside the selection: %q -> %q", i, src[i], got[i])
		}
	}

	// `function gamma() {` and everything after it kept its original text —
	// found by value, since the selection changed how many lines precede it.
	for _, want := range src[7:] {
		if want == "" {
			continue
		}

		if !containsLine(got, want) {
			t.Errorf("line %q was reformatted despite being outside the selection", want)
		}
	}
}

// TestRangeFormattingFormatsTheSelectedLines: the point of the feature.
func TestRangeFormattingFormatsTheSelectedLines(t *testing.T) {
	formatted := formatWhole(t, rangeSrc)

	edits := rangeEdits(rangeSrc, formatted, 4, 6)
	if len(edits) == 0 {
		t.Fatal("no edits for a selection over a misindented function")
	}

	got := applyEdits(t, rangeSrc, edits)

	if containsLine(strings.Split(got, "\n"), "    function beta( a,b,c ) {") {
		t.Error("the selected line kept its original formatting")
	}

	if !containsLine(strings.Split(got, "\n"), "\t\ty = 2;") {
		t.Errorf("the selected body was not reindented; got:\n%s", got)
	}
}

// TestRangeFormattingOnCleanLinesIsANoOp: a selection over already-formatted
// lines returns nothing, even though the rest of the document is a mess.
func TestRangeFormattingOnCleanLinesIsANoOp(t *testing.T) {
	src := "component {\n\n\tfunction alpha() {\n\n\t\tx = 1;\n\n\t}\n\nfunction beta() {\ny = 2;\n}\n}\n"
	formatted := formatWhole(t, src)

	if edits := rangeEdits(src, formatted, 2, 6); len(edits) != 0 {
		t.Errorf("got %d edits for an already-formatted selection: %+v", len(edits), edits)
	}

	// The control: the messy part of the same document does produce edits, so
	// the assertion above is not passing because nothing ever produces edits.
	if edits := rangeEdits(src, formatted, 8, 10); len(edits) == 0 {
		t.Error("no edits for the misformatted part of the same document")
	}
}

// TestRangeFormattingIdenticalDocumentReturnsNoEdits covers the early exit.
func TestRangeFormattingIdenticalDocumentReturnsNoEdits(t *testing.T) {
	clean := formatWhole(t, rangeSrc)
	if edits := rangeEdits(clean, clean, 0, 99); len(edits) != 0 {
		t.Errorf("got %d edits for an unchanged document", len(edits))
	}
}

func TestLineSpan(t *testing.T) {
	cases := []struct {
		name                string
		r                   protocol.Range
		wantFirst, wantLast int
	}{
		{
			"a whole-line selection ends at the start of the next line",
			protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 6}},
			3, 5,
		},
		{
			"a selection ending mid-line takes that line whole",
			protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 6, Character: 4}},
			3, 6,
		},
		{
			"a selection starting mid-line takes that line whole",
			protocol.Range{Start: protocol.Position{Line: 3, Character: 7}, End: protocol.Position{Line: 4, Character: 2}},
			3, 4,
		},
		{
			"a caret with no selection is one line",
			protocol.Range{Start: protocol.Position{Line: 3, Character: 2}, End: protocol.Position{Line: 3, Character: 2}},
			3, 3,
		},
		{
			"one whole line selected",
			protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 4}},
			3, 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, last := lineSpan(tc.r)
			if first != tc.wantFirst || last != tc.wantLast {
				t.Errorf("lineSpan = (%d, %d), want (%d, %d)", first, last, tc.wantFirst, tc.wantLast)
			}
		})
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}

	return false
}

package parser

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

func applyEachEdit(content string, edits []Edit) string {
	for _, e := range edits {
		content = ApplyEdit(content, e.StartLine, e.StartChar, e.EndLine, e.EndChar, e.Text)
	}

	return content
}

// ApplyEdits must agree with ApplyEdit applied once per edit on anything an
// editor could send, including what a real editor never does: edits out of
// order, overlapping ones, ranges running backwards, characters past the end
// of a line and lines past the end of the document. The pieces include
// multi-byte and surrogate-pair runes, since positions count UTF-16 units.
func TestApplyEditsMatchesApplyEdit(t *testing.T) {
	pieces := []string{"a", "bc", " ", "\n", "\n\n", "é", "日本", "😀", "x😀y", "\t", ""}

	rng := rand.New(rand.NewPCG(1, 2))

	randText := func(n int) string {
		var b strings.Builder
		for range rng.IntN(n + 1) {
			b.WriteString(pieces[rng.IntN(len(pieces))])
		}

		return b.String()
	}

	for i := range 20000 {
		content := randText(30)
		lines := strings.Count(content, "\n") + 1

		var edits []Edit

		ordered := rng.IntN(2) == 0
		line, char := 0, 0

		for range 1 + rng.IntN(8) {
			var e Edit

			if ordered {
				line += rng.IntN(3)
				if rng.IntN(2) == 0 {
					char = 0
				}

				char += rng.IntN(4)
				e.StartLine, e.StartChar = line, char
				e.EndLine, e.EndChar = line+rng.IntN(2), rng.IntN(6)

				if posBefore(e.EndLine, e.EndChar, e.StartLine, e.StartChar) {
					e.EndLine, e.EndChar = e.StartLine, e.StartChar+rng.IntN(3)
				}
			} else {
				e.StartLine, e.StartChar = rng.IntN(lines+2), rng.IntN(8)
				e.EndLine, e.EndChar = rng.IntN(lines+2), rng.IntN(8)
			}

			e.Text = randText(4)
			edits = append(edits, e)

			line, char = e.StartLine, e.StartChar
		}

		want := applyEachEdit(content, edits)
		if got := ApplyEdits(content, edits); got != want {
			t.Fatalf("case %d: content %q, edits %+v\nwant %q\ngot  %q", i, content, edits, want, got)
		}
	}
}

// The shape of reverting a reformat: an edit on every line, in document
// order. Once per edit this was quadratic; streamed it is linear, so ten times
// the edits must cost nowhere near a hundred times as long.
func TestApplyEditsIsLinearInOrderedEdits(t *testing.T) {
	build := func(n int) (content, want string, edits []Edit) {
		var b, w strings.Builder
		for i := range n {
			fmt.Fprintf(&b, "var x%d = foo.bar(id=arguments.id);\n", i)
			fmt.Fprintf(&w, "    var x%d = foo.bar(id=arguments.id);\n", i)
		}

		edits = make([]Edit, n)
		for i := range edits {
			edits[i] = Edit{StartLine: i, EndLine: i, Text: "    "}
		}

		return b.String(), w.String(), edits
	}

	timeIt := func(n int) time.Duration {
		content, want, edits := build(n)

		start := time.Now()
		got := ApplyEdits(content, edits)
		elapsed := time.Since(start)

		if got != want {
			t.Fatalf("n=%d: wrong result", n)
		}

		return elapsed
	}

	small, large := timeIt(2000), timeIt(20000)

	// The results are checked either way; the timing is skipped under -short,
	// which is how CI runs the race detector, as TestParsePerformance is.
	if testing.Short() {
		return
	}

	if large > 30*small+10*time.Millisecond {
		t.Errorf("2,000 edits took %v and 20,000 took %v: not linear", small, large)
	}

	if large > 200*time.Millisecond {
		t.Errorf("20,000 ordered edits took %v", large)
	}
}

package server

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

// applyTextEdits applies edits whose ranges all refer to content as given, as
// a TextEdit[] response's do, checking they are in order and do not overlap.
func applyTextEdits(t *testing.T, content string, edits []protocol.TextEdit) string {
	t.Helper()

	type span struct {
		start, end int
		text       string
	}

	spans := make([]span, len(edits))

	lines := strings.Split(content, "\n")

	// PositionToOffset clamps a column past the end of its line, so a
	// position counted in bytes rather than UTF-16 units would slip through
	// it. A client need not be so forgiving.
	valid := func(p protocol.Position) bool {
		if int(p.Line) >= len(lines) {
			return p.Line == uint32(len(lines)) && p.Character == 0
		}

		return int(p.Character) <= len(utf16.Encode([]rune(lines[p.Line])))
	}

	for i, e := range edits {
		if !valid(e.Range.Start) || !valid(e.Range.End) {
			t.Fatalf("edit %d has a position past the end of its line: %+v", i, e.Range)
		}

		spans[i] = span{
			start: parser.PositionToOffset(content, int(e.Range.Start.Line), int(e.Range.Start.Character)),
			end:   parser.PositionToOffset(content, int(e.Range.End.Line), int(e.Range.End.Character)),
			text:  e.NewText,
		}

		if spans[i].end < spans[i].start || i > 0 && spans[i].start < spans[i-1].end {
			t.Fatalf("edit %d is out of order or overlaps: %+v", i, edits)
		}
	}

	var b strings.Builder

	prev := 0

	for _, s := range spans {
		b.WriteString(content[prev:s.start])
		b.WriteString(s.text)
		prev = s.end
	}

	b.WriteString(content[prev:])

	return b.String()
}

// Whatever the diff finds, applying its edits must give the formatted text
// exactly. The inputs are what a formatter does to a file and then some:
// lines reindented, split, joined, inserted, deleted and moved, CRLF sources,
// no final newline, blank and repeated lines (which cannot anchor), and
// non-ASCII text on the last line, where the end position counts UTF-16 units.
func TestLineEditsReproduceTheTarget(t *testing.T) {
	pool := []string{"{", "}", "", "x = 1;", "return y;", "if (a) {", "foo.bar( id = 1 );", "<cfset é = 1>", "😀 = 2;", "\tq();"}

	rng := rand.New(rand.NewPCG(3, 4))

	for i := range 20000 {
		n := rng.IntN(25)
		lines := make([]string, n)

		for j := range lines {
			lines[j] = pool[rng.IntN(len(pool))]
			if rng.IntN(4) == 0 {
				lines[j] += " // " + string(rune('a'+j))
			}
		}

		nl := "\n"
		if rng.IntN(6) == 0 {
			nl = "\r\n"
		}

		before := strings.Join(lines, nl)
		if rng.IntN(3) > 0 && n > 0 {
			before += nl
		}

		var out []string

		for _, l := range lines {
			switch rng.IntN(8) {
			case 0: // deleted
			case 1:
				out = append(out, "    "+strings.TrimSpace(l))
			case 2:
				out = append(out, l, pool[rng.IntN(len(pool))])
			case 3:
				if len(out) > 0 {
					out[len(out)-1] += " " + l
				} else {
					out = append(out, l)
				}
			default:
				out = append(out, l)
			}
		}

		if rng.IntN(5) == 0 && len(out) > 2 {
			j, k := rng.IntN(len(out)), rng.IntN(len(out))
			out[j], out[k] = out[k], out[j]
		}

		after := strings.Join(out, "\n")
		if rng.IntN(3) > 0 && len(out) > 0 {
			after += "\n"
		}

		edits := lineEdits(before, after)
		if got := applyTextEdits(t, before, edits); got != after {
			t.Fatalf("case %d:\nbefore %q\nafter  %q\nedits  %+v\ngot    %q", i, before, after, edits, got)
		}

		if before == after && len(edits) != 0 {
			t.Fatalf("case %d: %d edits for identical text", i, len(edits))
		}
	}
}

// The point of it: a reformat that touches a few lines of a long file sends
// those lines, not the file.
func TestLineEditsSendOnlyTheChangedLines(t *testing.T) {
	var before, after strings.Builder

	for i := range 5000 {
		line := "var x" + strings.Repeat("y", i%7) + " = f(" + string(rune('a'+i%26)) + ");\n"
		line = strings.Replace(line, "x", "x"+itoa(i), 1)
		before.WriteString(line)

		if i%1000 == 0 {
			after.WriteString("    " + line)
		} else {
			after.WriteString(line)
		}
	}

	edits := lineEdits(before.String(), after.String())
	if len(edits) != 5 { // lines 0, 1000, 2000, 3000, 4000: none adjacent
		t.Fatalf("want 5 one-line edits, got %d", len(edits))
	}

	for _, e := range edits {
		if e.Range.End.Line != e.Range.Start.Line+1 || !strings.HasPrefix(e.NewText, "    var x") {
			t.Errorf("edit is not the one changed line: %+v", e)
		}
	}
}

// A reindented line is still that line, so it anchors the diff: reindenting
// one function leaves the unchanged lines between changed ones out of the
// edits, where matching raw lines would find no anchor and send the whole
// stretch back. A run of changed lines is one edit.
func TestLineEditsReindentLineByLine(t *testing.T) {
	before := "component {\nfunction a() {\nx = 1;\n}\n\tkeep = 1;\nfunction b() {\ny = 2;\n}\n}\n"
	after := "component {\n    function a() {\n        x = 1;\n    }\n\tkeep = 1;\n    function b() {\n        y = 2;\n    }\n}\n"

	edits := lineEdits(before, after)
	if len(edits) != 2 {
		t.Fatalf("want 2 edits, one per function, got %+v", edits)
	}

	for i, want := range [][2]uint32{{1, 4}, {5, 8}} {
		if r := edits[i].Range; r.Start.Line != want[0] || r.End.Line != want[1] {
			t.Errorf("edit %d spans lines %d-%d, want %d-%d", i, r.Start.Line, r.End.Line, want[0], want[1])
		}
	}
}

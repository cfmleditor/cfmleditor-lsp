package parser

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// walkLineOffsets is the walk lineOffsets replaced, kept as the reference its
// table must agree with.
func walkLineOffsets(content string, startLine, endLine int) (start, end int) {
	line := 0

	for line < startLine {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return -1, -1
		}

		start += idx + 1
		line++
	}

	end = start
	for line <= endLine {
		idx := strings.IndexByte(content[end:], '\n')
		if idx < 0 {
			end = len(content)

			break
		}

		end += idx + 1
		line++
	}

	return start, end
}

func TestLineOffsetsMatchesTheWalk(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	contents := []string{"", "\n", "a", "a\n", "\n\n", "a\nb", "a\nb\n", "a\r\nb\r\n\r\nc"}

	for range 200 {
		var b strings.Builder

		for range r.IntN(40) {
			if r.IntN(3) == 0 {
				b.WriteByte('\n')
			} else {
				b.WriteByte('x')
			}
		}

		contents = append(contents, b.String())
	}

	for _, content := range contents {
		pr := &ParseResult{Content: content}
		lines := strings.Count(content, "\n")

		for start := -1; start <= lines+2; start++ {
			for end := start - 2; end <= lines+2; end++ {
				ws, we := walkLineOffsets(content, start, end)
				gs, ge := pr.lineOffsets(start, end)

				if ws != gs || we != ge {
					t.Fatalf("content %q lines %d-%d: walk (%d,%d), table (%d,%d)", content, start, end, ws, we, gs, ge)
				}
			}
		}
	}
}

func TestLineOffsetsFollowsAnEditedContent(t *testing.T) {
	pr := &ParseResult{Content: "a\nb\nc\n"}

	if s, e := pr.lineOffsets(1, 1); s != 2 || e != 4 {
		t.Fatalf("before the edit: (%d,%d), want (2,4)", s, e)
	}

	pr.Content = "aaaa\nb\nc\n"

	if s, e := pr.lineOffsets(1, 1); s != 5 || e != 7 {
		t.Fatalf("after the edit: (%d,%d), want (5,7)", s, e)
	}
}

func TestLineAtCountsTheNewlinesBefore(t *testing.T) {
	for _, content := range []string{"", "a", "\n", "a\nb\n\nc", "\n\n\nx\n"} {
		pr := &ParseResult{Content: content}

		for off := 0; off <= len(content); off++ {
			if got, want := pr.lineAt(off), strings.Count(content[:off], "\n"); got != want {
				t.Fatalf("content %q offset %d: line %d, want %d", content, off, got, want)
			}
		}
	}
}

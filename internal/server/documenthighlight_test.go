package server

import (
	"fmt"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func highlightSet(hs []protocol.DocumentHighlight) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, fmt.Sprintf("%d:%d-%d", h.Range.Start.Line, h.Range.Start.Character, h.Range.End.Character))
	}

	return out
}

func assertHighlights(t *testing.T, src, word string, want []string) {
	t.Helper()

	got := highlightSet(highlightsOf(src, word))
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("highlights of %q:\n got %v\nwant %v", word, got, want)
	}
}

// TestHighlightsAreWholeIdentifiers is the defect a naive strings.Index has:
// `user` lighting up inside `username` and `getUser`.
func TestHighlightsAreWholeIdentifiers(t *testing.T) {
	src := "user = 1;\nusername = 2;\ngetUser();\nwriteOutput( user );\n"

	assertHighlights(t, src, "user", []string{"0:0-4", "3:13-17"})
}

// TestHighlightsAreCaseInsensitive: CFML is, so USER, User and user are one
// identifier and shading only the spelling under the cursor would be wrong.
func TestHighlightsAreCaseInsensitive(t *testing.T) {
	src := "USER = 1;\nUser = 2;\nuser = 3;\n"

	assertHighlights(t, src, "user", []string{"0:0-4", "1:0-4", "2:0-4"})
}

// TestHighlightsFindEveryOccurrenceOnALine. The scan resumes after each match
// rather than restarting the line, so several on one line must all be reported
// — the obvious off-by-one here loses every occurrence after the first.
func TestHighlightsFindEveryOccurrenceOnALine(t *testing.T) {
	src := "total = total + total;\n"

	assertHighlights(t, src, "total", []string{"0:0-5", "0:8-13", "0:16-21"})
}

// TestHighlightColumnsCountUTF16: an LSP character offset is UTF-16 code units,
// not bytes, so a non-ASCII character earlier on the line must not shift the
// highlight off the identifier.
func TestHighlightColumnsCountUTF16(t *testing.T) {
	// "é" is two bytes and one UTF-16 unit; the emoji is four bytes and two.
	src := "// é 🎉\nx = 1; // é x\n"

	assertHighlights(t, src, "x", []string{"1:0-1", "1:12-13"})
}

// TestHighlightsSpanAdjacentIdentifierCharacters: "_" and "$" are identifier
// characters to the parser's scanner, so a match butting against one is part of
// a longer word and not a match at all.
func TestHighlightsSpanAdjacentIdentifierCharacters(t *testing.T) {
	src := "id = 1;\n_id = 2;\nid$ = 3;\nfoo.id = 4;\n"

	assertHighlights(t, src, "id", []string{"0:0-2", "3:4-6"})
}

// TestHighlightsHandleCRLF pins a real input shape rather than a fix: CRLF
// needs no special handling here (see the note in highlightsOf), and this is
// the test that says so and would notice if that stopped being true.
func TestHighlightsHandleCRLF(t *testing.T) {
	src := "x = 1;\r\nwriteOutput( x );\r\n"

	assertHighlights(t, src, "x", []string{"0:0-1", "1:13-14"})
}

// TestHighlightsOfNothing: an empty word must not match at every position.
func TestHighlightsOfNothing(t *testing.T) {
	assertHighlights(t, "x = 1;\n", "", nil)
}

// TestHighlightLineNumbersSurviveTheWalk covers the line shapes that a manual
// walk gets wrong where strings.Split does not: a file with no trailing
// newline, consecutive newlines (an empty line must still consume a line
// number), a match on the very last line, and a file that is nothing but
// newlines.
func TestHighlightLineNumbersSurviveTheWalk(t *testing.T) {
	// No trailing newline, match on the final line.
	assertHighlights(t, "a = 1;\nuser = 2;", "user", []string{"1:0-4"})

	// Blank lines still advance the line counter.
	assertHighlights(t, "a = 1;\n\n\nuser = 2;\n", "user", []string{"3:0-4"})

	// Leading blank line.
	assertHighlights(t, "\nuser = 2;\n", "user", []string{"1:0-4"})

	// Trailing newline does not invent a match or a line.
	assertHighlights(t, "user = 1;\n", "user", []string{"0:0-4"})

	// Nothing but separators.
	assertHighlights(t, "\n\n\n", "user", nil)
}

// TestHighlightsCostTheAnswerNotTheFile pins the shape rather than a byte
// count: the same single match in a file a hundred times longer must not
// allocate a hundred times more memory. strings.Split did exactly that -- a
// string header per line, 16 bytes each, on a request the editor sends on every
// cursor move -- so this fails if it comes back.
//
// It has to weigh *bytes*, not allocations. Split takes one slice however long
// the file is, so an AllocsPerRun comparison passes just as happily with Split
// as without it; the first version of this test did, and caught nothing.
func TestHighlightsCostTheAnswerNotTheFile(t *testing.T) {
	const match = "user = 1;\n"

	bytesPerOp := func(src string) int64 {
		r := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if got := highlightsOf(src, "user"); len(got) != 1 {
					b.Fatalf("expected exactly 1 highlight, got %d", len(got))
				}
			}
		})

		return r.AllocedBytesPerOp()
	}

	small := bytesPerOp(match + strings.Repeat("a = 2;\n", 100))
	large := bytesPerOp(match + strings.Repeat("a = 2;\n", 10000))

	// The answer is one highlight either way, so the larger file may cost no
	// more than a slack factor over the smaller. Split makes it ~100x.
	if large > small*2 {
		t.Errorf("memory scales with the file, not the answer: %d B/op for 101 lines, %d B/op for 10001 lines",
			small, large)
	}
}

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

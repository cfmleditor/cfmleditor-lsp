package server

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// The three `>` handlers each read one line of the document. They used to reach
// it by splitting the whole document, which costs a string header per line on a
// keystroke path — 516KB on a 32,000-line component, three times over for one
// '>'.
//
// The swap is not obviously equivalent, which is why these exist: the split
// handed back a line carrying its newline, so it was one byte longer than the
// line a Position describes, and every length guard in these functions was
// written against that longer string. These check the new spelling against the
// old one over every position of every line.

func splitLineAt(content string, line int) (string, bool) {
	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) {
		return "", false
	}

	return lines[line], true
}

// oldDuplicateGt is duplicateGtCompletion as it was written.
func oldDuplicateGt(content string, line, char int) bool {
	lineText, ok := splitLineAt(content, line)
	if !ok || char < 2 {
		return false
	}

	if char > len(lineText) || lineText[char-2] != '>' {
		return false
	}

	before := lineText[:char-1]

	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return false
	}

	return !strings.ContainsRune(before[openIdx:len(before)-1], '>')
}

// oldCloseTag is closeTagCompletion's accept/reject decision as it was written,
// transcribed from the original, checks in the original order.
func oldCloseTag(content string, line, char int) bool {
	lineText, ok := splitLineAt(content, line)
	if !ok || char >= len(lineText) {
		return false
	}

	rest := lineText[char:]

	idx := strings.IndexByte(rest, '>')
	if idx == -1 {
		return false
	}

	middle := rest[:idx]
	if strings.TrimSpace(middle) == "" {
		return false
	}

	if strings.ContainsRune(middle, '<') {
		return false
	}

	before := lineText[:char]

	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return false
	}

	return !strings.ContainsRune(lineText[openIdx:char-1], '>')
}

var closeTagDocs = []string{
	"<cfoutput><cfif x EQ 1>>\n</cfif></cfoutput>\n",
	"<cfset x = 1>>\n",
	"<cfset x = 1>\n<cfset y = 2>>\n<cfset z = 3>\n",
	"no tags here at all\n",
	"",
	"\n",
	"\n\n\n",
	"<cfif a><cfif b>>\n",
	"<cfset s = \"a > b\">>\n",
	"trailing line with no newline <cfset q = 1>>",
	"<cfoutput>#x#</cfoutput>>\n\n",
	"\t<cfset indented = 1>>\n",
	"<a title=\"x > y\">>\n",
	">>\n",
	"<>\n",
}

// TestOneLineReadAgreesWithTheSplit walks every line and every character
// position of each document, including positions past the end of a line, and
// requires the two spellings to make the same decision.
func TestOneLineReadAgreesWithTheSplit(t *testing.T) {
	for i, doc := range closeTagDocs {
		t.Run(fmt.Sprintf("doc%d", i), func(t *testing.T) {
			lineCount := strings.Count(doc, "\n") + 1

			for line := range lineCount + 2 {
				// Positions on the line, and one past its end. Not further:
				// see TestAPositionPastTheEndOfALineIsRejected.
				for char := range len(lineAt(doc, line)) + 1 {
					if got, want := acceptDuplicateGt(doc, line, char), oldDuplicateGt(doc, line, char); got != want {
						t.Errorf("duplicateGt(line %d, char %d) = %v, want %v (doc %q)", line, char, got, want, doc)
					}

					if got, want := acceptCloseTag(doc, line, char), oldCloseTag(doc, line, char); got != want {
						t.Errorf("closeTag(line %d, char %d) = %v, want %v (doc %q)", line, char, got, want, doc)
					}
				}
			}
		})
	}
}

// lineAt is the line a Position describes: without its terminator, which is
// what makes the character bound below the real one.
func lineAt(content string, line int) string {
	text, _ := splitLineAt(content, line)

	return strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
}

func acceptDuplicateGt(content string, line, char int) bool {
	_, ok := duplicateGtCompletion(content, line, char)

	return ok
}

func acceptCloseTag(content string, line, char int) bool {
	_, ok := closeTagCompletion(content, line, char)

	return ok
}

// The same property against documents no hand-written case would produce.
func TestOneLineReadAgreesWithTheSplitOnRandomInput(t *testing.T) {
	const alphabet = "<>abc \t\n\"="

	rnd := rand.New(rand.NewSource(7))

	for range 400 {
		var b strings.Builder

		for range rnd.Intn(60) {
			b.WriteByte(alphabet[rnd.Intn(len(alphabet))])
		}

		doc := b.String()

		for line := range strings.Count(doc, "\n") + 2 {
			for char := range len(lineAt(doc, line)) + 1 {
				if got, want := acceptDuplicateGt(doc, line, char), oldDuplicateGt(doc, line, char); got != want {
					t.Fatalf("duplicateGt(line %d, char %d) = %v, want %v (doc %q)", line, char, got, want, doc)
				}

				if got, want := acceptCloseTag(doc, line, char), oldCloseTag(doc, line, char); got != want {
					t.Fatalf("closeTag(line %d, char %d) = %v, want %v (doc %q)", line, char, got, want, doc)
				}
			}
		}
	}
}

// TestAPositionPastTheEndOfALineIsRejected records the one deliberate
// difference the change above makes.
//
// The split handed back a line carrying its newline, so every length guard in
// these three functions was written against a string one byte longer than the
// line a Position describes — and a character one past the end of the line
// therefore passed the guard and read the byte before the newline as if it were
// the byte before the cursor. An editor does not send such a position: the
// character offset of a cursor at the end of a line is the line's length, not
// one more. Reading the line without its terminator rejects it, which is what
// it should always have done.
func TestAPositionPastTheEndOfALineIsRejected(t *testing.T) {
	// "<cfset x = 1>" is 13 characters, so 13 is the end of the line and 14 is
	// past it. At 14 the old code read index 12 — the '>' — and offered to
	// remove a duplicate that the cursor was nowhere near.
	const doc = "<cfset x = 1>\n<cfset y = 2>\n"

	if _, ok := duplicateGtCompletion(doc, 0, 14); ok {
		t.Error("duplicateGtCompletion accepted a character position past the end of its line")
	}

	if got := onTypeEdits(doc, 0, 14); len(got) != 0 {
		t.Errorf("onTypeEdits returned %d edits for a position past the end of its line", len(got))
	}

	// The last position actually on the line still behaves.
	if _, ok := duplicateGtCompletion(doc, 0, 13); ok {
		t.Error("duplicateGtCompletion accepted a line whose cursor follows a single '>'")
	}
}

// TestCloseTagHandlersDoNotScaleWithDocumentSize is the shape guard. Each
// decision is about one line and must cost what that line costs, not what the
// document does. Read through a split, this grew straight through the document:
// 8KB at 500 lines, 516KB at 32,000, three times over for a single '>'.
//
// Each case is measured at a position where its function *accepts*, so the
// whole path is walked rather than an early return; the positions were found by
// enumeration, not by reading the code.
//
// It measures allocated *bytes*, not the allocation count. Splitting a document
// is a single allocation however long the document is — the count is flat and
// the bytes are not — so a count-based version of this test passed with the
// split restored and pinned nothing.
func TestCloseTagHandlersDoNotScaleWithDocumentSize(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		char   int
		accept func(content string, line, char int) bool
	}{
		{"duplicateGtCompletion", "<cfset x = 1>>", 14, acceptDuplicateGt},
		{"closeTagCompletion", "<cfset x = 1>>", 5, acceptCloseTag},
		{"onTypeEdits", "<cfset x = 1>  >", 13, func(c string, l, ch int) bool {
			return len(onTypeEdits(c, l, ch)) > 0
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			build := func(lines int) (string, int) {
				var b strings.Builder

				b.WriteString("<cfcomponent>\n")

				for range lines {
					b.WriteString("\t<cfset filler = \"some value here\" />\n")
				}

				b.WriteString(tc.line + "\n</cfcomponent>\n")

				return b.String(), lines + 1
			}

			measure := func(lines int) float64 {
				content, line := build(lines)

				if !tc.accept(content, line, tc.char) {
					t.Fatalf("fixture does not reach the accept path at %d lines", lines)
				}

				res := testing.Benchmark(func(b *testing.B) {
					for b.Loop() {
						tc.accept(content, line, tc.char)
					}
				})

				return float64(res.AllocedBytesPerOp())
			}

			small := measure(500)
			large := measure(16000)

			t.Logf("B/op: 500 lines %.0f, 16000 lines %.0f", small, large)

			// 32x the document. Bytes held per line follow it; one line does not.
			if large > small+64 {
				t.Errorf("%s allocates %.0f bytes at 500 lines and %.0f at 16,000 (32x the document) "+
					"— the whole document is being split", tc.name, small, large)
			}
		})
	}
}

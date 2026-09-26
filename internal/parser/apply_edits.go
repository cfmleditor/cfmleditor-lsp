package parser

import (
	"strings"
	"unicode/utf8"
)

// Edit is one LSP content change: a range in the document as it stands after
// every edit before it, and the text that replaces it.
type Edit struct {
	StartLine, StartChar int
	EndLine, EndChar     int
	Text                 string
}

// ApplyEdits applies edits in order, each against the result of the ones
// before it, as didChange's contentChanges are. The result is exactly what
// calling ApplyEdit once per edit gives.
//
// Calling ApplyEdit once per edit costs the whole document per edit: each one
// walks from the top to find its position and then copies the string. That is
// fine for a keystroke and quadratic for a batch. Reverting a reformat in Zed
// sends one didChange holding an edit for every line the formatter touched,
// and 6,000 of them against a 110 KB file took 425ms, holding the document's
// lock throughout.
//
// Edits in document order, which is how an editor reports a batch, are
// streamed instead: the text before each one is copied once and the walk to
// its position starts where the last edit ended, so the batch costs the
// document once. An edit that starts before the previous one ended, or whose
// range runs backwards, falls back to ApplyEdit on the text so far.
func ApplyEdits(content string, edits []Edit) string {
	if len(edits) == 1 {
		e := edits[0]

		return ApplyEdit(content, e.StartLine, e.StartChar, e.EndLine, e.EndChar, e.Text)
	}

	grow := len(content)
	for _, e := range edits {
		grow += len(e.Text)
	}

	var out strings.Builder

	out.Grow(grow)

	// The document is out followed by rest. (line, col) is where rest starts,
	// the position actually reached rather than the one asked for, since a
	// position past the end of its line stops at the line's end.
	rest := content
	line, col := 0, 0

	for _, e := range edits {
		if posBefore(e.StartLine, e.StartChar, line, col) || posBefore(e.EndLine, e.EndChar, e.StartLine, e.StartChar) {
			doc := ApplyEdit(out.String()+rest, e.StartLine, e.StartChar, e.EndLine, e.EndChar, e.Text)

			// Start again from the top of the new text. The walk to the next
			// edit then costs the document once, as the fallback just did.
			out.Reset()
			out.Grow(grow)

			rest, line, col = doc, 0, 0

			continue
		}

		start, sLine, sCol := walkTo(rest, line, col, e.StartLine, e.StartChar)
		n, _, _ := walkTo(rest[start:], sLine, sCol, e.EndLine, e.EndChar)

		out.WriteString(rest[:start])
		out.WriteString(e.Text)

		rest = rest[start+n:]

		if nl := strings.LastIndexByte(e.Text, '\n'); nl >= 0 {
			line, col = sLine+strings.Count(e.Text, "\n"), utf16Len(e.Text[nl+1:])
		} else {
			line, col = sLine, sCol+utf16Len(e.Text)
		}
	}

	out.WriteString(rest)

	return out.String()
}

// walkTo finds the LSP position (toLine, toChar) in s, which starts at
// position (line, col) of the document, the same way PositionToOffset finds
// it from the top: a line past the end of the document is its end, and a
// character past the end of its line is the line's end. It returns the byte
// offset into s and the position actually reached.
func walkTo(s string, line, col, toLine, toChar int) (offset, atLine, atCol int) {
	for line < toLine {
		idx := strings.IndexByte(s[offset:], '\n')
		if idx < 0 {
			return len(s), line, col + utf16Len(s[offset:])
		}

		offset += idx + 1
		line++
		col = 0
	}

	for col < toChar && offset < len(s) {
		if b := s[offset]; b < utf8.RuneSelf {
			if b == '\n' {
				break
			}

			col++
			offset++

			continue
		}

		r, size := utf8.DecodeRuneInString(s[offset:])
		col += utf16Units(r)
		offset += size
	}

	return offset, line, col
}

func posBefore(aLine, aChar, bLine, bChar int) bool {
	return aLine < bLine || aLine == bLine && aChar < bChar
}

package server

import (
	"strings"
	"unicode/utf8"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// LSP positions count UTF-16 code units; everything in this server that reads
// a line works in bytes. The two agree on ASCII and nowhere else: é is two
// bytes and one unit, 😀 four bytes and two. Handlers took the client's column
// as a byte offset and sent byte offsets back as columns, so on a line with a
// non-ASCII character before the cursor hover named the wrong word and a
// completion's edit replaced the wrong span.
//
// The rule is to convert at the edge and nowhere else: a column coming in goes
// through byteCol before anything reads the line, and a column going out goes
// through utf16Len or a colMapper. The parser helpers in between keep working
// in bytes.

// byteCol converts an LSP character on line to a byte column in that line.
func byteCol(content string, line int, char uint32) int {
	return lineByteCol(parser.LineTextAt(content, line), char)
}

// lineByteCol converts an LSP character to a byte column in text, one line.
//
// A character past the end of the line keeps its excess, one byte per unit,
// rather than being clamped. Several handlers reject a position past the end
// of its line, and clamping would turn that position into a valid one at the
// line's end.
func lineByteCol(text string, char uint32) int {
	want := int(char)
	units := 0

	for i := 0; i < len(text); {
		if units >= want {
			return i
		}

		if b := text[i]; b < utf8.RuneSelf {
			units++
			i++

			continue
		}

		r, size := utf8.DecodeRuneInString(text[i:])
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}

		i += size
	}

	return len(text) + max(want-units, 0)
}

// lineCol converts a byte column in text, one line, to an LSP character. The
// inverse of lineByteCol, excess included.
func lineCol(text string, col int) uint32 {
	if col <= len(text) {
		return utf16Len(text[:col])
	}

	return utf16Len(text) + uint32(col-len(text))
}

// colMapper converts byte columns to LSP characters for many positions in one
// document, for the responses that carry one per link or per diagnostic.
//
// Finding each line afresh would make those quadratic in the document, so line
// starts are recorded as they are found and reused; and they are found only as
// far as the positions asked about reach, with strings.IndexByte, rather than
// by a pass over every byte up front. That pass — to learn whether the
// document was pure ASCII and so needed no conversion at all — doubled the
// cost of a documentLink request. Only the part of a line before the column is
// ever examined for non-ASCII bytes, and on an ASCII prefix the byte column is
// already the answer.
type colMapper struct {
	content string
	starts  []int // start offset of each line found so far; starts[0] == 0
}

func newColMapper(content string) *colMapper {
	return &colMapper{content: content, starts: []int{0}}
}

// line returns the text of line, or false when the document has fewer lines.
func (m *colMapper) line(line uint32) (string, bool) {
	for int(line)+1 >= len(m.starts) {
		last := m.starts[len(m.starts)-1]
		if last > len(m.content) {
			break
		}

		nl := strings.IndexByte(m.content[last:], '\n')
		if nl < 0 {
			// The last line: record a start one past the end, so its end is
			// known and the loop cannot look for another.
			m.starts = append(m.starts, len(m.content)+1)

			break
		}

		m.starts = append(m.starts, last+nl+1)
	}

	if int(line)+1 >= len(m.starts) {
		return "", false
	}

	return m.content[m.starts[line] : m.starts[line+1]-1], true
}

// col returns the LSP character for byte column col on line.
func (m *colMapper) col(line uint32, col uint32) uint32 {
	text, ok := m.line(line)
	if !ok {
		return col
	}

	prefix := text[:min(int(col), len(text))]

	for i := range len(prefix) {
		if prefix[i] >= utf8.RuneSelf {
			return lineCol(text, int(col))
		}
	}

	return col
}

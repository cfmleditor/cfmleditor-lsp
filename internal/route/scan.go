package route

import (
	"strings"
	"unicode"
)

// Ref is one route string found in source, with enough position for an editor to
// put a link on it.
type Ref struct {
	Attribute string // the attribute it was written in, lower-cased
	Value     string // the route, as written
	Line      uint32 // 0-based, as the parser numbers lines
	Start     uint32 // byte offset of the value's first character, from the file start
	End       uint32 // byte offset one past its last character
	Col       uint32 // 0-based byte column of the value's first character
}

// Scan finds every route-carrying attribute in content.
//
// A byte scan rather than a tree-sitter walk on purpose. These attributes live in
// HTML that is interleaved with CFML tags, inside CFML strings, and inside
// JavaScript that builds markup — places where the CST either has no node for the
// attribute or has the whole run as one opaque token. A scan finds them all, and
// the cost of a wrong one is an unresolvable route that produces no edge, so
// over-reaching here is cheap and under-reaching is not.
//
// Only double- and single-quoted literal values are returned. A value built at
// runtime cannot be resolved, so recording its position would put a link on text
// that leads nowhere.
func Scan(content string, attributes []string) []Ref {
	if len(attributes) == 0 || content == "" {
		return nil
	}

	var refs []Ref

	line := uint32(0)
	lineStart := 0

	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1

			continue
		}

		if content[i] != '-' && content[i] != '_' && !isAttrStart(content[i]) {
			continue
		}

		attr, valStart, valEnd, next := matchAttribute(content, i, attributes)
		if attr == "" {
			continue
		}

		value := content[valStart:valEnd]
		if value != "" {
			refs = append(refs, Ref{
				Attribute: attr,
				Value:     value,
				Line:      line,
				Start:     uint32(valStart),
				End:       uint32(valEnd),
				Col:       uint32(valStart - lineStart),
			})
		}

		// Step to the end of the value, counting any newlines inside it.
		for ; i < next && i < len(content); i++ {
			if content[i] == '\n' {
				line++
				lineStart = i + 1
			}
		}

		i--
	}

	return refs
}

func isAttrStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// matchAttribute tries to read `name="value"` starting at i. It returns the
// matched attribute name lower-cased, the bounds of the value, and the offset just
// past the closing quote.
func matchAttribute(content string, i int, attributes []string) (string, int, int, int) {
	// The character before must not be part of a longer identifier, or
	// "x-data-view" would match "data-view".
	if i > 0 && isNamePart(content[i-1]) {
		return "", 0, 0, 0
	}

	for _, attr := range attributes {
		if len(content)-i < len(attr)+3 {
			continue
		}

		if !strings.EqualFold(content[i:i+len(attr)], attr) {
			continue
		}

		j := i + len(attr)
		for j < len(content) && isSpace(content[j]) {
			j++
		}

		if j >= len(content) || content[j] != '=' {
			continue
		}

		j++
		for j < len(content) && isSpace(content[j]) {
			j++
		}

		if j >= len(content) || (content[j] != '"' && content[j] != '\'') {
			continue
		}

		quote := content[j]
		j++
		start := j

		for j < len(content) && content[j] != quote && content[j] != '\n' {
			j++
		}

		if j >= len(content) || content[j] != quote {
			continue
		}

		return attr, start, j, j + 1
	}

	return "", 0, 0, 0
}

func isNamePart(c byte) bool {
	return c == '-' || c == '_' || c == ':' ||
		unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c))
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

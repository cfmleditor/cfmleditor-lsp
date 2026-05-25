package parser

import "strings"

// FindCallContext finds the function name being called at the cursor position
// and which parameter the cursor is on (0-based). Also returns the full qualifier.
func FindCallContext(content string, line, char int) (funcName string, qualifier string, activeParam int) {
	lineText := LineTextAt(content, line)
	if lineText == "" {
		return "", "", 0
	}

	pos := min(char, len(lineText))

	// Walk backwards from cursor to find the opening paren
	depth := 0
	commas := 0

	i := pos - 1
	for i >= 0 {
		ch := lineText[i]
		switch ch {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				// Found the opening paren — extract function name before it
				end := i

				start := end - 1
				for start >= 0 && (isIdentPart(lineText[start]) || lineText[start] == '.') {
					start--
				}

				start++

				name := lineText[start:end]
				if dotIdx := strings.LastIndexByte(name, '.'); dotIdx >= 0 {
					qual := name[:dotIdx]
					funcN := name[dotIdx+1:]
					// If qualifier is empty, check for call expression before the dot
					if qual == "" && start > 0 && lineText[start-1] == ')' {
						j := start - 1
						parenDepth := 0

						for j >= 0 {
							switch lineText[j] {
							case ')':
								parenDepth++
							case '(':
								parenDepth--
								if parenDepth == 0 {
									fnStart := j - 1
									for fnStart >= 0 && isIdentPart(lineText[fnStart]) {
										fnStart--
									}

									fnStart++

									return funcN, lineText[fnStart:start], commas
								}
							}

							j--
						}
					}

					return funcN, qual, commas
				}

				return name, "", commas
			}

			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}

		i--
	}

	return "", "", 0
}

// LineTextAt returns the text of the given 0-based line.
func LineTextAt(content string, line int) string {
	offset := 0
	for l := 0; l < line; l++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return ""
		}

		offset += idx + 1
	}

	end := strings.IndexByte(content[offset:], '\n')
	if end < 0 {
		return content[offset:]
	}

	return content[offset : offset+end]
}

// IsWordChar reports whether b is a valid identifier character.
func IsWordChar(b byte) bool {
	return isIdentPart(b)
}

// WordAtPosition returns the word at the given line/char position.
func WordAtPosition(content string, line, char int) string {
	lineText := LineTextAt(content, line)
	if lineText == "" && line > 0 {
		return ""
	}

	char = min(char, len(lineText))

	start := char
	for start > 0 && IsWordChar(lineText[start-1]) {
		start--
	}

	end := char
	for end < len(lineText) && IsWordChar(lineText[end]) {
		end++
	}

	if start == end {
		return ""
	}

	return lineText[start:end]
}

// QualifierBeforeWord returns the identifier before the dot preceding the word at cursor.
// Returns "~" prefix for createObject/new patterns, "~?" for call expressions.
func QualifierBeforeWord(content string, line, char int) string {
	lineText := LineTextAt(content, line)
	if lineText == "" {
		return ""
	}

	start := min(char, len(lineText))
	for start > 0 && IsWordChar(lineText[start-1]) {
		start--
	}

	if start < 1 || lineText[start-1] != '.' {
		return ""
	}

	dotPos := start - 1
	if dotPos > 0 && (lineText[dotPos-1] == ')' || lineText[dotPos-1] == ']') {
		if lineText[dotPos-1] == ')' {
			prefix := lineText[:dotPos]
			lowerPrefix := strings.ToLower(prefix)

			if idx := strings.LastIndex(lowerPrefix, "createobject("); idx >= 0 {
				args := prefix[idx+13:]
				args = strings.TrimSuffix(args, ")")

				parts := strings.SplitN(args, ",", 2)
				if len(parts) == 2 {
					comp := strings.TrimSpace(parts[1])
					comp = strings.Trim(comp, "\"'")

					if comp != "" {
						return "~" + comp
					}
				}
			}

			if idx := strings.LastIndex(lowerPrefix, "new "); idx >= 0 {
				rest := strings.TrimSpace(prefix[idx+4:])
				if parenIdx := strings.IndexByte(rest, '('); parenIdx >= 0 {
					rest = rest[:parenIdx]
				}

				rest = strings.Trim(rest, "\"'")
				if rest != "" {
					return "~" + rest
				}
			}

			depth := 0

			i := dotPos - 1
			for i >= 0 {
				if lineText[i] == ')' {
					depth++
				} else if lineText[i] == '(' {
					depth--
					if depth == 0 {
						break
					}
				}

				i--
			}

			if i > 0 {
				fnEnd := i

				fnStart := fnEnd - 1
				for fnStart >= 0 && IsWordChar(lineText[fnStart]) {
					fnStart--
				}

				fnStart++
				if fnStart < fnEnd {
					return "~?" + lineText[fnStart:dotPos]
				}
			}
		}

		return "~"
	}

	end := dotPos

	s := end - 1
	for s >= 0 && IsWordChar(lineText[s]) {
		s--
	}

	s++
	if s == end {
		return ""
	}

	return lineText[s:end]
}

// ComponentPathAtCursor checks if the cursor is on a component dot-path in a
// recognized context (new, createObject, extends, implements, type, returntype, etc).
func ComponentPathAtCursor(content string, line, char int) string {
	lineText := LineTextAt(content, line)
	if lineText == "" {
		return ""
	}

	pos := min(char, len(lineText))

	start := pos
	for start > 0 && (IsWordChar(lineText[start-1]) || lineText[start-1] == '.') {
		start--
	}

	end := pos
	for end < len(lineText) && (IsWordChar(lineText[end]) || lineText[end] == '.') {
		end++
	}

	if start == end {
		return ""
	}

	dotPath := lineText[start:end]
	hasDot := strings.ContainsRune(dotPath, '.')

	before := strings.TrimRight(lineText[:start], " \t")
	lowerBefore := strings.ToLower(before)

	if strings.HasSuffix(lowerBefore, "new") {
		return dotPath
	}

	if start > 0 && (lineText[start-1] == '"' || lineText[start-1] == '\'') {
		if strings.Contains(lowerBefore, "createobject(") {
			return dotPath
		}

		if strings.Contains(lowerBefore, "isinstanceof(") {
			return dotPath
		}

		if strings.HasSuffix(strings.TrimRight(lowerBefore, " \t\"'"), "import") {
			return dotPath
		}
	}

	attrPrefixes := []string{"extends=", "implements=", "type=", "returntype=", "component="}
	for _, prefix := range attrPrefixes {
		if idx := strings.LastIndex(lowerBefore, prefix); idx >= 0 {
			afterAttr := before[idx+len(prefix):]

			trimmed := strings.TrimLeft(afterAttr, " \t\"'")
			if trimmed == "" {
				return dotPath
			}
		}
	}

	after := strings.TrimLeft(lineText[end:], " \t")

	lowerAfter := strings.ToLower(after)
	if hasDot && (strings.HasPrefix(lowerAfter, "function") || strings.HasPrefix(lowerAfter, "function ")) {
		return dotPath
	}

	if hasDot && len(after) > 0 && IsWordChar(after[0]) {
		beforeTrimmed := strings.TrimRight(lineText[:start], " \t")
		if len(beforeTrimmed) > 0 {
			lastCh := beforeTrimmed[len(beforeTrimmed)-1]
			if lastCh == '(' || lastCh == ',' {
				return dotPath
			}

			if strings.HasSuffix(strings.ToLower(beforeTrimmed), "required") {
				return dotPath
			}
		}
	}

	return ""
}

// CfInvokeComponentAtCursor returns the component path if the cursor is inside
// a <cfinvoke> tag's method attribute value.
func CfInvokeComponentAtCursor(content string, line, char int) string {
	tag, cursorInTag := EnclosingTagAt(content, line, char)
	if tag == "" || cursorInTag < 0 {
		return ""
	}

	lower := strings.ToLower(tag)
	if !strings.HasPrefix(lower, "<cfinvoke") {
		return ""
	}

	methodIdx := strings.Index(lower, "method=")
	if methodIdx < 0 {
		return ""
	}

	valStart := methodIdx + 7
	if valStart >= len(tag) {
		return ""
	}

	q := tag[valStart]
	if q != '"' && q != '\'' {
		return ""
	}

	closeQ := strings.IndexByte(tag[valStart+1:], q)
	if closeQ < 0 {
		return ""
	}

	if cursorInTag <= valStart || cursorInTag > valStart+1+closeQ {
		return ""
	}

	compIdx := strings.Index(lower, "component=")
	if compIdx < 0 {
		return ""
	}

	cs := compIdx + 10
	if cs >= len(tag) {
		return ""
	}

	cq := tag[cs]
	if cq != '"' && cq != '\'' {
		return ""
	}

	closeC := strings.IndexByte(tag[cs+1:], cq)
	if closeC < 0 {
		return ""
	}

	return tag[cs+1 : cs+1+closeC]
}

// EnclosingTagAt returns the full tag text and cursor position within it.
func EnclosingTagAt(content string, line, char int) (tag string, cursorOffset int) {
	offset := 0
	for l := 0; l < line; l++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return "", -1
		}

		offset += idx + 1
	}

	offset += char
	if offset > len(content) {
		offset = len(content)
	}

	tagStart := strings.LastIndex(content[:offset], "<")
	if tagStart < 0 {
		return "", -1
	}

	tagEnd := strings.IndexByte(content[tagStart:], '>')
	if tagEnd < 0 {
		return content[tagStart:offset], offset - tagStart
	}

	tagEnd += tagStart + 1
	if offset > tagEnd {
		return "", -1
	}

	return content[tagStart:tagEnd], offset - tagStart
}

// FilePathAtCursor checks if the cursor is inside a file path attribute value.
func FilePathAtCursor(content string, line, char int) string {
	lineText := LineTextAt(content, line)
	if lineText == "" {
		return ""
	}

	pos := min(char, len(lineText))
	qStart := -1

	var quote byte

	for i := pos - 1; i >= 0; i-- {
		if lineText[i] == '"' || lineText[i] == '\'' {
			qStart = i
			quote = lineText[i]

			break
		}
	}

	if qStart < 0 {
		return ""
	}

	qEnd := -1

	for i := qStart + 1; i < len(lineText); i++ {
		if lineText[i] == quote {
			qEnd = i

			break
		}
	}

	if qEnd < 0 || pos <= qStart || pos > qEnd {
		return ""
	}

	value := lineText[qStart+1 : qEnd]
	before := strings.ToLower(strings.TrimRight(lineText[:qStart], " \t"))

	if strings.HasSuffix(before, "template=") || strings.HasSuffix(before, "include") ||
		strings.HasSuffix(before, "href=") || strings.HasSuffix(before, "action=") {
		return value
	}

	return ""
}

// CountNewlines counts the number of newline characters in s.
func CountNewlines(s string) int {
	n := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}

	return n
}

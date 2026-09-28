package parser

import "strings"

// FindMatchingTag finds the matching open/close tag at the given position.
// Returns a map with "line" and "character" keys, or nil if no match.
func FindMatchingTag(content string, line, char int) map[string]any {
	lineText := LineTextAt(content, line)
	if lineText == "" {
		return nil
	}

	pos := min(char, len(lineText))

	tagStart := -1

	for i := pos; i >= 0; i-- {
		if i < len(lineText) && lineText[i] == '<' {
			tagStart = i

			break
		}
	}

	if tagStart < 0 {
		return nil
	}

	isClose := tagStart+1 < len(lineText) && lineText[tagStart+1] == '/'
	nameStart := tagStart + 1

	if isClose {
		nameStart = tagStart + 2
	}

	nameEnd := nameStart
	for nameEnd < len(lineText) && lineText[nameEnd] != ' ' && lineText[nameEnd] != '>' && lineText[nameEnd] != '/' {
		nameEnd++
	}

	if nameStart == nameEnd {
		return nil
	}

	tagName := strings.ToLower(lineText[nameStart:nameEnd])

	offset := 0
	for range line {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return nil
		}

		offset += idx + 1
	}

	cursorOffset := offset + pos

	if isClose {
		return findOpenTagBefore(content, cursorOffset, tagName)
	}

	return findCloseTagAfter(content, offset+nameEnd, tagName)
}

// findOpenTagBefore walks back from cursorOffset to the tag that the close
// tag being read closes, skipping nested pairs of the same name.
func findOpenTagBefore(content string, cursorOffset int, tagName string) map[string]any {
	depth := 0

	for i := cursorOffset - 1; i >= 0; i-- {
		if i > 0 && content[i-1] == '<' && content[i] == '/' {
			end := strings.IndexByte(content[i:], '>')
			if end > 0 {
				name := strings.ToLower(strings.TrimSpace(content[i+1 : i+end]))
				if name == tagName {
					depth++
				}
			}

			continue
		}

		if content[i] != '<' || (i+1 < len(content) && content[i+1] == '/') {
			continue
		}

		end := i + 1
		for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
			end++
		}

		if strings.ToLower(content[i+1:end]) != tagName {
			continue
		}

		if depth == 0 {
			return offsetToPosition(content, i)
		}

		depth--
	}

	return nil
}

// findCloseTagAfter walks forward from the end of the open tag's name at
// nameEnd to the tag that closes it, skipping nested pairs of the same name.
func findCloseTagAfter(content string, nameEnd int, tagName string) map[string]any {
	searchStart := nameEnd
	for searchStart < len(content) && content[searchStart] != '>' {
		searchStart++
	}

	depth := 0

	for i := searchStart + 1; i < len(content); i++ {
		if content[i] != '<' {
			continue
		}

		if i+1 < len(content) && content[i+1] == '/' {
			end := i + 2
			for end < len(content) && content[end] != '>' && content[end] != ' ' {
				end++
			}

			if strings.ToLower(content[i+2:end]) != tagName {
				continue
			}

			if depth == 0 {
				return offsetToPosition(content, i)
			}

			depth--

			continue
		}

		end := i + 1
		for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
			end++
		}

		if strings.ToLower(content[i+1:end]) == tagName {
			depth++
		}
	}

	return nil
}

func offsetToPosition(content string, offset int) map[string]any {
	line := 0
	lastNL := -1

	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			lastNL = i
		}
	}

	char := offset - lastNL - 1

	return map[string]any{"line": line, "character": char}
}

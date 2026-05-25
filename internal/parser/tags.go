package parser

import "strings"

// FindMatchingTag finds the matching open/close tag at the given position.
// Returns a map with "line" and "character" keys, or nil if no match.
func FindMatchingTag(content string, line, char int) map[string]interface{} {
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
	for l := 0; l < line; l++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return nil
		}

		offset += idx + 1
	}

	cursorOffset := offset + pos

	if isClose {
		depth := 0

		i := cursorOffset - 1
		for i >= 0 {
			if i > 0 && content[i-1] == '<' && content[i] == '/' {
				end := strings.IndexByte(content[i:], '>')
				if end > 0 {
					name := strings.ToLower(strings.TrimSpace(content[i+1 : i+end]))
					if name == tagName {
						depth++
					}
				}
			} else if content[i] == '<' && (i+1 >= len(content) || content[i+1] != '/') {
				end := i + 1
				for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
					end++
				}

				name := strings.ToLower(content[i+1 : end])
				if name == tagName {
					if depth == 0 {
						return offsetToPosition(content, i)
					}

					depth--
				}
			}

			i--
		}
	} else {
		searchStart := offset + nameEnd
		for searchStart < len(content) && content[searchStart] != '>' {
			searchStart++
		}

		searchStart++
		depth := 0

		i := searchStart
		for i < len(content) {
			if content[i] == '<' {
				if i+1 < len(content) && content[i+1] == '/' {
					end := i + 2
					for end < len(content) && content[end] != '>' && content[end] != ' ' {
						end++
					}

					name := strings.ToLower(content[i+2 : end])
					if name == tagName {
						if depth == 0 {
							return offsetToPosition(content, i)
						}

						depth--
					}
				} else {
					end := i + 1
					for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
						end++
					}

					name := strings.ToLower(content[i+1 : end])
					if name == tagName {
						depth++
					}
				}
			}

			i++
		}
	}

	return nil
}

func offsetToPosition(content string, offset int) map[string]interface{} {
	line := 0
	lastNL := -1

	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			lastNL = i
		}
	}

	char := offset - lastNL - 1

	return map[string]interface{}{"line": line, "character": char}
}

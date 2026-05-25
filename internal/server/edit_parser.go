package server

import "strings"

// findCallContext finds the function name being called at the cursor position
// and which parameter the cursor is on (0-based). Also returns the full qualifier.
func findCallContext(content string, line, char int) (funcName string, qualifier string, activeParam int) {
	lineText := lineAtOffset(content, line)
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
				for start >= 0 && (isWordChar(lineText[start]) || lineText[start] == '.') {
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
									for fnStart >= 0 && isWordChar(lineText[fnStart]) {
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

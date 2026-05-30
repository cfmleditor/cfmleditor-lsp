package parser

import "strings"

// TextBeforeCursor returns all content from the start of the document up to the cursor position.
func TextBeforeCursor(content string, line, char int) string {
	offset := 0
	for range line {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return content
		}

		offset += idx + 1
	}

	end := min(offset+char, len(content))

	return content[:end]
}

// FindCurrentAttr returns the attribute name the cursor is currently inside
// (i.e., after name="...|...) within an open tag.
func FindCurrentAttr(content string, line, char int) string {
	textBefore := TextBeforeCursor(content, line, char)

	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return ""
	}

	afterOpen := textBefore[lastOpen:]
	inSingle := false
	inDouble := false
	lastEq := -1

	for i, ch := range afterOpen {
		switch {
		case ch == '=' && !inSingle && !inDouble:
			lastEq = i
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		}
	}

	if lastEq == -1 {
		return ""
	}

	before := strings.TrimRight(afterOpen[:lastEq], " \t")
	start := strings.LastIndexAny(before, " \t\r\n") + 1

	return toLowerASCII(before[start:])
}

// IsTypingTagName returns true if the cursor is inside an incomplete tag name (e.g. "<cfif").
func IsTypingTagName(content string, line, char int) bool {
	textBefore := TextBeforeCursor(content, line, char)

	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return false
	}

	after := textBefore[lastOpen:]
	if strings.Contains(after, ">") {
		return false
	}

	rest := after[1:]
	if len(rest) == 0 || rest[0] == '/' || rest[0] == '!' {
		return false
	}

	if strings.ContainsAny(rest, " \t\r\n/>") {
		return false
	}

	return true
}

// IsSpecialTag returns true for tags that don't follow normal open/close patterns.
func IsSpecialTag(name string) bool {
	switch name {
	case "cfset", "cfif", "cfelse", "cfelseif":
		return true
	}

	return false
}

// IsSubordinateTag returns true for tags that share another tag's closing tag
// (e.g. cfelse and cfelseif are closed by </cfif>).
func IsSubordinateTag(name string) bool {
	switch name {
	case "cfelse", "cfelseif":
		return true
	}

	return false
}

// IsVoidTag returns true for self-closing tags that have no closing tag.
func IsVoidTag(name string) bool {
	switch name {
	case "cfparam", "cfreturn", "cfargument", "cfproperty", "cfrethrow", "cfthrow", "cfschedule", "cfhttpparam", "cfqueryparam", "cftimer", "cfflush", "cfcache", "cflogout", "cfprocessingdirective", "cfzipelement",
		"cfbreak", "cfcontinue", "cfabort", "cfexit", "cfinclude", "cflocation", "cfheader", "cfdump",
		"cfcontent", "cfcookie", "cflog", "cffile", "cfdirectory", "cfsetting", "cfwddx",
		"cfhtmlhead", "cfhtmlbody", "cfauthenticate", "cfntauthenticate", "cfreportparam",
		"cfprocparam", "cfprocresult", "cfinvokeargument", "cfspreadsheet", "cfpdfparam",
		"cfpdfformparam", "cfpdfsubform", "cfmailparam", "cfgridrow", "cfgridupdate", "cfimage",
		"cftreeitem", "cfmenuitem", "cfmaplocation", "cfpresenteritem", "cfimport", "cftrace",
		"cfgridcolumn",
		"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr":
		return true
	}

	return false
}

// FindUnclosedTags scans the document from startLine to the cursor and returns tag names
// that have been opened but not yet closed, most recent first.
func FindUnclosedTags(content string, startLine, line, char int) []string {
	text := TextBeforeCursor(content, line, char)

	if startLine > 0 {
		offset := 0
		for range startLine {
			idx := strings.IndexByte(text[offset:], '\n')
			if idx < 0 {
				offset = len(text)

				break
			}

			offset += idx + 1
		}

		text = text[offset:]
	}

	var stack []string

	i := 0
	for i < len(text) {
		idx := strings.Index(text[i:], "<")
		if idx == -1 {
			break
		}

		i += idx + 1
		if i >= len(text) {
			break
		}

		if text[i] == '/' {
			i++

			end := strings.IndexAny(text[i:], "> \t\r\n")
			if end == -1 {
				break
			}

			closeName := toLowerASCII(text[i : i+end])
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j] == closeName {
					stack = append(stack[:j], stack[j+1:]...)

					break
				}
			}

			i += end
		} else {
			end := strings.IndexAny(text[i:], " \t\r\n/>")
			if end == -1 {
				break
			}

			name := toLowerASCII(text[i : i+end])
			if name == "" || name[0] == '!' || name == "cfset" || IsSubordinateTag(name) || IsVoidTag(name) {
				i += end

				continue
			}

			closeIdx := strings.Index(text[i:], ">")
			if closeIdx != -1 && closeIdx > 0 && text[i+closeIdx-1] == '/' {
				i += closeIdx + 1

				continue
			}

			stack = append(stack, name)

			if closeIdx != -1 {
				i += closeIdx + 1
			} else {
				i += end
			}
		}
	}

	for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
		stack[i], stack[j] = stack[j], stack[i]
	}

	return stack
}

func toLowerASCII(s string) string {
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}

		b.WriteByte(c)
	}

	return b.String()
}

// IsClosingTagContext returns true if the cursor is right after "</".
func IsClosingTagContext(content string, line, char int) bool {
	textBefore := TextBeforeCursor(content, line, char)

	return strings.HasSuffix(textBefore, "</")
}

// IsInsideHashExpr returns true if the cursor is inside a #...# expression.
func IsInsideHashExpr(content string, line, char int) bool {
	textBefore := TextBeforeCursor(content, line, char)
	boundary := strings.LastIndex(textBefore, ">")
	lastOpen := strings.LastIndex(textBefore, "<")

	if lastOpen > boundary {
		boundary = lastOpen
	}

	if boundary == -1 {
		boundary = 0
	}

	lastNL := strings.LastIndex(textBefore, "\n")
	if lastNL > boundary {
		boundary = lastNL
	}

	return strings.Count(textBefore[boundary:], "#")%2 == 1
}

// IsInsideAttrValue returns true if the cursor is inside a quoted attribute value.
func IsInsideAttrValue(content string, line, char int) bool {
	textBefore := TextBeforeCursor(content, line, char)

	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return false
	}

	afterOpen := textBefore[lastOpen:]
	if strings.Contains(afterOpen, ">") {
		return false
	}

	inSingle := false
	inDouble := false

	for _, ch := range afterOpen {
		switch {
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		}
	}

	return inSingle || inDouble
}

// FindEnclosingTag scans backwards from the cursor to find the tag name
// if the cursor is inside an open CFML tag (after the tag name and a space).
func FindEnclosingTag(content string, line, char int) string {
	textBefore := TextBeforeCursor(content, line, char)

	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return ""
	}

	afterOpen := textBefore[lastOpen:]
	if strings.Contains(afterOpen, ">") {
		return ""
	}

	rest := strings.TrimLeft(afterOpen[1:], " \t")

	tagEnd := strings.IndexAny(rest, " \t\r\n/>")
	if tagEnd == -1 {
		return ""
	}

	tagName := strings.ToLower(rest[:tagEnd])
	if tagName == "" || tagName[0] == '/' {
		return ""
	}

	return tagName
}

// WordBeforeDot returns the identifier immediately before the dot at the cursor position.
func WordBeforeDot(content string, line, char int) string {
	lineText := LineTextAt(content, line)
	if lineText == "" && line > 0 {
		return ""
	}

	dotPos := char - 1
	if dotPos < 1 || dotPos >= len(lineText) || lineText[dotPos] != '.' {
		return ""
	}

	end := dotPos

	start := end - 1
	for start >= 0 && IsWordChar(lineText[start]) {
		start--
	}

	start++
	if start == end {
		return ""
	}

	return lineText[start:end]
}

// PositionToOffset converts a line/character position to a byte offset.
func PositionToOffset(content string, line, char int) int {
	offset := 0
	for range line {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return len(content)
		}

		offset += idx + 1
	}

	offset += char
	if offset > len(content) {
		offset = len(content)
	}

	return offset
}

// ApplyEdit replaces a range in content with newText.
func ApplyEdit(content string, startLine, startChar, endLine, endChar int, newText string) string {
	offset := PositionToOffset(content, startLine, startChar)
	endOffset := PositionToOffset(content, endLine, endChar)

	return content[:offset] + newText + content[endOffset:]
}

package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	items := []protocol.CompletionItem{}
	tags := make(map[string]int)

	content, hasDoc := s.getDocument(uri.URI(params.TextDocument.URI))

	tagName := ""
	if hasDoc {
		tagName = findEnclosingTag(content, int(params.Position.Line), int(params.Position.Character))
	}

	triggeredByTag := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == "<"

	triggeredByClose := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == ">"

	closing := false
	if hasDoc {
		closing = isClosingTagContext(content, int(params.Position.Line), int(params.Position.Character))
	}

	switch {
	case triggeredByClose && hasDoc:
		if item, ok := duplicateGtCompletion(content, int(params.Position.Line), int(params.Position.Character)); ok {
			items = append(items, item)
		}
		if item, ok := closeTagCompletion(content, int(params.Position.Line), int(params.Position.Character)); ok {
			items = append(items, item)
		}
	case closing:
		for _, tag := range findUnclosedTags(content, int(params.Position.Line), int(params.Position.Character)) {
			_, ok := tags[tag]
			if ( !ok ) {
				items = append(items, protocol.CompletionItem{
					Label:      tag,
					Kind:       protocol.CompletionItemKindKeyword,
					Detail:     "Close tag",
					InsertText: tag + ">",
				})
				tags[tag] = len(items);
			}
		}
	case tagName != "" && !isSpecialTag(tagName):
		for _, p := range docs.TagParams(tagName) {
			items = append(items, protocol.CompletionItem{
				Label:            p.Name,
				Kind:             protocol.CompletionItemKindProperty,
				Detail:           p.Description,
				InsertText:       p.Name + `="$1"`,
				InsertTextFormat: protocol.InsertTextFormatSnippet,
			})
		}
	case tagName == "cfelse":
		// Compute the column of the '<' that starts this tag.
		textBefore := textBeforeCursor(content, int(params.Position.Line), int(params.Position.Character))
		tagStart := strings.LastIndex(textBefore, "<")
		startChar := int(params.Position.Character) - (len(textBefore) - tagStart)
		items = append(items, protocol.CompletionItem{
			Label:           "if",
			Kind:            protocol.CompletionItemKindKeyword,
			Detail:          "Convert to cfelseif",
			FilterText:      "if",
			InsertTextFormat: protocol.InsertTextFormatSnippet,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: params.Position.Line, Character: uint32(startChar)},
					End:   params.Position,
				},
				NewText: "<cfelseif $1",
			},
		})
	case triggeredByTag:
		for _, tag := range docs.AllTags() {
			items = append(items, protocol.CompletionItem{
				Label:  tag.Name,
				Kind:   protocol.CompletionItemKindKeyword,
				Detail: tag.Description,
			})
		}
	default:
		for _, fn := range docs.AllFunctions() {
			items = append(items, protocol.CompletionItem{
				Label:            fn.Name,
				Kind:             protocol.CompletionItemKindFunction,
				Detail:           fn.Syntax,
				Documentation:    fn.Description,
				InsertTextFormat: protocol.InsertTextFormatPlainText,
			})
		}
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// isClosingTagContext returns true if the cursor is right after "</".
func isClosingTagContext(content string, line, char int) bool {
	textBefore := textBeforeCursor(content, line, char)
	return strings.HasSuffix(textBefore, "</")
}

func isVoidTag(name string) bool {
    switch name {
    case "cfparam", "cfreturn", "cfargument", "cfproperty", "cfrethrow", "cfthrow", "cfschedule", "cfhttpparam", "cfqueryparam", "cftimer", "cfflush", "cfcache", "cflogout", "cfprocessingdirective", "cfzipelement",
    "cfbreak", "cfcontinue", "cfabort", "cfexit", "cfinclude", "cflocation", "cfheader", "cfdump",
    "cfcontent", "cfcookie", "cflog", "cffile", "cfdirectory", "cfsetting", "cfwddx",
    "cfhtmlhead", "cfhtmlbody", "cfauthenticate", "cfntauthenticate", "cfreportparam",
    "cfprocparam", "cfprocresult", "cfinvokeargument", "cfspreadsheet", "cfpdfparam",
    "cfpdfformparam", "cfpdfsubform", "cfmailparam", "cfgridrow", "cfgridupdate", "cfimage",
    "cftreeitem", "cfmenuitem", "cfmaplocation", "cfpresenteritem", "cfimport", "cftrace", "br", "hr", "input",
    "cfgridcolumn":
        return true
    }
    return false
}

func isSpecialTag(name string) bool {
    switch name {
    case "cfset", "cfif", "cfelse", "cfelseif":
        return true
    }
    return false
}

// isSubordinateTag returns true for tags that share another tag's closing tag
// (e.g. cfelse and cfelseif are closed by </cfif>).
func isSubordinateTag(name string) bool {
    switch name {
    case "cfelse", "cfelseif":
        return true
    }
    return false
}

// findUnclosedTags scans the document up to the cursor and returns tag names
// that have been opened but not yet closed, most recent first.
func findUnclosedTags(content string, line, char int) []string {
	text := textBeforeCursor(content, line, char)

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
			// Closing tag
			i++
			end := strings.IndexAny(text[i:], "> \t\r\n")
			if end == -1 {
				break
			}
			closeName := strings.ToLower(text[i : i+end])
			// Pop matching tag from stack
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j] == closeName {
					stack = append(stack[:j], stack[j+1:]...)
					break
				}
			}
			i += end
		} else {
			// Opening tag
			end := strings.IndexAny(text[i:], " \t\r\n/>")
			if end == -1 {
				break
			}
			name := strings.ToLower(text[i : i+end])
			if name == "" || name[0] == '!' || name == "cfset" || isSubordinateTag(name) || isVoidTag(name) {
				i += end
				continue
			}

			// Check for self-closing />
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

	// Reverse so most recent unclosed tag is first
	for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
		stack[i], stack[j] = stack[j], stack[i]
	}
	return stack
}

func textBeforeCursor(content string, line, char int) string {
	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) {
		return content
	}
	var sb strings.Builder
	for i := 0; i < line; i++ {
		sb.WriteString(lines[i])
	}
	lineText := lines[line]
	if char > len(lineText) {
		char = len(lineText)
	}
	sb.WriteString(lineText[:char])
	return sb.String()
}

// findEnclosingTag scans backwards from the cursor position to determine
// if the cursor is inside an open CFML tag (after the tag name and a space).
// Returns the lowercase tag name if found, or empty string otherwise.
func findEnclosingTag(content string, line, char int) string {
	textBefore := textBeforeCursor(content, line, char)

	// Find the last '<' that isn't closed by '>'
	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return ""
	}
	afterOpen := textBefore[lastOpen:]
	if strings.Contains(afterOpen, ">") {
		return ""
	}

	// Extract tag name: first word after '<'
	rest := strings.TrimLeft(afterOpen[1:], " \t")
	tagEnd := strings.IndexAny(rest, " \t\r\n/>")
	if tagEnd == -1 {
		return "" // still typing the tag name
	}

	tagName := strings.ToLower(rest[:tagEnd])
	if tagName == "" || tagName[0] == '/' {
		return ""
	}

	return tagName
}

// duplicateGtCompletion offers to remove a duplicate '>' when the user types
// '>' immediately after an existing tag-closing '>'.
func duplicateGtCompletion(content string, line, char int) (protocol.CompletionItem, bool) {
	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) || char < 2 {
		return protocol.CompletionItem{}, false
	}
	lineText := lines[line]
	if char > len(lineText) || lineText[char-2] != '>' {
		return protocol.CompletionItem{}, false
	}
	// Verify the previous '>' closes a tag.
	before := lineText[:char-1]
	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return protocol.CompletionItem{}, false
	}
	if strings.ContainsRune(before[openIdx:len(before)-1], '>') {
		return protocol.CompletionItem{}, false
	}
	return protocol.CompletionItem{
		Label:      ">",
		Kind:       protocol.CompletionItemKindKeyword,
		Detail:     "Remove duplicate >",
		FilterText: ">",
		TextEdit: &protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(line), Character: uint32(char - 1)},
				End:   protocol.Position{Line: uint32(line), Character: uint32(char)},
			},
			NewText: "",
		},
	}, true
}

// closeTagCompletion returns a completion item when '>' is typed mid-tag
// with non-whitespace content between the typed '>' and the tag's existing '>'.
// The completion moves the content before the '>' and removes the duplicate.
func closeTagCompletion(content string, line, char int) (protocol.CompletionItem, bool) {
	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) {
		return protocol.CompletionItem{}, false
	}
	lineText := lines[line]
	if char >= len(lineText) {
		return protocol.CompletionItem{}, false
	}

	// Find next '>' after cursor.
	rest := lineText[char:]
	idx := strings.IndexByte(rest, '>')
	if idx == -1 {
		return protocol.CompletionItem{}, false
	}

	middle := rest[:idx]

	// Only offer completion when there's non-whitespace content.
	if strings.TrimSpace(middle) == "" {
		return protocol.CompletionItem{}, false
	}

	// Must not contain '<' (would mean we left the tag).
	if strings.ContainsRune(middle, '<') {
		return protocol.CompletionItem{}, false
	}

	// Verify we're inside a tag.
	before := lineText[:char]
	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return protocol.CompletionItem{}, false
	}
	if strings.ContainsRune(lineText[openIdx:char-1], '>') {
		return protocol.CompletionItem{}, false
	}

	endChar := char + idx + 1
	return protocol.CompletionItem{
		Label:           ">",
		Kind:            protocol.CompletionItemKindKeyword,
		Detail:          "Close tag",
		FilterText:      ">",
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		TextEdit: &protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(line), Character: uint32(char - 1)},
				End:   protocol.Position{Line: uint32(line), Character: uint32(endChar)},
			},
			NewText: middle + ">",
		},
	}, true
}

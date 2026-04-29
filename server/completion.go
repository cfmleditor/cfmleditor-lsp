package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/garethedwards/cfmleditor-lsp/cfml"
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

	content, hasDoc := s.getDocument(uri.URI(params.TextDocument.URI))

	tagName := ""
	if hasDoc {
		tagName = findEnclosingTag(content, int(params.Position.Line), int(params.Position.Character))
	}

	triggeredByTag := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == "<"

	closing := false
	if hasDoc {
		closing = isClosingTagContext(content, int(params.Position.Line), int(params.Position.Character))
	}

	switch {
	case closing:
		for _, tag := range findUnclosedTags(content, int(params.Position.Line), int(params.Position.Character)) {
			items = append(items, protocol.CompletionItem{
				Label:      tag,
				Kind:       protocol.CompletionItemKindKeyword,
				Detail:     "Close tag",
				InsertText: tag + ">",
			})
		}
	case tagName != "":

		attrs := cfml.TagAttributes()[tagName]
		for _, attr := range attrs {
			items = append(items, protocol.CompletionItem{
				Label:            attr.Name,
				Kind:             protocol.CompletionItemKindProperty,
				Detail:           attr.Detail,
				InsertText:       attr.Name + `="$1"`,
				InsertTextFormat: protocol.InsertTextFormatSnippet,
			})
		}
	case triggeredByTag:
		for _, tag := range cfml.Tags() {
			items = append(items, protocol.CompletionItem{
				Label:  tag.Name,
				Kind:   protocol.CompletionItemKindKeyword,
				Detail: tag.Detail,
			})
		}
	default:
		for _, fn := range cfml.Functions() {
			items = append(items, protocol.CompletionItem{
				Label:            fn.Name,
				Kind:             protocol.CompletionItemKindFunction,
				Detail:           fn.Signature,
				Documentation:    fn.Detail,
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
			if name == "" || name[0] == '!' {
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

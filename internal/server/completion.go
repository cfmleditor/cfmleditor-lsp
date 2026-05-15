package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// Completion feature flags — set to false to disable specific providers.
const (
	CompletionBuiltinFunctions = true
	CompletionLocalVariables   = true
	CompletionDotMethods       = true
	CompletionMemberFunctions  = true
	CompletionTags             = true
	CompletionCloseTags        = true
	CompletionAttributes       = true
)

// Cached global completion items (never change at runtime).
var (
	builtinFuncItems     []protocol.CompletionItem
	builtinFuncItemsOnce sync.Once
	memberFuncItems      []protocol.CompletionItem
	memberFuncItemsOnce  sync.Once
)

func getBuiltinFuncItems() []protocol.CompletionItem {
	builtinFuncItemsOnce.Do(func() {
		fns := docs.AllFunctions()
		scopes := []string{"VARIABLES", "ARGUMENTS", "THIS", "SERVER", "APPLICATION", "REQUEST", "SESSION"}
		builtinFuncItems = make([]protocol.CompletionItem, 0, len(fns)+len(scopes))
		for _, fn := range fns {
			builtinFuncItems = append(builtinFuncItems, protocol.CompletionItem{
				Label:            fn.Name,
				Kind:             protocol.CompletionItemKindFunction,
				Detail:           fn.Syntax,
				Documentation:    fn.Description,
				InsertTextFormat: protocol.InsertTextFormatPlainText,
			})
		}
		for _, s := range scopes {
			builtinFuncItems = append(builtinFuncItems, protocol.CompletionItem{
				Label: s,
				Kind:  protocol.CompletionItemKindKeyword,
			})
		}
	})
	return builtinFuncItems
}

func getMemberFuncItems() []protocol.CompletionItem {
	memberFuncItemsOnce.Do(func() {
		mfs := docs.AllMemberFunctions()
		memberFuncItems = make([]protocol.CompletionItem, 0, len(mfs))
		for _, mf := range mfs {
			memberFuncItems = append(memberFuncItems, protocol.CompletionItem{
				Label:         mf.Name,
				Kind:          protocol.CompletionItemKindMethod,
				Detail:        mf.Entry.Syntax,
				Documentation: mf.Entry.Description,
			})
		}
	})
	return memberFuncItems
}

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	totalStart := time.Now()
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	items := []protocol.CompletionItem(nil)

	content, hasDoc := s.getDocument(uri.URI(params.TextDocument.URI))

	t0 := time.Now()
	tagName := ""
	if hasDoc {
		tagName = findEnclosingTag(content, int(params.Position.Line), int(params.Position.Character))
	}

	triggeredByTag := (params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == "<") ||
		(hasDoc && strings.HasSuffix(textBeforeCursor(content, int(params.Position.Line), int(params.Position.Character)), "<"))

	triggeredByClose := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == ">"

	triggeredByDot := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == "."

	closing := false
	typingTag := false
	inHashExpr := false
	inAttrValue := false
	if hasDoc {
		closing = isClosingTagContext(content, int(params.Position.Line), int(params.Position.Character))
		if !closing && tagName == "" {
			typingTag = isTypingTagName(content, int(params.Position.Line), int(params.Position.Character))
		}
		inHashExpr = isInsideHashExpr(content, int(params.Position.Line), int(params.Position.Character))
		inAttrValue = isInsideAttrValue(content, int(params.Position.Line), int(params.Position.Character))
	}
	contextDur := time.Since(t0)

	switch {
	case inHashExpr:
		items = s.completionFromCache(uri.URI(params.TextDocument.URI), int(params.Position.Line))
	case inAttrValue:
		if CompletionAttributes {
			attrName := findCurrentAttr(content, int(params.Position.Line), int(params.Position.Character))
			if attrName != "" && tagName != "" {
				attrs := docs.TagParams(tagName)
				if attrs == nil {
					attrs = docs.HTMLTagParams(tagName)
				}
				for i := range attrs {
					if strings.ToLower(attrs[i].Name) == attrName {
						for _, v := range attrs[i].ParamValues() {
							items = append(items, protocol.CompletionItem{
								Label:  v,
								Kind:   protocol.CompletionItemKindValue,
								Detail: attrName + " value",
							})
						}
						break
					}
				}
			}
		}
		if CompletionBuiltinFunctions {
			items = append(items, getBuiltinFuncItems()...)
		}
	case triggeredByClose && hasDoc:
		if CompletionCloseTags {
			if item, ok := duplicateGtCompletion(content, int(params.Position.Line), int(params.Position.Character)); ok {
				items = append(items, item)
			}
			if item, ok := closeTagCompletion(content, int(params.Position.Line), int(params.Position.Character)); ok {
				items = append(items, item)
			}
		}
	case closing:
		if CompletionCloseTags {
			t1 := time.Now()
			trailingGt := -1
			if hasDoc {
				lineStart := 0
				for i := 0; i < int(params.Position.Line); i++ {
					idx := strings.IndexByte(content[lineStart:], '\n')
					if idx < 0 {
						lineStart = len(content)
						break
					}
					lineStart += idx + 1
				}
				lineEnd := strings.IndexByte(content[lineStart:], '\n')
				if lineEnd < 0 {
					lineEnd = len(content) - lineStart
				}
				lineText := content[lineStart : lineStart+lineEnd]
				charPos := int(params.Position.Character)
				if charPos < len(lineText) {
					after := lineText[charPos:]
					if idx := strings.IndexByte(after, '>'); idx != -1 && strings.TrimSpace(after[:idx]) == "" {
						trailingGt = charPos + idx + 1
					}
				}
			}
			tags := make(map[string]int)
			for i, tag := range s.findUnclosedTagsScoped(content, uri.URI(params.TextDocument.URI), int(params.Position.Line), int(params.Position.Character)) {
				_, ok := tags[tag]
				if !ok {
					item := protocol.CompletionItem{
						Label:    tag,
						Kind:     protocol.CompletionItemKindKeyword,
						Detail:   "Close tag",
						SortText: fmt.Sprintf("%04d", i),
					}
					if trailingGt >= 0 {
						item.TextEdit = &protocol.TextEdit{
							Range: protocol.Range{
								Start: params.Position,
								End:   protocol.Position{Line: params.Position.Line, Character: uint32(trailingGt)},
							},
							NewText: tag + ">",
						}
					} else {
						item.InsertText = tag + ">"
					}
					items = append(items, item)
					tags[tag] = len(items)
				}
			}
			s.logger.Info("completion: closeTags", zap.Duration("dur", time.Since(t1)))
		}
	case tagName != "" && !isSpecialTag(tagName):
		if CompletionAttributes {
			t1 := time.Now()
			attrs := docs.TagParams(tagName)
			if attrs == nil {
				attrs = docs.HTMLTagParams(tagName)
			}
			for _, p := range attrs {
				items = append(items, protocol.CompletionItem{
					Label:            p.Name,
					Kind:             protocol.CompletionItemKindProperty,
					Detail:           p.Description,
					InsertText:       p.Name + `="$1"`,
					InsertTextFormat: protocol.InsertTextFormatSnippet,
				})
			}
			s.logger.Info("completion: attributes", zap.Duration("dur", time.Since(t1)))
		}
	case tagName == "cfelse":
		items = append(items, protocol.CompletionItem{
			Label:            "if",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "Convert to cfelseif",
			FilterText:       "if",
			InsertTextFormat: protocol.InsertTextFormatSnippet,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: params.Position.Line, Character: uint32(int(params.Position.Character) - (len(textBeforeCursor(content, int(params.Position.Line), int(params.Position.Character))) - strings.LastIndex(textBeforeCursor(content, int(params.Position.Line), int(params.Position.Character)), "<")))},
					End:   params.Position,
				},
				NewText: "<cfelseif $1",
			},
		})
	case triggeredByTag:
		if CompletionTags {
			t1 := time.Now()
			for _, tag := range docs.AllTags() {
				items = append(items, protocol.CompletionItem{
					Label:  tag.Name,
					Kind:   protocol.CompletionItemKindKeyword,
					Detail: tag.Description,
				})
			}
			for _, tag := range docs.HTMLTags() {
				items = append(items, protocol.CompletionItem{
					Label:  tag.Name,
					Kind:   protocol.CompletionItemKindKeyword,
					Detail: tag.Description,
				})
			}
			s.logger.Info("completion: tags", zap.Duration("dur", time.Since(t1)))
		}
	case typingTag:
		if CompletionTags {
			for _, tag := range docs.AllTags() {
				items = append(items, protocol.CompletionItem{
					Label:  tag.Name,
					Kind:   protocol.CompletionItemKindKeyword,
					Detail: tag.Description,
				})
			}
			for _, tag := range docs.HTMLTags() {
				items = append(items, protocol.CompletionItem{
					Label:  tag.Name,
					Kind:   protocol.CompletionItemKindKeyword,
					Detail: tag.Description,
				})
			}
		}
	case triggeredByDot && hasDoc:
		if CompletionDotMethods {
			t1 := time.Now()
			if methods := s.dotCompletionMethods(content, uri.URI(params.TextDocument.URI), int(params.Position.Line), int(params.Position.Character)); len(methods) > 0 {
				items = append(items, methods...)
				s.logger.Info("completion: dotMethods", zap.Duration("dur", time.Since(t1)))
			} else if CompletionMemberFunctions {
				items = append(items, getMemberFuncItems()...)
				s.logger.Info("completion: memberFunctions", zap.Duration("dur", time.Since(t1)))
			}
		}
	default:
		items = s.completionFromCache(uri.URI(params.TextDocument.URI), int(params.Position.Line))
	}

	s.logger.Info("completion: total",
		zap.Duration("context", contextDur),
		zap.Duration("total", time.Since(totalStart)),
		zap.Int("items", len(items)),
	)

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

// isInsideHashExpr returns true if the cursor is inside a #...# expression.
func isInsideHashExpr(content string, line, char int) bool {
	textBefore := textBeforeCursor(content, line, char)
	return strings.Count(textBefore, "#")%2 == 1
}

// isInsideAttrValue returns true if the cursor is inside a quoted attribute value.
func isInsideAttrValue(content string, line, char int) bool {
	textBefore := textBeforeCursor(content, line, char)
	// Find the last '<' not closed by '>'
	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return false
	}
	afterOpen := textBefore[lastOpen:]
	if strings.Contains(afterOpen, ">") {
		return false
	}
	// Count quotes after the tag open to determine if we're inside a string
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

// findCurrentAttr returns the attribute name whose value the cursor is inside.
func findCurrentAttr(content string, line, char int) string {
	textBefore := textBeforeCursor(content, line, char)
	lastOpen := strings.LastIndex(textBefore, "<")
	if lastOpen == -1 {
		return ""
	}
	afterOpen := textBefore[lastOpen:]
	// Find the last '=' before an open quote that isn't closed
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
	// Extract attribute name before the '='
	before := strings.TrimRight(afterOpen[:lastEq], " \t")
	start := strings.LastIndexAny(before, " \t\r\n") + 1
	return strings.ToLower(before[start:])
}

// isTypingTagName returns true if the cursor is inside an incomplete tag name (e.g. "<cfif").
func isTypingTagName(content string, line, char int) bool {
	textBefore := textBeforeCursor(content, line, char)
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

func isVoidTag(name string) bool {
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

// findUnclosedTagsScoped scans for unclosed tags within the enclosing function body,
// falling back to the full file if the cursor is outside any function.
func (s *Server) findUnclosedTagsScoped(content string, docURI uri.URI, line, char int) []string {
	s.mu.RLock()
	funcs := s.funcRanges[docURI]
	s.mu.RUnlock()

	startLine := 0
	idx := sort.Search(len(funcs), func(i int) bool {
		return funcs[i].End >= line
	})
	if idx < len(funcs) && line >= funcs[idx].Start {
		startLine = funcs[idx].Start
	}
	return findUnclosedTags(content, startLine, line, char)
}

// findUnclosedTags scans the document from startLine to the cursor and returns tag names
// that have been opened but not yet closed, most recent first.
func findUnclosedTags(content string, startLine, line, char int) []string {
	text := textBeforeCursor(content, line, char)
	// Trim to startLine offset
	if startLine > 0 {
		offset := 0
		for i := 0; i < startLine; i++ {
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
	offset := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return content
		}
		offset += idx + 1
	}
	end := offset + char
	if end > len(content) {
		end = len(content)
	}
	return content[:end]
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
		Label:            ">",
		Kind:             protocol.CompletionItemKindKeyword,
		Detail:           "Close tag",
		FilterText:       ">",
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

// completionFromCache returns cached items for the cursor's scope.
// File cache already contains builtins + globals. Function cache has local vars only.
func (s *Server) completionFromCache(docURI uri.URI, line int) []protocol.CompletionItem {
	s.mu.RLock()
	funcs := s.funcRanges[docURI]
	s.mu.RUnlock()

	fileItems := s.compCache.GetFile(docURI)
	if fileItems == nil {
		fileItems = getBuiltinFuncItems()
	}

	idx := sort.Search(len(funcs), func(i int) bool {
		return funcs[i].End >= line
	})
	if idx < len(funcs) && line >= funcs[idx].Start {
		funcItems := s.compCache.GetFuncStale(docURI, funcs[idx].Name)
		if len(funcItems) == 0 {
			return fileItems
		}
		items := make([]protocol.CompletionItem, 0, len(fileItems)+len(funcItems))
		items = append(items, fileItems...)
		items = append(items, funcItems...)
		return items
	}

	return fileItems
}

// rebuildCompletionCache pre-computes completion items for scopes in a file.
// editLine indicates which line was edited; only the function containing that line
// has its local vars scanned. Pass -1 to skip function scanning (file-level only).
func (s *Server) rebuildCompletionCache(docURI uri.URI, content string, editLine int) {
	start := time.Now()
	s.mu.RLock()
	funcs := s.funcRanges[docURI]
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if editLine >= 0 {
		// Binary search — funcRanges is sorted by Start line
		idx := sort.Search(len(funcs), func(i int) bool {
			return funcs[i].End >= editLine
		})
		if idx < len(funcs) && editLine >= funcs[idx].Start {
			f := funcs[idx]
			hash := cache.HashScope(content, f.Start, f.End)
			if s.compCache.GetFunc(docURI, f.Name, hash) == nil {
				var vars []string
				if pr != nil {
					vars = pr.FuncVars(f.Start, f.End)
				} else {
					vars = cfparser.VarsInFunc(content, f.Start, f.End)
				}
				items := make([]protocol.CompletionItem, 0, len(vars))
				for _, v := range vars {
					items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
				}
				s.compCache.PutFunc(docURI, f.Name, hash, items)
				s.logger.Info("completion: func vars rebuilt",
					zap.String("uri", string(docURI)),
					zap.String("func", f.Name),
					zap.Int("vars", len(vars)),
					zap.Duration("dur", time.Since(start)),
				)
			}
		}
	}
}

// rebuildFileCompletionCache rebuilds the file-level completion cache (builtins + globals).
// Called on didOpen and didSave. Builtins are included here since this only runs
// on open/save, avoiding per-request copies.
func (s *Server) rebuildFileCompletionCache(docURI uri.URI) {
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()
	if pr != nil {
		s.rebuildFileCompletionCacheFromPR(docURI, pr)
		return
	}
	content, ok := s.getDocument(docURI)
	if !ok {
		return
	}
	newPR := cfparser.Parse(docURI, content)
	newPR.Log = &zapAdapter{s.logger}
	s.mu.Lock()
	s.parseResults[docURI] = newPR
	s.mu.Unlock()
	s.rebuildFileCompletionCacheFromPR(docURI, newPR)
}

// rebuildFileCompletionCacheFromPR rebuilds file-level completion from an existing ParseResult.
func (s *Server) rebuildFileCompletionCacheFromPR(docURI uri.URI, pr *cfparser.ParseResult) {
	start := time.Now()
	builtins := getBuiltinFuncItems()
	globals := pr.GlobalVars()
	items := make([]protocol.CompletionItem, 0, len(builtins)+len(globals)+len(pr.Funcs))
	items = append(items, builtins...)
	for _, v := range globals {
		items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
	}
	for _, f := range pr.Funcs {
		detail := f.Name + "("
		for i, arg := range f.Arguments {
			if i > 0 {
				detail += ", "
			}
			if arg.Type != "" {
				detail += arg.Type + " "
			}
			detail += arg.Name
		}
		detail += ")"
		items = append(items, protocol.CompletionItem{
			Label:  f.Name,
			Kind:   protocol.CompletionItemKindFunction,
			Detail: detail,
		})
	}
	s.compCache.PutFile(docURI, items)

	varsItems := make([]protocol.CompletionItem, 0)
	for _, v := range pr.VariablesVars() {
		varsItems = append(varsItems, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
	}
	s.compCache.PutFunc(docURI, "__variables__", 0, varsItems)

	thisItems := make([]protocol.CompletionItem, 0, len(pr.Funcs))
	for _, v := range pr.ThisVars() {
		thisItems = append(thisItems, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindProperty})
	}
	for _, f := range pr.Funcs {
		detail := f.Name + "("
		for i, arg := range f.Arguments {
			if i > 0 {
				detail += ", "
			}
			if arg.Type != "" {
				detail += arg.Type + " "
			}
			detail += arg.Name
		}
		detail += ")"
		thisItems = append(thisItems, protocol.CompletionItem{
			Label:  f.Name,
			Kind:   protocol.CompletionItemKindMethod,
			Detail: detail,
		})
	}
	s.compCache.PutFunc(docURI, "__this__", 0, thisItems)
	s.logger.Info("completion: file globals rebuilt",
		zap.String("uri", string(docURI)),
		zap.Int("globals", len(globals)),
		zap.Duration("dur", time.Since(start)),
	)
}

// scopeArgumentsCompletion returns the declared arguments for the enclosing function.
func (s *Server) scopeArgumentsCompletion(docURI uri.URI, line int) []protocol.CompletionItem {
	s.mu.RLock()
	funcs := s.funcRanges[docURI]
	s.mu.RUnlock()

	idx := sort.Search(len(funcs), func(i int) bool {
		return funcs[i].End >= line
	})
	if idx >= len(funcs) || line < funcs[idx].Start {
		return nil
	}
	funcName := funcs[idx].Name

	defs := s.index.FunctionsForFile(docURI)
	for _, d := range defs {
		if strings.EqualFold(d.Name, funcName) {
			items := make([]protocol.CompletionItem, 0, len(d.Arguments))
			for _, arg := range d.Arguments {
				detail := arg.Type
				if arg.Required {
					detail += " required"
				}
				items = append(items, protocol.CompletionItem{
					Label:  arg.Name,
					Kind:   protocol.CompletionItemKindVariable,
					Detail: strings.TrimSpace(detail),
				})
			}
			return items
		}
	}
	return nil
}

// superCompletion returns functions from the parent component (extends).
func (s *Server) superCompletion(docURI uri.URI) []protocol.CompletionItem {
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()
	if pr == nil || pr.Extends == "" {
		return nil
	}

	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)
	cfcPath := cfpath.ResolvePath(pr.Extends, baseDir, s.Mappings)
	if cfcPath == "" {
		for _, root := range s.WorkspaceFolders {
			cfcPath = cfpath.ResolvePath(pr.Extends, root, s.Mappings)
			if cfcPath != "" {
				break
			}
		}
	}
	if cfcPath == "" {
		return nil
	}

	cfcURI := uri.URI("file://" + cfcPath)
	defs := s.index.FunctionsForFile(cfcURI)
	if len(defs) == 0 {
		data, err := os.ReadFile(cfcPath)
		if err != nil {
			return nil
		}
		s.index.IndexFile(cfcURI, string(data))
		defs = s.index.FunctionsForFile(cfcURI)
	}

	items := make([]protocol.CompletionItem, 0, len(defs))
	for _, d := range defs {
		detail := d.Name + "("
		for i, arg := range d.Arguments {
			if i > 0 {
				detail += ", "
			}
			if arg.Type != "" {
				detail += arg.Type + " "
			}
			detail += arg.Name
		}
		detail += ")"
		items = append(items, protocol.CompletionItem{
			Label:  d.Name,
			Kind:   protocol.CompletionItemKindMethod,
			Detail: detail,
		})
	}
	return items
}

// dotCompletionMethods returns completion items for methods on a component
// instance variable. It extracts the word before the dot, looks up the
// component ref, resolves the CFC path, and returns its function defs.
func (s *Server) dotCompletionMethods(content string, docURI uri.URI, line, char int) []protocol.CompletionItem {
	// Extract variable name before the dot
	varName := wordBeforeDot(content, line, char)
	if varName == "" {
		return nil
	}

	// Scope dot completion: VARIABLES., ARGUMENTS., THIS., SUPER.
	switch strings.ToUpper(varName) {
	case "VARIABLES":
		return s.compCache.GetFuncStale(docURI, "__variables__")
	case "THIS":
		return s.compCache.GetFuncStale(docURI, "__this__")
	case "ARGUMENTS":
		return s.scopeArgumentsCompletion(docURI, line)
	case "SUPER":
		return s.superCompletion(docURI)
	}

	// Look up component ref for this variable in the current file
	ref := s.index.LookupComponentRefInFile(varName, docURI, uint32(line))
	if ref == nil {
		return nil
	}

	// Resolve the dot-path to a CFC file relative to the current file's directory
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)
	cfcPath := cfpath.ResolvePath(ref.Component, baseDir, s.Mappings)
	if cfcPath == "" {
		// Try workspace folders
		for _, root := range s.WorkspaceFolders {
			cfcPath = cfpath.ResolvePath(ref.Component, root, s.Mappings)
			if cfcPath != "" {
				break
			}
		}
	}
	if cfcPath == "" {
		return nil
	}

	// Get function defs from the index (already cached), fall back to parsing
	cfcURI := uri.URI("file://" + cfcPath)
	defs := s.index.FunctionsForFile(cfcURI)
	if len(defs) == 0 {
		// Not indexed yet — read, parse, and store in index
		cfcContent, ok := s.getDocument(cfcURI)
		if !ok {
			data, err := os.ReadFile(cfcPath)
			if err != nil {
				return nil
			}
			cfcContent = string(data)
		}
		s.index.IndexFile(cfcURI, cfcContent)
		defs = s.index.FunctionsForFile(cfcURI)
	}
	if len(defs) == 0 {
		return nil
	}

	items := make([]protocol.CompletionItem, 0, len(defs))
	for _, d := range defs {
		detail := d.Name + "("
		for i, arg := range d.Arguments {
			if i > 0 {
				detail += ", "
			}
			if arg.Type != "" {
				detail += arg.Type + " "
			}
			detail += arg.Name
		}
		detail += ")"
		items = append(items, protocol.CompletionItem{
			Label:  d.Name,
			Kind:   protocol.CompletionItemKindMethod,
			Detail: detail,
		})
	}
	return items
}

// wordBeforeDot extracts the identifier immediately before the dot at the cursor.
func wordBeforeDot(content string, line, char int) string {
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return ""
	}
	lineText := lines[line]
	// char is after the dot, so dot is at char-1
	dotPos := char - 1
	if dotPos < 1 || dotPos >= len(lineText) || lineText[dotPos] != '.' {
		return ""
	}
	end := dotPos
	start := end - 1
	for start >= 0 && isWordChar(lineText[start]) {
		start--
	}
	start++
	if start == end {
		return ""
	}
	return lineText[start:end]
}

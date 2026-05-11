package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
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
		builtinFuncItems = make([]protocol.CompletionItem, 0, len(fns))
		for _, fn := range fns {
			builtinFuncItems = append(builtinFuncItems, protocol.CompletionItem{
				Label:            fn.Name,
				Kind:             protocol.CompletionItemKindFunction,
				Detail:           fn.Syntax,
				Documentation:    fn.Description,
				InsertTextFormat: protocol.InsertTextFormatPlainText,
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

	items := []protocol.CompletionItem{}
	tags := make(map[string]int)

	content, hasDoc := s.getDocument(uri.URI(params.TextDocument.URI))

	t0 := time.Now()
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
		if cached := s.completionFromCache(content, uri.URI(params.TextDocument.URI), int(params.Position.Line)); cached != nil {
			items = cached
		} else {
			items = append(items, getBuiltinFuncItems()...)
			if CompletionLocalVariables && hasDoc {
				for _, v := range parser.VarsAt(content, int(params.Position.Line)) {
					items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
				}
			}
		}
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
				lines := strings.SplitAfter(content, "\n")
				if int(params.Position.Line) < len(lines) {
					lineText := lines[int(params.Position.Line)]
					if int(params.Position.Character) < len(lineText) {
						after := lineText[int(params.Position.Character):]
						if idx := strings.IndexByte(after, '>'); idx != -1 && strings.TrimSpace(after[:idx]) == "" {
							trailingGt = int(params.Position.Character) + idx + 1
						}
					}
				}
			}
			for _, tag := range findUnclosedTags(content, int(params.Position.Line), int(params.Position.Character)) {
				_, ok := tags[tag]
				if !ok {
					item := protocol.CompletionItem{
						Label:  tag,
						Kind:   protocol.CompletionItemKindKeyword,
						Detail: "Close tag",
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
		if cached := s.completionFromCache(content, uri.URI(params.TextDocument.URI), int(params.Position.Line)); cached != nil {
			items = cached
		} else {
			items = append(items, getBuiltinFuncItems()...)
			if CompletionLocalVariables && hasDoc {
				for _, v := range parser.VarsAt(content, int(params.Position.Line)) {
					items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
				}
			}
		}
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

// completionFromCache returns cached items for the cursor's scope, or nil on miss.
func (s *Server) completionFromCache(content string, docURI uri.URI, line int) []protocol.CompletionItem {
	funcs := s.funcRangesForContent(docURI, content)
	for _, f := range funcs {
		if line >= f.Start && line <= f.End {
			hash := cache.HashScope(content, f.Start, f.End)
			return s.compCache.GetFunc(docURI, f.Name, hash)
		}
	}
	hash := cache.HashScope(content, 0, strings.Count(content, "\n"))
	return s.compCache.GetFile(docURI, hash)
}

// rebuildCompletionCache pre-computes completion items for all scopes in a file.
func (s *Server) rebuildCompletionCache(docURI uri.URI, content string) {
	start := time.Now()
	funcs := s.funcRangesForContent(docURI, content)
	builtins := getBuiltinFuncItems()

	for _, f := range funcs {
		hash := cache.HashScope(content, f.Start, f.End)
		if s.compCache.GetFunc(docURI, f.Name, hash) != nil {
			continue
		}
		items := make([]protocol.CompletionItem, 0, len(builtins)+8)
		items = append(items, builtins...)
		for _, v := range parser.VarsAt(content, f.End) {
			items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
		}
		s.compCache.PutFunc(docURI, f.Name, hash, items)
	}

	fileHash := cache.HashScope(content, 0, strings.Count(content, "\n"))
	if s.compCache.GetFile(docURI, fileHash) == nil {
		items := make([]protocol.CompletionItem, 0, len(builtins)+8)
		items = append(items, builtins...)
		lastLine := strings.Count(content, "\n")
		for _, v := range parser.VarsAt(content, lastLine) {
			items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable})
		}
		s.compCache.PutFile(docURI, fileHash, items)
	}

	s.logger.Info("completion: cache rebuilt",
		zap.String("uri", string(docURI)),
		zap.Int("scopes", len(funcs)+1),
		zap.Duration("dur", time.Since(start)),
	)
}

// funcRangesForContent returns function line ranges for cache scope detection.
func (s *Server) funcRangesForContent(docURI uri.URI, content string) []cache.FuncRange {
	defs := parser.ParseFunctionDefs(docURI, content)
	ranges := make([]cache.FuncRange, 0, len(defs))
	lines := strings.Split(content, "\n")
	for _, d := range defs {
		// Estimate function end by finding next function or EOF
		end := len(lines) - 1
		for _, d2 := range defs {
			if d2.Line > d.Line && int(d2.Line) < end {
				end = int(d2.Line) - 1
			}
		}
		ranges = append(ranges, cache.FuncRange{
			Name:  d.Name,
			Start: int(d.Line),
			End:   end,
		})
	}
	return ranges
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

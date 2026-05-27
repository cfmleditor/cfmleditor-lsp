package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
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

// Sort priority prefixes for completion items. Lower ASCII = higher priority.
// Each category has a unique prefix to ensure deliberate ordering.
const (
	SortCloseTags       = "0" // immediate actions (close tag, remove duplicate >)
	SortFuncArguments   = "1" // named function arguments (companyCode=)
	SortLocalVariables  = "2" // local/function-scoped variables
	SortUserFunctions   = "3" // user-defined functions and methods in current file
	SortProperties      = "4" // this/variables properties
	SortGlobalVariables = "5" // file-scope global variables
	SortTags            = "6" // CF and HTML tags
	SortScopes          = "7" // scope keywords (VARIABLES, ARGUMENTS, etc.)
	SortBuiltinFuncs    = "8" // built-in CFML functions
	SortMemberFuncs     = "9" // member functions (after dot on unresolved type)
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
			insertText := fn.Name + "("

			for i, p := range fn.Params {
				if i > 0 {
					insertText += ", "
				}

				insertText += fmt.Sprintf("${%d:%s}", i+1, p.Name)
			}

			insertText += ")"
			builtinFuncItems = append(builtinFuncItems, protocol.CompletionItem{
				Label:            fn.Name,
				Kind:             protocol.CompletionItemKindFunction,
				Detail:           fn.Syntax,
				Documentation:    fn.Description,
				InsertText:       insertText,
				InsertTextFormat: protocol.InsertTextFormatSnippet,
				SortText:         SortBuiltinFuncs + fn.Name,
			})
		}

		for i, s := range scopes {
			builtinFuncItems = append(builtinFuncItems, protocol.CompletionItem{
				Label:    s,
				Kind:     protocol.CompletionItemKindKeyword,
				SortText: fmt.Sprintf("%s%d%s", SortScopes, i, s),
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
				SortText:      SortMemberFuncs + mf.Name,
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

	content, hasDoc := s.getDocument(params.TextDocument.URI)

	t0 := time.Now()

	tagName := ""
	if hasDoc {
		tagName = parser.FindEnclosingTag(content, int(params.Position.Line), int(params.Position.Character))
	}

	triggeredByTag := (params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == "<") ||
		(hasDoc && strings.HasSuffix(parser.TextBeforeCursor(content, int(params.Position.Line), int(params.Position.Character)), "<"))

	triggeredByClose := params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == ">"

	triggeredByDot := (params.Context != nil &&
		params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter &&
		params.Context.TriggerCharacter == ".") ||
		(hasDoc && parser.WordBeforeDot(content, int(params.Position.Line), int(params.Position.Character)) != "")

	closing := false
	typingTag := false
	inHashExpr := false
	inAttrValue := false

	if hasDoc {
		closing = parser.IsClosingTagContext(content, int(params.Position.Line), int(params.Position.Character))
		if !closing && tagName == "" {
			typingTag = parser.IsTypingTagName(content, int(params.Position.Line), int(params.Position.Character))
		}

		inHashExpr = parser.IsInsideHashExpr(content, int(params.Position.Line), int(params.Position.Character))
		inAttrValue = parser.IsInsideAttrValue(content, int(params.Position.Line), int(params.Position.Character))
	}

	contextDur := time.Since(t0)

	triggerKind := ""

	if params.Context != nil {
		switch params.Context.TriggerKind {
		case protocol.CompletionTriggerKindInvoked:
			triggerKind = "invoked"
		case protocol.CompletionTriggerKindTriggerCharacter:
			triggerKind = "char:" + params.Context.TriggerCharacter
		case protocol.CompletionTriggerKindTriggerForIncompleteCompletions:
			triggerKind = "incomplete"
		}
	}

	s.log.Debug("completion: request",
		cflog.String("trigger", triggerKind),
		cflog.Bool("triggeredByDot", triggeredByDot),
		cflog.Bool("inHashExpr", inHashExpr),
		cflog.Bool("inAttrValue", inAttrValue),
		cflog.String("tagName", tagName),
		cflog.Uint32("line", params.Position.Line),
		cflog.Uint32("char", params.Position.Character),
	)

	switch {
	case inHashExpr:
		items = s.completionFromCache(params.TextDocument.URI, int(params.Position.Line))
	case inAttrValue:
		if CompletionAttributes {
			attrName := parser.FindCurrentAttr(content, int(params.Position.Line), int(params.Position.Character))
			if attrName != "" && tagName != "" {
				attrs := docs.TagParams(tagName)
				if attrs == nil {
					attrs = docs.HTMLTagParams(tagName)
				}

				for i := range attrs {
					if strings.ToLower(attrs[i].Name) == attrName {
						for _, v := range attrs[i].ParamValues() {
							items = append(items, protocol.CompletionItem{SortText: SortProperties,
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
			for i, tag := range s.findUnclosedTagsScoped(content, params.TextDocument.URI, int(params.Position.Line), int(params.Position.Character)) {
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

			s.log.Debug("completion: closeTags", cflog.Duration("dur", time.Since(t1)))
		}
	case tagName != "" && !parser.IsSpecialTag(tagName):
		if CompletionAttributes {
			t1 := time.Now()

			attrs := docs.TagParams(tagName)
			if attrs == nil {
				attrs = docs.HTMLTagParams(tagName)
			}

			for _, p := range attrs {
				items = append(items, protocol.CompletionItem{SortText: SortProperties,
					Label:            p.Name,
					Kind:             protocol.CompletionItemKindProperty,
					Detail:           p.Description,
					InsertText:       p.Name + `="$1"`,
					InsertTextFormat: protocol.InsertTextFormatSnippet,
				})
			}

			s.log.Debug("completion: attributes", cflog.Duration("dur", time.Since(t1)))
		}
	case tagName == "cfelse":
		items = append(items, protocol.CompletionItem{SortText: SortProperties,
			Label:            "if",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "Convert to cfelseif",
			FilterText:       "if",
			InsertTextFormat: protocol.InsertTextFormatSnippet,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: params.Position.Line, Character: uint32(int(params.Position.Character) - (len(parser.TextBeforeCursor(content, int(params.Position.Line), int(params.Position.Character))) - strings.LastIndex(parser.TextBeforeCursor(content, int(params.Position.Line), int(params.Position.Character)), "<")))},
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
					Label:            tag.Name,
					SortText:         SortTags + tag.Name,
					Kind:             protocol.CompletionItemKindKeyword,
					Detail:           tag.Description,
					InsertText:       buildTagSnippet(tag),
					InsertTextFormat: protocol.InsertTextFormatSnippet,
				})
			}

			for _, tag := range docs.HTMLTags() {
				items = append(items, protocol.CompletionItem{
					Label:    tag.Name,
					SortText: SortTags + tag.Name,
					Kind:     protocol.CompletionItemKindKeyword,
					Detail:   tag.Description,
				})
			}

			s.log.Debug("completion: tags", cflog.Duration("dur", time.Since(t1)))
		}
	case typingTag:
		if CompletionTags {
			for _, tag := range docs.AllTags() {
				items = append(items, protocol.CompletionItem{
					Label:            tag.Name,
					SortText:         SortTags + tag.Name,
					Kind:             protocol.CompletionItemKindKeyword,
					Detail:           tag.Description,
					InsertText:       buildTagSnippet(tag),
					InsertTextFormat: protocol.InsertTextFormatSnippet,
				})
			}

			for _, tag := range docs.HTMLTags() {
				items = append(items, protocol.CompletionItem{
					Label:    tag.Name,
					SortText: SortTags + tag.Name,
					Kind:     protocol.CompletionItemKindKeyword,
					Detail:   tag.Description,
				})
			}
		}
	case triggeredByDot && hasDoc:
		if CompletionDotMethods {
			t1 := time.Now()

			if methods := s.dotCompletionMethods(content, params.TextDocument.URI, int(params.Position.Line), int(params.Position.Character)); len(methods) > 0 {
				items = append(items, methods...)

				s.log.Debug("completion: dotMethods", cflog.Duration("dur", time.Since(t1)))
			} else if CompletionMemberFunctions {
				items = append(items, getMemberFuncItems()...)

				s.log.Debug("completion: memberFunctions", cflog.Duration("dur", time.Since(t1)))
			}
		}
	default:
		// Check if inside a function call — offer named argument completions first
		if hasDoc {
			if argItems := s.argumentCompletion(content, params.TextDocument.URI, int(params.Position.Line), int(params.Position.Character)); len(argItems) > 0 {
				items = append(items, argItems...)
			}
		}

		items = append(items, s.completionFromCache(params.TextDocument.URI, int(params.Position.Line))...)
	}

	s.log.Debug("completion: total",
		cflog.Duration("context", contextDur),
		cflog.Duration("total", time.Since(totalStart)),
		cflog.Int("items", len(items)),
	)

	if !s.TagSnippets || !s.FunctionSnippets {
		for i := range items {
			if items[i].InsertTextFormat != protocol.InsertTextFormatSnippet {
				continue
			}

			isTag := items[i].Kind == protocol.CompletionItemKindKeyword
			isFunc := items[i].Kind == protocol.CompletionItemKindFunction || items[i].Kind == protocol.CompletionItemKindMethod

			if (isTag && !s.TagSnippets) || (isFunc && !s.FunctionSnippets) {
				items[i].InsertText = ""
				items[i].InsertTextFormat = protocol.InsertTextFormatPlainText
			}
		}
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
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

	return parser.FindUnclosedTags(content, startLine, line, char)
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
		SortText:   SortCloseTags,
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
		SortText:         SortCloseTags,
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
					vars = parser.VarsInFunc(content, f.Start, f.End)
				}

				items := make([]protocol.CompletionItem, 0, len(vars))
				for _, v := range vars {
					items = append(items, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable, SortText: SortLocalVariables + v})
				}

				s.compCache.PutFunc(docURI, f.Name, hash, items)
				s.log.Debug("completion: func vars rebuilt",
					cflog.String("uri", string(docURI)),
					cflog.String("func", f.Name),
					cflog.Int("vars", len(vars)),
					cflog.Duration("dur", time.Since(start)),
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

	newPR := s.parseContent(docURI, content)
	s.mu.Lock()
	s.parseResults[docURI] = newPR
	s.mu.Unlock()
	s.rebuildFileCompletionCacheFromPR(docURI, newPR)
}

// rebuildFileCompletionCacheFromPR rebuilds file-level completion from an existing ParseResult.
func (s *Server) rebuildFileCompletionCacheFromPR(docURI uri.URI, pr *parser.ParseResult) {
	start := time.Now()
	builtins := getBuiltinFuncItems()
	globals := pr.VariablesVars()
	thisVarNames := pr.ThisVars()
	items := make([]protocol.CompletionItem, 0, len(builtins)+len(globals)+len(thisVarNames)+len(pr.Funcs))
	items = append(items, builtins...)
	varPrefix := s.scopePrefix("variables")
	thisPrefix := s.scopePrefix("this")

	for _, v := range globals {
		scoped := varPrefix + "." + v
		items = append(items, protocol.CompletionItem{
			Label:    scoped,
			Kind:     protocol.CompletionItemKindVariable,
			SortText: SortGlobalVariables + v,
		})
		items = append(items, protocol.CompletionItem{
			Label:      v,
			Kind:       protocol.CompletionItemKindVariable,
			Detail:     scoped,
			InsertText: scoped,
			SortText:   SortGlobalVariables + v,
		})
	}

	for _, v := range thisVarNames {
		scoped := thisPrefix + "." + v
		items = append(items, protocol.CompletionItem{
			Label:    scoped,
			Kind:     protocol.CompletionItemKindProperty,
			SortText: SortGlobalVariables + v,
		})
		items = append(items, protocol.CompletionItem{
			Label:      v,
			Kind:       protocol.CompletionItemKindProperty,
			Detail:     scoped,
			InsertText: scoped,
			SortText:   SortGlobalVariables + v,
		})
	}

	for _, f := range pr.Funcs {
		detail := f.Name + "("
		insertText := f.Name + "("

		for i, arg := range f.Arguments {
			if i > 0 {
				detail += ", "
				insertText += ", "
			}

			if arg.Type != "" {
				detail += arg.Type + " "
			}

			detail += arg.Name
			insertText += fmt.Sprintf("${%d:%s}", i+1, arg.Name)
		}

		detail += ")"
		insertText += ")"
		items = append(items, protocol.CompletionItem{
			Label:            f.Name,
			Kind:             protocol.CompletionItemKindFunction,
			Detail:           detail,
			InsertText:       insertText,
			InsertTextFormat: protocol.InsertTextFormatSnippet,
			SortText:         SortUserFunctions + f.Name,
		})
	}

	s.compCache.PutFile(docURI, items)

	varsItems := make([]protocol.CompletionItem, 0, len(pr.VariablesVars()))
	for _, v := range pr.VariablesVars() {
		varsItems = append(varsItems, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindVariable, SortText: SortLocalVariables + v})
	}

	s.compCache.PutFunc(docURI, "__variables__", 0, varsItems)

	thisItems := make([]protocol.CompletionItem, 0, len(pr.Funcs))
	for _, v := range pr.ThisVars() {
		thisItems = append(thisItems, protocol.CompletionItem{Label: v, Kind: protocol.CompletionItemKindProperty, SortText: SortLocalVariables + v})
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
			Label:    f.Name,
			Kind:     protocol.CompletionItemKindMethod,
			Detail:   detail,
			SortText: SortUserFunctions + f.Name,
		})
	}

	s.compCache.PutFunc(docURI, "__this__", 0, thisItems)
	s.log.Debug("completion: file globals rebuilt",
		cflog.String("uri", string(docURI)),
		cflog.Int("globals", len(globals)),
		cflog.Duration("dur", time.Since(start)),
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
					Label:    arg.Name,
					Kind:     protocol.CompletionItemKindVariable,
					Detail:   strings.TrimSpace(detail),
					SortText: SortProperties + arg.Name,
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

	cfcPath := s.getResolver().ComponentPath(pr.Extends, baseDir)
	if cfcPath == "" {
		return nil
	}

	defs := s.getResolver().EnsureIndexed(cfcPath)

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
			Label:    d.Name,
			Kind:     protocol.CompletionItemKindMethod,
			Detail:   detail,
			SortText: SortUserFunctions + d.Name,
		})
	}

	return items
}

// dotCompletionMethods returns completion items for methods on a component
// instance variable. It extracts the word before the dot, looks up the
// component ref, resolves the CFC path, and returns its function defs.
func (s *Server) dotCompletionMethods(content string, docURI uri.URI, line, char int) []protocol.CompletionItem {
	// Extract variable name before the dot
	varName := parser.WordBeforeDot(content, line, char)

	// If no simple word, check for call expression before dot: e.g. getService("tours").
	if varName == "" {
		lineText := parser.LineTextAt(content, line)
		dotPos := char - 1

		if dotPos > 0 && dotPos < len(lineText) && lineText[dotPos] == '.' && lineText[dotPos-1] == ')' {
			// Find matching open paren
			depth := 0

			i := dotPos - 1
			for i >= 0 {
				if lineText[i] == ')' {
					depth++
				} else if lineText[i] == '(' {
					depth--
					if depth == 0 {
						// Find function name start
						fnStart := i - 1
						for fnStart >= 0 && parser.IsWordChar(lineText[fnStart]) {
							fnStart--
						}

						fnStart++
						callExpr := lineText[fnStart:dotPos]

						comp := parser.ResolveFromCall(callExpr, s.cfResolvers())
						if comp != "" {
							currentPath := strings.TrimPrefix(string(docURI), "file://")
							baseDir := filepath.Dir(currentPath)

							cfcPath := s.getResolver().ComponentPath(comp, baseDir)
							if cfcPath != "" {
								return s.methodCompletionItems(cfcPath)
							}
						}

						break
					}
				}

				i--
			}
		}

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

	var component string

	if ref != nil {
		component = ref.Component
	} else if comp := parser.ResolveFromCall(varName, s.cfResolvers()); comp != "" {
		component = comp
	} else {
		return nil
	}

	// Resolve the dot-path to a CFC file relative to the current file's directory
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	var cfcPath string

	if filepath.IsAbs(component) {
		if _, err := s.FS.Stat(component); err == nil {
			cfcPath = component
		}
	}

	if cfcPath == "" {
		cfcPath = s.getResolver().ComponentPath(component, baseDir)
	}

	if cfcPath == "" {
		return nil
	}

	// Get function defs from the index (already cached), fall back to parsing
	cfcURI := uri.URI("file://" + cfcPath)
	defs := s.getResolver().EnsureIndexed(cfcPath)

	thisVars := s.index.ThisVarsForFile(cfcURI)

	if len(defs) == 0 && len(thisVars) == 0 {
		return nil
	}

	items := make([]protocol.CompletionItem, 0, len(defs)+len(thisVars))
	for _, v := range thisVars {
		items = append(items, protocol.CompletionItem{
			Label:    v,
			Kind:     protocol.CompletionItemKindProperty,
			SortText: SortLocalVariables + v,
		})
	}

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
			Label:    d.Name,
			Kind:     protocol.CompletionItemKindMethod,
			Detail:   detail,
			SortText: SortUserFunctions + d.Name,
		})
	}

	return items
}

// argumentCompletion returns named argument completions when cursor is inside function parens.
func (s *Server) argumentCompletion(content string, docURI uri.URI, line, char int) []protocol.CompletionItem {
	funcName, qualifier, _ := parser.FindCallContext(content, line, char)
	if funcName == "" {
		return nil
	}

	var def *parser.FunctionDef

	// Try builtin
	if e, ok := docs.LookupFunction(funcName); ok {
		var items []protocol.CompletionItem
		for _, p := range e.Params {
			items = append(items, protocol.CompletionItem{
				Label:            p.Name + "=",
				Kind:             protocol.CompletionItemKindField,
				Detail:           p.Type,
				InsertText:       p.Name + "=",
				InsertTextFormat: protocol.InsertTextFormatPlainText,
				SortText:         SortFuncArguments + p.Name,
			})
		}

		return items
	}

	// Try qualified user function
	if qualifier != "" {
		def = s.resolveUserFunc(qualifier, funcName, docURI, uint32(line))
	}
	// Try unqualified
	if def == nil {
		defs := s.index.Lookup(funcName)
		if len(defs) > 0 {
			def = defs[0]

			for _, d := range defs {
				if d.URI == docURI {
					def = d

					break
				}
			}
		}
	}

	if def == nil || len(def.Arguments) == 0 {
		return nil
	}

	var items []protocol.CompletionItem
	for _, arg := range def.Arguments {
		items = append(items, protocol.CompletionItem{
			Label:            arg.Name + "=",
			Kind:             protocol.CompletionItemKindField,
			Detail:           arg.Type,
			InsertText:       arg.Name + "=",
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			SortText:         SortFuncArguments + arg.Name,
		})
	}

	return items
}

// methodCompletionItems returns completion items for all functions in a CFC file.
func (s *Server) methodCompletionItems(cfcPath string) []protocol.CompletionItem {
	defs := s.getResolver().EnsureIndexed(cfcPath)
	items := make([]protocol.CompletionItem, 0, len(defs))

	for _, d := range defs {
		detail := d.Name + "("
		insertText := d.Name + "("

		for i, arg := range d.Arguments {
			if i > 0 {
				detail += ", "
				insertText += ", "
			}

			if arg.Type != "" {
				detail += arg.Type + " "
			}

			detail += arg.Name
			insertText += fmt.Sprintf("${%d:%s}", i+1, arg.Name)
		}

		detail += ")"
		insertText += ")"
		items = append(items, protocol.CompletionItem{
			Label:            d.Name,
			Kind:             protocol.CompletionItemKindMethod,
			Detail:           detail,
			InsertText:       insertText,
			InsertTextFormat: protocol.InsertTextFormatSnippet,
			SortText:         SortUserFunctions + d.Name,
		})
	}

	return items
}

// buildTagSnippet creates a snippet for a CF tag with required attributes as tab stops.
func buildTagSnippet(tag *docs.Entry) string {
	params := tag.Params

	var b strings.Builder

	b.WriteString(tag.Name)

	tabIdx := 1

	for _, p := range params {
		if p.Required {
			fmt.Fprintf(&b, ` %s="${%d:%s}"`, p.Name, tabIdx, p.Name)

			tabIdx++
		}
	}

	b.WriteString(">")

	return b.String()
}

func (s *Server) scopePrefix(scope string) string {
	switch s.Formatting.ScopeCase {
	case "upper":
		return strings.ToUpper(scope)
	case "lower":
		return strings.ToLower(scope)
	default:
		return scope
	}
}




package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)
	word := wordAtPosition(content, line, char)
	if word == "" {
		return reply(ctx, nil, nil)
	}

	docURI := uri.URI(params.TextDocument.URI)
	s.logger.Debug("definition: request", zap.String("word", word), zap.Int("line", line), zap.Int("char", char))

	// Check if cursor is inside a resolver-matched call (e.g. getService("UserService"))
	if comp := s.resolverArgAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.logger.Debug("definition: resolver arg resolved", zap.String("component", comp))
			return reply(ctx, *loc, nil)
		}
	}

	// Check if cursor is on a component dot-path (new, createObject, extends, etc.)
	if comp := componentPathAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.logger.Debug("definition: component path resolved", zap.String("path", comp), zap.String("target", string(loc.URI)))
			return reply(ctx, *loc, nil)
		}
		s.logger.Debug("definition: component path not resolved", zap.String("path", comp))
	}

	// Check if cursor is on a file path (cfinclude, cfmodule template)
	if filePath := filePathAtCursor(content, line, char); filePath != "" {
		if loc := s.resolveFilePathDef(filePath, docURI); loc != nil {
			s.logger.Debug("definition: file path resolved", zap.String("path", filePath), zap.String("target", string(loc.URI)))
			return reply(ctx, *loc, nil)
		}
		// Cursor is inside a file path — don't fall through to word-based lookup
		return reply(ctx, nil, nil)
	}

	// Check if cursor is inside a <cfinvoke> method attribute
	if comp := cfInvokeComponentAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
			return reply(ctx, *loc, nil)
		}
	}

	// Check if there's a dot qualifier (e.g. persist.templateFunction)
	if qualifier := qualifierBeforeWord(content, line, char); qualifier != "" {
		s.logger.Debug("definition: qualifier found", zap.String("qualifier", qualifier), zap.String("word", word))
		if strings.HasPrefix(qualifier, "~?") { //nolint:gocritic // if-else is clearer than switch for prefix checks
			// Call expression — try component resolvers
			callExpr := qualifier[2:]
			comp := resolveComponentFromCall(callExpr, s.ComponentResolvers)
			s.logger.Debug("definition: call expression resolver", zap.String("expr", callExpr), zap.String("resolved", comp))
			if comp != "" {
				if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
			}
		} else if strings.HasPrefix(qualifier, "~") {
			// Direct component path from createObject/new expression
			comp := qualifier[1:]
			s.logger.Debug("definition: direct component path", zap.String("component", comp))
			if comp != "" {
				if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
			}
		} else {
			// Resolve via component ref
			ref := s.index.LookupComponentRefInFile(qualifier, docURI, uint32(line))
			if ref == nil {
				// Lazily index function body refs
				s.ensureFuncRefsIndexed(docURI, line)
				ref = s.index.LookupComponentRefInFile(qualifier, docURI, uint32(line))
			}
			if ref != nil {
				s.logger.Debug("definition: component ref found", zap.String("variable", qualifier), zap.String("component", ref.Component))
				if loc := s.resolveComponentDef(ref.Component, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
				s.logger.Debug("definition: component ref resolve failed", zap.String("component", ref.Component), zap.String("func", word))
			} else {
				s.logger.Debug("definition: no component ref for qualifier", zap.String("variable", qualifier))
			}
			// Try component resolvers for the qualifier itself (e.g. _parent → a CFC)
			if comp := resolveComponentFromCall(qualifier, s.ComponentResolvers); comp != "" {
				s.logger.Debug("definition: qualifier matched resolver", zap.String("qualifier", qualifier), zap.String("resolved", comp))
				if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
			}
		}
		// Qualified call that can't be resolved — fall through to all matches
		// but exclude current file (a qualified call is never local)
		if !s.GlobalFunctionResolution {
			return reply(ctx, nil, nil)
		}
		defs := s.index.Lookup(word)
		s.logger.Debug("definition: qualified fallback to global lookup", zap.String("word", word), zap.Int("matches", len(defs)))
		var locations []protocol.Location
		for _, d := range defs {
			if d.URI != docURI {
				locations = append(locations, protocol.Location{
					URI:   protocol.DocumentURI(d.URI),
					Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
				})
			}
		}
		if len(locations) == 1 {
			return reply(ctx, locations[0], nil)
		}
		if len(locations) > 1 {
			return reply(ctx, locations, nil)
		}
		return reply(ctx, nil, nil)
	}

	// No qualifier — prefer current file's definition
	defs := s.index.Lookup(word)
	if len(defs) == 0 {
		return reply(ctx, nil, nil)
	}
	for _, d := range defs {
		if d.URI == docURI {
			return reply(ctx, protocol.Location{
				URI:   protocol.DocumentURI(d.URI),
				Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
			}, nil)
		}
	}

	// Not in current file — only return if global resolution is enabled
	if !s.GlobalFunctionResolution {
		return reply(ctx, nil, nil)
	}
	var locations []protocol.Location
	for _, d := range defs {
		locations = append(locations, protocol.Location{
			URI:   protocol.DocumentURI(d.URI),
			Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
		})
	}
	if len(locations) == 1 {
		return reply(ctx, locations[0], nil)
	}
	return reply(ctx, locations, nil)
}

// func (s *Server) resolveQualifiedDef(qualifier, funcName string, docURI uri.URI, line uint32) *protocol.Location {
// 	ref := s.index.LookupComponentRefInFile(qualifier, docURI, line)
// 	if ref == nil {
// 		return nil
// 	}
// 	return s.resolveComponentDef(ref.Component, funcName, docURI)
// }

func (s *Server) resolveComponentDef(component, funcName string, docURI uri.URI) *protocol.Location {
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	// Check cache
	cacheKey := component + "|" + baseDir
	s.mu.RLock()
	cfcPath, cached := s.resolveCache[cacheKey]
	s.mu.RUnlock()

	if !cached {
		t0 := time.Now()
		// If component is already an absolute file path, use it directly
		if filepath.IsAbs(component) {
			if _, err := s.FS.Stat(component); err == nil {
				cfcPath = component
			}
		}
		if cfcPath == "" {
			cfcPath = s.getResolver().ComponentPath(component, baseDir)
			// Try ORM cfcLocation directories for bare entity names
			if cfcPath == "" && !strings.Contains(component, ".") {
				// Check entity index first
				if entityURI := s.index.LookupEntity(component); entityURI != "" {
					cfcPath = strings.TrimPrefix(string(entityURI), "file://")
				}
				// Fall back to ORM cfcLocation directories
				if cfcPath == "" {
					if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
						for _, ormDir := range cfpath.LoadOrmLocations(appDir) {
							cfcPath = cfpath.ResolvePath(component, ormDir, nil)
							if cfcPath != "" {
								break
							}
						}
					}
				}
			}
		}
		s.mu.Lock()
		if s.resolveCache == nil {
			s.resolveCache = make(map[string]string)
		}
		s.resolveCache[cacheKey] = cfcPath
		s.mu.Unlock()
		s.logger.Debug("definition: resolve cache miss", zap.String("component", component), zap.String("path", cfcPath), zap.Duration("dur", time.Since(t0)))
	}

	if cfcPath == "" {
		s.logger.Debug("definition: component path not resolved to file", zap.String("component", component))
		return nil
	}
	for _, d := range s.getResolver().EnsureIndexed(cfcPath) {
		if strings.EqualFold(d.Name, funcName) {
			s.logger.Debug("definition: resolved method", zap.String("component", component), zap.String("func", funcName), zap.String("file", cfcPath), zap.Uint32("line", d.Line))
			return &protocol.Location{
				URI:   protocol.DocumentURI(d.URI),
				Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
			}
		}
	}
	s.logger.Debug("definition: method not found in component", zap.String("component", component), zap.String("func", funcName), zap.String("file", cfcPath))
	return nil
}

// qualifierBeforeWord returns the identifier before the dot preceding the word at cursor.
// Also handles createObject('component','path').init() by returning the component path prefixed with "~".
func qualifierBeforeWord(content string, line, char int) string {
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return ""
	}
	start := min(char, len(lineText))
	for start > 0 && isWordChar(lineText[start-1]) {
		start--
	}
	if start < 1 || lineText[start-1] != '.' {
		return ""
	}
	// Check if before the dot is ')' or ']' — could be createObject(...).func() or [].func()
	dotPos := start - 1
	if dotPos > 0 && (lineText[dotPos-1] == ')' || lineText[dotPos-1] == ']') {
		if lineText[dotPos-1] == ')' {
			prefix := lineText[:dotPos]
			lowerPrefix := strings.ToLower(prefix)
			// Try createObject
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
			// Try new
			if idx := strings.LastIndex(lowerPrefix, "new "); idx >= 0 {
				rest := strings.TrimSpace(prefix[idx+4:])
				// Strip constructor arguments: new models.Widget("x") → models.Widget
				if parenIdx := strings.IndexByte(rest, '('); parenIdx >= 0 {
					rest = rest[:parenIdx]
				}
				rest = strings.Trim(rest, "\"'")
				if rest != "" {
					return "~" + rest
				}
			}
			// Extract the call expression (e.g. getService("timetable"))
			// Find the matching open paren
			depth := 0
			i := dotPos - 1 // at ')'
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
				// Find the function name before the '('
				fnEnd := i
				fnStart := fnEnd - 1
				for fnStart >= 0 && isWordChar(lineText[fnStart]) {
					fnStart--
				}
				fnStart++
				if fnStart < fnEnd {
					// Return the call as ~?funcName(args)
					return "~?" + lineText[fnStart:dotPos]
				}
			}
		}
		// Unresolvable qualifier — still a qualified call
		return "~"
	}
	// Normal identifier before dot
	end := dotPos
	s := end - 1
	for s >= 0 && isWordChar(lineText[s]) {
		s--
	}
	s++
	if s == end {
		return ""
	}
	return lineText[s:end]
}

func wordAtPosition(content string, line, char int) string {
	lineText := lineAtOffset(content, line)
	if lineText == "" && line > 0 {
		return ""
	}
	char = min(char, len(lineText))

	start := char
	for start > 0 && isWordChar(lineText[start-1]) {
		start--
	}
	end := char
	for end < len(lineText) && isWordChar(lineText[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return lineText[start:end]
}

// cfInvokeComponentAtCursor returns the component path if the cursor is inside
// a <cfinvoke> tag's method attribute value.
func cfInvokeComponentAtCursor(content string, line, char int) string {
	tag, cursorInTag := enclosingTagAt(content, line, char)
	if tag == "" || cursorInTag < 0 {
		return ""
	}
	lower := strings.ToLower(tag)
	if !strings.HasPrefix(lower, "<cfinvoke") {
		return ""
	}

	// Check cursor is inside method="..."
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

	// Extract component attribute
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

// enclosingTagAt returns the full tag text and cursor position within it
// for the tag enclosing the given line/char. Returns ("", -1) if not inside a tag.
func enclosingTagAt(content string, line, char int) (tag string, cursorOffset int) {
	// Find cursor byte offset
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

	// Scan backwards for '<'
	tagStart := strings.LastIndex(content[:offset], "<")
	if tagStart < 0 {
		return "", -1
	}

	// Find tag end '>'
	tagEnd := strings.IndexByte(content[tagStart:], '>')
	if tagEnd < 0 {
		// Tag not yet closed — use content up to offset
		return content[tagStart:offset], offset - tagStart
	}
	tagEnd += tagStart + 1
	if offset > tagEnd {
		// Cursor is past the tag close
		return "", -1
	}
	return content[tagStart:tagEnd], offset - tagStart
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// lineAtOffset returns the text of the given 0-based line without splitting the whole content.
func lineAtOffset(content string, line int) string {
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

// componentPathAtCursor checks if the cursor is on a component dot-path in a
// recognized context (new, createObject, extends, implements, type, returntype,
// component attribute, import). Returns the full dot-path or empty string.
func componentPathAtCursor(content string, line, char int) string {
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return ""
	}

	// Extract the full dot-path at cursor (word chars + dots)
	pos := min(char, len(lineText))
	start := pos
	for start > 0 && (isWordChar(lineText[start-1]) || lineText[start-1] == '.') {
		start--
	}
	end := pos
	for end < len(lineText) && (isWordChar(lineText[end]) || lineText[end] == '.') {
		end++
	}
	if start == end {
		return ""
	}
	dotPath := lineText[start:end]
	// Must contain at least one dot or be in a quoted context to be a component path
	hasDot := strings.ContainsRune(dotPath, '.')

	// Check context before the dot-path
	before := strings.TrimRight(lineText[:start], " \t")
	lowerBefore := strings.ToLower(before)

	// new keyword: "new models.Widget"
	if strings.HasSuffix(lowerBefore, "new") {
		return dotPath
	}

	// Check if inside a quoted string in a recognized context
	if start > 0 && (lineText[start-1] == '"' || lineText[start-1] == '\'') {
		// Look for createObject("component", "path") or createObject("path")
		if strings.Contains(lowerBefore, "createobject(") {
			return dotPath
		}
		// isInstanceOf(obj, "path")
		if strings.Contains(lowerBefore, "isinstanceof(") {
			return dotPath
		}
		// import "path"
		if strings.HasSuffix(strings.TrimRight(lowerBefore, " \t\"'"), "import") {
			return dotPath
		}
	}

	// Attribute contexts: extends="path", implements="path", type="path",
	// returntype="path", component="path"
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

	// CFScript function return type: "models.Widget function" or argument type: "(models.Widget arg)"
	after := strings.TrimLeft(lineText[end:], " \t")
	lowerAfter := strings.ToLower(after)
	if hasDot && (strings.HasPrefix(lowerAfter, "function") || strings.HasPrefix(lowerAfter, "function ")) {
		return dotPath
	}
	// Argument type: check if followed by an identifier (the arg name)
	if hasDot && len(after) > 0 && isWordChar(after[0]) {
		// Verify we're inside a function signature (after a '(' or ',')
		beforeTrimmed := strings.TrimRight(lineText[:start], " \t")
		if len(beforeTrimmed) > 0 {
			lastCh := beforeTrimmed[len(beforeTrimmed)-1]
			if lastCh == '(' || lastCh == ',' {
				return dotPath
			}
			// Could also be "required type name" — check if preceded by "required"
			if strings.HasSuffix(strings.ToLower(beforeTrimmed), "required") {
				return dotPath
			}
		}
	}

	return ""
}

// resolveComponentFileDef resolves a component dot-path to a file location (line 0).
func (s *Server) resolveComponentFileDef(component string, docURI uri.URI) *protocol.Location {
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	s.logger.Debug("definition: resolveComponentFileDef", zap.String("component", component), zap.String("baseDir", baseDir))
	cfcPath := s.getResolver().ComponentPath(component, baseDir)
	if cfcPath == "" {
		return nil
	}
	return &protocol.Location{
		URI:   protocol.DocumentURI(uri.URI("file://" + cfcPath)),
		Range: protocol.Range{Start: protocol.Position{Line: 0}, End: protocol.Position{Line: 0}},
	}
}

// filePathAtCursor checks if the cursor is inside a file path attribute value
// (cfinclude template, cfmodule template, include). Returns the path or empty string.
func filePathAtCursor(content string, line, char int) string {
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return ""
	}
	pos := min(char, len(lineText))

	// Find enclosing quotes
	// Search backward for opening quote
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
	// Search forward for closing quote
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

	// Check if preceded by a recognized attribute
	before := strings.ToLower(strings.TrimRight(lineText[:qStart], " \t"))
	if strings.HasSuffix(before, "template=") || strings.HasSuffix(before, "include") ||
		strings.HasSuffix(before, "href=") || strings.HasSuffix(before, "action=") {
		return value
	}
	return ""
}

// resolveFilePathDef resolves a file path (from cfinclude etc.) to a location.
func (s *Server) resolveFilePathDef(filePath string, docURI uri.URI) *protocol.Location {
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	// Try relative to current file
	candidate := filepath.Join(baseDir, filePath)
	if _, err := s.FS.Stat(candidate); err == nil {
		return &protocol.Location{
			URI:   protocol.DocumentURI(uri.URI("file://" + candidate)),
			Range: protocol.Range{},
		}
	}

	// Try relative to Application.cfc root
	if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
		candidate = filepath.Join(appDir, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return &protocol.Location{
				URI:   protocol.DocumentURI(uri.URI("file://" + candidate)),
				Range: protocol.Range{},
			}
		}
	}

	// Try mappings — match the first path segment against mapping keys
	mappings := s.getResolver().EffectiveMappings(baseDir)
	if len(mappings) > 0 {
		clean := strings.TrimPrefix(filePath, "/")
		if seg, rest, _ := strings.Cut(clean, "/"); seg != "" {
			for key, dir := range mappings {
				if strings.EqualFold(seg, key) {
					candidate = filepath.Join(dir, rest)
					if _, err := s.FS.Stat(candidate); err == nil {
						return &protocol.Location{
							URI:   protocol.DocumentURI(uri.URI("file://" + candidate)),
							Range: protocol.Range{},
						}
					}
				}
			}
		}
	}

	// Try relative to workspace folders
	for _, root := range s.WorkspaceFolders {
		candidate = filepath.Join(root, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return &protocol.Location{
				URI:   protocol.DocumentURI(uri.URI("file://" + candidate)),
				Range: protocol.Range{},
			}
		}
	}
	return nil
}

// resolverArgAtCursor checks if the cursor is inside a string argument of a
// resolver-matched call (e.g. getService("UserService")). Returns the resolved
// component dot-path or empty string.
func (s *Server) resolverArgAtCursor(content string, line, char int) string {
	if len(s.ComponentResolvers) == 0 {
		return ""
	}
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return ""
	}
	// Find enclosing quotes around cursor
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
	// Extract the call expression surrounding the string: find opening paren before quote
	parenIdx := -1
	for i := qStart - 1; i >= 0; i-- {
		if lineText[i] == '(' {
			parenIdx = i
			break
		}
	}
	if parenIdx < 0 {
		return ""
	}
	// Get the function name before the paren
	funcEnd := parenIdx
	funcStart := funcEnd
	for funcStart > 0 && isWordChar(lineText[funcStart-1]) {
		funcStart--
	}
	if funcStart == funcEnd {
		return ""
	}
	// Build the call expression with the string value for resolver matching
	value := lineText[qStart+1 : qEnd]
	callExpr := lineText[funcStart:funcEnd] + "(\"" + value + "\")"
	return resolveComponentFromCall(callExpr, s.ComponentResolvers)
}
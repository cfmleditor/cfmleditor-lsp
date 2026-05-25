package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)
	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return reply(ctx, nil, nil)
	}

	docURI := params.TextDocument.URI
	s.log.Debug("definition: request", cflog.String("word", word), cflog.Int("line", line), cflog.Int("char", char))

	// Check if cursor is inside a resolver-matched call (e.g. getService("UserService"))
	if comp := s.resolverArgAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.log.Debug("definition: resolver arg resolved", cflog.String("component", comp))
			return reply(ctx, *loc, nil)
		}
	}

	// Check if cursor is on a component dot-path (new, createObject, extends, etc.)
	if comp := parser.ComponentPathAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.log.Debug("definition: component path resolved", cflog.String("path", comp), cflog.String("target", string(loc.URI)))
			return reply(ctx, *loc, nil)
		}
		s.log.Debug("definition: component path not resolved", cflog.String("path", comp))
	}

	// Check if cursor is on a file path (cfinclude, cfmodule template)
	if filePath := parser.FilePathAtCursor(content, line, char); filePath != "" {
		if loc := s.resolveFilePathDef(filePath, docURI); loc != nil {
			s.log.Debug("definition: file path resolved", cflog.String("path", filePath), cflog.String("target", string(loc.URI)))
			return reply(ctx, *loc, nil)
		}
		// Cursor is inside a file path — don't fall through to word-based lookup
		return reply(ctx, nil, nil)
	}

	// Check if cursor is inside a <cfinvoke> method attribute
	if comp := parser.CfInvokeComponentAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
			return reply(ctx, *loc, nil)
		}
	}

	// Check if there's a dot qualifier (e.g. persist.templateFunction)
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		s.log.Debug("definition: qualifier found", cflog.String("qualifier", qualifier), cflog.String("word", word))
		if strings.HasPrefix(qualifier, "~?") { //nolint:gocritic // if-else is clearer than switch for prefix checks
			// Call expression — try component resolvers
			callExpr := qualifier[2:]
			comp := resolveComponentFromCall(callExpr, s.ComponentResolvers)
			s.log.Debug("definition: call expression resolver", cflog.String("expr", callExpr), cflog.String("resolved", comp))
			if comp != "" {
				if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
			}
		} else if strings.HasPrefix(qualifier, "~") {
			// Direct component path from createObject/new expression
			comp := qualifier[1:]
			s.log.Debug("definition: direct component path", cflog.String("component", comp))
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
				s.log.Debug("definition: component ref found", cflog.String("variable", qualifier), cflog.String("component", ref.Component))
				if loc := s.resolveComponentDef(ref.Component, word, docURI); loc != nil {
					return reply(ctx, *loc, nil)
				}
				s.log.Debug("definition: component ref resolve failed", cflog.String("component", ref.Component), cflog.String("func", word))
			} else {
				s.log.Debug("definition: no component ref for qualifier", cflog.String("variable", qualifier))
			}
			// Try component resolvers for the qualifier itself (e.g. _parent → a CFC)
			if comp := resolveComponentFromCall(qualifier, s.ComponentResolvers); comp != "" {
				s.log.Debug("definition: qualifier matched resolver", cflog.String("qualifier", qualifier), cflog.String("resolved", comp))
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
		s.log.Debug("definition: qualified fallback to global lookup", cflog.String("word", word), cflog.Int("matches", len(defs)))
		var locations []protocol.Location
		for _, d := range defs {
			if d.URI != docURI {
				locations = append(locations, protocol.Location{
					URI:   d.URI,
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
				URI:   d.URI,
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
			URI:   d.URI,
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
		s.log.Debug("definition: resolve cache miss", cflog.String("component", component), cflog.String("path", cfcPath), cflog.Duration("dur", time.Since(t0)))
	}

	if cfcPath == "" {
		s.log.Debug("definition: component path not resolved to file", cflog.String("component", component))
		return nil
	}
	for _, d := range s.getResolver().EnsureIndexed(cfcPath) {
		if strings.EqualFold(d.Name, funcName) {
			s.log.Debug("definition: resolved method", cflog.String("component", component), cflog.String("func", funcName), cflog.String("file", cfcPath), cflog.Uint32("line", d.Line))
			return &protocol.Location{
				URI:   d.URI,
				Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
			}
		}
	}
	s.log.Debug("definition: method not found in component", cflog.String("component", component), cflog.String("func", funcName), cflog.String("file", cfcPath))
	return nil
}

// qualifierBeforeWord returns the identifier before the dot preceding the word at cursor.
// Also handles createObject('component','path').init() by returning the component path prefixed with "~".
func (s *Server) resolveComponentFileDef(component string, docURI uri.URI) *protocol.Location {
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	s.log.Debug("definition: resolveComponentFileDef", cflog.String("component", component), cflog.String("baseDir", baseDir))
	cfcPath := s.getResolver().ComponentPath(component, baseDir)
	if cfcPath == "" {
		return nil
	}
	return &protocol.Location{
		URI:   uri.URI("file://" + cfcPath),
		Range: protocol.Range{Start: protocol.Position{Line: 0}, End: protocol.Position{Line: 0}},
	}
}

// filePathAtCursor checks if the cursor is inside a file path attribute value
// (cfinclude template, cfmodule template, include). Returns the path or empty string.
// resolveFilePathDef resolves a file path (from cfinclude etc.) to a location.
func (s *Server) resolveFilePathDef(filePath string, docURI uri.URI) *protocol.Location {
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	// Try relative to current file
	candidate := filepath.Join(baseDir, filePath)
	if _, err := s.FS.Stat(candidate); err == nil {
		return &protocol.Location{
			URI:   uri.URI("file://" + candidate),
			Range: protocol.Range{},
		}
	}

	// Try relative to Application.cfc root
	if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
		candidate = filepath.Join(appDir, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return &protocol.Location{
				URI:   uri.URI("file://" + candidate),
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
							URI:   uri.URI("file://" + candidate),
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
				URI:   uri.URI("file://" + candidate),
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
	lineText := parser.LineTextAt(content, line)
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
	for funcStart > 0 && parser.IsWordChar(lineText[funcStart-1]) {
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

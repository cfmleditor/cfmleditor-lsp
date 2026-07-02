package server

import (
	"context"
	json "github.com/go-json-experiment/json"
	"path/filepath"
	"strings"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDefinition(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return nil, nil
	}

	docURI := params.TextDocument.URI

	s.log.Debug("definition: request", cflog.String("word", word), cflog.Int("line", line), cflog.Int("char", char))

	// Check if cursor is inside a resolver-matched call (e.g. getService("UserService"))
	if comp := parser.ResolverArgAtCursor(content, line, char, s.cfResolvers()); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.log.Debug("definition: resolver arg resolved", cflog.String("component", comp))

			return *loc, nil
		}
	}

	// Check if cursor is on a component dot-path (new, createObject, extends, etc.)
	if comp := parser.ComponentPathAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			s.log.Debug("definition: component path resolved", cflog.String("path", comp), cflog.String("target", string(loc.URI)))

			return *loc, nil
		}

		s.log.Debug("definition: component path not resolved", cflog.String("path", comp))
	}

	// Check if cursor is on a file path (cfinclude, cfmodule template)
	if filePath := parser.FilePathAtCursor(content, line, char); filePath != "" {
		if loc := s.resolveFilePathDef(filePath, docURI); loc != nil {
			s.log.Debug("definition: file path resolved", cflog.String("path", filePath), cflog.String("target", string(loc.URI)))

			return *loc, nil
		}
		// Cursor is inside a file path — don't fall through to word-based lookup
		return nil, nil
	}

	// Check if cursor is inside a <cfinvoke> method attribute
	if comp := parser.CfInvokeComponentAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
			return *loc, nil
		}
	}

	// Check if there's a dot qualifier (e.g. persist.templateFunction)
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		s.log.Debug("definition: qualifier found", cflog.String("qualifier", qualifier), cflog.String("word", word))

		if def := s.resolveUserFunc(qualifier, word, docURI, uint32(line)); def != nil {
			return protocol.Location{
				URI:   def.URI,
				Range: protocol.Range{Start: protocol.Position{Line: def.Line}, End: protocol.Position{Line: def.Line}},
			}, nil
		}

		// Qualified call that can't be resolved — fall through to all matches
		if !s.GlobalFunctionResolution {
			return nil, nil
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
			return locations[0], nil
		}

		if len(locations) > 1 {
			return locations, nil
		}

		return nil, nil
	}

	// No qualifier — prefer current file's definition
	defs := s.index.Lookup(word)

	for _, d := range defs {
		if d.URI == docURI {
			return protocol.Location{
				URI:   d.URI,
				Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
			}, nil
		}
	}

	// Check extends chain of current file
	if loc := s.resolveSuper(word, docURI); loc != nil {
		return *loc, nil
	}

	if len(defs) == 0 {
		return nil, nil
	}

	// Not in current file — only return if global resolution is enabled
	if !s.GlobalFunctionResolution {
		return nil, nil
	}

	var locations []protocol.Location
	for _, d := range defs {
		locations = append(locations, protocol.Location{
			URI:   d.URI,
			Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
		})
	}

	if len(locations) == 1 {
		return locations[0], nil
	}

	return locations, nil
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

	if d := s.getResolver().ResolveFunc(component, funcName, baseDir); d != nil {
		return &protocol.Location{
			URI:   d.URI,
			Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
		}
	}

	return nil
}

// resolveSuper resolves super.funcName to the parent component's function.
func (s *Server) resolveSuper(funcName string, docURI uri.URI) *protocol.Location {
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if pr == nil || pr.Extends == "" {
		return nil
	}

	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	if d := s.getResolver().ResolveFunc(pr.Extends, funcName, baseDir); d != nil {
		return &protocol.Location{
			URI:   d.URI,
			Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
		}
	}

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

package server

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strings"

	"encoding/json/v2"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDefinition(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	defer s.lockDoc(params.TextDocument.URI)()

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	// Stage timings, so a slow answer names the stage that was slow. Reported
	// only when the request was slow; see defTimer.
	dt := newDefTimer()
	outcome := "none"

	defer func() { dt.report(s.log, outcome, string(params.TextDocument.URI), line, char) }()

	// Routes are checked before the word under the cursor, because a route is a
	// dotted string that WordAtPosition reads as a fragment: the cursor inside
	// "tassweb.admin.changelogsgridview.read" yields one segment, which then
	// resolves as an unrelated component path or not at all. The whole attribute
	// value is the thing being pointed at.
	ref, isRoute := s.routeAtPosition(content, line, char)

	dt.mark("routeScan")

	if isRoute {
		if locs := s.routeDefinitions(ref); len(locs) > 0 {
			dt.mark("routeResolve")

			outcome = "route"

			s.log.Debug("definition: route resolved",
				cflog.String("route", ref.Value), cflog.Int("targets", len(locs)))

			if len(locs) == 1 {
				return locs[0], nil
			}

			return locs, nil
		}
	}

	dt.mark("routeResolve")

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return nil, nil
	}

	docURI := params.TextDocument.URI

	s.log.Debug("definition: request", cflog.String("word", word), cflog.Int("line", line), cflog.Int("char", char))

	// Check if cursor is inside a resolver-matched call (e.g. getService("UserService"))
	if comp := parser.ResolverArgAtCursor(content, line, char, s.cfResolvers()); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			outcome = "resolverArg"

			s.log.Debug("definition: resolver arg resolved", cflog.String("component", comp))

			return *loc, nil
		}
	}

	// Check if cursor is on a component dot-path (new, createObject, extends, etc.)
	dt.mark("resolverArg")

	if comp := parser.ComponentPathAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentFileDef(comp, docURI); loc != nil {
			outcome = "componentPath"

			s.log.Debug("definition: component path resolved", cflog.String("path", comp), cflog.String("target", string(loc.URI)))

			return *loc, nil
		}

		s.log.Debug("definition: component path not resolved", cflog.String("path", comp))
	}

	// Check if cursor is on a file path (cfinclude, cfmodule template)
	dt.mark("componentPath")

	if filePath := parser.FilePathAtCursor(content, line, char); filePath != "" {
		if loc := s.resolveFilePathDef(filePath, docURI); loc != nil {
			outcome = "filePath"

			s.log.Debug("definition: file path resolved", cflog.String("path", filePath), cflog.String("target", string(loc.URI)))

			return *loc, nil
		}
		// Cursor is inside a file path — don't fall through to word-based lookup
		return nil, nil
	}

	// Check if cursor is inside a <cfinvoke> method attribute
	dt.mark("filePath")

	if comp := parser.CfInvokeComponentAtCursor(content, line, char); comp != "" {
		if loc := s.resolveComponentDef(comp, word, docURI); loc != nil {
			outcome = "cfinvoke"

			return *loc, nil
		}
	}

	dt.mark("cfinvoke")

	// Variables, before the function-name paths.
	//
	// Before, because the two namespaces overlap and the cursor says which is
	// meant: `total` on its own is a variable even in a component that declares
	// a `total()`, and resolveVariableDef declines anything followed by a paren
	// so a call never reaches it. After the component and file-path checks,
	// because a scope keyword is not a component and those checks are narrower.
	varLocs := s.resolveVariableDef(content, line, char, word, docURI)

	dt.mark("variable")

	if len(varLocs) > 0 {
		locs := varLocs
		outcome = "variable"

		s.log.Debug("definition: variable resolved",
			cflog.String("word", word), cflog.String("target", string(locs[0].URI)))

		return locs[0], nil
	}

	// Check if there's a dot qualifier (e.g. persist.templateFunction)
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		defer dt.mark("qualified")

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

		// A definition in the requesting file is kept, and ranked last.
		//
		// It used to be discarded, on the reasoning that `x.doThing()` is not a
		// call to this file's own `doThing()` — which is true, and is why it
		// sorts last, but is not a reason to answer nothing. Where a name is
		// declared only here, discarding it left the one shape this fallback
		// exists for with no answer at all: an unresolvable receiver
		// (`VARIABLES._svc.`, `ARGUMENTS.a.`, a chain, a bracket index) on a
		// method this component also declares. Every one of those returned nil
		// while the name sat in the index.
		//
		// Last rather than first because the qualifier is evidence against it:
		// nearest-first would otherwise put it at the top, since nothing is
		// nearer than the same file, and `myObj.init()` would jump to this
		// component's own `init()` ahead of a real candidate elsewhere.
		var locations, sameFile []protocol.Location

		for _, d := range defs {
			loc := protocol.Location{
				URI:   d.URI,
				Range: protocol.Range{Start: protocol.Position{Line: d.Line}, End: protocol.Position{Line: d.Line}},
			}

			if d.URI == docURI {
				sameFile = append(sameFile, loc)

				continue
			}

			locations = append(locations, loc)
		}

		// Nearest first, and the lowest URI among equals. The editor lists
		// these in the order they are returned, and that order used to be the
		// index's bucket order — the order eight parallel indexing goroutines
		// finished in, so the same "3 definitions found" list could come back
		// differently after a restart. Nearest is also the more useful first
		// entry, since it is the one the caller most likely meant.
		orderByNearest(locations, docURI)

		locations = append(locations, sameFile...)

		if len(locations) == 1 {
			return locations[0], nil
		}

		if len(locations) > 1 {
			return locations, nil
		}

		return nil, nil
	}

	// No qualifier — prefer current file's definition
	defer dt.mark("unqualified")

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
	currentPath := docURI.Path()
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

	currentPath := docURI.Path()
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
	currentPath := docURI.Path()
	baseDir := filepath.Dir(currentPath)

	s.log.Debug("definition: resolveComponentFileDef", cflog.String("component", component), cflog.String("baseDir", baseDir))

	cfcPath := s.getResolver().ComponentPath(component, baseDir)
	if cfcPath == "" {
		return nil
	}

	return &protocol.Location{
		URI:   cfpath.ToURI(cfcPath),
		Range: protocol.Range{Start: protocol.Position{Line: 0}, End: protocol.Position{Line: 0}},
	}
}

// filePathAtCursor checks if the cursor is inside a file path attribute value
// (cfinclude template, cfmodule template, include). Returns the path or empty string.
// resolveFilePathDef resolves a file path (from cfinclude etc.) to a location.
func (s *Server) resolveFilePathDef(filePath string, docURI uri.URI) *protocol.Location {
	currentPath := docURI.Path()
	baseDir := filepath.Dir(currentPath)

	// Try relative to current file
	candidate := filepath.Join(baseDir, filePath)
	if _, err := s.FS.Stat(candidate); err == nil {
		return &protocol.Location{
			URI:   cfpath.ToURI(candidate),
			Range: protocol.Range{},
		}
	}

	// Try relative to Application.cfc root
	if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
		candidate = filepath.Join(appDir, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return &protocol.Location{
				URI:   cfpath.ToURI(candidate),
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
							URI:   cfpath.ToURI(candidate),
							Range: protocol.Range{},
						}
					}
				}
			}
		}
	}

	// Try relative to workspace folders
	for _, root := range s.searchRoots() {
		candidate = filepath.Join(root, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return &protocol.Location{
				URI:   cfpath.ToURI(candidate),
				Range: protocol.Range{},
			}
		}
	}

	return nil
}

// orderByNearest sorts locations by directory distance from docURI, then by
// URI, so the list a client shows is stable across sessions rather than
// following whatever order the index happened to hold.
func orderByNearest(locations []protocol.Location, docURI uri.URI) {
	ref := string(docURI)

	slices.SortFunc(locations, func(a, b protocol.Location) int {
		if d := cmp.Compare(cfpath.URIDistance(ref, string(a.URI)), cfpath.URIDistance(ref, string(b.URI))); d != 0 {
			return d
		}

		if d := cmp.Compare(a.URI, b.URI); d != 0 {
			return d
		}

		return cmp.Compare(a.Range.Start.Line, b.Range.Start.Line)
	})
}

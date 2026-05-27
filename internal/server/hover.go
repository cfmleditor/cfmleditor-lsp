package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
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

	// Builtin function
	// User-defined function via qualifier (e.g. service.getMethod) — check first
	docURI := params.TextDocument.URI
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		if def := s.resolveUserFunc(qualifier, word, docURI, uint32(line)); def != nil {
			return reply(ctx, &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: def.FormatHover(),
				},
			}, nil)
		}

		if !s.GlobalFunctionResolution {
			return reply(ctx, nil, nil)
		}
	}

	// Builtin function
	if e, ok := docs.LookupFunction(word); ok {
		return reply(ctx, &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("**%s**\n\n```cfml\n%s\n```\n\n%s", e.Name, e.Syntax, e.Doc()),
			},
		}, nil)
	}

	// Builtin tag
	if e, ok := docs.LookupTag(word); ok {
		return reply(ctx, &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("**<%s>**\n\n%s", e.Name, e.Doc()),
			},
		}, nil)
	}

	// User-defined function in current file or index (unqualified)
	defs := s.index.Lookup(word)
	if len(defs) > 0 {
		// Only show if in current file or (global resolution enabled + exactly one match)
		var def *parser.FunctionDef

		for _, d := range defs {
			if d.URI == docURI {
				def = d

				break
			}
		}

		if def == nil && s.GlobalFunctionResolution && len(defs) == 1 {
			def = defs[0]
		}

		if def != nil {
			return reply(ctx, &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: def.FormatHover(),
				},
			}, nil)
		}
	}

	return reply(ctx, nil, nil)
}

func (s *Server) resolveUserFunc(qualifier, funcName string, docURI uri.URI, line uint32) *parser.FunctionDef {
	key := fmt.Sprintf("%s:%d:%s.%s", docURI, line, qualifier, funcName)

	s.mu.RLock()

	if s.lastResolveKey == key {
		def := s.lastResolveDef
		s.mu.RUnlock()

		return def
	}

	s.mu.RUnlock()

	def := s.doResolveUserFunc(qualifier, funcName, docURI, line)

	s.mu.Lock()
	s.lastResolveKey = key
	s.lastResolveDef = def
	s.mu.Unlock()

	return def
}

func (s *Server) doResolveUserFunc(qualifier, funcName string, docURI uri.URI, line uint32) *parser.FunctionDef {
	var comp string

	switch {
	case strings.EqualFold(qualifier, "super"):
		// Resolve from parent component
		s.mu.RLock()
		pr := s.parseResults[docURI]
		s.mu.RUnlock()

		if pr == nil || pr.Extends == "" {
			return nil
		}

		comp = pr.Extends
	case strings.HasPrefix(qualifier, "~?"):
		comp = parser.ResolveFromCall(qualifier[2:], s.cfResolvers())
	case strings.HasPrefix(qualifier, "~"):
		comp = qualifier[1:]
	default:
		ref := s.index.LookupComponentRefInFile(qualifier, docURI, line)
		if ref == nil {
			s.ensureFuncRefsIndexed(docURI, int(line))
			ref = s.index.LookupComponentRefInFile(qualifier, docURI, line)
		}

		if ref != nil {
			comp = ref.Component
		} else {
			comp = parser.ResolveFromCall(qualifier, s.cfResolvers())
		}
	}

	if comp == "" {
		return nil
	}

	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)

	return s.getResolver().ResolveFunc(comp, funcName, baseDir)
}

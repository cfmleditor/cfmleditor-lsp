package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
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
					Value: formatFuncHover(def),
				},
			}, nil)
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
					Value: formatFuncHover(def),
				},
			}, nil)
		}
	}

	return reply(ctx, nil, nil)
}

func (s *Server) resolveUserFunc(qualifier, funcName string, docURI uri.URI, line uint32) *parser.FunctionDef {
	var comp string
	switch {
	case strings.HasPrefix(qualifier, "~?"):
		comp = resolveComponentFromCall(qualifier[2:], s.ComponentResolvers)
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
			comp = resolveComponentFromCall(qualifier, s.ComponentResolvers)
		}
	}
	if comp == "" {
		return nil
	}
	currentPath := strings.TrimPrefix(string(docURI), "file://")
	baseDir := filepath.Dir(currentPath)
	cfcPath := s.getResolver().ComponentPath(comp, baseDir)
	if cfcPath == "" {
		return nil
	}
	for _, d := range s.getResolver().EnsureIndexed(cfcPath) {
		if strings.EqualFold(d.Name, funcName) {
			return d
		}
	}
	return nil
}

func formatFuncHover(def *parser.FunctionDef) string {
	var b strings.Builder
	b.WriteString("**")
	b.WriteString(def.Name)
	b.WriteString("**\n\n```cfml\n")
	b.WriteString(def.Name)
	b.WriteString("(")
	for i, arg := range def.Arguments {
		if i > 0 {
			b.WriteString(", ")
		}
		if arg.Required {
			b.WriteString("required ")
		}
		if arg.Type != "" {
			b.WriteString(arg.Type)
			b.WriteString(" ")
		}
		b.WriteString(arg.Name)
	}
	b.WriteString(")\n```")
	return b.String()
}

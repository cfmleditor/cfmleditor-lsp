package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleSignatureHelp(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.SignatureHelpParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.log.Debug("signatureHelp: request",
		cflog.String("uri", string(params.TextDocument.URI)),
		cflog.Uint32("line", params.Position.Line),
		cflog.Uint32("char", params.Position.Character))

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	funcName, qualifier, activeParam := parser.FindCallContext(content, line, char)
	if funcName == "" {
		return reply(ctx, nil, nil)
	}

	// Try builtin functions first
	if e, ok := docs.LookupFunction(funcName); ok {
		sig := buildBuiltinSignature(e)
		sig.ActiveParameter = uint32(activeParam)
		return reply(ctx, &protocol.SignatureHelp{
			Signatures:      []protocol.SignatureInformation{sig},
			ActiveSignature: 0,
			ActiveParameter: uint32(activeParam),
		}, nil)
	}

	docURI := uri.URI(params.TextDocument.URI)

	// Try resolving via qualifier (e.g. service.method or getService("x").method)
	if qualifier != "" {
		if def := s.resolveUserFunc(qualifier, funcName, docURI, uint32(line)); def != nil {
			return reply(ctx, &protocol.SignatureHelp{
				Signatures:      []protocol.SignatureInformation{buildUserSignature(def)},
				ActiveSignature: 0,
				ActiveParameter: uint32(activeParam),
			}, nil)
		}
	}

	// Try user-defined functions from the index
	defs := s.index.Lookup(funcName)
	if len(defs) == 0 {
		return reply(ctx, nil, nil)
	}

	// Prefer current file
	def := defs[0]
	for _, d := range defs {
		if d.URI == docURI {
			def = d
			break
		}
	}

	return reply(ctx, &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{buildUserSignature(def)},
		ActiveSignature: 0,
		ActiveParameter: uint32(activeParam),
	}, nil)
}

func buildUserSignature(def *parser.FunctionDef) protocol.SignatureInformation {
	label := def.Name + "("
	var paramInfos []protocol.ParameterInformation
	for i, arg := range def.Arguments {
		if i > 0 {
			label += ", "
		}
		paramLabel := ""
		if arg.Required {
			paramLabel += "required "
		}
		if arg.Type != "" {
			paramLabel += arg.Type + " "
		}
		paramLabel += arg.Name
		label += paramLabel
		paramInfos = append(paramInfos, protocol.ParameterInformation{Label: paramLabel})
	}
	label += ")"
	return protocol.SignatureInformation{Label: label, Parameters: paramInfos}
}

func buildBuiltinSignature(e *docs.Entry) protocol.SignatureInformation {
	var paramInfos []protocol.ParameterInformation
	label := e.Name + "("
	for i, p := range e.Params {
		if i > 0 {
			label += ", "
		}
		paramLabel := p.Name
		if p.Type != "" {
			paramLabel = p.Type + " " + p.Name
		}
		label += paramLabel
		doc := p.Description
		if len(p.Values) > 0 {
			doc += fmt.Sprintf(" (values: %s)", strings.Join(p.Values, ", "))
		}
		paramInfos = append(paramInfos, protocol.ParameterInformation{
			Label:         paramLabel,
			Documentation: doc,
		})
	}
	label += ")"

	return protocol.SignatureInformation{
		Label:         label,
		Documentation: &protocol.MarkupContent{Kind: protocol.Markdown, Value: e.Doc()},
		Parameters:    paramInfos,
	}
}

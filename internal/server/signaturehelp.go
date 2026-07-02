package server

import (
	"context"
	"fmt"
	json "github.com/go-json-experiment/json"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

func (s *Server) handleSignatureHelp(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.SignatureHelpParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	s.log.Debug("signatureHelp: request",
		cflog.String("uri", string(params.TextDocument.URI)),
		cflog.Uint32("line", params.Position.Line),
		cflog.Uint32("char", params.Position.Character))

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	funcName, qualifier, activeParam := parser.FindCallContext(content, line, char)
	if funcName == "" {
		return nil, nil
	}

	var activeSignature uint32

	// Try builtin functions first
	if e, ok := docs.LookupFunction(funcName); ok {
		sig := buildBuiltinSignature(e)
		sig.ActiveParameter = protocol.NewNullable(uint32(activeParam))

		return &protocol.SignatureHelp{
			Signatures:      []protocol.SignatureInformation{sig},
			ActiveSignature: &activeSignature,
			ActiveParameter: protocol.NewNullable(uint32(activeParam)),
		}, nil
	}

	docURI := params.TextDocument.URI

	// Try resolving via qualifier (e.g. service.method or getService("x").method)
	if qualifier != "" {
		if def := s.resolveUserFunc(qualifier, funcName, docURI, uint32(line)); def != nil {
			return &protocol.SignatureHelp{
				Signatures:      []protocol.SignatureInformation{buildUserSignature(def)},
				ActiveSignature: &activeSignature,
				ActiveParameter: protocol.NewNullable(uint32(activeParam)),
			}, nil
		}
	}

	// Try user-defined functions from the index
	defs := s.index.Lookup(funcName)
	if len(defs) == 0 {
		return nil, nil
	}

	// Prefer current file
	def := defs[0]

	for _, d := range defs {
		if d.URI == docURI {
			def = d

			break
		}
	}

	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{buildUserSignature(def)},
		ActiveSignature: &activeSignature,
		ActiveParameter: protocol.NewNullable(uint32(activeParam)),
	}, nil
}

func buildUserSignature(def *parser.FunctionDef) protocol.SignatureInformation {
	var label strings.Builder
	label.WriteString(def.Name)
	label.WriteByte('(')

	paramInfos := make([]protocol.ParameterInformation, 0, len(def.Arguments))

	for i, arg := range def.Arguments {
		if i > 0 {
			label.WriteString(", ")
		}

		paramLabel := ""
		if arg.Required {
			paramLabel += "required "
		}

		if arg.Type != "" {
			paramLabel += arg.Type + " "
		}

		paramLabel += arg.Name
		label.WriteString(paramLabel)
		paramInfos = append(paramInfos, protocol.ParameterInformation{Label: protocol.String(paramLabel)})
	}

	label.WriteString(")")

	return protocol.SignatureInformation{Label: label.String(), Parameters: paramInfos}
}

func buildBuiltinSignature(e *docs.Entry) protocol.SignatureInformation {
	paramInfos := make([]protocol.ParameterInformation, 0, len(e.Params))

	var label strings.Builder
	label.WriteString(e.Name)
	label.WriteByte('(')

	for i, p := range e.Params {
		if i > 0 {
			label.WriteString(", ")
		}

		paramLabel := p.Name
		if p.Type != "" {
			paramLabel = p.Type + " " + p.Name
		}

		label.WriteString(paramLabel)

		doc := p.Description
		if len(p.Values) > 0 {
			doc += fmt.Sprintf(" (values: %s)", strings.Join(p.Values, ", "))
		}

		paramInfos = append(paramInfos, protocol.ParameterInformation{
			Label:         protocol.String(paramLabel),
			Documentation: tooltip(doc),
		})
	}

	label.WriteString(")")

	return protocol.SignatureInformation{
		Label:         label.String(),
		Documentation: &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: e.Doc()},
		Parameters:    paramInfos,
	}
}

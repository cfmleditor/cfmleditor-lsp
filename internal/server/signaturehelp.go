package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
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

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	funcName, qualifier, activeParam := findCallContext(content, line, char)
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

// findCallContext finds the function name being called at the cursor position
// and which parameter the cursor is on (0-based). Also returns the full qualifier.
func findCallContext(content string, line, char int) (funcName string, qualifier string, activeParam int) {
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return "", "", 0
	}
	pos := min(char, len(lineText))

	// Walk backwards from cursor to find the opening paren
	depth := 0
	commas := 0
	i := pos - 1
	for i >= 0 {
		ch := lineText[i]
		switch ch {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				// Found the opening paren — extract function name before it
				end := i
				start := end - 1
				for start >= 0 && (isWordChar(lineText[start]) || lineText[start] == '.') {
					start--
				}
				start++
				name := lineText[start:end]
				if dotIdx := strings.LastIndexByte(name, '.'); dotIdx >= 0 {
					qual := name[:dotIdx]
					funcN := name[dotIdx+1:]
					// If qualifier is empty, check for call expression before the dot
					if qual == "" && start > 0 && lineText[start-1] == ')' {
						j := start - 1
						parenDepth := 0
						for j >= 0 {
							switch lineText[j] {
							case ')':
								parenDepth++
							case '(':
								parenDepth--
								if parenDepth == 0 {
									fnStart := j - 1
									for fnStart >= 0 && isWordChar(lineText[fnStart]) {
										fnStart--
									}
									fnStart++
									return funcN, lineText[fnStart:start], commas
								}
							}
							j--
						}
					}
					return funcN, qual, commas
				}
				return name, "", commas
			}
			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}
		i--
	}
	return "", "", 0
}

func buildUserSignature(def *cfparser.FunctionDef) protocol.SignatureInformation {
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
		Label:      label,
		Documentation: &protocol.MarkupContent{Kind: protocol.Markdown, Value: e.Doc()},
		Parameters: paramInfos,
	}
}

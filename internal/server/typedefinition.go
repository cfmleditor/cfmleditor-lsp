package server

import (
	"context"
	"encoding/json/v2"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// handleTypeDefinition answers textDocument/typeDefinition: the declaration of
// the component the symbol under the cursor holds, rather than the symbol's
// own declaration. On a variable, argument or property that is the component
// it was assigned or typed as; on a call, the component the function's return
// type names.
//
// It is the VS Code extension's CFMLTypeDefinitionProvider, which the
// extension stands down while this server runs. What a variable holds comes
// from resolve.ComponentOf, the lookup call resolution already makes, so
// go-to-type-definition and `unresolved` cannot disagree about a receiver.
// A framework's bundled API is never the answer (see stubs.go).
func (s *Server) handleTypeDefinition(ctx context.Context, rawParams []byte) (any, error) {
	v, err := s.typeDefinitionAnswer(ctx, rawParams)

	return withoutStubLocations(v), err
}

func (s *Server) typeDefinitionAnswer(_ context.Context, rawParams []byte) (any, error) {
	if !s.Features.TypeDefinition {
		return nil, nil
	}

	var params protocol.TypeDefinitionParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	content, ok := s.getDocument(docURI)
	if !ok {
		return nil, nil
	}

	line := int(params.Position.Line)
	char := byteCol(content, line, params.Position.Character)

	comp := s.typeAt(content, line, char, docURI)
	if comp == "" {
		return nil, nil
	}

	loc := s.resolveComponentFileDef(comp, docURI)
	if loc == nil {
		return nil, nil
	}

	return *loc, nil
}

// typeAt is the component the symbol at line and char holds, or "".
func (s *Server) typeAt(content string, line, char int, docURI uri.URI) string {
	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return ""
	}

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if pr == nil {
		return ""
	}

	qualifier := parser.QualifierBeforeWord(content, line, char)

	if followedByParen(parser.LineTextAt(content, line), char) {
		return s.returnComponent(qualifier, word, docURI, conv.Uint32(line), pr)
	}

	variable := word

	switch {
	case qualifier == "":
	case isVariableScope(qualifier):
		variable = qualifier + "." + word
	default:
		// A member of some other value, such as `user.address`: what that
		// holds is not recorded anywhere.
		return ""
	}

	baseDir := filepath.Dir(docURI.Path())

	return s.getResolver().ComponentOf(variable, conv.Uint32(line), pr, baseDir)
}

// returnComponent is the component the function a call names returns, or "".
func (s *Server) returnComponent(qualifier, name string, docURI uri.URI, line uint32, pr *parser.ParseResult) string {
	var def *parser.FunctionDef

	switch {
	case qualifier != "" && !strings.EqualFold(qualifier, "this"):
		def = s.resolveUserFunc(qualifier, name, docURI, line)
	default:
		for i := range pr.Funcs {
			if strings.EqualFold(pr.Funcs[i].Name, name) {
				def = &pr.Funcs[i]

				break
			}
		}

		if def == nil && pr.Extends != "" {
			def = s.getResolver().ResolveFunc(pr.Extends, name, filepath.Dir(docURI.Path()))
		}
	}

	if def == nil {
		return ""
	}

	if def.ReturnComponent != "" && def.ReturnComponent != "$any" {
		return def.ReturnComponent
	}

	if isComponentType(def.ReturnType) {
		return def.ReturnType
	}

	return ""
}

// followedByParen reports whether the identifier at col in text is followed,
// after any spaces, by an opening parenthesis: whether it is being called.
func followedByParen(text string, col int) bool {
	i := min(max(col, 0), len(text))
	for i < len(text) && isIdentByte(text[i]) {
		i++
	}

	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}

	return i < len(text) && text[i] == '('
}

// isVariableScope reports whether q is a scope a component can be stored in.
func isVariableScope(q string) bool {
	switch strings.ToLower(q) {
	case "local", "variables", "this", "arguments", "request", "session", "application", "server":
		return true
	}

	return false
}

// isComponentType reports whether a declared type names a component rather
// than one of CFML's own types. Only a dotted path is taken as one: a bare
// word may be a component in the same directory, but it is far more often a
// type such as "struct", and a wrong answer is worse than none.
func isComponentType(t string) bool {
	return strings.Contains(t, ".") && !strings.ContainsAny(t, " []")
}

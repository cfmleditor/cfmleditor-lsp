package resolve

import (
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
)

func (r *Resolver) expressionReturn(fd *parser.FunctionDef, expression, baseDir, component string) string {
	if ret := r.producerExpressionReturn(fd, expression, baseDir, component); ret != "" {
		return ret
	}

	if ret := r.absentArgumentComponent(fd, expression, baseDir, component); ret != "" {
		return ret
	}

	return r.wheelsFactoryReturn(fd, expression, baseDir)
}

// Keep conditional returns out of the unconditional method contract. Only a
// complete zero-argument call selects this source-backed, side-effect-free path.
func (r *Resolver) absentArgumentComponent(fd *parser.FunctionDef, expression, baseDir, component string) string {
	if fd == nil || !fd.URI.IsFile() {
		return ""
	}

	switch strings.ToLower(fd.ReturnType) {
	case "", "any", "component", "object":
	default:
		return ""
	}

	scanner := parser.NewScanner(expression)
	for scanner.PeekSkipComments().Kind != parser.TokLParen {
		if scanner.NextSkipComments().Kind == parser.TokEOF {
			return ""
		}
	}

	scanner.NextSkipComments()

	if scanner.NextSkipComments().Kind != parser.TokRParen || scanner.NextSkipComments().Kind != parser.TokEOF {
		return ""
	}

	field := r.wheelsSource(fd.URI.Path()).methods[strings.ToLower(fd.Name)].noArgsReturn
	if field == "" || r.ResolveFunc(component, "structKeyExists", baseDir) != nil {
		return ""
	}

	return r.startupComponent(field, baseDir, nil)
}

// Recognize only literal local initialization followed by a guard on an absent
// optional parameter. The unreachable branch may contain arbitrary work; the
// selected branch must directly return one shared-scope field. Defaults,
// required parameters, mutations, nested returns and trailing work are rejected.
func absentArgumentReturn(params [][]parser.Token, body []parser.Token) string {
	if len(body) > 2048 {
		return ""
	}

	names := optionalParameterNames(params)
	if len(names) == 0 {
		return ""
	}

	body = skipLiteralLocals(body)
	if len(body) < 12 || !tokenValues(body[:6], "if", "(", "structkeyexists", "(", "arguments", ",") || body[6].Kind != parser.TokString {
		return ""
	}

	name := strings.ToLower(strings.Trim(body[6].Value, "\"'"))
	if !names[name] || !tokenValues(body[7:10], ")", ")", "{") {
		return ""
	}

	body = afterGuardBlock(body[10:])
	if len(body) < 7 || !tokenValues(body[:3], "else", "{", "return") {
		return ""
	}

	body = body[3:]
	if len(body) < 4 || body[0].Kind != parser.TokIdent || body[1].Kind != parser.TokDot || body[2].Kind != parser.TokIdent {
		return ""
	}

	field := body[0].Value + "." + body[2].Value

	body = body[3:]
	if len(body) > 0 && body[0].Kind == parser.TokSemicolon {
		body = body[1:]
	}

	if len(body) != 1 || body[0].Kind != parser.TokRBrace {
		return ""
	}

	if _, ok := splitSharedScope(field); !ok {
		return ""
	}

	return field
}

func optionalParameterNames(params [][]parser.Token) map[string]bool {
	names := map[string]bool{}

	for _, param := range params {
		// Only unadorned names or a type followed by a name are admitted.
		if len(param) < 1 || len(param) > 2 {
			return nil
		}

		for _, tok := range param {
			if tok.Kind != parser.TokIdent || strings.EqualFold(tok.Value, "required") {
				return nil
			}
		}

		names[strings.ToLower(param[len(param)-1].Value)] = true
	}

	return names
}

func skipLiteralLocals(body []parser.Token) []parser.Token {
	for len(body) >= 5 && strings.EqualFold(body[0].Value, "var") && body[1].Kind == parser.TokIdent && body[2].Kind == parser.TokEquals && body[4].Kind == parser.TokSemicolon {
		literal := body[3]

		name := strings.ToLower(body[1].Value)
		if name == "arguments" || name == "structkeyexists" || name == "application" || name == "session" || name == "server" || name == "request" || strings.Contains(literal.Value, "#") {
			break
		}

		if literal.Kind != parser.TokString && literal.Kind != parser.TokNumber && !strings.EqualFold(literal.Value, "true") && !strings.EqualFold(literal.Value, "false") {
			break
		}

		body = body[5:]
	}

	return body
}

func afterGuardBlock(body []parser.Token) []parser.Token {
	depth := 1

	for i, tok := range body {
		switch tok.Kind {
		case parser.TokLBrace:
			depth++
		case parser.TokRBrace:
			depth--
		default:
			// Other tokens do not change block depth.
		}

		if depth == 0 {
			return body[i+1:]
		}
	}

	return nil
}

func tokenValues(tokens []parser.Token, values ...string) bool {
	if len(tokens) != len(values) {
		return false
	}

	for i, tok := range tokens {
		if tok.Kind == parser.TokString || !strings.EqualFold(tok.Value, values[i]) {
			return false
		}
	}

	return true
}

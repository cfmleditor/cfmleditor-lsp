package parser

import (
	"regexp"
	"strings"
)

// StartupBeanBinding describes a finite literal service-list assignment. The
// resolver must verify the factory before using any resulting bean identity.
type StartupBeanBinding struct{ Variable, Factory, Bean string }

var (
	serviceLoopHeader  = regexp.MustCompile(`(?is)^for\s*\(\s*((?:variables\.|local\.)?[\w$]+)\s+in\s+listToArray\s*\(\s*([^()]+)\s*\)\s*\)\s*\{`)
	serviceMemberWrite = regexp.MustCompile(`(?i)^(application|request|session|server)\s*\[\s*["']#\s*([\w.$]+)\s*#["']\s*\]\s*=\s*([\w.$]+)\.getBean\s*\(\s*["']#\s*([\w.$]+)\s*#["']\s*\)\s*;`)
)

// StartupBeanBindings accepts only complete listToArray loops with a known
// literal list, identical key/bean interpolation and an unchanged iterator.
func StartupBeanBindings(content string) []StartupBeanBinding {
	env := map[string]string{}

	var out []StartupBeanBinding

	sc := NewScanner(content)
	previous := TokEOF

	for {
		t := sc.NextSkipComments()
		before := previous

		previous = t.Kind
		if t.Kind == TokEOF {
			return out
		}

		if t.Kind != TokIdent || before == TokDot {
			continue
		}

		if identEq(t.Value, "for") {
			m := serviceLoopHeader.FindStringSubmatch(content[t.Offset:])
			if m == nil {
				continue
			}

			list, ok := stringLiteral(strings.TrimSpace(m[2]))
			if !ok {
				list, ok = env[envName(strings.TrimSpace(m[2]))]
			}

			if !ok || strings.Contains(list, "#") {
				continue
			}

			values := strings.Split(list, ",")
			if len(values) > 64 {
				continue
			}

			bodyStart := t.Offset + len(m[0])

			bodyEnd, valid := serviceLoopEnd(content, bodyStart)
			if !valid {
				continue
			}

			iterator := strings.ToLower(m[1])

			bindings, valid := serviceLoopBody(content[bodyStart:bodyEnd], iterator, values)
			if valid {
				out = append(out, bindings...)
			}

			continue
		}

		cursor := *sc
		cursor.Restore(ScannerState{pos: t.Offset, line: t.Line})

		target, key, ok := mappingSourceTarget(&cursor)
		if !ok || key != "" || cursor.NextSkipComments().Kind != TokEquals || cursor.PeekSkipComments().Kind == TokEquals {
			continue
		}

		expr := mappingSourceExpression(&cursor)
		if value, ok := stringLiteral(expr); ok {
			env[envName(target)] = value
		} else if value, ok := listAppendLiteral(expr, target, env); ok {
			env[envName(target)] = value
		} else {
			delete(env, envName(target))
		}
	}
}

func serviceLoopEnd(content string, start int) (int, bool) {
	sc := NewScanner(content[start:])
	depth := 1

	for {
		t := sc.NextSkipComments()
		switch t.Kind {
		case TokEOF:
			return 0, false
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
			if depth == 0 {
				return start + t.Offset, true
			}
		default:
		}
	}
}

func serviceLoopBody(body, iterator string, values []string) ([]StartupBeanBinding, bool) {
	sc := NewScanner(body)

	var out []StartupBeanBinding

	previous := TokEOF

	for {
		t := sc.NextSkipComments()
		before := previous

		previous = t.Kind
		if t.Kind == TokEOF {
			return out, true
		}

		if t.Kind != TokIdent || before == TokDot {
			continue
		}

		if identEq(t.Value, "for") || identEq(t.Value, "function") {
			return nil, false
		}

		cursor := *sc
		cursor.Restore(ScannerState{pos: t.Offset, line: t.Line})

		if target, _, ok := mappingSourceTarget(&cursor); ok && strings.EqualFold(target, iterator) && serviceIteratorMutation(&cursor) {
			return nil, false
		}

		m := serviceMemberWrite.FindStringSubmatch(body[t.Offset:])
		if m == nil {
			continue
		}

		if !strings.EqualFold(m[2], iterator) || !strings.EqualFold(m[4], iterator) {
			return nil, false
		}

		for _, value := range values {
			value = strings.TrimSpace(value)
			if !isDottedName(value) || strings.Contains(value, ".") {
				return nil, false
			}

			out = append(out, StartupBeanBinding{Variable: strings.ToLower(m[1]) + "." + value, Factory: m[3], Bean: value})
		}
	}
}

func serviceIteratorMutation(sc *Scanner) bool {
	t := sc.NextSkipComments()
	if t.Kind == TokEquals {
		return sc.PeekSkipComments().Kind != TokEquals
	}

	return (t.Kind == TokPlus || t.Kind == TokMinus || t.Kind == TokStar || t.Kind == TokSlash || t.Kind == TokAmpersand) && (sc.PeekSkipComments().Kind == TokEquals || sc.PeekSkipComments().Kind == t.Kind)
}

// StartupVariableAssignment records scalar variables-scope writes, including
// unknown replacements. It does not expose function-local names globally.
type StartupVariableAssignment struct{ Variable, Expression string }

// StartupVariableAssignments returns the scalar variables-scope writes in this source.
func (pr *ParseResult) StartupVariableAssignments() []StartupVariableAssignment {
	var out []StartupVariableAssignment

	for _, w := range pr.collectionWrites() {
		scope, name, ok := strings.Cut(w.target, ".")
		if !ok || !strings.EqualFold(scope, "variables") || name == "" || strings.Contains(name, ".") {
			continue
		}

		expression := w.expression
		if w.element || w.unknown {
			expression = ""
		}

		out = append(out, StartupVariableAssignment{Variable: w.target, Expression: expression})
	}

	return out
}

// serviceListAppend is `list = listAppend( list, "name" )`.
var serviceListAppend = regexp.MustCompile(`(?is)^listAppend\s*\(\s*((?:variables\.|local\.)?[\w$]+)\s*,\s*(["'][^"'#]*["'])\s*\)$`)

// listAppendLiteral is what a known literal list may hold after expr appends
// a literal to it: every name it held and the appended one. A loop over it
// then binds the appended name too, which is right whether or not the append
// ran: a binding says what the loop assigns that name, not that it does. It
// is how Mura adds a legacy service under a condition before its loop.
func listAppendLiteral(expr, target string, env map[string]string) (string, bool) {
	m := serviceListAppend.FindStringSubmatch(strings.TrimSpace(expr))
	if m == nil || envName(m[1]) != envName(target) {
		return "", false
	}

	list, known := env[envName(target)]
	if !known {
		return "", false
	}

	item, ok := stringLiteral(m[2])
	if !ok || item == "" || strings.Contains(item, ",") {
		return "", false
	}

	return list + "," + item, true
}

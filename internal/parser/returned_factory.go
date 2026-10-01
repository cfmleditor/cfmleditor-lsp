package parser

import "strings"

// scriptReturnExpression reads without moving the body's scanner or recording
// calls twice. A slice of the original source keeps nested arguments intact.
func scriptReturnExpression(scanner *Scanner) string {
	sc := *scanner
	start := sc.PeekSkipComments().Offset
	depth := 0

	for {
		t := sc.NextSkipComments()
		if t.Kind == TokEOF || depth == 0 && (t.Kind == TokSemicolon || t.Kind == TokRBrace) {
			return strings.TrimSpace(sc.src[start:t.Offset])
		}

		switch t.Kind {
		case TokLParen, TokLBracket, TokLBrace:
			depth++
		case TokRParen, TokRBracket, TokRBrace:
			depth--
		default:
			// Only delimiters affect expression nesting.
		}
	}
}

// factoryReturnChain accepts only a whole factory call followed by method
// calls. Operators, indexing and member reads produce different values.
func factoryReturnChain(expr string) (root string, methods []string) {
	sc := NewScanner(expr)
	if sc.NextSkipComments().Kind != TokIdent {
		return "", nil
	}

	for {
		if sc.NextSkipComments().Kind != TokLParen {
			return "", nil
		}

		depth := 1
		for depth > 0 {
			t := sc.NextSkipComments()
			switch t.Kind {
			case TokEOF:
				return "", nil
			case TokLParen:
				depth++
			case TokRParen:
				depth--
			default:
				// Arguments may contain any other token.
			}
		}

		if root == "" {
			root = expr[:sc.pos]
		}

		switch sc.NextSkipComments().Kind {
		case TokEOF:
			return root, methods
		case TokDot:
			t := sc.NextSkipComments()
			if t.Kind != TokIdent {
				return "", nil
			}

			methods = append(methods, t.Value)
		default:
			return "", nil
		}
	}
}

// applyFactoryReturnCalls follows configured factory identities through their
// methods. Every return path must agree; a branch returning a string, an
// unknown expression or a different component prevents a concrete answer.
func (pr *ParseResult) applyFactoryReturnCalls(calls []pendingCall) {
	if len(pr.Resolvers) == 0 {
		return
	}

	byFunction := make(map[string][]string)

	for i := range calls {
		if c := &calls[i]; c.returnExpr {
			byFunction[c.funcKey] = append(byFunction[c.funcKey], c.varName)
		}
	}

	scopeKeys := make(map[int]string, len(pr.Scopes))
	for _, scope := range pr.Scopes {
		scopeKeys[scope.Start] = funcKey(scope.Start, scope.End)
	}

	for i := range pr.Funcs {
		f := &pr.Funcs[i]

		key, ok := scopeKeys[int(f.Line)]
		if !ok {
			continue
		}

		var answer string

		candidate, uncertain := false, false

		for _, expr := range byFunction[key] {
			root, methods := factoryReturnChain(expr)

			comp := ""
			if root != "" {
				comp = pr.resolverSet.Resolve(root)
			}

			if comp == "" {
				uncertain = true

				continue
			}

			candidate = true

			if pr.FuncLookup != nil {
				comp = pr.walkChainRest(comp, methods)
			} else {
				comp = dynamicIfTyped(comp, methods)
			}

			if comp == "" || comp == "$any" || answer != "" && !strings.EqualFold(answer, comp) {
				uncertain = true
			}

			answer = comp
		}

		if candidate {
			// Prevent the earlier first/last-return heuristic from settling a
			// conflicting branch later during pending assignment resolution.
			f.returnVar = ""

			if uncertain {
				answer = ""
			}

			// A declared contract is checked by CFML and outranks inference.
			switch strings.ToLower(f.ReturnType) {
			case "", "any", "component", "object":
			default:
				answer = ""
			}

			f.ReturnComponent = pr.componentReturnFor(f, answer)
		}
	}
}

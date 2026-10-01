package parser

import "strings"

// PropertyHop distinguishes a field read from a method call in compact chains.
// FuncLookup receives the same marker so assignment and direct-call traversal
// agree; no additional field is needed in Scanner, FunctionDef or CallSite.
func PropertyHop(name string) string { return "$property:" + name }

// PropertyName extracts a field name from a chain hop.
func PropertyName(hop string) (string, bool) { return strings.CutPrefix(hop, "$property:") }

// CallHop retains argument-dependent factory identity until external lookup.
func CallHop(expression string) string { return "$call:" + expression }

// CallExpression extracts the argument evidence used by external lookup.
func CallExpression(hop string) (string, bool) { return strings.CutPrefix(hop, "$call:") }

func callExpressionAt(scanner *Scanner, name string) string {
	sc := *scanner

	start := sc.PeekSkipComments().Offset
	if sc.NextSkipComments().Kind != TokLParen {
		return ""
	}

	depth := 1
	for depth > 0 {
		tok := sc.NextSkipComments()
		switch tok.Kind {
		case TokEOF:
			return ""
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		default:
		}

		if depth == 0 {
			return name + sc.src[start:tok.Offset+1]
		}
	}

	return ""
}

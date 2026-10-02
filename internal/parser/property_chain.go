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

// callHopAt is name as a CallSite.Chain entry, with the argument list the
// scanner is in front of, or the bare name when there is none to read.
func callHopAt(scanner *Scanner, name string) string {
	if expression := callExpressionAt(scanner, name); expression != "" {
		return CallHop(expression)
	}

	return name
}

// hopSince is name as a CallSite.Chain entry, for an argument list the scan
// has just consumed from start, the offset of its "(". A peek leaves the
// scanner's position where it was, so pos is just past the ")".
func (p *scriptParser) hopSince(name string, start int) string {
	end := p.sc.pos
	if start < 0 || end <= start || end > len(p.sc.src) || p.sc.src[start] != '(' || p.sc.src[end-1] != ')' {
		return name
	}

	return CallHop(name + p.sc.src[start:end])
}

// CallHopName is the method a chain hop calls, whether the hop carries its
// arguments or is a bare name. Anything reporting a hop to a user reads it
// through this, so a reason or a trace names the method and not its call.
func CallHopName(hop string) string { return callHopName(hop) }

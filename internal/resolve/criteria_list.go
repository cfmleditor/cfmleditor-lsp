package resolve

import (
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
)

// A cborm service bound to an entity builds its finders on newCriteria():
//
//	array function getLatestEdits( … ) {
//		return newCriteria().createAlias( … ).when( …, function( c ){ … } ).list( max = arguments.max );
//	}
//
// and list() hands back an array of the entity, unless it is asked for a query
// (asQuery = true) or a stream. ContentBox's admin loops over what these
// return, and the loop variable had no type.

// returnsCriteriaList reports whether every return at fd's own level is a
// criteria list(): a chain on newCriteria() ending in .list( … ), or .list( … )
// on a local the function assigned newCriteria()… , asked for neither a query
// nor a stream.
func (r *Resolver) returnsCriteriaList(fd *parser.FunctionDef) bool {
	pr, tokens, open, end, ok := r.fnBody(fd)
	if !ok {
		return false
	}

	start := parser.FindFuncScopeAt(int(fd.Line), pr.Scopes).Start
	found := false

	for i := open + 1; i < end; i++ {
		switch {
		case tokens[i].Kind == parser.TokLBrace:
			if closureBody(tokens, i) {
				if e := matchingBrace(tokens, i); e > 0 {
					i = e
				}
			}
		case tokens[i].Kind == parser.TokIdent && strings.EqualFold(tokens[i].Value, "return"):
			stop := statementEnd(tokens, i+1, end)
			// fnBody's token lines count from the function's first line.
			if !criteriaList(tokens[i+1:stop], pr.Content, start, start+tokens[i].Line) {
				return false
			}

			found = true
			i = stop
		}
	}

	return found
}

// statementEnd is the index of the ; ending the statement starting at from, or
// of the } closing its block.
func statementEnd(tokens []parser.Token, from, end int) int {
	depth := 0

	for i := from; i < end; i++ {
		switch tokens[i].Kind {
		case parser.TokLParen, parser.TokLBracket, parser.TokLBrace:
			depth++
		case parser.TokRParen, parser.TokRBracket, parser.TokRBrace:
			if depth == 0 {
				return i
			}

			depth--
		case parser.TokSemicolon:
			if depth == 0 {
				return i
			}
		default:
		}
	}

	return end
}

// criteriaList reports whether expr is a criteria list() call, its head
// newCriteria() or a local the function (from line start, before line)
// assigned one.
func criteriaList(expr []parser.Token, content string, start, line int) bool {
	if len(expr) < 4 || expr[len(expr)-1].Kind != parser.TokRParen {
		return false
	}

	open := lastCallOpen(expr)
	if open < 2 || expr[open-2].Kind != parser.TokDot || !strings.EqualFold(expr[open-1].Value, "list") {
		return false
	}

	args := strings.ToLower(producerText(expr[open+1 : len(expr)-1]))
	if strings.Contains(args, "asstream") || strings.Contains(strings.ReplaceAll(args, " ", ""), "asquery=true") {
		return false
	}

	head := expr[0]
	if head.Kind != parser.TokIdent {
		return false
	}

	if strings.EqualFold(head.Value, "newCriteria") {
		return len(expr) > 1 && expr[1].Kind == parser.TokLParen
	}

	rhs, ok := localAssignment(content, head.Value, start, line)

	return ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(rhs)), "newcriteria(")
}

// lastCallOpen is the index of the ( opening the call whose ) ends expr.
func lastCallOpen(expr []parser.Token) int {
	depth := 0

	for i, e := range slices.Backward(expr) {
		switch e.Kind {
		case parser.TokRParen:
			depth++
		case parser.TokLParen:
			depth--
			if depth == 0 {
				return i
			}
		default:
		}
	}

	return -1
}

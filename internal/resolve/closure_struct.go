package resolve

import (
	"strings"

	"github.com/cfmleditor/clif/internal/docs"
	"github.com/cfmleditor/clif/internal/parser"
)

// A function may return a struct whose members are closures, and a chain is
// then a call on one of them: DI/1's declare() builds
//
//	var declaration = { beanName : beanName, built : false };
//	structAppend( declaration, { instanceOf : function( dottedPath ) { …; return declaration; }, … } );
//	return declaration;
//
// and FW/1 applications write declare( "x" ).instanceOf( "y" ).asSingleton().
// No component holds instanceOf, so the hop has no component return type. The
// struct's members are read from the function's own body: every struct literal
// assigned to, or structAppend()ed onto, the one local every top-level return
// returns. A member that returns that local again keeps the chain on the same
// struct; one that returns anything else ends what can be checked.

// closureStruct is what fd returns when it is such a struct: each closure
// member's lowercased name, and whether that member returns the struct.
func (r *Resolver) closureStruct(fd *parser.FunctionDef) map[string]bool {
	_, tokens, open, end, ok := r.fnBody(fd)
	if !ok {
		return nil
	}

	held := structReturnedLocal(tokens, open+1, end)
	if held == "" {
		return nil
	}

	members := map[string]bool{}

	for i := open + 1; i < end; i++ {
		if lit := structLiteralFor(tokens, i, held); lit >= 0 {
			collectClosureMembers(tokens, lit, held, members)
		}
	}

	if len(members) == 0 {
		return nil
	}

	return members
}

// fnBody is the tokens of fd's function and the indexes of its body's braces.
func (r *Resolver) fnBody(fd *parser.FunctionDef) (pr *parser.ParseResult, tokens []parser.Token, open, end int, ok bool) {
	if fd == nil || !fd.URI.IsFile() {
		return nil, nil, 0, 0, false
	}

	pr = r.handlerParse(fd.URI.Path())
	if pr == nil {
		return nil, nil, 0, 0, false
	}

	scope := parser.FindFuncScopeAt(int(fd.Line), pr.Scopes)
	if scope.Start == -1 || !strings.EqualFold(scope.Name, fd.Name) {
		return nil, nil, 0, 0, false
	}

	lines := strings.Split(pr.Content, "\n")
	if scope.End >= len(lines) {
		return nil, nil, 0, 0, false
	}

	tokens = significantTokens(strings.Join(lines[scope.Start:scope.End+1], "\n"))

	open = indexKind(tokens, 0, parser.TokLBrace)
	if open < 0 {
		return nil, nil, 0, 0, false
	}

	end = matchingBrace(tokens, open)

	return pr, tokens, open, end, end > 0
}

// engineValueReturn reports whether every return at fd's own level returns a
// literal or a chain headed by a built-in function, and at least one the
// latter: TestBox's getPageContextResponse() returns
// `getPageContext().getResponse()`, or a struct standing in for one under the
// CLI. What the engine hands back is dynamic, as a call chained on a built-in
// is where it is written.
func (r *Resolver) engineValueReturn(fd *parser.FunctionDef) bool {
	pr, tokens, open, end, ok := r.fnBody(fd)
	if !ok {
		return false
	}

	engine := false

	for i := open + 1; i < end; i++ {
		switch {
		case tokens[i].Kind == parser.TokLBrace:
			if closureBody(tokens, i) {
				if e := matchingBrace(tokens, i); e > 0 {
					i = e
				}
			}
		case tokens[i].Kind == parser.TokIdent && strings.EqualFold(tokens[i].Value, "return"):
			if i+1 >= end {
				return false
			}

			kind := returnKind(tokens, i+1, end, pr)
			if kind == retOther {
				return false
			}

			engine = engine || kind == retEngine
		}
	}

	return engine
}

type retKindT int

const (
	retOther retKindT = iota
	retLiteral
	retEngine
)

// returnKind classifies the return expression starting at tokens[from]: a
// literal (or nothing), a chain headed by a built-in function, or anything
// else. A ternary is the weaker of its two branches.
func returnKind(tokens []parser.Token, from, end int, pr *parser.ParseResult) retKindT {
	stop := from
	question, colon := -1, -1

	for depth := 0; stop < end; stop++ {
		switch tokens[stop].Kind {
		case parser.TokLParen, parser.TokLBrace, parser.TokLBracket:
			depth++
		case parser.TokRParen, parser.TokRBrace, parser.TokRBracket:
			depth--
		case parser.TokQuestion:
			if depth == 0 && question < 0 {
				question = stop
			}
		case parser.TokColon:
			if depth == 0 && question >= 0 && colon < 0 {
				colon = stop
			}
		default:
		}

		if depth == 0 && tokens[stop].Kind == parser.TokSemicolon {
			break
		}

		if depth < 0 {
			break
		}
	}

	if question >= 0 && colon > question {
		a, b := headKind(tokens, question+1, pr), headKind(tokens, colon+1, pr)
		if a == retOther || b == retOther {
			return retOther
		}

		return max(a, b)
	}

	return headKind(tokens, from, pr)
}

// headKind classifies the expression whose first token is tokens[i].
func headKind(tokens []parser.Token, i int, pr *parser.ParseResult) retKindT {
	if i >= len(tokens) {
		return retLiteral
	}

	switch next := tokens[i]; {
	case next.Kind == parser.TokSemicolon, next.Kind == parser.TokRBrace, next.Kind == parser.TokLBrace,
		next.Kind == parser.TokLBracket, next.Kind == parser.TokString:
		return retLiteral
	case next.Kind == parser.TokIdent && i+1 < len(tokens) && tokens[i+1].Kind == parser.TokLParen &&
		docs.IsBuiltinFunction(next.Value) && !hasFunc(pr, next.Value):
		return retEngine
	}

	return retOther
}

// closureStructHops checks the hops after hop i, then funcName, against the
// struct fd returns. It answers whether it settled the call and, if so, the
// reason ("" for accepted).
func (r *Resolver) closureStructHops(fd *parser.FunctionDef, comp, hop string, call *parser.CallSite, i int, tr *callTrace) (reason string, done bool) {
	members := r.closureStruct(fd)
	if members == nil {
		return "", false
	}

	names := make([]string, 0, len(call.Chain)-i)
	for _, h := range call.Chain[i+1:] {
		names = append(names, parser.CallHopName(h))
	}

	names = append(names, call.FuncName)

	for j, name := range names {
		self, ok := members[strings.ToLower(name)]
		if !ok {
			return "method '" + name + "' is not a member of the struct '" + hop + "' in " + displayComponent(comp) + " returns", true
		}

		tr.addf("%q is a closure member of the struct %q returns", name, hop)

		if !self && j < len(names)-1 {
			tr.addf("%q returns something other than that struct — the rest of the chain is dynamic", name)

			break
		}
	}

	tr.hit(TargetDynamic, comp, fd)

	return "", true
}

func significantTokens(text string) []parser.Token {
	all := allTokens(text)
	out := all[:0]

	for _, t := range all {
		if t.Kind != parser.TokNewline {
			out = append(out, t)
		}
	}

	return out
}

func indexKind(tokens []parser.Token, from int, kind parser.TokenKind) int {
	for i := from; i < len(tokens); i++ {
		if tokens[i].Kind == kind {
			return i
		}
	}

	return -1
}

// matchingBrace is the index of the } closing the { at open.
func matchingBrace(tokens []parser.Token, open int) int {
	depth := 0

	for i := open; i < len(tokens); i++ {
		switch tokens[i].Kind {
		case parser.TokLBrace:
			depth++
		case parser.TokRBrace:
			depth--
			if depth == 0 {
				return i
			}
		default:
		}
	}

	return -1
}

// structReturnedLocal is the one name every return at the body's own level returns
// (tokens[from:to], closures' bodies skipped), or "".
func structReturnedLocal(tokens []parser.Token, from, to int) string {
	held := ""

	for i := from; i < to; i++ {
		switch {
		case tokens[i].Kind == parser.TokLBrace:
			if e := matchingBrace(tokens, i); e > 0 {
				// A brace group that is not a closure's body is still this
				// function's code: an if or a loop.
				if !closureBody(tokens, i) {
					continue
				}

				i = e
			}
		case tokens[i].Kind == parser.TokIdent && strings.EqualFold(tokens[i].Value, "return"):
			if i+2 >= to || tokens[i+1].Kind != parser.TokIdent || tokens[i+2].Kind != parser.TokSemicolon {
				return ""
			}

			if held != "" && !strings.EqualFold(held, tokens[i+1].Value) {
				return ""
			}

			held = tokens[i+1].Value
		}
	}

	return held
}

// closureBody reports whether the { at i opens a function literal's body.
func closureBody(tokens []parser.Token, i int) bool {
	if i == 0 || tokens[i-1].Kind != parser.TokRParen {
		return false
	}

	depth := 0

	for j := i - 1; j >= 0; j-- {
		switch tokens[j].Kind {
		case parser.TokRParen:
			depth++
		case parser.TokLParen:
			depth--
			if depth == 0 {
				return j > 0 && tokens[j-1].Kind == parser.TokIdent && strings.EqualFold(tokens[j-1].Value, "function")
			}
		default:
		}
	}

	return false
}

// structLiteralFor is the index of the { of a struct literal assigned to held
// at tokens[i] (`var held = {` or `held = {`), or appended onto it
// (`structAppend( held, {`), or -1.
func structLiteralFor(tokens []parser.Token, i int, held string) int {
	t := tokens[i]
	if t.Kind != parser.TokIdent {
		return -1
	}

	if strings.EqualFold(t.Value, held) && i+2 < len(tokens) && tokens[i+1].Kind == parser.TokEquals && tokens[i+2].Kind == parser.TokLBrace {
		if i > 0 && tokens[i-1].Kind == parser.TokDot {
			return -1
		}

		return i + 2
	}

	if strings.EqualFold(t.Value, "structAppend") && i+4 < len(tokens) && tokens[i+1].Kind == parser.TokLParen &&
		tokens[i+2].Kind == parser.TokIdent && strings.EqualFold(tokens[i+2].Value, held) &&
		tokens[i+3].Kind == parser.TokComma && tokens[i+4].Kind == parser.TokLBrace {
		return i + 4
	}

	return -1
}

// collectClosureMembers adds the closure members of the struct literal at
// open, each with whether its body returns held.
func collectClosureMembers(tokens []parser.Token, open int, held string, members map[string]bool) {
	end := matchingBrace(tokens, open)
	if end < 0 {
		return
	}

	for i := open + 1; i < end; i++ {
		switch tokens[i].Kind {
		case parser.TokLBrace, parser.TokLBracket, parser.TokLParen:
			i = skipGroup(tokens, i)

			continue
		case parser.TokIdent, parser.TokString:
		default:
			continue
		}

		if i+2 >= end || tokens[i+1].Kind != parser.TokColon && tokens[i+1].Kind != parser.TokEquals {
			continue
		}

		if tokens[i+2].Kind != parser.TokIdent || !strings.EqualFold(tokens[i+2].Value, "function") {
			continue
		}

		name := strings.ToLower(strings.Trim(tokens[i].Value, `"'`))

		body := indexKind(tokens, i+3, parser.TokLBrace)
		if body < 0 || body >= end {
			return
		}

		bodyEnd := matchingBrace(tokens, body)
		if bodyEnd < 0 {
			return
		}

		members[name] = strings.EqualFold(structReturnedLocal(tokens, body+1, bodyEnd), held)
		i = bodyEnd
	}
}

// skipGroup is the index of the bracket closing the one at i.
func skipGroup(tokens []parser.Token, i int) int {
	depth := 0

	for j := i; j < len(tokens); j++ {
		switch tokens[j].Kind {
		case parser.TokLBrace, parser.TokLBracket, parser.TokLParen:
			depth++
		case parser.TokRBrace, parser.TokRBracket, parser.TokRParen:
			depth--
			if depth == 0 {
				return j
			}
		default:
		}
	}

	return len(tokens)
}

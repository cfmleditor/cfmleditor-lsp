package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
)

// Two cborm facts the stubs cannot state, both from cborm's BaseBuilder and
// CriteriaBuilder source:
//
//   - `when( test, target )` hands its closure the current builder: "the
//     closure to execute if test is true, it receives the current criteria as
//     the argument". ContentBox writes `newCriteria().when( len( search ),
//     function( c ){ c.$or( c.restrictions.like( … ) ) } )`, and the closure's
//     parameter has no declared type to read.
//   - a builder's `restrictions` is a cborm.models.criterion.Restrictions
//     (this.restrictions, assigned from an argument the stub cannot follow).
//
// The closure rule applies only to a closure written as an argument of `when`
// on a chain that is still a builder: newCriteria() followed by builder
// methods, as isNewCriteriaChain reads one, or a local assigned such a chain.

const (
	builderComponent      = "cborm.models.criterion.CriteriaBuilder"
	restrictionsComponent = "cborm.models.criterion.Restrictions"
)

// closureSpan is a function literal with parameters, the lines of its body and
// whether it is the target of a builder's when().
type closureSpan struct {
	params     []string // lowercased
	start, end int      // body lines
	builder    bool
}

// closureParamBuilder is the builder component when name is the first
// parameter of the innermost closure around line that is a builder chain's
// when() target, or "".
func (r *Resolver) closureParamBuilder(name string, line uint32, pr *parser.ParseResult) string {
	name = strings.ToLower(strings.TrimPrefix(strings.ToLower(name), "arguments."))
	if name == "" || strings.Contains(name, ".") {
		return ""
	}

	var best *closureSpan

	spans := r.closuresOf(pr)

	for i := range spans {
		s := &spans[i]
		if int(line) < s.start || int(line) > s.end {
			continue
		}

		if best == nil || s.end-s.start < best.end-best.start {
			best = s
		}
	}

	if best == nil || !best.builder || len(best.params) == 0 || best.params[0] != name {
		return ""
	}

	return builderComponent
}

// isBuilderComponent reports whether comp is cborm's criteria builder or one
// that extends it.
func isBuilderComponent(comp string) bool {
	if comp == "" {
		return false
	}

	for alt := range strings.SplitSeq(comp, "|") {
		p := strings.ToLower(filepath.ToSlash(alt))
		p = strings.ReplaceAll(p, ".", "/")

		if !strings.HasSuffix(p, "criterion/criteriabuilder") && !strings.HasSuffix(p, "criterion/basebuilder") &&
			!strings.HasSuffix(p, "criterion/detachedcriteriabuilder") && !strings.HasSuffix(p, "criterion/criteriabuilder.cfc") &&
			!strings.HasSuffix(p, "criterion/basebuilder.cfc") && !strings.HasSuffix(p, "criterion/detachedcriteriabuilder.cfc") {
			return false
		}
	}

	return true
}

func (r *Resolver) closuresOf(pr *parser.ParseResult) []closureSpan {
	return r.closureCache.get(&r.mu, pr, func() []closureSpan { return r.findClosures(pr) })
}

func (r *Resolver) findClosures(pr *parser.ParseResult) []closureSpan {
	tokens := allTokens(pr.Content)

	var out []closureSpan

	for i := 0; i+2 < len(tokens); i++ {
		if !strings.EqualFold(tokens[i].Value, "function") || tokens[i+1].Kind != parser.TokLParen {
			continue
		}

		closeParen := producerGroupEnd(tokens, i+1, parser.TokLParen, parser.TokRParen)
		if closeParen < 0 || closeParen+1 >= len(tokens) || tokens[closeParen+1].Kind != parser.TokLBrace {
			continue
		}

		closeBrace := producerGroupEnd(tokens, closeParen+1, parser.TokLBrace, parser.TokRBrace)
		if closeBrace < 0 {
			continue
		}

		span := closureSpan{start: tokens[closeParen+1].Line, end: tokens[closeBrace].Line}

		span.params = paramNames(tokens[i+2 : closeParen])
		span.builder = r.isBuilderWhenTarget(tokens, i, pr, span.start)

		out = append(out, span)
	}

	return out
}

// paramNames are the parameter names of a function's parameter list: in each
// comma-separated piece, the identifier before a default's `=`, or the last.
func paramNames(tokens []parser.Token) []string {
	var out []string

	for _, piece := range producerSplit(tokens) {
		name := ""

		for _, t := range piece {
			if t.Kind == parser.TokEquals {
				break
			}

			if t.Kind == parser.TokIdent {
				name = t.Value
			}
		}

		if name != "" {
			out = append(out, strings.ToLower(name))
		}
	}

	return out
}

// isBuilderWhenTarget reports whether the function literal at tokens[fn] is an
// argument of a when() call whose receiver is a builder chain.
func (r *Resolver) isBuilderWhenTarget(tokens []parser.Token, fn int, pr *parser.ParseResult, bodyLine int) bool {
	// Walk back to the unmatched `(` of the call the literal is an argument of.
	depth := 0
	open := -1

	for j := fn - 1; j >= 0 && open < 0; j-- {
		switch tokens[j].Kind {
		case parser.TokRParen, parser.TokRBracket, parser.TokRBrace:
			depth++
		case parser.TokLParen, parser.TokLBracket, parser.TokLBrace:
			if depth == 0 {
				if tokens[j].Kind != parser.TokLParen {
					return false
				}

				open = j
			} else {
				depth--
			}
		case parser.TokSemicolon:
			if depth == 0 {
				return false
			}
		default:
		}
	}

	if open < 2 || !strings.EqualFold(tokens[open-1].Value, "when") || tokens[open-2].Kind != parser.TokDot {
		return false
	}

	return r.builderChainEndingAt(tokens, open-2, pr, bodyLine)
}

// builderChainEndingAt reports whether the chain written before the dot at
// tokens[dot] is a builder: newCriteria() and builder methods, or a local
// assigned one.
func (r *Resolver) builderChainEndingAt(tokens []parser.Token, dot int, pr *parser.ParseResult, line int) bool {
	end := dot // exclusive end of the chain

	start := end

	for start > 0 {
		j := start - 1

		switch tokens[j].Kind {
		case parser.TokRParen:
			depth := 0

			for ; j >= 0; j-- {
				if tokens[j].Kind == parser.TokRParen {
					depth++
				} else if tokens[j].Kind == parser.TokLParen {
					depth--
					if depth == 0 {
						break
					}
				}
			}

			if j < 1 || tokens[j-1].Kind != parser.TokIdent {
				return false
			}

			start = j - 1
		case parser.TokIdent:
			start = j
		default:
			return false
		}

		if start > 0 && tokens[start-1].Kind == parser.TokDot {
			start--

			continue
		}

		break
	}

	chain := tokens[start:end]
	if isNewCriteriaChain(chain, r.isBuilderMethod) {
		return true
	}

	// A local holding a builder: `var c = newCriteria(); c.when( … )`.
	if len(chain) == 1 && chain[0].Kind == parser.TokIdent {
		if rhs, ok := localAssignment(pr.Content, chain[0].Value, 0, line); ok {
			return isNewCriteriaChain(producerTokens(rhs), r.isBuilderMethod)
		}
	}

	return false
}

// builderMember types a closure's builder parameter (`c` in `when( test,
// function( c ){ … } )`) and a builder's `restrictions`.
func (r *Resolver) builderMember(variable string, line uint32, caller, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if pr == nil || strings.ContainsAny(variable, "[(") {
		return ""
	}

	if root, member, ok := strings.Cut(variable, "."); ok && strings.EqualFold(member, "restrictions") && !isScopeWord(root) {
		comp, _ := r.receiverComponent(root, line, caller, funcName, pr, baseDir, nil)
		if isBuilderComponent(comp) {
			tr.addf("resolved %q to %q: a criteria builder's restrictions", variable, restrictionsComponent)

			return restrictionsComponent
		}

		return ""
	}

	if comp := r.closureParamBuilder(variable, line, pr); comp != "" {
		tr.addf("resolved %q to %q: the builder when() hands its closure", variable, comp)

		return comp
	}

	return ""
}

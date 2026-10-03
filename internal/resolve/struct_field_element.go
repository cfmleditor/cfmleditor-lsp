package resolve

import (
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A search function returns a struct whose fields hold collections:
//
//	var results = { "count": 0, "comments": [] };
//	var c       = newCriteria();
//	results.comments = c.list( offset = o, max = m, asQuery = false );
//	return results;
//
// and its caller reads one of them, `prc.comments = commentResults.comments`.
// The element of that field is the entity the service's criteria builder
// queries, when the source leaves no other possibility:
//
//   - the function script-declares one local struct and every return returns it,
//     and that local is used only as `x.field` — passed anywhere, indexed, or
//     assigned from another value, it may have been changed unseen;
//   - every value the field is given, in the literal and in assignments, is
//     either an empty array (compatible with any element) or the list() of a
//     criteria builder, and at least one is the latter;
//   - the builder is a local every assignment of which is newCriteria() on the
//     service the call was made on, which is bound to an entity;
//   - list() is called with `asQuery = false` written literally, or with a name
//     that is a parameter defaulting to false and not overridden at the call.
//
// A field given a query or a computed value has no element type. Every builder
// is the one service's, so the entity cannot differ between values.
// An omitted asQuery is not read as an array: cborm's default is not in the
// stubs, which drop defaults.

var (
	structFieldRe = regexp.MustCompile(`^(?:local\.)?(\w+)\.(\w+)$`)
	keyedFieldRe  = regexp.MustCompile(`(?i)^(?:local\s*\.\s*)?(\w+)\s*\[\s*variables\s*\.\s*(\w+)\s*\]$`)
)

// fieldElement is the element of the collection field of the struct the call
// expression returns, or "".
func (r *Resolver) fieldElement(call, field string, header int, caller string, pr *parser.ParseResult, baseDir, leaf string) string {
	m := loopCallRe.FindStringSubmatch(call)
	if m == nil {
		return ""
	}

	receiver, method, arguments := m[1], m[2], m[3]

	target := pr.URI.Path()

	if receiver != "" && !strings.EqualFold(receiver, "this") && !strings.EqualFold(receiver, "variables") {
		comp, _ := r.receiverComponentD(receiver, conv.Uint32(header), caller, method, pr, baseDir, nil, lookupCtx{leaf: leaf})
		target = comp
	}

	if target == "" || strings.HasPrefix(target, "$") || strings.Contains(target, "|") {
		return ""
	}

	fd := r.ResolveFunc(target, method, baseDir)
	if fd == nil {
		return ""
	}

	data, err := r.fs().ReadFile(fd.URI.Path())
	if err != nil {
		return ""
	}

	body, params, ok := scriptFunction(allTokens(string(data)), fd.Name, int(fd.Line))
	if !ok {
		return ""
	}

	entity := r.structFieldEntity(body, params, strings.ToLower(field), arguments, target, baseDir)
	if entity == "" {
		return ""
	}

	return r.withSubclasses(r.ComponentPath(entity, baseDir))
}

// scriptFunction is the body (without its braces) and parameter list of the
// script function called name declared at line, or the only one of that name.
func scriptFunction(tokens []parser.Token, name string, line int) (body, params []parser.Token, ok bool) {
	var found [][2]int

	for i := 0; i+2 < len(tokens); i++ {
		if !strings.EqualFold(tokens[i].Value, "function") || !strings.EqualFold(tokens[i+1].Value, name) || tokens[i+2].Kind != parser.TokLParen {
			continue
		}

		closeParen := producerGroupEnd(tokens, i+2, parser.TokLParen, parser.TokRParen)
		if closeParen < 0 {
			continue
		}

		open := closeParen + 1
		for open < len(tokens) && tokens[open].Kind != parser.TokLBrace && tokens[open].Kind != parser.TokSemicolon {
			open++ // attributes after the parameter list
		}

		if open >= len(tokens) || tokens[open].Kind != parser.TokLBrace {
			continue
		}

		closeBrace := producerGroupEnd(tokens, open, parser.TokLBrace, parser.TokRBrace)
		if closeBrace < 0 {
			continue
		}

		found = append(found, [2]int{i, closeBrace})
	}

	pick := -1

	for n, f := range found {
		if tokens[f[0]].Line == line || tokens[f[0]+1].Line == line {
			pick = n
		}
	}

	if pick < 0 && len(found) == 1 {
		pick = 0
	}

	if pick < 0 {
		return nil, nil, false
	}

	f := found[pick]
	closeParen := producerGroupEnd(tokens, f[0]+2, parser.TokLParen, parser.TokRParen)
	open := closeParen + 1

	for tokens[open].Kind != parser.TokLBrace {
		open++
	}

	return tokens[open+1 : f[1]], tokens[f[0]+3 : closeParen], true
}

// structFieldEntity is the entity every value of the field of body's one
// returned struct holds, or "".
func (r *Resolver) structFieldEntity(body, params []parser.Token, field, callArgs, service, baseDir string) string {
	returned, ok := returnedLocal(body)
	if !ok {
		return ""
	}

	statements := splitStatements(body)

	var values [][]parser.Token

	for _, s := range statements {
		switch kind, value := classifyStructStatement(s, returned, field); kind {
		case structOther:
			return ""
		case structValue:
			values = append(values, value)
		default:
		}
	}

	if len(values) == 0 {
		return ""
	}

	entity := ""

	for _, v := range values {
		if len(v) == 2 && v[0].Kind == parser.TokLBracket && v[1].Kind == parser.TokRBracket {
			continue
		}

		e := r.criteriaListEntity(v, statements, params, callArgs, service, baseDir)
		if e == "" {
			return ""
		}

		entity = e
	}

	return entity
}

// returnedLocal is the one variable every return of body returns.
func returnedLocal(body []parser.Token) (string, bool) {
	name := ""

	for i, t := range body {
		if t.Kind != parser.TokIdent || !strings.EqualFold(t.Value, "return") {
			continue
		}

		if i+2 >= len(body) || body[i+1].Kind != parser.TokIdent || body[i+2].Kind != parser.TokSemicolon {
			return "", false
		}

		v := strings.ToLower(body[i+1].Value)
		if name != "" && name != v {
			return "", false
		}

		name = v
	}

	return name, name != ""
}

func splitStatements(body []parser.Token) [][]parser.Token {
	var (
		out   [][]parser.Token
		cur   []parser.Token
		depth int // inside a struct literal, whose braces are not blocks
	)

	for _, t := range body {
		if depth > 0 {
			cur = append(cur, t)

			switch t.Kind {
			case parser.TokLBrace:
				depth++
			case parser.TokRBrace:
				depth--
			default:
			}

			continue
		}

		switch t.Kind {
		case parser.TokLBrace:
			if n := len(cur); n > 0 && (cur[n-1].Kind == parser.TokEquals || cur[n-1].Kind == parser.TokColon || cur[n-1].Kind == parser.TokLParen || cur[n-1].Kind == parser.TokComma) {
				depth = 1

				cur = append(cur, t)

				continue
			}

			fallthrough
		case parser.TokSemicolon, parser.TokRBrace:
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
		default:
			cur = append(cur, t)
		}
	}

	if len(cur) > 0 {
		out = append(out, cur)
	}

	return out
}

type structStatementKind int

const (
	structIrrelevant structStatementKind = iota // does not mention the struct
	structValue                                 // gives the field a value
	structOther                                 // uses the struct some other way
)

// classifyStructStatement says what statement does with the struct named
// name, and for a statement giving field a value, the tokens of that value.
// The struct's own declaration is read here: its literal's entry for field is
// the field's first value, and an entry that is not a plain key is unreadable.
func classifyStructStatement(s []parser.Token, name, field string) (structStatementKind, []parser.Token) {
	mentions := false

	for _, t := range s {
		if t.Kind == parser.TokIdent && strings.EqualFold(t.Value, name) {
			mentions = true
		}
	}

	if !mentions {
		return structIrrelevant, nil
	}

	// `return x` hands the struct back; returnedLocal has vouched for it.
	if len(s) == 2 && strings.EqualFold(s[0].Value, "return") && strings.EqualFold(s[1].Value, name) {
		return structIrrelevant, nil
	}

	i := 0
	if strings.EqualFold(s[0].Value, "var") {
		i = 1
	}

	if i+1 < len(s) && strings.EqualFold(s[i].Value, name) && s[i+1].Kind == parser.TokEquals && (i+2 >= len(s) || s[i+2].Kind != parser.TokEquals) {
		return declarationValue(s[i+2:], field)
	}

	if i+3 < len(s) && strings.EqualFold(s[i].Value, name) && s[i+1].Kind == parser.TokDot && s[i+2].Kind == parser.TokIdent && s[i+3].Kind == parser.TokEquals && (i+4 >= len(s) || s[i+4].Kind != parser.TokEquals) {
		// A write to another field is no use to this one, and not a problem.
		if strings.EqualFold(s[i+2].Value, field) {
			return structValue, s[i+4:]
		}

		if mentionsOutside(s[i+4:], name) {
			return structOther, nil
		}

		return structIrrelevant, nil
	}

	// Anything else may only read x.f: the name must be followed by a dot and
	// a member, and a member call (x.f.append()) could change the field.
	for j, t := range s {
		if t.Kind != parser.TokIdent || !strings.EqualFold(t.Value, name) {
			continue
		}

		if j+2 >= len(s) || s[j+1].Kind != parser.TokDot || s[j+2].Kind != parser.TokIdent {
			return structOther, nil
		}

		if j+3 < len(s) && (s[j+3].Kind == parser.TokDot || s[j+3].Kind == parser.TokLParen || s[j+3].Kind == parser.TokEquals) {
			return structOther, nil
		}
	}

	return structIrrelevant, nil
}

func mentionsOutside(tokens []parser.Token, name string) bool {
	for _, t := range tokens {
		if t.Kind == parser.TokIdent && strings.EqualFold(t.Value, name) {
			return true
		}
	}

	return false
}

// declarationValue reads `{ a: …, b: … }` and returns the value of field.
// Any other right-hand side is a struct nothing is known about.
func declarationValue(rhs []parser.Token, field string) (structStatementKind, []parser.Token) {
	if len(rhs) < 2 || rhs[0].Kind != parser.TokLBrace || producerGroupEnd(rhs, 0, parser.TokLBrace, parser.TokRBrace) != len(rhs)-1 {
		return structOther, nil
	}

	var found []parser.Token

	for _, entry := range producerSplit(rhs[1 : len(rhs)-1]) {
		if len(entry) == 0 {
			continue
		}

		if len(entry) < 3 || entry[1].Kind != parser.TokEquals && entry[1].Kind != parser.TokColon || entry[0].Kind != parser.TokIdent && entry[0].Kind != parser.TokString {
			return structOther, nil
		}

		if strings.EqualFold(strings.Trim(entry[0].Value, "\"'"), field) {
			found = entry[2:]
		}
	}

	if found == nil {
		return structIrrelevant, nil
	}

	return structValue, found
}

// criteriaListEntity is the entity the array value (a `c.list( … )` or
// `c.resultTransformer( … ).list( … )`) holds, or "".
func (r *Resolver) criteriaListEntity(value []parser.Token, statements [][]parser.Token, params []parser.Token, callArgs, service, baseDir string) string {
	if len(value) < 5 || value[0].Kind != parser.TokIdent {
		return ""
	}

	builder := strings.ToLower(value[0].Value)
	i := 1

	if value[i].Kind == parser.TokDot && value[i+1].Kind == parser.TokIdent && strings.EqualFold(value[i+1].Value, "resultTransformer") && value[i+2].Kind == parser.TokLParen {
		end := producerGroupEnd(value, i+2, parser.TokLParen, parser.TokRParen)
		if end < 0 {
			return ""
		}

		i = end + 1
	}

	if i+3 >= len(value) || value[i].Kind != parser.TokDot || !strings.EqualFold(value[i+1].Value, "list") || value[i+2].Kind != parser.TokLParen {
		return ""
	}

	end := producerGroupEnd(value, i+2, parser.TokLParen, parser.TokRParen)
	if end != len(value)-1 {
		return ""
	}

	if !asQueryFalse(value[i+3:end], params, callArgs) || !r.builderIsNewCriteria(builder, statements) {
		return ""
	}

	comp := service
	if r.ResolveFunc(comp, "newCriteria", baseDir) == nil {
		return ""
	}

	return r.boundEntity(r.ComponentPath(comp, baseDir))
}

// builderIsNewCriteria reports whether every assignment to the local name in
// statements is `newCriteria( … )`, and there is one.
func (r *Resolver) builderIsNewCriteria(name string, statements [][]parser.Token) bool {
	seen := false

	for _, s := range statements {
		i := 0
		if strings.EqualFold(s[0].Value, "var") {
			i = 1
		}

		if i+1 >= len(s) || !strings.EqualFold(s[i].Value, name) || s[i+1].Kind != parser.TokEquals || i+2 < len(s) && s[i+2].Kind == parser.TokEquals {
			if mentionsAsAssignee(s, name) {
				return false
			}

			continue
		}

		rhs := s[i+2:]
		if len(rhs) >= 3 && rhs[0].Kind == parser.TokIdent && (strings.EqualFold(rhs[0].Value, "this") || strings.EqualFold(rhs[0].Value, "variables")) && rhs[1].Kind == parser.TokDot {
			rhs = rhs[2:]
		}

		if !isNewCriteriaChain(rhs, r.isBuilderMethod) {
			return false
		}

		seen = true
	}

	return seen
}

// mentionsAsAssignee reports whether s writes name through a member or index,
// or hands it to a call that could replace it.
func mentionsAsAssignee(s []parser.Token, name string) bool {
	for j, t := range s {
		if t.Kind != parser.TokIdent || !strings.EqualFold(t.Value, name) {
			continue
		}

		if j+1 < len(s) && s[j+1].Kind == parser.TokEquals && (j+2 >= len(s) || s[j+2].Kind != parser.TokEquals) {
			return true
		}
	}

	return false
}

// asQueryFalse reports whether the argument list args passes asQuery as false:
// literally, or as a parameter of the function defaulting to false that the
// call (callArgs) does not name or sets to false.
func asQueryFalse(args, params []parser.Token, callArgs string) bool {
	for _, piece := range producerSplit(args) {
		if len(piece) < 3 || !strings.EqualFold(piece[0].Value, "asQuery") || piece[1].Kind != parser.TokEquals && piece[1].Kind != parser.TokColon {
			continue
		}

		value := piece[2:]
		if len(value) == 1 && strings.EqualFold(value[0].Value, "false") {
			return true
		}

		var name string

		switch {
		case len(value) == 1 && value[0].Kind == parser.TokIdent:
			name = value[0].Value
		case len(value) == 3 && strings.EqualFold(value[0].Value, "arguments") && value[1].Kind == parser.TokDot:
			name = value[2].Value
		default:
			return false
		}

		return parameterDefaultsFalse(params, name) && callPassesFalseOrNothing(callArgs, name)
	}

	return false
}

func parameterDefaultsFalse(params []parser.Token, name string) bool {
	for _, piece := range producerSplit(params) {
		for i, t := range piece {
			if t.Kind == parser.TokIdent && strings.EqualFold(t.Value, name) && i+2 < len(piece) && piece[i+1].Kind == parser.TokEquals {
				return len(piece) == i+3 && strings.EqualFold(piece[i+2].Value, "false")
			}
		}
	}

	return false
}

// callPassesFalseOrNothing reports whether the call arguments leave name at
// its default or name it false. A positional argument is not looked at, so a
// call passing any is declined unless it names nothing: the parameter's
// position is not tracked here.
func callPassesFalseOrNothing(callArgs, name string) bool {
	tokens := producerTokens(callArgs)

	for _, piece := range producerSplit(tokens) {
		if len(piece) >= 3 && piece[1].Kind == parser.TokEquals || len(piece) >= 3 && piece[1].Kind == parser.TokColon {
			if strings.EqualFold(piece[0].Value, name) {
				return len(piece) == 3 && strings.EqualFold(piece[2].Value, "false")
			}

			continue
		}

		if len(piece) > 0 {
			return false
		}
	}

	return true
}

// builderMethods are cborm criteria methods known to return the builder, so a
// chain of them on newCriteria() is still the builder. isBuilderMethod adds
// every method the Restrictions stub declares, since the builder answers each
// as a restriction and returns itself.
var builderMethods = map[string]bool{
	"eq": true, "ne": true, "gt": true, "ge": true, "lt": true, "le": true,
	"isEq": true, "isNe": true, "isGT": true, "isGE": true, "isLT": true, "isLE": true,
	"like": true, "ilike": true, "between": true, "in": true, "isIn": true,
	"isNull": true, "isNotNull": true, "isEmpty": true, "isNotEmpty": true, "idEq": true,
	"$or": true, "$and": true, "$not": true, "joinTo": true, "createAlias": true,
	"maxResults": true, "firstResult": true, "resultTransformer": true,
}

// isNewCriteriaChain reports whether tokens are `newCriteria( … )` followed by
// calls of builderMethods only.
func isNewCriteriaChain(tokens []parser.Token, isBuilderMethod func(string) bool) bool {
	if len(tokens) < 3 || !strings.EqualFold(tokens[0].Value, "newCriteria") || tokens[1].Kind != parser.TokLParen {
		return false
	}

	pos := producerGroupEnd(tokens, 1, parser.TokLParen, parser.TokRParen) + 1
	if pos == 0 {
		return false
	}

	for pos < len(tokens) {
		if pos+2 >= len(tokens) || tokens[pos].Kind != parser.TokDot || tokens[pos+1].Kind != parser.TokIdent || tokens[pos+2].Kind != parser.TokLParen {
			return false
		}

		end := producerGroupEnd(tokens, pos+2, parser.TokLParen, parser.TokRParen)
		if !isBuilderMethod(tokens[pos+1].Value) || end < 0 {
			return false
		}

		pos = end + 1
	}

	return true
}

// isBuilderMethod reports whether calling name on a criteria builder returns
// the builder: one of builderMethods, a restriction (the builder forwards each
// method of cborm's Restrictions to it), or a method the builder declares to
// return a builder.
func (r *Resolver) isBuilderMethod(name string) bool {
	for m := range builderMethods {
		if strings.EqualFold(m, name) {
			return true
		}
	}

	if r.ResolveFunc(restrictionsComponent, name, "") != nil {
		return true
	}

	if fd := r.ResolveFunc(builderComponent, name, ""); fd != nil {
		return strings.HasSuffix(strings.ToLower(fd.ReturnType), "builder")
	}

	return false
}

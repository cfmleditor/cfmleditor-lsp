package parser

import "strings"

// scriptParser extracts definitions from CFScript source.
type scriptParser struct {
	sc       *Scanner
	funcs    []FunctionDef
	vars     []VarDef
	refs     []ComponentRef
	fileURI  string
	baseLine int // line offset for this region within the file
}

func newScriptParser(src, fileURI string, baseLine int) *scriptParser {
	return &scriptParser{
		sc:       NewScanner(src),
		fileURI:  fileURI,
		baseLine: baseLine,
	}
}

// parse scans through CFScript source extracting all definitions.
func (p *scriptParser) parse() {
	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return
		}

		if tok.Kind == TokIdent {
			p.handleIdent(tok)
		}
	}
}

func (p *scriptParser) handleIdent(tok Token) {
	val := tok.Value
	lower := strings.ToLower(val)

	switch lower {
	case "function":
		p.parseFunction(tok, "", "")
	case "public", "private", "remote", "package":
		p.parseAccessModified(tok)
	case "var":
		p.parseVarDecl(tok)
	case "local":
		p.parseDotAssign(tok, ScopeLocal)
	case "arguments":
		p.parseDotAssign(tok, ScopeArguments)
	case "this":
		p.parseDotAssign(tok, ScopeThis)
	case "variables":
		p.parseDotAssign(tok, ScopeVariables)
	case "new":
		// new X() without assignment — skip
	default:
		p.parseIdentAssign(tok)
	}
}

// parseAccessModified handles: access [returntype] function name(...)
func (p *scriptParser) parseAccessModified(accessTok Token) {
	access := accessTok.Value

	next := p.sc.PeekSkipComments()
	if next.Kind != TokIdent {
		return
	}

	if identEq(next.Value, "function") {
		p.sc.NextSkipComments() // consume "function"
		p.parseFunction(accessTok, access, "")

		return
	}
	// Could be returntype before function
	retType := p.sc.NextSkipComments() // consume returntype

	next2 := p.sc.PeekSkipComments()
	if next2.Kind == TokIdent && identEq(next2.Value, "function") {
		p.sc.NextSkipComments() // consume "function"
		p.parseFunction(accessTok, access, retType.Value)
	}
}

// parseFunction handles: function name( args ) { body }
func (p *scriptParser) parseFunction(startTok Token, _ string, _ string) {
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	// Expect (
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

	args := p.parseArgList()

	// Create component refs for arguments with component-like types
	for _, a := range args {
		if isComponentType(a.Type) {
			p.refs = append(p.refs, ComponentRef{
				Variable:  a.Name,
				Component: a.Type,
				URI:       uriFromString(p.fileURI),
				Line:      uint32(p.baseLine + startTok.Line),
			})
		}
	}

	fd := FunctionDef{
		Name:      nameTok.Value,
		URI:       uriFromString(p.fileURI),
		Line:      uint32(p.baseLine + startTok.Line),
		Arguments: args,
	}
	p.funcs = append(p.funcs, fd)

	// Parse the body for variable declarations
	p.parseFuncBody()
}

// parseArgList parses comma-separated arguments until ')'.
// Format: [required] [type] name [= default]
func (p *scriptParser) parseArgList() []Argument {
	var args []Argument

	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokRParen || tok.Kind == TokEOF {
			break
		}

		if tok.Kind == TokComma {
			continue
		}

		if tok.Kind != TokIdent {
			continue
		}

		var required bool

		var typeName, name string

		// Collect up to 3 identifiers: [required] [type] name
		idents := []string{tok.Value}

		for {
			peek := p.sc.PeekSkipComments()
			if peek.Kind == TokIdent {
				p.sc.NextSkipComments()

				idents = append(idents, peek.Value)
				if len(idents) == 3 {
					break
				}
			} else {
				break
			}
		}

		switch len(idents) {
		case 1:
			name = idents[0]
		case 2:
			if identEq(idents[0], "required") {
				required = true
				name = idents[1]
			} else {
				typeName = idents[0]
				name = idents[1]
			}
		case 3:
			required = identEq(idents[0], "required")
			typeName = idents[1]
			name = idents[2]
		}

		// Skip default value: = expr (until , or ))
		peek := p.sc.PeekSkipComments()
		if peek.Kind == TokEquals {
			p.sc.NextSkipComments() // consume =
			p.skipDefaultValue()
		}

		args = append(args, Argument{Name: name, Type: typeName, Required: required})
	}

	return args
}

// skipDefaultValue skips tokens until we hit , or ) at depth 0.
func (p *scriptParser) skipDefaultValue() {
	depth := 0

	for {
		peek := p.sc.PeekSkipComments()
		if peek.Kind == TokEOF {
			return
		}

		if depth == 0 && (peek.Kind == TokComma || peek.Kind == TokRParen) {
			return
		}

		p.sc.NextSkipComments()

		switch peek.Kind { //nolint:exhaustive
		case TokLParen, TokLBrace, TokLBracket:
			depth++
		case TokRParen, TokRBrace, TokRBracket:
			depth--
		}
	}
}

// skipToBodyEnd skips past the next { ... } block (or ; for interface methods).
func (p *scriptParser) skipToBodyEnd() {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		p.sc.NextSkipComments()

		return
	}

	if tok.Kind != TokLBrace {
		return
	}

	p.sc.NextSkipComments() // consume {

	depth := 1
	for depth > 0 {
		t := p.sc.NextSkipComments()
		if t.Kind == TokEOF {
			return
		}

		switch t.Kind { //nolint:exhaustive
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
		}
	}
}

// parseFuncBody parses the body of a function for variable declarations.
// Handles { ... } or ; (interface methods).
func (p *scriptParser) parseFuncBody() {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		p.sc.NextSkipComments()

		return
	}

	if tok.Kind != TokLBrace {
		return
	}

	p.sc.NextSkipComments() // consume {

	depth := 1
	for depth > 0 {
		t := p.sc.NextSkipComments()
		if t.Kind == TokEOF {
			return
		}

		switch t.Kind { //nolint:exhaustive
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
		case TokIdent:
			if depth > 0 {
				p.handleBodyIdent(t, depth)
			}
		}
	}
}

// handleBodyIdent handles identifiers inside a function body for var extraction.
func (p *scriptParser) handleBodyIdent(tok Token, _ int) {
	lower := strings.ToLower(tok.Value)
	switch lower {
	case "var":
		p.parseVarDecl(tok)
	case "local":
		p.parseDotAssign(tok, ScopeLocal)
	case "arguments":
		p.parseDotAssign(tok, ScopeArguments)
	case "this":
		p.parseDotAssign(tok, ScopeThis)
	case "variables":
		p.parseDotAssign(tok, ScopeVariables)
	case "function":
		// Nested function — skip its body
		nameTok := p.sc.NextSkipComments()
		if nameTok.Kind != TokIdent {
			return
		}

		lp := p.sc.NextSkipComments()
		if lp.Kind != TokLParen {
			return
		}
		// Skip args
		pd := 1
		for pd > 0 {
			t := p.sc.NextSkipComments()
			if t.Kind == TokEOF {
				return
			}

			if t.Kind == TokLParen {
				pd++
			}

			if t.Kind == TokRParen {
				pd--
			}
		}

		p.skipToBodyEnd()
	case "new":
		// skip
	default:
		p.parseIdentAssign(tok)
	}
}

// parseVarDecl handles: var name = expr
func (p *scriptParser) parseVarDecl(varTok Token) {
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}
	// Check for = to confirm assignment
	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		return
	}

	p.sc.NextSkipComments() // consume =

	p.vars = append(p.vars, VarDef{
		Name:  nameTok.Value,
		Scope: ScopeLocal,
		Line:  uint32(p.baseLine + varTok.Line),
	})

	// Check RHS for component refs
	p.scanRHSForRefs(nameTok.Value, varTok.Line)
}

// parseDotAssign handles: scope.name = expr
func (p *scriptParser) parseDotAssign(scopeTok Token, scope Scope) {
	dot := p.sc.PeekSkipComments()
	if dot.Kind != TokDot {
		return
	}

	p.sc.NextSkipComments() // consume .

	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		return
	}

	p.sc.NextSkipComments() // consume =

	p.vars = append(p.vars, VarDef{
		Name:  nameTok.Value,
		Scope: scope,
		Line:  uint32(p.baseLine + scopeTok.Line),
	})

	p.scanRHSForRefs(nameTok.Value, scopeTok.Line)
}

// parseIdentAssign handles: name = expr (plain assignment → variables scope)
func (p *scriptParser) parseIdentAssign(identTok Token) {
	if isKeyword(identTok.Value) {
		return
	}

	peek := p.sc.PeekSkipComments()
	if peek.Kind == TokEquals { //nolint:gocritic,staticcheck // simple if-else is clearer here
		p.sc.NextSkipComments() // consume =

		p.vars = append(p.vars, VarDef{
			Name:  identTok.Value,
			Scope: ScopeVariables,
			Line:  uint32(p.baseLine + identTok.Line),
		})

		p.scanRHSForRefs(identTok.Value, identTok.Line)
	} else if peek.Kind == TokDot { //nolint:staticcheck,revive // intentionally empty — x.y = ... is not a new var
	}
}

// scanRHSForRefs checks if the RHS is a new/createObject/entityNew expression.
func (p *scriptParser) scanRHSForRefs(varName string, line int) {
	tok := p.sc.PeekSkipComments()
	if tok.Kind != TokIdent {
		return
	}

	lower := strings.ToLower(tok.Value)
	switch lower {
	case "new":
		p.sc.NextSkipComments() // consume "new"
		p.parseNewExpr(varName, line)
	case "createobject":
		p.sc.NextSkipComments() // consume "createObject"
		p.parseCreateObject(varName, line)
	case "entitynew":
		p.sc.NextSkipComments() // consume "entityNew"
		p.parseEntityNew(varName, line)
	case "entityload":
		p.sc.NextSkipComments() // consume "entityLoad"
		p.parseEntityNew(varName, line)
	}
}

// parseNewExpr handles: new ["]path.Component["][()]
func (p *scriptParser) parseNewExpr(varName string, line int) {
	tok := p.sc.NextSkipComments()

	var component string

	if tok.Kind == TokString { //nolint:gocritic // if-else chain is clearer than switch here
		// Quoted: new "path.Component" or new 'path.Component'
		component = unquote(tok.Value)
	} else if tok.Kind == TokIdent {
		// Unquoted: new path.Component
		component = tok.Value

		for {
			peek := p.sc.PeekSkipComments()
			if peek.Kind == TokDot {
				p.sc.NextSkipComments()

				next := p.sc.NextSkipComments()
				if next.Kind == TokIdent {
					component += "." + next.Value
				}
			} else {
				break
			}
		}
	} else {
		return
	}

	if component != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable:  varName,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(p.baseLine + line),
		})
	}
}

// parseCreateObject handles: createObject("component", "path.Component")
func (p *scriptParser) parseCreateObject(varName string, line int) {
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}
	// First arg should be "component"
	arg1 := p.sc.NextSkipComments()
	if arg1.Kind != TokString || !identEq(unquote(arg1.Value), "component") {
		return
	}

	comma := p.sc.NextSkipComments()
	if comma.Kind != TokComma {
		return
	}

	arg2 := p.sc.NextSkipComments()
	if arg2.Kind != TokString {
		return
	}

	component := unquote(arg2.Value)
	if component != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable:  varName,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(p.baseLine + line),
		})
	}
}

// parseEntityNew handles: entityNew("EntityName")
func (p *scriptParser) parseEntityNew(varName string, line int) {
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

	arg := p.sc.NextSkipComments()
	if arg.Kind != TokString {
		return
	}

	component := unquote(arg.Value)
	if component != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable:  varName,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(p.baseLine + line),
		})
	}
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		return s[1 : len(s)-1]
	}

	return s
}

func isKeyword(s string) bool {
	switch strings.ToLower(s) {
	case "var", "local", "if", "else", "for", "while", "do", "switch", "case",
		"try", "catch", "finally", "return", "break", "continue", "function",
		"component", "interface", "new", "throw", "import", "true", "false":
		return true
	}

	return false
}

package cfparser

import "strings"

// shallowScriptParser extracts function signatures and component refs
// without parsing function bodies for variables.
type shallowScriptParser struct {
	sc       *Scanner
	funcs    []FunctionDef
	refs     []ComponentRef
	scopes   []FuncScope
	fileURI  string
	baseLine int
}

func newShallowScriptParser(src, fileURI string, baseLine int) *shallowScriptParser {
	return &shallowScriptParser{
		sc:       NewScanner(src),
		fileURI:  fileURI,
		baseLine: baseLine,
	}
}

func (p *shallowScriptParser) parse() {
	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return
		}
		if tok.Kind != TokIdent {
			continue
		}
		lower := strings.ToLower(tok.Value)
		switch lower {
		case "function":
			p.parseFunction(tok, "", "")
		case "public", "private", "remote", "package":
			p.parseAccessModified(tok)
		default:
			// Check for component refs in assignments: ident = new/createObject/entityNew
			p.checkAssignRef(tok)
		}
	}
}

func (p *shallowScriptParser) parseAccessModified(accessTok Token) {
	next := p.sc.PeekSkipComments()
	if next.Kind != TokIdent {
		return
	}
	if identEq(next.Value, "function") {
		p.sc.NextSkipComments()
		p.parseFunction(accessTok, accessTok.Value, "")
		return
	}
	retType := p.sc.NextSkipComments()
	next2 := p.sc.PeekSkipComments()
	if next2.Kind == TokIdent && identEq(next2.Value, "function") {
		p.sc.NextSkipComments()
		p.parseFunction(accessTok, accessTok.Value, retType.Value)
	}
}

func (p *shallowScriptParser) parseFunction(startTok Token, _ string, _ string) {
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}
	args := p.parseArgList()

	funcLine := p.baseLine + startTok.Line
	p.funcs = append(p.funcs, FunctionDef{
		Name:      nameTok.Value,
		URI:       uriFromString(p.fileURI),
		Line:      uint32(funcLine),
		Arguments: args,
	})

	// Record scope and skip body
	endLine := p.skipBody()
	p.scopes = append(p.scopes, FuncScope{Start: funcLine, End: p.baseLine + endLine})
}

func (p *shallowScriptParser) parseArgList() []Argument {
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
		peek := p.sc.PeekSkipComments()
		if peek.Kind == TokEquals {
			p.sc.NextSkipComments()
			p.skipDefault()
		}
		args = append(args, Argument{Name: name, Type: typeName, Required: required})
	}
	return args
}

func (p *shallowScriptParser) skipDefault() {
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
		switch peek.Kind {
		case TokLParen, TokLBrace, TokLBracket:
			depth++
		case TokRParen, TokRBrace, TokRBracket:
			depth--
		}
	}
}

// skipBody skips { ... } or ; and returns the line of the closing token.
func (p *shallowScriptParser) skipBody() int {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		t := p.sc.NextSkipComments()
		return t.Line
	}
	if tok.Kind != TokLBrace {
		return tok.Line
	}
	p.sc.NextSkipComments()
	depth := 1
	var last Token
	for depth > 0 {
		last = p.sc.NextSkipComments()
		if last.Kind == TokEOF {
			return last.Line
		}
		if last.Kind == TokLBrace {
			depth++
		}
		if last.Kind == TokRBrace {
			depth--
		}
	}
	return last.Line
}

func (p *shallowScriptParser) checkAssignRef(tok Token) {
	if isKeyword(tok.Value) {
		return
	}
	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		return
	}
	p.sc.NextSkipComments() // consume =
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		return
	}
	lower := strings.ToLower(rhs.Value)
	switch lower {
	case "new":
		p.sc.NextSkipComments()
		p.parseNewRef(tok.Value, tok.Line)
	case "createobject":
		p.sc.NextSkipComments()
		p.parseCreateObjectRef(tok.Value, tok.Line)
	case "entitynew":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(tok.Value, tok.Line)
	}
}

func (p *shallowScriptParser) parseNewRef(varName string, line int) {
	tok := p.sc.NextSkipComments()
	var component string
	if tok.Kind == TokString {
		component = unquote(tok.Value)
	} else if tok.Kind == TokIdent {
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
	}
	if component != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable: varName, Component: component,
			URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
		})
	}
}

func (p *shallowScriptParser) parseCreateObjectRef(varName string, line int) {
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}
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
	comp := unquote(arg2.Value)
	if comp != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable: varName, Component: comp,
			URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
		})
	}
}

func (p *shallowScriptParser) parseEntityNewRef(varName string, line int) {
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}
	arg := p.sc.NextSkipComments()
	if arg.Kind != TokString {
		return
	}
	comp := unquote(arg.Value)
	if comp != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable: varName, Component: comp,
			URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
		})
	}
}

// globalScriptParser extracts only variable declarations outside function bodies.
type globalScriptParser struct {
	sc       *Scanner
	vars     []VarDef
	baseLine int
	scopes   []FuncScope
}

func newGlobalScriptParser(src string, baseLine int, scopes []FuncScope) *globalScriptParser {
	return &globalScriptParser{
		sc:       NewScanner(src),
		baseLine: baseLine,
		scopes:   scopes,
	}
}

func (p *globalScriptParser) parse() {
	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return
		}
		if tok.Kind != TokIdent {
			continue
		}
		line := p.baseLine + tok.Line
		// Skip if inside a function
		if findFuncScope(line, p.scopes).Start != -1 {
			// Skip to end of function body
			p.skipPastScope(line)
			continue
		}
		lower := strings.ToLower(tok.Value)
		switch lower {
		case "var":
			p.parseVar(tok)
		case "local":
			p.parseDot(tok, ScopeVariables) // local outside func → variables
		case "this":
			p.parseDot(tok, ScopeThis)
		case "variables":
			p.parseDot(tok, ScopeVariables)
		case "function", "public", "private", "remote", "package":
			// skip function declarations
		default:
			p.parsePlain(tok)
		}
	}
}

func (p *globalScriptParser) skipPastScope(line int) {
	fs := findFuncScope(line, p.scopes)
	if fs.Start == -1 {
		return
	}
	// Skip tokens until we're past the function's end line
	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return
		}
		if p.baseLine+tok.Line > fs.End {
			return
		}
	}
}

func (p *globalScriptParser) parseVar(tok Token) {
	name := p.sc.NextSkipComments()
	if name.Kind != TokIdent {
		return
	}
	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		return
	}
	p.vars = append(p.vars, VarDef{
		Name: name.Value, Scope: ScopeVariables,
		Line: uint32(p.baseLine + tok.Line),
	})
}

func (p *globalScriptParser) parseDot(tok Token, scope Scope) {
	dot := p.sc.PeekSkipComments()
	if dot.Kind != TokDot {
		return
	}
	p.sc.NextSkipComments()
	name := p.sc.NextSkipComments()
	if name.Kind != TokIdent {
		return
	}
	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		return
	}
	p.vars = append(p.vars, VarDef{
		Name: name.Value, Scope: scope,
		Line: uint32(p.baseLine + tok.Line),
	})
}

func (p *globalScriptParser) parsePlain(tok Token) {
	if isKeyword(tok.Value) {
		return
	}
	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		return
	}
	p.vars = append(p.vars, VarDef{
		Name: tok.Value, Scope: ScopeVariables,
		Line: uint32(p.baseLine + tok.Line),
	})
}

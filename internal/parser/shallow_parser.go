package parser

import "strings"

// propertyDef holds parsed property metadata.
type propertyDef struct {
	name     string
	typeName string
	line     uint32
	attrs    map[string]string // all attribute key=value pairs (lowercase keys)
}

// shallowScriptParser extracts function signatures and component refs
// without parsing function bodies for variables.
type shallowScriptParser struct {
	sc         *Scanner
	funcs      []FunctionDef
	refs       []ComponentRef
	scopes     []FuncScope
	properties []propertyDef
	extends    string
	persistent bool
	fileURI    string
	baseLine   int
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
		case "component":
			p.parseComponentAttrs()
		case "property":
			p.parseProperty(tok)
		default:
			// Check for component refs in assignments: ident = new/createObject/entityNew
			p.checkAssignRef(tok)
		}
	}
}

// parseProperty handles script-style property declarations:
//
//	property name="person" type="models.Person";
//	property string name;
//	property name;
func (p *shallowScriptParser) parseProperty(startTok Token) {
	line := uint32(p.baseLine + startTok.Line)

	var name, typeName string

	attrs := make(map[string]string)

	// Collect tokens until semicolon or EOF
	var tokens []Token

	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF || tok.Kind == TokSemicolon {
			break
		}

		tokens = append(tokens, tok)
	}

	// Check for attribute-style: property name="x" type="y" inject="z";
	for i, tok := range tokens {
		if tok.Kind == TokIdent && i+1 < len(tokens) && tokens[i+1].Kind == TokEquals {
			if i+2 < len(tokens) && tokens[i+2].Kind == TokString {
				val := unquote(tokens[i+2].Value)
				attrs[strings.ToLower(tok.Value)] = val
			}
		}
	}

	name = attrs["name"]
	typeName = attrs["type"]

	// If no name= attribute, try positional: property [type] name [attrs...];
	if name == "" {
		idents := make([]string, 0, 2)

		for i, tok := range tokens {
			if tok.Kind == TokIdent {
				// Stop if this ident is followed by = (it's an attribute, not positional)
				if i+1 < len(tokens) && tokens[i+1].Kind == TokEquals {
					break
				}

				idents = append(idents, tok.Value)
			} else {
				break
			}
		}

		switch len(idents) {
		case 1:
			name = idents[0]
		case 2:
			typeName = idents[0]
			name = idents[1]
		}
	}

	if name == "" {
		return
	}

	p.properties = append(p.properties, propertyDef{name: name, typeName: typeName, line: line, attrs: attrs})
}

func (p *shallowScriptParser) parseComponentAttrs() {
	for {
		tok := p.sc.PeekSkipComments()
		if tok.Kind == TokLBrace || tok.Kind == TokEOF {
			return
		}

		p.sc.NextSkipComments()

		if tok.Kind == TokIdent {
			if strings.EqualFold(tok.Value, "extends") {
				eq := p.sc.PeekSkipComments()
				if eq.Kind == TokEquals {
					p.sc.NextSkipComments()

					val := p.sc.NextSkipComments()
					if val.Kind == TokString {
						p.extends = unquote(val.Value)
					}
				}
			} else if strings.EqualFold(tok.Value, "persistent") {
				eq := p.sc.PeekSkipComments()
				if eq.Kind == TokEquals {
					p.sc.NextSkipComments()

					val := p.sc.NextSkipComments()
					if val.Kind == TokString && isTruthy(unquote(val.Value)) {
						p.persistent = true
					} else if val.Kind == TokIdent && isTruthy(val.Value) {
						p.persistent = true
					}
				}
			}
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

func (p *shallowScriptParser) parseFunction(startTok Token, access string, returnType string) {
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

	// Create component refs for arguments with component-like types
	for _, a := range args {
		if isComponentType(a.Type) {
			p.refs = append(p.refs, ComponentRef{
				Variable:  a.Name,
				Component: a.Type,
				URI:       uriFromString(p.fileURI),
				Line:      uint32(funcLine),
			})
		}
	}

	p.funcs = append(p.funcs, FunctionDef{
		Name:      nameTok.Value,
		URI:       uriFromString(p.fileURI),
		Line:      uint32(funcLine),
		Arguments: args,
	})

	// Record scope and skip body
	endLine := p.skipBody()
	p.scopes = append(p.scopes, FuncScope{Name: nameTok.Value, Access: access, ReturnType: returnType, Start: funcLine, End: p.baseLine + endLine})
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

	loop:
		for {
			peek := p.sc.PeekSkipComments()
			switch peek.Kind { //nolint:exhaustive // only care about dot and ident
			case TokDot:
				p.sc.NextSkipComments() // consume dot

				next := p.sc.PeekSkipComments()
				if next.Kind == TokIdent {
					p.sc.NextSkipComments()

					idents[len(idents)-1] += "." + next.Value
				}
			case TokIdent:
				p.sc.NextSkipComments()

				idents = append(idents, peek.Value)
				if len(idents) == 3 {
					break loop
				}
			default:
				break loop
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

		switch peek.Kind { //nolint:exhaustive
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
	case "entityload":
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
		case "property":
			// skip property declaration tokens until semicolon
			for {
				t := p.sc.NextSkipComments()
				if t.Kind == TokEOF || t.Kind == TokSemicolon {
					break
				}
			}
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

func isTruthy(s string) bool {
	return strings.EqualFold(s, "true") || strings.EqualFold(s, "yes")
}

// looksLikeCFCType returns true if a property type looks like a CFC reference
// (dotted path or non-primitive name).
func looksLikeCFCType(t string) bool {
	if strings.Contains(t, ".") {
		return true
	}

	switch strings.ToLower(t) {
	case "string", "numeric", "boolean", "date", "struct", "array", "query",
		"binary", "guid", "uuid", "void", "any", "xml", "function":
		return false
	}

	return true
}

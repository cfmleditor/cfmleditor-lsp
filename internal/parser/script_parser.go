package parser

import "strings"

// propertyDef holds parsed property metadata.
type propertyDef struct {
	name     string
	typeName string
	line     uint32
	attrs    map[string]string // all attribute key=value pairs (lowercase keys)
}

// scriptParser extracts function signatures, component refs, and variable
// declarations from CFScript source in a single pass.
type scriptParser struct {
	sc                  *Scanner
	funcs               []FunctionDef
	vars                []VarDef
	componentRefs       []ComponentRef
	funcRefs            map[string][]ComponentRef // keyed by "start:end"
	funcLinks           map[string][]DocumentLink // keyed by "start:end"
	links               []DocumentLink            // global scope links
	scopes              []FuncScope
	properties          []propertyDef
	extends             string
	persistent          bool
	fileURI             string
	baseLine            int
	resolvers           []Resolver
	extractLinks        bool // whether to extract document links
	builtinReturnLookup func(string) string
	inFunc              string          // current function scope key, empty if global
	localVarSet         map[string]bool // var'd/local. names in current function
	forceGlobal         bool            // when true, addRef routes to componentRefs
	returnVar           string          // last "return varName" seen in current function
	pendingCalls        []pendingCall   // unresolved varName = funcCall(...) assignments
}

// pendingCall records an unresolved assignment from a function call.
type pendingCall struct {
	varName  string
	funcName string
	baseVar  string // for x = baseVar.method() — resolve x to same component as baseVar
	line     uint32
	funcKey  string // scope key, empty if global
}

func newScriptParser(src, fileURI string, baseLine int, resolvers []Resolver) *scriptParser {
	return &scriptParser{
		sc:        NewScanner(src),
		fileURI:   fileURI,
		baseLine:  baseLine,
		resolvers: resolvers,
	}
}

func (p *scriptParser) addRef(ref ComponentRef) {
	if p.inFunc == "" || p.forceGlobal {
		p.componentRefs = append(p.componentRefs, ref)
	} else {
		if p.funcRefs == nil {
			p.funcRefs = make(map[string][]ComponentRef)
		}

		p.funcRefs[p.inFunc] = append(p.funcRefs[p.inFunc], ref)
	}
}

func (p *scriptParser) isVarDeclaredLocal(name string) bool {
	if p.localVarSet == nil {
		return false
	}

	return p.localVarSet[strings.ToLower(name)]
}

// extractAllLinks scans source lines for document links, routing them to
// global links or funcLinks based on which scope the line falls in.
func (p *scriptParser) extractAllLinks() {
	src := p.sc.src
	lineNum := p.baseLine
	scopeIdx := 0

	for len(src) > 0 {
		nl := strings.IndexByte(src, '\n')

		var line string
		if nl < 0 {
			line = src
			src = ""
		} else {
			line = src[:nl]
			src = src[nl+1:]
		}

		// Find which scope this line belongs to
		for scopeIdx < len(p.scopes) && lineNum > p.scopes[scopeIdx].End {
			scopeIdx++
		}

		if scopeIdx < len(p.scopes) && lineNum > p.scopes[scopeIdx].Start && lineNum < p.scopes[scopeIdx].End {
			key := funcKey(p.scopes[scopeIdx].Start, p.scopes[scopeIdx].End)
			if p.funcLinks == nil {
				p.funcLinks = make(map[string][]DocumentLink)
			}

			links := p.funcLinks[key]
			extractLinksFromLine(line, lineNum, &links)
			p.funcLinks[key] = links
		} else {
			extractLinksFromLine(line, lineNum, &p.links)
		}

		lineNum++
	}
}

// parseVarDecl handles: var name = expr
func (p *scriptParser) parseVarDecl(tok Token) {
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		return
	}

	p.sc.NextSkipComments() // consume =

	if p.localVarSet != nil {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	}

	p.vars = append(p.vars, VarDef{
		Name: nameTok.Value, Scope: ScopeLocal,
		Line: uint32(p.baseLine + tok.Line),
	})

	// Check RHS for component refs
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		return
	}

	switch strings.ToLower(rhs.Value) {
	case "new":
		p.sc.NextSkipComments()
		p.parseNewRef(nameTok.Value, tok.Line)
	case "createobject":
		p.sc.NextSkipComments()
		p.parseCreateObjectRef(nameTok.Value, tok.Line)
	case "entitynew":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(nameTok.Value, tok.Line)
	case "entityload":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(nameTok.Value, tok.Line)
	default:
		p.checkVarRHS(nameTok.Value, tok.Line)
	}
}

// checkVarRHS handles unrecognized RHS for var declarations: walks dot chain,
// tries resolver match or records pending call.
func (p *scriptParser) checkVarRHS(varName string, line int) {
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent || isKeyword(rhs.Value) {
		return
	}

	p.sc.NextSkipComments() // consume first ident

	prevIdent := ""
	lastIdent := rhs.Value
	fullChain := rhs.Value

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		next := p.sc.PeekSkipComments()
		if next.Kind == TokIdent {
			p.sc.NextSkipComments()

			prevIdent = lastIdent
			lastIdent = next.Value
			fullChain += "." + next.Value
		} else {
			break
		}
	}

	if p.sc.PeekSkipComments().Kind == TokLParen {
		if comp := p.tryResolveCall(fullChain); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		} else if p.builtinReturnLookup != nil {
			if comp := p.builtinReturnLookup(lastIdent); comp != "" {
				p.addRef(ComponentRef{
					Variable: varName, Component: comp,
					URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
				})
			} else {
				p.pendingCalls = append(p.pendingCalls, pendingCall{
					varName: varName, funcName: lastIdent, baseVar: prevIdent,
					line: uint32(p.baseLine + line), funcKey: p.inFunc,
				})
			}
		} else {
			p.pendingCalls = append(p.pendingCalls, pendingCall{
				varName: varName, funcName: lastIdent, baseVar: prevIdent,
				line: uint32(p.baseLine + line), funcKey: p.inFunc,
			})
		}
	} else if len(p.resolvers) > 0 {
		if comp := ResolveFromCall(fullChain, p.resolvers); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		}
	}
}

// parseScopedVar handles: scope.name = expr (local., arguments., this., variables.)
func (p *scriptParser) parseScopedVar(tok Token, scope Scope) {
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
		Name: nameTok.Value, Scope: scope,
		Line: uint32(p.baseLine + tok.Line),
	})

	isLocal := scope == ScopeLocal || scope == ScopeArguments
	if isLocal && p.localVarSet != nil {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	}

	if !isLocal {
		p.forceGlobal = true
	}

	// Check RHS for component refs
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind == TokIdent {
		switch strings.ToLower(rhs.Value) {
		case "new":
			p.sc.NextSkipComments()
			p.parseNewRef(nameTok.Value, tok.Line)
		case "createobject":
			p.sc.NextSkipComments()
			p.parseCreateObjectRef(nameTok.Value, tok.Line)
		case "entitynew":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, tok.Line)
		case "entityload":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, tok.Line)
		default:
			p.checkVarRHS(nameTok.Value, tok.Line)
		}
	}

	p.forceGlobal = false
}

func (p *scriptParser) parse() {
	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			break
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
		case "var":
			p.parseVarDecl(tok)
		case "local":
			p.parseScopedVar(tok, ScopeLocal)
		case "arguments":
			p.parseScopedVar(tok, ScopeArguments)
		case "this":
			p.parseScopedVar(tok, ScopeThis)
		case "variables":
			p.parseScopedVar(tok, ScopeVariables)
		default:
			// Check for returnType function pattern (e.g. "string function getName()")
			peek := p.sc.PeekSkipComments()
			switch {
			case peek.Kind == TokIdent && identEq(peek.Value, "function"):
				p.sc.NextSkipComments()
				p.parseFunction(tok, "", tok.Value)
			case peek.Kind == TokDot:
				// Could be dotted return type: models.User function ...
				var retVal strings.Builder
				retVal.WriteString(tok.Value)

				for p.sc.PeekSkipComments().Kind == TokDot {
					p.sc.NextSkipComments()

					seg := p.sc.NextSkipComments()
					if seg.Kind == TokIdent {
						retVal.WriteByte('.')
						retVal.WriteString(seg.Value)
					}
				}

				if next := p.sc.PeekSkipComments(); next.Kind == TokIdent && identEq(next.Value, "function") {
					p.sc.NextSkipComments()
					p.parseFunction(tok, "", retVal.String())
				}
			default:
				p.checkAssignRef(tok)
			}
		}
	}

	if p.extractLinks {
		p.extractAllLinks()
	}
}

// parseProperty handles script-style property declarations:
//
//	property name="person" type="models.Person";
//	property string name;
//	property name;
func (p *scriptParser) parseProperty(startTok Token) {
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

func (p *scriptParser) parseComponentAttrs() {
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

func (p *scriptParser) parseAccessModified(accessTok Token) {
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

	var retVal strings.Builder
	retVal.WriteString(retType.Value)

	// Handle dotted return types (e.g. models.User)
	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments()

		seg := p.sc.NextSkipComments()
		if seg.Kind == TokIdent {
			retVal.WriteByte('.')
			retVal.WriteString(seg.Value)
		}
	}

	next2 := p.sc.PeekSkipComments()
	if next2.Kind == TokIdent && identEq(next2.Value, "function") {
		p.sc.NextSkipComments()
		p.parseFunction(accessTok, accessTok.Value, retVal.String())
	}
}

func (p *scriptParser) parseFunction(startTok Token, access string, returnType string) {
	// Capture JSDoc comment that preceded this function
	docComment := p.sc.LastBlockComment
	p.sc.LastBlockComment = ""

	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

	args := p.parseArgList()

	// Apply JSDoc @param {type} annotations to arguments
	if docComment != "" {
		applyJSDocParams(docComment, args)
	}

	funcLine := p.baseLine + startTok.Line

	// Create component refs for arguments with component-like types
	for _, a := range args {
		if isComponentType(a.Type) {
			p.componentRefs = append(p.componentRefs, ComponentRef{
				Variable:  a.Name,
				Component: a.Type,
				URI:       uriFromString(p.fileURI),
				Line:      uint32(funcLine),
			})
		}
	}

	p.funcs = append(p.funcs, FunctionDef{
		Name:       nameTok.Value,
		URI:        uriFromString(p.fileURI),
		Line:       uint32(funcLine),
		Arguments:  args,
		ReturnType: returnType,
	})

	// Process body: set inFunc scope, parse assignments, then clear
	endLine := p.parseBody(funcLine, args)
	p.scopes = append(p.scopes, FuncScope{Name: nameTok.Value, Access: access, ReturnType: returnType, Start: funcLine, End: p.baseLine + endLine})
}

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

func (p *scriptParser) skipDefault() {
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

// parseBody processes { ... } or ; for a function body, extracting refs.
// Sets inFunc/localVarSet on entry, clears on exit. Returns the line of the closing token.
func (p *scriptParser) parseBody(funcLine int, args []Argument) int {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		t := p.sc.NextSkipComments()

		return t.Line
	}

	if tok.Kind != TokLBrace {
		return tok.Line
	}

	p.sc.NextSkipComments() // consume {

	// Enter function scope with a temporary key (will fix up after finding end)
	prevInFunc := p.inFunc
	prevLocalVarSet := p.localVarSet
	prevReturnVar := p.returnVar

	// Use a placeholder funcKey; we'll remap after finding the real end
	tempKey := funcKey(funcLine, funcLine)
	p.inFunc = tempKey
	p.returnVar = ""

	p.localVarSet = make(map[string]bool)
	for _, a := range args {
		p.localVarSet[strings.ToLower(a.Name)] = true
	}

	depth := 1

	var endLine int

	for depth > 0 {
		t := p.sc.NextSkipComments()
		if t.Kind == TokEOF {
			p.inFunc = prevInFunc
			p.localVarSet = prevLocalVarSet
			p.returnVar = prevReturnVar

			return t.Line
		}

		switch t.Kind { //nolint:exhaustive
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
			if depth == 0 {
				endLine = t.Line
			}
		case TokIdent:
			if depth > 0 {
				p.handleBodyToken(t, depth)
			}
		}
	}

	// Remap funcRefs from temp key to real key
	realKey := funcKey(funcLine, p.baseLine+endLine)
	if p.funcRefs != nil {
		if refs, ok := p.funcRefs[tempKey]; ok {
			delete(p.funcRefs, tempKey)
			p.funcRefs[realKey] = refs
		}
	}

	// Remap pending calls
	for i := range p.pendingCalls {
		if p.pendingCalls[i].funcKey == tempKey {
			p.pendingCalls[i].funcKey = realKey
		}
	}

	p.inFunc = realKey

	// Resolve ReturnComponent on the current function
	if len(p.funcs) > 0 {
		f := &p.funcs[len(p.funcs)-1]
		if f.ReturnComponent == "" && p.returnVar != "" {
			// Look up returnVar in this function's refs
			if refs := p.funcRefs[p.inFunc]; refs != nil {
				for _, ref := range refs {
					if strings.EqualFold(ref.Variable, p.returnVar) {
						f.ReturnComponent = ref.Component

						break
					}
				}
			}
			// Also check componentRefs (for variables./this. scoped)
			if f.ReturnComponent == "" {
				for _, ref := range p.componentRefs {
					if strings.EqualFold(ref.Variable, p.returnVar) {
						f.ReturnComponent = ref.Component

						break
					}
				}
			}
			// If still unresolved, store for deferred resolution
			if f.ReturnComponent == "" {
				f.returnVar = p.returnVar
			}
		}
	}

	// Exit function scope
	p.inFunc = prevInFunc
	p.localVarSet = prevLocalVarSet
	p.returnVar = prevReturnVar

	return endLine
}

// handleBodyToken processes an identifier inside a function body.
func (p *scriptParser) handleBodyToken(tok Token, depth int) {
	lower := strings.ToLower(tok.Value)
	switch lower {
	case "var":
		p.parseBodyVarDecl(tok)
	case "local":
		p.parseBodyScopedVar(tok, ScopeLocal)
	case "arguments":
		p.parseBodyScopedVar(tok, ScopeArguments)
	case "variables":
		p.parseBodyScopedVar(tok, ScopeVariables)
	case "this":
		p.parseBodyScopedVar(tok, ScopeThis)
	case "return":
		p.checkReturnComponent()
	case "function", "public", "private", "remote", "package":
		// Nested function — skip its entire body
		p.skipNestedFunction(tok, depth)
	default:
		p.checkAssignRef(tok)
	}
}

// checkReturnComponent checks if a return statement returns a component expression or variable.
func (p *scriptParser) checkReturnComponent() {
	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokIdent {
		return
	}

	var comp string

	switch strings.ToLower(peek.Value) {
	case "new":
		p.sc.NextSkipComments()
		comp = p.readNewComponent()
	case "createobject":
		p.sc.NextSkipComments()
		comp = p.readCreateObjectComponent()
	case "entitynew":
		p.sc.NextSkipComments()
		comp = p.readEntityNewComponent()
	default:
		// return varName — track for resolution after body parse
		p.returnVar = peek.Value

		return
	}

	if comp != "" && len(p.funcs) > 0 {
		p.funcs[len(p.funcs)-1].ReturnComponent = comp
	}
}

// readNewComponent reads the component path after "new" keyword.
func (p *scriptParser) readNewComponent() string {
	tok := p.sc.NextSkipComments()
	if tok.Kind == TokString {
		return unquote(tok.Value)
	}

	if tok.Kind == TokIdent {
		var comp strings.Builder
		comp.WriteString(tok.Value)

		for {
			if p.sc.PeekSkipComments().Kind == TokDot {
				p.sc.NextSkipComments()

				next := p.sc.NextSkipComments()
				if next.Kind == TokIdent {
					comp.WriteByte('.')
					comp.WriteString(next.Value)
				}
			} else {
				break
			}
		}

		return comp.String()
	}

	return ""
}

// readCreateObjectComponent reads the component path from createObject("component","path").
func (p *scriptParser) readCreateObjectComponent() string {
	if p.sc.NextSkipComments().Kind != TokLParen {
		return ""
	}

	arg1 := p.sc.NextSkipComments()
	if arg1.Kind != TokString || !identEq(unquote(arg1.Value), "component") {
		return ""
	}

	if p.sc.NextSkipComments().Kind != TokComma {
		return ""
	}

	arg2 := p.sc.NextSkipComments()
	if arg2.Kind != TokString {
		return ""
	}

	return unquote(arg2.Value)
}

// readEntityNewComponent reads the entity name from entityNew("Name").
func (p *scriptParser) readEntityNewComponent() string {
	if p.sc.NextSkipComments().Kind != TokLParen {
		return ""
	}

	arg := p.sc.NextSkipComments()
	if arg.Kind != TokString {
		return ""
	}

	return unquote(arg.Value)
}

// parseBodyVarDecl handles: var name = expr inside a function body.
func (p *scriptParser) parseBodyVarDecl(varTok Token) {
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		return
	}

	p.sc.NextSkipComments() // consume =

	p.localVarSet[strings.ToLower(nameTok.Value)] = true
	p.vars = append(p.vars, VarDef{
		Name: nameTok.Value, Scope: ScopeLocal,
		Line: uint32(p.baseLine + varTok.Line),
	})

	// Check RHS for component refs
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		return
	}

	rhsLower := strings.ToLower(rhs.Value)
	switch rhsLower {
	case "new":
		p.sc.NextSkipComments()
		p.parseNewRef(nameTok.Value, varTok.Line)
	case "createobject":
		p.sc.NextSkipComments()
		p.parseCreateObjectRef(nameTok.Value, varTok.Line)
	case "entitynew":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(nameTok.Value, varTok.Line)
	case "entityload":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(nameTok.Value, varTok.Line)
	default:
		if !isKeyword(rhs.Value) {
			p.sc.NextSkipComments()

			prevIdent := ""
			lastIdent := rhs.Value
			fullChain := rhs.Value

			for p.sc.PeekSkipComments().Kind == TokDot {
				p.sc.NextSkipComments()

				next := p.sc.PeekSkipComments()
				if next.Kind == TokIdent {
					p.sc.NextSkipComments()

					prevIdent = lastIdent
					lastIdent = next.Value
					fullChain += "." + next.Value
				} else {
					break
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				if comp := p.tryResolveCall(fullChain); comp != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: comp,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + varTok.Line),
					})
				} else {
					p.pendingCalls = append(p.pendingCalls, pendingCall{
						varName:  nameTok.Value,
						funcName: lastIdent,
						baseVar:  prevIdent,
						line:     uint32(p.baseLine + varTok.Line),
						funcKey:  p.inFunc,
					})
				}
			} else if len(p.resolvers) > 0 {
				if comp := ResolveFromCall(fullChain, p.resolvers); comp != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: comp,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + varTok.Line),
					})
				}
			}
		}
	}
}

// parseBodyScopedVar handles: scope.name = expr inside a function body.
func (p *scriptParser) parseBodyScopedVar(scopeTok Token, scope Scope) {
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
		Name: nameTok.Value, Scope: scope,
		Line: uint32(p.baseLine + scopeTok.Line),
	})

	isLocal := scope == ScopeLocal || scope == ScopeArguments
	if isLocal {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	} else {
		p.forceGlobal = true
	}

	// Check RHS for component refs
	rhs := p.sc.PeekSkipComments()
	if rhs.Kind == TokIdent {
		switch strings.ToLower(rhs.Value) {
		case "new":
			p.sc.NextSkipComments()
			p.parseNewRef(nameTok.Value, scopeTok.Line)
		case "createobject":
			p.sc.NextSkipComments()
			p.parseCreateObjectRef(nameTok.Value, scopeTok.Line)
		case "entitynew":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, scopeTok.Line)
		case "entityload":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, scopeTok.Line)
		default:
			if !isKeyword(rhs.Value) {
				p.sc.NextSkipComments()

				prevIdent := ""
				lastIdent := rhs.Value
				fullChain := rhs.Value

				for p.sc.PeekSkipComments().Kind == TokDot {
					p.sc.NextSkipComments()

					next := p.sc.PeekSkipComments()
					if next.Kind == TokIdent {
						p.sc.NextSkipComments()

						prevIdent = lastIdent
						lastIdent = next.Value
						fullChain += "." + next.Value
					} else {
						break
					}
				}

				if p.sc.PeekSkipComments().Kind == TokLParen {
					if comp := p.tryResolveCall(fullChain); comp != "" {
						p.addRef(ComponentRef{
							Variable: nameTok.Value, Component: comp,
							URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + scopeTok.Line),
						})
					} else {
						p.pendingCalls = append(p.pendingCalls, pendingCall{
							varName:  nameTok.Value,
							funcName: lastIdent,
							baseVar:  prevIdent,
							line:     uint32(p.baseLine + scopeTok.Line),
							funcKey:  p.inFunc,
						})
					}
				} else if len(p.resolvers) > 0 {
					if comp := ResolveFromCall(fullChain, p.resolvers); comp != "" {
						p.addRef(ComponentRef{
							Variable: nameTok.Value, Component: comp,
							URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + scopeTok.Line),
						})
					}
				}
			}
		}
	}

	p.forceGlobal = false
}

// skipNestedFunction skips a nested function declaration and its body.
func (p *scriptParser) skipNestedFunction(tok Token, _ int) {
	lower := strings.ToLower(tok.Value)
	// Handle access modifier before function keyword
	if lower != "function" {
		next := p.sc.PeekSkipComments()
		if next.Kind != TokIdent {
			return
		}

		if identEq(next.Value, "function") {
			p.sc.NextSkipComments()
		} else {
			// access returnType function
			p.sc.NextSkipComments()

			next2 := p.sc.PeekSkipComments()
			if next2.Kind == TokIdent && identEq(next2.Value, "function") {
				p.sc.NextSkipComments()
			} else {
				return
			}
		}
	}
	// Skip name (or handle anonymous function)
	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind == TokLParen {
		// Anonymous function: function() { ... }
		// Already consumed (, skip args
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

		p.skipBody()

		return
	}

	if nameTok.Kind != TokIdent {
		return
	}
	// Skip args
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

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
	// Skip body
	p.skipBody()
}

// skipBody skips { ... } or ; without processing content.
func (p *scriptParser) skipBody() {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		p.sc.NextSkipComments()

		return
	}

	if tok.Kind != TokLBrace {
		return
	}

	p.sc.NextSkipComments()

	depth := 1

	for depth > 0 {
		t := p.sc.NextSkipComments()
		if t.Kind == TokEOF {
			return
		}

		if t.Kind == TokLBrace {
			depth++
		}

		if t.Kind == TokRBrace {
			depth--
		}
	}
}

func (p *scriptParser) checkAssignRef(tok Token) {
	if isKeyword(tok.Value) {
		return
	}

	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		return
	}

	p.sc.NextSkipComments() // consume =

	// Record the variable
	p.vars = append(p.vars, VarDef{
		Name: tok.Value, Scope: ScopeVariables,
		Line: uint32(p.baseLine + tok.Line),
	})

	// If inside a function and variable is NOT declared local, route to componentRefs
	if p.inFunc != "" && !p.isVarDeclaredLocal(tok.Value) {
		p.forceGlobal = true
	}

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		p.forceGlobal = false

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
	default:
		// Check if RHS is a function call: funcName( or someVar.method(
		if !isKeyword(rhs.Value) {
			p.sc.NextSkipComments() // consume first ident

			// Walk dot chain: [scope.]varName.method(
			prevIdent := ""
			lastIdent := rhs.Value
			fullChain := rhs.Value

			for p.sc.PeekSkipComments().Kind == TokDot {
				p.sc.NextSkipComments() // consume .

				next := p.sc.PeekSkipComments()
				if next.Kind == TokIdent {
					p.sc.NextSkipComments()

					prevIdent = lastIdent
					lastIdent = next.Value
					fullChain += "." + next.Value
				} else {
					break
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				if comp := p.tryResolveCall(fullChain); comp != "" {
					p.addRef(ComponentRef{
						Variable: tok.Value, Component: comp,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + tok.Line),
					})
				} else {
					p.pendingCalls = append(p.pendingCalls, pendingCall{
						varName:  tok.Value,
						funcName: lastIdent,
						baseVar:  prevIdent,
						line:     uint32(p.baseLine + tok.Line),
						funcKey:  p.inFunc,
					})
				}
			} else if len(p.resolvers) > 0 {
				// Try generic resolver match on non-call RHS (e.g. "_parent")
				if comp := ResolveFromCall(fullChain, p.resolvers); comp != "" {
					p.addRef(ComponentRef{
						Variable: tok.Value, Component: comp,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + tok.Line),
					})
				}
			}
		}
	}

	p.forceGlobal = false
}

func (p *scriptParser) parseNewRef(varName string, line int) {
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
		p.addRef(ComponentRef{
			Variable: varName, Component: component,
			URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
		})
	}
}

func (p *scriptParser) parseCreateObjectRef(varName string, line int) {
	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

	arg1 := p.sc.NextSkipComments()
	if arg1.Kind != TokString {
		return
	}

	arg1Val := unquote(arg1.Value)

	if identEq(arg1Val, "component") {
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
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		}
	} else if len(p.resolvers) > 0 {
		// Try resolvers for non-component createObject (e.g. java)
		comma := p.sc.NextSkipComments()
		if comma.Kind != TokComma {
			return
		}

		arg2 := p.sc.NextSkipComments()
		if arg2.Kind != TokString {
			return
		}

		expr := "createObject(\"" + arg1Val + "\",\"" + unquote(arg2.Value) + "\")"
		if comp := ResolveFromCall(expr, p.resolvers); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		}
	}
}

func (p *scriptParser) parseEntityNewRef(varName string, line int) {
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
		p.addRef(ComponentRef{
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

// tryResolveCall attempts to resolve a function call via resolvers.
// Called when scanner is positioned at '(' (peeked, not consumed).
// Reconstructs the call expression (e.g. getService("foo")) and tries resolvers.
func (p *scriptParser) tryResolveCall(callExpr string) string {
	if len(p.resolvers) == 0 {
		return ""
	}

	// Quick prefix check
	hasPrefix := false

	for i := range p.resolvers {
		if p.resolvers[i].Prefix == "" || containsFold(callExpr, p.resolvers[i].Prefix) {
			hasPrefix = true

			break
		}
	}

	if !hasPrefix {
		return ""
	}

	// Save position — peek ahead to build the expression
	saved := p.sc.Save()
	p.sc.NextSkipComments() // consume (

	arg := p.sc.PeekSkipComments()
	if arg.Kind == TokString {
		p.sc.NextSkipComments()

		// Build arg list: read comma-separated string args
		args := "\"" + unquote(arg.Value) + "\""

		for {
			next := p.sc.PeekSkipComments()
			if next.Kind != TokComma {
				break
			}

			p.sc.NextSkipComments() // consume ,

			nextArg := p.sc.PeekSkipComments()
			if nextArg.Kind != TokString {
				break
			}

			p.sc.NextSkipComments()

			args += ", \"" + unquote(nextArg.Value) + "\""
		}

		expr := callExpr + "(" + args + ")"
		if comp := ResolveFromCall(expr, p.resolvers); comp != "" {
			return comp
		}
	} else {
		// Try no-arg match: callExpr()
		expr := callExpr + "()"
		if comp := ResolveFromCall(expr, p.resolvers); comp != "" {
			return comp
		}
	}

	// No match — restore scanner
	p.sc.Restore(saved)

	return ""
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

// applyJSDocParams parses @param {type} name annotations from a JSDoc comment
// and sets the Type on matching arguments (only if their Type is empty or "any").
func applyJSDocParams(comment string, args []Argument) {
	for len(comment) > 0 {
		idx := strings.Index(comment, "@param")
		if idx < 0 {
			break
		}

		comment = comment[idx+6:]

		// Skip whitespace
		i := 0
		for i < len(comment) && (comment[i] == ' ' || comment[i] == '\t') {
			i++
		}

		if i >= len(comment) || comment[i] != '{' {
			continue
		}

		// Extract type inside braces
		i++ // skip {

		end := strings.IndexByte(comment[i:], '}')
		if end < 0 {
			break
		}

		typeName := strings.TrimSpace(comment[i : i+end])
		comment = comment[i+end+1:]

		if typeName == "" {
			continue
		}

		// Skip whitespace to get param name
		i = 0
		for i < len(comment) && (comment[i] == ' ' || comment[i] == '\t') {
			i++
		}

		// Read param name
		nameStart := i
		for i < len(comment) && comment[i] != ' ' && comment[i] != '\t' && comment[i] != '\n' && comment[i] != '\r' && comment[i] != '*' {
			i++
		}

		paramName := comment[nameStart:i]
		if paramName == "" {
			continue
		}

		// Match to argument and override type if it's generic
		for j := range args {
			if strings.EqualFold(args[j].Name, paramName) {
				if args[j].Type == "" || strings.EqualFold(args[j].Type, "any") || strings.EqualFold(args[j].Type, "struct") {
					args[j].Type = typeName
				}

				break
			}
		}
	}
}

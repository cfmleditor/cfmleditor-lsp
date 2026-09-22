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
	funcCalls           map[string][]CallSite     // keyed by "start:end"
	links               []DocumentLink            // global scope links
	calls               []CallSite                // global scope call sites
	scopes              []FuncScope
	properties          []propertyDef
	extends             string
	persistent          bool
	fileURI             string
	baseLine            int
	resolvers           []Resolver
	resolverSet         *ResolverSet
	extractLinks        bool              // whether to extract document links
	extractCalls        bool              // whether to extract all call sites
	argNesting          int               // recursion depth of skipParenBody's scan, bounded by maxArgNesting
	afterLT             bool              // previous token was '<' — see looksLikeTagAttrs
	imports             map[string]string // last segment (lowercased) → full dot-path, from `import`
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

func (p *scriptParser) resolveCall(expr string) string {
	if p.resolverSet != nil {
		return p.resolverSet.Resolve(expr)
	}

	return ResolveFromCall(expr, p.resolvers)
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

func (p *scriptParser) addCall(call CallSite) {
	// The single gate on recording a call. Consuming one is not gated: the
	// dispatch that walks a call and its argument list runs in every mode,
	// because a (...) group left unconsumed is not skipped — it is scanned as
	// if it were statements, and `svc.save(force = true)` then declares a
	// variable called `force`. See parseScriptTagAttrs for the other half of
	// that class of bug.
	if !p.extractCalls {
		return
	}

	if p.inFunc == "" {
		p.calls = append(p.calls, call)
	} else {
		if p.funcCalls == nil {
			p.funcCalls = make(map[string][]CallSite)
		}

		p.funcCalls[p.inFunc] = append(p.funcCalls[p.inFunc], call)
	}
}

// asCFScript marks the parser's text as genuine CFScript rather than a tag
// function's raw body, which turns on interpolation-aware string scanning.
//
// It is opt-in so that a caller which has not thought about it gets the safe
// scan: applied to markup the rule pairs the hashes of two `href="#…"`
// fragments and swallows what is between them. See Scanner.scanString.
func (p *scriptParser) asCFScript() *scriptParser {
	p.sc.interpStrings = true

	return p
}

// recordBareCallAndChain handles funcName(...) optionally followed by .method(...) chains.
func (p *scriptParser) recordBareCallAndChain(tok Token) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	p.addCall(CallSite{
		FuncName: tok.Value,
		Line:     uint32(p.baseLine + tok.Line),
		Caller:   caller,
	})

	// Consume the argument list, capturing the first string arg for the
	// resolver and recording any calls written inside it. This used to be a
	// second copy of scanParenBody's loop, which is why a call nested in a
	// *bare* call's arguments stayed invisible after the dotted path learned to
	// see one: writeOutput(svc.getName()) recorded only writeOutput.
	p.sc.NextSkipComments() // consume (

	firstArg, ok := p.scanParenBody()
	if !ok {
		return
	}

	// Build call expression for resolver: funcName("arg")
	callExpr := tok.Value
	if firstArg != "" {
		callExpr = tok.Value + "(\"" + firstArg + "\")"
	}

	// comp is this bare call's resolved return component (if any); it's the
	// base receiver for every subsequent chained hop below. Hops beyond the
	// first accumulate in chainHops so CanResolveCall can walk comp's type
	// forward through each intermediate call before checking the current one.
	comp := p.tryResolveCall(callExpr)
	funcName := tok.Value

	var chainHops []string

	first := true

	// Check for .method( chain
	for {
		if p.sc.PeekSkipComments().Kind == TokLBracket {
			// Dynamic index between hops (e.g. someFunc()[key].method()) —
			// skip it and keep walking the chain; no receiver identifier is
			// at risk of misattribution here (the base is a call return, not
			// a bare variable), so no poisoning is needed, just continuity.
			if !p.skipBracketIndex() {
				return
			}

			continue
		}

		if p.sc.PeekSkipComments().Kind != TokDot {
			break
		}

		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments() // consume method name

		if p.sc.PeekSkipComments().Kind == TokLParen {
			if !first {
				chainHops = append(chainHops, funcName)
			}

			first = false
			funcName = methTok.Value

			hops := make([]string, len(chainHops))
			copy(hops, chainHops)

			p.addCall(CallSite{
				FuncName:  funcName,
				Component: comp,
				Chain:     hops,
				Line:      uint32(p.baseLine + tok.Line),
				Caller:    caller,
				Resolved:  comp != "",
			})

			// Consume this hop's arguments too, so the chain walk can
			// continue — through the same scan, so a call written inside
			// them is found: this was the third copy of the paren loop and
			// the last one still losing `a().b(svc.c())`.
			p.sc.NextSkipComments() // consume (

			if _, ok := p.scanParenBody(); !ok {
				return
			}
		} else {
			break
		}
	}
}

// recordCallFromChain records a call site when a dot chain ending in ( is detected.
// fullChain is e.g. "VARIABLES.service.GetData" or just "GetData", line is the source line.
func (p *scriptParser) recordCallFromChain(fullChain string, line int) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	dotIdx := strings.LastIndexByte(fullChain, '.')
	if dotIdx < 0 {
		// Bare function call (no dot)
		p.addCall(CallSite{
			FuncName: fullChain,
			Line:     uint32(p.baseLine + line),
			Caller:   caller,
		})

		return
	}

	p.addCall(CallSite{
		FuncName: fullChain[dotIdx+1:],
		Variable: fullChain[:dotIdx],
		Line:     uint32(p.baseLine + line),
		Caller:   caller,
	})
}

// continueChainCalls consumes the rest of a chained call expression whose
// first hop's resolver lookup failed (tryResolveCall/tryExtendChain both
// returned "", falling through to a pendingCall) — e.g.
// "document.getJavaUtils().getRGBColor(r=16,g=58,b=59)" where
// "document.getJavaUtils" matched no resolver. tryResolveCall/tryExtendChain
// restore the scanner to the unconsumed "(" on failure, so without this the
// caller's next token read rediscovers ".getRGBColor(...)" as an orphaned,
// unqualified bare call ("no qualifier, not in file") instead of a call
// chained off baseVar. Must be called with the scanner positioned at that
// first "(". baseVar is the chain's receiver (may be "" for a bare call
// chain) and funcName is the first hop's own name; further hops get
// CallSites with Variable=baseVar and an accumulating Chain, matching
// checkBareCall's shape so CanResolveCall can walk them.
func (p *scriptParser) continueChainCalls(baseVar, funcName string, line int) {
	if !p.skipParens() {
		return
	}

	p.recordChainContinuation(baseVar, funcName, line)
}

// recordChainContinuation is continueChainCalls' shared core, factored out so
// it can also run after the first hop's resolver lookup SUCCEEDED — e.g.
// "document.getJavaUtils()" matching a "getJavaUtils" resolver even though
// the real expression continues ".getRGBColor(r=16,g=58,b=59)". In that case
// tryResolveCall has already consumed the first hop's own "(...)" (it must,
// to know where the match ends), so — unlike continueChainCalls — the
// caller here must already have the scanner positioned right after that
// closing ')', with no extra skipParens() call needed for the first hop.
func (p *scriptParser) recordChainContinuation(baseVar, funcName string, line int) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	var chainHops []string

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments() // consume method name

		if p.sc.PeekSkipComments().Kind != TokLParen {
			break
		}

		chainHops = append(chainHops, funcName)
		funcName = methTok.Value

		if !p.skipParens() {
			return
		}

		if p.extractCalls {
			hops := make([]string, len(chainHops))
			copy(hops, chainHops)

			p.addCall(CallSite{
				FuncName: funcName,
				Variable: baseVar,
				Chain:    hops,
				Line:     uint32(p.baseLine + line),
				Caller:   caller,
			})
		}
	}
}

// recordChainFromScope records `scope.name.method(...)` and then any further
// hops chained onto it.
//
// Both scoped-var handlers recorded the first call and then merely skipped its
// argument list, so the `.c()` in `variables.a.b().c()` was left for the outer
// loop to rediscover as an orphaned *bare* call. That is a wrong answer rather
// than a missing one: in a component that declares a `c`, it is an edge the
// call never takes. `continueChainCalls` exists to stop exactly this on the
// unscoped path; this routes the scoped path through the same helper.
func (p *scriptParser) recordChainFromScope(fullChain string, line int) {
	p.recordCallFromChain(fullChain, line)

	if !p.skipParens() {
		return
	}

	base, name := "", fullChain
	if dot := strings.LastIndexByte(fullChain, '.'); dot >= 0 {
		base, name = fullChain[:dot], fullChain[dot+1:]
	}

	p.recordChainContinuation(base, name, line)
}

func (p *scriptParser) isVarDeclaredLocal(name string) bool {
	if p.localVarSet == nil {
		return false
	}

	var buf foldScratch

	return p.localVarSet[string(buf.lowerFold(name))]
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
		p.declareLoopVar(nameTok, peek)

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
	p.skipLiteralGroup()

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		return
	}

	var buf foldScratch
	switch string(buf.lowerFold(rhs.Value)) {
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

	var fullChain chainBuilder
	fullChain.reset(rhs.Value)

chainWalk:
	for {
		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokLBracket:
			// Dynamic key (e.g. REQUEST['a' & b & 'c'] or arr[i]) — can't be
			// resolved statically. Skip the whole [...] group and poison
			// fullChain with a marker that can never collide with a real
			// resolver/ComponentRef match, so tryResolveCall/resolveCall
			// below safely fail instead of misattributing to whatever the
			// bare base identifier happens to resolve to elsewhere.
			if !p.skipBracketIndex() {
				return
			}

			fullChain.writeString("[]")
		case TokDoubleColon:
			// `var x = models.Foo::bar()`. The chain so far is a component,
			// not a receiver — see parseStaticCall. The assignment's own type
			// inference stops here, as it already did.
			p.recordStaticCall(fullChain.String(), Token{Line: line})

			return
		case TokDot:
			p.sc.NextSkipComments() // consume .

			next := p.sc.PeekSkipComments()
			if next.Kind == TokIdent {
				p.sc.NextSkipComments()

				prevIdent = lastIdent
				lastIdent = next.Value

				fullChain.writeDot()
				fullChain.writeString(next.Value)
			} else {
				break chainWalk
			}
		default:
			break chainWalk
		}
	}

	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.recordCallFromChain(fullChain.String(), line)

		if comp := p.tryResolveCall(fullChain.String()); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				ChainBase: prevIdent, ChainMethod: lastIdent,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
			p.recordChainContinuation(prevIdent, lastIdent, line)
		} else if comp := p.tryExtendChain(fullChain.String()); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				ChainBase: prevIdent, ChainMethod: lastIdent,
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
				p.continueChainCalls(prevIdent, lastIdent, line)
			}
		} else {
			p.pendingCalls = append(p.pendingCalls, pendingCall{
				varName: varName, funcName: lastIdent, baseVar: prevIdent,
				line: uint32(p.baseLine + line), funcKey: p.inFunc,
			})
			p.continueChainCalls(prevIdent, lastIdent, line)
		}
	} else if len(p.resolvers) > 0 {
		if comp := p.resolveCall(fullChain.String()); comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		}
	}
}

// parseScopedVar handles: scope.name = expr (local., arguments., this., variables.)
// recordScopedMemberCall records `scope.name(...)` — a call whose whole
// receiver is a scope.
//
// Both scoped-var handlers read `scope` `.` `name` and then looked only for
// `=` (an assignment) or `.` (a longer chain), so the shape where the statement
// *is* the call recorded nothing: `this.init()`, `variables.buildCache()`,
// `request.getRemoteClients()`. `x = request.getRemote()` and
// `request.a.getRemote()` both worked, which is why this survived — it is only
// the bare statement form that was lost, and that is how a component calls its
// own method with an explicit scope.
//
// `this.` and `variables.` name a member of the component being parsed, so the
// call is recorded unqualified and resolves against the file's own functions —
// including the ones `parseFunctionValue` files from `this.helper = function(){}`.
// Every other scope holds a value put there at runtime, so the receiver is
// `$any`: the call site is recorded, and the method-exists check is skipped
// rather than answered wrongly against a same-named function elsewhere.
func (p *scriptParser) recordScopedMemberCall(scopeTok, nameTok Token) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	call := CallSite{
		FuncName: nameTok.Value,
		Line:     uint32(p.baseLine + scopeTok.Line),
		Caller:   caller,
	}

	// The scope is read from the token rather than from the Scope value the
	// dispatch passed: `request`, `session` and `application` are all dispatched
	// as ScopeVariables, deliberately, so that an assignment through one keeps
	// the component its right-hand side establishes. They are not this
	// component's members, and testing the enum would record every one of them
	// as a call to a function of that name in this file.
	if !identEq(scopeTok.Value, "this") && !identEq(scopeTok.Value, "variables") {
		call.Variable = scopeTok.Value
		call.Component = "$any"
		call.Resolved = true
	}

	p.addCall(call)

	if !p.skipParens() {
		return
	}

	// A hop chained onto it is a call on what the first one returned, and
	// leaving it to the outer loop makes it an orphaned bare call — the same
	// wrong answer recordChainFromScope exists to stop. A member of this
	// component walks the chain through its declared return type; a dynamic
	// receiver stays dynamic all the way down, as a literal receiver's chain
	// already does.
	if call.Component == "" {
		p.recordChainContinuation("", nameTok.Value, scopeTok.Line)

		return
	}

	p.recordDynamicChain(call.Variable, scopeTok.Line, caller)
}

// recordDynamicChain records the hops chained onto a call whose receiver is
// dynamic, keeping every one of them dynamic. The scanner must sit just past
// the first call's ')'.
func (p *scriptParser) recordDynamicChain(recv string, line int, caller string) {
	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			return
		}

		p.sc.NextSkipComments()

		if p.sc.PeekSkipComments().Kind != TokLParen {
			return
		}

		p.addCall(CallSite{
			FuncName:  methTok.Value,
			Variable:  recv,
			Component: "$any",
			Line:      uint32(p.baseLine + line),
			Caller:    caller,
			Resolved:  true,
		})

		if !p.skipParens() {
			return
		}
	}
}

func (p *scriptParser) parseScopedVar(tok Token, scope Scope) {
	dot := p.sc.PeekSkipComments()
	if dot.Kind != TokDot {
		// See parseBodyScopedVar's identical case for why.
		if dot.Kind == TokLBracket {
			p.checkBareCall(tok)
		}

		return
	}

	p.sc.NextSkipComments() // consume .

	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		if eq.Kind == TokLParen {
			p.recordScopedMemberCall(tok, nameTok)

			return
		}

		// Not an assignment — check for method call chain: scope.name.method(...)
		if eq.Kind == TokDot {
			var fullChain chainBuilder
			fullChain.reset(tok.Value)
			fullChain.writeDot()
			fullChain.writeString(nameTok.Value)

			for p.sc.PeekSkipComments().Kind == TokDot {
				p.sc.NextSkipComments()

				next := p.sc.PeekSkipComments()
				if next.Kind == TokIdent {
					p.sc.NextSkipComments()

					fullChain.writeDot()
					fullChain.writeString(next.Value)
				} else {
					break
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				p.recordChainFromScope(fullChain.String(), tok.Line)
			}
		}

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
	if (scope == ScopeThis || scope == ScopeVariables) &&
		p.parseFunctionValue(nameTok.Value, tok) {
		return
	}

	p.skipLiteralGroup()

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind == TokIdent {
		var buf foldScratch
		switch string(buf.lowerFold(rhs.Value)) {
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
		case "this":
			p.sc.NextSkipComments()

			// Only treat `scope.x = this` (bare) as a self-ref.
			// `scope.x = this.prop = ...` is a chained assignment — the real
			// component comes from the continuation, so skip the ref here.
			if p.sc.PeekSkipComments().Kind != TokDot {
				if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: selfPath,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + tok.Line),
					})
				}
			}
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

		afterLT := p.afterLT
		p.afterLT = tok.Kind == TokLT

		if tok.Kind != TokIdent {
			p.handleLiteralToken(tok)

			continue
		}

		var buf foldScratch
		switch string(buf.lowerFold(tok.Value)) {
		case "function":
			p.parseFunction(tok, "", "")
		case "public", "private", "remote", "package":
			p.parseAccessModified(tok)
		case "component", "interface":
			// An interface's attribute list is a component's — `interface
			// extends="IBase"` left `extends` to be read as an assignment, and
			// declared a variable called `extends`.
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
		case "request", "session", "application":
			// See the matching case in handleBodyToken for why this is needed:
			// without it, a top-level (outside any function) "REQUEST.x = ..."
			// assignment falls through to the bare-call path and its RHS
			// component type is silently dropped.
			p.parseScopedVar(tok, ScopeVariables)
		case "import":
			p.parseImport()
		case "new":
			p.parseStandaloneNew(tok)
		case "catch":
			p.parseCatchVar()
		case "throw":
			p.consumeThrowArgs()
		default:
			// Check for returnType function pattern (e.g. "string function getName()")
			peek := p.sc.PeekSkipComments()
			switch {
			case peek.Kind == TokIdent && identEq(peek.Value, "function"):
				p.sc.NextSkipComments()
				p.parseFunction(tok, "", tok.Value)
			case peek.Kind == TokDot:
				// Walk the dot chain — could be dotted return type or bare call
				var retVal chainBuilder
				retVal.reset(tok.Value)

				lastIdent := tok.Value
				prevIdent := ""

				for p.sc.PeekSkipComments().Kind == TokDot {
					p.sc.NextSkipComments()

					seg := p.sc.PeekSkipComments()
					if seg.Kind == TokIdent {
						p.sc.NextSkipComments()

						prevIdent = lastIdent
						lastIdent = seg.Value

						retVal.writeDot()
						retVal.writeString(seg.Value)
					} else {
						break
					}
				}

				next := p.sc.PeekSkipComments()
				if next.Kind == TokIdent && identEq(next.Value, "function") {
					p.sc.NextSkipComments()
					p.parseFunction(tok, "", retVal.String())
				} else if next.Kind == TokLParen && !isKeyword(tok.Value) {
					_ = prevIdent

					varName := ""
					chain := retVal.String()

					if dotIdx := strings.LastIndexByte(chain, '.'); dotIdx >= 0 {
						varName = chain[:dotIdx]
					}

					funcName := lastIdent
					line := tok.Line

					if !p.skipParens() {
						return
					}

					p.addCall(CallSite{
						FuncName: funcName,
						Variable: varName,
						Line:     uint32(p.baseLine + line),
					})

					// Continue walking further .method() hops chained off this
					// call's return value (e.g. "table.getTable().setSkipFirstHeader(...)")
					// — without this, the scanner resumes right after this hop's
					// "(" and the next ".method(" is rediscovered as an orphaned,
					// unqualified bare call. Mirrors checkBareCall's chain loop.
					var chainHops []string

					for p.sc.PeekSkipComments().Kind == TokDot {
						p.sc.NextSkipComments() // consume .

						methTok := p.sc.PeekSkipComments()
						if methTok.Kind != TokIdent {
							break
						}

						p.sc.NextSkipComments() // consume method name

						if p.sc.PeekSkipComments().Kind != TokLParen {
							break
						}

						chainHops = append(chainHops, funcName)
						funcName = methTok.Value

						if !p.skipParens() {
							return
						}

						hops := make([]string, len(chainHops))
						copy(hops, chainHops)

						p.addCall(CallSite{
							FuncName: funcName,
							Variable: varName,
							Chain:    hops,
							Line:     uint32(p.baseLine + line),
						})
					}
				}
			case peek.Kind == TokDoubleColon:
				p.parseStaticCall(tok)
			case peek.Kind == TokLParen && !isKeyword(tok.Value):
				p.recordBareCallAndChain(tok)
			case !afterLT && peek.Kind == TokIdent && !isKeyword(tok.Value) && looksLikeTagAttrs(p.sc):
				p.parseScriptTagAttrs()
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

	var retVal chainBuilder
	retVal.reset(retType.Value)

	// Handle dotted return types (e.g. models.User)
	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments()

		seg := p.sc.NextSkipComments()
		if seg.Kind == TokIdent {
			retVal.writeDot()
			retVal.writeString(seg.Value)
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

// parseFunctionValue declares a method for a function assigned to a name, and
// reports whether it consumed one.
//
// `this.helper = function(a) { … }` and `variables.helper = (a) => a` are how a
// component exposes a method it builds rather than declares, and the parser
// recorded only a variable: no FunctionDef, so the component had none of those
// methods in Funcs — no completion, no signature help, no go-to-definition and
// nothing in the index.
//
// It fires only outside a function body, and only for `this.`, `variables.` and
// an unscoped name. A `var`- or `local.`-scoped closure is a local value, not a
// method of the component, and declaring one as a method would put a helper
// private to one function into every caller's completion list.
//
// The paren-less single-argument arrow (`this.x = a => a * 2`) is not
// recognised: telling it from an ordinary `this.x = a` needs three tokens of
// lookahead on every assignment whose right-hand side is a bare identifier,
// which is most of them.
func (p *scriptParser) parseFunctionValue(name string, startTok Token) bool {
	if p.inFunc != "" {
		return false
	}

	// The docblock sits before the assignment, not before the `function`
	// keyword, so it is read here rather than where parseFunction reads it.
	docComment := p.sc.LastBlockComment

	rhs := p.sc.PeekSkipComments()

	switch {
	case rhs.Kind == TokIdent && identEq(rhs.Value, "function"):
		p.sc.NextSkipComments() // consume `function`

		// A named function expression: `this.x = function x() { … }`.
		if p.sc.PeekSkipComments().Kind == TokIdent {
			p.sc.NextSkipComments()
		}

		if p.sc.PeekSkipComments().Kind != TokLParen {
			return false
		}

		p.sc.NextSkipComments() // consume (

	case rhs.Kind == TokLParen && p.arrowFollowsParens():
		p.sc.NextSkipComments() // consume (

	default:
		return false
	}

	p.sc.LastBlockComment = ""

	args := p.parseArgList()
	if docComment != "" {
		applyJSDocParams(docComment, args)
	}

	if rhs.Kind == TokLParen {
		// The `=>` the lookahead found, now that the argument list is consumed.
		p.sc.NextSkipComments()
		p.sc.NextSkipComments()
	}

	p.recordFunctionValue(name, startTok, args)

	return true
}

// arrowFollowsParens reports whether the (...) the scanner is positioned on is
// an arrow function's argument list, without moving it.
//
// `=>` is two tokens to this scanner, and the parentheses have to be walked to
// reach them, so this is the one place a rewound scan is worth its cost: an
// assignment whose right-hand side opens with a paren is uncommon, and telling
// `(a) => a` from `(a + b) * c` has no cheaper test.
func (p *scriptParser) arrowFollowsParens() bool {
	saved := p.sc.Save()
	defer p.sc.Restore(saved)

	if !p.skipParensQuiet() {
		return false
	}

	if p.sc.PeekSkipComments().Kind != TokEquals {
		return false
	}

	p.sc.NextSkipComments()

	return p.sc.PeekSkipComments().Kind == TokGT
}

// skipParensQuiet consumes a balanced (...) group without recording anything,
// for a lookahead that will be rewound — scanParenBody would record every call
// inside the group twice over.
func (p *scriptParser) skipParensQuiet() bool {
	if p.sc.PeekSkipComments().Kind != TokLParen {
		return false
	}

	p.sc.NextSkipComments() // consume (

	depth := 1

	for depth > 0 {
		switch tok := p.sc.NextSkipComments(); tok.Kind { //nolint:exhaustive
		case TokEOF:
			return false
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		}
	}

	return true
}

// recordFunctionValue files the declaration and walks the body, as
// parseFunction does for a declared method.
func (p *scriptParser) recordFunctionValue(name string, startTok Token, args []Argument) {
	funcLine := p.baseLine + startTok.Line

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
		Name:      name,
		URI:       uriFromString(p.fileURI),
		Line:      uint32(funcLine),
		Arguments: args,
	})

	endLine := p.parseBody(funcLine, args)
	p.scopes = append(p.scopes, FuncScope{Name: name, Start: funcLine, End: p.baseLine + endLine})
}

// declareLoopVar declares the loop variable of `for (var row in qry)`.
//
// The two var-decl parsers only ever looked for `=`, so a `var` that binds
// through `in` declared nothing and go-to-definition on the loop variable
// failed — in every for-in loop, which is how CFML iterates a query, an array
// and a struct.
func (p *scriptParser) declareLoopVar(nameTok, next Token) {
	if next.Kind != TokIdent || !identEq(next.Value, "in") {
		return
	}

	p.declareVar(nameTok, ScopeLocal)
}

// parseCatchVar declares a catch block's exception variable.
//
// `catch (any e)` and `catch (e)` bind `e` for the block, and neither was
// declared: go-to-definition on the exception variable failed in every catch
// block there is.
func (p *scriptParser) parseCatchVar() {
	if p.sc.PeekSkipComments().Kind != TokLParen {
		return
	}

	p.sc.NextSkipComments() // consume (

	// The **last** identifier before the closing paren is the variable, and
	// whatever precedes it is the type — which is how one rule reads `catch (e)`,
	// `catch (any e)` and a dotted `catch (org.Foo e)` alike.
	var last Token

	for {
		tok := p.sc.NextSkipComments()

		switch tok.Kind { //nolint:exhaustive
		case TokIdent:
			last = tok
		case TokDot:
		case TokRParen, TokEOF:
			if last.Kind == TokIdent {
				p.declareVar(last, ScopeLocal)
			}

			return
		default:
			return
		}
	}
}

// declareVar records a binding that is neither an assignment nor an argument —
// a loop variable or a caught exception. Outside a function body there is no
// local scope to put it in, so it lands where an unscoped assignment would.
func (p *scriptParser) declareVar(nameTok Token, scope Scope) {
	if p.inFunc == "" {
		scope = ScopeVariables
	} else if scope == ScopeLocal && p.localVarSet != nil {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	}

	p.vars = append(p.vars, VarDef{
		Name: nameTok.Value, Scope: scope,
		Line: uint32(p.baseLine + nameTok.Line),
	})
}

// parseStaticCall records `Foo::bar()`, Lucee's and ACF's static member access.
//
// The `::` was two unrecognised tokens, so `Foo` was dropped and the call was
// recorded as a bare `bar()` — reported as `bar (no qualifier, not in file)`,
// which names the wrong problem: the call *is* qualified, and by a component
// rather than a variable. The CallSite therefore carries Component, not
// Variable, so resolution looks for the method in `Foo` instead of among the
// file's own functions.
//
// Only the statement form is recognised. A static call in an expression
// (`x = Foo::bar()`) is left where it was, since the chain walks that would
// have to learn about it each hold a receiver *name* and this has none.
func (p *scriptParser) parseStaticCall(qualifier Token) {
	p.recordStaticCall(qualifier.Value, qualifier)
}

// recordStaticCall consumes `::method(...)` from the scanner and files the call
// against component, which the caller has already read.
func (p *scriptParser) recordStaticCall(component string, startTok Token) {
	p.sc.NextSkipComments() // consume ::

	methTok := p.sc.PeekSkipComments()
	if methTok.Kind != TokIdent {
		return
	}

	p.sc.NextSkipComments()

	if p.sc.PeekSkipComments().Kind != TokLParen {
		return
	}

	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	p.addCall(CallSite{
		FuncName:  methTok.Value,
		Component: component,
		Line:      uint32(p.baseLine + startTok.Line),
		Caller:    caller,
		Resolved:  true,
	})

	p.skipParens()
}

// consumeThrowArgs consumes `throw(...)`'s argument list.
//
// `throw` is the one CFScript keyword invoked like a function, and being a
// keyword it never reached the dispatch that consumes an argument list — so
// `throw(type = "x", message = "y")` declared variables called `type` and
// `message`, and `throw(object = new errors.Bad())` recorded a component ref
// for a variable called `object`. The other keywords that take parentheses
// (`if`, `while`, `switch`) hold an expression rather than named arguments, and
// the statement scan reads those correctly as it is.
//
// `throw "message";` and `throw new Foo();` take no parentheses, and are left
// for the loop to carry on with.
func (p *scriptParser) consumeThrowArgs() {
	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.skipParens()
	}
}

// handleLiteralToken is what every token loop does with a string or a closing
// bracket: scan a string's #...# interpolations for calls, and record a member
// function called on the literal.
//
// It is one function because there are six loops that walk tokens — parse,
// parseBody, scanParenBody, scanNestedFunctionBody, skipLiteralGroup and
// skipDefault — and a literal reaches all of them.
// TestEveryTokenLoopHandlesLiterals fails if one stops routing through here.
func (p *scriptParser) handleLiteralToken(tok Token) {
	if tok.Kind == TokString {
		p.scanInterpolation(tok)
	}

	if isLiteralReceiver(tok.Kind) && p.sc.PeekSkipComments().Kind == TokDot {
		p.recordLiteralMemberCall(tok)
	}
}

// scanInterpolation records the calls written inside a string's #...# spans.
//
// The scanner takes a quoted string as one token, so everything in it was
// invisible: `writeOutput("id #svc.getName()#")` recorded only writeOutput.
// That is idiomatic CFML rather than an edge case — interpolation is how a
// computed value reaches a string — and it cost the same as the argument-list
// gap did: no edge into the callee, so the code map read it as unreachable and
// `unresolved` never checked it.
//
// Each span is scanned by the same parser over a scanner of its own, so the
// calls land in the enclosing function's bucket with the resolver machinery
// that a call outside a string gets. Line numbers within a multi-line string
// collapse onto the line the string starts at.
func (p *scriptParser) scanInterpolation(tok Token) {
	// A span with no '(' in it cannot hold a call, and a call is the only thing
	// this records — `"#user.name#"` and `"#i#"` are most of the interpolation
	// in any CFML file, so rejecting them on one byte scan is what keeps this
	// off the profile.
	if strings.IndexByte(tok.Value, '#') < 0 || strings.IndexByte(tok.Value, '(') < 0 ||
		p.argNesting >= maxArgNesting {
		return
	}

	outerScanner, outerBase := p.sc, p.baseLine

	p.argNesting++

	defer func() {
		p.sc, p.baseLine = outerScanner, outerBase
		p.argNesting--
	}()

	tokBase := p.baseLine + tok.Line

	for _, span := range interpolatedSpans(tok.Value) {
		// A string token can span lines — CFML's `""` escape is what makes a
		// multi-line one ordinary — so the span's own line is the token's start
		// plus the newlines before it, not the token's start.
		p.baseLine = tokBase + countNewlines(tok.Value[:span.start])
		p.sc = NewScanner(tok.Value[span.start:span.end])
		p.sc.interpStrings = true

		for {
			t := p.sc.NextSkipComments()
			if t.Kind == TokEOF {
				break
			}

			switch t.Kind { //nolint:exhaustive
			case TokIdent:
				p.scanNestedCall(t)
			case TokString, TokRBracket:
				p.handleLiteralToken(t)
			}
		}
	}
}

// countNewlines reports how many lines a byte range spans past its first.
func countNewlines(s string) int {
	return strings.Count(s, "\n")
}

// hashSpan is the half-open byte range between a pair of interpolation hashes.
// The tag parser needs the offsets to place a line number, so the ranges rather
// than the substrings are what this returns.
type hashSpan struct{ start, end int }

// interpolatedSpans returns the range inside each #...# of a piece of source.
//
// `##` is CFML's escape for a literal hash and opens nothing, which is what
// stops `"a ## b"` being read as a span containing " ".
func interpolatedSpans(s string) []hashSpan {
	var spans []hashSpan

	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}

		if i+1 < len(s) && s[i+1] == '#' {
			i++ // an escaped hash: skip the pair

			continue
		}

		end := matchingHash(s, i+1)
		if end < 0 {
			// A lone `#` — a CSS colour, a jQuery id selector, prose. It opens
			// nothing, and the spans after it are still spans: this used to
			// give up on the whole remainder, which lost every interpolation
			// later in the chunk.
			continue
		}

		if end > i+1 {
			spans = append(spans, hashSpan{start: i + 1, end: end})
		}

		i = end
	}

	return spans
}

// matchingHash finds the `#` that closes a span opened just before from,
// stepping over any quoted string on the way.
//
// A string inside a span may hold a span of its own — CFML nests them:
//
//	var u = "#html.elixirPath( root = '#cb.themeRoot()#/includes' )#";
//
// and taking the first `#` as the close made the *inner* opening hash the
// outer's terminator, so the outer call was sub-parsed from a fragment and
// every pairing after it on the line was inverted. Scanner.scanHashExpr already
// reads CFScript this way; this is the same rule for the text a tag parser
// hands over.
func matchingHash(s string, from int) int {
	// Both scans are IndexByte: a quote before the next `#` is what makes
	// nesting possible, and only then is there anything to step over. A span
	// with no quote in it — which is nearly all of them — costs the same two
	// byte scans the single loop this replaces cost as one.
	for i := from; ; {
		h := strings.IndexByte(s[i:], '#')
		if h < 0 {
			return -1
		}

		end := i + h

		q := indexQuote(s[i:end])
		if q < 0 {
			return end
		}

		j := skipQuotedIn(s, i+q)
		if j < 0 {
			return -1
		}

		i = j + 1
	}
}

// indexQuote returns the offset of the first single or double quote, or -1.
func indexQuote(s string) int {
	d := strings.IndexByte(s, '"')

	sq := strings.IndexByte(s, '\'')
	if d < 0 {
		return sq
	}

	if sq < 0 || d < sq {
		return d
	}

	return sq
}

// skipQuotedIn returns the index of the quote closing the string opened at i,
// honouring CFML's doubled-quote escape, or -1 if it never closes.
func skipQuotedIn(s string, i int) int {
	q := s[i]

	for j := i + 1; j < len(s); j++ {
		if s[j] != q {
			continue
		}

		if j+1 < len(s) && s[j+1] == q {
			j++

			continue
		}

		return j
	}

	return -1
}

// isLiteralReceiver reports whether a token can close a literal that a member
// function is then called on.
func isLiteralReceiver(k TokenKind) bool {
	return k == TokString || k == TokRBracket
}

// recordLiteralMemberCall records `"abc".ucase()` and `[1,2].map(f)`.
//
// The receiver is a value rather than a variable, so the scan walked past it
// and recorded a *bare* `ucase()` — indistinguishable from an unqualified call
// to a function of that name, which in a component that happens to declare one
// is an edge to it that does not exist. The component is `$any`, the existing
// spelling for "genuinely dynamic", so the method-exists check is skipped the
// way it is for any other value whose type is not static.
func (p *scriptParser) recordLiteralMemberCall(recv Token) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			return
		}

		p.sc.NextSkipComments()

		if p.sc.PeekSkipComments().Kind != TokLParen {
			continue // a property read, e.g. `"a,b".listLen` — nothing to record
		}

		p.addCall(CallSite{
			FuncName:  methTok.Value,
			Component: "$any",
			Line:      uint32(p.baseLine + recv.Line),
			Caller:    caller,
			Resolved:  true,
		})

		if !p.skipParens() {
			return
		}
	}
}

func (p *scriptParser) parseArgList() []Argument {
	args := make([]Argument, 0, 4)

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

		// Capacity 3 because the loop below stops there: an argument is at most
		// `required type name`. Built from a literal of one, it reallocated twice
		// for every argument of every function in the file.
		idents := make([]string, 1, 3)
		idents[0] = tok.Value

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

// skipDefault consumes an argument's default value, recording any calls written
// in it: `function load(id, dao = newDao())` calls newDao on every invocation
// that omits it, and the scan walked past it a token at a time.
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
		case TokIdent:
			p.scanNestedCall(peek)
		case TokString:
			p.handleLiteralToken(peek)
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

		afterLT := p.afterLT
		p.afterLT = t.Kind == TokLT

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
				p.handleBodyToken(t, depth, afterLT)
			}
		case TokString, TokRBracket:
			if depth > 0 {
				p.handleLiteralToken(t)
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

	// Remap funcCalls from temp key to real key
	if p.funcCalls != nil {
		if calls, ok := p.funcCalls[tempKey]; ok {
			delete(p.funcCalls, tempKey)
			p.funcCalls[realKey] = calls
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
func (p *scriptParser) handleBodyToken(tok Token, depth int, afterLT bool) {
	var buf foldScratch
	switch string(buf.lowerFold(tok.Value)) {
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
	case "request", "session", "application":
		// Request/session/application-scoped assignments (e.g. "REQUEST.generator =
		// document.createTable(1);") — checkAssignRef's default path only recognizes
		// a bare "x = ..." (identifier directly followed by "="); for a scope-prefixed
		// LHS the next token is "." not "=", so without this case it falls through to
		// checkBareCall and the assignment (and any component type it establishes) is
		// silently dropped. parseBodyScopedVar already handles the "scope.name = rhs"
		// vs "scope.name.method()" split correctly for variables./this./arguments.; the
		// same handling applies verbatim here. Treated as global (forceGlobal), same as
		// this./variables., since these scopes outlive the current function.
		p.parseBodyScopedVar(tok, ScopeVariables)
	case "return":
		p.checkReturnComponent()
	case "new":
		p.parseStandaloneNew(tok)
	case "function", "public", "private", "remote", "package":
		// Nested function — skip its entire body
		p.skipNestedFunction(tok, depth)
	case "catch":
		p.parseCatchVar()
	case "throw":
		p.consumeThrowArgs()
	default:
		peek := p.sc.PeekSkipComments()
		if peek.Kind == TokDoubleColon {
			p.parseStaticCall(tok)

			return
		}

		if !isKeyword(tok.Value) && peek.Kind == TokLParen {
			p.recordBareCallAndChain(tok)

			return
		}

		if !afterLT && peek.Kind == TokIdent && !isKeyword(tok.Value) && looksLikeTagAttrs(p.sc) {
			p.parseScriptTagAttrs()

			return
		}

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

	var buf foldScratch
	switch string(buf.lowerFold(peek.Value)) {
	case "new":
		p.sc.NextSkipComments()

		comp = p.readNewComponent()
		if p.sc.PeekSkipComments().Kind == TokLParen {
			p.skipParens()
		}

		p.scanChainedCalls(comp, peek.Line)
	case "createobject":
		p.sc.NextSkipComments()

		comp = p.readCreateObjectComponent()
		if p.sc.PeekSkipComments().Kind == TokRParen {
			p.sc.NextSkipComments()
		}

		p.scanChainedCalls(comp, peek.Line)
	case "entitynew":
		p.sc.NextSkipComments()

		comp = p.readEntityNewComponent()
		if p.sc.PeekSkipComments().Kind == TokRParen {
			p.sc.NextSkipComments()
		}

		p.scanChainedCalls(comp, peek.Line)
	default:
		// Check for return obj.method(...) or return func(...).
		if !isKeyword(peek.Value) {
			p.sc.NextSkipComments() // consume first ident

			nextKind := p.sc.PeekSkipComments().Kind
			if nextKind == TokDot {
				var fullChain chainBuilder
				fullChain.reset(peek.Value)

				for p.sc.PeekSkipComments().Kind == TokDot {
					p.sc.NextSkipComments() // consume .

					seg := p.sc.PeekSkipComments()
					if seg.Kind == TokIdent {
						p.sc.NextSkipComments()

						fullChain.writeDot()
						fullChain.writeString(seg.Value)
					} else {
						break
					}
				}

				if p.sc.PeekSkipComments().Kind == TokLParen {
					p.recordCallFromChain(fullChain.String(), peek.Line)
				}
			} else if nextKind == TokLParen {
				// Bare function call: return funcName(...)
				caller := ""
				if len(p.funcs) > 0 {
					caller = p.funcs[len(p.funcs)-1].Name
				}

				p.addCall(CallSite{
					FuncName: peek.Value,
					Line:     uint32(p.baseLine + peek.Line),
					Caller:   caller,
				})
			}
		}

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

	if tok.Kind != TokIdent {
		return ""
	}

	// Lucee's type prefix: `new java:java.io.File(…)`, `new cfml:models.Base(…)`.
	// The prefix is not part of the path — reading it as one yields a component
	// literally named "java", which then fails every method check against it.
	if (identEq(tok.Value, "java") || identEq(tok.Value, "cfml")) &&
		p.sc.PeekSkipComments().Kind == TokColon {
		p.sc.NextSkipComments() // consume the ':'

		path := p.readDottedPath(p.sc.NextSkipComments())

		if identEq(tok.Value, "cfml") {
			return p.applyImport(path)
		}

		// `java:` names a Java class, so it resolves through the same
		// javaStubsPath machinery as createObject("java", …).
		if path == "" {
			return ""
		}

		return p.resolveCall("createObject(\"java\",\"" + path + "\")")
	}

	return p.applyImport(p.readDottedPath(tok))
}

// parseImport records `import models.User;`, so a later `new User()` resolves
// to models.User rather than to a component literally called User.
//
// Only the explicit form is recorded. `import models.*;` names a directory, and
// which component a bare name then refers to is a question about what is on
// disk — the parser has no filesystem and guessing would produce a confident
// wrong path, so a wildcard import is left to resolve as it did.
func (p *scriptParser) parseImport() {
	path := p.readDottedPath(p.sc.NextSkipComments())
	if path == "" {
		return
	}

	dot := strings.LastIndexByte(path, '.')
	if dot < 0 {
		return
	}

	last := path[dot+1:]
	if last == "" || last == "*" {
		return
	}

	if p.imports == nil {
		p.imports = make(map[string]string, 4)
	}

	p.imports[strings.ToLower(last)] = path
}

// applyImport rewrites a bare component name that an import brought into scope.
// A dotted path is already qualified and is left alone.
func (p *scriptParser) applyImport(comp string) string {
	if comp == "" || len(p.imports) == 0 || strings.ContainsRune(comp, '.') {
		return comp
	}

	var buf foldScratch
	if full, ok := p.imports[string(buf.lowerFold(comp))]; ok {
		return full
	}

	return comp
}

// readDottedPath continues a dotted path from an identifier already read,
// returning "" if that token was not an identifier.
func (p *scriptParser) readDottedPath(tok Token) string {
	if tok.Kind != TokIdent {
		return ""
	}

	var comp chainBuilder

	comp.reset(tok.Value)

	for {
		if p.sc.PeekSkipComments().Kind == TokDot {
			p.sc.NextSkipComments()

			next := p.sc.NextSkipComments()
			if next.Kind == TokIdent {
				comp.writeDot()
				comp.writeString(next.Value)
			}
		} else {
			break
		}
	}

	return comp.String()
}

// readCreateObjectComponent reads the component path from createObject("component","path")
// or resolves createObject("java","class") via configured resolvers. Scanner is left at the
// closing ) so the caller can consume it and scan for chained calls.
func (p *scriptParser) readCreateObjectComponent() string {
	if p.sc.NextSkipComments().Kind != TokLParen {
		return ""
	}

	arg1 := p.sc.NextSkipComments()
	if arg1.Kind != TokString {
		return ""
	}

	arg1Val := unquote(arg1.Value)

	if identEq(arg1Val, "component") {
		if p.sc.NextSkipComments().Kind != TokComma {
			return ""
		}

		arg2 := p.sc.NextSkipComments()
		if arg2.Kind != TokString {
			return ""
		}

		return unquote(arg2.Value)
	}

	// Non-component createObject (e.g. java) — consume comma+arg2 and try resolvers
	if p.sc.PeekSkipComments().Kind != TokComma {
		return ""
	}

	p.sc.NextSkipComments() // consume ,

	arg2 := p.sc.NextSkipComments()
	if arg2.Kind != TokString {
		return ""
	}

	expr := "createObject(\"" + arg1Val + "\",\"" + unquote(arg2.Value) + "\")"

	return p.resolveCall(expr)
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
		p.declareLoopVar(nameTok, peek)

		return
	}

	p.sc.NextSkipComments() // consume =

	p.localVarSet[strings.ToLower(nameTok.Value)] = true
	p.vars = append(p.vars, VarDef{
		Name: nameTok.Value, Scope: ScopeLocal,
		Line: uint32(p.baseLine + varTok.Line),
	})

	// Check RHS for component refs
	p.skipLiteralGroup()

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		return
	}

	var buf foldScratch
	switch string(buf.lowerFold(rhs.Value)) {
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

			var fullChain chainBuilder
			fullChain.reset(rhs.Value)

		chainWalk:
			for {
				switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
				case TokLBracket:
					// See checkVarRHS's identical case for why: skip the
					// dynamic key and poison fullChain so resolution safely
					// fails instead of misattributing to the bare base var.
					if !p.skipBracketIndex() {
						return
					}

					fullChain.writeString("[]")
				case TokDoubleColon:
					// See checkVarRHS's identical case.
					p.recordStaticCall(fullChain.String(), varTok)

					return
				case TokDot:
					p.sc.NextSkipComments()

					next := p.sc.PeekSkipComments()
					if next.Kind == TokIdent {
						p.sc.NextSkipComments()

						prevIdent = lastIdent
						lastIdent = next.Value

						fullChain.writeDot()
						fullChain.writeString(next.Value)
					} else {
						break chainWalk
					}
				default:
					break chainWalk
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				p.recordCallFromChain(fullChain.String(), varTok.Line)

				if comp := p.tryResolveCall(fullChain.String()); comp != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: comp,
						ChainBase: prevIdent, ChainMethod: lastIdent,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + varTok.Line),
					})
					p.recordChainContinuation(prevIdent, lastIdent, varTok.Line)
				} else if comp := p.tryExtendChain(fullChain.String()); comp != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: comp,
						ChainBase: prevIdent, ChainMethod: lastIdent,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + varTok.Line),
					})
				} else if p.builtinReturnLookup != nil {
					if comp := p.builtinReturnLookup(lastIdent); comp != "" {
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
						p.continueChainCalls(prevIdent, lastIdent, varTok.Line)
					}
				} else {
					p.pendingCalls = append(p.pendingCalls, pendingCall{
						varName:  nameTok.Value,
						funcName: lastIdent,
						baseVar:  prevIdent,
						line:     uint32(p.baseLine + varTok.Line),
						funcKey:  p.inFunc,
					})
					p.continueChainCalls(prevIdent, lastIdent, varTok.Line)
				}
			} else if len(p.resolvers) > 0 {
				if comp := p.resolveCall(fullChain.String()); comp != "" {
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
		// Not "scope.name" at all — e.g. REQUEST[key].method(), indexing the
		// whole scope struct with a dynamic key rather than a dotted name.
		// checkBareCall knows how to skip the "[...]" group and poison the
		// receiver; without this, the scanner is left stuck at "[" and the
		// trailing ".method()" gets rediscovered as an orphaned bare call.
		if dot.Kind == TokLBracket {
			p.checkBareCall(scopeTok)
		}

		return
	}

	p.sc.NextSkipComments() // consume .

	nameTok := p.sc.NextSkipComments()
	if nameTok.Kind != TokIdent {
		return
	}

	eq := p.sc.PeekSkipComments()
	if eq.Kind != TokEquals {
		if eq.Kind == TokLParen {
			p.recordScopedMemberCall(scopeTok, nameTok)

			return
		}

		// Not an assignment — check for method call chain: scope.name.method(...)
		if eq.Kind == TokDot {
			var fullChain chainBuilder
			fullChain.reset(scopeTok.Value)
			fullChain.writeDot()
			fullChain.writeString(nameTok.Value)

		chainWalk:
			for {
				switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
				case TokLBracket:
					// See checkVarRHS's identical case for why.
					if !p.skipBracketIndex() {
						return
					}

					fullChain.writeString("[]")
				case TokDot:
					p.sc.NextSkipComments()

					next := p.sc.PeekSkipComments()
					if next.Kind == TokIdent {
						p.sc.NextSkipComments()

						fullChain.writeDot()
						fullChain.writeString(next.Value)
					} else {
						break chainWalk
					}
				default:
					break chainWalk
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				p.recordChainFromScope(fullChain.String(), scopeTok.Line)
			}
		}

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
	p.skipLiteralGroup()

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind == TokIdent {
		var buf foldScratch
		switch string(buf.lowerFold(rhs.Value)) {
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
		case "this":
			p.sc.NextSkipComments()

			// Only treat `scope.x = this` (bare) as a self-ref.
			// `scope.x = this.prop = ...` is a chained assignment — the real
			// component comes from the continuation, so skip the ref here.
			if p.sc.PeekSkipComments().Kind != TokDot {
				if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
					p.addRef(ComponentRef{
						Variable: nameTok.Value, Component: selfPath,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + scopeTok.Line),
					})
				}
			}
		default:
			if !isKeyword(rhs.Value) {
				p.sc.NextSkipComments()

				prevIdent := ""
				lastIdent := rhs.Value

				var fullChain chainBuilder
				fullChain.reset(rhs.Value)

			chainWalk2:
				for {
					switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
					case TokLBracket:
						// See checkVarRHS's identical case for why.
						if !p.skipBracketIndex() {
							return
						}

						fullChain.writeString("[]")
					case TokDoubleColon:
						// See checkVarRHS's identical case.
						p.recordStaticCall(fullChain.String(), scopeTok)

						return
					case TokDot:
						p.sc.NextSkipComments()

						next := p.sc.PeekSkipComments()
						if next.Kind == TokIdent {
							p.sc.NextSkipComments()

							prevIdent = lastIdent
							lastIdent = next.Value

							fullChain.writeDot()
							fullChain.writeString(next.Value)
						} else {
							break chainWalk2
						}
					default:
						break chainWalk2
					}
				}

				if p.sc.PeekSkipComments().Kind == TokLParen {
					p.recordCallFromChain(fullChain.String(), scopeTok.Line)

					if comp := p.tryResolveCall(fullChain.String()); comp != "" {
						p.addRef(ComponentRef{
							Variable: nameTok.Value, Component: comp,
							ChainBase: prevIdent, ChainMethod: lastIdent,
							URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + scopeTok.Line),
						})
						p.recordChainContinuation(prevIdent, lastIdent, scopeTok.Line)
					} else {
						p.pendingCalls = append(p.pendingCalls, pendingCall{
							varName:  nameTok.Value,
							funcName: lastIdent,
							baseVar:  prevIdent,
							line:     uint32(p.baseLine + scopeTok.Line),
							funcKey:  p.inFunc,
						})
						p.continueChainCalls(prevIdent, lastIdent, scopeTok.Line)
					}
				} else if len(p.resolvers) > 0 {
					if comp := p.resolveCall(fullChain.String()); comp != "" {
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
	// Handle access modifier before function keyword
	if !identEq(tok.Value, "function") {
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
		// Anonymous function: function() { ... }. The '(' is already consumed,
		// so the argument list is scanned from inside it — an argument default
		// there holds calls like any other.
		if _, ok := p.scanParenBody(); !ok {
			return
		}

		p.scanNestedFunctionBody()

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

	if _, ok := p.scanParenBody(); !ok {
		return
	}

	p.scanNestedFunctionBody()
}

// scanNestedFunctionBody consumes a nested function's `{ ... }` or `;`,
// recording the calls written inside it.
//
// It used to discard the body, so an immediately-invoked function lost
// everything it called: `return (function(){ return svc.x(); })();` recorded
// nothing at all, while the same closure passed as an argument recorded
// svc.x — because that path counts parentheses rather than skipping the
// function.
//
// The calls are attributed to the *enclosing* function, which is where the
// closure's code runs from and matches what an argument-position closure
// already did. Its own scope is not declared: a nested function is not a method
// of the component, for the reason parseFunctionValue gives.
func (p *scriptParser) scanNestedFunctionBody() {
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

		switch t.Kind { //nolint:exhaustive
		case TokEOF:
			return
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
		case TokIdent:
			p.scanNestedCall(t)
		case TokString, TokRBracket:
			p.handleLiteralToken(t)
		}
	}
}

func (p *scriptParser) checkAssignRef(tok Token) {
	if isKeyword(tok.Value) {
		return
	}

	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokEquals {
		// `for (row in qry)` without `var`. Unscoped, so it is a variables-scope
		// binding, which is CFML's rule for any unscoped assignment and the
		// reason the `var` form is the one to write.
		if peek.Kind == TokIdent && identEq(peek.Value, "in") {
			p.declareVar(tok, ScopeVariables)

			return
		}

		// Check for bare dotted call: obj.method() — also routes a bracket-indexed
		// receiver (obj[key].method()) through checkBareCall, whose own chain-walk
		// knows how to skip the "[...]" group; without TokLBracket here, a bare
		// statement starting with obj[key]... never reaches checkBareCall at all
		// and the scanner is left stuck at "[".
		if peek.Kind == TokDot || peek.Kind == TokLBracket {
			p.checkBareCall(tok)
		}

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

	if p.parseFunctionValue(tok.Value, tok) {
		p.forceGlobal = false

		return
	}

	p.skipLiteralGroup()

	rhs := p.sc.PeekSkipComments()
	if rhs.Kind != TokIdent {
		p.forceGlobal = false

		return
	}

	var buf foldScratch
	switch string(buf.lowerFold(rhs.Value)) {
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

			var fullChain chainBuilder
			fullChain.reset(rhs.Value)

		chainWalk:
			for {
				switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
				case TokLBracket:
					// See checkVarRHS's identical case for why: skip the
					// dynamic key and poison fullChain so resolution safely
					// fails instead of misattributing to the bare base var.
					if !p.skipBracketIndex() {
						return
					}

					fullChain.writeString("[]")
				case TokDoubleColon:
					// See checkVarRHS's identical case.
					p.recordStaticCall(fullChain.String(), tok)

					return
				case TokDot:
					p.sc.NextSkipComments() // consume .

					next := p.sc.PeekSkipComments()
					if next.Kind == TokIdent {
						p.sc.NextSkipComments()

						prevIdent = lastIdent
						lastIdent = next.Value

						fullChain.writeDot()
						fullChain.writeString(next.Value)
					} else {
						break chainWalk
					}
				default:
					break chainWalk
				}
			}

			if p.sc.PeekSkipComments().Kind == TokLParen {
				p.recordCallFromChain(fullChain.String(), tok.Line)

				if comp := p.tryResolveCall(fullChain.String()); comp != "" {
					p.addRef(ComponentRef{
						Variable: tok.Value, Component: comp,
						ChainBase: prevIdent, ChainMethod: lastIdent,
						URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + tok.Line),
					})
					p.recordChainContinuation(prevIdent, lastIdent, tok.Line)
				} else {
					p.pendingCalls = append(p.pendingCalls, pendingCall{
						varName:  tok.Value,
						funcName: lastIdent,
						baseVar:  prevIdent,
						line:     uint32(p.baseLine + tok.Line),
						funcKey:  p.inFunc,
					})
					p.continueChainCalls(prevIdent, lastIdent, tok.Line)
				}
			} else if len(p.resolvers) > 0 {
				// Try generic resolver match on non-call RHS (e.g. "_parent")
				if comp := p.resolveCall(fullChain.String()); comp != "" {
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

// checkBareCall handles obj.method() calls (not in assignment context),
// including further .method() calls chained off the return value of a previous
// call in the same chain (e.g. "kpg.generateKeyPair().getPublic().getParams()").
// Records one CallSite per hop when extractCalls is enabled. The scanner
// position is on the first ident (the variable/scope prefix); peek is a dot.
func (p *scriptParser) checkBareCall(tok Token) {
	// Walk the leading dot chain of plain identifiers: a.b.c
	identChain := []string{tok.Value}

chainWalk:
	for {
		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokLBracket:
			// Dynamic key (e.g. REQUEST['a' & b & 'c'].method()) — skip it and
			// poison the receiver so it can't be misattributed to whatever
			// the bare base identifier resolves to elsewhere. See
			// checkVarRHS's identical case for the full rationale.
			if !p.skipBracketIndex() {
				return
			}

			identChain[len(identChain)-1] += "[]"
		case TokDot:
			p.sc.NextSkipComments() // consume .

			next := p.sc.PeekSkipComments()
			if next.Kind != TokIdent {
				break chainWalk
			}

			p.sc.NextSkipComments()

			identChain = append(identChain, next.Value)
		case TokDoubleColon:
			// `models.Foo::bar()`. Everything walked so far is the component,
			// not a receiver — see parseStaticCall.
			p.recordStaticCall(strings.Join(identChain, "."), tok)

			return
		default:
			break chainWalk
		}
	}

	// Must end with ( to be a call
	if p.sc.PeekSkipComments().Kind != TokLParen {
		return
	}

	// The last identifier is the method; everything before it is the receiver variable.
	funcName := identChain[len(identChain)-1]
	varName := strings.Join(identChain[:len(identChain)-1], ".")

	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	if !p.skipParens() {
		return
	}

	p.addCall(CallSite{
		FuncName: funcName,
		Variable: varName,
		Line:     uint32(p.baseLine + tok.Line),
		Caller:   caller,
	})

	// Continue walking further .method() hops chained off this call's return
	// value. Each hop's CallSite keeps Variable pointing at the original
	// receiver and accumulates the intermediate method names in Chain, so
	// CanResolveCall can walk the receiver's type through each call.
	var chainHops []string

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments() // consume method name

		if p.sc.PeekSkipComments().Kind != TokLParen {
			break
		}

		chainHops = append(chainHops, funcName)
		funcName = methTok.Value

		if !p.skipParens() {
			return
		}

		hops := make([]string, len(chainHops))
		copy(hops, chainHops)

		p.addCall(CallSite{
			FuncName: funcName,
			Variable: varName,
			Chain:    hops,
			Line:     uint32(p.baseLine + tok.Line),
			Caller:   caller,
		})
	}
}

func (p *scriptParser) parseNewRef(varName string, line int) {
	component := p.readNewComponent()

	if component != "" {
		p.addRef(ComponentRef{
			Variable: varName, Component: component,
			URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
		})
	}

	// Consume constructor args and handle any chained .method() calls. The
	// args are consumed even when nothing resolved (an unmapped `new java:…`
	// with no javaStubsPath, say), or the scanner would be left sitting on
	// the "(" and the rest of the statement would be read as its own.
	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.skipParens()

		if component != "" {
			p.scanChainedCalls(component, line)
		}
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

		// Consume closing ) and handle any chained .method() calls
		if p.sc.PeekSkipComments().Kind == TokRParen {
			p.sc.NextSkipComments()
		}

		if comp != "" {
			p.scanChainedCalls(comp, line)
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
		comp := p.resolveCall(expr)

		if comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(p.baseLine + line),
			})
		}

		// Consume closing ) and handle any chained .method() calls
		if p.sc.PeekSkipComments().Kind == TokRParen {
			p.sc.NextSkipComments()
		}

		if comp != "" {
			p.scanChainedCalls(comp, line)
		}
	}
}

// scanChainedCalls records resolved CallSites for any .method() chains following
// a constructor or factory call whose component is already known.
func (p *scriptParser) scanChainedCalls(component string, line int) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments()

		if p.sc.PeekSkipComments().Kind != TokLParen {
			break
		}

		p.addCall(CallSite{
			FuncName:  methTok.Value,
			Component: component,
			Line:      uint32(p.baseLine + line),
			Caller:    caller,
			Resolved:  true,
		})

		p.skipParens()
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

// globalScriptParser extracts only variable declarations outside function
// bodies. It is a second, much smaller scan than scriptParser's, which is what
// makes VariablesVars/ThisVars cheap enough to answer per keystroke — it skips
// every function body outright rather than parsing it.
//
// Being a second implementation, it is also a second place every rule about
// what does *not* declare a variable has to hold: named arguments, struct
// literal keys and script-tag attributes all fooled both.
// TestBothVariableScansAgree pins them together.
type globalScriptParser struct {
	sc       *Scanner
	vars     []VarDef
	baseLine int
	scopes   []FuncScope
}

// newGlobalScriptParser is only ever handed a RegionScript's text, so its
// scanner reads interpolated strings the way scriptParser's does — otherwise
// the two variable scans would tokenise the same file differently.
func newGlobalScriptParser(src string, baseLine int, scopes []FuncScope) *globalScriptParser {
	sc := NewScanner(src)
	sc.interpStrings = true

	return &globalScriptParser{
		sc:       sc,
		baseLine: baseLine,
		scopes:   scopes,
	}
}

func (p *globalScriptParser) parse() {
	afterLT := false

	for {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return
		}

		wasLT := afterLT
		afterLT = tok.Kind == TokLT

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

		var buf foldScratch
		switch string(buf.lowerFold(tok.Value)) {
		case "var":
			p.parseVar(tok)
		case "local":
			p.parseDot(tok, ScopeVariables) // local outside func → variables
		case "this":
			p.parseDot(tok, ScopeThis)
		case "variables":
			p.parseDot(tok, ScopeVariables)
		case "component", "interface":
			// Their attribute lists are not assignments. parsePlain's keyword
			// guard runs before its tag-attribute check and both names are
			// keywords, so without this arm `component extends="models.Base"
			// accessors="true"` put `extends` and `accessors` into
			// VariablesVars — which is what completion offers and what the
			// index stores.
			p.skipTagAttrs()
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
			p.parsePlain(tok, wasLT)
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

	p.consumeAssignment()
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

	p.consumeAssignment()
}

func (p *globalScriptParser) parsePlain(tok Token, afterLT bool) {
	if isKeyword(tok.Value) {
		return
	}

	eq := p.sc.PeekSkipComments()

	switch {
	case eq.Kind == TokLParen:
		// A call. Its argument list is not statements, and a named argument
		// written `f(force = true)` declares nothing.
		p.skipGroup()

		return
	case !afterLT && eq.Kind == TokIdent && looksLikeTagAttrs(p.sc):
		// A script-syntax CF tag. See scriptParser.parseScriptTagAttrs.
		p.skipTagAttrs()

		return
	case eq.Kind != TokEquals:
		return
	}

	p.vars = append(p.vars, VarDef{
		Name: tok.Value, Scope: ScopeVariables,
		Line: uint32(p.baseLine + tok.Line),
	})

	p.consumeAssignment()
}

// consumeAssignment takes the "=" its three callers have only peeked at, and
// with it a struct or array literal standing as the right-hand side.
//
// This scan reads a bare `ident =` as a declaration, so anything it walks into
// that spells one declares a variable — and `{force = true}` spells one.
func (p *globalScriptParser) consumeAssignment() {
	p.sc.NextSkipComments() // consume =

	switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
	case TokLBrace, TokLBracket:
		p.skipGroup()
	default:
	}
}

// skipGroup consumes a balanced (...), {...} or [...] group that the scanner is
// positioned on. Returns false if it is on none of them, or the group is
// unclosed.
func (p *globalScriptParser) skipGroup() bool {
	var open, closing TokenKind

	switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
	case TokLParen:
		open, closing = TokLParen, TokRParen
	case TokLBrace:
		open, closing = TokLBrace, TokRBrace
	case TokLBracket:
		open, closing = TokLBracket, TokRBracket
	default:
		return false
	}

	p.sc.NextSkipComments() // consume the opener

	depth := 1

	for depth > 0 {
		switch tok := p.sc.NextSkipComments(); tok.Kind { //nolint:exhaustive
		case TokEOF:
			return false
		case open:
			depth++
		case closing:
			depth--
		}
	}

	return true
}

// skipTagAttrs consumes a script-syntax CF tag's attribute list, stopping
// before the body or the terminating semicolon.
func (p *globalScriptParser) skipTagAttrs() {
	for looksLikeTagAttrs(p.sc) {
		p.sc.NextSkipComments() // attribute name
		p.sc.NextSkipComments() // =

		if !p.skipTagAttrValue() {
			return
		}
	}
}

// skipTagAttrValue consumes one attribute value. See the scriptParser method of
// the same name for why it stops where it does.
func (p *globalScriptParser) skipTagAttrValue() bool {
	for {
		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokEOF, TokSemicolon, TokLBrace, TokRBrace, TokLT, TokGT:
			return false
		case TokHash:
			p.sc.NextSkipComments() // consume the opening #

			for {
				tok := p.sc.NextSkipComments()
				if tok.Kind == TokEOF || tok.Kind == TokLBrace ||
					tok.Kind == TokLT || tok.Kind == TokGT {
					return false
				}

				if tok.Kind == TokHash {
					break
				}
			}
		case TokLParen, TokLBracket:
			if !p.skipGroup() {
				return false
			}
		default:
			p.sc.NextSkipComments()
		}

		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokLParen, TokLBracket, TokDot, TokAmpersand, TokHash:
		default:
			return true
		}
	}
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

	var buf foldScratch
	switch string(buf.lowerFold(t)) {
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
		if p.resolvers[i].Prefix == "" || prefixContainsFold(callExpr, p.resolvers[i].Prefix) {
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
		var args strings.Builder
		args.WriteByte('"')
		args.WriteString(unquote(arg.Value))
		args.WriteByte('"')

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

			args.WriteString(`, "`)
			args.WriteString(unquote(nextArg.Value))
			args.WriteByte('"')
		}

		expr := callExpr + "(" + args.String() + ")"
		if comp := p.resolveCall(expr); comp != "" {
			// The matched resolver only had to account for the string args we
			// read above — consume whatever real tokens remain through the
			// matching ')' (there may be more args, or none) so a further
			// ".method(...)" chained after this call isn't left dangling for
			// the caller to rediscover as an orphaned bare call.
			p.skipParenBody()

			return comp
		}
	} else {
		// Try no-arg match: callExpr()
		expr := callExpr + "()"
		if comp := p.resolveCall(expr); comp != "" {
			p.skipParenBody()

			return comp
		}
	}

	// No match — try bare name for exact-match resolvers (e.g. "getFile" matches getFile(...))
	// Only use exact match to avoid substring conflicts (e.g. "_objInit" matching "objInit")
	for i := range p.resolvers {
		r := &p.resolvers[i]
		if r.Prefix != "" && prefixEqualFold(callExpr, r.Prefix) {
			if comp := matchResolverWithCache(callExpr, r); comp != "" {
				p.skipParenBody()

				return comp
			}
		}
	}

	// No match — restore scanner
	p.sc.Restore(saved)

	return ""
}

// skipParens consumes a balanced (...) group from the current scanner position.
// Returns false if the scanner is not positioned at '(' or the group is unclosed.
func (p *scriptParser) skipParens() bool {
	if p.sc.PeekSkipComments().Kind != TokLParen {
		return false
	}

	p.sc.NextSkipComments() // consume (

	return p.skipParenBody()
}

// maxArgNesting bounds how deep the scan below will recurse into nested
// argument lists.
//
// The recursion is driven by the source, and Go cannot recover from stack
// exhaustion — it is a fatal runtime error, not a panic, so the recover() that
// guards every parse entry point would not catch it. Real CFML does not nest
// calls anywhere near this deep; a file that does gets its innermost calls
// skipped, which is what this did everywhere before.
const maxArgNesting = 64

// skipParenBody consumes the rest of a (...) group whose opening '(' has
// already been consumed (depth starts at 1), recording any calls written
// inside it. Returns false if the group is unclosed (EOF reached first).
//
// It used to discard the group a token at a time, which made every call in an
// argument list invisible: `writeOutput(svc.getName())` recorded only
// writeOutput, and `arrayAppend(rows, dao.load(id))` only arrayAppend. Nothing
// else lost calls this way — a condition, a return, an assignment's RHS, a
// string concatenation, a struct or array literal and a ternary all extract
// correctly — so an argument list was the one place a call could hide.
//
// What it cost was not completeness for its own sake. A method called only
// from inside an argument list had no edge into it, so internal/codemap read
// it as unreachable and the `unresolved` scan never checked it; a broken call
// there was reported nowhere.
//
// Nested calls are dispatched back through the same two helpers the statement
// path uses, which consume the call and its own argument list whole. That is
// what makes the depth counter here stay correct through a recursive call, and
// it is also what makes a closure passed as an argument work without a case of
// its own: its body is just more tokens to scan.
func (p *scriptParser) skipParenBody() bool {
	_, ok := p.scanParenBody()

	return ok
}

// scanParenBody is skipParenBody's body, and also reports the first string
// argument at the top level of the group — which the bare-call path resolves
// componentResolvers against, and which is the only reason it used to keep a
// loop of its own.
func (p *scriptParser) scanParenBody() (firstArg string, ok bool) {
	depth := 1

	for depth > 0 {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return "", false
		}

		if tok.Kind == TokString && firstArg == "" && depth == 1 {
			firstArg = unquote(tok.Value)
		}

		switch tok.Kind { //nolint:exhaustive
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString, TokRBracket:
			p.handleLiteralToken(tok)
		}
	}

	return firstArg, true
}

// scanNestedCall dispatches an identifier met inside an argument list to the
// call-recording helpers, when it begins one.
func (p *scriptParser) scanNestedCall(tok Token) {
	if p.argNesting >= maxArgNesting {
		return
	}

	p.argNesting++
	defer func() { p.argNesting-- }()

	// `new a.b.C()` is an instantiation, and its path is not a receiver and a
	// method. Skipping the `new` keyword and letting the scan meet `a.b.C(`
	// on its own invents a call to C on a — which the test for this found
	// before the change was committed, and which is the same class of made-up
	// answer this scan exists to stop losing.
	if identEq(tok.Value, "new") {
		p.skipInstantiation()

		return
	}

	// Any other keyword is handled by what follows it rather than by being
	// read as a receiver: `function` opens a closure whose body is just more
	// tokens in this group.
	if isKeyword(tok.Value) {
		return
	}

	switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
	case TokLParen:
		p.recordBareCallAndChain(tok)
	case TokDot, TokLBracket:
		// A dotted or bracket-indexed receiver. checkBareCall walks the chain
		// and records only when it ends at a '(' — a bare `a.b` property read
		// is consumed and dropped, which is the same thing the old scan did to
		// it.
		p.checkBareCall(tok)
	}
}

// skipInstantiation consumes the component path of a `new` expression and its
// constructor arguments, whose own contents are still scanned for calls.
func (p *scriptParser) skipInstantiation() {
	// The path: an identifier, then any number of `.ident` hops. A quoted path
	// (`new "a.b.C"()`) is a single string token instead.
	if p.sc.PeekSkipComments().Kind == TokString {
		p.sc.NextSkipComments()
	} else {
		for p.sc.PeekSkipComments().Kind == TokIdent {
			p.sc.NextSkipComments()

			if p.sc.PeekSkipComments().Kind != TokDot {
				break
			}

			p.sc.NextSkipComments() // consume .
		}
	}

	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.skipParens()
	}
}

// skipBracketIndex consumes a balanced [...] group from the current scanner
// position, e.g. the dynamic key expression in REQUEST['a' & b & 'c'] or
// arr[i]. Mirrors skipParens() for (...). Returns false if the scanner isn't
// at a '[' or the group is unclosed (EOF reached first) — callers should
// treat false as "nothing to skip" / "malformed, bail" respectively (the
// only way to tell them apart is checking the token before calling).
// skipLiteralGroup consumes a struct or array literal standing as an
// assignment's right-hand side, recording any calls written inside it.
//
// A struct literal's keys may be spelled `{force = true}` as well as
// `{force: true}`, and nothing else consumes the group — `{` is not an
// identifier, so the statement scan walked straight through it and read the
// first spelling as an assignment, declaring a variable called `force`. Only
// the `=` spelling was ever affected, which is why the two look like different
// constructs in a `.cfc` and are the same one.
func (p *scriptParser) skipLiteralGroup() {
	switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
	case TokLBrace, TokLBracket:
	default:
		return
	}

	p.sc.NextSkipComments() // consume { or [

	depth := 1

	for depth > 0 {
		tok := p.sc.NextSkipComments()

		switch tok.Kind { //nolint:exhaustive
		case TokEOF:
			return
		case TokLBrace, TokLBracket:
			depth++
		case TokRBrace, TokRBracket:
			depth--
		case TokLParen:
			// Hand a nested (...) to the scan that knows it, so a call written
			// inside a literal's value is still found.
			if !p.skipParenBody() {
				return
			}
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString:
			p.handleLiteralToken(tok)
		}
	}
}

// looksLikeTagAttrs reports whether the scanner sits on `ident =`, without
// moving it.
//
// That is two tokens of lookahead and the scanner offers one, so this saves and
// restores rather than peeking twice. It is a free function because both script
// parsers need it — see the note on globalScriptParser.
func looksLikeTagAttrs(sc *Scanner) bool {
	saved := sc.Save()
	defer sc.Restore(saved)

	if sc.NextSkipComments().Kind != TokIdent {
		return false
	}

	return sc.PeekSkipComments().Kind == TokEquals
}

// parseScriptTagAttrs consumes the attribute list of a script-syntax CF tag —
// `query name="q" datasource="ds" { … }`, `savecontent variable="out" { … }`,
// `lock name="l" timeout="5" { … }`, `param name="form.id" default="0";` — and
// stops before the body or the terminating semicolon, which the caller's loop
// goes on to read as usual.
//
// The tag name is an ordinary identifier to the scanner and the parser holds no
// list of tag names, so without this the attributes were read as statements and
// every one of them declared a variable: go-to-definition on `name` anywhere in
// a file containing a `<cfquery>` written in script jumped to that tag's
// attribute. A list of tag names would be the other way to recognise this, and
// is worse: `internal/docs` is generated and would have to be imported into a
// package kept deliberately dependency-light, and a tag it does not list would
// silently go back to declaring variables.
//
// `ident ident =` is what identifies the shape, and it spells nothing else in
// CFScript — a typed declaration is `var x = …`, and a return-typed function is
// caught by the `function` case that runs before this one.
func (p *scriptParser) parseScriptTagAttrs() {
	for looksLikeTagAttrs(p.sc) {
		p.sc.NextSkipComments() // attribute name
		p.sc.NextSkipComments() // =

		if !p.skipTagAttrValue() {
			return
		}
	}
}

// skipTagAttrValue consumes one attribute value and reports whether the scan
// can continue.
//
// It takes one token, one #...# interpolation or one balanced group, and then
// keeps going only through the operators that genuinely extend a value — a
// call, an index, a dot or an `&` concatenation. Consuming "everything up to
// the body" instead is what an earlier version did, and on the tag text that
// parseFuncBody hands this parser it swallowed a whole `<cffunction>` body
// along with the `<cfset var x = 1>` inside it.
func (p *scriptParser) skipTagAttrValue() bool {
	for {
		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokEOF, TokSemicolon, TokLBrace, TokRBrace, TokLT, TokGT:
			return false
		case TokHash:
			if !p.skipHashExpr() {
				return false
			}
		case TokLParen:
			if !p.skipParens() {
				return false
			}
		case TokLBracket:
			if !p.skipBracketIndex() {
				return false
			}
		case TokIdent:
			// An attribute value can be a call: `array=structKeyArray(rows)`,
			// `result=serializeJson(body.data)`. Consuming the identifier
			// without dispatching it recorded the calls *inside* its argument
			// list — skipParens scans those — and never the call itself.
			tok := p.sc.NextSkipComments()
			p.scanNestedCall(tok)
		case TokString:
			tok := p.sc.NextSkipComments()
			p.handleLiteralToken(tok)
		default:
			p.sc.NextSkipComments()
		}

		switch p.sc.PeekSkipComments().Kind { //nolint:exhaustive
		case TokLParen, TokLBracket, TokDot, TokAmpersand, TokHash:
		default:
			return true
		}
	}
}

// skipHashExpr consumes a #...# interpolation, which an attribute value may be
// written as: `directory="#root#"` unquoted is `directory=#root#`.
func (p *scriptParser) skipHashExpr() bool {
	p.sc.NextSkipComments() // consume the opening #

	for {
		switch tok := p.sc.NextSkipComments(); tok.Kind { //nolint:exhaustive
		case TokEOF, TokSemicolon, TokLBrace, TokLT, TokGT:
			return false
		case TokHash:
			return true
		}
	}
}

// skipBracketIndex consumes a balanced [...] group, recording any calls written
// inside it.
//
// It mirrored skipParens — the *old* skipParens, which discarded its group a
// token at a time. An index is an expression like any other, so
// `sorted[ sorted.len() ]`, `arr[ f() ]` and `a.b[ f() ]` recorded nothing at
// all, and `g( arr[ f() ] )` recorded only `g`. That is the same defect
// skipParenBody exists to have fixed, left in the one group the consolidation
// did not reach.
//
// The chain text the callers build is unaffected: a bracket still poisons it
// with the literal "[]" marker, so `REQUEST[key].method()` still falls through
// to an honest "no component ref" rather than resolving as `REQUEST.method()`.
// What changes is only that the key expression is read rather than thrown away.
func (p *scriptParser) skipBracketIndex() bool {
	if p.sc.PeekSkipComments().Kind != TokLBracket {
		return false
	}

	p.sc.NextSkipComments() // consume [

	depth := 1

	for depth > 0 {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return false
		}

		switch tok.Kind { //nolint:exhaustive
		case TokLBracket:
			depth++
		case TokRBracket:
			depth--
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString:
			p.handleLiteralToken(tok)
		}
	}

	return true
}

// tryExtendChain is called after tryResolveCall has failed and the scanner
// sits at the '(' that tryResolveCall restored. It skips the argument list,
// checks for a following '.method(' continuation, and tries tryResolveCall
// on the extended chain. On success the scanner is advanced past the new
// call's ')'. On failure the scanner position is unchanged.
func (p *scriptParser) tryExtendChain(chain string) string {
	if len(p.resolvers) == 0 {
		return ""
	}

	saved := p.sc.Save()

	if !p.skipParens() {
		p.sc.Restore(saved)

		return ""
	}

	if p.sc.PeekSkipComments().Kind != TokDot {
		p.sc.Restore(saved)

		return ""
	}

	p.sc.NextSkipComments() // consume .

	next := p.sc.PeekSkipComments()
	if next.Kind != TokIdent {
		p.sc.Restore(saved)

		return ""
	}

	p.sc.NextSkipComments() // consume method name

	var extChain chainBuilder
	extChain.reset(chain)
	extChain.writeDot()
	extChain.writeString(next.Value)

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments()

		n2 := p.sc.PeekSkipComments()
		if n2.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments()

		extChain.writeDot()
		extChain.writeString(n2.Value)
	}

	if p.sc.PeekSkipComments().Kind != TokLParen {
		p.sc.Restore(saved)

		return ""
	}

	if comp := p.tryResolveCall(extChain.String()); comp != "" {
		return comp
	}

	p.sc.Restore(saved)

	return ""
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		return s[1 : len(s)-1]
	}

	return s
}

// parseStandaloneNew handles `new com.path(args)` expressions that are not
// assignment RHS (those are handled by parseNewRef). It consumes the component
// dot-path and constructor args so the scanner doesn't mis-parse the path as a
// variable.method() call site. Any chained `.method()` calls after the constructor
// are recorded as resolved call sites against the instantiated component.
func (p *scriptParser) parseStandaloneNew(newTok Token) {
	if p.sc.PeekSkipComments().Kind != TokIdent {
		return
	}

	component := p.readNewComponent()

	if p.sc.PeekSkipComments().Kind != TokLParen {
		return
	}

	p.skipParens()

	if component == "" {
		return
	}

	p.scanChainedCalls(component, newTok.Line)
}

func isKeyword(s string) bool {
	var buf foldScratch
	switch string(buf.lowerFold(s)) {
	case "var", "local", "if", "else", "for", "while", "do", "switch", "case",
		"try", "catch", "finally", "return", "break", "continue", "function",
		"component", "interface", "new", "throw", "import", "true", "false",
		"and", "or", "not", "eq", "neq", "lt", "gt", "lte", "gte", "mod",
		"in", "default", "null":
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

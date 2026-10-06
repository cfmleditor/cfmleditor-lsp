package parser

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
)

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
	fileURI             string
	baseLine            int
	resolvers           []Resolver
	resolverSet         *ResolverSet
	imports             map[string]string // last segment (lowercased) → full dot-path, from `import`
	builtinReturnLookup func(string) string
	argumentTypes       func(string, string, []Argument)
	inFunc              string          // current function scope key, empty if global
	localVarSet         map[string]bool // var'd/local. names in current function
	returnVar           string          // last "return varName" seen in current function
	pendingCalls        []pendingCall   // unresolved varName = funcCall(...) assignments
	flow                *flowBlocks     // the blocks a function body opens; nil outside a full parse

	// The flags sit together: spread between the wider fields above, each
	// was padded to eight bytes, and the struct fell into a larger size class.
	accessors    bool
	persistent   bool
	extractLinks bool // whether to extract document links
	extractCalls bool // whether to extract all call sites
	afterLT      bool // previous token was '<' — see looksLikeTagAttrs
	forceGlobal  bool // when true, addRef routes to componentRefs
	inClosure    bool // scanning a closure's body as statements — see scanClosureBody
	refThis      bool // the assignment being parsed is through `this.` — see ComponentRef.This

	// Two four-byte fields share the eight bytes after the flags.
	argNesting int32  // recursion depth of skipParenBody's scan, bounded by maxArgNesting
	returnLine uint32 // the line of the return returnVar came from
}

// pendingCall records an unresolved assignment from a function call.
type pendingCall struct {
	varName    string
	funcName   string
	expression string // argument-dependent root call, resolved after parsing
	baseVar    string // for x = baseVar.method() — resolve x to same component as baseVar
	line       uint32

	// refThis is carried to the ref as ComponentRef.This; baseScope says which
	// scope's refs baseVar may be read from. global files the ref at
	// component level, as addRef does under forceGlobal: an unscoped
	// assignment in a function is a variables-scope one, and the component
	// the call returns is what every function — and every sibling closure —
	// reads. funcKey is kept, since baseVar may still be a local. Beside line,
	// in its padding.
	refThis    bool
	global     bool
	returnExpr bool   // a return expression, grouped by function for factory-chain inference
	memberSet  bool   // not a call: `varName.funcName = …`; see checkMemberSet
	baseLocal  bool   // baseVar is the function's own, an argument or a local; see baseVarComponent
	rebinds    bool   // the call is made on varName itself; see ComponentRef.Rebinds
	block      uint32 // the block the assignment was made in; see flowBlocks
	baseScope  RefScope

	funcKey string   // scope key, empty if global
	rest    []string // calls chained after funcName, carried to the ref as ChainRest

	// The closure the assignment was made in, carried to the ref; see
	// ComponentRef.VisibleFrom.
	visibleFrom, visibleTo uint32
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
		if comp := p.resolverSet.Resolve(expr); comp != "" {
			return comp
		}

		return dynamicCall(expr)
	}

	if comp := ResolveFromCall(expr, p.resolvers); comp != "" {
		return comp
	}

	return dynamicCall(expr)
}

// dynamicCall is the component a call returns when no componentResolver
// said and the value is known to be one no component describes: "$any",
// or "" for any other call. A configured resolver is asked first, so a
// project can still type either.
//
//   - A MockBox mock — createMock, createEmptyMock, prepareMock and
//     createStub, whatever they are called on. Its methods are added at
//     runtime ($(), $property()), so typing it as the mocked component would
//     report those missing; left untyped, every call on one was "no
//     component ref", about 2,600 over the six-project corpus.
//   - A Java object with no stub to check it against, createObject("java",
//     …): a javaStubsPath resolver types it when one is configured.
func dynamicCall(expr string) string {
	name, args := finalCall(expr)

	var buf foldScratch
	switch string(buf.lowerFold(name)) {
	case "createmock", "createemptymock":
		return mockOf(mockClassArg(args))
	case "preparemock", "createstub":
		return "$any"
	case "createobject":
		if args = strings.TrimLeft(args, " \t"); args != "" && (args[0] == '"' || args[0] == '\'') &&
			hasPrefixFold(args[1:], "java") && len(args) > 5 && args[5] == args[0] {
			return "$any"
		}
	}

	return ""
}

// docReturn is the dotted component a doc comment's @return names, or "".
// A bare word is far more often a type than a component, and is left out.
func docReturn(comment string) string {
	_, after, ok := strings.Cut(comment, "@return")
	if !ok {
		return ""
	}

	rest := after
	if strings.HasPrefix(rest, "s ") || strings.HasPrefix(rest, "s\t") {
		rest = rest[1:] // @returns
	}

	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}

	rest = strings.TrimLeft(rest, " \t")

	end := 0
	for end < len(rest) && (isIdentPart(rest[end]) || rest[end] == '.') {
		end++
	}

	word := strings.Trim(rest[:end], ".")
	if !strings.Contains(word, ".") {
		return ""
	}

	return word
}

// MockPrefix marks a component a MockBox mock was made from:
// `$mock:models.User`. The resolver checks calls against the class when it
// resolves and takes them as dynamic when it does not — a spec mocking a
// class of a module the workspace lacks is not a finding — and the methods
// MockBox adds are accepted on any component.
const MockPrefix = "$mock:"

// ApplicationBase marks an implicit base that is the governing Application.cfc
// when that file is the framework instance, and otherwise the component after
// the marker: FW/1 includes a view inside the framework object, and an
// Application.cfc extending the framework is that object. config's FW/1 preset
// writes it and the resolver reads it; the parser only holds the spelling.
const ApplicationBase = "$application:"

// mockDecorations are the methods MockBox's decorateMock adds to an object it
// mocks (TestBox system/MockBox.cfc). Each returns the mock it is called on,
// except the ones that report on it ($count, $callLog, …), whose result no
// chain goes on from. Every one starts with $, which no component's own
// method conventionally does.
var mockDecorations = map[string]bool{
	"$": true, "$spy": true, "$property": true, "$getproperty": true,
	"$results": true, "$throws": true, "$callback": true, "$args": true,
	"$calllog": true, "$count": true, "$times": true, "$never": true,
	"$verifycallcount": true, "$atleast": true, "$once": true, "$atmost": true,
	"$debug": true, "$reset": true,
}

// IsMockDecoration reports whether name is a method MockBox adds to a mock.
func IsMockDecoration(name string) bool {
	if !strings.HasPrefix(name, "$") {
		return false
	}

	var buf foldScratch

	return mockDecorations[string(buf.lowerFold(name))]
}

// mockOf is the component a mock of class is, or $any without one.
func mockOf(class string) string {
	if class == "" || strings.ContainsAny(class, "#$ ") {
		return "$any"
	}

	return MockPrefix + class
}

// mockClassArg is the class a createMock or createEmptyMock argument list
// names: its first argument when that is a string, or className="…".
func mockClassArg(args string) string {
	args = strings.TrimSpace(args)
	if len(args) > 1 && (args[0] == '"' || args[0] == '\'') {
		if end := strings.IndexByte(args[1:], args[0]); end > 0 {
			return args[1 : 1+end]
		}
	}

	if m := mockClassNameRe.FindStringSubmatch(args); m != nil {
		return m[1]
	}

	return ""
}

var mockClassNameRe = regexp.MustCompile(`(?i)\bclassName\s*[=:]\s*["']([^"'#]+)["']`)

// finalCall is the name of the last call in expr and the text after its
// `(`: `getMockBox().createEmptyMock("x")` gives createEmptyMock and `"x")`.
// A `(` inside another call's arguments is not the final call's.
func finalCall(expr string) (name, args string) {
	depth, open := 0, -1

	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			if depth == 0 {
				open = i
			}

			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '"', '\'':
			// Skip a string, whose parentheses are text.
			if end := strings.IndexByte(expr[i+1:], expr[i]); end >= 0 {
				i += end + 1
			}
		}
	}

	if open < 0 {
		return "", ""
	}

	start := open
	for start > 0 && isIdentPart(expr[start-1]) {
		start--
	}

	return expr[start:open], expr[open+1:]
}

func (p *scriptParser) addRef(ref *ComponentRef) {
	ref.This = p.refThis

	if p.flow != nil && p.inFunc != "" {
		p.flow.note(ref.Variable, ref.Line, p.flow.innermost())
	}

	if p.inFunc == "" || p.forceGlobal {
		p.componentRefs = append(p.componentRefs, *ref)
	} else {
		if p.funcRefs == nil {
			p.funcRefs = make(map[string][]ComponentRef)
		}

		p.funcRefs[p.inFunc] = append(p.funcRefs[p.inFunc], *ref)
	}
}

func (p *scriptParser) addCall(call *CallSite) {
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
		p.calls = append(p.calls, *call)
	} else {
		if p.funcCalls == nil {
			p.funcCalls = make(map[string][]CallSite)
		}

		p.funcCalls[p.inFunc] = append(p.funcCalls[p.inFunc], *call)
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
// When the call stands alone — no hop or index follows it — alone is the
// expression the componentResolvers are offered for it, so a `return` can be
// typed as an assignment's right-hand side is; it is "" otherwise.
func (p *scriptParser) recordBareCallAndChain(tok Token) (alone string) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	p.addCall(&CallSite{
		FuncName: tok.Value,
		Line:     conv.Uint32(p.baseLine + tok.Line),
		Caller:   caller,
	})

	// Consume the argument list, capturing the first string arg for the
	// resolver and recording any calls written inside it. This used to be a
	// second copy of scanParenBody's loop, which is why a call nested in a
	// *bare* call's arguments stayed invisible after the dotted path learned to
	// see one: writeOutput(svc.getName()) recorded only writeOutput.
	argStart := p.sc.PeekSkipComments().Offset
	p.sc.NextSkipComments() // consume (

	firstArg, positional, ok := p.scanParenArgs()
	if !ok {
		return ""
	}

	callExpr := resolverCallExpr(tok.Value, firstArg, positional)

	// hop is funcName as a Chain entry: the call with its arguments, which
	// resolution reads an argument-sensitive return from.
	hop := p.hopSince(tok.Value, argStart)

	if next := p.sc.PeekSkipComments().Kind; next != TokDot && next != TokLBracket {
		alone = callExpr
	}

	// comp is this bare call's resolved return component (if any); it's the
	// base receiver for every subsequent chained hop below. Hops beyond the
	// first accumulate in chainHops so CanResolveCall can walk comp's type
	// forward through each intermediate call before checking the current one.
	//
	// The argument list is already consumed, so this matches callExpr as
	// built above rather than going through tryResolveCall, which expects to
	// read the arguments itself from an unconsumed "(". A call with no string
	// argument is offered as name() — getObjInit().getMailServer() — the same
	// form tryResolveCall offers an assignment's right-hand side. It is only
	// asked when a hop follows, since nothing else reads comp.
	comp := ""

	if len(p.resolvers) > 0 && p.sc.PeekSkipComments().Kind == TokDot {
		comp = p.resolveCall(callExpr)
	}

	var (
		funcName  string
		chainHops []string
	)

	first := true

	// Check for .method( chain
	for {
		if p.sc.PeekSkipComments().Kind == TokLBracket {
			// Dynamic index between hops (e.g. someFunc()[key].method()) —
			// skip it and keep walking the chain; no receiver identifier is
			// at risk of misattribution here (the base is a call return, not
			// a bare variable), so no poisoning is needed, just continuity.
			if !p.skipBracketIndex() {
				return ""
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
			// The bare call is the first hop's receiver. When a resolver
			// named its component the hop carries that instead; otherwise
			// the call goes at the front of the chain, so resolution walks
			// its declared return type. Leaving it out made `f().g()` record
			// a `g` indistinguishable from a bare call to a function named
			// g, as recordChainContinuationFrom already avoids.
			if comp == "" || !first {
				chainHops = append(chainHops, hop)
			}

			first = false
			funcName = methTok.Value

			hops := make([]string, len(chainHops))
			copy(hops, chainHops)

			p.addCall(&CallSite{
				FuncName:  funcName,
				Component: comp,
				Chain:     hops,
				Line:      conv.Uint32(p.baseLine + tok.Line),
				Caller:    caller,
				Resolved:  comp != "",
			})

			// Consume this hop's arguments too, so the chain walk can
			// continue — through the same scan, so a call written inside
			// them is found: this was the third copy of the paren loop and
			// the last one still losing `a().b(svc.c())`.
			argStart = p.sc.PeekSkipComments().Offset
			p.sc.NextSkipComments() // consume (

			if _, ok := p.scanParenBody(); !ok {
				return ""
			}

			hop = p.hopSince(funcName, argStart)
		} else {
			break
		}
	}

	return alone
}

// scopeReceiver says how a call made directly on a scope is recorded, for the
// scopes a function body dispatches as call receivers. `this.` and
// `variables.` name a member of the component being parsed, so the call is
// recorded unqualified and resolves against the file's own functions. Every
// other scope holds a value put there at runtime, so the receiver is kept and
// the component is `$any`. ok is false for a word that is not one of them.
//
// Every path that records such a call goes through here — a statement, a
// `return` and an assignment's right-hand side. The last two used to record
// the scope itself as the receiver, so `x = variables.f()` reported
// "variable 'variables' has no component ref" where the statement
// `variables.f()` resolved.
func scopeReceiver(word string) (variable, component string, ok bool) {
	var buf foldScratch

	switch string(buf.lowerFold(word)) {
	case "this", "variables":
		return "", "", true
	case "local", "arguments", "request", "session", "application", "server":
		return word, "$any", true
	default:
		return "", "", false
	}
}

// onScope gives c the receiver scopeReceiver says a call made directly on
// the scope word has, when word is one, and records whether it was `this.`:
// the parser records `this.f()` and `f()` alike, and only the first reaches
// onMissingMethod.
func (c *CallSite) onScope(word string) {
	v, comp, ok := scopeReceiver(word)
	if !ok {
		return
	}

	c.Variable, c.Component, c.Resolved = v, comp, comp != ""
	c.This = identEq(word, "this")
}

// recordCallFromChain records a call site when a dot chain ending in ( is detected.
// fullChain is e.g. "VARIABLES.service.GetData" or just "GetData", line is the source line.
func (p *scriptParser) recordCallFromChain(fullChain string, line int) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	recv, method, ok := strings.CutLast(fullChain, ".")
	if !ok {
		// Bare function call (no dot)
		p.addCall(&CallSite{
			FuncName: fullChain,
			Line:     conv.Uint32(p.baseLine + line),
			Caller:   caller,
		})

		return
	}

	call := CallSite{
		FuncName: method,
		Variable: recv,
		Line:     conv.Uint32(p.baseLine + line),
		Caller:   caller,
	}

	call.onScope(recv)

	p.addCall(&call)
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
func (p *scriptParser) continueChainCalls(baseVar, funcName string, line int) []string {
	callExpr := funcName
	if baseVar != "" {
		callExpr = baseVar + "." + funcName
	}

	comp, hop, ok := p.skipParensResolving(callExpr)
	if !ok {
		return nil
	}

	return p.recordChainContinuation(baseVar, hop, comp, line)
}

// skipParensResolving is skipParens for the first hop of a chain: it also
// matches componentResolvers against callExpr with the group's string
// arguments (see resolverCallExpr), so a hop that names its component in an argument —
// getService("company") — hands the next hop that component. A Chain entry
// is only a method name, and resolving the hop again from it later sees
// getService() and can only reach a resolver that ignores which service was
// asked for. Unlike tryResolveCall this reads the argument wherever the scan
// finds it, so the named form getService(service="company") resolves too.
// comp is "" when no resolver matches.
//
// hop is the call as a chain entry for the hops after it: with its arguments
// when a hop follows, so resolution can read an argument-sensitive return
// from them, and the bare method name otherwise.
func (p *scriptParser) skipParensResolving(callExpr string) (comp, hop string, ok bool) {
	_, hop, _ = strings.CutLast("."+callExpr, ".")

	if p.sc.PeekSkipComments().Kind != TokLParen {
		return "", hop, false
	}

	start := p.sc.PeekSkipComments().Offset
	p.sc.NextSkipComments() // consume (

	firstArg, positional, ok := p.scanParenArgs()
	if !ok || p.sc.PeekSkipComments().Kind != TokDot {
		return "", hop, ok
	}

	hop = p.hopSince(hop, start)
	if firstArg == "" {
		return "", hop, true
	}

	return p.resolveCall(resolverCallExpr(callExpr, firstArg, positional)), hop, true
}

// recordChainContinuation is continueChainCalls' shared core, factored out so
// it can also run after the first hop's resolver lookup SUCCEEDED — e.g.
// "document.getJavaUtils()" matching a "getJavaUtils" resolver even though
// the real expression continues ".getRGBColor(r=16,g=58,b=59)". In that case
// tryResolveCall has already consumed the first hop's own "(...)" (it must,
// to know where the match ends), so — unlike continueChainCalls — the
// caller here must already have the scanner positioned right after that
// closing ')', with no extra skipParens() call needed for the first hop.
//
// baseComp is the component the first hop was already resolved to, when the
// caller knows it — tryResolveCall matching "getService(\"company\")" against
// its string arguments. The hops then start from that component rather than
// re-deriving the first hop from its bare name: a chain entry carries no
// arguments, so the resolve step would see "getService()" and could only reach
// a resolver that ignores which service was asked for.
//
// It returns the names of the hops it consumed after funcName, in order, which
// an assignment keeps on its ref (ComponentRef.ChainRest) so the variable is
// typed by the last call rather than by funcName.
//
// funcName is the first hop as a chain entry: a bare name, or a CallHop when
// the caller read its arguments before consuming them.
func (p *scriptParser) recordChainContinuation(baseVar, funcName, baseComp string, line int) (consumed []string) {
	// A chain on a call made directly on a scope carries the receiver
	// scopeReceiver gives that call: none for this component's own scopes,
	// and a dynamic one, all the way down, for the rest.
	if v, comp, ok := scopeReceiver(baseVar); ok {
		baseVar = v

		if comp != "" {
			baseComp = comp
		}
	}

	return p.recordChainContinuationFrom(baseVar, nil, funcName, baseComp, line)
}

// recordChainContinuationFrom is recordChainContinuation for a chain that
// already has hops before funcName: prior is carried at the front of every
// hop's Chain.
func (p *scriptParser) recordChainContinuationFrom(baseVar string, prior []string, funcName, baseComp string, line int) (consumed []string) {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	chainHops := slices.Clone(prior)

	// hop is funcName as a Chain entry. The first is whatever the caller
	// passed, which carries its arguments when the caller read them; every
	// later one carries them.
	hop := funcName
	first := true

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments() // consume method name

		if p.sc.PeekSkipComments().Kind != TokLParen {
			if baseComp == "" || !first {
				chainHops = append(chainHops, hop)
			}

			first = false
			funcName = PropertyHop(methTok.Value)
			hop = funcName
			consumed = append(consumed, funcName)

			continue
		}

		if baseComp == "" || !first {
			chainHops = append(chainHops, hop)
		}

		first = false
		funcName = methTok.Value
		hop = callHopAt(p.sc, funcName)
		consumed = append(consumed, CallHop(callExpressionAt(p.sc, funcName)))

		if !p.skipParens() {
			return consumed
		}

		if p.extractCalls {
			hops := make([]string, len(chainHops))
			copy(hops, chainHops)

			p.addCall(&CallSite{
				FuncName:  funcName,
				Variable:  baseVar,
				Component: baseComp,
				Chain:     hops,
				Line:      conv.Uint32(p.baseLine + line),
				Caller:    caller,
				Resolved:  baseComp != "",
			})
		}
	}

	return consumed
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

	comp, hop, ok := p.skipParensResolving(fullChain)
	if !ok {
		return
	}

	base, _, _ := strings.CutLast(fullChain, ".")

	p.recordChainContinuation(base, hop, comp, line)
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
	p.funcLinks = extractLinksByScope(p.sc.src, p.baseLine, p.scopes, p.funcLinks, &p.links)
}

// parseVarDecl handles: var name = expr.
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
		Line: conv.Uint32(p.baseLine + tok.Line),
	})

	if p.chainedAssign(nameTok.Value) {
		return
	}

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
	case "entityload", "entityloadbypk":
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

	// `var node = this` holds this component, as `variables.x = this` does
	// in parseBodyScopedVar; `this` is a keyword, so the check below would
	// otherwise drop it. `x = this.y` reads a member and is not this.
	if rhs.Kind == TokIdent && identEq(rhs.Value, "this") {
		st := p.sc.Save()
		p.sc.NextSkipComments()

		if p.sc.PeekSkipComments().Kind != TokDot {
			if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
				p.addRef(&ComponentRef{
					Variable: varName, Component: selfPath,
					URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
				})
			}

			return
		}

		p.sc.Restore(st)
	}

	if rhs.Kind != TokIdent || isKeyword(rhs.Value) {
		return
	}

	p.sc.NextSkipComments() // consume first ident

	var fullChain chainBuilder
	fullChain.reset(rhs.Value)

	prevIdent, lastIdent, ok := p.walkChain(&fullChain, rhs.Value, Token{Line: line})
	if !ok {
		return
	}

	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.recordCallFromChain(fullChain.String(), line)

		if comp := p.tryResolveCall(fullChain.String()); comp != "" {
			rest := p.recordChainContinuation(receiverOf(fullChain.String()), lastIdent, comp, line)
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp,
				ChainBase: prevIdent, ChainMethod: lastIdent, ChainRest: rest,
				URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
				Rebinds: rebinds(varName, receiverOf(fullChain.String())),
			})
		} else if comp, ext := p.tryExtendChain(fullChain.String()); comp != "" {
			rest := p.continueExtendedChain(receiverOf(fullChain.String()), lastIdent, ext, line)
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp, ChainRest: rest,
				URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
			})
		} else if p.builtinReturnLookup != nil {
			if comp := p.builtinReturnLookup(lastIdent); comp != "" {
				p.addRef(&ComponentRef{
					Variable: varName, Component: comp,
					URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
				})
			} else {
				p.addPendingCall(varName, prevIdent, lastIdent, fullChain.String(), line)
			}
		} else {
			p.addPendingCall(varName, prevIdent, lastIdent, fullChain.String(), line)
		}
	} else {
		p.assignNonCall(varName, fullChain.String(), line)
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
		Line:     conv.Uint32(p.baseLine + scopeTok.Line),
		Caller:   caller,
	}

	// The scope is read from the token rather than from the Scope value the
	// dispatch passed: what matters is whether the scope is this component's
	// own (this., variables.) or not, which the token says directly. Testing
	// the enum once recorded request., session. and application. calls as calls
	// to functions of that name in this file, when they were dispatched as
	// ScopeVariables.
	call.onScope(scopeTok.Value)

	p.addCall(&call)

	comp, hop, ok := p.skipParensResolving(nameTok.Value)
	if !ok {
		return
	}

	// A hop chained onto it is a call on what the first one returned, and
	// leaving it to the outer loop makes it an orphaned bare call — the same
	// wrong answer recordChainFromScope exists to stop. A member of this
	// component walks the chain through its declared return type; a dynamic
	// receiver stays dynamic all the way down, as a literal receiver's chain
	// already does.
	if call.Component == "" {
		p.recordChainContinuation("", hop, comp, scopeTok.Line)

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

		p.addCall(&CallSite{
			FuncName:  methTok.Value,
			Variable:  recv,
			Component: "$any",
			Line:      conv.Uint32(p.baseLine + line),
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
		Line: conv.Uint32(p.baseLine + tok.Line),
	})

	isLocal := scope == ScopeLocal || scope == ScopeArguments
	if isLocal && p.localVarSet != nil {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	}

	if !isLocal {
		p.forceGlobal = true
	}

	p.refThis = scope == ScopeThis
	defer func() { p.refThis = false }()

	if p.chainedAssign(nameTok.Value) {
		p.forceGlobal = false

		return
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
		case "entityload", "entityloadbypk":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, tok.Line)
		case "this":
			p.sc.NextSkipComments()

			// Only treat `scope.x = this` (bare) as a self-ref.
			// `scope.x = this.prop = ...` is a chained assignment — the real
			// component comes from the continuation, so skip the ref here.
			if p.sc.PeekSkipComments().Kind != TokDot {
				if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
					p.addRef(&ComponentRef{
						Variable: nameTok.Value, Component: selfPath,
						URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + tok.Line),
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
		case "request", "session", "application", "server":
			// See the matching case in handleBodyToken for why this is needed:
			// without it, a top-level (outside any function) "REQUEST.x = ..."
			// assignment falls through to the bare-call path and its RHS
			// component type is silently dropped.
			p.parseScopedVar(tok, sharedScopeOf(tok))
		case "return":
			// A script island can be inside a tag function whose scope was seeded.
			if p.inFunc != "" {
				p.checkReturnComponent()
			}
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
				named := !isKeyword(tok.Value) || operatorWordIsName(tok, peek)

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
				} else if next.Kind == TokLParen && named {
					_ = prevIdent

					varName := ""
					chain := retVal.String()

					if recv, _, ok := strings.CutLast(chain, "."); ok {
						varName = recv
					}

					funcName := lastIdent
					line := tok.Line

					comp, hop, ok := p.skipParensResolving(chain)
					if !ok {
						return
					}

					p.addCall(&CallSite{
						FuncName: funcName,
						Variable: varName,
						Line:     conv.Uint32(p.baseLine + line),
					})

					// Continue walking further .method() hops chained off this
					// call's return value (e.g. "table.getTable().setSkipFirstHeader(...)")
					// — without this, the scanner resumes right after this hop's
					// "(" and the next ".method(" is rediscovered as an orphaned,
					// unqualified bare call.
					p.recordChainContinuation(varName, hop, comp, line)
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
	line := conv.Uint32(p.baseLine + startTok.Line)

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
	// A flag WireBox reads by its presence (`inject delegate`) is recorded
	// with an empty value, and taken out of the positional form below.
	flags := 0

	for i, tok := range tokens {
		if tok.Kind != TokIdent {
			continue
		}

		if i+1 < len(tokens) && tokens[i+1].Kind == TokEquals {
			if i+2 < len(tokens) && (tokens[i+2].Kind == TokString || tokens[i+2].Kind == TokIdent || tokens[i+2].Kind == TokNumber) {
				val := propertyAttributeValue(tokens, i+2)
				attrs[strings.ToLower(tok.Value)] = val
			}

			continue
		}

		if key := strings.ToLower(tok.Value); propertyFlags[key] && (i > 0 || len(tokens) > 1) {
			if _, set := attrs[key]; !set {
				attrs[key] = ""
			}

			flags++
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

				if flags > 0 && propertyFlags[strings.ToLower(tok.Value)] {
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

	if typeName != "" {
		attrs["type"] = typeName
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

		if tok.Kind != TokIdent {
			continue
		}

		switch {
		case strings.EqualFold(tok.Value, "extends"):
			if val, ok := p.attrValue(); ok && val.Kind == TokString {
				p.extends = unquote(val.Value)
			}
		case strings.EqualFold(tok.Value, "accessors"):
			if val, ok := p.attrValue(); ok {
				p.accessors = isTruthy(unquote(val.Value))
			}
		case strings.EqualFold(tok.Value, "persistent"):
			if val, ok := p.attrValue(); ok &&
				((val.Kind == TokString && isTruthy(unquote(val.Value))) || (val.Kind == TokIdent && isTruthy(val.Value))) {
				p.persistent = true
			}
		}
	}
}

// attrValue reads `= value` after an attribute name, returning the value
// token, or false when no `=` follows.
func (p *scriptParser) attrValue() (Token, bool) {
	if p.sc.PeekSkipComments().Kind != TokEquals {
		return Token{}, false
	}

	p.sc.NextSkipComments()

	return p.sc.NextSkipComments(), true
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

// skipFunctionAttrs steps over a declaration's attributes (the shared
// skipFunctionAttrs), reading each value as the expression it is: a string's
// #...# spans and a bare call are recorded like any other.
func (p *scriptParser) skipFunctionAttrs() {
	skipFunctionAttrs(p.sc, func(v Token) {
		switch {
		case v.Kind == TokString:
			p.scanInterpolation(v)
		case v.Kind == TokIdent && p.sc.PeekSkipComments().Kind == TokLParen:
			p.recordBareCallAndChain(v)
		default:
		}
	})
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
		applyParameterDocs(docComment, args)
	}

	if p.inFunc == "" {
		if p.argumentTypes != nil {
			p.argumentTypes(nameTok.Value, access, args)
		}
	}

	funcLine := p.baseLine + startTok.Line

	p.funcs = append(p.funcs, FunctionDef{
		Name:       nameTok.Value,
		URI:        uriFromString(p.fileURI),
		Line:       conv.Uint32(funcLine),
		Arguments:  args,
		ReturnType: returnType,
		DocReturn:  docReturn(docComment),
	})

	// Process body: set inFunc scope, parse assignments, then clear
	p.skipFunctionAttrs()

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
		applyParameterDocs(docComment, args)
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
		switch tok := p.sc.NextSkipComments(); tok.Kind {
		case TokEOF:
			return false
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		default:
			// Any other token is passed over.
		}
	}

	return true
}

// recordFunctionValue files the declaration and walks the body, as
// parseFunction does for a declared method.
func (p *scriptParser) recordFunctionValue(name string, startTok Token, args []Argument) {
	funcLine := p.baseLine + startTok.Line

	p.funcs = append(p.funcs, FunctionDef{
		Name:      name,
		URI:       uriFromString(p.fileURI),
		Line:      conv.Uint32(funcLine),
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

		switch tok.Kind {
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
		Line: conv.Uint32(p.baseLine + nameTok.Line),
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

	call := CallSite{
		FuncName:  methTok.Value,
		Component: component,
		Line:      conv.Uint32(p.baseLine + startTok.Line),
		Caller:    caller,
		Resolved:  true,
	}

	// `super::method()` is the parent's method, the call `super.method()` is,
	// not a static call on a component named super.
	if identEq(component, "super") {
		call.Variable, call.Component, call.Resolved = component, "", false
	}

	p.addCall(&call)

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

			switch t.Kind {
			case TokIdent:
				p.scanNestedCall(t)
			case TokString, TokRBracket:
				p.handleLiteralToken(t)
			default:
				// Any other token is passed over.
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

		p.addCall(&CallSite{
			FuncName:  methTok.Value,
			Component: "$any",
			Line:      conv.Uint32(p.baseLine + recv.Line),
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
			switch peek.Kind {
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

		switch peek.Kind {
		case TokLParen, TokLBrace, TokLBracket:
			depth++
		case TokRParen, TokRBrace, TokRBracket:
			depth--
		case TokIdent:
			p.scanNestedCall(peek)
		case TokString:
			p.handleLiteralToken(peek)
		default:
			// Any other token is passed over.
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

	// The body is a block: what it assigns runs before a return in it.
	if p.flow != nil {
		p.flow.open(tok.Offset, "")
		defer p.flow.close("")
	}

	// Enter function scope with a temporary key (will fix up after finding end)
	prevInFunc := p.inFunc
	prevLocalVarSet := p.localVarSet
	prevReturnVar, prevReturnLine := p.returnVar, p.returnLine

	// Use a placeholder funcKey; we'll remap after finding the real end
	tempKey := funcKey(funcLine, funcLine)
	p.inFunc = tempKey
	p.returnVar = ""

	p.localVarSet = make(map[string]bool)
	for _, a := range args {
		p.localVarSet[strings.ToLower(a.Name)] = true

		if comp := argumentComponentType(&a); comp != "" {
			if p.funcRefs == nil {
				p.funcRefs = make(map[string][]ComponentRef)
			}

			p.funcRefs[tempKey] = append(p.funcRefs[tempKey], ComponentRef{
				Variable: a.Name, Component: comp,
				URI: uriFromString(p.fileURI), Line: conv.Uint32(funcLine),
			})
		}
	}

	depth := 1

	var endLine int

	for depth > 0 {
		t := p.sc.NextSkipComments()
		if t.Kind == TokEOF {
			p.inFunc = prevInFunc
			p.localVarSet = prevLocalVarSet
			p.returnVar, p.returnLine = prevReturnVar, prevReturnLine

			if p.flow != nil {
				p.flow.dropBraceless()
			}

			return t.Line
		}

		if p.flow != nil && len(p.flow.ends) > 0 {
			p.flow.closeEnded(t.Offset)
		}

		afterLT := p.afterLT
		p.afterLT = t.Kind == TokLT

		switch t.Kind {
		case TokLBrace:
			depth++

			if p.flow != nil {
				p.flow.open(t.Offset, "")
			}
		case TokRBrace:
			depth--
			if depth == 0 {
				endLine = t.Line
			} else if p.flow != nil {
				p.flow.close("")
			}
		case TokIdent:
			if depth > 0 {
				if p.flow != nil {
					p.openBracelessBody(t)
				}

				mark := p.flowMark()
				p.handleBodyToken(t, depth, afterLT)
				p.stampFlow(mark)
			}
		case TokString, TokRBracket:
			if depth > 0 {
				mark := p.flowMark()
				p.handleLiteralToken(t)
				p.stampFlow(mark)
			}
		default:
			// Any other token is passed over.
		}
	}

	realKey := funcKey(funcLine, p.baseLine+endLine)
	p.rekeyFunc(tempKey, realKey)

	p.inFunc = realKey

	// Resolve ReturnComponent on the current function
	if len(p.funcs) > 0 {
		p.settleReturnComponent(&p.funcs[len(p.funcs)-1])
	}

	// Exit function scope
	p.inFunc = prevInFunc
	p.localVarSet = prevLocalVarSet
	p.returnVar, p.returnLine = prevReturnVar, prevReturnLine

	return endLine
}

// openBracelessBody opens a block for the statement a braceless `if`, `else`,
// `for` or `while` governs, so an assignment in it is read as one that may not
// run. It acts only when the whole statement is one line ending in a
// semicolon with no brace in it: CFScript needs neither, and a statement it
// cannot bound is left as it was read before, as one that always runs. A
// lookahead on a saved scanner state; the tokens are read again.
func (p *scriptParser) openBracelessBody(t Token) {
	if len(t.Value) < 2 || len(t.Value) > 5 {
		return
	}

	var buf foldScratch

	withCondition := false

	switch string(buf.lowerFold(t.Value)) {
	case "if", "for", "while":
		withCondition = true
	case "else":
	default:
		return
	}

	saved := p.sc.Save()
	defer p.sc.Restore(saved)

	if withCondition {
		if p.sc.NextSkipComments().Kind != TokLParen {
			return
		}

		for depth := 1; depth > 0; {
			switch p.sc.NextSkipComments().Kind {
			case TokLParen:
				depth++
			case TokRParen:
				depth--
			case TokEOF:
				return
			default:
			}
		}
	}

	first := p.sc.NextSkipComments()
	switch first.Kind {
	case TokLBrace, TokRBrace, TokSemicolon, TokEOF:
		return
	default:
	}

	if !withCondition && first.Kind == TokIdent && strings.EqualFold(first.Value, "if") {
		return // `else if` is the if's block
	}

	for tok, depth := first, 0; ; tok = p.sc.NextSkipComments() {
		if tok.Line != first.Line {
			return
		}

		switch tok.Kind {
		case TokLParen, TokLBracket:
			depth++
		case TokRParen, TokRBracket:
			depth--
		case TokLBrace, TokRBrace, TokEOF:
			if depth <= 0 {
				return
			}
		case TokSemicolon:
			if depth == 0 {
				p.flow.openBraceless(t.Offset, tok.Offset)

				return
			}
		default:
		}
	}
}

// flowMark is how many pending calls there were before a statement, so
// stampFlow can find the ones it made. A ref notes its block itself, in
// addRef; a pending call is made at a dozen sites and its ref only later.
func (p *scriptParser) flowMark() int { return len(p.pendingCalls) }

// stampFlow records the innermost open block as the one the pending calls
// made since mark were made in.
func (p *scriptParser) stampFlow(mark int) {
	stampPending(p.flow, p.pendingCalls[mark:])
}

// stampPending gives calls the innermost block open in fb.
func stampPending(fb *flowBlocks, calls []pendingCall) {
	if fb == nil || len(calls) == 0 {
		return
	}

	b := fb.innermost()
	for i := range calls {
		calls[i].block = b
	}
}

// sharedScopeOf is the scope a request., session., application. or server.
// keyword names. Only called from the dispatch arms for those four.
func sharedScopeOf(tok Token) Scope {
	scope, _ := ScopeForPrefix(tok.Value)

	return scope
}

// rekeyFunc moves what a function body recorded under the placeholder key it
// was parsed with to its real key, once its end is known.
func (p *scriptParser) rekeyFunc(tempKey, realKey string) {
	if refs, ok := p.funcRefs[tempKey]; ok {
		delete(p.funcRefs, tempKey)
		p.funcRefs[realKey] = refs
	}

	if calls, ok := p.funcCalls[tempKey]; ok {
		delete(p.funcCalls, tempKey)
		p.funcCalls[realKey] = calls
	}

	for i := range p.pendingCalls {
		if p.pendingCalls[i].funcKey == tempKey {
			p.pendingCalls[i].funcKey = realKey
		}
	}
}

// settleReturnComponent gives f the component its return variable holds,
// from a ref in its body or at component level, or records the variable for
// resolvePendingCalls to settle later.
func (p *scriptParser) settleReturnComponent(f *FunctionDef) {
	if f.ReturnComponent != "" || p.returnVar == "" {
		return
	}

	// Look up returnVar in this function's refs, then in componentRefs. A
	// name read through variables. or this. is the component's alone, and
	// only a ref made through that scope holds it. Of the refs for it, the
	// ones that may reach the return decide; a closure's local of the same
	// name is another variable.
	name, called, scope := returnedVar(p.returnVar)

	lookIn := [][]ComponentRef{p.funcRefs[p.inFunc], p.componentRefs}
	admit := func(ref *ComponentRef) bool { return ref.VisibleTo == 0 }

	if scope != RefAny {
		lookIn = lookIn[1:]
		admit = scope.Admits
	}

	for _, refs := range lookIn {
		reaching := p.flow.reaching(refs, name, p.returnLine, admit)
		if len(reaching) == 0 {
			continue
		}

		// A ref still to be typed, or an assignment among those reaching the
		// return that only resolvePendingCalls can type, is settled there.
		// The ones that may reach it must agree: two branches assigning two
		// components give the function no return type.
		if !slices.ContainsFunc(reaching, chainPending) && !assignedBetween(p.pendingCalls, name, reaching[0].Line, p.returnLine) {
			f.ReturnComponent = returnedComponent(agreedComponent(reaching, func(ref *ComponentRef) string { return ref.Component }), called)
		}

		break
	}

	if f.ReturnComponent != "" {
		return
	}

	// If still unresolved, store for deferred resolution
	f.returnVar, f.returnLine = p.returnVar, p.returnLine
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
	case "request", "session", "application", "server":
		// Request/session/application/server-scoped assignments (e.g. "REQUEST.generator =
		// document.createTable(1);") — checkAssignRef's default path only recognizes
		// a bare "x = ..." (identifier directly followed by "="); for a scope-prefixed
		// LHS the next token is "." not "=", so without this case it falls through to
		// checkBareCall and the assignment (and any component type it establishes) is
		// silently dropped. parseBodyScopedVar already handles the "scope.name = rhs"
		// vs "scope.name.method()" split correctly for variables./this./arguments.; the
		// same handling applies verbatim here. Treated as global (forceGlobal), same as
		// this./variables., since these scopes outlive the current function.
		//
		// The declaration keeps its own scope. It used to be recorded as
		// ScopeVariables, which put `application.cache` among the component's
		// variables and left go-to-definition on `application.cache` nothing to
		// find in a script-syntax Application.cfc. Only the scope is recorded
		// differently: the component ref, the forced-global filing and the call
		// extraction are the same for either value.
		p.parseBodyScopedVar(tok, sharedScopeOf(tok))
	case "return":
		if p.inClosure {
			// A closure's return is the closure's, not the enclosing
			// function's: its call is recorded, its type is not the
			// function's return type.
			if peek := p.sc.PeekSkipComments(); peek.Kind == TokIdent {
				_, _ = p.returnCall(peek)
			}

			return
		}

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
	if p.inFunc != "" {
		p.pendingCalls = append(p.pendingCalls, pendingCall{
			varName: scriptReturnExpression(p.sc), funcKey: p.inFunc, returnExpr: true,
		})
	}

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
		bareThis := identEq(peek.Value, "this") && p.returnsBareThis()
		scopedVar, scope := p.returnedScopedVar()

		// return varName — track for resolution after body parse. Only the
		// name alone is what the variable holds: `return shell.pwd()`
		// returns what pwd() does, and read as `return shell` it declared the
		// function a Shell. A call on a dynamic value is dynamic, though,
		// which returnsCallOn keeps.
		p.returnVar = peek.Value
		p.returnLine = conv.Uint32(p.baseLine + peek.Line)

		if p.flow != nil {
			p.flow.noteReturn(p.returnLine)
		}

		bare, call := p.returnCall(peek)
		if !bare {
			p.returnVar = returnsCallOn + peek.Value
		}

		// A bare call standing alone is typed as it would be on an
		// assignment's right-hand side: `return getInstance( "X" )` returns
		// what a componentResolver says getInstance( "X" ) is.
		if call != "" && p.atStatementEnd() {
			if comp := p.resolveCall(call); comp != "" && len(p.funcs) > 0 {
				p.funcs[len(p.funcs)-1].ReturnComponent = comp
			}
		}

		switch {
		case bareThis:
			p.returnVar = returnsThis
		case scopedVar != "":
			p.returnVar = scopedReturnVar(scope, scopedVar)
		}

		return
	}

	if comp != "" && len(p.funcs) > 0 {
		p.funcs[len(p.funcs)-1].ReturnComponent = comp
	}
}

// atStatementEnd reports whether the next token ends a statement.
func (p *scriptParser) atStatementEnd() bool {
	switch p.sc.PeekSkipComments().Kind {
	case TokSemicolon, TokRBrace, TokEOF:
		return true
	default:
		return false
	}
}

// returnedScopedVar is the name a return reads through variables. or this.
// when that is the whole returned expression — `return variables.print;` —
// and the scope it names, without moving the scanner. It is "" for anything
// else, `return variables.print.line()` and `return this;` included.
func (p *scriptParser) returnedScopedVar() (string, RefScope) {
	saved := p.sc.Save()
	defer p.sc.Restore(saved)

	scopeTok := p.sc.NextSkipComments()

	scope := RefVariables
	if identEq(scopeTok.Value, "this") {
		scope = RefThis
	} else if !identEq(scopeTok.Value, "variables") {
		return "", RefAny
	}

	if p.sc.NextSkipComments().Kind != TokDot {
		return "", RefAny
	}

	name := p.sc.NextSkipComments()
	if name.Kind != TokIdent {
		return "", RefAny
	}

	if !p.atStatementEnd() {
		return "", RefAny
	}

	return name.Value, scope
}

// returnsBareThis reports whether the `this` the scanner is on is the whole
// returned expression — `return this;` rather than `return this.x;` —
// without moving the scanner.
func (p *scriptParser) returnsBareThis() bool {
	saved := p.sc.Save()
	defer p.sc.Restore(saved)

	p.sc.NextSkipComments() // this

	return p.atStatementEnd()
}

// returnCall records the call a `return` statement makes, through the same
// walkers a statement uses: `return svc.a().b()` and `return f().g()` carry
// the receiver to every hop, and `return variables.f()` is recorded as the
// statement `variables.f()` is. It had a chain walk of its own that recorded
// the first call and left the rest to the outer loop, which met `.b()` with
// nothing before it and recorded a bare call to `b`.
//
// Only walkers that record calls are reached, never the statement dispatch
// itself: a return holds an expression, and `return x == 1` read as a
// statement is the assignment `x = …`. tok is the peeked first token after
// `return`; a keyword other than a scope is left for the caller's loop.
// It reports whether the name stood alone, the case in which the function
// returns what that variable holds, and for a bare call standing alone the
// expression the componentResolvers are offered for it.
func (p *scriptParser) returnCall(tok Token) (bare bool, call string) {
	_, _, isScope := scopeReceiver(tok.Value)
	if !isScope && isKeyword(tok.Value) {
		return true, ""
	}

	p.sc.NextSkipComments()

	next := p.sc.PeekSkipComments()

	switch {
	case isScope && next.Kind == TokDot:
		p.sc.NextSkipComments()

		nameTok := p.sc.PeekSkipComments()
		if nameTok.Kind != TokIdent {
			return false, ""
		}

		p.sc.NextSkipComments()
		p.scopedCall(tok, nameTok)
	case next.Kind == TokLParen:
		call = p.recordBareCallAndChain(tok)
	case next.Kind == TokDot || next.Kind == TokLBracket:
		p.checkBareCall(tok)
	case next.Kind == TokDoubleColon:
		p.parseStaticCall(tok)
	default:
		return true, ""
	}

	return false, call
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

	// An inline component, `new component { … }` (Lucee), has no file to
	// name: its type is the body that follows. It was read as a path, a
	// component literally called "component", which then failed every
	// method check against it.
	if identEq(tok.Value, "component") && p.sc.PeekSkipComments().Kind == TokLBrace {
		return "$any"
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

	_, last, ok := strings.CutLast(path, ".")
	if !ok {
		return
	}

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

	if p.sc.NextSkipComments().Kind != TokComma {
		return ""
	}

	arg2 := p.sc.NextSkipComments()
	if arg2.Kind == TokRParen || arg2.Kind == TokEOF {
		return ""
	}

	// A path computed at run time — createObject( "component",
	// drivernames[ type ] ), "pkg." & name — names whichever component the
	// program picks, as an unmapped #...# in a literal path does, and is
	// dynamic for the same reason. Nothing was recorded for it, so every call
	// on the variable was "no component ref", and the scan stopped inside the
	// argument.
	if arg2.Kind != TokString || !p.atArgEnd() {
		p.skipComputedArg(arg2)

		return "$any"
	}

	if identEq(arg1Val, "component") {
		return unquote(arg2.Value)
	}

	// Non-component createObject (e.g. java) — try resolvers
	expr := "createObject(\"" + arg1Val + "\",\"" + unquote(arg2.Value) + "\")"

	return p.resolveCall(expr)
}

// atArgEnd reports whether the next token ends an argument.
func (p *scriptParser) atArgEnd() bool {
	k := p.sc.PeekSkipComments().Kind

	return k == TokRParen || k == TokComma
}

// skipComputedArg consumes the rest of an argument whose first token, first,
// is already consumed, recording the calls it makes, and stops before the ,
// or ) that ends it.
func (p *scriptParser) skipComputedArg(first Token) {
	depth := 0

	for tok := first; ; tok = p.sc.NextSkipComments() {
		switch tok.Kind {
		case TokLParen, TokLBracket, TokLBrace:
			depth++
		case TokRParen, TokRBracket, TokRBrace:
			depth--

			p.handleLiteralToken(tok)
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString:
			p.handleLiteralToken(tok)
		case TokEOF:
			return
		default:
		}

		if next := p.sc.PeekSkipComments().Kind; next == TokEOF || (depth <= 0 && (next == TokRParen || next == TokComma)) {
			return
		}
	}
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
		Line: conv.Uint32(p.baseLine + varTok.Line),
	})

	if p.chainedAssign(nameTok.Value) {
		return
	}

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
	case "entityload", "entityloadbypk":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(nameTok.Value, varTok.Line)
	default:
		// The same right-hand side an assignment outside a function has.
		p.checkVarRHS(nameTok.Value, varTok.Line)
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

	if p.sc.PeekSkipComments().Kind != TokEquals {
		p.scopedCall(scopeTok, nameTok)

		return
	}

	p.sc.NextSkipComments() // consume =

	p.vars = append(p.vars, VarDef{
		Name: nameTok.Value, Scope: scope,
		Line: conv.Uint32(p.baseLine + scopeTok.Line),
	})

	isLocal := scope == ScopeLocal || scope == ScopeArguments
	if isLocal {
		p.localVarSet[strings.ToLower(nameTok.Value)] = true
	} else {
		p.forceGlobal = true
	}

	p.refThis = scope == ScopeThis
	defer func() { p.refThis = false }()

	if p.chainedAssign(nameTok.Value) {
		p.forceGlobal = false

		return
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
		case "entityload", "entityloadbypk":
			p.sc.NextSkipComments()
			p.parseEntityNewRef(nameTok.Value, scopeTok.Line)
		case "this":
			p.sc.NextSkipComments()

			// Only treat `scope.x = this` (bare) as a self-ref.
			// `scope.x = this.prop = ...` is a chained assignment — the real
			// component comes from the continuation, so skip the ref here.
			if p.sc.PeekSkipComments().Kind != TokDot {
				if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
					p.addRef(&ComponentRef{
						Variable: nameTok.Value, Component: selfPath,
						URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + scopeTok.Line),
					})
				}
			}
		default:
			if !isKeyword(rhs.Value) {
				p.sc.NextSkipComments()

				var fullChain chainBuilder
				fullChain.reset(rhs.Value)

				prevIdent, lastIdent, ok := p.walkChain(&fullChain, rhs.Value, scopeTok)
				if !ok {
					return
				}

				p.assignFromChain(nameTok.Value, &fullChain, prevIdent, lastIdent, scopeTok.Line)
			}
		}
	}

	p.forceGlobal = false
}

// skipNestedFunction handles a function met inside another function's body or
// inside a group. An anonymous one is a value and only its body is scanned; a
// named one is declared, for the reason the named branch gives.
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
		// Anonymous function: function() { ... }. The '(' is already consumed.
		// parseArgList records the calls an argument default holds, and the
		// names are the closure's parameters.
		args := p.parseArgList()
		p.scanNestedFunctionBody(argNames(args))

		return
	}

	if nameTok.Kind != TokIdent {
		return
	}

	lp := p.sc.NextSkipComments()
	if lp.Kind != TokLParen {
		return
	}

	// A *named* nested function is a declaration, not a value. CFML hoists it
	// into the enclosing component's variables scope, which is what lets
	// `function run(){ setup(); function setup(){…} }` call it before the line
	// it is written on — and the tag parser has always recorded it, so the two
	// syntaxes disagreed about the same code. Without it there is no index
	// entry, no completion and nowhere for go-to-definition to land.
	//
	// The argument list goes through parseArgList rather than scanParenBody so
	// the signature reaches signature help; both record the calls an argument
	// default holds, since parseArgList routes one through skipDefault.
	args := p.parseArgList()
	funcLine := p.baseLine + tok.Line

	p.funcs = append(p.funcs, FunctionDef{
		Name:      nameTok.Value,
		URI:       uriFromString(p.fileURI),
		Line:      conv.Uint32(funcLine),
		Arguments: args,
	})

	p.skipFunctionAttrs()

	endLine := p.scanNestedFunctionBody(argNames(args))
	p.scopes = append(p.scopes, FuncScope{Name: nameTok.Value, Start: funcLine, End: p.baseLine + endLine})
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
// already did. An anonymous closure declares no scope of its own — it is a
// value, for the reason parseFunctionValue gives — so the line this returns is
// read only by the named branch, which does.
func (p *scriptParser) scanNestedFunctionBody(params []string) int {
	tok := p.sc.PeekSkipComments()
	if tok.Kind == TokSemicolon {
		p.sc.NextSkipComments()

		return tok.Line
	}

	if tok.Kind != TokLBrace {
		return tok.Line
	}

	p.sc.NextSkipComments()

	return p.scanClosureBody(tok.Line, params)
}

// argNames is the names of args, a closure's parameters.
func argNames(args []Argument) []string {
	names := make([]string, 0, len(args))
	for i := range args {
		names = append(names, args[i].Name)
	}

	return names
}

// declareClosureParams declares a closure's parameters, on the line it
// starts, in the enclosing function. Outside a function there is nowhere to
// put them, as scanClosureBody says of its locals.
func (p *scriptParser) declareClosureParams(params []string, line int) {
	if p.inFunc == "" {
		return
	}

	for _, name := range params {
		p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeArguments, Line: conv.Uint32(p.baseLine + line)})
	}
}

// scopeToClosure gives the function refs and pending calls recorded since
// refsBefore and pendingBefore the closure's lines, from and to, as their
// visibility. One a nested closure already scoped keeps its narrower range.
func (p *scriptParser) scopeToClosure(refsBefore, pendingBefore, from, to int) {
	refs := p.funcRefs[p.inFunc]
	for i := refsBefore; i < len(refs); i++ {
		if refs[i].VisibleTo == 0 {
			refs[i].VisibleFrom, refs[i].VisibleTo = conv.Uint32(from), conv.Uint32(to)
		}
	}

	for i := pendingBefore; i < len(p.pendingCalls); i++ {
		if c := &p.pendingCalls[i]; c.visibleTo == 0 && !c.global {
			c.visibleFrom, c.visibleTo = conv.Uint32(from), conv.Uint32(to)
		}
	}
}

// scanClosureBody consumes the body of a closure, an anonymous or arrow
// function, whose `{` has been consumed, and returns the line of its `}`.
//
// Inside a function it is read as statements, the way the function's own body
// is: `it( "x", () => { var t = prepareMock( … ); t.go(); } )` declares `t`
// and records what it holds. The body used to be scanned for calls alone, so
// every `var` and every assignment in a closure declared nothing and typed
// nothing — which is most of a TestBox spec, where each test is a closure,
// and go-to-definition, completion and `unresolved` all read `t` as unknown.
// What a closure declares is attributed to the enclosing function, as its
// calls always were: that is the scope the parse has, and a local of one
// closure is visible to the function's other closures only by name.
//
// Outside a function the old call-only scan is kept, because there is no
// function scope to put a closure's locals in, and at component level a
// `var` would land among the component's variables.
func (p *scriptParser) scanClosureBody(line int, params []string) int {
	statements := p.inFunc != "" && p.argNesting < maxArgNesting
	prevInClosure, prevLocals := p.inClosure, p.localVarSet
	p.inClosure = statements || p.inClosure

	// What the closure declares is its own: its `var t` must not make a
	// later unscoped `t = …` in the enclosing function a local.
	if statements {
		p.localVarSet = maps.Clone(prevLocals)
		if p.localVarSet == nil {
			p.localVarSet = make(map[string]bool)
		}
	}

	refsBefore, pendingBefore := len(p.funcRefs[p.inFunc]), len(p.pendingCalls)

	// The parameters are declared where the closure starts: `( table ) => {`
	// and `function( ctx ) {` bind names the body reads.
	if statements {
		for _, name := range params {
			p.localVarSet[strings.ToLower(name)] = true
		}

		p.declareClosureParams(params, line)
	}

	p.argNesting++

	defer func() {
		p.inClosure, p.localVarSet = prevInClosure, prevLocals
		p.argNesting--
	}()

	depth := 1
	last := line

	defer func() {
		if statements {
			p.scopeToClosure(refsBefore, pendingBefore, p.baseLine+line, p.baseLine+last)
		}
	}()

	for depth > 0 {
		t := p.sc.NextSkipComments()
		last = t.Line

		afterLT := p.afterLT
		p.afterLT = t.Kind == TokLT

		switch t.Kind {
		case TokEOF:
			return last
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
		case TokIdent:
			if statements {
				p.handleBodyToken(t, depth, afterLT)
			} else {
				p.scanNestedCall(t)
			}
		case TokString, TokRBracket:
			p.handleLiteralToken(t)
		default:
			// Any other token is passed over.
		}
	}

	return last
}

func (p *scriptParser) checkAssignRef(tok Token) {
	if isKeyword(tok.Value) && !operatorWordIsName(tok, p.sc.PeekSkipComments()) {
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
		Line: conv.Uint32(p.baseLine + tok.Line),
	})

	// If inside a function and variable is NOT declared local, route to componentRefs
	if p.inFunc != "" && !p.isVarDeclaredLocal(tok.Value) {
		p.forceGlobal = true
	}

	if p.chainedAssign(tok.Value) {
		p.forceGlobal = false

		return
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
	case "entityload", "entityloadbypk":
		p.sc.NextSkipComments()
		p.parseEntityNewRef(tok.Value, tok.Line)
	default:
		// Check if RHS is a function call: funcName( or someVar.method(
		if !isKeyword(rhs.Value) {
			p.sc.NextSkipComments() // consume first ident

			var fullChain chainBuilder
			fullChain.reset(rhs.Value)

			prevIdent, lastIdent, ok := p.walkChain(&fullChain, rhs.Value, tok)
			if !ok {
				return
			}

			p.assignFromChain(tok.Value, &fullChain, prevIdent, lastIdent, tok.Line)
		}
	}

	p.forceGlobal = false
}

// chainedAssign handles a right-hand side that is itself an assignment:
// `outer = b = rhs`, `outer = this.b = rhs` or `outer = variables.b = rhs`.
// TestBox keeps `$assert` in both scopes that way. The inner assignment is
// parsed as the statement it is, and every ref and pending call it makes is
// made again for outer under the state the outer assignment set up, so
// `var a = b = new X()` makes a local a and a variables-scope b. Reports
// whether the right-hand side was one; the scanner has not moved if not.
func (p *scriptParser) chainedAssign(outer string) bool {
	// Decided from the peek and the bytes after it, which leave the peek
	// cached for the caller: saving and restoring on every assignment was
	// 4.5% of a script parse, because a restore drops it.
	peek := p.sc.PeekSkipComments()
	if peek.Kind != TokIdent {
		return false
	}

	switch first, second := p.sc.bytesAfterPeek(); {
	case first == '=' && second != '=':
		if isKeyword(peek.Value) {
			return false
		}
	case first == '.' && (identEq(peek.Value, "this") || identEq(peek.Value, "variables")):
	default:
		return false
	}

	st := p.sc.Save()
	tok := p.sc.NextSkipComments()

	scope, ok := p.assignmentAhead(tok)
	if !ok {
		p.sc.Restore(st)

		return false
	}

	forceGlobal, refThis := p.forceGlobal, p.refThis
	nGlobal, nLocal, nPending := len(p.componentRefs), len(p.funcRefs[p.inFunc]), len(p.pendingCalls)

	switch {
	case scope == ScopeLocal:
		p.checkAssignRef(tok)
	case p.inFunc != "":
		p.parseBodyScopedVar(tok, scope)
	default:
		p.parseScopedVar(tok, scope)
	}

	made := refsMadeFor(p.componentRefs, nGlobal, p.funcRefs[p.inFunc], nLocal, outer)
	p.forceGlobal, p.refThis = forceGlobal, refThis

	for i := range made {
		p.addRef(&made[i])
	}

	p.pendingCalls = appendPendingFor(p.pendingCalls, nPending, outer, refThis)

	return true
}

// assignmentAhead reports whether tok, just read, starts an assignment:
// `tok =`, or `this.name =` / `variables.name =`, and not `==`, which is two
// `=` tokens. An unscoped target is reported as ScopeLocal, meaning "no
// scope written"; the scanner does not move.
func (p *scriptParser) assignmentAhead(tok Token) (Scope, bool) {
	st := p.sc.Save()
	defer p.sc.Restore(st)

	scope := ScopeLocal

	switch {
	case identEq(tok.Value, "this"):
		scope = ScopeThis
	case identEq(tok.Value, "variables"):
		scope = ScopeVariables
	}

	if scope != ScopeLocal {
		if p.sc.NextSkipComments().Kind != TokDot || p.sc.NextSkipComments().Kind != TokIdent {
			return 0, false
		}
	}

	if p.sc.NextSkipComments().Kind != TokEquals || p.sc.PeekSkipComments().Kind == TokEquals {
		return 0, false
	}

	return scope, true
}

// addPendingCall records `varName = …prevIdent.lastIdent(…)` for
// resolvePendingCalls, which types varName once every function's return type
// is known, carrying on along whatever the chain calls after lastIdent.
func (p *scriptParser) addPendingCall(varName, prevIdent, lastIdent, chain string, line int) {
	// A receiver ending in an index is an element of prevIdent, not
	// prevIdent: `scopes[ k ].get()` says nothing about what scopes holds.
	recv := receiverOf(chain)
	if strings.HasSuffix(recv, "[]") {
		prevIdent = ""
	}

	p.pendingCalls = append(p.pendingCalls, pendingCall{
		varName:    varName,
		funcName:   lastIdent,
		expression: callExpressionAt(p.sc, chain),
		baseVar:    prevIdent,
		line:       conv.Uint32(p.baseLine + line),
		funcKey:    p.inFunc,
		refThis:    p.refThis,
		global:     p.forceGlobal,
		baseScope:  ReceiverRefScope(recv),
		baseLocal:  readThroughLocal(recv) || recv == prevIdent && p.inFunc != "" && p.isVarDeclaredLocal(prevIdent),
		rebinds:    rebinds(varName, recv),
	})
	p.pendingCalls[len(p.pendingCalls)-1].rest = p.continueChainCalls(receiverOf(chain), lastIdent, line)
}

// scopedChainCall records `scope.name.a.b(…)`, a call on a scoped variable
// that is a statement rather than an assignment.
// scopedCall records the call a `scope.name` begins when it is not an
// assignment: `scope.name(…)` directly on the scope, or a chain on a value it
// holds, `scope.name.method(…)`. The scanner must sit just past the name.
func (p *scriptParser) scopedCall(scopeTok, nameTok Token) {
	switch p.sc.PeekSkipComments().Kind {
	case TokLParen:
		p.recordScopedMemberCall(scopeTok, nameTok)
	case TokDot:
		p.scopedChainCall(scopeTok, nameTok)
	default:
	}
}

func (p *scriptParser) scopedChainCall(scopeTok, nameTok Token) {
	var fullChain chainBuilder
	fullChain.reset(scopeTok.Value)
	fullChain.writeDot()
	fullChain.writeString(nameTok.Value)

	for {
		switch p.sc.PeekSkipComments().Kind {
		case TokLBracket:
			// See walkChain for why.
			if !p.skipBracketIndex() {
				return
			}

			fullChain.writeString("[]")

			continue
		case TokDot:
			p.sc.NextSkipComments()

			next := p.sc.PeekSkipComments()
			if next.Kind == TokIdent {
				p.sc.NextSkipComments()

				fullChain.writeDot()
				fullChain.writeString(next.Value)

				continue
			}
		default:
		}

		break
	}

	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.recordChainFromScope(fullChain.String(), scopeTok.Line)
	}
}

// assignFromChain types varName from the chain on the right of its `=`: a
// call's resolved component, else a pending call for resolvePendingCalls; a
// whole argument value or configured expression when it is not a call.
func (p *scriptParser) assignFromChain(varName string, c *chainBuilder, prevIdent, lastIdent string, line int) {
	if p.sc.PeekSkipComments().Kind != TokLParen {
		p.assignNonCall(varName, c.String(), line)

		return
	}

	p.recordCallFromChain(c.String(), line)

	if comp := p.tryResolveCall(c.String()); comp != "" {
		rest := p.recordChainContinuation(receiverOf(c.String()), lastIdent, comp, line)
		p.addRef(&ComponentRef{
			Variable: varName, Component: comp,
			ChainBase: prevIdent, ChainMethod: lastIdent, ChainRest: rest,
			URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
			Rebinds: rebinds(varName, receiverOf(c.String())),
		})

		return
	}

	p.addPendingCall(varName, prevIdent, lastIdent, c.String(), line)
}

// argumentComponent is the declared type of a whole arguments.name value.
// Copying it into variables scope preserves the constructor's dependency;
// typing the parameter itself at component level instead leaked to siblings.
func (p *scriptParser) argumentComponent(chain string) string {
	return wholeArgumentComponent(chain, p.inFunc, p.funcs)
}

// assignNonCall handles a whole argument value or a configured non-call RHS.
func (p *scriptParser) assignNonCall(varName, chain string, line int) {
	comp := ""
	if p.atStatementEnd() {
		comp = p.argumentComponent(chain)
	}

	if comp == "" && len(p.resolvers) > 0 {
		comp = p.resolveCall(chain)
	}

	if comp != "" {
		p.addRef(&ComponentRef{
			Variable: varName, Component: comp,
			URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
		})
	}
}

// walkChain walks the `.name` hops after the identifier first, already
// consumed and already in c, writing each into c. A bracket index is skipped
// and marked "[]": a dynamic key cannot be resolved statically, and the marker
// can never match a real resolver or ComponentRef, so resolution fails rather
// than being misattributed to whatever the bare base identifier resolves to
// elsewhere. A `::` makes what was walked a component, not a receiver: the
// static call is recorded against staticTok (see parseStaticCall) and the
// walk reports the statement consumed, as does a bracket index that does not
// close. Otherwise it reports the last two identifiers walked.
func (p *scriptParser) walkChain(c *chainBuilder, first string, staticTok Token) (prevIdent, lastIdent string, ok bool) {
	lastIdent = first

	for {
		switch p.sc.PeekSkipComments().Kind {
		case TokLBracket:
			if !p.skipBracketIndex() {
				return prevIdent, lastIdent, false
			}

			c.writeString("[]")
		case TokDoubleColon:
			p.recordStaticCall(c.String(), staticTok)

			return prevIdent, lastIdent, false
		case TokDot:
			p.sc.NextSkipComments() // consume .

			next := p.sc.PeekSkipComments()
			if next.Kind != TokIdent {
				return prevIdent, lastIdent, true
			}

			p.sc.NextSkipComments()

			prevIdent = lastIdent
			lastIdent = next.Value

			c.writeDot()
			c.writeString(next.Value)
		default:
			return prevIdent, lastIdent, true
		}
	}
}

// checkMemberSet records `a.m = …`, a value stored on a member of a variable,
// when the scanner is at the `=`. fw1's tests give a bean a method at run
// time — `a.getVariables = getVariables;` — and then call it (see
// ParseResult.AssignsMember). `a.m == x` is a comparison, and
// `a[ k ].m = …` is on an element, not on a.
//
// It travels as a pendingCall marked memberSet, which is already keyed by
// function, rekeyed and merged per region, and resolvePendingCalls files it
// in ParseResult.memberSets. A slice of its own put scriptParser in a larger
// size class, and a ref for `a.m` reached every other file through the index.
func (p *scriptParser) checkMemberSet(identChain []string, line int) {
	if len(identChain) < 2 || strings.HasSuffix(identChain[len(identChain)-2], "[]") ||
		p.sc.PeekSkipComments().Kind != TokEquals {
		return
	}

	if first, _ := p.sc.bytesAfterPeek(); first == '=' {
		return
	}

	p.pendingCalls = append(p.pendingCalls, pendingCall{
		varName:   strings.Join(identChain[:len(identChain)-1], "."),
		funcName:  identChain[len(identChain)-1],
		line:      conv.Uint32(p.baseLine + line),
		funcKey:   p.inFunc,
		memberSet: true,
	})
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
		switch p.sc.PeekSkipComments().Kind {
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
		p.checkMemberSet(identChain, tok.Line)

		return
	}

	// The last identifier is the method; everything before it is the receiver variable.
	funcName := identChain[len(identChain)-1]
	varName := strings.Join(identChain[:len(identChain)-1], ".")

	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	comp, hop, ok := p.skipParensResolving(strings.Join(identChain, "."))
	if !ok {
		return
	}

	call := CallSite{
		FuncName: funcName,
		Variable: varName,
		Line:     conv.Uint32(p.baseLine + tok.Line),
		Caller:   caller,
	}

	// A call made directly on a scope, reached here from an argument list —
	// `f( server.getTestService() )` — is the call the statement
	// `server.getTestService()` is.
	call.onScope(varName)

	p.addCall(&call)

	// Continue walking further .method() hops chained off this call's return
	// value. Each hop's CallSite keeps Variable pointing at the original
	// receiver and accumulates the intermediate method names in Chain, so
	// CanResolveCall can walk the receiver's type through each call.
	p.recordChainContinuation(varName, hop, comp, tok.Line)
}

func (p *scriptParser) parseNewRef(varName string, line int) {
	component := p.readNewComponent()

	var hops []string

	// Consume constructor args and handle any chained .method() calls. The
	// args are consumed even when nothing resolved (an unmapped `new java:…`
	// with no javaStubsPath, say), or the scanner would be left sitting on
	// the "(" and the rest of the statement would be read as its own.
	if p.sc.PeekSkipComments().Kind == TokLParen {
		p.skipParens()

		if component != "" {
			hops = p.scanChainedCalls(component, line)
		}
	}

	if component != "" {
		p.addRef(&ComponentRef{
			Variable: varName, Component: component, ChainRest: hops,
			URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
		})
	}
}

func (p *scriptParser) parseCreateObjectRef(varName string, line int) {
	comp := p.readCreateObjectComponent()

	// Consume closing ) and handle any chained .method() calls
	if p.sc.PeekSkipComments().Kind == TokRParen {
		p.sc.NextSkipComments()
	}

	if comp != "" {
		hops := p.scanChainedCalls(comp, line)
		p.addRef(&ComponentRef{
			Variable: varName, Component: comp, ChainRest: hops,
			URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
		})
	}
}

// scanChainedCalls records resolved CallSites for any .method() chains following
// a constructor or factory call whose component is already known, and returns
// the method names in order, for an assignment to type its variable by.
//
// Each hop carries the hops before it in Chain. It used to give every one the
// constructed component and nothing else, so in
// createObject("java", "FirebaseOptions").builder().setCredentials(c) the
// setCredentials call was checked against FirebaseOptions rather than against
// what builder() returns.
func (p *scriptParser) scanChainedCalls(component string, line int) []string {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	// hops is what the caller's ref walks; chain is the same hops as the
	// CallSites carry them, each call with its arguments.
	var hops, chain []string

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments() // consume .

		methTok := p.sc.PeekSkipComments()
		if methTok.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments()

		if p.sc.PeekSkipComments().Kind != TokLParen {
			hops = append(hops, PropertyHop(methTok.Value))
			chain = append(chain, PropertyHop(methTok.Value))

			continue
		}

		p.addCall(&CallSite{
			FuncName:  methTok.Value,
			Component: component,
			Chain:     slices.Clone(chain),
			Line:      conv.Uint32(p.baseLine + line),
			Caller:    caller,
			Resolved:  true,
		})

		hops = append(hops, methTok.Value)
		chain = append(chain, callHopAt(p.sc, methTok.Value))

		p.skipParens()
	}

	return hops
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
		p.addRef(&ComponentRef{
			Variable: varName, Component: comp,
			URI: uriFromString(p.fileURI), Line: conv.Uint32(p.baseLine + line),
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
		Line: conv.Uint32(p.baseLine + tok.Line),
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
		Line: conv.Uint32(p.baseLine + tok.Line),
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
		Line: conv.Uint32(p.baseLine + tok.Line),
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

	switch p.sc.PeekSkipComments().Kind {
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

	switch p.sc.PeekSkipComments().Kind {
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
		switch tok := p.sc.NextSkipComments(); tok.Kind {
		case TokEOF:
			return false
		case open:
			depth++
		case closing:
			depth--
		default:
			// Any other token is passed over.
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
		switch p.sc.PeekSkipComments().Kind {
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

		switch p.sc.PeekSkipComments().Kind {
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
	if comp := p.tryConfiguredResolvers(callExpr); comp != "" {
		return comp
	}

	// No resolver typed it; a call whose value no component describes is
	// dynamic (dynamicCall). Its arguments are consumed as a match's are.
	if dynamicCall(callExpr+"()") == "" || p.sc.PeekSkipComments().Kind != TokLParen {
		return ""
	}

	p.sc.NextSkipComments() // consume (

	// createMock( "models.User" ) and createEmptyMock( className = … ) name
	// the class; the argument list is read to its end either way.
	class := ""
	if name := callExpr[strings.LastIndexByte(callExpr, '.')+1:]; strings.EqualFold(name, "createMock") || strings.EqualFold(name, "createEmptyMock") {
		class = p.peekMockClass()
	}

	p.skipParenBody()

	if class != "" {
		return mockOf(class)
	}

	return "$any"
}

// peekMockClass is the class the argument list the scanner is inside names,
// without moving it: a leading string, or className = "…".
func (p *scriptParser) peekMockClass() string {
	saved := p.sc.Save()
	defer p.sc.Restore(saved)

	tok := p.sc.NextSkipComments()
	if tok.Kind == TokString {
		return unquote(tok.Value)
	}

	for depth := 0; tok.Kind != TokEOF; tok = p.sc.NextSkipComments() {
		switch tok.Kind {
		case TokLParen:
			depth++
		case TokRParen:
			if depth == 0 {
				return ""
			}

			depth--
		case TokIdent:
			if depth == 0 && identEq(tok.Value, "className") {
				if eq := p.sc.NextSkipComments(); eq.Kind == TokEquals || eq.Kind == TokColon {
					if v := p.sc.NextSkipComments(); v.Kind == TokString {
						return unquote(v.Value)
					}
				}
			}
		default:
		}
	}

	return ""
}

// tryConfiguredResolvers is tryResolveCall against the componentResolvers.
func (p *scriptParser) tryConfiguredResolvers(callExpr string) string {
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

	// A single named argument, getService(service="company"), is read as the
	// positional one: falling through to the no-arg match below instead
	// offers the resolvers getService(), which only a catch-all can answer —
	// with a component that ignores which service was named.
	if arg.Kind == TokIdent {
		named := p.sc.Save()
		p.sc.NextSkipComments() // name

		if kind := p.sc.PeekSkipComments().Kind; kind == TokEquals || kind == TokColon {
			p.sc.NextSkipComments() // = or :

			if p.sc.PeekSkipComments().Kind == TokString {
				arg = p.sc.PeekSkipComments()
			} else {
				p.sc.Restore(named)
			}
		} else {
			p.sc.Restore(named)
		}
	}

	if arg.Kind == TokString {
		p.sc.NextSkipComments()

		if kind := p.sc.PeekSkipComments().Kind; kind != TokComma && kind != TokRParen {
			p.sc.Restore(saved)

			return ""
		}

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
				p.resolverSet.noteSoft(r, comp)

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
	firstArg, _, ok = p.scanParenArgs()

	return firstArg, ok
}

// scanParenArgs is scanParenBody that also reports the leading run of
// arguments that are each a string literal and nothing else, stopping at the
// first that is not — the arguments tryResolveCall reads for an assignment.
// A resolver can need more than one: createObject("java", "java.io.File")
// names its component in the second, so offering it only the first left
// createObject("java", "x").m() unresolved as a bare call to m.
func (p *scriptParser) scanParenArgs() (firstArg string, positional []string, ok bool) {
	depth := 1
	// argStart is true while the scan sits at the start of a top-level
	// argument; pending holds a string that began one, until what follows it
	// shows whether it was the whole argument.
	argStart, collecting := true, true
	pending, havePending := "", false

	var arrow arrowParams

	for depth > 0 {
		tok := p.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return "", nil, false
		}

		// An arrow function's block body: `=>` is `=` then `>`.
		if tok.Kind == TokGT && arrow.prev == TokEquals && p.sc.PeekSkipComments().Kind == TokLBrace {
			p.sc.NextSkipComments() // consume {
			p.scanClosureBody(tok.Line, arrow.params())

			arrow = arrowParams{prev: TokRBrace}
			argStart = false

			continue
		}

		// An expression-bodied arrow, `( v ) => v.isActive()`: its body is
		// scanned here as it always was, and only its parameters need
		// declaring.
		if tok.Kind == TokGT && arrow.prev == TokEquals {
			p.declareClosureParams(arrow.params(), tok.Line)
		}

		arrow.see(tok, depth)

		if tok.Kind == TokString && firstArg == "" && depth == 1 {
			firstArg = unquote(tok.Value)
		}

		if depth == 1 && collecting {
			switch {
			case havePending && (tok.Kind == TokComma || tok.Kind == TokRParen):
				positional = append(positional, pending)
				havePending = false
			case havePending:
				collecting, havePending = false, false
			case argStart && tok.Kind == TokString:
				pending, havePending = unquote(tok.Value), true
			case argStart && tok.Kind != TokRParen:
				collecting = false
			}
		}

		argStart = depth == 1 && tok.Kind == TokComma

		switch tok.Kind {
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString, TokRBracket:
			p.handleLiteralToken(tok)
		default:
			// Any other token is passed over.
		}
	}

	return firstArg, positional, true
}

// arrowParams follows an argument list's tokens closely enough to name an
// arrow function's parameters once its `=>` arrives, since by then they have
// been scanned: the names in the last (...) group, one per slot (the last
// identifier before each `,` or `)`, so `(any a)` names `a`), or the single
// identifier before `=>` in `x => {…}`.
type arrowParams struct {
	slot       string
	group      []string
	last       []string
	lastIdent  string
	groupDepth int
	prev       TokenKind
	prevprev   TokenKind
}

func (a *arrowParams) see(tok Token, depth int) {
	switch tok.Kind {
	case TokLParen:
		// depth has not counted this `(` yet.
		a.groupDepth, a.group, a.slot = depth+1, nil, ""
	case TokIdent:
		if depth == a.groupDepth {
			a.slot = tok.Value
		}

		a.lastIdent = tok.Value
	case TokComma:
		if depth == a.groupDepth && a.slot != "" {
			a.group, a.slot = append(a.group, a.slot), ""
		}
	case TokRParen:
		if depth == a.groupDepth {
			if a.slot != "" {
				a.group = append(a.group, a.slot)
			}

			a.last, a.group, a.slot, a.groupDepth = a.group, nil, "", 0
		}
	default:
	}

	a.prevprev, a.prev = a.prev, tok.Kind
}

// params is the parameters of the arrow whose `=` was the last token seen.
func (a *arrowParams) params() []string {
	switch a.prevprev {
	case TokRParen:
		return a.last
	case TokIdent:
		return []string{a.lastIdent}
	default:
		return nil
	}
}

// resolverCallExpr builds the expression a first hop is offered to the
// resolvers as: every leading string argument when there are any, as
// tryResolveCall builds it, else the first string argument found anywhere —
// the named getService(service="x") — else name().
func resolverCallExpr(name, firstArg string, positional []string) string {
	switch {
	case len(positional) > 0:
		return name + `("` + strings.Join(positional, `", "`) + `")`
	case firstArg != "":
		return name + `("` + firstArg + `")`
	default:
		return name + "()"
	}
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

	// A named function met inside a group is a declaration, and CFML hoists it
	// into the enclosing component's variables scope however deeply it is
	// nested — `it( function(){ function helper(){…} })` declares `helper`.
	// Reaching skipNestedFunction from here is what makes that true in every
	// group, since this is the one place all six token loops dispatch an
	// identifier through.
	if identEq(tok.Value, "function") {
		p.skipNestedFunction(tok, 0)

		return
	}

	// Any other keyword is handled by what follows it rather than by being
	// read as a receiver — bar a scope, which is one: `f( local.g() )` left
	// `.g()` to the next loop, which recorded a bare call to g.
	if _, _, scope := scopeReceiver(tok.Value); !scope && isKeyword(tok.Value) && !operatorWordIsName(tok, p.sc.PeekSkipComments()) {
		return
	}

	switch p.sc.PeekSkipComments().Kind {
	case TokLParen:
		p.recordBareCallAndChain(tok)
	case TokDot, TokLBracket:
		// A dotted or bracket-indexed receiver. checkBareCall walks the chain
		// and records only when it ends at a '(' — a bare `a.b` property read
		// is consumed and dropped, which is the same thing the old scan did to
		// it.
		p.checkBareCall(tok)
	default:
		// Anything else after the name: it is not a call.
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
	switch p.sc.PeekSkipComments().Kind {
	case TokLBrace, TokLBracket:
	default:
		return
	}

	p.sc.NextSkipComments() // consume { or [

	depth := 1

	for depth > 0 {
		tok := p.sc.NextSkipComments()

		switch tok.Kind {
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
		default:
			// Any other token is passed over.
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
		switch p.sc.PeekSkipComments().Kind {
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

		switch p.sc.PeekSkipComments().Kind {
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
		switch tok := p.sc.NextSkipComments(); tok.Kind {
		case TokEOF, TokSemicolon, TokLBrace, TokLT, TokGT:
			return false
		case TokHash:
			return true
		default:
			// Any other token is part of the span.
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

		switch tok.Kind {
		case TokLBracket:
			depth++
		case TokRBracket:
			depth--
		case TokIdent:
			p.scanNestedCall(tok)
		case TokString:
			p.handleLiteralToken(tok)
		default:
			// Any other token is passed over.
		}
	}

	return true
}

// tryExtendChain is called after tryResolveCall has failed and the scanner
// sits at the '(' that tryResolveCall restored. It skips the argument list,
// checks for a following '.method(' continuation, and tries tryResolveCall
// on the extended chain. On success the scanner is advanced past the new
// call's ')'. On failure the scanner position is unchanged.
//
// It also returns the names it read past the first call's ')', in order: the
// method it resolved on, preceded by any property names between the two
// (`a.b(x).c.d(y)` reads c and d). The caller records that call and carries on
// from it, since the scanner now sits beyond both.
func (p *scriptParser) tryExtendChain(chain string) (string, []string) {
	if len(p.resolvers) == 0 {
		return "", nil
	}

	saved, recorded := p.sc.Save(), p.mark()

	// The look ahead walks the argument list, which records what it holds;
	// rewinding the scanner without rewinding those left them recorded, and
	// the walk that follows recorded them again — every call in the
	// arguments of a top-level scoped assignment twice, wherever a
	// componentResolver was configured.
	rewind := func() {
		p.sc.Restore(saved)
		p.rewind(&recorded)
	}

	if !p.skipParens() {
		rewind()

		return "", nil
	}

	if p.sc.PeekSkipComments().Kind != TokDot {
		rewind()

		return "", nil
	}

	p.sc.NextSkipComments() // consume .

	next := p.sc.PeekSkipComments()
	if next.Kind != TokIdent {
		rewind()

		return "", nil
	}

	p.sc.NextSkipComments() // consume method name

	var extChain chainBuilder
	extChain.reset(chain)
	extChain.writeDot()
	extChain.writeString(next.Value)

	names := []string{next.Value}

	for p.sc.PeekSkipComments().Kind == TokDot {
		p.sc.NextSkipComments()

		n2 := p.sc.PeekSkipComments()
		if n2.Kind != TokIdent {
			break
		}

		p.sc.NextSkipComments()

		extChain.writeDot()
		extChain.writeString(n2.Value)

		names = append(names, n2.Value)
	}

	if p.sc.PeekSkipComments().Kind != TokLParen {
		rewind()

		return "", nil
	}

	if comp := p.tryResolveCall(extChain.String()); comp != "" {
		return comp, names
	}

	rewind()

	return "", nil
}

// parseMark is how much a scriptParser has recorded, taken before a look
// ahead so that a look ahead which is rewound can be un-recorded too.
type parseMark struct {
	calls, funcCalls, pending, vars, refs, funcRefs, funcs, scopes, links, funcLinks int
	hadFuncCalls, hadFuncRefs, hadFuncLinks                                          bool
}

func (p *scriptParser) mark() parseMark {
	fc, hadFC := p.funcCalls[p.inFunc]
	fr, hadFR := p.funcRefs[p.inFunc]
	fl, hadFL := p.funcLinks[p.inFunc]

	return parseMark{
		calls: len(p.calls), funcCalls: len(fc), pending: len(p.pendingCalls), vars: len(p.vars),
		refs: len(p.componentRefs), funcRefs: len(fr), funcs: len(p.funcs), scopes: len(p.scopes),
		links: len(p.links), funcLinks: len(fl),
		hadFuncCalls: hadFC, hadFuncRefs: hadFR, hadFuncLinks: hadFL,
	}
}

// rewind drops everything recorded since m. p.inFunc must be what it was.
func (p *scriptParser) rewind(m *parseMark) {
	p.calls = p.calls[:m.calls]
	p.pendingCalls = p.pendingCalls[:m.pending]
	p.vars = p.vars[:m.vars]
	p.componentRefs = p.componentRefs[:m.refs]
	p.funcs = p.funcs[:m.funcs]
	p.scopes = p.scopes[:m.scopes]
	p.links = p.links[:m.links]

	rewindEntry(p.funcCalls, p.inFunc, m.funcCalls, m.hadFuncCalls)
	rewindEntry(p.funcRefs, p.inFunc, m.funcRefs, m.hadFuncRefs)
	rewindEntry(p.funcLinks, p.inFunc, m.funcLinks, m.hadFuncLinks)
}

// rewindEntry cuts m[key] back to n, removing it if it was not there.
func rewindEntry[T any](m map[string][]T, key string, n int, had bool) {
	if !had {
		delete(m, key)

		return
	}

	if xs, ok := m[key]; ok {
		m[key] = xs[:n]
	}
}

// continueExtendedChain records the call tryExtendChain resolved on and every
// hop chained after it, from recv, the receiver the first call was made on.
//
// tryExtendChain reads `first(...).ext(...)` whole to match a resolver against
// it, and its callers used to take the component and stop: ext was never
// recorded, and a hop after it was left for the outer loop to rediscover as a
// bare call. In `VARIABLES.x = REQUEST.kernel.getSandBox("f").getEntityObj()
// .getSelected()`, getEntityObj went missing and getSelected was reported "no
// qualifier, not in file". The hops are walked from recv with no component
// assumed: the component tryExtendChain returns is a resolver's guess at the
// whole text, and the walk through the declared return types is the better
// answer. It returns the hops after ext, which the caller's ref walks.
//
// A property access between the two calls (`a.b(x).c.d(y)`) breaks the walk:
// the value of .c is not a return type. The call and what follows are then
// recorded against a dynamic receiver, as a literal receiver's chain is.
func (p *scriptParser) continueExtendedChain(recv, first string, ext []string, line int) []string {
	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	if len(ext) == 0 {
		return nil
	}

	method := ext[len(ext)-1]

	if len(ext) > 1 {
		p.addCall(&CallSite{
			FuncName: method, Variable: recv, Component: "$any", Resolved: true,
			Line: conv.Uint32(p.baseLine + line), Caller: caller,
		})
		p.recordDynamicChain(recv, line, caller)

		return []string{method, "?"} // the rest is dynamic: see dynamicIfTyped
	}

	p.addCall(&CallSite{
		FuncName: method, Variable: recv, Chain: []string{first},
		Line: conv.Uint32(p.baseLine + line), Caller: caller,
	})

	return p.recordChainContinuationFrom(recv, []string{first}, method, "", line)
}

// receiverOf is the receiver of the last call in a dotted chain: "a.b" for
// "a.b.c", "" for a bare "c". A chain continued from an assignment is walked
// from it, not from the last name before the call — which in
// "x = REQUEST.kernel.a().b()" was "kernel".
func receiverOf(chain string) string {
	if recv, _, ok := strings.CutLast(chain, "."); ok {
		return recv
	}

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

// operatorWordIsName reports whether tok, a word operator, is a variable's
// name: CFML lets `mod` and the comparison words name a variable, and
// cfwheels' specs hold their module in one, `mod.generate( … )`. Read as the
// operator, the receiver was dropped and the call recorded as a bare one. A
// dot after the word is what decides it: an operand never starts with one
// but a number, `x mod .5`, and a chain needs a name after the dot.
func operatorWordIsName(tok, next Token) bool {
	if next.Kind != TokDot {
		return false
	}

	var buf foldScratch
	switch string(buf.lowerFold(tok.Value)) {
	case "and", "or", "not", "eq", "neq", "lt", "gt", "lte", "gte", "mod":
		return true
	}

	return false
}

// operatorWordIsNameStr is operatorWordIsName for text: s begins with the
// word name, and a dot follows it directly.
func operatorWordIsNameStr(s, name string) bool {
	if len(s) <= len(name) || s[len(name)] != '.' {
		return false
	}

	return operatorWordIsName(Token{Kind: TokIdent, Value: name}, Token{Kind: TokDot})
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
	for comment != "" {
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

// Unquoted attribute literals may be dotted component names. Keep the complete
// path rather than interpreting its first segment as the property's type.
func propertyAttributeValue(tokens []Token, start int) string {
	if tokens[start].Kind != TokIdent {
		return unquote(tokens[start].Value)
	}

	var value strings.Builder
	value.WriteString(tokens[start].Value)

	for i := start + 1; i+1 < len(tokens) && tokens[i].Kind == TokDot && tokens[i+1].Kind == TokIdent; i += 2 {
		value.WriteByte('.')
		value.WriteString(tokens[i+1].Value)
	}

	return value.String()
}

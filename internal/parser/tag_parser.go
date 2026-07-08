package parser

import (
	"strings"
)

// tagParser extracts definitions from CFML tag-based source.
type tagParser struct {
	src                 string
	fileURI             string
	funcs               []FunctionDef
	vars                []VarDef
	componentRefs       []ComponentRef
	funcRefs            map[string][]ComponentRef // keyed by "start:end"
	funcLinks           map[string][]DocumentLink // keyed by "start:end"
	funcCalls           map[string][]CallSite     // keyed by "start:end"
	links               []DocumentLink            // global scope links
	calls               []CallSite                // global scope call sites
	scopes              []FuncScope
	pendingCalls        []pendingCall
	properties          []propertyDef
	extends             string
	persistent          bool
	lineIndex           []int // byte offset of each line start
	resolvers           []Resolver
	resolverSet         *ResolverSet
	extractLinks        bool // whether to extract document links
	extractCalls        bool // whether to extract all call sites
	builtinReturnLookup func(string) string
	inFunc              string          // current function scope key ("start:end"), empty if global
	localVarSet         map[string]bool // var'd/local. variable names in current function
	forceGlobal         bool            // when true, addRef routes to componentRefs regardless of inFunc
}

func newTagParser(src, fileURI string) *tagParser {
	p := &tagParser{src: src, fileURI: fileURI}
	p.buildLineIndex()

	return p
}

func (p *tagParser) buildLineIndex() {
	n := 1

	for i := 0; i < len(p.src); i++ {
		if p.src[i] == '\n' {
			n++
		}
	}

	p.lineIndex = make([]int, 1, n)
	for i := 0; i < len(p.src); i++ {
		if p.src[i] == '\n' {
			p.lineIndex = append(p.lineIndex, i+1)
		}
	}
}

// lineAt returns the 0-based line number for a byte offset using binary search.
func (p *tagParser) lineAt(offset int) int {
	lo, hi := 0, len(p.lineIndex)
	for lo < hi {
		mid := (lo + hi) / 2
		if p.lineIndex[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo - 1
}

// parse scans through tag-based CFML extracting definitions.
func (p *tagParser) parse() {
	pos := 0

	for pos < len(p.src) {
		// Find next < that could be a CF tag
		idx := strings.IndexByte(p.src[pos:], '<')
		if idx < 0 {
			break
		}

		idx += pos

		// Skip CFML comments (nested)
		if idx+4 < len(p.src) && p.src[idx:idx+5] == "<!---" {
			depth := 1
			j := idx + 5

			for j < len(p.src) && depth > 0 {
				switch {
				case j+4 < len(p.src) && p.src[j:j+5] == "<!---":
					depth++
					j += 5
				case j+3 < len(p.src) && p.src[j:j+4] == "--->":
					depth--
					j += 4
				default:
					j++
				}
			}

			pos = j

			continue
		}

		// Check for cfscript block
		if idx+10 <= len(p.src) && strings.EqualFold(p.src[idx:idx+10], "<cfscript>") {
			bodyStart := idx + 10
			closeIdx := indexCFTag(p.src[bodyStart:], "/cfscript>")

			var bodyEnd int

			if closeIdx < 0 {
				bodyEnd = len(p.src)
			} else {
				bodyEnd = bodyStart + closeIdx
			}

			baseLine := p.lineAt(bodyStart)
			sp := newScriptParser(p.src[bodyStart:bodyEnd], p.fileURI, baseLine, p.resolvers)
			sp.resolverSet = p.resolverSet
			sp.extractCalls = p.extractCalls
			sp.parse()
			p.funcs = append(p.funcs, sp.funcs...)
			p.vars = append(p.vars, sp.vars...)

			// Merge calls from cfscript sub-parser
			if p.extractCalls {
				for _, c := range sp.calls {
					p.addCall(c)
				}

				for _, calls := range sp.funcCalls {
					for _, c := range calls {
						p.addCall(c)
					}
				}
			}

			if p.inFunc != "" {
				for _, ref := range sp.componentRefs {
					p.addRef(ref)
				}
			} else {
				p.componentRefs = append(p.componentRefs, sp.componentRefs...)
			}

			if closeIdx < 0 {
				pos = len(p.src)
			} else {
				pos = bodyEnd + 11 // len("</cfscript>")
			}

			continue
		}

		// Check for CF tags we care about
		if idx+3 < len(p.src) && toLowerByte(p.src[idx+1]) == 'c' && toLowerByte(p.src[idx+2]) == 'f' {
			// Fast skip: only process tags starting with cfc/cff/cfp/cfs/cfo/cfi/cfr
			// (cfcomponent, cffunction, cfproperty, cfset, cfobject, cfinvoke, cfreturn)
			ch := toLowerByte(p.src[idx+3])
			if ch != 'c' && ch != 'f' && ch != 'p' && ch != 's' && ch != 'o' && ch != 'i' && ch != 'r' && ch != '/' {
				pos = idx + 1

				continue
			}

			tagEnd := strings.IndexByte(p.src[idx:], '>')
			if tagEnd < 0 {
				pos = idx + 1

				continue
			}

			tagEnd += idx + 1 // past the >
			tag := p.src[idx:tagEnd]
			line := p.lineAt(idx)

			// Detect </cffunction> to exit function scope
			if len(tag) > 13 && tag[1] == '/' && strings.EqualFold(tag[2:13], "cffunction") {
				p.inFunc = ""
				p.localVarSet = nil
				p.forceGlobal = false
				pos = tagEnd

				continue
			}

			switch {
			case hasCFTagPrefix(tag, "<cfcomponent"):
				p.extends = getAttr(tag, "extends")
				if isTruthy(getAttr(tag, "persistent")) {
					p.persistent = true
				}
			case hasCFTagPrefix(tag, "<cffunction"):
				p.parseCFFunction(tag, idx, tagEnd, line)
				// Find end of function to set scope
				if closeIdx := indexCFTag(p.src[tagEnd:], "/cffunction"); closeIdx >= 0 {
					endLine := p.lineAt(tagEnd + closeIdx)
					p.inFunc = funcKey(line, endLine)
					p.scopes = append(p.scopes, FuncScope{Start: line, End: endLine})
					p.localVarSet = make(map[string]bool)
					// Add function arguments to local var set
					if len(p.funcs) > 0 {
						for _, arg := range p.funcs[len(p.funcs)-1].Arguments {
							p.localVarSet[strings.ToLower(arg.Name)] = true
						}
					}
				}

				pos = tagEnd

				continue
			case hasCFTagPrefix(tag, "<cfproperty"):
				p.parseCFProperty(tag, line)
			case hasCFTagPrefix(tag, "<cfset"):
				p.parseCFSet(tag, line)
			case hasCFTagPrefix(tag, "<cfreturn"):
				p.parseCFReturn(tag)
			case hasCFTagPrefix(tag, "<cfobject"):
				p.parseCFObject(tag, line)
			case hasCFTagPrefix(tag, "<cfinvoke"):
				p.parseCFInvoke(tag, line)
			}

			pos = tagEnd
		} else {
			pos = idx + 1
		}
	}

	if p.extractLinks {
		p.extractAllLinks()
	}
}

// parseCFFunction extracts a function def from <cffunction> and its <cfargument> children.
func (p *tagParser) parseCFFunction(tag string, idx, tagEnd, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}

	// Check for JSDoc comment preceding this function tag
	docComment := p.precedingComment(idx)

	// Find arguments between this tag and </cffunction> or next <cffunction
	rest := p.src[tagEnd:]

	end := len(rest)
	if ci := indexCFTag(rest, "/cffunction"); ci >= 0 && ci < end {
		end = ci
	}

	if ci := indexCFTag(rest, "cffunction"); ci >= 0 && ci < end {
		end = ci
	}

	block := rest[:end]

	args := p.parseCFArguments(block)

	// Apply JSDoc @param {type} annotations
	if docComment != "" {
		applyJSDocParams(docComment, args)
	}

	// Create component refs for arguments with component-like types.
	// p.inFunc is not yet set at this call site — the main loop sets it after
	// parseCFFunction returns. Using p.addRef would route refs to componentRefs
	// (global), which causes cachedFuncRefs to miss them when the funcRefsMap
	// entry already exists from resolver-derived refs in the same scope.
	// Instead, compute the funcKey directly and write to funcRefs[key].
	for _, a := range args {
		if isComponentType(a.Type) {
			ref := ComponentRef{
				Variable:  a.Name,
				Component: a.Type,
				URI:       uriFromString(p.fileURI),
				Line:      uint32(line),
			}
			if end < len(rest) {
				endLine := p.lineAt(tagEnd + end)
				key := funcKey(line, endLine)

				if p.funcRefs == nil {
					p.funcRefs = make(map[string][]ComponentRef)
				}

				p.funcRefs[key] = append(p.funcRefs[key], ref)
			} else {
				p.componentRefs = append(p.componentRefs, ref)
			}
		}
	}

	p.funcs = append(p.funcs, FunctionDef{
		Name:       name,
		URI:        uriFromString(p.fileURI),
		Line:       uint32(line),
		Arguments:  args,
		ReturnType: getAttr(tag, "returntype"),
	})
}

// parseCFArguments extracts <cfargument> tags from a block of source.
// precedingComment extracts the CFML comment (<!--- ... --->) immediately before position idx.
func (p *tagParser) precedingComment(idx int) string {
	// Walk backward from idx skipping whitespace to find --->
	i := idx - 1
	for i >= 0 && (p.src[i] == ' ' || p.src[i] == '\t' || p.src[i] == '\n' || p.src[i] == '\r') {
		i--
	}

	// Check for ---> ending (i points to '>')
	if i < 3 || p.src[i-3:i+1] != "--->" {
		return ""
	}

	// Find matching <!---
	start := strings.LastIndex(p.src[:i-3], "<!---")
	if start < 0 {
		return ""
	}

	return p.src[start+5 : i-3]
}

func (p *tagParser) parseCFArguments(block string) []Argument {
	var args []Argument

	pos := 0

	for {
		idx := indexCFTag(block[pos:], "cfargument")
		if idx < 0 {
			break
		}

		idx += pos

		end := strings.IndexByte(block[idx:], '>')
		if end < 0 {
			break
		}

		tag := block[idx : idx+end+1]

		name := getAttr(tag, "name")
		if name != "" {
			a := Argument{Name: name}
			a.Type = getAttr(tag, "type")
			a.Hint = getAttr(tag, "hint")
			req := getAttr(tag, "required")
			a.Required = strings.EqualFold(req, "true") || strings.EqualFold(req, "yes")
			// Promote hint to type when type is generic and hint looks like a component path
			if isComponentType(a.Hint) && !isComponentType(a.Type) {
				a.Type = a.Hint
			}

			args = append(args, a)
		}

		pos = idx + end + 1
	}

	return args
}

// parseCFSet handles <cfset var x = ...>, <cfset local.x = ...>, etc.
func (p *tagParser) parseCFSet(tag string, line int) {
	// Strip <cfset and trailing >
	inner := tag
	if i := strings.IndexByte(inner, ' '); i >= 0 {
		inner = inner[i+1:]
	}

	inner = strings.TrimSuffix(inner, ">")
	inner = strings.TrimSuffix(strings.TrimSpace(inner), "/")
	inner = strings.TrimSpace(inner)

	switch {
	case hasPrefixFold(inner, "var "):
		rest := strings.TrimSpace(inner[4:])

		name := extractIdent(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})
			if p.localVarSet != nil {
				p.localVarSet[strings.ToLower(name)] = true
			}

			p.checkSetRHS(rest, name, line)
		}
	case hasPrefixFold(inner, "local."):
		rest := inner[6:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})
			if p.localVarSet != nil {
				p.localVarSet[strings.ToLower(name)] = true
			}

			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "arguments."):
		rest := inner[10:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeArguments, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "this."):
		rest := inner[5:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeThis, Line: uint32(line)})
			p.forceGlobal = true
			p.checkSetRHSStr(rhs, name, line)
			p.forceGlobal = false
		}
	case hasPrefixFold(inner, "variables."):
		rest := inner[10:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
			p.forceGlobal = true
			p.checkSetRHSStr(rhs, name, line)
			p.forceGlobal = false
		}
	default:
		name, rhs := splitAssign(inner)
		if name != "" && !isKeyword(name) {
			if rhs == "" && p.extractCalls {
				// No assignment — check for bare call: obj.method(...)
				p.checkBareCallStr(inner, line)
			} else if rhs != "" {
				// If var was previously declared local in this function, keep it local
				isLocal := p.inFunc != "" && p.isVarDeclaredLocal(name)
				if !isLocal {
					p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
				}

				p.forceGlobal = !isLocal
				p.checkSetRHSStr(rhs, name, line)
				p.forceGlobal = false
			}
		}
	}
}

// parseCFObject handles <cfobject component="path" name="var">
func (p *tagParser) parseCFObject(tag string, line int) {
	component := getAttr(tag, "component")
	name := getAttr(tag, "name")

	if component != "" && name != "" {
		p.addRef(ComponentRef{
			Variable:  name,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFInvoke handles <cfinvoke component="path" returnvariable="var">
func (p *tagParser) parseCFInvoke(tag string, line int) {
	component := getAttr(tag, "component")
	variable := getAttr(tag, "returnvariable")

	if component != "" && variable != "" {
		p.addRef(ComponentRef{
			Variable:  variable,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFReturn handles <cfreturn expr /> to infer function return component.
func (p *tagParser) parseCFReturn(tag string) {
	if p.inFunc == "" || len(p.funcs) == 0 {
		return
	}

	f := &p.funcs[len(p.funcs)-1]
	if f.ReturnComponent != "" || f.returnVar != "" {
		return // already set from an earlier return
	}
	// Extract the expression: strip <cfreturn and trailing /> or >
	inner := tag
	if i := strings.IndexByte(inner, ' '); i >= 0 {
		inner = inner[i+1:]
	}

	inner = strings.TrimSuffix(inner, "/>")
	inner = strings.TrimSuffix(inner, ">")

	inner = strings.TrimSpace(inner)
	if inner == "" {
		return
	}
	// Check for direct new/createObject
	switch {
	case hasPrefixFold(inner, "new "):
		if comp := extractComponentPath(inner[4:]); comp != "" {
			f.ReturnComponent = comp
		}
	case hasPrefixFold(inner, "createobject("):
		if comp := extractCreateObjectArg(inner[13:]); comp != "" {
			f.ReturnComponent = comp
		}
	default:
		// return varName — store for deferred resolution
		varName := extractIdent(inner)
		if varName != "" && !strings.Contains(inner, "(") {
			f.returnVar = varName
		}
	}
}

// parseCFProperty handles <cfproperty name="x" type="y" />
func (p *tagParser) parseCFProperty(tag string, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}

	typeName := getAttr(tag, "type")
	attrs := extractAllAttrs(tag)
	p.properties = append(p.properties, propertyDef{name: name, typeName: typeName, line: uint32(line), attrs: attrs})
}

func (p *tagParser) checkSetRHS(rest, varName string, line int) {
	// Find = and check what's after it
	if _, after, ok := strings.Cut(rest, "="); ok {
		p.checkSetRHSStr(strings.TrimSpace(after), varName, line)
	}
}

func (p *tagParser) checkSetRHSStr(rhs, varName string, line int) {
	rhs = strings.TrimSpace(rhs)

	switch {
	case strings.EqualFold(rhs, "this"):
		if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: selfPath,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "new "):
		comp := extractComponentPath(rhs[4:])
		if comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "createobject("):
		comp := extractCreateObjectArg(rhs[13:])
		if comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		} else if len(p.resolvers) > 0 {
			if comp := p.resolveCall(rhs); comp != "" {
				p.addRef(ComponentRef{
					Variable: varName, Component: comp,
					URI: uriFromString(p.fileURI), Line: uint32(line),
				})
			}
		}
	case hasPrefixFold(rhs, "entitynew("):
		comp := extractEntityNewArg(rhs[10:])
		if comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "entityload("):
		comp := extractEntityNewArg(rhs[11:])
		if comp != "" {
			p.addRef(ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	default:
		// Try generic resolver match on the RHS expression
		if len(p.resolvers) > 0 {
			if comp := p.resolveCall(rhs); comp != "" {
				p.addRef(ComponentRef{
					Variable: varName, Component: comp,
					URI: uriFromString(p.fileURI), Line: uint32(line),
				})

				return
			}
			// Try bare function name for exact-match resolvers
			if funcName := extractIdent(rhs); funcName != "" {
				for i := range p.resolvers {
					r := &p.resolvers[i]
					if r.Prefix != "" && prefixEqualFold(funcName, r.Prefix) {
						if comp := matchResolverWithCache(funcName, r); comp != "" {
							p.addRef(ComponentRef{
								Variable: varName, Component: comp,
								URI: uriFromString(p.fileURI), Line: uint32(line),
							})

							return
						}
					}
				}
			}
		}
		// Detect x = someVar.method(...) or x = funcName(...) pattern
		if baseVar := extractMethodCallBase(rhs); baseVar != "" {
			// Extract method name for pendingCall
			var methodForPending string

			before, _, _ := strings.Cut(rhs, "(")
			if dot := strings.LastIndexByte(before, '.'); dot >= 0 {
				methodForPending = before[dot+1:]
			}

			if p.extractCalls {
				// Record full call: extract method name from rhs
				if dot := strings.LastIndexByte(before, '.'); dot >= 0 {
					methodName := before[dot+1:]

					varChain := before[:dot]
					if isIdentifier(methodName) && isValidVarChain(varChain) {
						caller := ""
						if p.inFunc != "" && len(p.funcs) > 0 {
							caller = p.funcs[len(p.funcs)-1].Name
						}

						comp := p.lookupComponentRef(varChain)

						p.addCall(CallSite{
							FuncName:  methodName,
							Variable:  varChain,
							Component: comp,
							Resolved:  comp != "",
							Line:      uint32(line),
							Caller:    caller,
						})
					}
				}
			}

			p.pendingCalls = append(p.pendingCalls, pendingCall{
				varName:  varName,
				funcName: methodForPending,
				baseVar:  baseVar,
				line:     uint32(line),
				funcKey:  p.inFunc,
			})
		} else if paren := strings.IndexByte(rhs, '('); paren > 0 {
			funcName := extractIdent(rhs)
			if funcName != "" && !isKeyword(funcName) {
				if p.builtinReturnLookup != nil {
					if comp := p.builtinReturnLookup(funcName); comp != "" {
						p.addRef(ComponentRef{
							Variable: varName, Component: comp,
							URI: uriFromString(p.fileURI), Line: uint32(line),
						})

						return
					}
				}

				if p.extractCalls {
					caller := ""
					if p.inFunc != "" && len(p.funcs) > 0 {
						caller = p.funcs[len(p.funcs)-1].Name
					}

					p.addCall(CallSite{
						FuncName: funcName,
						Line:     uint32(line),
						Caller:   caller,
					})
				}

				p.pendingCalls = append(p.pendingCalls, pendingCall{
					varName:  varName,
					funcName: funcName,
					line:     uint32(line),
					funcKey:  p.inFunc,
				})
			}
		}
	}
}

// checkBareCallStr detects bare obj.method(...) patterns in a <cfset> without assignment.
func (p *tagParser) checkBareCallStr(expr string, line int) {
	before, _, ok := strings.Cut(expr, "(")
	if !ok {
		return
	}

	// If there's an = before the (, it's an assignment, not a bare call
	if strings.IndexByte(before, '=') >= 0 {
		return
	}

	dot := strings.LastIndexByte(before, '.')
	if dot < 0 {
		return
	}

	varName := strings.TrimSpace(before[:dot])
	methodName := strings.TrimSpace(before[dot+1:])

	if methodName == "" || varName == "" || !isIdentifier(methodName) {
		return
	}

	if !isValidVarChain(varName) {
		return
	}

	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	comp := p.lookupComponentRef(varName)

	p.addCall(CallSite{
		FuncName:  methodName,
		Variable:  varName,
		Component: comp,
		Resolved:  comp != "",
		Line:      uint32(line),
		Caller:    caller,
	})
}

// getAttr extracts an attribute value from a tag string (case-insensitive).
func getAttr(tag, attr string) string {
	lower := strings.ToLower(tag)
	attrLower := strings.ToLower(attr)

	// Find the attribute name followed by optional whitespace and =
	idx := -1
	searchFrom := 0

	for {
		i := strings.Index(lower[searchFrom:], attrLower)
		if i < 0 {
			break
		}

		i += searchFrom

		j := i + len(attrLower)
		for j < len(tag) && isWhitespace(tag[j]) {
			j++
		}

		if j < len(tag) && tag[j] == '=' {
			idx = i

			break
		}

		searchFrom = i + 1
	}

	if idx < 0 {
		return ""
	}

	// Skip past attr name, whitespace, =, whitespace to reach the value
	valStart := idx + len(attrLower)
	for valStart < len(tag) && isWhitespace(tag[valStart]) {
		valStart++
	}

	valStart++ // skip =

	for valStart < len(tag) && isWhitespace(tag[valStart]) {
		valStart++
	}

	if valStart >= len(tag) {
		return ""
	}

	ch := tag[valStart]
	if ch == '"' || ch == '\'' {
		valStart++

		end := strings.IndexByte(tag[valStart:], ch)
		if end < 0 {
			return ""
		}

		return tag[valStart : valStart+end]
	}
	// Unquoted value — read until space or >
	end := valStart
	for end < len(tag) && tag[end] != ' ' && tag[end] != '>' && tag[end] != '\t' {
		end++
	}

	return tag[valStart:end]
}

func extractIdent(s string) string {
	i := 0
	for i < len(s) && isIdentPart(s[i]) {
		i++
	}

	if i == 0 {
		return ""
	}

	return s[:i]
}

// extractAllAttrs extracts all attribute key=value pairs from a tag string.
// Keys are lowercased.
func extractAllAttrs(tag string) map[string]string {
	attrs := make(map[string]string)
	// Skip tag name
	i := 1 // past '<'
	for i < len(tag) && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '>' {
		i++
	}

	for i < len(tag) {
		// Skip whitespace
		for i < len(tag) && isWhitespace(tag[i]) {
			i++
		}

		if i >= len(tag) || tag[i] == '>' || tag[i] == '/' {
			break
		}
		// Read attribute name
		start := i

		for i < len(tag) && tag[i] != '=' && tag[i] != ' ' && tag[i] != '>' && tag[i] != '/' {
			i++
		}

		key := strings.ToLower(tag[start:i])

		if i >= len(tag) || tag[i] != '=' {
			continue
		}

		i++ // past '='
		if i >= len(tag) {
			break
		}

		if tag[i] == '"' || tag[i] == '\'' {
			q := tag[i]
			i++
			valStart := i

			for i < len(tag) && tag[i] != q {
				i++
			}

			attrs[key] = tag[valStart:i]

			if i < len(tag) {
				i++ // past closing quote
			}
		} else {
			valStart := i

			for i < len(tag) && tag[i] != ' ' && tag[i] != '>' {
				i++
			}

			attrs[key] = tag[valStart:i]
		}
	}

	return attrs
}

func splitAssign(s string) (name, rhs string) {
	name = extractIdent(s)
	if name == "" {
		return "", ""
	}

	rest := strings.TrimSpace(s[len(name):])
	if len(rest) > 0 && rest[0] == '=' {
		return name, strings.TrimSpace(rest[1:])
	}

	return name, ""
}

func extractComponentPath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}

	if s[0] == '"' || s[0] == '\'' {
		q := s[0]

		end := strings.IndexByte(s[1:], q)
		if end < 0 {
			return ""
		}

		return s[1 : 1+end]
	}
	// Unquoted dotted path
	i := 0
	for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
		i++
	}

	return s[:i]
}

func extractCreateObjectArg(s string) string {
	// Expects: "component", "path") — we're past the opening (
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return ""
	}

	q := s[0]

	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}

	first := s[1 : 1+end]
	if !strings.EqualFold(first, "component") {
		return ""
	}

	rest := s[2+end:]

	ci := strings.IndexByte(rest, ',')
	if ci < 0 {
		return ""
	}

	rest = strings.TrimSpace(rest[ci+1:])
	if len(rest) == 0 || (rest[0] != '"' && rest[0] != '\'') {
		return ""
	}

	q2 := rest[0]

	end2 := strings.IndexByte(rest[1:], q2)
	if end2 < 0 {
		return ""
	}

	return rest[1 : 1+end2]
}

func extractEntityNewArg(s string) string {
	// Expects: "EntityName") — we're past the opening (
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return ""
	}

	q := s[0]

	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}

	return s[1 : 1+end]
}

// extractMethodCallBase returns the base variable name from a "baseVar.method(" pattern.
// For "variables.jss.getInstance(..." returns "jss". Returns "" if not a method call.
func extractMethodCallBase(rhs string) string {
	before, _, ok := strings.Cut(rhs, "(")
	if !ok {
		return ""
	}

	prefix := before

	dot := strings.LastIndexByte(prefix, '.')
	if dot < 0 {
		return ""
	}

	base := prefix[:dot]
	if lastDot := strings.LastIndexByte(base, '.'); lastDot >= 0 {
		return base[lastDot+1:]
	}

	return base
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}

	return b
}

// hasPrefixFold checks if s starts with prefix (case-insensitive, ASCII only).
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}

	return strings.EqualFold(s[:len(prefix)], prefix)
}

// addRef appends a component ref to the correct bucket (global or per-function).
// isVarDeclaredLocal returns true if the variable was declared with var or local.
// in the current function.
func (p *tagParser) isVarDeclaredLocal(name string) bool {
	if p.localVarSet == nil {
		return false
	}

	return p.localVarSet[strings.ToLower(name)]
}

func (p *tagParser) resolveCall(expr string) string {
	if p.resolverSet != nil {
		return p.resolverSet.Resolve(expr)
	}

	return ResolveFromCall(expr, p.resolvers)
}

// Refs assigned to VARIABLES. or this. scopes are always global.
func (p *tagParser) addRef(ref ComponentRef) {
	if p.inFunc == "" || p.forceGlobal {
		p.componentRefs = append(p.componentRefs, ref)
	} else {
		if p.funcRefs == nil {
			p.funcRefs = make(map[string][]ComponentRef)
		}

		p.funcRefs[p.inFunc] = append(p.funcRefs[p.inFunc], ref)
	}
}

// lookupComponentRef finds a previously-recorded ComponentRef for varName, so
// a call site like `psService.updatePerson(...)` can carry the Component that
// an earlier `psService = getService("person")` assignment already resolved
// (mirrors resolve.CanResolveCall's function-scope-then-global precedence).
// Only sees refs recorded earlier in this single forward pass — same
// limitation goto-definition has for forward references.
func (p *tagParser) lookupComponentRef(varName string) string {
	lookupVar := varName
	if dot := strings.LastIndexByte(lookupVar, '.'); dot >= 0 {
		lookupVar = lookupVar[dot+1:]
	}

	if p.inFunc != "" {
		for _, ref := range p.funcRefs[p.inFunc] {
			if strings.EqualFold(ref.Variable, lookupVar) {
				return ref.Component
			}
		}
	}

	for i := range p.componentRefs {
		if strings.EqualFold(p.componentRefs[i].Variable, lookupVar) {
			return p.componentRefs[i].Component
		}
	}

	return ""
}

func (p *tagParser) addCall(call CallSite) {
	if p.inFunc == "" {
		p.calls = append(p.calls, call)
	} else {
		if p.funcCalls == nil {
			p.funcCalls = make(map[string][]CallSite)
		}

		p.funcCalls[p.inFunc] = append(p.funcCalls[p.inFunc], call)
	}
}

// extractAllLinks scans source lines for document links, routing them to
// global links or funcLinks based on which scope the line falls in.
func (p *tagParser) extractAllLinks() {
	src := p.src
	lineNum := 0
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

		// Advance past finished scopes
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

// isWhitespace returns true if the byte is any whitespace character.
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// hasCFTagPrefix checks if tag starts with prefix (case-insensitive) followed by whitespace.
func hasCFTagPrefix(tag, prefix string) bool {
	n := len(prefix)
	if len(tag) <= n {
		return false
	}

	if !strings.EqualFold(tag[:n], prefix) {
		return false
	}

	c := tag[n]

	return isWhitespace(c)
}

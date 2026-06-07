package parser

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/uri"
)

// Logger is an optional interface for parse diagnostics.
type Logger = log.Logger

// ParseResult caches a single parse of a file. It extracts function signatures
// and component refs eagerly, but defers function body parsing until requested.
type ParseResult struct {
	URI                uri.URI
	Content            string
	Regions            []Region
	Funcs              []FunctionDef
	ComponentRefs      []ComponentRef
	Scopes             []FuncScope
	Extends            string              // dot-path of parent component (from extends attribute)
	Persistent         bool                // true if component has persistent="true" (ORM entity)
	Properties         []propertyDef       // parsed property declarations
	Links              []DocumentLink      // file path references extracted during shallow scan
	Calls              []CallSite          // function call sites (when FindCalls is set)
	log                Logger              // optional logger for timing and errors
	Resolvers          []Resolver          // optional component resolvers for RHS matching
	PropertyResolvers  []PropertyResolver  // optional property-to-component resolvers
	BeanLookup         func(string) string // optional bean name → dot-path lookup
	expressionMappings map[string]string   // runtime expression → static value substitutions
	extractLinks       bool                // whether to extract links during global scan
	findCalls          []string            // function names to scan for
	scanAllScopes      bool                // scan all lines including function bodies
	shallow            bool                // minimal parse mode

	// Lazy global var caches (protected by mu).
	mu            sync.Mutex
	globalVars    []string
	globalDone    bool
	variablesVars []string
	varsDone      bool
	thisVars      []string
	thisDone      bool

	// funcVars caches per-function variable lists keyed by "start:end".
	funcVarsMu sync.Mutex
	funcVars   map[string][]string

	// funcRefs caches per-function component refs keyed by "start:end".
	funcRefsMu   sync.Mutex
	funcRefsMap  map[string][]ComponentRef
	funcLinksMap map[string][]DocumentLink
}

// ParseOptions configures optional parse behaviour.
type ParseOptions struct {
	Logger             Logger
	Resolvers          []Resolver
	PropertyResolvers  []PropertyResolver
	BeanLookup         func(name string) string // optional: resolve bean name → dot-path
	ExpressionMappings map[string]string        // runtime expression → static value substitutions
	ExtractLinks       bool                     // extract document links during global scan
	FindCalls          []string                 // function names to find call sites for
	ScanAllScopes      bool                     // scan all lines including function bodies (for refs/deps)
	Shallow            bool                     // minimal parse: signatures only, no refs/properties/args
}

// Parse performs a full file parse: extracts function signatures, component refs,
// and function scopes. Function bodies are NOT parsed for variables until requested.
func Parse(fileURI uri.URI, content string, resolvers ...[]Resolver) *ParseResult {
	pr := &ParseResult{
		URI:      fileURI,
		Content:  content,
		funcVars: make(map[string][]string),
	}
	if len(resolvers) > 0 {
		pr.Resolvers = resolvers[0]
	}

	start := time.Now()
	pr.Regions = ClassifyRegions(content)
	pr.extractSignatures()
	pr.logDebug("parse", "uri", string(fileURI), "funcs", len(pr.Funcs), "refs", len(pr.ComponentRefs), "dur", time.Since(start))

	return pr
}

// ParseWithOptions performs a full file parse with extended options.
func ParseWithOptions(fileURI uri.URI, content string, opts ParseOptions) *ParseResult {
	pr := &ParseResult{
		URI:                fileURI,
		Content:            content,
		funcVars:           make(map[string][]string),
		log:                opts.Logger,
		Resolvers:          opts.Resolvers,
		PropertyResolvers:  opts.PropertyResolvers,
		BeanLookup:         opts.BeanLookup,
		expressionMappings: opts.ExpressionMappings,
		extractLinks:       opts.ExtractLinks,
		findCalls:          opts.FindCalls,
		scanAllScopes:      opts.ScanAllScopes,
		shallow:            opts.Shallow,
	}
	start := time.Now()
	pr.Regions = ClassifyRegions(content)
	pr.extractSignatures()
	pr.logDebug("parse", "uri", string(fileURI), "funcs", len(pr.Funcs), "refs", len(pr.ComponentRefs), "dur", time.Since(start))

	return pr
}

// extractSignatures does a shallow parse: function names/args, component refs, scopes.
func (pr *ParseResult) extractSignatures() {
	defer func() {
		if r := recover(); r != nil {
			pr.logWarn("parse panic in extractSignatures", "uri", string(pr.URI), "error", fmt.Sprint(r))
		}
	}()

	var allPendingCalls []pendingCall

	for _, r := range pr.Regions {
		if r.Kind == RegionScript {
			sp := newScriptParser(r.Text, string(pr.URI), r.StartLine, pr.Resolvers)
			sp.parse()
			pr.Funcs = append(pr.Funcs, sp.funcs...)
			pr.ComponentRefs = append(pr.ComponentRefs, sp.componentRefs...)
			pr.Scopes = append(pr.Scopes, sp.scopes...)
			pr.Properties = append(pr.Properties, sp.properties...)

			// Merge function-scoped refs from script parser
			if len(sp.funcRefs) > 0 {
				if pr.funcRefsMap == nil {
					pr.funcRefsMap = make(map[string][]ComponentRef)
				}

				for k, refs := range sp.funcRefs {
					pr.funcRefsMap[k] = append(pr.funcRefsMap[k], refs...)
				}
			}

			allPendingCalls = append(allPendingCalls, sp.pendingCalls...)

			if sp.extends != "" {
				pr.Extends = sp.extends
			}

			if sp.persistent {
				pr.Persistent = true
			}
		} else {
			tp := newTagParser(r.Text, string(pr.URI))
			tp.parse()

			for i := range tp.funcs {
				tp.funcs[i].Line += uint32(r.StartLine)
			}

			for i := range tp.componentRefs {
				tp.componentRefs[i].Line += uint32(r.StartLine)
			}

			for i := range tp.properties {
				tp.properties[i].line += uint32(r.StartLine)
			}

			pr.Funcs = append(pr.Funcs, tp.funcs...)
			pr.ComponentRefs = append(pr.ComponentRefs, tp.componentRefs...)
			pr.Properties = append(pr.Properties, tp.properties...)

			// Merge function-scoped refs from tag parser
			if len(tp.funcRefs) > 0 {
				if pr.funcRefsMap == nil {
					pr.funcRefsMap = make(map[string][]ComponentRef)
				}

				for k, refs := range tp.funcRefs {
					// Offset the key and ref lines by region start line
					if r.StartLine > 0 {
						parts := strings.SplitN(k, ":", 2)
						if len(parts) == 2 {
							start := atoi(parts[0]) + r.StartLine
							end := atoi(parts[1]) + r.StartLine
							k = funcKey(start, end)
						}

						for i := range refs {
							refs[i].Line += uint32(r.StartLine)
						}
					}

					pr.funcRefsMap[k] = append(pr.funcRefsMap[k], refs...)
				}
			}

			// Collect pending calls from tag parser (offset lines and funcKey)
			for i := range tp.pendingCalls {
				tp.pendingCalls[i].line += uint32(r.StartLine)
				if r.StartLine > 0 && tp.pendingCalls[i].funcKey != "" {
					parts := strings.SplitN(tp.pendingCalls[i].funcKey, ":", 2)
					if len(parts) == 2 {
						start := atoi(parts[0]) + r.StartLine
						end := atoi(parts[1]) + r.StartLine
						tp.pendingCalls[i].funcKey = funcKey(start, end)
					}
				}
			}

			allPendingCalls = append(allPendingCalls, tp.pendingCalls...)

			scopes := findTagFuncScopes(r.Text, r.StartLine)

			pr.Scopes = append(pr.Scopes, scopes...)
			if tp.extends != "" {
				pr.Extends = tp.extends
			}

			if tp.persistent {
				pr.Persistent = true
			}
		}
	}
	// Generate synthetic accessor functions for properties (skip if explicit function exists).
	if !pr.shallow {
		pr.applyExpressionMappings()
		pr.generatePropertyAccessors()
		pr.appendResolverRefs()
		pr.resolvePendingCalls(allPendingCalls)
	}
}

// applyExpressionMappings replaces runtime expressions in component paths with static values.
func (pr *ParseResult) applyExpressionMappings() {
	if len(pr.expressionMappings) == 0 {
		return
	}

	for i := range pr.ComponentRefs {
		pr.ComponentRefs[i].Component = pr.replaceExpressions(pr.ComponentRefs[i].Component)
	}

	for k, refs := range pr.funcRefsMap {
		for i := range refs {
			refs[i].Component = pr.replaceExpressions(refs[i].Component)
		}

		pr.funcRefsMap[k] = refs
	}
}

func (pr *ParseResult) replaceExpressions(comp string) string {
	if !strings.Contains(comp, "#") {
		return comp
	}

	for expr, value := range pr.expressionMappings {
		if strings.Contains(comp, expr) {
			comp = strings.ReplaceAll(comp, expr, value)
		}
	}

	return comp
}

// resolvePendingCalls resolves varName = funcCall(...) assignments against same-file functions.
func (pr *ParseResult) resolvePendingCalls(calls []pendingCall) {
	// Build lookup of function name → return component
	funcReturns := make(map[string]string, len(pr.Funcs))
	for i := range pr.Funcs {
		f := &pr.Funcs[i]

		comp := f.ReturnComponent
		if comp == "" && isComponentType(f.ReturnType) {
			comp = f.ReturnType
		}

		if comp != "" {
			funcReturns[strings.ToLower(f.Name)] = comp
		}
	}

	// Track which functions have pending return vars to resolve
	type returnPending struct {
		funcIdx int
		varName string
		funcKey string
	}

	var returnPendings []returnPending

	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if f.ReturnComponent == "" && f.returnVar != "" {
			scope := findFuncScope(int(f.Line), pr.Scopes)
			if scope.Start >= 0 {
				returnPendings = append(returnPendings, returnPending{
					funcIdx: i,
					varName: f.returnVar,
					funcKey: funcKey(scope.Start, scope.End),
				})
			}
		}
	}

	// Resolve ReturnComponent from return var before processing calls
	for _, rp := range returnPendings {
		if refs := pr.funcRefsMap[rp.funcKey]; refs != nil {
			for _, ref := range refs {
				if strings.EqualFold(ref.Variable, rp.varName) {
					pr.Funcs[rp.funcIdx].ReturnComponent = ref.Component

					break
				}
			}
		}
	}

	// Rebuild funcReturns with newly resolved ReturnComponents
	for i := range pr.Funcs {
		f := &pr.Funcs[i]

		comp := f.ReturnComponent
		if comp == "" && isComponentType(f.ReturnType) {
			comp = f.ReturnType
		}

		if comp != "" {
			funcReturns[strings.ToLower(f.Name)] = comp
		}
	}

	for _, c := range calls {
		// Skip if this variable already has a ref (e.g. from appendResolverRefs)
		varLower := strings.ToLower(c.varName)
		alreadyResolved := false

		if c.funcKey != "" && pr.funcRefsMap != nil {
			for _, ref := range pr.funcRefsMap[c.funcKey] {
				if strings.EqualFold(ref.Variable, varLower) {
					alreadyResolved = true

					break
				}
			}
		}

		if !alreadyResolved {
			for _, ref := range pr.ComponentRefs {
				if strings.EqualFold(ref.Variable, varLower) {
					alreadyResolved = true

					break
				}
			}
		}

		if alreadyResolved {
			continue
		}

		comp := funcReturns[strings.ToLower(c.funcName)]

		// Fallback: x = baseVar.method() — assign x same component as baseVar
		if comp == "" && c.baseVar != "" {
			baseVarLower := strings.ToLower(c.baseVar)
			for _, ref := range pr.ComponentRefs {
				if strings.EqualFold(ref.Variable, baseVarLower) {
					comp = ref.Component

					break
				}
			}

			if comp == "" && c.funcKey != "" && pr.funcRefsMap != nil {
				refs := pr.funcRefsMap[c.funcKey]
				for _, ref := range refs {
					if strings.EqualFold(ref.Variable, baseVarLower) {
						comp = ref.Component

						break
					}
				}
			}
		}

		if comp == "" {
			continue
		}

		ref := ComponentRef{
			Variable: c.varName, Component: comp,
			URI: pr.URI, Line: c.line,
		}
		if c.funcKey == "" {
			pr.ComponentRefs = append(pr.ComponentRefs, ref)
		} else {
			if pr.funcRefsMap == nil {
				pr.funcRefsMap = make(map[string][]ComponentRef)
			}

			pr.funcRefsMap[c.funcKey] = append(pr.funcRefsMap[c.funcKey], ref)
		}
	}

	// Second pass: resolve ReturnComponent for functions whose return var was just added by calls
	for _, rp := range returnPendings {
		if pr.Funcs[rp.funcIdx].ReturnComponent != "" {
			continue
		}

		if refs := pr.funcRefsMap[rp.funcKey]; refs != nil {
			for _, ref := range refs {
				if strings.EqualFold(ref.Variable, rp.varName) {
					pr.Funcs[rp.funcIdx].ReturnComponent = ref.Component

					break
				}
			}
		}
	}
}

// extractBeanName strips framework namespace prefixes from an inject value.
// Handles: "model:UserService" → "UserService", "UserDAO@model" → "UserDAO",
// "coldbox:setting:appName" → "appName", "userService" → "userService".
func extractBeanName(inject string) string {
	inject = strings.TrimSpace(inject)
	// WireBox @-style: "BeanName@namespace"
	if at := strings.IndexByte(inject, '@'); at > 0 {
		return inject[:at]
	}
	// Colon-namespaced: take the last segment after ':'
	if colon := strings.LastIndexByte(inject, ':'); colon >= 0 {
		return inject[colon+1:]
	}

	return inject
}

// normalizeBeanKey converts an inject value to the bean map key format.
// "UserDAO@model" → "userdao@model", "model:UserService" → "userservice@model",
// "userService" → "userservice".
func normalizeBeanKey(inject string) string {
	inject = strings.TrimSpace(inject)
	// Already in @-style: "BeanName@namespace"
	if at := strings.IndexByte(inject, '@'); at > 0 {
		return strings.ToLower(inject[:at]) + "@" + strings.ToLower(inject[at+1:])
	}
	// Colon-namespaced: "namespace:BeanName" → "beanname@namespace"
	if colon := strings.LastIndexByte(inject, ':'); colon >= 0 {
		ns := inject[:colon]
		name := inject[colon+1:]
		// If namespace itself has colons (e.g. "coldbox:setting"), use last segment as ns
		if innerColon := strings.LastIndexByte(ns, ':'); innerColon >= 0 {
			ns = ns[innerColon+1:]
		}

		return strings.ToLower(name) + "@" + strings.ToLower(ns)
	}

	return strings.ToLower(inject)
}

// ucFirst capitalizes the first character of a string.
func ucFirst(s string) string {
	if s == "" {
		return s
	}

	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}

	return s
}

// generatePropertyAccessors creates synthetic get/set FunctionDefs and ComponentRefs
// for properties, skipping any where an explicit or already-generated function exists.
func (pr *ParseResult) generatePropertyAccessors() {
	if len(pr.Properties) == 0 {
		return
	}
	// Build set of existing function names (explicit + previously generated)
	existing := make(map[string]bool, len(pr.Funcs)+len(pr.Properties)*2)
	for _, f := range pr.Funcs {
		existing[strings.ToLower(f.Name)] = true
	}

	u := pr.URI

	for _, prop := range pr.Properties {
		capName := ucFirst(prop.name)

		getter := "get" + strings.ToLower(prop.name)
		if !existing[getter] {
			existing[getter] = true

			pr.Funcs = append(pr.Funcs, FunctionDef{
				Name: "get" + capName, URI: u, Line: prop.line,
			})
		}

		setter := "set" + strings.ToLower(prop.name)
		if !existing[setter] {
			existing[setter] = true

			pr.Funcs = append(pr.Funcs, FunctionDef{
				Name: "set" + capName, URI: u, Line: prop.line,
				Arguments: []Argument{{Name: prop.name, Type: prop.typeName}},
			})
		}
		// Resolve component path: try property resolvers first, then type, then bean map
		comp := ""
		if len(pr.PropertyResolvers) > 0 && len(prop.attrs) > 0 {
			comp = ResolveProperty(prop.attrs, pr.PropertyResolvers)
		}

		if comp == "" && prop.typeName != "" && looksLikeCFCType(prop.typeName) {
			comp = prop.typeName
		}

		if comp == "" && pr.BeanLookup != nil {
			// Try inject attribute: full value (for namespace-qualified), then stripped name
			if inject := prop.attrs["inject"]; inject != "" {
				comp = pr.BeanLookup(normalizeBeanKey(inject))
				if comp == "" {
					comp = pr.BeanLookup(extractBeanName(inject))
				}
			}

			if comp == "" {
				comp = pr.BeanLookup(prop.name)
			}
		}

		if comp != "" {
			pr.ComponentRefs = append(pr.ComponentRefs, ComponentRef{
				Variable: prop.name, Component: comp, URI: u, Line: prop.line,
			})
		}
	}
}

// GlobalVars returns this.x and variables.x names declared outside any function.
func (pr *ParseResult) GlobalVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if !pr.globalDone {
		pr.globalVars = pr.computeGlobalVars()
		pr.globalDone = true
	}

	return pr.globalVars
}

// VariablesVars returns variables-scoped names from outside functions.
func (pr *ParseResult) VariablesVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if !pr.varsDone {
		pr.variablesVars = pr.computeScopedVars(ScopeVariables)
		pr.varsDone = true
	}

	return pr.variablesVars
}

// ThisVars returns this-scoped property names from outside functions.
func (pr *ParseResult) ThisVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if !pr.thisDone {
		pr.thisVars = pr.computeScopedVars(ScopeThis)
		pr.thisDone = true
	}

	return pr.thisVars
}

// FuncVars returns local/arguments variable names within the function at [start, end].
// Results are cached; call InvalidateFunc to force re-parse.
func (pr *ParseResult) FuncVars(funcStart, funcEnd int) []string {
	key := funcKey(funcStart, funcEnd)

	pr.funcVarsMu.Lock()
	if cached, ok := pr.funcVars[key]; ok {
		pr.funcVarsMu.Unlock()

		return cached
	}
	pr.funcVarsMu.Unlock()

	vars := pr.parseFuncBody(funcStart, funcEnd)

	pr.funcVarsMu.Lock()
	pr.funcVars[key] = vars
	pr.funcVarsMu.Unlock()

	return vars
}

// InvalidateFunc clears the cached variables for a specific function,
// forcing re-parse on next FuncVars call.
func (pr *ParseResult) InvalidateFunc(funcStart, funcEnd int) {
	key := funcKey(funcStart, funcEnd)

	pr.funcVarsMu.Lock()
	delete(pr.funcVars, key)
	pr.funcVarsMu.Unlock()
}

// parseFuncBody parses a single function body for variable declarations.
func (pr *ParseResult) parseFuncBody(funcStart, funcEnd int) (names []string) {
	defer func() {
		if r := recover(); r != nil {
			pr.logWarn("parse panic in parseFuncBody", "uri", string(pr.URI), "funcStart", funcStart, "error", fmt.Sprint(r))
		}
	}()

	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}

	body := pr.Content[start:end]

	t := time.Now()
	sp := newScriptParser(body, "", 0, nil)
	sp.parse()

	seen := make(map[string]bool)

	for _, v := range sp.vars {
		if v.Scope == ScopeLocal || v.Scope == ScopeArguments {
			if !seen[v.Name] {
				seen[v.Name] = true

				names = append(names, v.Name)
			}
		}
	}

	pr.logDebug("parseFuncBody", "uri", string(pr.URI), "funcStart", funcStart, "vars", len(names), "dur", time.Since(t))

	return names
}

// computeGlobalVars extracts global-scope variables (variables.x, this.x, plain assigns).
func (pr *ParseResult) computeGlobalVars() []string {
	vars := pr.computeScopedVars(ScopeVariables)
	vars = append(vars, pr.computeScopedVars(ScopeThis)...)

	return vars
}

// computeScopedVars extracts variables of a specific scope from outside functions
// and from the init() function body.
func (pr *ParseResult) computeScopedVars(scope Scope) []string {
	seen := make(map[string]bool)

	var names []string

	// Properties default to variables scope
	if scope == ScopeVariables {
		for _, prop := range pr.Properties {
			if !seen[prop.name] {
				seen[prop.name] = true

				names = append(names, prop.name)
			}
		}
	}

	for _, r := range pr.Regions {
		var regionVars []VarDef

		if r.Kind == RegionScript {
			sp := newGlobalScriptParser(r.Text, r.StartLine, pr.Scopes)
			sp.parse()
			regionVars = sp.vars
		} else {
			tp := newTagParser(r.Text, "")
			tp.parse()

			for i := range tp.vars {
				tp.vars[i].Line += uint32(r.StartLine)
			}

			regionVars = tp.vars
		}

		for _, v := range regionVars {
			if v.Scope != scope {
				continue
			}

			fs := findFuncScope(int(v.Line), pr.Scopes)
			if fs.Start != -1 {
				continue // inside a function
			}

			if !seen[v.Name] {
				seen[v.Name] = true

				names = append(names, v.Name)
			}
		}
	}

	// Also include vars from init() body
	initScope := pr.initFuncScope()
	if initScope.Start == -1 {
		return names
	}

	start, end := lineOffsets(pr.Content, initScope.Start, initScope.End)
	if start < 0 {
		return names
	}

	body := pr.Content[start:end]
	regionKind := RegionScript

	for _, r := range pr.Regions {
		if r.StartLine <= initScope.Start {
			regionKind = r.Kind
		}
	}

	var bodyVars []VarDef

	if regionKind == RegionScript {
		sp := newScriptParser(body, "", initScope.Start, nil)
		sp.parse()
		bodyVars = sp.vars
	} else {
		tp := newTagParser(body, "")
		tp.parse()
		bodyVars = tp.vars
	}

	for _, v := range bodyVars {
		if v.Scope == scope && !seen[v.Name] {
			seen[v.Name] = true

			names = append(names, v.Name)
		}
	}

	return names
}

// initFuncScope returns the FuncScope for the init() function, or {-1,-1} if not found.
func (pr *ParseResult) initFuncScope() FuncScope {
	for _, f := range pr.Funcs {
		if strings.EqualFold(f.Name, "init") {
			return findFuncScope(int(f.Line), pr.Scopes)
		}
	}

	return FuncScope{Start: -1, End: -1}
}

// appendResolverRefs scans content for assignments matching configured resolvers.
// Only considers global scope and init() body, same as other ref extraction.
func (pr *ParseResult) appendResolverRefs() {
	if len(pr.Resolvers) == 0 && !pr.extractLinks && len(pr.findCalls) == 0 {
		return
	}

	var prefixes []string
	if len(pr.Resolvers) > 0 {
		prefixes = make([]string, len(pr.Resolvers))
		for i := range pr.Resolvers {
			prefixes[i] = pr.Resolvers[i].Prefix
		}
	}

	content := pr.Content
	lineNum := 0
	scopeIdx := 0
	currentFunc := ""
	isInitFunc := false
	commentDepth := 0

	for len(content) > 0 {
		nl := strings.IndexByte(content, '\n')

		var line string

		if nl < 0 {
			line = content
			content = ""
		} else {
			line = content[:nl]
			content = content[nl+1:]
		}

		// Track block comments
		commentDepth += strings.Count(line, "/*") + strings.Count(line, "<!---")

		commentDepth -= strings.Count(line, "*/") + strings.Count(line, "--->")
		if commentDepth < 0 {
			commentDepth = 0
		}

		if commentDepth > 0 {
			lineNum++

			continue
		}
		// Skip single-line comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "<!---") {
			lineNum++

			continue
		}

		// Track current function scope
		if scopeIdx < len(pr.Scopes) {
			if lineNum > pr.Scopes[scopeIdx].End {
				scopeIdx++
				currentFunc = ""
				isInitFunc = false
			}

			if scopeIdx < len(pr.Scopes) && lineNum == pr.Scopes[scopeIdx].Start+1 {
				currentFunc = pr.Scopes[scopeIdx].Name
				isInitFunc = strings.EqualFold(currentFunc, "init")
			}
		}

		// Skip lines inside non-init function bodies (unless scanning all scopes)
		if !pr.scanAllScopes && scopeIdx < len(pr.Scopes) && lineNum > pr.Scopes[scopeIdx].Start {
			if lineNum < pr.Scopes[scopeIdx].End {
				if !isInitFunc {
					lineNum++

					continue
				}
			}
		}

		// Extract document links from this line
		if pr.extractLinks {
			extractLinksFromLine(line, lineNum, &pr.Links)
		}

		// Scan for function calls
		if len(pr.findCalls) > 0 {
			pr.scanLineForCalls(line, lineNum, currentFunc)
		}

		// Resolver ref extraction (only if resolvers configured)
		if len(prefixes) > 0 {
			hasPrefix := false

			for _, p := range prefixes {
				if p == "" || containsFold(line, p) {
					hasPrefix = true

					break
				}
			}

			if hasPrefix {
				eqIdx := strings.IndexByte(line, '=')
				if eqIdx >= 0 && (eqIdx+1 >= len(line) || line[eqIdx+1] != '=') {
					rhs := strings.TrimSpace(line[eqIdx+1:])
					rhs = strings.TrimSuffix(strings.TrimRight(rhs, " \t"), "/>")
					rhs = strings.TrimSuffix(rhs, ">")
					rhs = strings.TrimSuffix(rhs, ";")
					rhs = strings.TrimSpace(rhs)

					if rhs != "" {
						comp := ResolveFromCall(rhs, pr.Resolvers)
						// Multi-line call: RHS has unbalanced parens, try with closing )
						if comp == "" && strings.Count(rhs, "(") > strings.Count(rhs, ")") {
							comp = ResolveFromCall(rhs+")", pr.Resolvers)
						}

						if comp != "" {
							lhs := strings.TrimSpace(line[:eqIdx])
							varName := lhs

							if dotIdx := strings.LastIndexByte(lhs, '.'); dotIdx >= 0 {
								varName = strings.TrimSpace(lhs[dotIdx+1:])
							} else if spIdx := strings.LastIndexByte(lhs, ' '); spIdx >= 0 {
								varName = strings.TrimSpace(lhs[spIdx+1:])
							}

							if varName != "" {
								ref := ComponentRef{Variable: varName, Component: comp, URI: pr.URI, Line: uint32(lineNum)}
								// Route to funcRefsMap if inside a function body
								fs := findFuncScope(lineNum, pr.Scopes)
								if fs.Start != -1 && !strings.EqualFold(fs.Name, "init") {
									key := funcKey(fs.Start, fs.End)

									if pr.funcRefsMap == nil {
										pr.funcRefsMap = make(map[string][]ComponentRef)
									}

									pr.funcRefsMap[key] = append(pr.funcRefsMap[key], ref)
								} else {
									pr.ComponentRefs = append(pr.ComponentRefs, ref)
								}
							}
						}
					}
				}
			}
		}

		lineNum++
	}
}

// extractLinksFromLine extracts file path references from a single line.
func extractLinksFromLine(line string, lineNum int, links *[]DocumentLink) {
	lower := strings.ToLower(line)

	for _, attr := range linkAttrs {
		idx := 0

		for {
			pos := strings.Index(lower[idx:], attr)
			if pos < 0 {
				break
			}

			pos += idx + len(attr)
			for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
				pos++
			}

			if pos >= len(line) {
				break
			}

			q := line[pos]
			if q != '"' && q != '\'' {
				idx = pos

				continue
			}

			start := pos + 1

			end := strings.IndexByte(line[start:], q)
			if end < 0 {
				break
			}

			end += start
			path := line[start:end]

			if path != "" && !strings.Contains(path, "#") && !strings.Contains(path, "://") {
				*links = append(*links, DocumentLink{
					Path:  path,
					Line:  uint32(lineNum),
					Start: uint32(start),
					End:   uint32(end),
				})
			}

			idx = end + 1
		}
	}
}

// linkAttrs are the attribute names that contain file paths.
var linkAttrs = []string{"template=", "include ", "href=", "action="}

// ExtractLinks scans content for file path references (cfinclude, href, etc.).
func ExtractLinks(content string) []DocumentLink {
	var links []DocumentLink

	lineNum := 0

	for len(content) > 0 {
		nl := strings.IndexByte(content, '\n')

		var line string

		if nl < 0 {
			line = content
			content = ""
		} else {
			line = content[:nl]
			content = content[nl+1:]
		}

		lower := strings.ToLower(line)

		for _, attr := range linkAttrs {
			idx := 0

			for {
				pos := strings.Index(lower[idx:], attr)
				if pos < 0 {
					break
				}

				pos += idx + len(attr)
				// Skip whitespace and find opening quote
				for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
					pos++
				}

				if pos >= len(line) {
					break
				}

				q := line[pos]
				if q != '"' && q != '\'' {
					idx = pos

					continue
				}

				start := pos + 1

				end := strings.IndexByte(line[start:], q)
				if end < 0 {
					break
				}

				end += start
				path := line[start:end]

				if path != "" && !strings.Contains(path, "#") && !strings.Contains(path, "://") {
					links = append(links, DocumentLink{
						Path:  path,
						Line:  uint32(lineNum),
						Start: uint32(start),
						End:   uint32(end),
					})
				}

				idx = end + 1
			}
		}

		lineNum++
	}

	return links
}

// FuncComponentRefs returns cached component refs for a function scope.
func (pr *ParseResult) FuncComponentRefs(funcStart, funcEnd int) []ComponentRef {
	refs, _ := pr.cachedFuncRefs(funcStart, funcEnd)

	return refs
}

// FuncRefs returns cached component refs and document links for a function body.
func (pr *ParseResult) FuncRefs(funcStart, funcEnd int) ([]ComponentRef, []DocumentLink) {
	return pr.cachedFuncRefs(funcStart, funcEnd)
}

func (pr *ParseResult) cachedFuncRefs(funcStart, funcEnd int) ([]ComponentRef, []DocumentLink) {
	key := funcKey(funcStart, funcEnd)

	pr.funcRefsMu.Lock()
	defer pr.funcRefsMu.Unlock()

	if pr.funcRefsMap != nil {
		if cached, ok := pr.funcRefsMap[key]; ok {
			links := pr.funcLinksMap[key]

			return cached, links
		}
	}

	refs, links := pr.funcRefsUncached(funcStart, funcEnd)

	if pr.funcRefsMap == nil {
		pr.funcRefsMap = make(map[string][]ComponentRef)
	}

	if pr.funcLinksMap == nil {
		pr.funcLinksMap = make(map[string][]DocumentLink)
	}

	pr.funcRefsMap[key] = refs
	pr.funcLinksMap[key] = links

	return refs, links
}

func (pr *ParseResult) funcRefsUncached(funcStart, funcEnd int) ([]ComponentRef, []DocumentLink) {
	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return nil, nil
	}

	body := pr.Content[start:end]

	var refs []ComponentRef

	var links []DocumentLink

	lineNum := funcStart + 1

	for len(body) > 0 {
		nl := strings.IndexByte(body, '\n')

		var line string

		if nl < 0 {
			line = body
			body = ""
		} else {
			line = body[:nl]
			body = body[nl+1:]
		}

		// Extract links from this line
		extractLinksFromLine(line, lineNum, &links)

		// Scan for function calls
		if len(pr.findCalls) > 0 {
			pr.scanLineForCalls(line, lineNum, pr.callerAtLine(lineNum))
		}

		// Extract resolver refs
		if len(pr.Resolvers) > 0 {
			eqIdx := strings.IndexByte(line, '=')
			if eqIdx >= 0 && (eqIdx+1 >= len(line) || line[eqIdx+1] != '=') {
				rhs := strings.TrimSpace(line[eqIdx+1:])
				rhs = strings.TrimSuffix(strings.TrimRight(rhs, " \t"), "/>")
				rhs = strings.TrimSuffix(rhs, ">")
				rhs = strings.TrimSuffix(rhs, ";")
				rhs = strings.TrimSpace(rhs)

				if rhs != "" {
					if comp := ResolveFromCall(rhs, pr.Resolvers); comp != "" {
						lhs := strings.TrimSpace(line[:eqIdx])
						varName := lhs

						if dotIdx := strings.LastIndexByte(lhs, '.'); dotIdx >= 0 {
							varName = strings.TrimSpace(lhs[dotIdx+1:])
						} else if spIdx := strings.LastIndexByte(lhs, ' '); spIdx >= 0 {
							varName = strings.TrimSpace(lhs[spIdx+1:])
						}

						if varName != "" {
							refs = append(refs, ComponentRef{Variable: varName, Component: comp, URI: pr.URI, Line: uint32(lineNum)})
						}
					}
				}
			}
		}

		lineNum++
	}

	return refs, links
}

// FuncCalls scans a function body and returns all variable.method() calls
// with resolved components. Used for downstream dependency tracing.
func (pr *ParseResult) FuncCalls(funcStart, funcEnd int) []CallSite {
	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}

	body := pr.Content[start:end]

	var calls []CallSite

	seen := make(map[string]bool)
	lineNum := funcStart + 1

	// Get function-local component refs
	localRefs, _ := pr.FuncRefs(funcStart, funcEnd)

	// Find the caller name for this scope
	caller := ""

	for _, f := range pr.Funcs {
		if int(f.Line) == funcStart {
			caller = f.Name

			break
		}
	}

	var commentDepth int

	// Determine comment style based on region context
	isScript := false

	for _, r := range pr.Regions {
		regionEnd := r.StartLine + strings.Count(r.Text, "\n")
		if funcStart >= r.StartLine && funcStart <= regionEnd {
			isScript = r.Kind == RegionScript

			break
		}
	}

	for len(body) > 0 {
		nl := strings.IndexByte(body, '\n')

		var line string

		if nl < 0 {
			line = body
			body = ""
		} else {
			line = body[:nl]
			body = body[nl+1:]
		}

		// Track nested block comments
		if isScript {
			commentDepth += strings.Count(line, "/*")
			commentDepth -= strings.Count(line, "*/")
		} else {
			commentDepth += strings.Count(line, "<!---")
			commentDepth -= strings.Count(line, "--->")
		}

		if commentDepth < 0 {
			commentDepth = 0
		}

		if commentDepth > 0 {
			lineNum++

			continue
		}

		// Skip lines that are entirely within a comment (opened and closed on same line)
		if isScript {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "/*") {
				lineNum++

				continue
			}

			if strings.HasPrefix(trimmed, "//") {
				lineNum++

				continue
			}
		} else {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "<!---") {
				lineNum++

				continue
			}
		}

		// Find all variable.method( patterns in this line — mask quoted strings
		// to avoid matching content inside string literals (e.g. JS in cfset).
		masked := maskStrings(line)

		lower := strings.ToLower(masked)
		for i := 0; i < len(lower); i++ {
			if lower[i] != '(' {
				continue
			}
			// Walk back to find method name
			methEnd := i

			methStart := methEnd - 1
			for methStart >= 0 && (lower[methStart] >= 'a' && lower[methStart] <= 'z' || lower[methStart] >= '0' && lower[methStart] <= '9' || lower[methStart] == '_') {
				methStart--
			}

			methStart++
			if methStart == methEnd {
				continue
			}
			// Check for dot before method
			if methStart == 0 || line[methStart-1] != '.' {
				continue
			}

			methodName := line[methStart:methEnd]

			// Walk back to find variable name
			varEnd := methStart - 1

			varStart := varEnd - 1
			for varStart >= 0 && (line[varStart] >= 'a' && line[varStart] <= 'z' || line[varStart] >= 'A' && line[varStart] <= 'Z' || line[varStart] >= '0' && line[varStart] <= '9' || line[varStart] == '_' || line[varStart] == '.') {
				varStart--
			}

			varStart++
			if varStart == varEnd {
				continue
			}

			varName := line[varStart:varEnd]

			// Skip numeric literals (e.g. 1.02(...) is not a method call)
			if varName[0] >= '0' && varName[0] <= '9' {
				continue
			}

			// Strip scope prefix for resolution (VARIABLES.service -> service)
			resolveVar := varName
			if dotIdx := strings.LastIndexByte(resolveVar, '.'); dotIdx >= 0 {
				resolveVar = resolveVar[dotIdx+1:]
			}

			comp := pr.resolveVarComponentScoped(resolveVar, uint32(lineNum), localRefs)

			// Skip if unresolved but method is a known member function (native type call)
			if comp == "" && isMemberMethod(methodName) {
				continue
			}

			key := strings.ToLower(comp + "." + methodName)
			if seen[key] {
				continue
			}

			seen[key] = true

			calls = append(calls, CallSite{
				FuncName:  methodName,
				Component: comp,
				Variable:  varName,
				Line:      uint32(lineNum),
				Caller:    caller,
				Resolved:  comp != "",
				Text:      strings.TrimSpace(line),
			})
		}

		lineNum++
	}

	return calls
}

// callerAtLine returns the enclosing function name for a given line number.
func (pr *ParseResult) callerAtLine(lineNum int) string {
	for _, sc := range pr.Scopes {
		if lineNum > sc.Start && lineNum < sc.End {
			return sc.Name
		}
	}

	return ""
}

// scanLineForCalls checks a line for calls to any of pr.findCalls targets.
func (pr *ParseResult) scanLineForCalls(line string, lineNum int, caller string) {
	lower := strings.ToLower(line)
	trimmed := strings.TrimSpace(lower)
	// Skip function definition lines
	if strings.HasPrefix(trimmed, "function ") || strings.Contains(trimmed, " function ") ||
		strings.HasPrefix(trimmed, "<cffunction") {
		return
	}

	for _, target := range pr.findCalls {
		t := strings.ToLower(target)
		if idx := strings.Index(lower, "."+t+"("); idx >= 0 {
			// Extract variable name before the dot
			varEnd := idx

			varStart := varEnd - 1
			for varStart >= 0 && (line[varStart] >= 'a' && line[varStart] <= 'z' || line[varStart] >= 'A' && line[varStart] <= 'Z' || line[varStart] >= '0' && line[varStart] <= '9' || line[varStart] == '_') {
				varStart--
			}

			varStart++
			varName := line[varStart:varEnd]
			comp := pr.resolveVarComponent(varName)
			// If no variable match, try resolving call expression before the dot
			if comp == "" && varEnd > 0 && line[varEnd-1] == ')' && len(pr.Resolvers) > 0 {
				// Find matching open paren
				depth := 0

				j := varEnd - 1
				for j >= 0 {
					if line[j] == ')' {
						depth++
					} else if line[j] == '(' {
						depth--
						if depth == 0 {
							fnStart := j - 1
							for fnStart >= 0 && (line[fnStart] >= 'a' && line[fnStart] <= 'z' || line[fnStart] >= 'A' && line[fnStart] <= 'Z' || line[fnStart] >= '0' && line[fnStart] <= '9' || line[fnStart] == '_') {
								fnStart--
							}

							fnStart++
							callExpr := line[fnStart:varEnd]
							comp = ResolveFromCall(callExpr, pr.Resolvers)

							break
						}
					}

					j--
				}
			}

			pr.Calls = append(pr.Calls, CallSite{
				FuncName: target, Component: comp, Variable: varName, Line: uint32(lineNum), Caller: caller,
				Resolved: comp != "", Text: strings.TrimSpace(line),
			})
		} else if strings.Contains(lower, " "+t+"(") || strings.Contains(lower, "="+t+"(") || strings.HasPrefix(lower, t+"(") {
			pr.Calls = append(pr.Calls, CallSite{
				FuncName: target, Line: uint32(lineNum), Caller: caller,
				Resolved: false, Text: strings.TrimSpace(line),
			})
		}
	}
}

// resolveVarComponent finds the component a variable resolves to from pr.ComponentRefs.
func (pr *ParseResult) resolveVarComponent(varName string) string {
	// Check ComponentRefs — these are component-wide, always valid
	for _, ref := range pr.ComponentRefs {
		if strings.EqualFold(ref.Variable, varName) {
			return ref.Component
		}
	}

	return ""
}

// resolveVarComponentScoped checks function-local refs only.
// ComponentRefs are checked separately by the caller when appropriate.
func (pr *ParseResult) resolveVarComponentScoped(varName string, line uint32, funcRefs []ComponentRef) string {
	// Check function-local refs only
	var (
		best     string
		bestLine uint32
	)

	for _, ref := range funcRefs {
		if !strings.EqualFold(ref.Variable, varName) || ref.Line > line {
			continue
		}

		if ref.Line >= bestLine {
			best = ref.Component
			bestLine = ref.Line
		}
	}

	return best
}

// maskStrings replaces content inside quoted strings with spaces,
// preserving character positions so index-based scanning still works.
func maskStrings(line string) string {
	var buf []byte

	inQ := byte(0)

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQ == 0:
			if c == '"' || c == '\'' {
				inQ = c

				if buf == nil {
					buf = []byte(line)
				}

				buf[i] = ' '
			}
		case c == inQ:
			inQ = 0

			if buf != nil {
				buf[i] = ' '
			}
		default:
			if buf != nil {
				buf[i] = ' '
			}
		}
	}

	if buf != nil {
		return string(buf)
	}

	return line
}

func funcKey(start, end int) string {
	return strings.Join([]string{itoa(start), itoa(end)}, ":")
}

func atoi(s string) int {
	n := 0

	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}

	return n
}

func itoa(n int) string {
	if n < 0 {
		return "-" + uitoa(uint(-n))
	}

	return uitoa(uint(n))
}

func uitoa(n uint) string {
	if n == 0 {
		return "0"
	}

	var buf [20]byte

	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}

// lineOffsets converts line numbers to byte offsets.
func lineOffsets(content string, startLine, endLine int) (int, int) {
	start := 0
	line := 0

	for line < startLine {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return -1, -1
		}

		start += idx + 1
		line++
	}

	end := start
	for line <= endLine {
		idx := strings.IndexByte(content[end:], '\n')
		if idx < 0 {
			end = len(content)

			break
		}

		end += idx + 1
		line++
	}

	return start, end
}

func (pr *ParseResult) logDebug(msg string, keysAndValues ...any) {
	if pr.log != nil {
		pr.log.Debug(msg, keysAndValues...)
	}
}

func (pr *ParseResult) logWarn(msg string, keysAndValues ...any) {
	if pr.log != nil {
		pr.log.Warn(msg, keysAndValues...)
	}
}

// IsMemberMethod returns true if the method name is a known CFML member function
// on native types (Array, Struct, Query, String, List).
func IsMemberMethod(name string) bool {
	return isMemberMethod(name)
}

// isMemberMethod returns true if the method name is a known CFML member function
// on native types (Array, Struct, Query, String, List).
func isMemberMethod(name string) bool {
	switch strings.ToLower(name) {
	// Array
	case "append", "prepend", "clear", "delete", "deleteat", "each", "every", "filter",
		"find", "findall", "findallnocase", "findnocase", "first", "getat", "indexexists",
		"insertat", "isdefined", "isempty", "last", "len", "map", "max", "median", "merge",
		"mid", "min", "new", "pop", "push", "range", "reduce", "reduceright",
		"removeduplicates", "resize", "reverse", "set", "shift", "slice", "some", "sort",
		"splice", "sum", "swap", "tolist", "tostruct", "unshift", "avg", "contains",
		"containsnocase", "addall", "getduplicates", "compact", "rest",
		// Struct
		"copy", "count", "equals", "findkey", "findvalue", "get", "getmetadata",
		"insert", "iscasesensitive", "isordered", "keyarray", "keyexists", "keylist",
		"keytranslate", "listnew", "setmetadata", "toquerystring", "tosorted", "update",
		"valuearray",
		// Query
		"addcolumn", "addrow", "close", "columnarray", "columncount", "columndata",
		"columnexists", "columnlist", "currentrow", "deletecolumn", "deleterow", "execute",
		"getcell", "getcellbyindex", "getresult", "getrow", "lazy",
		"recordcount", "renamecolumn", "rowbyindex", "rowdata", "rowdatabyindex", "rowswap",
		"setcell", "setrow", "convertforgrid",
		// String/List
		"changedelims", "qualifiedtoarray", "qualify", "itemtrim", "trim",
		"valuecount", "valuecountnocase",
		// Common global/Java methods
		"getbytes", "tostring", "hashcode", "getclass", "init":
		return true
	}

	return false
}

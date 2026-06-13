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
	URI                 uri.URI
	Content             string
	Regions             []Region
	Funcs               []FunctionDef
	ComponentRefs       []ComponentRef
	Scopes              []FuncScope
	Extends             string                                  // dot-path of parent component (from extends attribute)
	Persistent          bool                                    // true if component has persistent="true" (ORM entity)
	Properties          []propertyDef                           // parsed property declarations
	Links               []DocumentLink                          // file path references extracted during shallow scan
	Calls               []CallSite                              // function call sites (when FindCalls is set)
	log                 Logger                                  // optional logger for timing and errors
	Resolvers           []Resolver                              // optional component resolvers for RHS matching
	resolverSet         *ResolverSet                            // pre-grouped resolvers for fast matching
	PropertyResolvers   []PropertyResolver                      // optional property-to-component resolvers
	BeanLookup          func(string) string                     // optional bean name → dot-path lookup
	BuiltinReturnLookup func(string) string                     // optional: builtin function → return component
	FuncLookup          func(component, funcName string) string // optional: resolve method return type from external components
	expressionMappings  map[string]string                       // runtime expression → static value substitutions
	extractLinks        bool                                    // whether to extract links during global scan
	extractCalls        bool                                    // whether to extract all call sites during parsing
	findCalls           []string                                // function names to scan for
	scanAllScopes       bool                                    // scan all lines including function bodies
	shallow             bool                                    // minimal parse mode

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
	funcCallsMap map[string][]CallSite // per-function call sites keyed by "start:end"
}

// ParseOptions configures optional parse behaviour.
type ParseOptions struct {
	Logger              Logger
	Resolvers           []Resolver
	PropertyResolvers   []PropertyResolver
	BeanLookup          func(name string) string // optional: resolve bean name → dot-path
	BuiltinReturnLookup func(name string) string // optional: resolve builtin function → return component
	ExpressionMappings  map[string]string        // runtime expression → static value substitutions
	ExtractLinks        bool                     // extract document links during global scan
	ExtractCalls        bool                     // extract all variable.method() call sites during parsing
	FindCalls           []string                 // function names to find call sites for
	ScanAllScopes       bool                     // scan all lines including function bodies (for refs/deps)
	Shallow             bool                     // minimal parse: signatures only, no refs/properties/args
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
		URI:                 fileURI,
		Content:             content,
		funcVars:            make(map[string][]string),
		log:                 opts.Logger,
		Resolvers:           opts.Resolvers,
		PropertyResolvers:   opts.PropertyResolvers,
		BeanLookup:          opts.BeanLookup,
		BuiltinReturnLookup: opts.BuiltinReturnLookup,
		expressionMappings:  opts.ExpressionMappings,
		extractLinks:        opts.ExtractLinks,
		extractCalls:        opts.ExtractCalls,
		findCalls:           opts.FindCalls,
		scanAllScopes:       opts.ScanAllScopes,
		shallow:             opts.Shallow,
	}
	if len(pr.Resolvers) > 0 {
		pr.resolverSet = BuildResolverSet(pr.Resolvers)
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
			sp.resolverSet = pr.resolverSet
			sp.extractLinks = pr.extractLinks
			sp.extractCalls = pr.extractCalls
			sp.builtinReturnLookup = pr.BuiltinReturnLookup
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

			// Merge links from script parser
			pr.Links = append(pr.Links, sp.links...)
			if len(sp.funcLinks) > 0 {
				if pr.funcLinksMap == nil {
					pr.funcLinksMap = make(map[string][]DocumentLink)
				}

				for k, links := range sp.funcLinks {
					pr.funcLinksMap[k] = append(pr.funcLinksMap[k], links...)
				}
			}

			// Merge calls from script parser
			pr.Calls = append(pr.Calls, sp.calls...)
			if len(sp.funcCalls) > 0 {
				if pr.funcCallsMap == nil {
					pr.funcCallsMap = make(map[string][]CallSite)
				}

				for k, calls := range sp.funcCalls {
					pr.funcCallsMap[k] = append(pr.funcCallsMap[k], calls...)
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
			tp.resolvers = pr.Resolvers
			tp.resolverSet = pr.resolverSet
			tp.extractLinks = pr.extractLinks
			tp.extractCalls = pr.extractCalls
			tp.builtinReturnLookup = pr.BuiltinReturnLookup
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

			// Merge links from tag parser
			if r.StartLine > 0 {
				for i := range tp.links {
					tp.links[i].Line += uint32(r.StartLine)
				}
			}

			pr.Links = append(pr.Links, tp.links...)
			if len(tp.funcLinks) > 0 {
				if pr.funcLinksMap == nil {
					pr.funcLinksMap = make(map[string][]DocumentLink)
				}

				for k, links := range tp.funcLinks {
					if r.StartLine > 0 {
						parts := strings.SplitN(k, ":", 2)
						if len(parts) == 2 {
							start := atoi(parts[0]) + r.StartLine
							end := atoi(parts[1]) + r.StartLine
							k = funcKey(start, end)
						}

						for i := range links {
							links[i].Line += uint32(r.StartLine)
						}
					}

					pr.funcLinksMap[k] = append(pr.funcLinksMap[k], links...)
				}
			}

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

			// Merge calls from tag parser
			if r.StartLine > 0 {
				for i := range tp.calls {
					tp.calls[i].Line += uint32(r.StartLine)
				}
			}

			pr.Calls = append(pr.Calls, tp.calls...)
			if len(tp.funcCalls) > 0 {
				if pr.funcCallsMap == nil {
					pr.funcCallsMap = make(map[string][]CallSite)
				}

				for k, calls := range tp.funcCalls {
					if r.StartLine > 0 {
						parts := strings.SplitN(k, ":", 2)
						if len(parts) == 2 {
							start := atoi(parts[0]) + r.StartLine
							end := atoi(parts[1]) + r.StartLine
							k = funcKey(start, end)
						}

						for i := range calls {
							calls[i].Line += uint32(r.StartLine)
						}
					}

					pr.funcCallsMap[k] = append(pr.funcCallsMap[k], calls...)
				}
			}

			// Tag scopes are computed from full content after region processing
			// to handle functions spanning region boundaries.
			if tp.extends != "" {
				pr.Extends = tp.extends
			}

			if tp.persistent {
				pr.Persistent = true
			}
		}
	}

	// Detect tag-based function scopes from full content (handles functions
	// that span region boundaries, e.g. containing <cfscript> blocks).
	hasTagRegion := false

	for _, r := range pr.Regions {
		if r.Kind == RegionTag {
			hasTagRegion = true

			break
		}
	}

	if hasTagRegion {
		tagScopes := findTagFuncScopes(pr.Content, 0)

		for _, s := range tagScopes {
			duplicate := false

			for _, existing := range pr.Scopes {
				if existing.Start == s.Start {
					duplicate = true

					break
				}
			}

			if !duplicate {
				pr.Scopes = append(pr.Scopes, s)
			}
		}
	}

	// Sort scopes by start line to match function order.
	sortScopes(pr.Scopes)

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

// appendResolverRefs scans content for function call sites (findCalls).
// Resolver refs are now handled by the script/tag parsers during initial parse.
func (pr *ParseResult) appendResolverRefs() {
	if len(pr.findCalls) == 0 {
		return
	}

	if pr.extractCalls {
		// Filter recorded calls by target function names
		pr.filterCallsByName()

		return
	}

	content := pr.Content
	lineNum := 0
	scopeIdx := 0
	currentFunc := ""

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

		// Track current function scope
		if scopeIdx < len(pr.Scopes) {
			if lineNum > pr.Scopes[scopeIdx].End {
				scopeIdx++
				currentFunc = ""
			}

			if scopeIdx < len(pr.Scopes) && lineNum == pr.Scopes[scopeIdx].Start+1 {
				currentFunc = pr.Scopes[scopeIdx].Name
			}
		}

		pr.scanLineForCalls(line, lineNum, currentFunc)

		lineNum++
	}
}

// filterCallsByName populates pr.Calls with calls from funcCallsMap and global
// calls that match the pr.findCalls target names.
func (pr *ParseResult) filterCallsByName() {
	targets := make(map[string]bool, len(pr.findCalls))
	for _, t := range pr.findCalls {
		targets[strings.ToLower(t)] = true
	}

	// Gather all calls (global + per-function)
	var allCalls []CallSite

	allCalls = append(allCalls, pr.Calls...)

	for _, calls := range pr.funcCallsMap {
		allCalls = append(allCalls, calls...)
	}

	// Filter by target names and resolve components
	pr.Calls = nil

	for _, call := range allCalls {
		if !targets[strings.ToLower(call.FuncName)] {
			continue
		}

		// Resolve component from variable name
		if call.Variable != "" && call.Component == "" {
			// Strip scope prefix (VARIABLES.service → service)
			resolveVar := call.Variable
			if dotIdx := strings.LastIndexByte(resolveVar, '.'); dotIdx >= 0 {
				resolveVar = resolveVar[dotIdx+1:]
			}

			call.Component = pr.resolveVarComponent(resolveVar)
			call.Resolved = call.Component != ""
		}

		// Determine caller from line
		if call.Caller == "" {
			call.Caller = pr.callerAtLine(int(call.Line))
		}

		pr.Calls = append(pr.Calls, call)
	}
}

// extractLinksFromLine extracts file path references from a single line.
func extractLinksFromLine(line string, lineNum int, links *[]DocumentLink) {
	// Quick reject: all link attrs contain '=' or "include " — check for quote presence
	if !strings.ContainsAny(line, "\"'") {
		return
	}

	for _, attr := range linkAttrs {
		idx := 0

		for {
			pos := indexFold(line[idx:], attr)
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

		for _, attr := range linkAttrs {
			idx := 0

			for {
				pos := indexFold(line[idx:], attr)
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

// FuncLinks returns document links for a function body from the parse-time cache.
func (pr *ParseResult) FuncLinks(funcStart, funcEnd int) []DocumentLink {
	key := funcKey(funcStart, funcEnd)

	pr.funcRefsMu.Lock()
	defer pr.funcRefsMu.Unlock()

	if pr.funcLinksMap != nil {
		return pr.funcLinksMap[key]
	}

	return nil
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

	// Determine region kind for this function
	regionKind := RegionScript

	for _, r := range pr.Regions {
		if r.StartLine <= funcStart {
			regionKind = r.Kind
		}
	}

	var (
		refs  []ComponentRef
		links []DocumentLink
	)

	if regionKind == RegionScript {
		sp := newScriptParser(body, string(pr.URI), funcStart, pr.Resolvers)
		sp.resolverSet = pr.resolverSet
		sp.extractLinks = true
		sp.parse()
		refs = sp.componentRefs
		links = sp.links
		// Include function-scoped refs (nested functions in body)
		for _, r := range sp.funcRefs {
			refs = append(refs, r...)
		}

		for _, l := range sp.funcLinks {
			links = append(links, l...)
		}
	} else {
		tp := newTagParser(body, string(pr.URI))
		tp.resolvers = pr.Resolvers
		tp.resolverSet = pr.resolverSet
		tp.extractLinks = true
		tp.parse()
		refs = tp.componentRefs

		links = tp.links
		for _, r := range tp.funcRefs {
			refs = append(refs, r...)
		}

		for _, l := range tp.funcLinks {
			links = append(links, l...)
		}
	}

	// Offset lines by funcStart for tag parser (script parser uses baseLine)
	if regionKind != RegionScript {
		for i := range refs {
			refs[i].Line += uint32(funcStart)
		}

		for i := range links {
			links[i].Line += uint32(funcStart)
		}
	}

	// Scan for function calls (still line-based as it's only for exportDeps)
	if len(pr.findCalls) > 0 && !pr.extractCalls {
		lineNum := funcStart + 1

		scan := pr.Content[start:end]
		for len(scan) > 0 {
			nl := strings.IndexByte(scan, '\n')

			var line string
			if nl < 0 {
				line = scan
				scan = ""
			} else {
				line = scan[:nl]
				scan = scan[nl+1:]
			}

			pr.scanLineForCalls(line, lineNum, pr.callerAtLine(lineNum))
			lineNum++
		}
	}

	refs = pr.resolveMethodReturnRefs(funcStart, funcEnd, refs)

	return refs, links
}

// resolveMethodReturnRefs scans a function body for x = y.method() assignments
// where y has a known component ref and method declares a return type.
func (pr *ParseResult) resolveMethodReturnRefs(funcStart, funcEnd int, existingRefs []ComponentRef) []ComponentRef {
	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return existingRefs
	}

	body := pr.Content[start:end]
	lineNum := funcStart + 1

	// Combine global + existing function refs for lookup
	allRefs := make([]ComponentRef, 0, len(pr.ComponentRefs)+len(existingRefs))
	allRefs = append(allRefs, pr.ComponentRefs...)
	allRefs = append(allRefs, existingRefs...)

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

		// Look for: varName = something.method(
		eqIdx := strings.IndexByte(line, '=')
		if eqIdx < 0 || (eqIdx+1 < len(line) && line[eqIdx+1] == '=') {
			lineNum++

			continue
		}

		rhs := strings.TrimSpace(line[eqIdx+1:])
		// Find pattern: ident.ident(
		dotIdx := strings.IndexByte(rhs, '.')
		if dotIdx <= 0 {
			lineNum++

			continue
		}

		parenIdx := strings.IndexByte(rhs[dotIdx:], '(')
		if parenIdx < 0 {
			lineNum++

			continue
		}

		baseVar := strings.TrimSpace(rhs[:dotIdx])
		methodName := strings.TrimSpace(rhs[dotIdx+1 : dotIdx+parenIdx])

		if baseVar == "" || methodName == "" || !isIdentifier(baseVar) || !isIdentifier(methodName) {
			lineNum++

			continue
		}

		// Find component for baseVar
		var baseComp string

		for _, ref := range allRefs {
			if strings.EqualFold(ref.Variable, baseVar) {
				baseComp = ref.Component

				break
			}
		}

		if baseComp == "" {
			lineNum++

			continue
		}

		// Look up the method's return type in the component
		retComp := pr.lookupMethodReturn(baseComp, methodName)
		if retComp == "" {
			lineNum++

			continue
		}

		// Extract LHS variable name
		lhs := strings.TrimSpace(line[:eqIdx])

		varName := lhs
		if spIdx := strings.LastIndexByte(lhs, ' '); spIdx >= 0 {
			varName = strings.TrimSpace(lhs[spIdx+1:])
		}

		if dotIdx := strings.LastIndexByte(varName, '.'); dotIdx >= 0 {
			varName = varName[dotIdx+1:]
		}

		if varName != "" && isIdentifier(varName) {
			existingRefs = append(existingRefs, ComponentRef{
				Variable: varName, Component: retComp, URI: pr.URI, Line: uint32(lineNum),
			})
			allRefs = append(allRefs, existingRefs[len(existingRefs)-1])
		}

		lineNum++
	}

	return existingRefs
}

// lookupMethodReturn finds a method's ReturnComponent in a component.
// Uses FuncLookup callback if available (for cross-file resolution via index).
func (pr *ParseResult) lookupMethodReturn(component, methodName string) string {
	if pr.FuncLookup != nil {
		if ret := pr.FuncLookup(component, methodName); ret != "" {
			return ret
		}
	}

	// Fallback: check same-file functions
	for _, f := range pr.Funcs {
		if strings.EqualFold(f.Name, methodName) {
			if f.ReturnComponent != "" {
				return f.ReturnComponent
			}

			if isComponentType(f.ReturnType) {
				return f.ReturnType
			}
		}
	}

	return ""
}

func isIdentifier(s string) bool {
	for i, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$' {
			continue
		}

		if i > 0 && c >= '0' && c <= '9' {
			continue
		}

		return false
	}

	return len(s) > 0
}

// isValidVarChain returns true if s looks like a valid CFML variable chain
// (e.g. "VARIABLES.prs", "obj", "result[1].data"). Rejects strings containing
// operators, quotes, hash signs, or whitespace.
func isValidVarChain(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$' || c >= '0' && c <= '9' || c == '.' || c == '[' || c == ']' {
			continue
		}

		return false
	}

	return true
}

// FuncCalls returns all variable.method() calls recorded for a function scope.
// Requires ExtractCalls: true in parse options.
func (pr *ParseResult) FuncCalls(funcStart, funcEnd int) []CallSite {
	if pr.extractCalls {
		key := funcKey(funcStart, funcEnd)
		if calls, ok := pr.funcCallsMap[key]; ok {
			return calls
		}

		// Key not found — aggregate all calls (full-file scan)
		var all []CallSite

		all = append(all, pr.Calls...)

		for _, calls := range pr.funcCallsMap {
			all = append(all, calls...)
		}

		return all
	}

	return pr.funcCallsUncached(funcStart, funcEnd)
}

// funcCallsUncached parses a function body on-demand to extract call sites.
func (pr *ParseResult) funcCallsUncached(funcStart, funcEnd int) []CallSite {
	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}

	body := pr.Content[start:end]

	regionKind := RegionScript

	for _, r := range pr.Regions {
		if r.StartLine <= funcStart {
			regionKind = r.Kind
		}
	}

	var calls []CallSite

	if regionKind == RegionScript {
		sp := newScriptParser(body, string(pr.URI), funcStart, pr.Resolvers)
		sp.resolverSet = pr.resolverSet
		sp.extractCalls = true
		sp.parse()
		calls = append(calls, sp.calls...)

		for _, c := range sp.funcCalls {
			calls = append(calls, c...)
		}
	} else {
		tp := newTagParser(body, string(pr.URI))
		tp.resolvers = pr.Resolvers
		tp.resolverSet = pr.resolverSet
		tp.extractCalls = true
		tp.parse()
		calls = append(calls, tp.calls...)

		for _, c := range tp.funcCalls {
			calls = append(calls, c...)
		}

		for i := range calls {
			calls[i].Line += uint32(funcStart)
		}
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
			for varStart >= 0 && (line[varStart] >= 'a' && line[varStart] <= 'z' || line[varStart] >= 'A' && line[varStart] <= 'Z' || line[varStart] >= '0' && line[varStart] <= '9' || line[varStart] == '_' || line[varStart] == '$') {
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
							for fnStart >= 0 && (line[fnStart] >= 'a' && line[fnStart] <= 'z' || line[fnStart] >= 'A' && line[fnStart] <= 'Z' || line[fnStart] >= '0' && line[fnStart] <= '9' || line[fnStart] == '_' || line[fnStart] == '$') {
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

func sortScopes(scopes []FuncScope) {
	for i := 1; i < len(scopes); i++ {
		for j := i; j > 0 && scopes[j].Start < scopes[j-1].Start; j-- {
			scopes[j], scopes[j-1] = scopes[j-1], scopes[j]
		}
	}
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
		"getbytes", "tostring", "hashcode", "getclass", "init", "tobytearray",
		// java.lang.Class / java.lang.reflect methods
		"getcomponenttype", "getname", "getsimplename", "getdeclaredmethods",
		"getmethods", "getdeclaredfields", "getfields", "newinstance",
		"isarray", "isassignablefrom", "isinstance", "getinterfaces",
		"getsuperclass", "forname",
		// JavaScript String/Array/RegExp methods
		"split", "substr", "substring", "indexof", "lastindexof", "tolowercase",
		"touppercase", "charat", "concat", "search", "test", "exec", "join",
		"replace", "match", "startswith", "endswith", "includes", "padstart",
		"padend", "repeat", "charcodeat", "normalize", "trimstart", "trimend",
		// JavaScript DOM/Event/BOM methods
		"focus", "submit", "add", "remove", "item", "preventdefault",
		"stoppropagation", "closest", "on", "configure", "setattribute",
		"getattribute", "removeattribute", "insertcell", "hasownproperty",
		"setdate", "getdate", "getday", "gettime", "getmonth", "setmonth",
		"getfullyear", "setfullyear", "getdocumentelement", "getelementsbytagname",
		"getelementbyid", "getelementsbyclassname", "queryselector", "queryselectorall",
		// jQuery/JS common
		"is", "has", "call", "apply", "bind", "then", "catch", "finally",
		// Common property-like methods (Java/iText objects)
		"width", "height", "length":
		return true
	}

	return false
}

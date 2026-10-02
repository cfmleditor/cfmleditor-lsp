package parser

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
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
	Accessors           bool                                    // component has accessors=true
	Persistent          bool                                    // true if component has persistent="true" (ORM entity)
	Properties          []propertyDef                           // parsed property declarations
	Delegates           []Delegate                              // WireBox delegations the component declares
	Links               []DocumentLink                          // file path references extracted during shallow scan
	Calls               []CallSite                              // function call sites (when FindCalls is set)
	log                 Logger                                  // optional logger for timing and errors
	Resolvers           []Resolver                              // optional component resolvers for RHS matching
	resolverSet         *ResolverSet                            // pre-grouped resolvers for fast matching
	PropertyResolvers   []PropertyResolver                      // optional property-to-component resolvers
	managedSetterLookup func(string) string                     // current property metadata applied to setter injection
	ConstructorLookup   func(string) string                     // known factory constructor dependency identity
	SetterLookup        func(string) string                     // managed bean setter dependency → component
	BeanLookup          func(string) string                     // optional bean name → dot-path lookup
	PropertyBeanLookup  func(string, map[string]string) string  // optional factory property eligibility and identity
	BuiltinReturnLookup func(string) string                     // optional: builtin function → return component
	FuncLookup          func(component, funcName string) string // optional: resolve method return type from external components
	expressionMappings  map[string]string                       // runtime expression → static value substitutions
	expressionKeys      []string                                // expressionMappings' keys, in the order they apply
	// ServicePropertyResolvers maps a "@serviceproperty" annotation kind (e.g. "package",
	// "service", "controller") to a dot-path template containing "${name}". Lets a project
	// document the real component type of a generically-typed (e.g. <cfargument type="struct">)
	// constructor-injected dependency via a "<!--- @serviceproperty varName kind|name --->"
	// comment at each use site, instead of (or in addition to) a componentResolver.
	ServicePropertyResolvers map[string]string
	extractLinks             bool     // whether to extract links during global scan
	extractCalls             bool     // whether to extract all call sites during parsing
	findCalls                []string // function names to scan for
	scanAllScopes            bool     // scan all lines including function bodies
	shallow                  bool     // minimal parse mode
	interpolateAll           bool     // ParseOptions.InterpolateAllText
	outputScanned            bool     // outputSpans and importPrefixes are computed
	outputSpans              [][2]int // byte ranges whose text ColdFusion evaluates
	importPrefixes           []string // <cfimport prefix="..."> values, lowercased

	// contentLineIdx is the line index ClassifyRegions built over Content on its
	// way to the regions, kept so extractSignatures does not build a second one
	// over the same string. nil for a script file, which never needs it.
	contentLineIdx []int32

	// Lazy global var caches (protected by mu).
	mu            sync.Mutex
	globalVars    []string
	globalDone    bool
	variablesVars []string
	varsDone      bool
	thisVars      []string
	thisDone      bool
	allVars       []VarDef
	allVarsDone   bool

	// anyScopedVars caches, per Scope, every name ever assigned in that scope
	// anywhere in the file (see HasScopedAssignment) — unlike variablesVars/thisVars,
	// which only look outside functions and inside init().
	anyScopedVars map[Scope][]string

	// memberSets are the `a.m = …` assignments met; see AssignsMember.
	memberSets []pendingCall

	// funcVars caches per-function variable lists keyed by "start:end".
	funcVarsMu sync.Mutex
	funcVars   map[string][]string

	// funcRefs caches per-function component refs keyed by "start:end".
	memberSnapshot        *ParseResult // member refs computed once per document version
	memberSnapshotContent string
	funcRefsMu            sync.Mutex
	funcRefsMap           map[string][]ComponentRef
	funcLinksMap          map[string][]DocumentLink
	funcCallsMap          map[string][]CallSite // per-function call sites keyed by "start:end"

	// writesMemo, while set, holds collectionWrites' answer for the passes
	// in extractSignatures that share it.
	writesMemo *writesMemo

	// lineStarts holds the byte offset of each line of lineStartsContent, so
	// a function body is sliced without walking the file from its start.
	lineStartsMu      sync.Mutex
	lineStarts        []int
	lineStartsContent string
}

// ParseOptions configures optional parse behaviour.
type ParseOptions struct {
	Logger                   Logger
	Resolvers                []Resolver
	PropertyResolvers        []PropertyResolver
	ConstructorLookup        func(name string) string // optional, only for known factory construction
	SetterLookup             func(name string) string // optional, only for factory-managed components
	BeanLookup               func(name string) string // optional: resolve bean name → dot-path
	PropertyBeanLookup       func(name string, attrs map[string]string) string
	BuiltinReturnLookup      func(name string) string                // optional: resolve builtin function → return component
	FuncLookup               func(component, funcName string) string // optional: resolve method return type from external components
	ExpressionMappings       map[string]string                       // runtime expression → static value substitutions
	ServicePropertyResolvers map[string]string                       // "@serviceproperty" annotation kind → dot-path template
	ExtractLinks             bool                                    // extract document links during global scan
	ExtractCalls             bool                                    // extract all variable.method() call sites during parsing
	FindCalls                []string                                // function names to find call sites for
	ScanAllScopes            bool                                    // scan all lines including function bodies (for refs/deps)
	Shallow                  bool                                    // minimal parse: signatures only, no refs/properties/args
	// InterpolateAllText scans #...# in all of a tag file's text, as the
	// parser did before it learned where ColdFusion evaluates it (see
	// outputContext), and reads a tag-free .cfm template as CFScript. It is
	// the features.outputContextInterpolation switch turned off.
	InterpolateAllText bool
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
	pr.Regions, pr.contentLineIdx = pr.classifyRegions()
	pr.extractSignatures()
	pr.logDebug("parse", "uri", string(fileURI), "funcs", len(pr.Funcs), "refs", len(pr.ComponentRefs), "dur", time.Since(start))

	return pr
}

// classifyRegions is ClassifyRegionsIdx for this file, which also knows its
// name: a .cfm template with no CF tags that is HTML is text, not CFScript,
// unless InterpolateAllText asks for the old reading.
func (pr *ParseResult) classifyRegions() ([]Region, []int32) {
	if !pr.interpolateAll && isMarkupTemplate(string(pr.URI), pr.Content) {
		return splitCFScriptBlocks(pr.Content)
	}

	return ClassifyRegionsIdx(pr.Content)
}

// IsSoftComponent reports whether comp came, in this parse, from a resolver
// marked dynamicIfMissing: a component whose absence is not a finding.
//
// A framework object an injection names counts too, whichever file declared
// the property — a base class usually does: without the framework's source
// or its stubs, a call on one is not a finding.
func (pr *ParseResult) IsSoftComponent(comp string) bool {
	return pr != nil && pr.resolverSet.isSoft(comp) || isInjectedFrameworkComponent(comp)
}

// outputGate returns the output-context ranges and cfimport prefixes of the
// file, computed once, and whether text scanning is gated on them at all.
func (pr *ParseResult) outputGate() (spans [][2]int, prefixes []string, gated bool) {
	if pr.interpolateAll || !pr.extractCalls {
		return nil, nil, false
	}

	if !pr.outputScanned {
		pr.outputSpans, pr.importPrefixes = outputContext(pr.Content)
		pr.outputScanned = true
	}

	return pr.outputSpans, pr.importPrefixes, true
}

// ParseWithOptions performs a full file parse with extended options. opts is
// only read; nil means the zero options.
func ParseWithOptions(fileURI uri.URI, content string, opts *ParseOptions) *ParseResult {
	if opts == nil {
		opts = &ParseOptions{}
	}

	pr := &ParseResult{
		URI:                      fileURI,
		Content:                  content,
		funcVars:                 make(map[string][]string),
		log:                      opts.Logger,
		Resolvers:                opts.Resolvers,
		PropertyResolvers:        opts.PropertyResolvers,
		BeanLookup:               opts.BeanLookup,
		PropertyBeanLookup:       opts.PropertyBeanLookup,
		SetterLookup:             opts.SetterLookup,
		ConstructorLookup:        opts.ConstructorLookup,
		BuiltinReturnLookup:      opts.BuiltinReturnLookup,
		FuncLookup:               opts.FuncLookup,
		expressionMappings:       opts.ExpressionMappings,
		expressionKeys:           ExpressionMappingOrder(opts.ExpressionMappings),
		ServicePropertyResolvers: opts.ServicePropertyResolvers,
		extractLinks:             opts.ExtractLinks,
		extractCalls:             opts.ExtractCalls,
		findCalls:                opts.FindCalls,
		scanAllScopes:            opts.ScanAllScopes,
		shallow:                  opts.Shallow,
		interpolateAll:           opts.InterpolateAllText,
	}
	if len(pr.Resolvers) > 0 {
		pr.resolverSet = BuildResolverSet(pr.Resolvers)
	}

	start := time.Now()
	pr.Regions, pr.contentLineIdx = pr.classifyRegions()
	pr.extractSignatures()
	pr.logDebug("parse", "uri", string(fileURI), "funcs", len(pr.Funcs), "refs", len(pr.ComponentRefs), "dur", time.Since(start))

	return pr
}

// extractSignatures does a shallow parse: function names/args, component refs, scopes.
func (pr *ParseResult) extractSignatures() {
	defer func() {
		if r := recover(); r != nil {
			log.Recovered(pr.log, "parse panic in extractSignatures", r, "uri", string(pr.URI))
		}
	}()

	pr.prepareManagedSetterLookup()

	var allPendingCalls []pendingCall

	// Pre-compute tag function boundaries from the whole file (not per-region)
	// so a function whose body is interrupted by a nested <cfscript> block
	// (which splits the file into separate Tag/Script/Tag... regions) still
	// gets a correct end line and in-function tracking in every region it
	// spans — not just the region containing its opening <cffunction> tag.
	var (
		tagScopes    []FuncScope
		hasTagRegion bool
	)

	for _, r := range pr.Regions {
		if r.Kind == RegionTag {
			hasTagRegion = true
			tagScopes = findTagFuncScopesIdx(pr.Content, 0, pr.contentLineIdx)

			break
		}
	}

	// The locals of a tag function a region split, carried to the regions
	// after the split.
	var open openLocals

	for _, r := range pr.Regions {
		// A RegionSkip is a literal <script> block, left out of the script
		// regions so its JavaScript is never fed to the CFScript scanner. Its
		// `#...#` spans are CFML all the same — a <script> body inside
		// <cfoutput> is where a page writes `var id = "#o.getContentID()#";` —
		// and dropping the region dropped those with it.
		//
		// It goes to the tag parser below, which needs no mode of its own:
		// findScriptSkipSpans only makes a span of a block that holds no `<cf`
		// tag at all, so the walk can find nothing in one *but* its
		// interpolation. A flag restricting it to the spans measured
		// identically over the corpus and against tag-shaped text inside a JS
		// string, which is the case it was written for — and which is not a
		// skip region, because it contains `<cf`.
		if r.Kind == RegionSkip && !pr.extractCalls {
			continue
		}

		if r.Kind == RegionScript {
			allPendingCalls = append(allPendingCalls, pr.mergeScriptRegion(&r, tagScopes, &open)...)
		} else {
			allPendingCalls = append(allPendingCalls, pr.mergeTagRegion(&r, tagScopes, &open)...)
		}
	}

	// Detect tag-based function scopes from full content (handles functions
	// that span region boundaries, e.g. containing <cfscript> blocks).
	//
	// tagScopes and hasTagRegion are the ones computed at the top of this
	// function. findTagFuncScopes used to run here as well, over the same
	// pr.Content, under a guard testing the same condition — 18% of everything a
	// tag parse allocated, spent scanning the file a second time for an answer
	// already in hand.
	if hasTagRegion {
		// Which start lines are already recorded, rather than a scan of pr.Scopes
		// per candidate: both lists are one entry per function, so the pair was
		// quadratic in a component's method count.
		seen := make(map[int]struct{}, len(pr.Scopes))
		for _, existing := range pr.Scopes {
			seen[existing.Start] = struct{}{}
		}

		for _, sc := range tagScopes {
			if _, dup := seen[sc.Start]; dup {
				continue
			}

			seen[sc.Start] = struct{}{}

			pr.Scopes = append(pr.Scopes, sc)
		}
	}

	// Sort scopes by start line to match function order.
	sortScopes(pr.Scopes)

	if pr.extractCalls {
		pr.fillCallers()
	}

	pr.discardPrimitiveReturnComponents()

	// Generate synthetic accessor functions for properties (skip if explicit function exists).
	if !pr.shallow {
		pr.applyExpressionMappings()
		pr.applyServiceProperties()
		pr.generatePropertyAccessors()
		pr.collectDelegates()
		pr.appendResolverRefs()
		// Both passes read the same writes, and nothing between them changes
		// the regions or scopes those are read from.
		pr.writesMemo = &writesMemo{}
		pr.applyMemberBindings()
		pr.applyFactoryReturnCalls(allPendingCalls)
		pr.applyCollectionReturns(allPendingCalls)
		pr.writesMemo = nil
		pr.resolvePendingCalls(allPendingCalls)
		pr.applyChainedReturnLookup()

		if pr.hasMemberBinding(pr.Content) {
			pr.memberSnapshot = &ParseResult{funcRefsMap: maps.Clone(pr.funcRefsMap)}
			pr.memberSnapshotContent = pr.Content
		}
	} else {
		// A shallow parse cannot afford FuncLookup, and a ref typed by the
		// first call of a longer chain is wrong, so such a ref is dynamic.
		dropChainRest(pr, dynamicIfTyped)
	}
}

// discardPrimitiveReturnComponents keeps an inference from overriding a
// declared primitive return type. Without this, a string method's local
// assigned from a receiver call can make the method and its callers return
// the receiver's component. Generic any/component declarations still infer.
func (pr *ParseResult) discardPrimitiveReturnComponents() {
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		f.ReturnComponent = pr.componentReturnFor(f, f.ReturnComponent)
	}
}

// componentReturnFor also guards returns settled after resolver refs become
// available. A runtime-created component remains dynamic even when its method
// declares struct; it must not acquire a concrete component type.
func (pr *ParseResult) componentReturnFor(f *FunctionDef, comp string) string {
	if f.ReturnType != "" && !strings.EqualFold(f.ReturnType, "any") && !looksLikeCFCType(f.ReturnType) {
		if pr.replaceExpressions(comp) == "$any" {
			return "$any"
		}

		return ""
	}

	return comp
}

// openLocals are the names declared local — `var`, `local.` — in the tag
// function still open where a region ends, keyed by the function's file-level
// funcKey. A <cfscript> block inside a <cffunction> splits the file into
// regions, and the parser of each region starts with only the function's
// arguments as its locals: `<cfset var conn = "">` above the block, and
// `conn = uri.openConnection();` inside it read as an unscoped assignment,
// which is a variables-scope one, so the variable was filed for the whole
// component.
type openLocals struct {
	key   string
	names []string
}

// namesFor is the names declared in the function keyed key, if they are the
// ones held.
func (o *openLocals) namesFor(key string) []string {
	if o.key != key {
		return nil
	}

	return o.names
}

// mergeScriptRegion parses a script region and merges what it found into pr,
// returning its pending calls for resolvePendingCalls.
func (pr *ParseResult) mergeScriptRegion(r *Region, tagScopes []FuncScope, open *openLocals) []pendingCall {
	sp := newScriptParser(r.Text, string(pr.URI), r.StartLine, pr.Resolvers).asCFScript()
	sp.resolverSet = pr.resolverSet
	sp.extractLinks = pr.extractLinks
	sp.extractCalls = pr.extractCalls
	sp.builtinReturnLookup = pr.BuiltinReturnLookup
	sp.argumentTypes = pr.argumentTypeLookup()

	// If this <cfscript> region sits inside a tag <cffunction> body
	// (nested script island — ClassifyRegions splits the file there),
	// seed inFunc so refs/pending calls in it route to that function's
	// scope instead of global. scriptParser bakes baseLine into its own
	// keys, so the seed must be the function's absolute funcKey — unlike
	// the tag-region continuation seed below, which stays region-relative.
	seed := -1

	if s, ok := pr.regionTagScope(tagScopes, r.StartLine); ok {
		sp.inFunc = funcKey(s.Start, s.End)
		seed, sp.funcs = pr.seedRegionFunction(s.Start, s.Name)
		sp.localVarSet = make(map[string]bool)

		for _, arg := range pr.argsOfFuncAt(s.Start) {
			sp.localVarSet[strings.ToLower(arg.Name)] = true
		}

		for _, name := range open.namesFor(sp.inFunc) {
			sp.localVarSet[strings.ToLower(name)] = true
		}
	}

	key := sp.inFunc

	sp.parse()

	// What the block declared local stays local in the regions after it.
	if key != "" {
		names := slices.Clone(open.namesFor(key))
		for name := range sp.localVarSet {
			names = append(names, name)
		}

		*open = openLocals{key: key, names: names}
	}

	if seed >= 0 {
		pr.Funcs[seed] = sp.funcs[0]
		sp.funcs = sp.funcs[1:]
	}

	pr.Funcs = append(pr.Funcs, sp.funcs...)
	pr.ComponentRefs = append(pr.ComponentRefs, sp.componentRefs...)
	pr.Scopes = append(pr.Scopes, sp.scopes...)
	pr.Properties = append(pr.Properties, sp.properties...)

	// Merge function-scoped refs from script parser
	for k, refs := range sp.funcRefs {
		pr.funcRefsMap = appendKeyed(pr.funcRefsMap, k, refs)
	}

	// Merge links from script parser
	pr.Links = append(pr.Links, sp.links...)
	for k, links := range sp.funcLinks {
		pr.funcLinksMap = appendKeyed(pr.funcLinksMap, k, links)
	}

	// Merge calls from script parser
	pr.Calls = append(pr.Calls, sp.calls...)
	for k, calls := range sp.funcCalls {
		pr.funcCallsMap = appendKeyed(pr.funcCallsMap, k, calls)
	}

	if sp.extends != "" {
		pr.Extends = sp.extends
	}

	pr.Accessors = pr.Accessors || sp.accessors

	if sp.persistent {
		pr.Persistent = true
	}

	return sp.pendingCalls
}

// mergeTagRegion parses a tag region and merges what it found into pr, with
// its lines and function keys moved from region-relative to file lines,
// returning its pending calls for resolvePendingCalls.
func (pr *ParseResult) mergeTagRegion(r *Region, tagScopes []FuncScope, open *openLocals) []pendingCall {
	tp := newTagParser(r.Text, string(pr.URI))
	tp.resolvers = pr.Resolvers
	tp.resolverSet = pr.resolverSet
	tp.extractLinks = pr.extractLinks
	tp.extractCalls = pr.extractCalls
	tp.builtinReturnLookup = pr.BuiltinReturnLookup
	tp.argumentTypes = pr.argumentTypeLookup()
	tp.baseLine = r.StartLine
	tp.knownScopes = tagScopes
	tp.outputSpans, tp.importPrefixes, tp.gated = pr.outputGate()
	tp.srcOffset = r.Offset

	// If this region starts partway through a function whose opening
	// <cffunction> tag was in an earlier region (interrupted by a
	// nested <cfscript> region split), seed inFunc/localVars so refs
	// in this region still route to function scope instead of global.
	seed := -1

	if s, ok := pr.regionTagScope(tagScopes, r.StartLine); ok {
		tp.inFunc = funcKey(s.Start-r.StartLine, s.End-r.StartLine)
		seed, tp.funcs = pr.seedRegionFunction(s.Start, s.Name)
		tp.localVars = nil

		for _, arg := range pr.argsOfFuncAt(s.Start) {
			tp.markVarLocal(arg.Name)
		}

		for _, name := range open.namesFor(funcKey(s.Start, s.End)) {
			tp.markVarLocal(name)
		}
	}

	tp.parse()

	if seed >= 0 {
		pr.Funcs[seed] = tp.funcs[0]
		tp.funcs = tp.funcs[1:]
	}

	// The function still open where the region ends continues in the next.
	if tp.inFunc != "" {
		*open = openLocals{key: shiftFuncKey(tp.inFunc, r.StartLine), names: slices.Clone(tp.localVars)}
	}

	for i := range tp.funcs {
		tp.funcs[i].Line += conv.Uint32(r.StartLine)
	}

	for i := range tp.componentRefs {
		tp.componentRefs[i].Line += conv.Uint32(r.StartLine)
	}

	for i := range tp.properties {
		tp.properties[i].line += conv.Uint32(r.StartLine)
	}

	pr.Funcs = append(pr.Funcs, tp.funcs...)
	pr.ComponentRefs = append(pr.ComponentRefs, tp.componentRefs...)
	pr.Properties = append(pr.Properties, tp.properties...)

	// Merge links from tag parser
	if r.StartLine > 0 {
		for i := range tp.links {
			tp.links[i].Line += conv.Uint32(r.StartLine)
		}
	}

	pr.Links = append(pr.Links, tp.links...)
	for k, links := range tp.funcLinks {
		if r.StartLine > 0 {
			k = shiftFuncKey(k, r.StartLine)

			for i := range links {
				links[i].Line += conv.Uint32(r.StartLine)
			}
		}

		pr.funcLinksMap = appendKeyed(pr.funcLinksMap, k, links)
	}

	// Merge function-scoped refs from tag parser
	for k, refs := range tp.funcRefs {
		// Offset the key and ref lines by region start line
		if r.StartLine > 0 {
			k = shiftFuncKey(k, r.StartLine)

			for i := range refs {
				refs[i].Line += conv.Uint32(r.StartLine)
			}
		}

		pr.funcRefsMap = appendKeyed(pr.funcRefsMap, k, refs)
	}

	// Collect pending calls from tag parser (offset lines and funcKey)
	for i := range tp.pendingCalls {
		tp.pendingCalls[i].line += conv.Uint32(r.StartLine)
		if r.StartLine > 0 && tp.pendingCalls[i].funcKey != "" {
			tp.pendingCalls[i].funcKey = shiftFuncKey(tp.pendingCalls[i].funcKey, r.StartLine)
		}
	}

	// Merge calls from tag parser
	if r.StartLine > 0 {
		for i := range tp.calls {
			tp.calls[i].Line += conv.Uint32(r.StartLine)
		}
	}

	pr.Calls = append(pr.Calls, tp.calls...)
	for k, calls := range tp.funcCalls {
		if r.StartLine > 0 {
			k = shiftFuncKey(k, r.StartLine)

			for i := range calls {
				calls[i].Line += conv.Uint32(r.StartLine)
			}
		}

		pr.funcCallsMap = appendKeyed(pr.funcCallsMap, k, calls)
	}

	// Tag scopes are computed from full content after region processing
	// to handle functions spanning region boundaries.
	if tp.extends != "" {
		pr.Extends = tp.extends
	}

	pr.Accessors = pr.Accessors || tp.accessors

	if tp.persistent {
		pr.Persistent = true
	}

	return tp.pendingCalls
}

// enclosingTagScope is the tag function a region starting at startLine opens
// inside of: one whose opening <cffunction> is in an earlier region, the file
// having been split around a nested <cfscript>.
func enclosingTagScope(tagScopes []FuncScope, startLine int) (FuncScope, bool) {
	for _, s := range tagScopes {
		if s.Start < startLine && startLine <= s.End {
			return s, true
		}
	}

	return FuncScope{}, false
}

// argsOfFuncAt is the argument list of the first function declared at line.
func (pr *ParseResult) argsOfFuncAt(line int) []Argument {
	for i := range pr.Funcs {
		if int(pr.Funcs[i].Line) == line {
			return pr.Funcs[i].Arguments
		}
	}

	return nil
}

// shiftFuncKey moves a "start:end" function key by delta lines.
func shiftFuncKey(k string, delta int) string {
	parts := strings.SplitN(k, ":", 2)
	if len(parts) != 2 {
		return k
	}

	return funcKey(atoi(parts[0])+delta, atoi(parts[1])+delta)
}

// appendKeyed appends v to m[k], making the map on first use, and returns it.
func appendKeyed[T any](m map[string][]T, k string, v []T) map[string][]T {
	if m == nil {
		m = make(map[string][]T)
	}

	m[k] = append(m[k], v...)

	return m
}

// servicePropertyRe matches a "@serviceproperty varName kind|name" annotation inside a
// CFML tag comment, e.g. "<!--- @serviceproperty donorObj package|sandbox.tasscom.components.donor --->".
// This is a project-specific documentation convention (not standard CFML/CFC syntax) for
// annotating the real component type of a dependency that's declared with a generic
// <cfargument type="struct"> (CF doesn't enforce component types on constructor args, so
// projects sometimes type them loosely and document the real type in a comment instead).
// varName allows "." and "[...]" (e.g. "result.data[i]") so the same convention can
// annotate a bracket-indexed receiver — a case CanResolveCall otherwise can't resolve at
// all without a global componentResolver keyed on the bare receiver text, which risks
// matching an unrelated same-named receiver anywhere else in the workspace. An in-source
// annotation is scoped to this one file, so it doesn't carry that risk.
var servicePropertyRe = regexp.MustCompile(`@serviceproperty\s+([A-Za-z_][A-Za-z0-9_.\[\]]*)\s+([A-Za-z]+)\|(\S+)`)

// applyServiceProperties scans the whole file for "@serviceproperty" annotations and
// registers a ComponentRef for the annotated variable at the annotation's line, resolved
// via the per-kind dot-path template in pr.ServicePropertyResolvers (e.g.
// "service" -> "tassweb.packages.${name}.service" turns "@serviceproperty objGLJournal
// service|gljournal" into a ref pointing at "tassweb.packages.gljournal.service"). A no-op
// when ServicePropertyResolvers isn't configured, so projects not using the convention pay
// no parsing cost. Runs after pr.Scopes is finalized so each ref can be routed to the
// enclosing function's scope, same as any other ref — this is what makes the annotation
// work like a real (if file-scanned rather than scanner-tracked) assignment: a later
// re-annotation of the same variable name overrides an earlier one, via the existing
// nearest-preceding-ComponentRef resolution in resolve.CanResolveCall.
func (pr *ParseResult) applyServiceProperties() {
	if len(pr.ServicePropertyResolvers) == 0 {
		return
	}

	for _, m := range servicePropertyRe.FindAllStringSubmatchIndex(pr.Content, -1) {
		varName := pr.Content[m[2]:m[3]]
		kind := strings.ToLower(pr.Content[m[4]:m[5]])
		name := pr.Content[m[6]:m[7]]

		tmpl, ok := pr.ServicePropertyResolvers[kind]
		if !ok {
			continue
		}

		component := strings.ReplaceAll(tmpl, "${name}", name)
		line := conv.Uint32(strings.Count(pr.Content[:m[0]], "\n"))

		// resolve.CanResolveCall strips a receiver down to the text after its last
		// top-level "." before comparing against ComponentRef.Variable (e.g.
		// "VARIABLES.donorObj" is looked up as "donorObj"; StripReceiverScope skips any
		// "." inside a "[...]" subscript, so "linkMap[arguments.startSource]" is left
		// unchanged rather than being cut to "startSource]"). Apply the same stripping
		// here so an annotation written against the natural, full receiver text at the
		// call site (e.g. "result.data[i]", matching what a reader actually sees in the
		// source) is stored under the same key CanResolveCall will look it up with.
		lookupVar := StripReceiverScope(varName)

		ref := ComponentRef{
			Variable: lookupVar, Component: component,
			URI: pr.URI, Line: line,
		}

		if fs := findFuncScope(int(line), pr.Scopes); fs.Start != -1 {
			if pr.funcRefsMap == nil {
				pr.funcRefsMap = make(map[string][]ComponentRef)
			}

			key := funcKey(fs.Start, fs.End)
			pr.funcRefsMap[key] = append(pr.funcRefsMap[key], ref)
		} else {
			pr.ComponentRefs = append(pr.ComponentRefs, ref)
		}
	}
}

// applyChainedReturnLookup re-checks component refs created from a receiver.method()
// call (see ComponentRef.ChainBase/ChainMethod) against FuncLookup, overriding the
// componentResolver's guess on the call-site text with the callee's own declared
// return type when FuncLookup can verify it (e.g. a Java stub's getInstance()
// modeling its real return type). Runs last so a verified answer always wins.
//
// It then walks ChainRest, the calls chained after the one that typed the ref,
// so "chart = b.width(1).height(2).build()" types chart by build rather than
// by width. A call it cannot type makes the variable dynamic: keeping the type
// of an earlier call is the wrong answer this exists to stop, and it reported
// every method of the real value as missing from the builder.
func (pr *ParseResult) applyChainedReturnLookup() {
	if pr.FuncLookup == nil {
		dropChainRest(pr, dynamicIfTyped)

		return
	}

	lookupBase := func(baseVar string, scopedRefs []ComponentRef) string {
		for i := range scopedRefs {
			ref := &scopedRefs[i]

			if strings.EqualFold(ref.Variable, baseVar) {
				return ref.Component
			}
		}

		for i := range pr.ComponentRefs {
			ref := &pr.ComponentRefs[i]

			if strings.EqualFold(ref.Variable, baseVar) {
				return ref.Component
			}
		}

		return ""
	}

	override := func(refs []ComponentRef, scoped []ComponentRef) {
		for i := range refs {
			if refs[i].ChainBase == "" {
				continue
			}

			baseComp := lookupBase(refs[i].ChainBase, scoped)
			if baseComp == "" {
				continue
			}

			if ret := pr.FuncLookup(baseComp, refs[i].ChainMethod); ret != "" {
				refs[i].Component = ret
			}
		}
	}

	override(pr.ComponentRefs, nil)

	for k, refs := range pr.funcRefsMap {
		override(refs, refs)
		pr.funcRefsMap[k] = refs
	}

	dropChainRest(pr, pr.walkChainRest)
}

// walkChainRest follows comp through the calls in rest by their declared
// return types. init() is taken to return the object it is called on, which is
// the convention for a CFC constructor and what a java stub's constructor
// models.
func (pr *ParseResult) walkChainRest(comp string, rest []string) string {
	for _, hop := range rest {
		name := callHopName(hop)
		switch {
		case comp == "" || comp == "$any":
			return comp
		case strings.HasPrefix(comp, "$builtin."):
			return "$any"
		case strings.EqualFold(name, "init"), IsMockDecoration(name):
			// A MockBox decoration returns the mock it is called on.
			continue
		}

		ret := pr.FuncLookup(comp, hop)
		if ret == "" && name != hop {
			ret = pr.FuncLookup(comp, name)
		}

		if ret == "" {
			return "$any"
		}

		comp = ret
	}

	return comp
}

// dynamicIfTyped is the answer without FuncLookup: a chain whose rest holds
// any call but init() ends somewhere unknown.
func dynamicIfTyped(comp string, rest []string) string {
	if restTypes(rest) {
		return "$any"
	}

	return comp
}

// restTypes reports whether a chain's rest can change the type it started
// with: anything but init() calls and MockBox decorations, which return what
// they are called on.
func restTypes(rest []string) bool {
	return slices.ContainsFunc(rest, func(h string) bool {
		return !strings.EqualFold(callHopName(h), "init") && !IsMockDecoration(callHopName(h))
	})
}

// settledComponent is ref's Component with any pending chain walked, for a
// reader during the parse that has FuncLookup to hand — resolvePendingCalls,
// typing a variable from the one it was called on — so it reads what the
// finished ref will hold rather than the first call's type.
func (pr *ParseResult) settledComponent(ref *ComponentRef) string {
	if !chainPending(ref) {
		return ref.Component
	}

	if pr.FuncLookup == nil {
		return strictChainComponent(ref, dynamicIfTyped(ref.Component, ref.ChainRest))
	}

	return strictChainComponent(ref, pr.walkChainRest(ref.Component, ref.ChainRest))
}

func strictChainComponent(ref *ComponentRef, component string) string {
	if ref.strictChain && strings.HasPrefix(component, "$") {
		return ""
	}

	return component
}

// chainPending reports whether ref's Component is not yet its answer: it was
// typed by the first call of a longer chain that applyChainedReturnLookup has
// still to walk. A lookup made during the parse must not read it — it would
// hand on the first call's type, which is exactly the wrong answer the walk
// exists to replace — and gets nothing instead, which the resolve step, run
// after the walk, answers from the finished ref.
func chainPending(ref *ComponentRef) bool {
	return restTypes(ref.ChainRest)
}

// dropChainRest applies type to every ref carrying a ChainRest, then clears it.
func dropChainRest(pr *ParseResult, typ func(comp string, rest []string) string) {
	apply := func(refs []ComponentRef) {
		for i := range refs {
			if len(refs[i].ChainRest) == 0 {
				continue
			}

			refs[i].Component = strictChainComponent(&refs[i], typ(refs[i].Component, refs[i].ChainRest))
			refs[i].ChainRest = nil
		}
	}

	apply(pr.ComponentRefs)

	for _, refs := range pr.funcRefsMap {
		apply(refs)
	}
}

// applyExpressionMappings replaces runtime expressions in component paths with static
// values, then treats anything still containing a "#...#" CFML expression as genuinely
// dynamic (e.g. CreateObject("component", "tools.templates.#ARGUMENTS.template#.generator") —
// a runtime-computed path with no configured mapping) and collapses it to "$any" so
// CanResolveCall accepts calls through it instead of surfacing the raw "#...#" text as a
// bogus "not found in #...#" component name. Runs even with no expressionMappings
// configured, since the "$any" fallback is unconditional.
func (pr *ParseResult) applyExpressionMappings() {
	for i := range pr.ComponentRefs {
		pr.ComponentRefs[i].Component = pr.replaceExpressions(pr.ComponentRefs[i].Component)
	}

	for k, refs := range pr.funcRefsMap {
		for i := range refs {
			refs[i].Component = pr.replaceExpressions(refs[i].Component)
		}

		pr.funcRefsMap[k] = refs
	}

	// CallSite.Component can also be baked in directly at parse time (e.g. a bare
	// "x.method()" call whose receiver was resolved inline via lookupComponentRef,
	// or a chained new/createObject) — fix those up too, not just ComponentRefs.
	for i := range pr.Calls {
		pr.Calls[i].Component = pr.replaceExpressions(pr.Calls[i].Component)
	}

	for k, calls := range pr.funcCallsMap {
		for i := range calls {
			calls[i].Component = pr.replaceExpressions(calls[i].Component)
		}

		pr.funcCallsMap[k] = calls
	}
}

// ExpressionMappingOrder returns the keys of an expressionMappings block in the
// order they are applied: longest first, and by key among equals. Applying
// them in map order let two overlapping keys — `#A#` and `#A#.b` — replace in
// either order from run to run, so one component path named two different
// components, and a code-map edge moved between them. The longer, more
// specific expression goes first, as route aliases do.
func ExpressionMappingOrder(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}

	keys := slices.Collect(maps.Keys(m))
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})

	return keys
}

func (pr *ParseResult) replaceExpressions(comp string) string {
	if !strings.Contains(comp, "#") {
		return comp
	}

	for _, key := range pr.expressionKeys {
		value := pr.expressionMappings[key]

		for expr := range strings.SplitSeq(key, "|") {
			if expr != "" && strings.Contains(comp, expr) {
				comp = strings.ReplaceAll(comp, expr, value)
			}
		}
	}

	if strings.Contains(comp, "#") {
		return "$any"
	}

	return comp
}

// resolvePendingCalls resolves varName = funcCall(...) assignments against same-file functions.
func (pr *ParseResult) resolvePendingCalls(calls []pendingCall) {
	returnPendings := pr.pendingReturnVars()

	// Resolve ReturnComponent from return var before processing calls
	pr.settleReturnVars(returnPendings)

	// Function name → return component, including the ones just settled.
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

	for j := range calls {
		c := &calls[j]

		if c.returnExpr {
			continue
		}

		if c.memberSet {
			pr.memberSets = append(pr.memberSets, *c)

			continue
		}

		// Skip if this variable already has a ref (e.g. from appendResolverRefs)
		if pr.hasRefFor(c) {
			continue
		}

		comp := funcReturns[strings.ToLower(c.funcName)]

		// Fallback: x = baseVar.method() — assign x same component as baseVar
		if comp == "" && c.baseVar != "" {
			comp = pr.baseVarComponent(c)
		}

		// x = inherited(): a method the file does not declare is its base's,
		// which FuncLookup reaches from the file itself. ContentBox's
		// services write `var c = newCriteria()`, a method of cborm's
		// BaseORMService, and c was untyped.
		if comp == "" && c.baseVar == "" && pr.FuncLookup != nil && pr.URI.IsFile() {
			comp = pr.FuncLookup(pr.URI.Path(), c.funcName)
		}

		if comp == "" && c.baseVar == "" && c.expression != "" && pr.FuncLookup != nil && pr.URI.IsFile() {
			comp = pr.FuncLookup(pr.URI.Path(), CallHop(c.expression))
		}

		// A mock made through the MockBox a spec holds,
		// getMockBox().createEmptyMock(…), is as dynamic as one made directly.
		if comp == "" {
			last := c.funcName
			if len(c.rest) > 0 {
				last = callHopName(c.rest[len(c.rest)-1])
			}

			if comp = dynamicCall(last + "()"); comp != "" {
				c.rest = nil // the chain ends in the mock; nothing to walk
			}
		}

		// A chain that decorates what it is made on is made on a mock, and
		// a mock of a class nothing here names is dynamic:
		// `variables.iService = model.init( c ).$( "getCache", x )`, where
		// model is the base class's createMock( annotations.model ).
		if comp == "" && (IsMockDecoration(c.funcName) || slices.ContainsFunc(c.rest, func(hop string) bool { return IsMockDecoration(callHopName(hop)) })) {
			comp, c.rest = "$any", nil
		}

		if comp == "" {
			continue
		}

		ref := ComponentRef{
			Variable: c.varName, Component: comp, ChainRest: c.rest,
			URI: pr.URI, Line: c.line, This: c.refThis,
			VisibleFrom: c.visibleFrom, VisibleTo: c.visibleTo,
		}
		if c.funcKey == "" || c.global {
			pr.ComponentRefs = append(pr.ComponentRefs, ref)
		} else {
			pr.funcRefsMap = appendKeyed(pr.funcRefsMap, c.funcKey, []ComponentRef{ref})
		}
	}

	// Second pass: resolve ReturnComponent for functions whose return var was just added by calls
	pr.settleReturnVars(returnPendings)
}

// returnPending is a function whose return type is whatever a variable it
// returns holds, still to be looked up.
type returnPending struct {
	funcIdx int
	varName string
	funcKey string
}

// pendingReturnVars lists the functions that return a variable and have no
// return component yet.
func (pr *ParseResult) pendingReturnVars() []returnPending {
	var out []returnPending

	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if f.ReturnComponent != "" || f.returnVar == "" {
			continue
		}

		// `return this;` returns the component that declares the function:
		// its file, named by path, which is how the resolver takes it.
		if f.returnVar == returnsThis {
			if path, ok := strings.CutPrefix(string(f.URI), "file://"); ok && path != "" {
				f.ReturnComponent = pr.componentReturnFor(f, path)
			}

			continue
		}

		if scope := findFuncScope(int(f.Line), pr.Scopes); scope.Start >= 0 {
			out = append(out, returnPending{
				funcIdx: i,
				varName: f.returnVar,
				funcKey: funcKey(scope.Start, scope.End),
			})
		}
	}

	return out
}

// returnsThis is the returnVar of a function whose return is `this` alone,
// a name no variable can have.
const returnsThis = "$this"

// returnsCallOn prefixes the returnVar of a function returning a call made
// on a variable — `return di.toClazz( c )` — for which the variable's
// component answers only when it is dynamic. It holds a colon, which no
// variable name can.
const returnsCallOn = "$call:"

// returnsVariablesVar and returnsThisVar prefix the returnVar of a function
// returning a variable read through its scope — `return variables.print;`,
// `return this.other;`. Such a variable is the component's, never a local of
// the function, and `this.x` and `variables.x` are separate stores, so each
// is read only from the component's refs made through its own scope. Like
// returnsCallOn, each holds a colon, which no variable name can.
const (
	returnsVariablesVar = "$variables:"
	returnsThisVar      = "$this:"
)

// scopedReturnVar is the returnVar of a function returning name read through
// scope, which is RefThis or RefVariables.
func scopedReturnVar(scope RefScope, name string) string {
	if scope == RefThis {
		return returnsThisVar + name
	}

	return returnsVariablesVar + name
}

// returnedVar is the variable a returnVar names, whether the function
// returns a call made on it rather than the variable itself, and the scope
// it was read through: RefAny for a bare name, which may be a local.
func returnedVar(returnVar string) (name string, called bool, scope RefScope) {
	if name, ok := strings.CutPrefix(returnVar, returnsCallOn); ok {
		return name, true, RefAny
	}

	if name, ok := strings.CutPrefix(returnVar, returnsVariablesVar); ok {
		return name, false, RefVariables
	}

	if name, ok := strings.CutPrefix(returnVar, returnsThisVar); ok {
		return name, false, RefThis
	}

	return returnVar, false, RefAny
}

// returnedComponent is what a function returns, given the component its
// return variable holds: that component, or for a call made on the variable,
// "$any" when the variable is dynamic and nothing otherwise — what a method
// returns is not the component it is a method of.
func returnedComponent(comp string, called bool) string {
	if !called || comp == "$any" {
		return comp
	}

	return ""
}

// settleReturnVars gives each pending function without a return component
// the component its return variable holds, when a ref in its body says.
func (pr *ParseResult) settleReturnVars(pending []returnPending) {
	for _, rp := range pending {
		if pr.Funcs[rp.funcIdx].ReturnComponent != "" {
			continue
		}

		name, called, scope := returnedVar(rp.varName)

		// A scoped name is the component's variable, not a local of the
		// function's.
		if scope != RefAny {
			if ref := firstRefIn(pr.ComponentRefs, name, scope); ref != nil {
				pr.Funcs[rp.funcIdx].ReturnComponent = pr.componentReturnFor(&pr.Funcs[rp.funcIdx], pr.settledComponent(ref))
			}

			continue
		}

		// A closure's local of the same name is not what the function
		// returns. An unscoped name the function never declared is a
		// variables-scope one, as settleReturnComponent reads it: a pending
		// call assigning it settles at component level.
		ref := lastWideRefNamed(pr.funcRefsMap[rp.funcKey], name)
		if ref == nil {
			ref = firstRefNamed(pr.ComponentRefs, name)
		}

		if ref != nil {
			pr.Funcs[rp.funcIdx].ReturnComponent = pr.componentReturnFor(&pr.Funcs[rp.funcIdx], returnedComponent(pr.settledComponent(ref), called))
		}
	}
}

// hasRefFor reports whether the variable a pending call assigns already has a
// ref, in its function or at file level.
//
// Only a ref in the pending call's own scope counts: a sibling closure's `t`
// is another variable, and letting it stand for this one left this `t`
// untyped whenever an earlier test in the spec declared a `t` of its own.
func (pr *ParseResult) hasRefFor(c *pendingCall) bool {
	if c.funcKey != "" {
		refs := pr.funcRefsMap[c.funcKey]
		for i := range refs {
			if ref := &refs[i]; strings.EqualFold(ref.Variable, c.varName) &&
				ref.VisibleFrom == c.visibleFrom && ref.VisibleTo == c.visibleTo {
				return true
			}
		}
	}

	return firstRefNamed(pr.ComponentRefs, c.varName) != nil
}

// baseVarComponent is the component `x = baseVar.method()` gives x: the
// method's declared return type when FuncLookup can say, else baseVar's own
// component, or "$any" when the method is known to declare none.
func (pr *ParseResult) baseVarComponent(c *pendingCall) string {
	var comp string

	if ref := firstRefIn(pr.ComponentRefs, c.baseVar, c.baseScope); ref != nil {
		comp = pr.settledComponent(ref)
	}

	if comp == "" && c.funcKey != "" {
		if ref := firstRefIn(pr.funcRefsMap[c.funcKey], c.baseVar, c.baseScope); ref != nil {
			comp = pr.settledComponent(ref)
		}
	}

	// If FuncLookup is available, prefer the called method's own declared
	// return type over the "same as baseVar" guess — it may differ from the
	// receiver's type. If the method declares no component return type, don't
	// propagate the base variable's component at all.
	// `x = m.$( "get", 1 )`: a decoration returns the mock it is called on.
	if comp != "" && IsMockDecoration(c.funcName) {
		return comp
	}

	if comp != "" && c.funcName != "" && pr.FuncLookup != nil {
		if ret := pr.FuncLookup(comp, c.funcName); ret != "" {
			return ret
		}

		if c.expression != "" {
			if ret := pr.FuncLookup(comp, CallHop(c.expression)); ret != "" {
				return ret
			}
		}

		return "$any"
	}

	// A framework object an injection typed hands back other things: an
	// injector's getInstance() is not an injector. The guess is for a
	// project's own fluent components, as a preset's name-only resolvers keep
	// it from the objects they type.
	if isInjectedFrameworkComponent(comp) {
		return ""
	}

	return comp
}

// firstRefIn is firstRefNamed among the refs scope admits.
func firstRefIn(refs []ComponentRef, name string, scope RefScope) *ComponentRef {
	for i := range refs {
		if strings.EqualFold(refs[i].Variable, name) && scope.Admits(&refs[i]) {
			return &refs[i]
		}
	}

	return nil
}

// firstRefNamed is the first ref in refs for the variable name, compared
// case-insensitively, or nil.
func firstRefNamed(refs []ComponentRef, name string) *ComponentRef {
	for i := range refs {
		if strings.EqualFold(refs[i].Variable, name) {
			return &refs[i]
		}
	}

	return nil
}

// lastWideRefNamed is the latest assignment the whole function makes to name,
// by line, passing over the ones a closure declared. A function returns what
// its variable holds at the return, which follows the assignments: taking
// the first typed kernel2's getSandBox by `var result = getService(…)` when
// a cfinvoke two lines later replaces result with what the sandbox returns.
func lastWideRefNamed(refs []ComponentRef, name string) *ComponentRef {
	var last *ComponentRef

	for i := range refs {
		if refs[i].VisibleTo == 0 && strings.EqualFold(refs[i].Variable, name) && (last == nil || refs[i].Line >= last.Line) {
			last = &refs[i]
		}
	}

	return last
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
	for i := range pr.Funcs {
		f := &pr.Funcs[i]

		existing[strings.ToLower(f.Name)] = true
	}

	u := pr.URI

	// A generated setter returns the component, for chaining — Lucee's and
	// Adobe's alike — so it is typed as `return this;` is: by the file's path.
	self, _ := strings.CutPrefix(string(u), "file://")

	fieldComponents := pr.propertyFieldComponents()

	for _, prop := range pr.Properties {
		capName := ucFirst(prop.name)

		getterIdx := -1

		getter := "get" + strings.ToLower(prop.name)
		if !existing[getter] {
			existing[getter] = true
			getterIdx = len(pr.Funcs)

			pr.Funcs = append(pr.Funcs, FunctionDef{
				Name: "get" + capName, URI: u, Line: prop.line,
				ReturnType: prop.typeName,
			})
		}

		setter := "set" + strings.ToLower(prop.name)
		if !existing[setter] {
			existing[setter] = true

			pr.Funcs = append(pr.Funcs, FunctionDef{
				Name: "set" + capName, URI: u, Line: prop.line,
				Arguments:       []Argument{{Name: prop.name, Type: prop.typeName}},
				ReturnComponent: self,
			})
		}

		for _, m := range relationshipMethods(&prop) {
			if !existing[strings.ToLower(m)] {
				existing[strings.ToLower(m)] = true

				pr.Funcs = append(pr.Funcs, FunctionDef{Name: m, URI: u, Line: prop.line})
			}
		}
		// Resolve component path: try property resolvers first, then type, then bean map
		comp := ""
		if len(pr.PropertyResolvers) > 0 && len(prop.attrs) > 0 {
			comp = ResolveProperty(prop.attrs, pr.PropertyResolvers)
		}

		if comp == "" && prop.typeName != "" && looksLikeCFCType(prop.typeName) {
			comp = prop.typeName
		}

		if comp == "" {
			comp = prop.documentedComponent()
		}

		// Only a CFML ORM entity: Mura's own beans declare relationships the
		// same way, but their cfc names a bean id ("site"), not a component.
		if comp == "" && pr.Persistent {
			comp = prop.relatedEntity()
		}

		if comp == "" {
			comp = pr.propertyBeanComponent(&prop)
		}

		if comp != "" {
			pr.ComponentRefs = append(pr.ComponentRefs, ComponentRef{
				Variable: prop.name, Component: comp, URI: u, Line: prop.line,
			})

			// A generated getter returns the property, so it returns what
			// the property holds: cborm's getWireBox() is the injector.
			if getterIdx >= 0 {
				pr.Funcs[getterIdx].ReturnComponent = comp
			}
		} else if getterIdx >= 0 && (prop.typeName == "" || strings.EqualFold(prop.typeName, "any")) {
			pr.Funcs[getterIdx].ReturnComponent = fieldComponents[strings.ToLower(prop.name)]
		}
	}
}

// relatedEntity is the entity a single-valued ORM relationship holds: a
// many-to-one or one-to-one property names it in its cfc attribute, so the
// generated getter returns one. ContentBox's content items reach their site
// through `property name="site" fieldtype="many-to-one" cfc="…system.Site"`.
// A collection relationship holds an array or struct of them, not one.
func (prop *propertyDef) relatedEntity() string {
	switch strings.ToLower(prop.attrs["fieldtype"]) {
	case "many-to-one", "one-to-one":
	default:
		return ""
	}

	if prop.typeName != "" && !strings.EqualFold(prop.typeName, "any") {
		return ""
	}

	cfc := strings.TrimSpace(prop.attrs["cfc"])
	if cfc == "" || strings.Contains(cfc, "#") {
		return ""
	}

	return cfc
}

// CFML's doc_generic property metadata names the value's component, just as
// argument.doc_generic does for a generic argument. Preserve explicit types
// and do not treat an array's element type as its receiver.
func (prop *propertyDef) documentedComponent() string {
	if prop.typeName != "" && !strings.EqualFold(prop.typeName, "any") && !strings.EqualFold(prop.typeName, "struct") {
		return ""
	}

	if documented := prop.attrs["doc_generic"]; isComponentType(documented) {
		return documented
	}

	return ""
}

// A generated getter reads variables.name. Constructor assignments
// already type that field, even when the property has no metadata.
// Locals and this.name are separate stores; unresolved call chains
// cannot supply a getter type. Conflicting field types stay dynamic.
func (pr *ParseResult) propertyFieldComponents() map[string]string {
	fieldComponents := make(map[string]string, len(pr.ComponentRefs))
	for i := range pr.ComponentRefs {
		ref := &pr.ComponentRefs[i]
		if ref.This {
			continue
		}

		candidate := ref.Component
		if candidate == "" || ref.ChainBase != "" || chainPending(ref) {
			candidate = "$any"
		}

		key := strings.ToLower(ref.Variable)
		if previous := fieldComponents[key]; previous != "" && !strings.EqualFold(previous, candidate) {
			candidate = "$any"
		}

		fieldComponents[key] = candidate
	}

	return fieldComponents
}

// injectedComponent is the component a WireBox injection names, when no
// beanPaths entry or propertyResolver said.
//
// `inject="id:settingService@contentbox"`, `inject="model:UserService"` and
// `inject="provider:UserService"` name the component UserService, which the
// resolver then finds by path and, failing that, by file name — WireBox's own
// convention is that a model's id is its file's. A provider stands in for the
// object it provides, so its methods are that object's. A dotted id is a path
// already.
//
// The DSL's namespaces name the frameworks' own objects, and are answered by
// their ColdBox dot-paths: `coldbox` the controller, `coldbox:requestService`
// a service, `wirebox:populator` the object populator, `logbox:logger:{this}`
// a logger, `cachebox:template` a cache — the provider ColdBox configures its
// caches with, since the interface they share declares less than each has. So
// are the models ColdBox registers as `Name@coldbox`, which a file-name search
// would find whatever the workspace calls. Without ColdBox's source or its
// stubs, a call on any of them is dynamic, not a component that is missing
// (IsSoftComponent). Whatever else the DSL names is not a component — `coldbox:setting:x`
// is a setting, `coldbox:moduleSettings:x` a struct — and resolving it by
// name would find an unrelated file.
func injectedComponent(inject string) string {
	id := strings.TrimSpace(inject)

	// A provider stands in for what it provides, DSL included:
	// `provider:cachebox` is the CacheFactory.
	if p := "provider:"; len(id) > len(p) && strings.EqualFold(id[:len(p)], p) {
		id = id[len(p):]
	}

	if c := dslComponent(id); c != "" {
		return c
	}

	for _, prefix := range []string{"id:", "model:"} {
		if len(id) > len(prefix) && strings.EqualFold(id[:len(prefix)], prefix) {
			id = id[len(prefix):]

			break
		}
	}

	name, module, qualified := strings.Cut(id, "@")
	if qualified {
		if c, found := coldboxModels[strings.ToLower(name)]; found && strings.EqualFold(module, "coldbox") {
			return coldboxSystem + c
		}
	}

	if name == "" || strings.ContainsAny(name, ":{}$#/\\ ") {
		return ""
	}

	var buf foldScratch
	switch string(buf.lowerFold(name)) {
	case "box", "executor", "java", "entityservice":
		return ""
	}

	// The module is kept: `X@cbstorages` is X as that module registers it,
	// which the resolver finds in the module's own models and, with no such
	// module in the workspace, takes as the module not being installed
	// (resolve.wireboxID).
	if qualified && module != "" && !strings.ContainsAny(module, ":{}$#/\\ ") {
		return name + "@" + module
	}

	return name
}

const coldboxSystem = "coldbox.system."

// injectionDSL maps a WireBox DSL string, lowercased, to the class it
// injects, under coldbox.system. Measured over the corpus: the namespaces a
// property uses and that hold a component.
var injectionDSL = map[string]string{
	"coldbox":                    "web.Controller",
	"coldbox:flash":              "web.flash.AbstractFlashScope",
	"coldbox:renderer":           "web.Renderer",
	"coldbox:requestcontext":     "web.context.RequestContext",
	"coldbox:router":             "web.routing.Router",
	"coldbox:requestservice":     "web.services.RequestService",
	"coldbox:interceptorservice": "web.services.InterceptorService",
	"coldbox:moduleservice":      "web.services.ModuleService",
	"coldbox:routingservice":     "web.services.RoutingService",
	"coldbox:handlerservice":     "web.services.HandlerService",
	"coldbox:loaderservice":      "web.services.LoaderService",
	"coldbox:schedulerservice":   "web.services.SchedulerService",
	"coldbox:asyncmanager":       "async.AsyncManager",
	"wirebox":                    "ioc.Injector",
	"wirebox:root":               "ioc.Injector",
	"wirebox:binder":             "ioc.config.Binder",
	"wirebox:populator":          "core.dynamic.ObjectPopulator",
	"wirebox:asyncmanager":       "async.AsyncManager",
	"cachebox":                   "cache.CacheFactory",
	"logbox":                     "logging.LogBox",
	"logbox:root":                "logging.Logger",
}

// coldboxModels are the models ColdBox registers under its own module, as
// `Name@coldbox`: a file-name search for them would find whatever the
// workspace holds by that name.
var coldboxModels = map[string]string{
	"htmlhelper":     "modules.HTMLHelper.models.HTMLHelper",
	"renderer":       "web.Renderer",
	"datamarshaller": "core.conversion.DataMarshaller",
	"xmlconverter":   "core.conversion.XMLConverter",
}

// isInjectedFrameworkComponent reports whether comp is one of
// InjectedFrameworkComponents.
func isInjectedFrameworkComponent(comp string) bool {
	if len(comp) <= len(coldboxSystem) || !strings.EqualFold(comp[:len(coldboxSystem)], coldboxSystem) {
		return false
	}

	injectedOnce.Do(func() {
		injected = map[string]bool{}
		for _, c := range InjectedFrameworkComponents() {
			injected[strings.ToLower(c)] = true
		}
	})

	return injected[strings.ToLower(comp)]
}

var (
	injectedOnce sync.Once
	injected     map[string]bool
)

// InjectedFrameworkComponents lists every framework class an injection can
// name, sorted: the bundled framework API must hold each of them.
func InjectedFrameworkComponents() []string {
	out := make([]string, 0, 2+len(injectionDSL)+len(coldboxModels))

	out = append(out, coldboxSystem+"logging.Logger", coldboxSystem+"cache.providers.CacheBoxColdBoxProvider")
	for _, c := range injectionDSL {
		out = append(out, coldboxSystem+c)
	}

	for _, c := range coldboxModels {
		out = append(out, coldboxSystem+c)
	}

	slices.Sort(out)

	return slices.Compact(out)
}

// InjectionDSL maps each WireBox DSL string that names one of ColdBox's
// classes, lowercased, to that class: what `getInstance( "logbox:root" )`
// returns as much as what `inject="logbox:root"` injects.
func InjectionDSL() map[string]string {
	out := make(map[string]string, len(injectionDSL))
	for k, c := range injectionDSL {
		out[k] = coldboxSystem + c
	}

	return out
}

// dslComponent is the framework class a DSL injection names, or "".
func dslComponent(id string) string {
	lower := strings.ToLower(id)
	if c, ok := injectionDSL[lower]; ok {
		return coldboxSystem + c
	}

	switch {
	case strings.HasPrefix(lower, "logbox:logger:"):
		return coldboxSystem + "logging.Logger"
	case strings.HasPrefix(lower, "cachebox:") && !strings.ContainsAny(lower[len("cachebox:"):], ":{}#"):
		return coldboxSystem + "cache.providers.CacheBoxColdBoxProvider"
	}

	return ""
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

// AllVars returns every variable declaration in the file, with its scope, line,
// and enclosing function range.
//
// Memoised behind the same lock and invalidated by the same edit paths as
// VariablesVars and ThisVars, because it is the same scan: go-to-definition on a
// variable asks for it once per request, and a request is cheap only if the
// answer is not a fresh parse of the whole document every time. The siblings
// return names; this returns the declarations, which is what a caller needing a
// line to jump to requires.
func (pr *ParseResult) AllVars() []VarDef {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if !pr.allVarsDone {
		pr.allVars = ParseVars(pr.Content)
		pr.allVarsDone = true
	}

	return pr.allVars
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

// AssignsMember reports whether the function holding line assigns
// variable.member on a line before it: `a.getVariables = getVariables;` then
// `a.getVariables()` calls what was stored there, which is no method of a's
// component. Compared case-insensitively, as CFML names are.
func (pr *ParseResult) AssignsMember(variable, member string, line uint32) bool {
	key := ""
	if s := findFuncScope(int(line), pr.Scopes); s.Start >= 0 {
		key = funcKey(s.Start, s.End)
	}

	for i := range pr.memberSets {
		m := &pr.memberSets[i]
		if m.funcKey == key && m.line <= line && strings.EqualFold(m.funcName, member) && strings.EqualFold(m.varName, variable) {
			return true
		}
	}

	return false
}

// HasScopedAssignment reports whether name was ever assigned in the given scope
// (ScopeVariables or ScopeThis) anywhere in the file — globally, inside init(), or
// inside any other function body. VARIABLES./THIS.-scoped values assigned inside one
// function remain visible from every other function in the file (unlike a var-scoped
// local), so telling "VARIABLES.someName(...) calls a property holding a function
// reference" apart from "no such property exists" requires checking every function
// body, not just VariablesVars/ThisVars (which only cover outside-function code and
// init()). Results are cached per scope on first call.
func (pr *ParseResult) HasScopedAssignment(scope Scope, name string) bool {
	names := pr.VariablesVars()
	if scope == ScopeThis {
		names = pr.ThisVars()
	}

	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}

	pr.mu.Lock()
	cached, ok := pr.anyScopedVars[scope]
	pr.mu.Unlock()

	if !ok {
		cached = pr.computeAnyScopedVars(scope)

		pr.mu.Lock()
		if pr.anyScopedVars == nil {
			pr.anyScopedVars = make(map[Scope][]string)
		}

		pr.anyScopedVars[scope] = cached
		pr.mu.Unlock()
	}

	for _, n := range cached {
		if strings.EqualFold(n, name) {
			return true
		}
	}

	return false
}

// computeAnyScopedVars scans every function body in the file for assignments in the
// given scope, so HasScopedAssignment can find a VARIABLES./THIS.-scoped assignment
// regardless of which function set it.
func (pr *ParseResult) computeAnyScopedVars(scope Scope) []string {
	seen := make(map[string]bool)

	var names []string

	for _, fs := range pr.Scopes {
		start, end := pr.lineOffsets(fs.Start, fs.End)
		if start < 0 {
			continue
		}

		body := pr.Content[start:end]

		regionKind := RegionScript

		for _, r := range pr.Regions {
			if r.StartLine <= fs.Start {
				regionKind = r.Kind
			}
		}

		var bodyVars []VarDef

		if regionKind == RegionScript {
			sp := newScriptParser(body, "", fs.Start, nil)
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
	}

	return names
}

// InvalidateFunc clears the body-dependent variable, ref and link caches.
func (pr *ParseResult) InvalidateFunc(funcStart, funcEnd int) {
	key := funcKey(funcStart, funcEnd)

	pr.funcVarsMu.Lock()
	delete(pr.funcVars, key)
	pr.funcVarsMu.Unlock()
	// Ref and link answers depend on the current body as well as its vars.
	pr.funcRefsMu.Lock()
	delete(pr.funcRefsMap, key)
	pr.memberSnapshot = nil
	delete(pr.funcLinksMap, key)
	pr.funcRefsMu.Unlock()
}

// parseFuncBody parses a single function body for variable declarations.
func (pr *ParseResult) parseFuncBody(funcStart, funcEnd int) (names []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Recovered(pr.log, "parse panic in parseFuncBody", r, "uri", string(pr.URI), "funcStart", funcStart)
		}
	}()

	start, end := pr.lineOffsets(funcStart, funcEnd)
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
		if r.Kind == RegionSkip {
			continue
		}

		var regionVars []VarDef

		if r.Kind == RegionScript {
			sp := newGlobalScriptParser(r.Text, r.StartLine, pr.Scopes)
			sp.parse()
			regionVars = sp.vars
		} else {
			tp := newTagParser(r.Text, "")
			tp.parse()

			for i := range tp.vars {
				tp.vars[i].Line += conv.Uint32(r.StartLine)
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

	start, end := pr.lineOffsets(initScope.Start, initScope.End)
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
	for i := range pr.Funcs {
		f := &pr.Funcs[i]

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

	for content != "" {
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

	for i := range allCalls {
		call := allCalls[i] // a copy: edited below, and kept

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
					Line:  conv.Uint32(lineNum),
					Start: conv.Uint32(start),
					End:   conv.Uint32(end),
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

	for content != "" {
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
						Start: conv.Uint32(start),
						End:   conv.Uint32(end),
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
//
// A ref a closure in the function declared is left out: it exists only inside
// the closure. FuncComponentRefsAt includes the ones in force at a line.
func (pr *ParseResult) FuncComponentRefs(funcStart, funcEnd int) []ComponentRef {
	refs, _ := pr.cachedFuncRefs(funcStart, funcEnd)

	scoped := 0

	for i := range refs {
		if refs[i].VisibleTo != 0 {
			scoped++
		}
	}

	if scoped == 0 {
		return refs
	}

	wide := make([]ComponentRef, 0, len(refs)-scoped)

	for i := range refs {
		if refs[i].VisibleTo == 0 {
			wide = append(wide, refs[i])
		}
	}

	return wide
}

// FuncComponentRefsAt returns the function's refs in force at line: the ones
// the whole function sees, and the ones declared by a closure holding line.
// The slice is the parse's own storage when no closure declared anything, so
// callers must not write to it.
func (pr *ParseResult) FuncComponentRefsAt(funcStart, funcEnd int, line uint32) []ComponentRef {
	refs, _ := pr.cachedFuncRefs(funcStart, funcEnd)

	for i := range refs {
		if !refs[i].VisibleAt(line) {
			out := slices.Clone(refs[:i])

			for j := i + 1; j < len(refs); j++ {
				if refs[j].VisibleAt(line) {
					out = append(out, refs[j])
				}
			}

			return out
		}
	}

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
	start, end := pr.lineOffsets(funcStart, funcEnd)
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
		sp.argumentTypes = pr.argumentTypeLookup()
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
		tp.argumentTypes = pr.argumentTypeLookup()
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
			refs[i].Line += conv.Uint32(funcStart)
		}

		for i := range links {
			links[i].Line += conv.Uint32(funcStart)
		}
	}

	// Scan for function calls (still line-based as it's only for exportDeps)
	if len(pr.findCalls) > 0 && !pr.extractCalls {
		lineNum := funcStart + 1

		scan := pr.Content[start:end]
		for scan != "" {
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

	// This re-parse never reaches applyChainedReturnLookup, so a chained
	// assignment is typed by its rest here or it keeps its first call's type.
	typ := dynamicIfTyped
	if pr.FuncLookup != nil {
		typ = pr.walkChainRest
	}

	for i := range refs {
		if len(refs[i].ChainRest) > 0 {
			refs[i].Component = strictChainComponent(&refs[i], typ(refs[i].Component, refs[i].ChainRest))
			refs[i].ChainRest = nil
		}
	}

	if pr.hasMemberBinding(body) {
		// A fresh full parse owns its slices and computes member identities once
		// per edited document, rather than repeating whole-file work per function.
		if pr.memberSnapshot == nil || pr.memberSnapshotContent != pr.Content {
			pr.memberSnapshot = ParseWithOptions(pr.URI, pr.Content, &ParseOptions{
				Resolvers: pr.Resolvers, PropertyResolvers: pr.PropertyResolvers,
				FuncLookup: pr.FuncLookup, BeanLookup: pr.BeanLookup,
				SetterLookup: pr.SetterLookup, ConstructorLookup: pr.ConstructorLookup,
				PropertyBeanLookup: pr.PropertyBeanLookup, BuiltinReturnLookup: pr.BuiltinReturnLookup,
				ExpressionMappings: pr.expressionMappings, ServicePropertyResolvers: pr.ServicePropertyResolvers,
			})
			pr.memberSnapshotContent = pr.Content
		}

		refs = slices.Clone(pr.memberSnapshot.funcRefsMap[funcKey(funcStart, funcEnd)])
	}

	return refs, links
}

// resolveMethodReturnRefs scans a function body for x = y.method() assignments
// where y has a known component ref and method declares a return type.
func (pr *ParseResult) resolveMethodReturnRefs(funcStart, funcEnd int, existingRefs []ComponentRef) []ComponentRef {
	start, end := pr.lineOffsets(funcStart, funcEnd)
	if start < 0 {
		return existingRefs
	}

	body := pr.Content[start:end]
	lineNum := funcStart + 1

	// Combine global + existing function refs for lookup
	allRefs := make([]ComponentRef, 0, len(pr.ComponentRefs)+len(existingRefs))
	allRefs = append(allRefs, pr.ComponentRefs...)
	allRefs = append(allRefs, existingRefs...)

	for body != "" {
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

		for i := range allRefs {
			ref := &allRefs[i]

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
		if _, after, ok := strings.CutLast(lhs, " "); ok {
			varName = strings.TrimSpace(after)
		}

		if dotIdx := strings.LastIndexByte(varName, '.'); dotIdx >= 0 {
			varName = varName[dotIdx+1:]
		}

		if varName != "" && isIdentifier(varName) {
			existingRefs = append(existingRefs, ComponentRef{
				Variable: varName, Component: retComp, URI: pr.URI, Line: conv.Uint32(lineNum),
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
	for i := range pr.Funcs {
		f := &pr.Funcs[i]

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

	return s != ""
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
//
// A function that calls nothing has no entry, and that is not the same question
// as "is this a function scope at all". This used to answer both by falling
// back to every call in the file, so a leaf method was handed its siblings'
// calls — `deps` drew an edge out of an empty function, labelled with another
// function's line. Callers wanting the whole file ask AllCalls for it.
func (pr *ParseResult) FuncCalls(funcStart, funcEnd int) []CallSite {
	if pr.extractCalls {
		return pr.funcCallsMap[funcKey(funcStart, funcEnd)]
	}

	return pr.funcCallsUncached(funcStart, funcEnd)
}

// AllCalls returns every call site in the file, wherever it was made: the ones
// outside any function and the ones inside each.
//
// This is what a whole-file scan wants — `unresolved`, `explain`, the code map
// and the MCP server each check or attribute every call in a file, and each
// used to ask FuncCalls for a line range covering the file and rely on the key
// missing. Requires ExtractCalls: true; without it there is nothing recorded to
// return.
func (pr *ParseResult) AllCalls() []CallSite {
	if !pr.extractCalls {
		return nil
	}

	n := len(pr.Calls)
	for _, calls := range pr.funcCallsMap {
		n += len(calls)
	}

	all := make([]CallSite, 0, n)
	all = append(all, pr.Calls...)

	for _, calls := range pr.funcCallsMap {
		all = append(all, calls...)
	}

	// The buckets come out of a map, so without this the order changes between
	// runs of one build — and every caller here writes a report a reader is
	// meant to diff against an earlier one.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Line != all[j].Line {
			return all[i].Line < all[j].Line
		}

		if all[i].Variable != all[j].Variable {
			return all[i].Variable < all[j].Variable
		}

		return all[i].FuncName < all[j].FuncName
	})

	return all
}

// funcCallsUncached parses a function body on-demand to extract call sites.
func (pr *ParseResult) funcCallsUncached(funcStart, funcEnd int) []CallSite {
	start, end := pr.lineOffsets(funcStart, funcEnd)
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
			calls[i].Line += conv.Uint32(funcStart)
		}
	}

	return calls
}

// callerAtLine returns the enclosing function name for a given line number.
// fillCallers names the enclosing function of every call recorded without one.
//
// A parser names a call's caller from the functions it has parsed itself, and
// a region cut from a function body holds none: the <cffunction> tag was in an
// earlier region. So the calls after a <cfscript> island or a <script> block in
// a tag function, and every #...# call inside such a block, came back with no
// caller, and the ARGUMENTS lookup for a function-reference call and the
// function column of find-references both lost them. The scopes are the
// file's, so the line answers it for every region alike.
func (pr *ParseResult) fillCallers() {
	fill := func(calls []CallSite) {
		for i := range calls {
			if calls[i].Caller == "" {
				calls[i].Caller = pr.callerAtLine(int(calls[i].Line))
			}
		}
	}

	fill(pr.Calls)

	for _, calls := range pr.funcCallsMap {
		fill(calls)
	}
}

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
			varName, comp := pr.qualifiedTarget(line, idx)

			pr.Calls = append(pr.Calls, CallSite{
				FuncName: target, Component: comp, Variable: varName, Line: conv.Uint32(lineNum), Caller: caller,
				Resolved: comp != "", Text: strings.TrimSpace(line),
			})
		} else if strings.Contains(lower, " "+t+"(") || strings.Contains(lower, "="+t+"(") || strings.HasPrefix(lower, t+"(") {
			pr.Calls = append(pr.Calls, CallSite{
				FuncName: target, Line: conv.Uint32(lineNum), Caller: caller,
				Resolved: false, Text: strings.TrimSpace(line),
			})
		}
	}
}

// qualifiedTarget reads the receiver of a call whose dot is at dot: the
// variable before it and its component, or, when the receiver is itself a
// call, the component a resolver gives that call.
func (pr *ParseResult) qualifiedTarget(line string, dot int) (varName, comp string) {
	varStart := dot - 1
	for varStart >= 0 && isLineIdentByte(line[varStart]) {
		varStart--
	}

	varName = line[varStart+1 : dot]
	comp = pr.resolveVarComponent(varName)

	// If no variable match, try resolving call expression before the dot
	if comp != "" || dot == 0 || line[dot-1] != ')' || len(pr.Resolvers) == 0 {
		return varName, comp
	}

	// Find matching open paren
	depth := 0

	for j := dot - 1; j >= 0; j-- {
		switch line[j] {
		case ')':
			depth++
		case '(':
			depth--
			if depth != 0 {
				continue
			}

			fnStart := j - 1
			for fnStart >= 0 && isLineIdentByte(line[fnStart]) {
				fnStart--
			}

			return varName, ResolveFromCall(line[fnStart+1:dot], pr.Resolvers)
		}
	}

	return varName, comp
}

// isLineIdentByte is the identifier test scanLineForCalls has always used:
// ASCII letters, digits, '_' and '$'.
func isLineIdentByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '$'
}

// resolveVarComponent finds the component a variable resolves to from pr.ComponentRefs.
func (pr *ParseResult) resolveVarComponent(varName string) string {
	// Check ComponentRefs — these are component-wide, always valid
	for i := range pr.ComponentRefs {
		ref := &pr.ComponentRefs[i]

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

// funcKey builds the "start:end" key the lazy per-function caches
// (funcVars/funcRefsMap/funcCallsMap/funcLinksMap) are stored under.
//
// It is on every per-keystroke path -- hover, definition and completion each
// reach FuncVars/FuncRefs, and FuncLinks calls it once per link -- so it is
// written to allocate exactly once, for the returned string. The obvious
// spelling, strings.Join([]string{itoa(start), itoa(end)}, ":"), costs four:
// a string from each itoa, the slice holding them, and the join's result. In a
// profile of the document-link handler that was 600,001 of its 630,284
// objects, 95% of everything it allocated.
//
// A struct key would allocate none at all, and is deliberately not used: the
// zero value of a struct{start, end int} is {0, 0}, which is a real function
// beginning on line 0, so it could not also mean "no enclosing function" the
// way the empty string does for every `inFunc == ""` test in the two parsers.
func funcKey(start, end int) string {
	// Two base-10 ints with sign, plus the separator.
	var buf [2*20 + 1]byte

	b := strconv.AppendInt(buf[:0], int64(start), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(end), 10)

	return string(b)
}

func atoi(s string) int {
	neg := false

	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}

	n := 0

	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}

	if neg {
		return -n
	}

	return n
}

// lineOffsets converts line numbers to byte offsets: start is where startLine
// begins and end is just past endLine's newline (or the end of the content).
// The line table is built once per content, since every function body's
// parse asks and walking from the top each time made a file with many
// functions quadratic in its length. Comparing the cached content is cheap:
// two strings sharing their bytes compare equal without reading them.
func (pr *ParseResult) lineOffsets(startLine, endLine int) (start, end int) {
	return offsetsFromTable(pr.lineStartsTable(), len(pr.Content), startLine, endLine)
}

// lineAt is the 0-based line holding byte offset off of Content, from the
// same table lineOffsets reads.
func (pr *ParseResult) lineAt(off int) int {
	starts := pr.lineStartsTable()

	return sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
}

func (pr *ParseResult) lineStartsTable() []int {
	pr.lineStartsMu.Lock()
	defer pr.lineStartsMu.Unlock()

	if pr.lineStarts == nil || pr.lineStartsContent != pr.Content {
		pr.lineStarts = lineStartTable(pr.Content)
		pr.lineStartsContent = pr.Content
	}

	return pr.lineStarts
}

func lineStartTable(content string) []int {
	starts := make([]int, 1, strings.Count(content, "\n")+1)

	for i := 0; ; {
		idx := strings.IndexByte(content[i:], '\n')
		if idx < 0 {
			return starts
		}

		i += idx + 1
		starts = append(starts, i)
	}
}

func offsetsFromTable(starts []int, size, startLine, endLine int) (start, end int) {
	if startLine < 0 {
		startLine = 0
	}

	if startLine >= len(starts) {
		return -1, -1
	}

	start = starts[startLine]

	switch {
	case endLine < startLine:
		end = start
	case endLine+1 < len(starts):
		end = starts[endLine+1]
	default:
		end = size
	}

	return start, end
}

func (pr *ParseResult) logDebug(msg string, keysAndValues ...any) {
	if pr.log != nil {
		pr.log.Debug(msg, keysAndValues...)
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
	var buf foldScratch

	switch string(buf.lowerFold(name)) {
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
		"getbytes", "tostring", "hashcode", "getclass", "init", "tobytearray", "tochararray",
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

// Declares reports whether the file declares name anywhere, case-insensitively:
// a variable in any scope, a property, a function or a function's argument.
// A name it does not declare is one it expects from elsewhere — its base
// component, an injector, the framework — which is what the resolver needs to
// know before blaming a missing base for a receiver with no component.
func (pr *ParseResult) Declares(name string) bool {
	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, name) {
			return true
		}

		for j := range pr.Funcs[i].Arguments {
			if strings.EqualFold(pr.Funcs[i].Arguments[j].Name, name) {
				return true
			}
		}
	}

	for i := range pr.Properties {
		if strings.EqualFold(pr.Properties[i].name, name) {
			return true
		}
	}

	for _, v := range pr.AllVars() {
		if strings.EqualFold(v.Name, name) {
			return true
		}
	}

	return false
}

// A script island or tag continuation has no opening function declaration of
// its own. Carry the enclosing typed arguments without publishing a duplicate
// definition; whole-argument assignments must see the same signature throughout.
func (pr *ParseResult) seedRegionFunction(start int, name string) (int, []FunctionDef) {
	for i := range pr.Funcs {
		fn := &pr.Funcs[i]
		if int(fn.Line) != start || !strings.EqualFold(fn.Name, name) {
			continue
		}

		for _, arg := range fn.Arguments {
			if argumentComponentType(&arg) != "" {
				return i, []FunctionDef{*fn}
			}
		}

		break
	}

	return -1, nil
}

// An island can begin on the opening tag's own line. The declaration must
// already have been parsed: a first tag region on that line is not a continuation.
func (pr *ParseResult) regionTagScope(scopes []FuncScope, line int) (FuncScope, bool) {
	if scope, ok := enclosingTagScope(scopes, line); ok {
		return scope, true
	}

	for _, scope := range scopes {
		if scope.Start != line {
			continue
		}

		for i := range pr.Funcs {
			fn := &pr.Funcs[i]
			if int(fn.Line) == line && strings.EqualFold(fn.Name, scope.Name) {
				return scope, true
			}
		}
	}

	return FuncScope{}, false
}

// prepareManagedSetterLookup reads properties before signatures so declaration
// order cannot change DI/1 eligibility. The metadata parse has no injection
// callbacks, and therefore does not recurse. Global edits rebuild this snapshot.
func (pr *ParseResult) prepareManagedSetterLookup() {
	pr.managedSetterLookup = pr.SetterLookup
	if pr.SetterLookup == nil || pr.PropertyBeanLookup == nil {
		return
	}

	metadata := pr.managedPropertyMetadata()

	properties := make(map[string]map[string]string, len(metadata.Properties))
	for _, prop := range metadata.Properties {
		if !metadata.Accessors && !metadata.Persistent {
			continue
		}

		properties[strings.ToLower(prop.name)] = prop.attrs
	}

	pr.managedSetterLookup = func(name string) string {
		attrs, found := properties[strings.ToLower(name)]
		// setter=false omits the implicit property setter, allowing an explicit one.
		if found && !strings.EqualFold(attrs["setter"], "false") && pr.PropertyBeanLookup(name, attrs) == "" {
			return ""
		}

		return pr.SetterLookup(name)
	}
}

func (pr *ParseResult) propertyBeanComponent(prop *propertyDef) string {
	if pr.PropertyBeanLookup != nil && !pr.Accessors && !pr.Persistent {
		return ""
	}

	if pr.BeanLookup != nil {
		lookup := pr.BeanLookup
		if pr.PropertyBeanLookup != nil {
			lookup = func(name string) string { return pr.PropertyBeanLookup(name, prop.attrs) }
		}

		if inject := prop.attrs["inject"]; inject != "" {
			if comp := lookup(normalizeBeanKey(inject)); comp != "" {
				return comp
			}

			if comp := lookup(extractBeanName(inject)); comp != "" {
				return comp
			}
		}

		if comp := lookup(prop.name); comp != "" {
			return comp
		}
	}

	if pr.PropertyBeanLookup != nil {
		return ""
	}

	inject, ok := prop.attrs["inject"]
	if ok && inject == "" {
		inject = prop.name
	}

	return injectedComponent(inject)
}

func (pr *ParseResult) applyManagedArgumentTypes(name, access string, args []Argument) {
	applySetterArgumentTypes(name, access, args, pr.managedSetterLookup)
	applyConstructorArgumentTypes(name, access, args, pr.ConstructorLookup)
}

func (pr *ParseResult) argumentTypeLookup() func(string, string, []Argument) {
	if pr.SetterLookup == nil && pr.ConstructorLookup == nil {
		return nil
	}

	return pr.applyManagedArgumentTypes
}

func callHopName(hop string) string {
	if expression, ok := CallExpression(hop); ok {
		name, _, _ := strings.Cut(expression, "(")
		_, name, _ = strings.CutLast("."+name, ".")

		return strings.TrimSpace(name)
	}

	return hop
}

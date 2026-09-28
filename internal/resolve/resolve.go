// Package resolve provides component and function resolution logic.
package resolve

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// Resolver resolves component dot-paths to files and functions.
type Resolver struct {
	FS                 vfs.FS
	WorkspaceFolders   []string
	Mappings           map[string]string
	ExpressionMappings map[string]string
	Index              *index.Index
	Resolvers          []parser.Resolver
	mu                 sync.RWMutex
	appRootCache       map[string]string // dir → Application.cfc root
	resolveCache       map[string]string // component+"\t"+baseDir → file path
	dirCache           *cfpath.DirCache  // directory listings behind those resolutions
	incGraph           *includeGraph     // the index's cfincludes, rebuilt when they change
	exprKeys           []string          // ExpressionMappings' keys in the order they apply
}

// describeResolver names the resolver at idx for trace output, so a wrong component can be
// traced back to the exact componentResolvers entry that produced it rather than just to
// "a componentResolver".
func (r *Resolver) describeResolver(idx int) string {
	if idx < 0 || idx >= len(r.Resolvers) {
		return "unknown resolver"
	}

	return r.Resolvers[idx].Describe()
}

// ComponentPath resolves a component dot-path to an absolute .cfc file path
// using the standard fallback chain: baseDir → Application.cfc root → workspace folders.
// If component contains pipe characters, each alternative is tried left-to-right.
func (r *Resolver) ComponentPath(component, baseDir string) string {
	// Apply expression mappings (replace runtime expressions with static values).
	// A key may list multiple pipe-delimited alternatives that all map to the same value.
	for _, key := range r.expressionKeys() {
		value := r.ExpressionMappings[key]

		for expr := range strings.SplitSeq(key, "|") {
			if expr != "" && strings.Contains(component, expr) {
				component = strings.ReplaceAll(component, expr, value)
			}
		}
	}

	key := component + "\t" + baseDir

	r.mu.RLock()

	if r.resolveCache != nil {
		if p, ok := r.resolveCache[key]; ok {
			r.mu.RUnlock()

			return p
		}
	}

	r.mu.RUnlock()

	var result string

	if strings.Contains(component, "|") {
		for alt := range strings.SplitSeq(component, "|") {
			if p := r.componentPathUncached(alt, baseDir); p != "" {
				result = p

				break
			}
		}
	} else {
		result = r.componentPathUncached(component, baseDir)
	}

	r.mu.Lock()
	if r.resolveCache == nil {
		r.resolveCache = make(map[string]string)
	}

	r.resolveCache[key] = result
	r.mu.Unlock()

	return result
}

// expressionKeys returns ExpressionMappings' keys in the order they apply
// (parser.ExpressionMappingOrder), worked out once: the resolver is dropped
// wherever its configuration changes.
func (r *Resolver) expressionKeys() []string {
	if len(r.ExpressionMappings) == 0 {
		return nil
	}

	r.mu.RLock()
	keys := r.exprKeys
	r.mu.RUnlock()

	if keys != nil {
		return keys
	}

	keys = parser.ExpressionMappingOrder(r.ExpressionMappings)

	r.mu.Lock()
	r.exprKeys = keys
	r.mu.Unlock()

	return keys
}

// dirs returns the resolver's directory-listing cache, creating it on first
// use. Its lifetime is the resolver's: the server drops the whole Resolver
// wherever it decides a path answer may have gone stale, which is what the
// cache needs and what resolveCache beside it already relies on.
func (r *Resolver) dirs() *cfpath.DirCache {
	r.mu.RLock()
	c := r.dirCache
	r.mu.RUnlock()

	if c != nil {
		return c
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dirCache == nil {
		r.dirCache = cfpath.NewDirCache()
	}

	return r.dirCache
}

func (r *Resolver) componentPathUncached(component, baseDir string) string {
	mappings := r.effectiveMappings(baseDir)
	dirs := r.dirs()

	if p := cfpath.ResolvePathCached(component, baseDir, mappings, dirs); p != "" {
		return p
	}

	if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
		if p := cfpath.ResolvePathCached(component, appDir, mappings, dirs); p != "" {
			return p
		}
	}

	for _, root := range r.WorkspaceFolders {
		if p := cfpath.ResolvePathCached(component, root, mappings, dirs); p != "" {
			return p
		}
	}

	// For bare names (no dots/slashes), search the index by filename as a last resort.
	// This handles `extends="BaseAssertionsTest"` where the file isn't in the same
	// directory or workspace root but is somewhere in the indexed workspace.
	if !strings.Contains(component, ".") && !strings.Contains(component, "/") && r.Index != nil {
		candidates := r.Index.FindFilesByBasename(component)
		if len(candidates) == 1 {
			return candidates[0]
		}

		if len(candidates) > 1 {
			// Multiple matches — pick the one with the shortest relative path
			// from baseDir, and the lowest path among equals, as
			// Index.LookupPreferred does for a function. Several copies of a
			// component in sibling directories — ContentBox keeps one
			// RailoDBInfo.cfc per patch — are all equally near a sibling that
			// has none, and taking whichever came first made the answer, and
			// every code-map edge through it, change from run to run.
			best := ""
			bestDist := -1

			for _, c := range candidates {
				rel, err := filepath.Rel(baseDir, c)
				if err != nil {
					continue
				}

				dist := strings.Count(rel, string(filepath.Separator))

				if bestDist < 0 || dist < bestDist || dist == bestDist && c < best {
					best = c
					bestDist = dist
				}
			}

			if best != "" {
				return best
			}
		}
	}

	return ""
}

// EnsureIndexed ensures a CFC file is indexed, loading from disk if needed.
//
// The "needed" test is whether the index holds the file (HasFile), not whether
// it holds any functions for it. Those differ for a component that legitimately
// declares none — a property-only bean, a DTO, a `this`-scope struct — and
// asking the second question re-read and re-parsed every such file on every
// single lookup, since re-indexing it produced the same empty result.
func (r *Resolver) EnsureIndexed(cfcPath string) []*parser.FunctionDef {
	cfcURI := cfpath.ToURI(cfcPath)

	if !r.Index.HasFile(cfcURI) {
		data, err := r.FS.ReadFile(cfcPath)
		if err != nil {
			return nil
		}

		r.Index.IndexFile(cfcURI, string(data))
	}

	return r.Index.FunctionsForFile(cfcURI)
}

// LookupFuncWithExtends searches for a function in cfcPath, walking the extends chain.
func (r *Resolver) LookupFuncWithExtends(cfcPath, funcName string) *parser.FunctionDef {
	seen := make(map[string]bool)
	for cfcPath != "" && !seen[cfcPath] {
		seen[cfcPath] = true
		cfcURI := cfpath.ToURI(cfcPath)

		defs := r.EnsureIndexed(cfcPath)
		for _, d := range defs {
			if strings.EqualFold(d.Name, funcName) {
				return d
			}
		}

		ext, ok := r.extendsOf(cfcPath, cfcURI)
		if !ok || ext == "" {
			break
		}

		baseDir := filepath.Dir(cfcPath)
		cfcPath = r.ComponentPath(ext, baseDir)
	}

	return nil
}

// extendsOf reports what cfcPath extends, reading and parsing the file only if
// the index cannot say.
//
// This used to read and fully parse the component every time, to take one
// field off the result and discard the rest. That is the single most expensive
// thing the open path did: LookupFuncWithExtends runs once per unresolved call
// site, so a component with hundreds of them re-read and re-parsed the same
// bases hundreds of times. On a 540KB service.cfc, textDocument/didOpen
// allocated 35.8MB and took 57.7ms; through the index it is 13.2MB and 16.7ms.
// The bare parse of that file is 2.25MB, which is what says the cost was never
// the parser.
//
// It is also why allocation tracked call sites rather than file size — a
// persist.cfc three times larger allocated a third as much per byte.
//
// Nothing needs to invalidate this. The record lives in the index and
// removeFileEntries drops it, so any re-index forgets it and the next call
// re-establishes it from the file as it then stands.
func (r *Resolver) extendsOf(cfcPath string, cfcURI uri.URI) (string, bool) {
	if ext, ok := r.Index.ExtendsForFile(cfcURI); ok {
		return ext, true
	}

	data, err := r.FS.ReadFile(cfcPath)
	if err != nil {
		return "", false
	}

	ext := parser.Parse(cfcURI, string(data)).Extends
	r.Index.SetExtends(cfcURI, ext)

	return ext, true
}

// ResolveFunc finds a function definition by component path and function name,
// handling pipe-separated alternatives, absolute paths, and the extends chain.
func (r *Resolver) ResolveFunc(component, funcName, baseDir string) *parser.FunctionDef {
	alternatives := []string{component}
	if strings.Contains(component, "|") {
		alternatives = strings.Split(component, "|")
	}

	for _, alt := range alternatives {
		var cfcPath string
		if filepath.IsAbs(alt) {
			cfcPath = alt
		} else {
			cfcPath = r.ComponentPath(alt, baseDir)
		}

		if cfcPath == "" {
			continue
		}

		if d := r.LookupFuncWithExtends(cfcPath, funcName); d != nil {
			return d
		}
	}

	return nil
}

// HasFunction returns true if the component has a function with the given name.
func (r *Resolver) HasFunction(component, funcName, baseDir string) bool {
	cfcPath := r.ComponentPath(component, baseDir)
	if cfcPath == "" {
		return false
	}

	for _, d := range r.EnsureIndexed(cfcPath) {
		if strings.EqualFold(d.Name, funcName) {
			return true
		}
	}

	return false
}

// FindApplicationRoot walks up from dir looking for Application.cfc or Application.cfm.
func (r *Resolver) FindApplicationRoot(dir string) string {
	r.mu.RLock()

	if r.appRootCache != nil {
		if v, ok := r.appRootCache[dir]; ok {
			r.mu.RUnlock()

			return v
		}
	}

	r.mu.RUnlock()

	result := r.findApplicationRootUncached(dir)

	r.mu.Lock()
	if r.appRootCache == nil {
		r.appRootCache = make(map[string]string)
	}

	r.appRootCache[dir] = result
	r.mu.Unlock()

	return result
}

func (r *Resolver) findApplicationRootUncached(dir string) string {
	d := dir

	for {
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			if _, err := r.FS.Stat(filepath.Join(d, name)); err == nil {
				return d
			}
		}

		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}

		d = parent
	}
}

// EffectiveMappings returns config mappings merged with Application.cfc mappings.
func (r *Resolver) EffectiveMappings(baseDir string) map[string]string {
	return r.effectiveMappings(baseDir)
}

func (r *Resolver) effectiveMappings(baseDir string) map[string]string {
	appDir := r.FindApplicationRoot(baseDir)
	if appDir == "" {
		return r.Mappings
	}

	appMappings := cfpath.LoadAppMappings(appDir)
	if len(appMappings) == 0 {
		return r.Mappings
	}

	if len(r.Mappings) == 0 {
		return appMappings
	}

	merged := make(map[string]string, len(appMappings)+len(r.Mappings))
	maps.Copy(merged, appMappings)

	maps.Copy(merged, r.Mappings)

	return merged
}

// ResolveFromCall resolves a call expression against configured resolvers.
func (r *Resolver) ResolveFromCall(expr string) string {
	return parser.ResolveFromCall(expr, r.Resolvers)
}

// CanResolveCall determines whether a function call can be resolved given the
// parse result context. Returns empty string if resolved, or a reason if not.
func (r *Resolver) CanResolveCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string) string {
	return r.canResolveCall(call, pr, baseDir, nil)
}

// ExplainCall runs the same resolution logic as CanResolveCall but also returns a
// human-readable trace of every decision point that produced (or failed to produce)
// the final verdict — which mechanism set the call's component, which componentResolver
// or FuncLookup hop fired, and why the final method check succeeded or failed. Intended
// for the `explain` CLI command; not used on the hot lint path.
func (r *Resolver) ExplainCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string) (string, []string) {
	tr := &callTrace{}
	reason := r.canResolveCall(call, pr, baseDir, tr)

	return reason, tr.steps
}

// callTrace accumulates what canResolveCall learned on the way to its verdict: a
// human-readable step list for [Resolver.ExplainCall], and the callee it landed on
// for [Resolver.ResolveCallTarget]. A nil *callTrace is always safe to call add or
// hit on (both no-op), so canResolveCall can be shared between the hot lint path
// (tr == nil) and its two introspecting callers without extra branching.
//
// Recording the target here rather than widening canResolveCall's return is
// deliberate: the function has twenty-odd `return ""` sites, and threading a second
// value through every one of them is exactly the kind of hand-maintained parallel
// list this codebase keeps getting bitten by. A recorder lets each site opt in where
// it already opts in to a trace step, and a site that forgets degrades to
// TargetNone — an edge the call graph reports as unresolved — rather than to a wrong
// edge. TestEveryAcceptPathRecordsATarget pins the set that must not forget.
type callTrace struct {
	steps  []string
	target CallTarget
}

func (t *callTrace) addf(format string, args ...any) {
	if t == nil {
		return
	}

	t.steps = append(t.steps, fmt.Sprintf(format, args...))
}

// hit records where a call site resolved to. def may be nil for the dynamic kinds,
// where there is a component (or not even that) but no definition to point at.
func (t *callTrace) hit(kind TargetKind, component string, def *parser.FunctionDef) {
	if t == nil {
		return
	}

	t.target.Kind = kind
	t.target.Component = component

	if def != nil {
		t.target.URI = def.URI
		t.target.FuncName = def.Name
		t.target.Line = def.Line
	}
}

func (r *Resolver) canResolveCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName := call.FuncName
	variable := call.Variable

	// Unqualified call — check same file, then extends chain.
	// Skip if call.Component is already set (e.g. resolved via chained new/createObject).
	if variable == "" && call.Component == "" {
		return r.resolveBareCall(call, pr, baseDir, tr)
	}

	// this. qualifier — refers to the current component
	if strings.EqualFold(variable, "this") {
		return r.resolveThisCall(funcName, pr, baseDir, tr)
	}

	// super. qualifier
	if strings.EqualFold(variable, "super") {
		return r.resolveSuperCall(funcName, pr, baseDir, tr)
	}

	// Bare VARIABLES qualifier (no further dot, e.g. "VARIABLES.someName(...)") —
	// a scope can't itself be a component ref, so treating "VARIABLES" as the thing
	// needing a ComponentRef is a category error. What's actually being called is a
	// property assigned onto the scope (e.g. "VARIABLES.someName = ARGUMENTS.callback;
	// ... VARIABLES.someName(...)"), a function-reference value passed in and stored —
	// same reasoning as the ARGUMENTS-as-function-reference case below, just for a
	// VARIABLES-scoped property instead of a bare argument. Accept if funcName was
	// ever assigned in VARIABLES scope anywhere in the file (assignments inside any
	// function are visible from every other function, unlike a var-scoped local).
	if strings.EqualFold(variable, "VARIABLES") {
		tr.addf("VARIABLES.%s called as a function reference — checking for a VARIABLES-scoped assignment", funcName)

		if pr.HasScopedAssignment(parser.ScopeVariables, funcName) {
			tr.addf("%q was assigned in VARIABLES scope — treating as a call through a function-reference property", funcName)
			tr.hit(TargetDynamic, "", nil)

			return ""
		}
	}

	// ARGUMENTS qualifier — funcName is being called as a function-reference argument.
	// These are dynamic (type="any"), so we can't verify them statically; accept if the
	// argument is declared in the enclosing function.
	if strings.EqualFold(variable, "ARGUMENTS") {
		tr.addf("ARGUMENTS.%s called as a function reference — checking caller %q's argument list", funcName, call.Caller)

		if declaresArgument(pr, call.Caller, funcName) {
			tr.hit(TargetDynamic, "", nil)

			return ""
		}
	}

	// Qualified call — find the component from refs
	comp := call.Component

	// softComp is a component a dynamicIfMissing resolver produced during this
	// resolution; the parse records its own (ParseResult.IsSoftComponent).
	softComp := ""

	if comp != "" {
		tr.addf("call.Component already set to %q (resolved earlier via chained new/createObject)", comp)
	}

	if comp == "" {
		var member bool

		comp, member = r.receiverComponent(variable, call.Line, call.Caller, funcName, pr, baseDir, tr)
		if member {
			tr.hit(TargetMember, "", nil)

			return ""
		}
	}

	if comp == "" {
		// Try component resolvers
		var noFollow, soft bool

		comp, noFollow, soft = r.matchResolver(variable, tr, func(c, desc string, nf bool) {
			tr.addf("resolved %q to %q via componentResolver matching the variable name [%s] (noFollow=%v)",
				variable, c, desc, nf)
		})
		if soft {
			softComp = comp
		}

		if noFollow && comp != "" {
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}
	}

	if comp == "" && call.Text != "" {
		// Try resolvers against the full line text (handles chained calls like x.method().prop.func())
		var noFollow, soft bool

		comp, noFollow, soft = r.matchResolver(call.Text, tr, func(c, desc string, nf bool) {
			tr.addf("resolved %q to %q via componentResolver matching the full line text %q [%s] (noFollow=%v)",
				variable, c, call.Text, desc, nf)
		})
		if soft {
			softComp = comp
		}

		if noFollow && comp != "" {
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}
	}

	if comp == "" {
		tr.addf("no ComponentRef and no componentResolver matched %q", variable)
	}

	// Walk any intermediate .method() hops between the resolved base and this
	// call (e.g. "kpg.generateKeyPair().getPublic().getParams()" — comp here is
	// kpg's own component; call.Chain lists "generateKeyPair", "getPublic", each
	// needing its own declared return type applied before checking funcName below).
	if comp != "" && comp != "$any" && !strings.HasPrefix(comp, "$builtin.") {
		for _, hop := range call.Chain {
			// A hop that returned "$any" makes the rest of the chain dynamic.
			// Walking on asked "$any" for the next method, which it can never
			// define: getSandBox("x").getAttendanceObj().getLog() was reported
			// as getAttendanceObj missing from $any. The accept below, after
			// the walk, is the answer for a dynamic receiver.
			if comp == "$any" {
				tr.addf("chain hop %q is on a dynamic ($any) value — the rest of the chain is dynamic", hop)

				break
			}

			fd := r.ResolveFunc(comp, hop, baseDir)
			if fd == nil {
				return r.missingChainHop(comp, softComp, hop, funcName, pr, baseDir, tr)
			}

			ret, noFollow, soft := r.chainHopReturn(comp, hop, fd, tr)
			if soft {
				softComp = ret
			}

			if noFollow && ret != "" {
				tr.hit(TargetDynamic, ret, nil)

				return ""
			}

			if ret == "" {
				return "method '" + hop + "' in " + comp + " has no component return type (chain to '" + funcName + "')"
			}

			comp = ret
		}
	}

	return r.checkMethodOn(comp, softComp, call, pr, baseDir, tr)
}

// resolveBareCall is canResolveCall for an unqualified call: this file, a
// function-reference local, the extends chain, cfinclude, and a
// variables-scope function reference.
func (r *Resolver) resolveBareCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName := call.FuncName

	tr.addf("unqualified call to %q — checking same file", funcName)

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, funcName) {
			tr.hit(TargetSameFile, "", &pr.Funcs[i])
			tr.addf("found %q defined in this file", funcName)

			return ""
		}
	}

	// A bare call whose name matches a declared local variable or argument in
	// the enclosing function is a call through a function-reference value (e.g.
	// "var columnfn = ARGUMENTS.extend[x].column; ... columnfn(...)"), not a call
	// to a missing/undefined function — CFML supports first-class function
	// values assigned to locals/arguments. This is a genuinely different case
	// from "no qualifier, not in file": the identifier IS declared, its value is
	// just dynamic (unknown until runtime), so — same reasoning as the
	// ARGUMENTS.x-as-function-reference case below — there's nothing further to
	// verify statically, and it should not be reported as if it were a missing
	// function.
	if scope := parser.FindFuncScopeAt(int(call.Line), pr.Scopes); scope.Start != -1 {
		for _, v := range pr.FuncVars(scope.Start, scope.End) {
			if strings.EqualFold(v, funcName) {
				tr.addf("%q is a declared local variable/argument in the enclosing function — treating as a call through a function-reference value, not a missing function", funcName)
				tr.hit(TargetDynamic, "", nil)

				return ""
			}
		}
	}

	// Check extends chain
	if pr.Extends != "" {
		tr.addf("not in this file — checking extends chain (%s)", pr.Extends)

		if def := r.ResolveFunc(pr.Extends, funcName, baseDir); def != nil {
			tr.hit(TargetExtends, pr.Extends, def)
			tr.addf("found %q in extends chain", funcName)

			return ""
		}
	}

	if def, via := r.findThroughIncludes(pr, funcName); def != nil {
		tr.hit(TargetInclude, "", def)
		tr.addf("found %q through cfinclude, in %s", funcName, via)

		return ""
	}

	// CFML looks a bare name up in the variables scope, so a bare call to
	// one the file assigns there — VARIABLES.render = ARGUMENTS.render —
	// is a call through that function-reference property, the same as the
	// qualified VARIABLES.render() accepted below. The parser records
	// VARIABLES.name() as a bare call wherever it reads it as a member of
	// this component, which is what left `#VARIABLES._renderTemplate()#`
	// in a string "no qualifier, not in file".
	if pr.HasScopedAssignment(parser.ScopeVariables, funcName) {
		tr.addf("%q is assigned in VARIABLES scope — a call through a function-reference property", funcName)
		tr.hit(TargetDynamic, "", nil)

		return ""
	}

	if pr.Extends != "" {
		if base := r.MissingBase(pr.Extends, baseDir); base != "" {
			return MissingBaseReason(base)
		}

		return "not found in extends chain"
	}

	return "no qualifier, not in file"
}

// resolveThisCall is canResolveCall for `this.name()`.
func (r *Resolver) resolveThisCall(funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	tr.addf("'this' qualifier — checking same file")

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, funcName) {
			tr.hit(TargetSameFile, "", &pr.Funcs[i])

			return ""
		}
	}

	if pr.Extends != "" {
		tr.addf("not in this file — checking extends chain (%s)", pr.Extends)

		if def := r.ResolveFunc(pr.Extends, funcName, baseDir); def != nil {
			tr.hit(TargetExtends, pr.Extends, def)

			return ""
		}
	}

	if def, via := r.findThroughIncludes(pr, funcName); def != nil {
		tr.hit(TargetInclude, "", def)
		tr.addf("found %q through cfinclude, in %s", funcName, via)

		return ""
	}

	if pr.Extends != "" {
		if base := r.MissingBase(pr.Extends, baseDir); base != "" {
			return MissingBaseReason(base)
		}
	}

	return "method '" + funcName + "' not found in current component"
}

// resolveSuperCall is canResolveCall for `super.name()`.
func (r *Resolver) resolveSuperCall(funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if pr.Extends == "" {
		return "super used but no extends"
	}

	tr.addf("'super' qualifier — checking extends chain (%s)", pr.Extends)

	if def := r.ResolveFunc(pr.Extends, funcName, baseDir); def != nil {
		tr.hit(TargetExtends, pr.Extends, def)

		return ""
	}

	if base := r.MissingBase(pr.Extends, baseDir); base != "" {
		return MissingBaseReason(base)
	}

	return "not found in parent component"
}

// missingBasePrefix starts MissingBaseReason's answer, which MissingBaseOf
// reads back.
const missingBasePrefix = "extends chain breaks at "

// MissingBaseReason is canResolveCall's answer for an inherited call when a
// component in the file's extends chain does not resolve. The method may well
// be declared there; what is missing is the base, and a report that blames the
// call names the wrong problem once per call — 56,000 times over the six-project
// corpus, for a dozen missing bases.
func MissingBaseReason(base string) string {
	return missingBasePrefix + "'" + base + "', which does not resolve"
}

// MissingBaseOf reports the base a MissingBaseReason names.
func MissingBaseOf(reason string) (string, bool) {
	rest, ok := strings.CutPrefix(reason, missingBasePrefix+"'")
	if !ok {
		return "", false
	}

	base, _, ok := strings.Cut(rest, "'")

	return base, ok
}

// inheritedFromMissingBase is the base a receiver with no component is taken
// to come from: the missing link of the file's extends chain, when the
// receiver is an unscoped or variables./this. name the file never declares.
// `print` in a CommandBox command and `$assert` in a TestBox spec are the
// base's, injected or declared there, and with the base missing they are as
// unchecked as its methods. A name in any other scope, or one the file
// declares, is not the base's to explain.
func (r *Resolver) inheritedFromMissingBase(variable string, line uint32, pr *parser.ParseResult, baseDir string) string {
	if pr.Extends == "" || variable == "" || strings.ContainsAny(variable, "[(") {
		return ""
	}

	root, rest, _ := strings.Cut(variable, ".")
	if strings.EqualFold(root, "variables") || strings.EqualFold(root, "this") {
		root, _, _ = strings.Cut(rest, ".")
	} else if isScopeName(root) {
		return ""
	}

	if root == "" || pr.Declares(root) {
		return ""
	}

	// A closure's locals and a catch variable are the enclosing function's
	// to the body scan, and not in the file-wide declarations.
	if scope := parser.FindFuncScopeAt(int(line), pr.Scopes); scope.Start != -1 {
		for _, v := range pr.FuncVars(scope.Start, scope.End) {
			if strings.EqualFold(v, root) {
				return ""
			}
		}
	}

	// And the body scan does not see a `var` inside an arrow function, or a
	// closure's parameters, which is most of a TestBox spec. Reading the
	// text for the shapes of a declaration errs towards "declared", which
	// only leaves the call reported as it was.
	if declaredInText(pr.Content, root) {
		return ""
	}

	return r.MissingBase(pr.Extends, baseDir)
}

// declaredInText reports whether content holds name, as a whole word, in the
// shape of a declaration: after `var`, before an `=` that is not `==`, as the
// variable of a catch, or alone between `(` or `,` and `)` or `,`, which is a
// parameter list — or an argument, which counts too.
func declaredInText(content, name string) bool {
	for from := 0; ; {
		i := indexWordFold(content[from:], name)
		if i < 0 {
			return false
		}

		i += from
		from = i + len(name)
		before := strings.TrimRight(content[:i], " \t\r\n")
		after := strings.TrimLeft(content[from:], " \t\r\n")

		switch {
		case hasSuffixFold(before, "var") && !isWordByte(before, len(before)-4):
			return true
		case strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "=="):
			return true
		case strings.HasPrefix(after, ")") && isCatchHead(before):
			return true
		case (strings.HasSuffix(before, "(") || strings.HasSuffix(before, ",")) &&
			(strings.HasPrefix(after, ")") || strings.HasPrefix(after, ",")):
			return true
		}
	}
}

// indexWordFold is the index of name in s as a whole identifier,
// ASCII case-insensitively; -1 when it is not there.
func indexWordFold(s, name string) int {
	for i := 0; i+len(name) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(name)], name) && !isWordByte(s, i-1) && !isWordByte(s, i+len(name)) {
			return i
		}
	}

	return -1
}

// isWordByte reports whether s[i] is part of an identifier; false out of
// range.
func isWordByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}

	c := s[i]

	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// isCatchHead reports whether before ends `catch ( type`, what precedes a
// catch's variable.
func isCatchHead(before string) bool {
	j := len(before)
	for j > 0 && (isWordByte(before, j-1) || before[j-1] == '.') {
		j--
	}

	head := strings.TrimRight(before[:j], " \t\r\n")
	if !strings.HasSuffix(head, "(") {
		return false
	}

	return hasSuffixFold(strings.TrimRight(head[:len(head)-1], " \t\r\n"), "catch")
}

// isScopeName reports whether name is a CFML scope other than variables and
// this, whose members a base component does not supply.
func isScopeName(name string) bool {
	switch strings.ToLower(name) {
	case "arguments", "local", "request", "session", "application", "server",
		"url", "form", "cgi", "cookie", "client", "super", "attributes", "caller", "thread":
		return true
	default:
		return false
	}
}

// MissingBase returns the first component in the extends chain starting at
// extends, from a file in baseDir, that does not resolve to a file; "" when
// every link resolves, or the chain loops.
func (r *Resolver) MissingBase(extends, baseDir string) string {
	seen := make(map[string]bool)

	for extends != "" && !strings.Contains(extends, "|") {
		p := r.ComponentPath(extends, baseDir)
		if p == "" {
			return extends
		}

		if seen[p] {
			return ""
		}

		seen[p] = true

		ext, ok := r.extendsOf(p, cfpath.ToURI(p))
		if !ok {
			return ""
		}

		extends, baseDir = ext, filepath.Dir(p)
	}

	return ""
}

// matchResolver tries the componentResolvers against text. It reports the
// component, the resolver's noFollow, and whether that resolver is
// dynamicIfMissing; describe is told about a match, for the trace.
func (r *Resolver) matchResolver(text string, tr *callTrace, describe func(comp, desc string, noFollow bool)) (comp string, noFollow, soft bool) {
	comp, noFollow, idx := parser.ResolveFromCallMatch(text, r.Resolvers)
	soft = idx >= 0 && r.Resolvers[idx].DynamicIfMissing

	if comp != "" && tr != nil {
		describe(comp, r.describeResolver(idx), noFollow)
	}

	return comp, noFollow, soft
}

// missingChainHop is canResolveCall's answer when a hop of a chained call is
// not a method of the component the chain has reached.
func (r *Resolver) missingChainHop(comp, softComp, hop, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if !r.componentExists(comp, baseDir) {
		if softMissing(comp, softComp, pr) {
			tr.addf("%q names no file and came from a dynamicIfMissing resolver — the rest of the chain is dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return "component '" + comp + "' does not exist (chain hop '" + hop + "' to '" + funcName + "')"
	}

	return "method '" + hop + "' not found in " + comp + " (chain to '" + funcName + "')"
}

// chainHopReturn is the component a chain hop's method returns: its declared
// or inferred component, a dotted return type, the receiver for an untyped
// init(), or a componentResolver matching the hop. It reports the resolver's
// noFollow and whether the resolver is dynamicIfMissing, for the last.
func (r *Resolver) chainHopReturn(comp, hop string, fd *parser.FunctionDef, tr *callTrace) (ret string, noFollow, soft bool) {
	ret = fd.ReturnComponent
	if ret != "" {
		tr.addf("chain hop %q on %q: declared/inferred ReturnComponent %q", hop, comp, ret)
	} else if fd.ReturnType != "" && strings.Contains(fd.ReturnType, ".") {
		ret = fd.ReturnType

		tr.addf("chain hop %q on %q: using dotted ReturnType %q", hop, comp, ret)
	}

	// An init() that declares nothing returns the object it was called
	// on: the CFC constructor convention, and what the parser assumes
	// when it types a variable assigned through one. Falling through to
	// the resolvers offered them init(), which nothing can answer.
	if ret == "" && strings.EqualFold(hop, "init") {
		ret = comp

		tr.addf("chain hop %q on %q: an untyped init() returns the object it is called on", hop, comp)
	}

	if ret != "" {
		return ret, false, false
	}

	// The real function's declared return type isn't a component (e.g.
	// a generic returntype="struct" on a factory method that actually
	// returns a specific component instance) — fall back to a
	// componentResolver matching the hop's own call shape, same as the
	// non-chain "altComp" fallback below.
	return r.matchResolver(hop+"()", tr, func(c, _ string, nf bool) {
		tr.addf("chain hop %q on %q: no declared return type — componentResolver matched %q(): %q (noFollow=%v)", hop, comp, hop, c, nf)
	})
}

// checkMethodOn is canResolveCall's last step: whether the component the
// receiver resolved to defines the method, with the dynamic, builtin,
// member-method, onMissingMethod and altComp fallbacks.
func (r *Resolver) checkMethodOn(comp, softComp string, call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName, variable := call.FuncName, call.Variable

	if comp == "" {
		if base := r.inheritedFromMissingBase(variable, call.Line, pr, baseDir); base != "" {
			return MissingBaseReason(base)
		}

		return "variable '" + variable + "' has no component ref"
	}

	// Dynamic return type — method called on a result of a function returning "any".
	if comp == "$any" {
		tr.addf("component is $any (dynamic) — accepted without a method check")
		tr.hit(TargetDynamic, "$any", nil)

		return ""
	}

	// Builtin return type — check method exists in-memory
	if strings.HasPrefix(comp, "$builtin.") {
		builtinName := comp[9:]
		if docs.LookupBuiltinMethod(builtinName, funcName) {
			tr.hit(TargetBuiltin, comp, nil)

			return ""
		}

		return "method '" + funcName + "' not found on builtin " + builtinName
	}

	tr.addf("checking whether %q defines method %q", comp, funcName)

	if def := r.ResolveFunc(comp, funcName, baseDir); def != nil {
		tr.hit(TargetComponent, comp, def)

		return ""
	}

	if parser.IsMemberMethod(funcName) {
		tr.addf("%q is a known member/Java-interop method — accepted without finding it in %q", funcName, comp)
		tr.hit(TargetMember, comp, nil)

		return ""
	}

	// Component defines onMissingMethod — any method call is valid.
	if r.ResolveFunc(comp, "onMissingMethod", baseDir) != nil {
		tr.addf("%q defines onMissingMethod — any method call accepted", comp)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	// The ref-derived component didn't have the method. Try the variable-name resolver
	// as a fallback — a pendingCall propagation may have assigned the wrong component
	// (e.g. var objFile = _parent.getFile() inherits _parent's component, but the
	// "objFile" resolver names the real type).
	if altComp, altNoFollow := parser.ResolveFromCallFull(variable, r.Resolvers); altComp != "" && altComp != comp {
		tr.addf("%q not found in %q — trying altComp fallback: componentResolver on variable name gives %q (noFollow=%v)", funcName, comp, altComp, altNoFollow)

		if altNoFollow {
			tr.hit(TargetDynamic, altComp, nil)

			return ""
		}

		if altDef := r.ResolveFunc(altComp, funcName, baseDir); altDef != nil {
			tr.hit(TargetComponent, altComp, altDef)

			return ""
		}
	}

	// A component that names no file is a different finding from a method a
	// real component lacks, and reporting it as the second hid it. It is
	// almost always a componentResolver producing a path — a broad get$1()
	// catch-all turning getInjectorController() into tass.injectorcontroller —
	// so saying so points at the config rather than at the code.
	if !r.componentExists(comp, baseDir) {
		// Unless a resolver marked dynamicIfMissing produced it: that is a
		// broad pattern's guess, and a guess that names nothing says the value
		// is not one of the components it was written for — not that one is
		// missing. A component named any other way is still reported.
		if softMissing(comp, softComp, pr) {
			tr.addf("%q names no file and came from a dynamicIfMissing resolver — accepted as dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return "component '" + comp + "' does not exist (calling '" + funcName + "')"
	}

	return "method '" + funcName + "' not found in " + comp
}

// ComponentOf reports the component variable holds at line, or "" when that
// is not known or is not a component. It is the answer go-to-type-definition
// wants, and it runs the receiver lookup CanResolveCall runs, so the two cannot
// disagree about what a variable holds.
//
// Two of CanResolveCall's fallbacks are left out on purpose. A
// componentResolver is tried against the variable name, but not against the
// whole line, which exists for chained calls and would answer for whatever
// else the line holds. And "$any" and "$builtin." are not components, so they
// are reported as unknown.
func (r *Resolver) ComponentOf(variable string, line uint32, pr *parser.ParseResult, baseDir string) string {
	caller := parser.FindFuncScopeAt(int(line), pr.Scopes).Name

	comp, _ := r.receiverComponent(variable, line, caller, "", pr, baseDir, nil)

	if comp == "" {
		comp, _, _ = parser.ResolveFromCallMatch(variable, r.Resolvers)
	}

	if comp == "$any" || strings.HasPrefix(comp, "$builtin.") {
		return ""
	}

	return comp
}

// receiverComponent finds the component a qualified call's receiver holds at
// line: a ref scoped to the enclosing function, the nearest preceding ref in the
// file, an Application.cfc ref, an ARGUMENTS.x type, and refs up the extends
// chain, in that order. It is canResolveCall's lookup, shared with
// ComponentOf so that go-to-type-definition answers exactly as call
// resolution does; the componentResolver fallbacks stay with each caller.
//
// member reports the one case that is not a component at all: an ARGUMENTS.x
// of primitive type calling a known member method, which canResolveCall
// accepts outright. It needs funcName; ComponentOf passes none.
func (r *Resolver) receiverComponent(variable string, line uint32, caller, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) (comp string, member bool) {
	// Strip scope prefix for matching (VARIABLES.x -> x). Bracket-aware: a "."
	// inside a "[...]" subscript (e.g. "linkMap[arguments.startSource]") is not a
	// scope prefix and must not be stripped there.
	lookupVar := parser.StripReceiverScope(variable)

	// Each lookup below is tried while comp is still empty. A matching ref
	// with no component is logged and passed over, as it always was.

	// Try function-scoped refs first
	if ref := funcScopedRef(pr, line, lookupVar); ref != nil {
		comp = ref.Component

		tr.addf("resolved %q to %q via function-scoped ComponentRef", variable, comp)
	}

	if comp == "" {
		if best := fileLevelRef(pr, line, lookupVar); best != nil {
			comp = best.Component

			tr.addf("resolved %q to %q via file-level ComponentRef (nearest preceding assignment at line %d)", variable, comp, best.Line+1)
		}
	}

	// Fall back to Application.cfc component refs
	if comp == "" {
		for _, hit := range r.applicationRefs(baseDir, lookupVar) {
			comp = hit.ref.Component

			tr.addf("resolved %q to %q via %s ComponentRef", variable, comp, hit.file)

			if comp != "" {
				break
			}
		}
	}

	// ARGUMENTS.x qualifier — resolve directly from the enclosing function's argument list.
	// This handles cases where the argument has a component type (via hint promotion or
	// explicit type) without requiring a ComponentRef to have been created.
	if comp == "" && strings.HasPrefix(strings.ToUpper(variable), "ARGUMENTS.") {
		argName := variable[10:]

		if arg := argumentOf(pr, caller, argName); arg != nil {
			if strings.Contains(arg.Type, ".") {
				comp = arg.Type

				tr.addf("resolved %q to %q via <cfargument type>", variable, comp)
			} else if parser.IsMemberMethod(funcName) {
				// Primitive-typed argument (string/numeric/array/etc.)
				// calling a known member/Java-interop method (e.g.
				// a string argument's .toCharArray()) — no component
				// is needed to verify it.
				tr.addf("ARGUMENTS.%s has primitive type %q, but %q is a known member method — accepted without a component", argName, arg.Type, funcName)

				return "", true
			}
		}
	}

	// Fall back to extends chain component refs (e.g. variables.$assert assigned in a parent)
	if comp == "" && pr.Extends != "" {
		tr.addf("no ref found in this file — checking extends chain (%s) for a ComponentRef", pr.Extends)

		r.walkExtendsRefs(pr.Extends, baseDir, lookupVar, func(ref *parser.ComponentRef, parent string) bool {
			comp = ref.Component

			tr.addf("resolved %q to %q via ComponentRef in parent %s", variable, comp, parent)

			return comp != ""
		})
	}

	return comp, false
}

// funcScopedRef is the first ref for name inside the function enclosing line.
func funcScopedRef(pr *parser.ParseResult, line uint32, name string) *parser.ComponentRef {
	for _, scope := range pr.Scopes {
		if int(line) < scope.Start || int(line) > scope.End {
			continue
		}

		refs := pr.FuncComponentRefs(scope.Start, scope.End)

		for i := range refs {
			if strings.EqualFold(refs[i].Variable, name) {
				return &refs[i]
			}
		}

		return nil
	}

	return nil
}

// fileLevelRef is the file-level ref for name that is in force at line.
//
// A scratch variable can be reassigned multiple times in the same file (e.g.
// once per <cfswitch>/<cfcase> branch) — using the first matching ref in file
// order would lock onto whichever branch happens to appear earliest,
// regardless of which branch the call site is actually in. Prefer the ref
// with the highest line number at or before the call site (the assignment
// that's actually in scope there); only fall back to file order for a genuine
// forward reference, where no preceding ref exists.
func fileLevelRef(pr *parser.ParseResult, line uint32, name string) *parser.ComponentRef {
	var best *parser.ComponentRef

	for i := range pr.ComponentRefs {
		ref := &pr.ComponentRefs[i]
		if !strings.EqualFold(ref.Variable, name) || ref.Line > line {
			continue
		}

		if best == nil || ref.Line > best.Line {
			best = ref
		}
	}

	if best != nil {
		return best
	}

	for i := range pr.ComponentRefs {
		if strings.EqualFold(pr.ComponentRefs[i].Variable, name) {
			return &pr.ComponentRefs[i]
		}
	}

	return nil
}

// appRef is a ref for a name in one of the Application files.
type appRef struct {
	ref  *parser.ComponentRef
	file string
}

// applicationRefs is the first ref for name in the governing Application.cfc
// and then Application.cfm, for each that has one.
func (r *Resolver) applicationRefs(baseDir, name string) []appRef {
	appDir := r.FindApplicationRoot(baseDir)
	if appDir == "" {
		return nil
	}

	var out []appRef

	for _, appName := range []string{"Application.cfc", "Application.cfm"} {
		appURI := cfpath.ToURI(filepath.Join(appDir, appName))
		for _, ref := range r.Index.RefsForFile(appURI) {
			if strings.EqualFold(ref.Variable, name) {
				out = append(out, appRef{ref: ref, file: appName})

				break
			}
		}
	}

	return out
}

// declaresArgument reports whether any function called caller declares an
// argument called name.
func declaresArgument(pr *parser.ParseResult, caller, name string) bool {
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if !strings.EqualFold(f.Name, caller) {
			continue
		}

		for _, arg := range f.Arguments {
			if strings.EqualFold(arg.Name, name) {
				return true
			}
		}
	}

	return false
}

// argumentOf is the argument called name of the function called caller, or
// nil. Only the first function with that name is consulted.
func argumentOf(pr *parser.ParseResult, caller, name string) *parser.Argument {
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if !strings.EqualFold(f.Name, caller) {
			continue
		}

		for j := range f.Arguments {
			if strings.EqualFold(f.Arguments[j].Name, name) {
				return &f.Arguments[j]
			}
		}

		return nil
	}

	return nil
}

// walkExtendsRefs walks the extends chain from extends, handing visit the
// first ref for name in each ancestor that has one, until visit returns true.
func (r *Resolver) walkExtendsRefs(extends, baseDir, name string, visit func(ref *parser.ComponentRef, parent string) bool) {
	seen := make(map[string]bool)

	for extends != "" && !seen[extends] {
		seen[extends] = true

		cfcPath := r.ComponentPath(extends, baseDir)
		if cfcPath == "" {
			return
		}

		parentURI := cfpath.ToURI(cfcPath)

		// Ensure the parent is indexed so RefsForFile returns its component refs.
		// (EnsureIndexed is a fast no-op if already indexed.)
		r.EnsureIndexed(cfcPath)

		for _, ref := range r.Index.RefsForFile(parentURI) {
			if strings.EqualFold(ref.Variable, name) {
				if visit(ref, extends) {
					return
				}

				break
			}
		}

		// Walk up the extends chain
		data, err := r.FS.ReadFile(cfcPath)
		if err != nil {
			return
		}

		extends = parser.Parse(parentURI, string(data)).Extends
	}
}

// softMissing reports whether comp, which names no file, came from a
// dynamicIfMissing resolver: during this resolution or during the parse.
func softMissing(comp, softComp string, pr *parser.ParseResult) bool {
	return (softComp != "" && strings.EqualFold(comp, softComp)) || pr.IsSoftComponent(comp)
}

// componentExists reports whether component, or any of its pipe-delimited
// alternatives, names a file. A dynamic or builtin marker counts as existing:
// it names no file by design.
func (r *Resolver) componentExists(component, baseDir string) bool {
	for alt := range strings.SplitSeq(component, "|") {
		switch {
		case alt == "":
			continue
		case strings.HasPrefix(alt, "$"):
			return true
		case filepath.IsAbs(alt):
			for _, p := range []string{alt, alt + ".cfc"} {
				if info, err := r.FS.Stat(p); err == nil && !info.IsDir() {
					return true
				}
			}
		}

		if r.ComponentPath(alt, baseDir) != "" {
			return true
		}
	}

	return false
}

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
	for key, value := range r.ExpressionMappings {
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

func (r *Resolver) componentPathUncached(component, baseDir string) string {
	mappings := r.effectiveMappings(baseDir)
	if p := cfpath.ResolvePath(component, baseDir, mappings); p != "" {
		return p
	}

	if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
		if p := cfpath.ResolvePath(component, appDir, mappings); p != "" {
			return p
		}
	}

	for _, root := range r.WorkspaceFolders {
		if p := cfpath.ResolvePath(component, root, mappings); p != "" {
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
			// Multiple matches — pick the one with the shortest relative path from baseDir.
			best := ""
			bestDist := -1

			for _, c := range candidates {
				rel, err := filepath.Rel(baseDir, c)
				if err != nil {
					continue
				}

				dist := strings.Count(rel, string(filepath.Separator))

				if bestDist < 0 || dist < bestDist {
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
func (r *Resolver) EnsureIndexed(cfcPath string) []*parser.FunctionDef {
	cfcURI := cfpath.ToURI(cfcPath)

	defs := r.Index.FunctionsForFile(cfcURI)
	if len(defs) == 0 {
		data, err := r.FS.ReadFile(cfcPath)
		if err != nil {
			return nil
		}

		r.Index.IndexFile(cfcURI, string(data))
		defs = r.Index.FunctionsForFile(cfcURI)
	}

	return defs
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

		// Get extends from parse result
		data, err := r.FS.ReadFile(cfcPath)
		if err != nil {
			break
		}

		pr := parser.Parse(cfcURI, string(data))
		if pr.Extends == "" {
			break
		}

		baseDir := filepath.Dir(cfcPath)
		cfcPath = r.ComponentPath(pr.Extends, baseDir)
	}

	return nil
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
func (r *Resolver) CanResolveCall(call parser.CallSite, pr *parser.ParseResult, baseDir string) string {
	return r.canResolveCall(call, pr, baseDir, nil)
}

// ExplainCall runs the same resolution logic as CanResolveCall but also returns a
// human-readable trace of every decision point that produced (or failed to produce)
// the final verdict — which mechanism set the call's component, which componentResolver
// or FuncLookup hop fired, and why the final method check succeeded or failed. Intended
// for the `explain` CLI command; not used on the hot lint path.
func (r *Resolver) ExplainCall(call parser.CallSite, pr *parser.ParseResult, baseDir string) (string, []string) {
	tr := &traceRecorder{}
	reason := r.canResolveCall(call, pr, baseDir, tr)

	return reason, tr.steps
}

// traceRecorder accumulates human-readable resolution steps. A nil *traceRecorder
// is always safe to call add on (no-op), so canResolveCall can be shared between the
// hot lint path (tr == nil) and ExplainCall (tr != nil) without extra branching.
type traceRecorder struct {
	steps []string
}

func (t *traceRecorder) add(format string, args ...any) {
	if t == nil {
		return
	}

	t.steps = append(t.steps, fmt.Sprintf(format, args...))
}

func (r *Resolver) canResolveCall(call parser.CallSite, pr *parser.ParseResult, baseDir string, tr *traceRecorder) string {
	funcName := call.FuncName
	variable := call.Variable

	// Unqualified call — check same file, then extends chain.
	// Skip if call.Component is already set (e.g. resolved via chained new/createObject).
	if variable == "" && call.Component == "" {
		tr.add("unqualified call to %q — checking same file", funcName)

		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, funcName) {
				tr.add("found %q defined in this file", funcName)

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
					tr.add("%q is a declared local variable/argument in the enclosing function — treating as a call through a function-reference value, not a missing function", funcName)

					return ""
				}
			}
		}

		// Check extends chain
		if pr.Extends != "" {
			tr.add("not in this file — checking extends chain (%s)", pr.Extends)

			if r.ResolveFunc(pr.Extends, funcName, baseDir) != nil {
				tr.add("found %q in extends chain", funcName)

				return ""
			}

			return "not found in extends chain"
		}

		return "no qualifier, not in file"
	}

	// this. qualifier — refers to the current component
	if strings.EqualFold(variable, "this") {
		tr.add("'this' qualifier — checking same file")

		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, funcName) {
				return ""
			}
		}

		if pr.Extends != "" {
			tr.add("not in this file — checking extends chain (%s)", pr.Extends)

			if r.ResolveFunc(pr.Extends, funcName, baseDir) != nil {
				return ""
			}
		}

		return "method '" + funcName + "' not found in current component"
	}

	// super. qualifier
	if strings.EqualFold(variable, "super") {
		if pr.Extends == "" {
			return "super used but no extends"
		}

		tr.add("'super' qualifier — checking extends chain (%s)", pr.Extends)

		if r.ResolveFunc(pr.Extends, funcName, baseDir) != nil {
			return ""
		}

		return "not found in parent component"
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
		tr.add("VARIABLES.%s called as a function reference — checking for a VARIABLES-scoped assignment", funcName)

		if pr.HasScopedAssignment(parser.ScopeVariables, funcName) {
			tr.add("%q was assigned in VARIABLES scope — treating as a call through a function-reference property", funcName)

			return ""
		}
	}

	// ARGUMENTS qualifier — funcName is being called as a function-reference argument.
	// These are dynamic (type="any"), so we can't verify them statically; accept if the
	// argument is declared in the enclosing function.
	if strings.EqualFold(variable, "ARGUMENTS") {
		tr.add("ARGUMENTS.%s called as a function reference — checking caller %q's argument list", funcName, call.Caller)

		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, call.Caller) {
				for _, arg := range f.Arguments {
					if strings.EqualFold(arg.Name, funcName) {
						return ""
					}
				}
			}
		}
	}

	// Qualified call — find the component from refs
	comp := call.Component
	if comp != "" {
		tr.add("call.Component already set to %q (resolved earlier via chained new/createObject)", comp)
	}

	if comp == "" {
		// Strip scope prefix for matching (VARIABLES.x -> x). Bracket-aware: a "."
		// inside a "[...]" subscript (e.g. "linkMap[arguments.startSource]") is not a
		// scope prefix and must not be stripped there.
		lookupVar := parser.StripReceiverScope(variable)

		// Try function-scoped refs first
		for _, scope := range pr.Scopes {
			if int(call.Line) >= scope.Start && int(call.Line) <= scope.End {
				for _, ref := range pr.FuncComponentRefs(scope.Start, scope.End) {
					if strings.EqualFold(ref.Variable, lookupVar) {
						comp = ref.Component

						tr.add("resolved %q to %q via function-scoped ComponentRef", variable, comp)

						break
					}
				}

				break
			}
		}

		// Fall back to component refs. A scratch variable can be reassigned multiple
		// times in the same file (e.g. once per <cfswitch>/<cfcase> branch) — using
		// the first matching ref in file order would lock onto whichever branch
		// happens to appear earliest, regardless of which branch the call site is
		// actually in. Prefer the ref with the highest line number at or before the
		// call site (the assignment that's actually in scope there); only fall back
		// to file order for a genuine forward reference, where no preceding ref exists.
		if comp == "" {
			var best *parser.ComponentRef

			for i := range pr.ComponentRefs {
				ref := &pr.ComponentRefs[i]
				if !strings.EqualFold(ref.Variable, lookupVar) {
					continue
				}

				if ref.Line > call.Line {
					continue
				}

				if best == nil || ref.Line > best.Line {
					best = ref
				}
			}

			if best == nil {
				for i := range pr.ComponentRefs {
					ref := &pr.ComponentRefs[i]
					if strings.EqualFold(ref.Variable, lookupVar) {
						best = ref

						break
					}
				}
			}

			if best != nil {
				comp = best.Component

				tr.add("resolved %q to %q via file-level ComponentRef (nearest preceding assignment at line %d)", variable, comp, best.Line+1)
			}
		}

		// Fall back to Application.cfc component refs
		if comp == "" {
			if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
				for _, appName := range []string{"Application.cfc", "Application.cfm"} {
					appURI := cfpath.ToURI(filepath.Join(appDir, appName))
					for _, ref := range r.Index.RefsForFile(appURI) {
						if strings.EqualFold(ref.Variable, lookupVar) {
							comp = ref.Component

							tr.add("resolved %q to %q via %s ComponentRef", variable, comp, appName)

							break
						}
					}

					if comp != "" {
						break
					}
				}
			}
		}

		// ARGUMENTS.x qualifier — resolve directly from the enclosing function's argument list.
		// This handles cases where the argument has a component type (via hint promotion or
		// explicit type) without requiring a ComponentRef to have been created.
		if comp == "" && strings.HasPrefix(strings.ToUpper(variable), "ARGUMENTS.") {
			argName := variable[10:]

			for _, f := range pr.Funcs {
				if strings.EqualFold(f.Name, call.Caller) {
					for _, arg := range f.Arguments {
						if !strings.EqualFold(arg.Name, argName) {
							continue
						}

						if strings.Contains(arg.Type, ".") {
							comp = arg.Type

							tr.add("resolved %q to %q via <cfargument type>", variable, comp)
						} else if parser.IsMemberMethod(funcName) {
							// Primitive-typed argument (string/numeric/array/etc.)
							// calling a known member/Java-interop method (e.g.
							// a string argument's .toCharArray()) — no component
							// is needed to verify it.
							tr.add("ARGUMENTS.%s has primitive type %q, but %q is a known member method — accepted without a component", argName, arg.Type, funcName)

							return ""
						}

						break
					}

					break
				}
			}
		}

		// Fall back to extends chain component refs (e.g. variables.$assert assigned in a parent)
		if comp == "" && pr.Extends != "" {
			tr.add("no ref found in this file — checking extends chain (%s) for a ComponentRef", pr.Extends)

			seen := make(map[string]bool)
			extends := pr.Extends

			for extends != "" && !seen[extends] {
				seen[extends] = true

				cfcPath := r.ComponentPath(extends, baseDir)
				if cfcPath == "" {
					break
				}

				parentURI := cfpath.ToURI(cfcPath)

				// Ensure the parent is indexed so RefsForFile returns its component refs.
				// (EnsureIndexed is a fast no-op if already indexed.)
				r.EnsureIndexed(cfcPath)

				for _, ref := range r.Index.RefsForFile(parentURI) {
					if strings.EqualFold(ref.Variable, lookupVar) {
						comp = ref.Component

						tr.add("resolved %q to %q via ComponentRef in parent %s", variable, comp, extends)

						break
					}
				}

				if comp != "" {
					break
				}

				// Walk up the extends chain
				data, err := r.FS.ReadFile(cfcPath)
				if err != nil {
					break
				}

				parentPR := parser.Parse(parentURI, string(data))
				extends = parentPR.Extends
			}
		}
	}

	if comp == "" {
		// Try component resolvers
		var (
			noFollow bool
			idx      int
		)

		comp, noFollow, idx = parser.ResolveFromCallMatch(variable, r.Resolvers)
		if comp != "" {
			tr.add("resolved %q to %q via componentResolver matching the variable name [%s] (noFollow=%v)",
				variable, comp, r.describeResolver(idx), noFollow)
		}

		if noFollow && comp != "" {
			return ""
		}
	}

	if comp == "" && call.Text != "" {
		// Try resolvers against the full line text (handles chained calls like x.method().prop.func())
		var (
			noFollow bool
			idx      int
		)

		comp, noFollow, idx = parser.ResolveFromCallMatch(call.Text, r.Resolvers)
		if comp != "" {
			tr.add("resolved %q to %q via componentResolver matching the full line text %q [%s] (noFollow=%v)",
				variable, comp, call.Text, r.describeResolver(idx), noFollow)
		}

		if noFollow && comp != "" {
			return ""
		}
	}

	if comp == "" {
		tr.add("no ComponentRef and no componentResolver matched %q", variable)
	}

	// Walk any intermediate .method() hops between the resolved base and this
	// call (e.g. "kpg.generateKeyPair().getPublic().getParams()" — comp here is
	// kpg's own component; call.Chain lists "generateKeyPair", "getPublic", each
	// needing its own declared return type applied before checking funcName below).
	if comp != "" && comp != "$any" && !strings.HasPrefix(comp, "$builtin.") {
		for _, hop := range call.Chain {
			fd := r.ResolveFunc(comp, hop, baseDir)
			if fd == nil {
				return "method '" + hop + "' not found in " + comp + " (chain to '" + funcName + "')"
			}

			ret := fd.ReturnComponent
			if ret != "" {
				tr.add("chain hop %q on %q: declared/inferred ReturnComponent %q", hop, comp, ret)
			} else if ret == "" && fd.ReturnType != "" && strings.Contains(fd.ReturnType, ".") {
				ret = fd.ReturnType

				tr.add("chain hop %q on %q: using dotted ReturnType %q", hop, comp, ret)
			}

			// The real function's declared return type isn't a component (e.g.
			// a generic returntype="struct" on a factory method that actually
			// returns a specific component instance) — fall back to a
			// componentResolver matching the hop's own call shape, same as the
			// non-chain "altComp" fallback below.
			if ret == "" {
				var noFollow bool

				ret, noFollow = parser.ResolveFromCallFull(hop+"()", r.Resolvers)
				if ret != "" {
					tr.add("chain hop %q on %q: no declared return type — componentResolver matched %q(): %q (noFollow=%v)", hop, comp, hop, ret, noFollow)
				}

				if noFollow && ret != "" {
					return ""
				}
			}

			if ret == "" {
				return "method '" + hop + "' in " + comp + " has no component return type (chain to '" + funcName + "')"
			}

			comp = ret
		}
	}

	if comp == "" {
		return "variable '" + variable + "' has no component ref"
	}

	// Dynamic return type — method called on a result of a function returning "any".
	if comp == "$any" {
		tr.add("component is $any (dynamic) — accepted without a method check")

		return ""
	}

	// Builtin return type — check method exists in-memory
	if strings.HasPrefix(comp, "$builtin.") {
		builtinName := comp[9:]
		if docs.LookupBuiltinMethod(builtinName, funcName) {
			return ""
		}

		return "method '" + funcName + "' not found on builtin " + builtinName
	}

	tr.add("checking whether %q defines method %q", comp, funcName)

	if r.ResolveFunc(comp, funcName, baseDir) != nil {
		return ""
	}

	if parser.IsMemberMethod(funcName) {
		tr.add("%q is a known member/Java-interop method — accepted without finding it in %q", funcName, comp)

		return ""
	}

	// Component defines onMissingMethod — any method call is valid.
	if r.ResolveFunc(comp, "onMissingMethod", baseDir) != nil {
		tr.add("%q defines onMissingMethod — any method call accepted", comp)

		return ""
	}

	// The ref-derived component didn't have the method. Try the variable-name resolver
	// as a fallback — a pendingCall propagation may have assigned the wrong component
	// (e.g. var objFile = _parent.getFile() inherits _parent's component, but the
	// "objFile" resolver names the real type).
	if altComp, altNoFollow := parser.ResolveFromCallFull(variable, r.Resolvers); altComp != "" && altComp != comp {
		tr.add("%q not found in %q — trying altComp fallback: componentResolver on variable name gives %q (noFollow=%v)", funcName, comp, altComp, altNoFollow)

		if altNoFollow || r.ResolveFunc(altComp, funcName, baseDir) != nil {
			return ""
		}
	}

	return "method '" + funcName + "' not found in " + comp
}

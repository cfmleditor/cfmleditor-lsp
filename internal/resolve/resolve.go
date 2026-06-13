// Package resolve provides component and function resolution logic.
package resolve

import (
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
}

// ComponentPath resolves a component dot-path to an absolute .cfc file path
// using the standard fallback chain: baseDir → Application.cfc root → workspace folders.
// If component contains pipe characters, each alternative is tried left-to-right.
func (r *Resolver) ComponentPath(component, baseDir string) string {
	// Apply expression mappings (replace runtime expressions with static values)
	for expr, value := range r.ExpressionMappings {
		if strings.Contains(component, expr) {
			component = strings.ReplaceAll(component, expr, value)
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

	return ""
}

// EnsureIndexed ensures a CFC file is indexed, loading from disk if needed.
func (r *Resolver) EnsureIndexed(cfcPath string) []*parser.FunctionDef {
	cfcURI := uri.URI("file://" + cfcPath)

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
		cfcURI := uri.URI("file://" + cfcPath)

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
	funcName := call.FuncName
	variable := call.Variable

	// Unqualified call — check same file, then extends chain
	if variable == "" {
		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, funcName) {
				return ""
			}
		}
		// Check extends chain
		if pr.Extends != "" {
			if r.ResolveFunc(pr.Extends, funcName, baseDir) != nil {
				return ""
			}

			return "not found in extends chain"
		}

		return "no qualifier, not in file"
	}

	// this. qualifier — refers to the current component
	if strings.EqualFold(variable, "this") {
		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, funcName) {
				return ""
			}
		}

		if pr.Extends != "" {
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

		if r.ResolveFunc(pr.Extends, funcName, baseDir) != nil {
			return ""
		}

		return "not found in parent component"
	}

	// Qualified call — find the component from refs
	comp := call.Component
	if comp == "" {
		// Strip scope prefix for matching (VARIABLES.x -> x)
		lookupVar := variable
		if dotIdx := strings.LastIndexByte(lookupVar, '.'); dotIdx >= 0 {
			lookupVar = lookupVar[dotIdx+1:]
		}

		// Try function-scoped refs first
		for _, scope := range pr.Scopes {
			if int(call.Line) >= scope.Start && int(call.Line) <= scope.End {
				for _, ref := range pr.FuncComponentRefs(scope.Start, scope.End) {
					if strings.EqualFold(ref.Variable, lookupVar) {
						comp = ref.Component

						break
					}
				}

				break
			}
		}

		// Fall back to component refs
		if comp == "" {
			for i := range pr.ComponentRefs {
				ref := &pr.ComponentRefs[i]
				if strings.EqualFold(ref.Variable, lookupVar) {
					comp = ref.Component

					break
				}
			}
		}

		// Fall back to Application.cfc component refs
		if comp == "" {
			if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
				for _, appName := range []string{"Application.cfc", "Application.cfm"} {
					appURI := uri.URI("file://" + filepath.Join(appDir, appName))
					for _, ref := range r.Index.RefsForFile(appURI) {
						if strings.EqualFold(ref.Variable, lookupVar) {
							comp = ref.Component

							break
						}
					}

					if comp != "" {
						break
					}
				}
			}
		}
	}

	if comp == "" {
		// Try component resolvers
		comp = parser.ResolveFromCall(variable, r.Resolvers)
	}

	if comp == "" && call.Text != "" {
		// Try resolvers against the full line text (handles chained calls like x.method().prop.func())
		comp = parser.ResolveFromCall(call.Text, r.Resolvers)
	}

	if comp == "" {
		return "variable '" + variable + "' has no component ref"
	}

	// Builtin return type — check method exists in-memory
	if strings.HasPrefix(comp, "$builtin.") {
		builtinName := comp[9:]
		if docs.LookupBuiltinMethod(builtinName, funcName) {
			return ""
		}

		return "method '" + funcName + "' not found on builtin " + builtinName
	}

	if r.ResolveFunc(comp, funcName, baseDir) != nil {
		return ""
	}

	if parser.IsMemberMethod(funcName) {
		return ""
	}

	return "method '" + funcName + "' not found in " + comp
}

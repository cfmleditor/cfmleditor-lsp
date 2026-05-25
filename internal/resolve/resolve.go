// Package resolve provides component and function resolution logic.
package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// Resolver resolves component dot-paths to files and functions.
type Resolver struct {
	FS               vfs.FS
	WorkspaceFolders []string
	Mappings         map[string]string
	Index            *index.Index
	Resolvers        []parser.Resolver
	appRootCache     map[string]string // dir → Application.cfc root
	resolveCache     map[string]string // component+"\t"+baseDir → file path
}

// ComponentPath resolves a component dot-path to an absolute .cfc file path
// using the standard fallback chain: baseDir → Application.cfc root → workspace folders.
func (r *Resolver) ComponentPath(component, baseDir string) string {
	key := component + "\t" + baseDir
	if r.resolveCache != nil {
		if p, ok := r.resolveCache[key]; ok {
			return p
		}
	}
	result := r.componentPathUncached(component, baseDir)
	if r.resolveCache == nil {
		r.resolveCache = make(map[string]string)
	}
	r.resolveCache[key] = result
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
	if r.appRootCache != nil {
		if v, ok := r.appRootCache[dir]; ok {
			return v
		}
	}
	result := r.findApplicationRootUncached(dir)
	if r.appRootCache == nil {
		r.appRootCache = make(map[string]string)
	}
	r.appRootCache[dir] = result
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
	for k, v := range appMappings {
		merged[k] = v
	}
	for k, v := range r.Mappings {
		merged[k] = v
	}
	return merged
}

// ResolveFromCall resolves a call expression against configured resolvers.
func (r *Resolver) ResolveFromCall(expr string) string {
	return parser.ResolveFromCall(expr, r.Resolvers)
}

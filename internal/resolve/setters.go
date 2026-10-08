package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// SetterLookup supplies dependency types only for components inside configured
// bean roots or source-proven FW/1 controller/service scopes. Merely naming a
// parameter after a CFC does not imply injection.
func (r *Resolver) SetterLookup(file string) func(string) string {
	if !cfpath.IsCFCFile(file) || r.discoveringDI {
		return nil
	}

	roots := r.managedBeanPaths()
	managed := false

	for _, root := range roots {
		rel, err := filepath.Rel(root, file)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			managed = true

			break
		}
	}

	if !managed && !r.fw1SetterManaged(file) {
		return nil
	}

	return r.InjectionBeanLookup(file)
}

// BeanLookup honors literal aliases and explicit factory rules before bean
// filenames. Property getters and setter arguments must use the same identity.
func (r *Resolver) BeanLookup(name string) string {
	comp := parser.ResolveFromCall(`getBean("`+name+`")`, r.Resolvers)
	if comp == "" || strings.EqualFold(comp, name) {
		comp = r.Index.LookupBean(name)
	} else if !filepath.IsAbs(comp) {
		baseDir := ""
		if len(r.WorkspaceFolders) > 0 {
			baseDir = r.WorkspaceFolders[0]
		}

		comp = r.ComponentPath(comp, baseDir)
	}

	if comp == "" || !cfpath.IsCFCFile(comp) {
		return ""
	}

	if info, err := r.fs().Stat(comp); err == nil && !info.IsDir() {
		return comp
	}

	return ""
}

func (r *Resolver) managedBeanPaths() map[string]string {
	r.mu.RLock()
	cached := r.beanPathsCache
	r.mu.RUnlock()

	if cached != nil {
		return cached
	}

	appDirs := []string{}

	for _, root := range r.WorkspaceFolders {
		if dir := r.FindApplicationRoot(root); dir != "" {
			appDirs = append(appDirs, dir)
		}
	}

	paths := cfpath.BeanPathsFor(r.BeanPaths, appDirs)
	r.mu.Lock()
	if r.beanPathsCache == nil {
		r.beanPathsCache = paths
	}

	cached = r.beanPathsCache
	r.mu.Unlock()

	return cached
}

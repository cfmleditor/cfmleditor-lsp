package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// WireBox registers a model under an id, and most ids are not the path to
// the component: `UserService@users` is the UserService in the users
// module's models, `RequestStorage@cbstorages` one in a module the
// workspace may not have installed, and `binder.map( "Mailer" ).to(
// "#moduleMapping#.models.MailService" )` an id that names no file at all.
// This file is what the workspace's ModuleConfig.cfc and config/WireBox.cfc
// say about ids, read the way ColdBox reads them:
//
//   - A module registers its models under `this.modelNamespace`, which
//     defaults to the module's name — its directory's.
//   - `this.cfmapping` is a mapping to the module's directory, so a dot-path
//     starting with it is inside the module.
//   - `map( id ).to( path )` is an id's component, `#moduleMapping#` being
//     the module's own directory.

// wireboxWorkspace is that knowledge, for one set of configuration files.
type wireboxWorkspace struct {
	key        string
	namespaces map[string][]string // lowercased namespace -> module directories
	cfmappings map[string]string   // lowercased cfmapping -> module directory
	ids        map[string]idTarget // lowercased id -> what it is mapped to
}

// idTarget is a `map( id ).to( path )`: path is a dot-path, read from dir.
type idTarget struct {
	path, dir string
}

var (
	modelNamespaceRe = regexp.MustCompile(`(?i)this\.modelNamespace\s*=\s*["']([^"']*)["']`)
	cfmappingRe      = regexp.MustCompile(`(?i)this\.cfmapping\s*=\s*["']([^"']*)["']`)
	binderMapRe      = regexp.MustCompile(`(?is)\bmap\(\s*["']([^"'#]+)["']\s*\)\s*\.\s*to\(\s*["']([^"']+)["']\s*\)`)
)

// wirebox reads every ModuleConfig.cfc and config/WireBox.cfc the index
// holds, remembered until that set of files changes, as applicationHelpers
// is.
func (r *Resolver) wirebox() *wireboxWorkspace {
	if r.Index == nil || r.FS == nil {
		return nil
	}

	modules := r.Index.FindFilesByBasename("ModuleConfig")

	var binders []string

	for _, p := range r.Index.FindFilesByBasename("WireBox") {
		if strings.EqualFold(filepath.Base(filepath.Dir(p)), "config") {
			binders = append(binders, p)
		}
	}

	key := strings.Join(modules, "\x00") + "\x01" + strings.Join(binders, "\x00")

	r.mu.RLock()
	cached := r.wb
	r.mu.RUnlock()

	if cached != nil && cached.key == key {
		return cached
	}

	wb := &wireboxWorkspace{
		key:        key,
		namespaces: map[string][]string{},
		cfmappings: map[string]string{},
		ids:        map[string]idTarget{},
	}

	for _, m := range modules {
		data, err := r.fs().ReadFile(m)
		if err != nil {
			continue
		}

		src := string(data)
		dir := filepath.Dir(m)

		ns := filepath.Base(dir)
		if sm := modelNamespaceRe.FindStringSubmatch(src); sm != nil && strings.TrimSpace(sm[1]) != "" {
			ns = strings.TrimSpace(sm[1])
		}

		wb.namespaces[strings.ToLower(ns)] = append(wb.namespaces[strings.ToLower(ns)], dir)

		if sm := cfmappingRe.FindStringSubmatch(src); sm != nil && strings.TrimSpace(sm[1]) != "" {
			wb.cfmappings[strings.ToLower(strings.Trim(strings.TrimSpace(sm[1]), "/"))] = dir
		}

		wb.addMaps(src, dir)
	}

	for _, b := range binders {
		if data, err := r.fs().ReadFile(b); err == nil {
			// config/WireBox.cfc is the application's: its paths are read
			// from the application's root, the directory above config/.
			wb.addMaps(string(data), filepath.Dir(filepath.Dir(b)))
		}
	}

	r.mu.Lock()
	r.wb = wb
	r.mu.Unlock()

	return wb
}

// addMaps records every `map( id ).to( path )` in src.
func (wb *wireboxWorkspace) addMaps(src, dir string) {
	for _, m := range binderMapRe.FindAllStringSubmatch(src, -1) {
		id := strings.ToLower(strings.TrimSpace(m[1]))
		if _, ok := wb.ids[id]; !ok {
			wb.ids[id] = idTarget{path: strings.TrimSpace(m[2]), dir: dir}
		}
	}
}

// wireboxID is ComponentPath's answer for a WireBox id: a mapped one's
// target, and `Name@namespace` the Name in that module's models. It reports
// whether component was an id it answers for; a plain dot-path is not, and
// is left to the lookups after it.
func (r *Resolver) wireboxID(component, baseDir string) (string, bool) {
	wb := r.wirebox()

	// Ids are looked up only for a component with no dot: a mapped target is
	// a dot-path, so resolving one cannot come back here, and `map( "X" ).to(
	// "X" )` is not an id worth the loop.
	if wb != nil && !strings.Contains(component, ".") {
		if t, ok := wb.ids[strings.ToLower(component)]; ok && !strings.EqualFold(t.path, component) {
			return r.idTargetPath(t, wb), true
		}
	}

	name, ns, qualified := strings.Cut(component, "@")
	if !qualified {
		return "", false
	}

	if wb != nil {
		if p := r.inModuleModels(name, wb.namespaces[strings.ToLower(ns)]); p != "" {
			return p, true
		}
	}

	// Not among the module's models, or a module the workspace does not
	// hold: the name alone, as it was always read. When that names nothing
	// either, uninstalledModule says whether it is a finding.
	return r.ComponentPath(name, baseDir), true
}

// idTargetPath is the file a `map( id ).to( path )` names.
func (r *Resolver) idTargetPath(t idTarget, wb *wireboxWorkspace) string {
	path := t.path

	for _, token := range []string{"#moduleMapping#.", "#moduleMapping#/"} {
		if len(path) > len(token) && strings.EqualFold(path[:len(token)], token) {
			return r.ComponentPath(path[len(token):], t.dir)
		}
	}

	if first, rest, ok := strings.Cut(path, "."); ok {
		if dir, mapped := wb.cfmappings[strings.ToLower(first)]; mapped {
			if p := r.ComponentPath(rest, dir); p != "" {
				return p
			}
		}
	}

	return r.ComponentPath(path, t.dir)
}

// inModuleModels is the component called name among the models of any of
// the module directories: ColdBox maps a module's models directory, every
// component in it under its file name.
func (r *Resolver) inModuleModels(name string, dirs []string) string {
	if len(dirs) == 0 || name == "" || strings.ContainsAny(name, "./\\") {
		return ""
	}

	var hits []string

	for _, c := range r.Index.FindFilesByBasename(name) {
		for _, d := range dirs {
			if strings.HasPrefix(strings.ToLower(c), strings.ToLower(filepath.Join(d, "models"))+string(filepath.Separator)) {
				hits = append(hits, c)
			}
		}
	}

	if len(hits) == 0 {
		return ""
	}

	slices.Sort(hits)

	return hits[0]
}

// inModuleMapping resolves a dot-path whose first segment is a module's
// cfmapping, inside the module.
func (r *Resolver) inModuleMapping(component string) string {
	first, rest, ok := strings.Cut(component, ".")
	if !ok || rest == "" {
		return ""
	}

	wb := r.wirebox()
	if wb == nil {
		return ""
	}

	dir, mapped := wb.cfmappings[strings.ToLower(first)]
	if !mapped {
		return ""
	}

	return r.ComponentPath(rest, dir)
}

// uninstalledModule reports whether component is an id of a module the
// workspace does not hold — `RequestStorage@cbstorages` with no cbstorages
// module in sight — which is the module not being installed, not a
// component that is missing.
func (r *Resolver) uninstalledModule(component string) bool {
	_, ns, ok := strings.Cut(component, "@")
	if !ok || ns == "" {
		return false
	}

	wb := r.wirebox()

	return wb == nil || len(wb.namespaces[strings.ToLower(ns)]) == 0
}

package path

import (
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/vfs"
)

// BuildBeanMap scans configured bean directories and builds a lookup map.
// The input is namespace → directory path. For each namespace, all .cfc files
// in that directory (recursively) are registered under "name@namespace".
// CFCs with unique names across ALL namespaces also get a bare "name" entry.
// Values are absolute file paths.
func BuildBeanMap(beanPaths map[string]string, fsys vfs.FS) map[string]string {
	type beanEntry struct {
		absPath string
		ns      string
		name    string
	}

	var all []beanEntry

	// Tracks, per bare name, the distinct files that claim it.
	bareOwner := make(map[string]string)
	bareAmbiguous := make(map[string]bool)

	for ns, root := range beanPaths {
		_ = fsys.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if !IsCFCFile(path) {
				return nil
			}

			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			all = append(all, beanEntry{absPath: path, ns: ns, name: name})

			key := strings.ToLower(name)
			if prev, seen := bareOwner[key]; seen {
				if !SamePath(prev, path) {
					bareAmbiguous[key] = true
				}
			} else {
				bareOwner[key] = path
			}

			return nil
		})
	}

	beans := make(map[string]string, len(all))

	for _, b := range all {
		key := strings.ToLower(b.name)

		// Namespace-qualified entry (only if namespace is non-empty)
		if b.ns != "" {
			beans[key+"@"+strings.ToLower(b.ns)] = b.absPath
		}

		// Bare name entry, but only where the name identifies one file. The tally
		// this replaces was written and never read: every bean got a bare entry,
		// and when two namespaces held the same name the winner was whichever
		// came last out of `range beanPaths` — a Go map, so the order is
		// randomised per process. `svc` resolved to one component on one launch
		// and another on the next, with nothing to indicate a choice had been
		// made. The documented rule is a bare name "when unique across all
		// namespaces".
		//
		// Uniqueness is by file, not by occurrence: a nested namespace is also
		// walked by its parent, so the same .cfc is legitimately reached twice
		// and its bare name is still unambiguous.
		if !bareAmbiguous[key] {
			beans[key] = b.absPath
		}
	}

	// DI/1 also registers each bean under its name and its folder's
	// singular — services/user.cfc is userService as well as user
	// (framework/ioc.cfc) — which is the name FW/1 code autowires by:
	// `property userService;`. A name some bean already has keeps it.
	for _, b := range all {
		dir := filepath.Base(filepath.Dir(b.absPath))
		if dir == "" || dir == "." {
			continue
		}

		alias := strings.ToLower(b.name + di1Singular(dir))
		if _, taken := beans[alias]; !taken {
			beans[alias] = b.absPath
		}
	}

	return beans
}

// di1Singular is DI/1's singular of a folder name: a trailing s dropped.
func di1Singular(plural string) string {
	if strings.HasSuffix(strings.ToLower(plural), "s") {
		return plural[:len(plural)-1]
	}

	return plural
}

// BeanPathsFor is the bean namespaces a workspace declares: those in each
// application root's Application.cfc, the first root to name a namespace
// winning, under the configured beanPaths, which win over all of them. It is
// what the server builds its bean map from, and the batch scans must build
// theirs alike or a property's type depends on which tool asked.
func BeanPathsFor(configured map[string]string, appDirs []string) map[string]string {
	all := make(map[string]string)

	for _, appDir := range appDirs {
		if appDir == "" {
			continue
		}

		for ns, dir := range LoadAppBeanPaths(appDir) {
			if _, exists := all[ns]; !exists {
				all[ns] = dir
			}
		}
	}

	maps.Copy(all, configured)

	return all
}

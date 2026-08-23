package server

import (
	"os"
	"path/filepath"
	"strings"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// buildBeanMap scans configured bean directories and builds a lookup map.
// The input is namespace → directory path. For each namespace, all .cfc files
// in that directory (recursively) are registered under "name@namespace".
// CFCs with unique names across ALL namespaces also get a bare "name" entry.
// Values are absolute file paths.
func buildBeanMap(beanPaths map[string]string, fsys vfs.FS) map[string]string {
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

			if !cfpath.IsCFCFile(path) {
				return nil
			}

			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			all = append(all, beanEntry{absPath: path, ns: ns, name: name})

			key := strings.ToLower(name)
			if prev, seen := bareOwner[key]; seen {
				if !cfpath.SamePath(prev, path) {
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

	return beans
}

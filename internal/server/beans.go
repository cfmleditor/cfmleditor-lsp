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

	bareCount := make(map[string]int)

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
			bareCount[strings.ToLower(name)]++

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

		// Bare name entry — prefer namespaced over root namespace
		if _, exists := beans[key]; !exists {
			beans[key] = b.absPath
		} else if b.ns != "" {
			// Namespaced entry overrides root namespace entry
			beans[key] = b.absPath
		}
	}

	return beans
}

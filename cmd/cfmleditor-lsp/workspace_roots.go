package main

import (
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// cliWorkspaceFolders gives resolution the same roots the command uses when
// workspacePaths is omitted. A config containing only presets or mappings must
// not remove the command's workspace. Explicit workspacePaths retain priority.
// File targets contribute their directory to lookup, without widening the scan.
func cliWorkspaceFolders(fsys vfs.FS, cfg *daemon.Config, paths []string) []string {
	if cfg != nil {
		if roots := cfg.WorkspaceFolders(); len(roots) > 0 {
			return roots
		}
	}

	var roots []string

	seen := map[string]bool{}

	for _, path := range paths {
		info, err := fsys.Stat(path)
		if err != nil {
			continue
		}

		if !info.IsDir() {
			path = filepath.Dir(path)
		}

		root, err := filepath.Abs(path)
		if err != nil || seen[root] {
			continue
		}

		seen[root] = true
		roots = append(roots, root)
	}

	return roots
}

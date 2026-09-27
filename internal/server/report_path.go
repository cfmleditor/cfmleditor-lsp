package server

import (
	"fmt"
	"path/filepath"
	"strings"
)

// reportPath is where a report named base is written in dir, or an error when
// it would land outside the workspace.
//
// Both halves come from a command's arguments: findRefs names its report after
// the function it was asked about and writes it beside the source file it was
// given, and exportDeps after the function or the document it was given. The
// editor checks neither, so a name holding "../" or a document outside the
// workspace put a file anywhere the server could write. The code map's output
// path has always been confined the same way (codeMapOutputPath).
func (s *Server) reportPath(dir, base string) (string, error) {
	if base == "." || base == ".." || base != filepath.Base(base) || strings.ContainsAny(base, `/\`) {
		return "", fmt.Errorf("refusing a report name that is not a plain file name: %q", base)
	}

	for _, root := range s.searchRoots() {
		if insideDir(root, dir) {
			return filepath.Join(dir, base), nil
		}
	}

	return "", fmt.Errorf("refusing to write a report outside the workspace: %s", dir)
}

// insideDir reports whether path is root or lies beneath it.
func insideDir(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(rootAbs, abs)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

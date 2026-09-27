package server

import (
	"fmt"
	"os"
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

// reportPerm is owner-only: a report can quote the source it was made from, and
// nothing but the user who asked for it needs to read it.
const reportPerm = 0o600

// writeReport writes a report the server generated to path.
//
// It goes through an os.Root on the report's directory. reportPath and
// codeMapOutputPath check the path as text, which a symlink planted where the
// report goes defeats: the write would follow it to wherever it points. The
// root refuses that. Only the file name is confined, not the directories above
// it, so a workspace reached through a symlinked directory still gets its
// reports.
func writeReport(path string, data []byte) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}

	defer func() { _ = root.Close() }()

	return root.WriteFile(filepath.Base(path), data, reportPerm)
}

// createReport is writeReport for a report written as a stream. The file stays
// usable after the root is closed.
func createReport(path string) (*os.File, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	defer func() { _ = root.Close() }()

	return root.OpenFile(filepath.Base(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, reportPerm)
}

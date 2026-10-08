package cflint

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// ScanRoots is what a project lint covers: the directories asked for. The one
// exception is the config's own directory, which stands for the workspace
// folders beneath it: ~/tassdev holds far more than the seven repos its config
// lists. A folder outside the config's directory is never linted.
// workspacePaths exists for resolution, so a project's config lists every
// sibling repo it calls into (tassweb's names all twelve), and CFLint resolves
// nothing. Both the cflint CLI and clif.exportCFLint linted every
// workspace folder, and then rewrote reports for directories nobody asked
// about or dropped what fell outside the one report they wrote.
func ScanRoots(roots []string, configDir string, workspaceFolders []string) []string {
	if len(roots) != 1 || roots[0] != configDir {
		return roots
	}

	var under []string

	for _, f := range workspaceFolders {
		if Within(f, configDir) {
			under = append(under, f)
		}
	}

	if len(under) == 0 {
		return roots
	}

	return under
}

// WriteTargets narrows a config's report files to those a lint of the
// directories asked for may rewrite. Pass what was asked, not what ScanRoots
// expanded it to: the config's own directory stands for its workspace, so its
// report is the workspace's, though the directory holds more than the folders
// linted. A report under a linted directory is rewritten whole; one beside
// them is left alone, since none of the issues found belong in it; and one
// whose directory holds a linted directory and more is an error, because
// rewriting it from a partial scan would delete every entry outside it.
func WriteTargets(targets, roots []string) ([]string, error) {
	var keep []string

	for _, t := range targets {
		dir := filepath.Dir(t)

		switch {
		case slices.ContainsFunc(roots, func(r string) bool { return Within(dir, r) }):
			keep = append(keep, t)
		case slices.ContainsFunc(roots, func(r string) bool { return Within(r, dir) }):
			return nil, fmt.Errorf("refusing to replace %s: its directory, %s, holds more than was linted", t, dir)
		}
	}

	if len(keep) == 0 {
		return nil, errors.New("no CFLint report file lies under the directories linted")
	}

	return keep, nil
}

// Within reports whether path is dir or lies beneath it.
func Within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

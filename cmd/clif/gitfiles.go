package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// git runs git in repo and returns its stdout, or an error carrying its stderr.
func git(repo string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repo}, args...)...) //nolint:gosec // git with fixed subcommands; a ref or path is one argument, never shell text

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}

		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return out, nil
}

// gitTopLevel is the root of the repository holding dir.
func gitTopLevel(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not in a git repository (%w)", err)
	}

	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
}

// gitPaths turns git's NUL-separated, repository-relative names into absolute
// paths. -z keeps a name with a space, or a non-ASCII one, as it is.
func gitPaths(repo string, out []byte) []string {
	var paths []string

	for name := range strings.SplitSeq(string(out), "\x00") {
		if name != "" {
			paths = append(paths, filepath.Join(repo, filepath.FromSlash(name)))
		}
	}

	return paths
}

// gitStagedFiles are the files staged in repo, added, copied, modified or
// renamed: a deleted file has nothing to lint.
func gitStagedFiles(repo string) ([]string, error) {
	out, err := git(repo, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
	if err != nil {
		return nil, err
	}

	return gitPaths(repo, out), nil
}

// gitChangedFiles are the files changed between ref's merge base with HEAD and
// HEAD, as a pull request's diff reads.
func gitChangedFiles(repo, ref string) ([]string, error) {
	out, err := git(repo, "diff", "--name-only", "--diff-filter=ACMR", "-z", ref+"...HEAD")
	if err != nil {
		return nil, err
	}

	return gitPaths(repo, out), nil
}

// gitStagedContent is path's content in the index.
func gitStagedContent(repo, path string) ([]byte, error) {
	rel, err := filepath.Rel(repo, path)
	if err != nil {
		return nil, err
	}

	return git(repo, "show", ":"+filepath.ToSlash(rel))
}

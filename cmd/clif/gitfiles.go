package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
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

// gitAddedLines are the lines each file gains or changes, by absolute path and
// 1-based line: staged ones, or those since ref's merge base with HEAD. A
// finding outside them was already there before the change.
func gitAddedLines(repo string, staged bool, ref string) (map[string]map[int]bool, error) {
	args := []string{"diff", "--no-color", "--no-ext-diff", "-U0", "--diff-filter=ACMR"}
	if staged {
		args = append(args, "--cached")
	} else {
		args = append(args, ref+"...HEAD")
	}

	out, err := git(repo, args...)
	if err != nil {
		return nil, err
	}

	return parseAddedLines(repo, string(out)), nil
}

// parseAddedLines reads a unified diff with no context: the "+++ b/<path>"
// line names the file, and each "@@ -a,b +c,d @@" hunk adds lines c to c+d-1
// (d is 1 when left out, and 0 for a hunk that only removes).
func parseAddedLines(repo, diff string) map[string]map[int]bool {
	added := map[string]map[int]bool{}

	var file string

	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			name := strings.TrimPrefix(line, "+++ ")
			if name == "/dev/null" {
				file = ""

				continue
			}

			// git quotes a name with unusual characters, C-style.
			if unq, err := strconv.Unquote(name); err == nil {
				name = unq
			}

			file = filepath.Join(repo, filepath.FromSlash(strings.TrimPrefix(name, "b/")))
		case strings.HasPrefix(line, "@@ ") && file != "":
			start, count, ok := hunkNewRange(line)
			if !ok {
				continue
			}

			if added[file] == nil {
				added[file] = map[int]bool{}
			}

			for n := start; n < start+count; n++ {
				added[file][n] = true
			}
		}
	}

	return added
}

// hunkNewRange reads the new side of a hunk header, "+c,d" or "+c".
func hunkNewRange(header string) (start, count int, ok bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, false
	}

	startText, countText, hasCount := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")

	start, err := strconv.Atoi(startText)
	if err != nil {
		return 0, 0, false
	}

	count = 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			return 0, 0, false
		}
	}

	return start, count, true
}

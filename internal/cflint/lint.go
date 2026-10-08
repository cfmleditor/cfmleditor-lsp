package cflint

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
)

// Run is a whole lint run's result, as CFLint reports it, merged across the
// several CFLint processes one run takes: the issues at or above the runner's
// severity floor, and the totals CFLint's own reports print.
type Run struct {
	// Version is CFLint's, as its JSON report gives it.
	Version string
	// Issues hold one location each, ordered by file, line, column and rule,
	// so the same project gives the same report.
	Issues []RunIssue
	// TotalFiles and TotalLines are what CFLint read.
	TotalFiles int
	TotalLines int
}

// RunIssue is one issue, parsed, together with CFLint's JSON for it, which the
// JSON report passes through unchanged so a consumer of CFLint's own schema
// reads every field it always did.
type RunIssue struct {
	Issue

	Raw json.RawMessage
}

// Source is a file's path and the content to lint under it: what is staged,
// say, rather than what is on disk. CFLint reads .cflintrc and resolves
// @CFLintIgnore against the path, so they apply as they would to the file.
type Source struct {
	Path    string
	Content []byte
}

// rawReport is CFLint's JSON report, keeping each issue's JSON.
type rawReport struct {
	Version string            `json:"version"`
	Issues  []json.RawMessage `json:"issues"`
	Counts  struct {
		TotalFiles int `json:"totalFiles"`
		TotalLines int `json:"totalLines"`
	} `json:"counts"`
}

// Lint runs CFLint over files from disk, several to a process and a few
// processes at once, as ScanFiles does.
func (r *Runner) Lint(ctx context.Context, files []string) (*Run, error) {
	var jobs [][]string

	for start := 0; start < len(files); start += filesPerRun {
		batch := files[start:min(start+filesPerRun, len(files))]
		jobs = append(jobs, []string{"-file", strings.Join(batch, ",")})
	}

	return r.lintJobs(ctx, jobs, nil)
}

// LintContent runs CFLint over each source's content, one process each, since
// CFLint reads only one file from stdin.
func (r *Runner) LintContent(ctx context.Context, sources []Source) (*Run, error) {
	jobs := make([][]string, 0, len(sources))
	stdin := make([][]byte, 0, len(sources))

	for i := range sources {
		jobs = append(jobs, []string{"-stdin", sources[i].Path})
		stdin = append(stdin, sources[i].Content)
	}

	return r.lintJobs(ctx, jobs, stdin)
}

func (r *Runner) lintJobs(ctx context.Context, jobs [][]string, stdin [][]byte) (*Run, error) {
	run := &Run{}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)

	sem := make(chan struct{}, 4)

	for i, args := range jobs {
		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			var in []byte
			if stdin != nil {
				in = stdin[i]
			}

			report, err := r.runRaw(ctx, in, append(args, "-json", "-stdout", "-q")...)

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				err = run.add(report, r.minRank)
			}

			if err != nil && firstErr == nil {
				firstErr = err
			}
		}()
	}

	wg.Wait()

	if firstErr == nil {
		firstErr = ctx.Err()
	}

	if firstErr != nil {
		return nil, firstErr
	}

	run.sort()

	return run, nil
}

// sort orders the issues by file, line, column and rule. The processes of one
// run finish in any order, so without it the same project gave a differently
// ordered report each time.
func (run *Run) sort() {
	slices.SortStableFunc(run.Issues, func(a, b RunIssue) int {
		la, lb := firstLocation(&a.Issue), firstLocation(&b.Issue)

		return cmp.Or(
			cmp.Compare(la.File, lb.File),
			cmp.Compare(la.Line, lb.Line),
			cmp.Compare(la.Column, lb.Column),
			cmp.Compare(a.ID, b.ID),
		)
	})
}

// add merges one CFLint report into the run, dropping issues under minRank.
func (run *Run) add(report *rawReport, minRank int) error {
	if run.Version == "" {
		run.Version = report.Version
	}

	run.TotalFiles += report.Counts.TotalFiles
	run.TotalLines += report.Counts.TotalLines

	for _, raw := range report.Issues {
		var issue Issue
		if err := json.Unmarshal(raw, &issue); err != nil {
			return fmt.Errorf("parsing a cflint issue: %w", err)
		}

		if !meetsFloor(issue.Severity, minRank) {
			continue
		}

		run.Issues = append(run.Issues, RunIssue{Issue: issue, Raw: raw})
	}

	return nil
}

// runRaw runs the binary with args, feeding it stdin when there is any, and
// decodes its JSON report keeping each issue's JSON.
func (r *Runner) runRaw(ctx context.Context, stdin []byte, args ...string) (*rawReport, error) {
	cmd := exec.CommandContext(ctx, r.binPath, args...) //nolint:gosec // the CFLint binary this package downloaded, with paths and fixed flags

	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}

	var stderr strings.Builder

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("cflint failed: %w; stderr: %s", err, stderr.String())
	}

	var report rawReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("parsing cflint output: %w\nstdout: %s\nstderr: %s", err, string(out), stderr.String())
	}

	return &report, nil
}

// firstLocation is where an issue is, for ordering: CFLint gives each issue
// one location, but an empty list must not panic.
func firstLocation(issue *Issue) Location {
	if len(issue.Locations) == 0 {
		return Location{}
	}

	return issue.Locations[0]
}

// Diagnostics is the run as editor diagnostics keyed by file, the shape the
// known-issues report is written from.
func (run *Run) Diagnostics() map[string][]protocol.Diagnostic {
	out := map[string][]protocol.Diagnostic{}

	issues := make([]Issue, 0, len(run.Issues))
	for i := range run.Issues {
		issues = append(issues, run.Issues[i].Issue)
	}

	eachDiagnostic(Result{Issues: issues}, noSeverityFloor, func(file string, d protocol.Diagnostic) {
		file = filepath.Clean(file)
		out[file] = append(out[file], d)
	})

	return out
}

// KeepLines drops every issue not on one of lines (by cleaned path, then
// 1-based line), returning how many it dropped.
func (run *Run) KeepLines(lines map[string]map[int]bool) int {
	kept := run.Issues[:0]

	for i := range run.Issues {
		loc := firstLocation(&run.Issues[i].Issue)
		if lines[filepath.Clean(loc.File)][loc.Line] {
			kept = append(kept, run.Issues[i])
		}
	}

	dropped := len(run.Issues) - len(kept)
	run.Issues = kept

	return dropped
}

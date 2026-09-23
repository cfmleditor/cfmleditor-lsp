package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cflint"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/knownissues"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/unresolved"
	"go.lsp.dev/protocol"
)

// report is one generated known-issues file's content, before it is written.
type report struct {
	path    string
	content string
	entries int
}

// handleExport answers cfmleditor.exportUnresolved and cfmleditor.exportCFLint:
// it scans the whole workspace from disk and writes the findings to the
// knownIssues files marked generate kind, or to that kind's default file
// beside .cfmleditor.json (see config.GenerateTargets), then reloads the ones
// knownIssues lists so their entries are published straight away.
//
// Like generateCodeMap, it returns the paths at once and works in the
// background, reporting through window/showMessage.
func (s *Server) handleExport(kind string) (any, error) {
	roots := s.searchRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("no workspace root to scan; open a folder first")
	}

	configDir := roots[0]
	if p, _ := s.governingConfig(); p != "" {
		configDir = filepath.Dir(p)
	}

	targets := config.GenerateTargets(s.KnownIssues, kind, configDir)

	if _, busy := s.exporting.LoadOrStore(kind, true); busy {
		return nil, fmt.Errorf("a %s export is already running", kind)
	}

	s.safeGo("export:"+kind, func() {
		defer s.exporting.Delete(kind)

		// context.Background: this goroutine outlives the handler, whose ctx
		// is pooled and reset on return.
		ctx := context.Background()
		started := time.Now()

		s.notifyInfo(ctx, "Scanning the workspace for the "+kind+" report…")

		files := collectWorkspaceCFMLFiles(s, roots)

		var (
			reports []report
			left    int
			err     error
		)

		switch kind {
		case config.GenerateCFLint:
			reports, left, err = s.cflintReports(ctx, files, targets)
		default:
			reports, left = s.unresolvedReports(files, targets)
		}

		if err != nil {
			s.notifyError(ctx, "The "+kind+" report failed: "+err.Error())

			return
		}

		var written, unlisted []string

		for _, r := range reports {
			if err := os.WriteFile(r.path, []byte(r.content), 0o644); err != nil { //nolint:gosec // a report committed to the project, read by everyone
				s.notifyError(ctx, "Could not write "+r.path+": "+err.Error())

				return
			}

			written = append(written, fmt.Sprintf("%s (%d)", r.path, r.entries))

			// The file watcher would reload it too, but not every client
			// watches, and the export should show its result either way.
			if s.isKnownIssuesFile(r.path) {
				s.loadKnownIssuesFile(ctx, r.path)
			} else {
				unlisted = append(unlisted, filepath.Base(r.path))
			}
		}

		msg := fmt.Sprintf("The %s report: %s in %s.", kind, strings.Join(written, ", "), time.Since(started).Round(time.Millisecond))
		if left > 0 {
			msg += fmt.Sprintf(" %d findings under no report's directory were left out.", left)
		}

		if len(unlisted) > 0 {
			msg += fmt.Sprintf(` List %s under knownIssues in .cfmleditor.json to see the entries as diagnostics.`, strings.Join(unlisted, ", "))
		}

		s.log.Info("known issues exported", cflog.String("kind", kind), cflog.Strings("files", written), cflog.Int("leftOut", left))
		s.notifyInfo(ctx, msg)
	})

	return map[string]any{"paths": targets, "started": true}, nil
}

// unresolvedReports runs the unresolved scan the CLI runs, with this
// session's settings, and splits it across targets.
func (s *Server) unresolvedReports(files, targets []string) ([]report, int) {
	opt := unresolved.Options{
		Mappings:                 s.Mappings,
		ExpressionMappings:       s.ExpressionMappings,
		ServicePropertyResolvers: s.ServicePropertyResolvers,
		WorkspaceFolders:         s.WorkspaceFolders,
		InterpolateAll:           !s.Features.OutputContextInterpolation,
	}

	for _, r := range s.ComponentResolvers {
		opt.Resolvers = append(opt.Resolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, NoFollow: r.NoFollow, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing})
	}

	rep := unresolved.Scan(s.FS, files, nil, opt)
	byTarget, rest := unresolved.SplitByTarget(rep.Calls, targets)

	out := make([]report, 0, len(targets))

	for _, t := range targets {
		var b strings.Builder

		skipped := unresolved.WriteKnownIssues(&b, byTarget[t], filepath.Dir(t), false, unresolved.RegenerateHint, s.Version)
		out = append(out, report{path: t, content: b.String(), entries: len(byTarget[t]) - skipped})
	}

	return out, len(rest)
}

// cflintRegenerateHint is the cflint report's how-to-regenerate header line.
const cflintRegenerateHint = "the editor's cfmleditor.exportCFLint command"

// cflintReports runs CFLint over files and splits its findings across
// targets. It uses the session's runner, or starts one when linting on save
// is off: a project report is still wanted where per-save linting is not.
func (s *Server) cflintReports(ctx context.Context, files, targets []string) ([]report, int, error) {
	s.mu.RLock()
	runner := s.linter
	s.mu.RUnlock()

	if runner == nil {
		r, err := cflint.NewRunner(s.LintMinSeverity)
		if err != nil {
			return nil, 0, err
		}

		runner = r
	}

	found, err := runner.ScanFiles(ctx, files)
	if err != nil {
		return nil, 0, err
	}

	rows := map[string][]knownissues.Row{}
	counts := map[string]int{}
	left := 0

	for file, diags := range found {
		t := deepestTarget(file, targets)
		if t == "" {
			left += len(diags)

			continue
		}

		rel, err := filepath.Rel(filepath.Dir(t), file)
		if err != nil {
			left += len(diags)

			continue
		}

		for _, d := range diags {
			rows[t] = append(rows[t], knownissues.Row{
				Path:     filepath.ToSlash(rel),
				Line:     int(d.Range.Start.Line) + 1,
				Col:      int(d.Range.Start.Character) + 1,
				Severity: knownissues.SeverityName(d.Severity),
				Code:     diagnosticCode(d),
				Message:  diagnosticMessage(d),
			})
		}

		counts[t] += len(diags)
	}

	out := make([]report, 0, len(targets))

	for _, t := range targets {
		var b strings.Builder

		knownissues.Write(&b, []string{
			"CFLint issues across the project, one per line: path:line:col: [severity RULE] message.",
			"Paths are relative to this file's directory. Listed under knownIssues in .cfmleditor.json",
			`with "generate": "cflint", the entries show as cflint diagnostics, and a file's give way`,
			"to CFLint's own results once it is linted on save. Regenerate with:",
			"  " + cflintRegenerateHint,
			fmt.Sprintf("%d entries; cfmleditor-lsp %s.", counts[t], s.Version),
		}, rows[t])

		out = append(out, report{path: t, content: b.String(), entries: counts[t]})
	}

	return out, left, nil
}

// deepestTarget is the target whose directory holds file, the deepest when
// directories nest; empty when none does.
func deepestTarget(file string, targets []string) string {
	best := ""

	for _, t := range targets {
		dir := filepath.Dir(t)

		rel, err := filepath.Rel(dir, file)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}

		if len(dir) > len(filepath.Dir(best)) || best == "" {
			best = t
		}
	}

	return best
}

func diagnosticCode(d protocol.Diagnostic) string {
	switch c := d.Code.(type) {
	case protocol.String:
		return string(c)
	case nil:
		return ""
	default:
		return fmt.Sprint(c)
	}
}

func diagnosticMessage(d protocol.Diagnostic) string {
	if m, ok := d.Message.(protocol.String); ok {
		return string(m)
	}

	return ""
}

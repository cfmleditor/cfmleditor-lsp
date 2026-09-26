package cflint

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/knownissues"
	"go.lsp.dev/protocol"
)

// RegenerateHint is the CFLint report header's how-to-regenerate line. It is
// the same whichever writes the report, so one regenerated from the editor and
// from the command line diffs clean.
const RegenerateHint = "cfmleditor-lsp cflint --write <project>, or the editor's cfmleditor.exportCFLint command"

// Report is one generated CFLint known-issues file, before it is written.
type Report struct {
	Path    string
	Content string
	Entries int
}

// Reports splits a ScanFiles result across the target files: each gets the
// issues under its own directory, the deepest when directories nest, as a
// known-issues file relative to that directory. It also returns how many
// issues fell under no target.
func Reports(found map[string][]protocol.Diagnostic, targets []string, version string) ([]Report, int) {
	rows := map[string][]knownissues.Row{}
	left := 0

	for file, diags := range found {
		t := knownissues.DeepestTarget(file, targets)

		rel, err := filepath.Rel(filepath.Dir(t), file)
		if t == "" || err != nil {
			left += len(diags)

			continue
		}

		for i := range diags {
			d := &diags[i]

			rows[t] = append(rows[t], knownissues.Row{
				Path:     filepath.ToSlash(rel),
				Line:     int(d.Range.Start.Line) + 1,
				Col:      int(d.Range.Start.Character) + 1,
				Severity: knownissues.SeverityName(d.Severity),
				Code:     diagnosticCode(d),
				Message:  diagnosticMessage(d),
			})
		}
	}

	out := make([]Report, 0, len(targets))

	for _, t := range targets {
		var b strings.Builder

		knownissues.Write(&b, []string{
			"CFLint issues across the project, one per line: path:line:col: [severity RULE] message.",
			"Paths are relative to this file's directory. Listed under knownIssues in .cfmleditor.json",
			`with "generate": "cflint", the entries show as cflint diagnostics, and a file's give way`,
			"to CFLint's own results once it is linted on save. Regenerate with:",
			"  " + RegenerateHint,
			fmt.Sprintf("%d entries; cfmleditor-lsp %s.", len(rows[t]), version),
		}, rows[t])

		out = append(out, Report{Path: t, Content: b.String(), Entries: len(rows[t])})
	}

	return out, left
}

func diagnosticCode(d *protocol.Diagnostic) string {
	switch c := d.Code.(type) {
	case protocol.String:
		return string(c)
	case nil:
		return ""
	default:
		return fmt.Sprint(c)
	}
}

func diagnosticMessage(d *protocol.Diagnostic) string {
	if m, ok := d.Message.(protocol.String); ok {
		return string(m)
	}

	return ""
}

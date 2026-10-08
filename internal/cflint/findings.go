package cflint

import (
	"slices"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/cfmleditor/cfmleditor-lsp/internal/knownissues"
)

// Finding is one CFLint issue as a caller outside an editor wants it: a path,
// a 1-based line and column, the CFLint rule and its message.
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Findings flattens a ScanFiles result, ordered by file, line and column so
// the same project gives the same list; ScanFiles returns a map.
func Findings(found map[string][]protocol.Diagnostic) []Finding {
	out := []Finding{}

	for file, diags := range found {
		for i := range diags {
			d := &diags[i]

			out = append(out, Finding{
				File:     file,
				Line:     int(d.Range.Start.Line) + 1,
				Col:      int(d.Range.Start.Character) + 1,
				Severity: knownissues.SeverityName(d.Severity),
				Rule:     diagnosticCode(d),
				Message:  diagnosticMessage(d),
			})
		}
	}

	slices.SortFunc(out, func(a, b Finding) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}

		if a.Line != b.Line {
			return a.Line - b.Line
		}

		if a.Col != b.Col {
			return a.Col - b.Col
		}

		return strings.Compare(a.Rule, b.Rule)
	})

	return out
}

package cflint

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// TestReportsSplitByDirectoryAndCarryTheRule covers the report the CLI and the
// server both write: each target gets the issues under its own directory,
// the deepest when they nest, with the rule and severity as the entry's tag,
// and an issue under no target is counted rather than written.
func TestReportsSplitByDirectoryAndCarryTheRule(t *testing.T) {
	root := filepath.FromSlash("/work")
	top := filepath.Join(root, ".cfmleditor-cflint.txt")
	web := filepath.Join(root, "tassweb", ".cfmleditor-cflint.txt")

	issue := func(line uint32, code, msg string) protocol.Diagnostic {
		return protocol.Diagnostic{
			Range:    protocol.Range{Start: protocol.Position{Line: line, Character: 2}},
			Severity: protocol.DiagnosticSeverityWarning,
			Code:     protocol.String(code),
			Message:  protocol.String(msg),
		}
	}

	found := map[string][]protocol.Diagnostic{
		filepath.Join(root, "tassweb", "a.cfc"): {issue(9, "MISSING_VAR", "x")},
		filepath.Join(root, "kiosk", "b.cfm"):   {issue(0, "IMPLICIT_SCOPE", "y")},
		filepath.FromSlash("/elsewhere/c.cfm"):  {issue(0, "X", "z")},
	}

	reports, left := Reports(found, []string{top, web}, "test")
	if left != 1 || len(reports) != 2 {
		t.Fatalf("left %d, reports %d", left, len(reports))
	}

	for _, c := range []struct {
		r    Report
		want string
	}{
		{reports[0], "kiosk/b.cfm:1:3: [warning IMPLICIT_SCOPE] y\n"},
		{reports[1], "a.cfc:10:3: [warning MISSING_VAR] x\n"},
	} {
		if c.r.Entries != 1 || !strings.HasSuffix(c.r.Content, c.want) {
			t.Errorf("%s: %d entries, content\n%s", c.r.Path, c.r.Entries, c.r.Content)
		}

		if !strings.Contains(c.r.Content, RegenerateHint) {
			t.Errorf("%s: header does not say how to regenerate it", c.r.Path)
		}
	}
}

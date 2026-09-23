package knownissues

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestParseKeepsRelativeEntriesOnly(t *testing.T) {
	base := filepath.FromSlash("/repo")
	content := strings.Join([]string{
		"# header",
		"",
		"packages/a.cfc:12: svc.run (method 'run' not found in x)",
		"webroot/b.cfm:3:7: TODO tidy this",
		"/abs/elsewhere.cfc:1: absolute is refused",
		"C:\\work\\x.cfc:1: a drive is refused",
		"../kiosk/c.cfm:4: a sibling project is kept",
		"not an entry",
		"packages/a.cfc:0: line zero is not a line",
	}, "\n")

	got := Parse(content, base)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(got), got)
	}

	if got[2].Path != filepath.Join(filepath.Dir(base), "kiosk", "c.cfm") {
		t.Errorf("sibling entry %+v", got[2])
	}

	if got[0].Path != filepath.Join(base, "packages", "a.cfc") || got[0].Line != 11 || got[0].Col != -1 {
		t.Errorf("first entry %+v", got[0])
	}

	if got[1].Line != 2 || got[1].Col != 6 || got[1].Message != "TODO tidy this" {
		t.Errorf("second entry %+v", got[1])
	}
}

func TestDiagnosticAnchorsAtTheCallAndFollowsAnEdit(t *testing.T) {
	lines := []string{
		"<cfcomponent>",
		"\t<cfset x = 1>",
		"\t<cfset y = svc.run(a)>",
		"</cfcomponent>",
	}

	// Recorded on line 3 (index 2): underlined at run.
	d := Diagnostic(Entry{Line: 2, Col: -1, Message: "svc.run (method 'run' not found in x)"}, lines, protocol.DiagnosticSeverityWarning, "known issue")
	if d.Range.Start.Line != 2 || d.Range.Start.Character != 16 || d.Range.End.Character != 19 {
		t.Errorf("on its line: %+v", d.Range)
	}

	// Recorded on line 1 after an edit moved the call down: found again.
	d = Diagnostic(Entry{Line: 0, Col: -1, Message: "svc.run (method 'run' not found in x)"}, lines, protocol.DiagnosticSeverityWarning, "known issue")
	if d.Range.Start.Line != 2 {
		t.Errorf("after an edit: %+v", d.Range)
	}

	// Free text covers the line's text.
	d = Diagnostic(Entry{Line: 1, Col: -1, Message: "TODO tidy this"}, lines, protocol.DiagnosticSeverityInformation, "todo")
	if d.Range.Start.Line != 1 || d.Range.Start.Character != 1 || d.Range.End.Character != uint32(len(lines[1])) {
		t.Errorf("free text: %+v", d.Range)
	}

	if d.Severity != protocol.DiagnosticSeverityInformation {
		t.Errorf("severity %v", d.Severity)
	}
}

func TestSeverity(t *testing.T) {
	for name, want := range map[string]protocol.DiagnosticSeverity{
		"error": protocol.DiagnosticSeverityError, "Warning": protocol.DiagnosticSeverityWarning,
		"hint": protocol.DiagnosticSeverityHint, "": protocol.DiagnosticSeverityWarning,
		"information": protocol.DiagnosticSeverityInformation, "info": protocol.DiagnosticSeverityInformation,
		"bogus": protocol.DiagnosticSeverityWarning,
	} {
		if got := Severity(name); got != want {
			t.Errorf("Severity(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestTaggedEntriesCarryTheirCodeAndSeverity covers the [severity CODE] tag a
// CFLint report writes, and that Write and Parse agree on it.
func TestTaggedEntriesCarryTheirCodeAndSeverity(t *testing.T) {
	var b strings.Builder

	Write(&b, []string{"report"}, []Row{
		{Path: "b.cfm", Line: 3, Col: 5, Severity: "error", Code: "MISSING_VAR", Message: "Variable x is\nnot declared"},
		{Path: "a.cfc", Line: 1, Code: "TODO", Message: "tidy"},
	})

	want := "# report\na.cfc:1: [TODO] tidy\nb.cfm:3:5: [error MISSING_VAR] Variable x is not declared\n"
	if b.String() != want {
		t.Fatalf("Write:\n%s\nwant\n%s", b.String(), want)
	}

	got := Parse(b.String(), "/p")
	if len(got) != 2 {
		t.Fatalf("Parse: %+v", got)
	}

	if got[0].Code != "TODO" || got[0].Severity != "" || got[0].Message != "tidy" {
		t.Errorf("untagged severity: %+v", got[0])
	}

	lint := got[1]
	if lint.Code != "MISSING_VAR" || lint.Severity != "error" || lint.Col != 4 || lint.Message != "Variable x is not declared" {
		t.Errorf("tagged: %+v", lint)
	}

	d := Diagnostic(lint, nil, protocol.DiagnosticSeverityInformation, "cflint")
	if d.Code != protocol.String("MISSING_VAR") || d.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("diagnostic: code %v severity %v", d.Code, d.Severity)
	}

	// An unresolved entry has no tag and keeps the file's severity and code.
	plain := Parse("a.cfc:2: svc.run (reason)\n", "/p")[0]
	if d := Diagnostic(plain, nil, protocol.DiagnosticSeverityWarning, "known issue"); d.Code != protocol.String("known-issue") || d.Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("untagged diagnostic: %+v", d)
	}
}

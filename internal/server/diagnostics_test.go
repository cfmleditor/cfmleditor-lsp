package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestKnownIssuesMergeWithOtherDiagnosticsAndReload covers the store the
// known-issues feature needed. A publish replaces everything the server sent
// for a file, so known issues have to be merged with CFLint's and the parse
// scan's rather than wiping them or being wiped; they are about the file, so
// closing its buffer keeps them; and editing the known-issues file withdraws an
// entry it no longer lists.
func TestKnownIssuesMergeWithOtherDiagnosticsAndReload(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()

		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		return p
	}

	a := write("packages/a.cfc", "<cfcomponent>\n<cfset x = svc.run()>\n</cfcomponent>\n")
	b := write("packages/b.cfc", "<cfcomponent>\n<cfset y = 1>\n</cfcomponent>\n")
	known := write(".cfmleditor-unresolved.txt", "# baseline\npackages/a.cfc:2: svc.run (method 'run' not found in x)\npackages/b.cfc:2: TODO tidy\n")

	s := newTestServer()
	s.KnownIssues = []config.KnownIssues{{File: known, Severity: "warning", Scope: "workspace"}}

	ctx := context.Background()
	s.loadKnownIssues(ctx)

	aURI, bURI := uri.File(a), uri.File(b)
	if got := s.diagnosticsFor(aURI); len(got) != 1 || got[0].Severity != protocol.DiagnosticSeverityWarning {
		t.Fatalf("a after load: %+v", got)
	}

	// CFLint reports on a; both are published together.
	s.storeDiagnostics(aURI, sourceCFLint, []protocol.Diagnostic{{Message: protocol.String("lint")}})

	if got := s.diagnosticsFor(aURI); len(got) != 2 {
		t.Fatalf("a with lint: %d diagnostics, want 2", len(got))
	}

	// Closing a's buffer drops the lint and keeps the known issue.
	s.storeDiagnostics(aURI, sourceCFLint, nil)
	s.storeDiagnostics(aURI, sourceParse, nil)

	if got := s.diagnosticsFor(aURI); len(got) != 1 || got[0].Code != protocol.String("known-issue") {
		t.Fatalf("a after close: %+v", got)
	}

	// b's entry is removed from the file; a reload withdraws it.
	write(".cfmleditor-unresolved.txt", "packages/a.cfc:2: svc.run (method 'run' not found in x)\n")
	s.loadKnownIssuesFile(ctx, known)

	if got := s.diagnosticsFor(bURI); len(got) != 0 {
		t.Errorf("b after its entry was removed: %+v", got)
	}

	if got := s.diagnosticsFor(aURI); len(got) != 1 {
		t.Errorf("a after reload: %+v", got)
	}

	if !s.isKnownIssuesFile(known) || s.isKnownIssuesFile(a) {
		t.Error("isKnownIssuesFile disagrees with the config")
	}
}

// TestOpenScopedKnownIssuesFollowTheBuffer covers scope "open": a file's
// entries are held but not shown while it is closed, appear when it opens, and
// go again when it closes, without disturbing a workspace-scoped file's.
func TestOpenScopedKnownIssuesFollowTheBuffer(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "a.cfc")
	if err := os.WriteFile(src, []byte("<cfcomponent>\n<cfset x = svc.run()>\n</cfcomponent>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	openFile := filepath.Join(dir, "todo.txt")
	wsFile := filepath.Join(dir, "baseline.txt")

	for f, body := range map[string]string{openFile: "a.cfc:1: TODO revisit\n", wsFile: "a.cfc:2: svc.run (method 'run' not found in x)\n"} {
		if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	s := newTestServer()
	s.KnownIssues = []config.KnownIssues{{File: openFile}, {File: wsFile, Scope: "workspace"}} // open is the default

	ctx := context.Background()
	s.loadKnownIssues(ctx)

	u := uri.File(src)
	if got := s.diagnosticsFor(u); len(got) != 1 {
		t.Fatalf("closed: %d diagnostics, want 1 (the workspace-scoped one)", len(got))
	}

	s.diagnosticsOpened(ctx, u)

	if got := s.diagnosticsFor(u); len(got) != 2 {
		t.Fatalf("open: %d diagnostics, want 2", len(got))
	}

	s.diagnosticsClosed(u)

	if got := s.diagnosticsFor(u); len(got) != 1 {
		t.Fatalf("closed again: %d diagnostics, want 1", len(got))
	}
}

// TestCFLintReportGivesWayToALiveRun covers a generated CFLint report beside
// CFLint on save. The report's entries are labelled cflint and carry the rule;
// once a file is linted, the report's entries for it are hidden, even when the
// run finds nothing, so no issue is listed twice or outlives its fix; and a
// regenerated report shows again for files no longer open.
func TestCFLintReportGivesWayToALiveRun(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.cfm")
	b := filepath.Join(dir, "b.cfm")

	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("<cfset x = 1>\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report := filepath.Join(dir, ".cfmleditor-cflint.txt")
	if err := os.WriteFile(report, []byte("a.cfm:1:1: [warning MISSING_VAR] x\nb.cfm:1:1: [warning MISSING_VAR] x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := newTestServer()
	s.KnownIssues = []config.KnownIssues{{File: report, Generate: config.GenerateCFLint, Scope: "workspace"}}

	ctx := context.Background()
	s.loadKnownIssues(ctx)

	aURI, bURI := uri.File(a), uri.File(b)

	got := s.diagnosticsFor(aURI)
	if len(got) != 1 || got[0].Source != protocol.NewOptional(sourceCFLint) || got[0].Code != protocol.String("MISSING_VAR") {
		t.Fatalf("a from the report: %+v", got)
	}

	// a is opened and linted clean: the report's entry goes.
	s.diagnosticsOpened(ctx, aURI)
	s.diagnosticsRan(aURI, sourceCFLint)
	s.storeDiagnostics(aURI, sourceCFLint, []protocol.Diagnostic{})

	if got := s.diagnosticsFor(aURI); len(got) != 0 {
		t.Errorf("a after a clean run: %+v", got)
	}

	if got := s.diagnosticsFor(bURI); len(got) != 1 {
		t.Errorf("b, never linted, lost its report entry: %+v", got)
	}

	// Closing a keeps the report's stale entry hidden.
	s.diagnosticsClosed(aURI)
	s.storeDiagnostics(aURI, sourceCFLint, nil)

	if got := s.diagnosticsFor(aURI); len(got) != 0 {
		t.Errorf("a after close: %+v", got)
	}

	// A regenerated report is fresher than a closed file's run.
	s.loadKnownIssuesFile(ctx, report)

	if got := s.diagnosticsFor(aURI); len(got) != 1 {
		t.Errorf("a after the report was regenerated: %+v", got)
	}
}

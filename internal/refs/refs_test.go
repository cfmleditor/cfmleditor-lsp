package refs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func testdataDir() string {
	_, f, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(f), "..", "..", "testdata", "refs")
}

// TestTrace_OnlyMatchesTargetComponent verifies that "find all calls to"
// persist.cfc::GetData only finds calls that resolve to persist.cfc,
// not calls to service.cfc::GetData or controller.cfc::GetData.
func TestTrace_OnlyMatchesTargetComponent(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	persistPath, _ := filepath.Abs(filepath.Join(dir, "persist.cfc"))
	servicePath, _ := filepath.Abs(filepath.Join(dir, "service.cfc"))

	opts := Options{
		FuncName:   "GetData",
		SourceFile: persistPath,
		VerifyTarget: func(component, _, sourceFile string) bool {
			switch component {
			case "persist":
				return sourceFile == persistPath
			case "service":
				return sourceFile == servicePath
			}

			return false
		},
	}

	entries := Trace(fs, roots, opts)

	// Only consider resolved entries
	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	viewPath, _ := filepath.Abs(filepath.Join(dir, "view.cfm"))

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile == controllerPath {
			t.Errorf("should not match controller.cfc: line %d %s", e.Line, e.Call)
		}

		if absFile == viewPath {
			t.Errorf("should not match view.cfm: line %d %s", e.Line, e.Call)
		}
	}

	found := false

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile == servicePath && e.Function == "GetData" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected call in service.cfc::GetData, got %d entries", len(entries))

		for _, e := range entries {
			t.Logf("  %s:%d func=%s call=%s", e.File, e.Line, e.Function, e.Call)
		}
	}
}

// TestTrace_RecursesUpstream verifies that same-file callers are traced recursively.
// controller.cfc::GetReport calls GetData (same file), so tracing GetData should
// find GetReport, then recursively find view.cfm calling GetReport.
// It should NOT find a.cfc, b.cfc, persist.cfc, or report_view.cfm.
func TestTrace_RecursesUpstream(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))

	opts := Options{
		FuncName:   "GetData",
		SourceFile: controllerPath,
		VerifyTarget: func(component, fileDir, sourceFile string) bool {
			if component == "controller" {
				resolved, _ := filepath.Abs(filepath.Join(fileDir, "controller.cfc"))

				return resolved == sourceFile
			}

			return false
		},
	}

	entries := Trace(fs, roots, opts)

	// Only consider resolved entries
	// Negative assertions — unrelated files must not appear
	excluded := map[string]string{
		"a.cfc":       filepath.Join(dir, "a.cfc"),
		"b.cfc":       filepath.Join(dir, "b.cfc"),
		"persist.cfc": filepath.Join(dir, "persist.cfc"),
	}
	for name, path := range excluded {
		absPath, _ := filepath.Abs(path)

		for _, e := range entries {
			if !e.Resolved {
				continue
			}

			absFile, _ := filepath.Abs(e.File)
			if absFile == absPath {
				t.Errorf("should not match %s: line %d func=%s call=%s", name, e.Line, e.Function, e.Call)
			}
		}
	}

	// Positive assertions
	foundGetReport := false
	foundView := false
	viewPath, _ := filepath.Abs(filepath.Join(dir, "view.cfm"))

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile == controllerPath && e.Function == "GetReport" {
			foundGetReport = true
		}

		if absFile == viewPath {
			foundView = true
		}
	}

	if !foundGetReport {
		t.Errorf("expected same-file caller GetReport")

		for _, e := range entries {
			t.Logf("  %s:%d func=%s call=%s", e.File, e.Line, e.Function, e.Call)
		}
	}

	if !foundView {
		t.Errorf("expected view.cfm calling GetReport (recursive)")

		for _, e := range entries {
			t.Logf("  %s:%d func=%s call=%s", e.File, e.Line, e.Function, e.Call)
		}
	}
}

// TestTrace_DoesNotMatchSameNameInOtherFile verifies that an unqualified call
// to DoWork() in b.cfc does not match when tracing a.cfc::DoWork.
// Also verifies that controller.cfc, service.cfc, persist.cfc are excluded.
func TestTrace_DoesNotMatchSameNameInOtherFile(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	aPath, _ := filepath.Abs(filepath.Join(dir, "a.cfc"))

	opts := Options{
		FuncName:   "DoWork",
		SourceFile: aPath,
		VerifyTarget: func(_, _, _ string) bool {
			return false // no qualified calls in this test
		},
	}

	entries := Trace(fs, roots, opts)

	// Only consider resolved entries
	var resolved []Entry

	for _, e := range entries {
		if e.Resolved {
			resolved = append(resolved, e)
		}
	}

	// Negative assertions — no file other than a.cfc should appear
	for _, e := range resolved {
		absFile, _ := filepath.Abs(e.File)
		if absFile != aPath {
			t.Errorf("should not match %s: line %d func=%s call=%s", filepath.Base(e.File), e.Line, e.Function, e.Call)
		}
	}

	// Positive: should find a.cfc::Process and a.cfc::GetReport (both call DoWork)
	foundProcess := false
	foundGetReport := false

	for _, e := range resolved {
		absFile, _ := filepath.Abs(e.File)
		if absFile == aPath && e.Function == "Process" {
			foundProcess = true
		}

		if absFile == aPath && e.Function == "GetReport" {
			foundGetReport = true
		}
	}

	if !foundProcess {
		t.Errorf("expected a.cfc::Process")
	}

	if !foundGetReport {
		t.Errorf("expected a.cfc::GetReport")
	}

	if t.Failed() {
		for _, e := range entries {
			t.Logf("  %s:%d func=%s call=%s", e.File, e.Line, e.Function, e.Call)
		}
	}
}

// TestTrace_SameFileMatchToleratesPathFormatting verifies that a same-file bare
// call still matches when SourceFile (derived from a client-supplied document
// URI) differs cosmetically from the path produced by walking the workspace —
// e.g. case differences or redundant "./" segments. This mirrors how the
// findRefs LSP command previously lost same-file matches when the URI-decoded
// path didn't compare byte-for-byte equal to the filesystem-walked path.
func TestTrace_SameFileMatchToleratesPathFormatting(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	aPath, _ := filepath.Abs(filepath.Join(dir, "a.cfc"))
	messySourceFile := filepath.Join(filepath.Dir(aPath), ".", strings.ToUpper(filepath.Base(aPath)))

	opts := Options{
		FuncName:   "DoWork",
		SourceFile: messySourceFile,
		VerifyTarget: func(_, _, _ string) bool {
			return false // no qualified calls in this test
		},
	}

	entries := Trace(fs, roots, opts)

	foundProcess := false
	foundGetReport := false

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile != aPath {
			continue
		}

		switch e.Function {
		case "Process":
			foundProcess = true
		case "GetReport":
			foundGetReport = true
		}
	}

	if !foundProcess {
		t.Errorf("expected a.cfc::Process to match despite cosmetic SourceFile differences")
	}

	if !foundGetReport {
		t.Errorf("expected a.cfc::GetReport to match despite cosmetic SourceFile differences")
	}
}

// TestTrace_GetReportChainIsolated verifies that tracing controller.cfc::GetReport
// finds RunReport (same-file caller) and report_view.cfm (calls RunReport),
// but does NOT find a.cfc::RunReport, b.cfc::RunReport, or other unrelated files.
func TestTrace_GetReportChainIsolated(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))

	opts := Options{
		FuncName:   "GetReport",
		SourceFile: controllerPath,
		VerifyTarget: func(component, fileDir, sourceFile string) bool {
			if component == "controller" {
				resolved, _ := filepath.Abs(filepath.Join(fileDir, "controller.cfc"))

				return resolved == sourceFile
			}

			return false
		},
	}

	entries := Trace(fs, roots, opts)

	// Only consider resolved entries
	// Negative assertions — unrelated files must not appear
	excluded := []string{"a.cfc", "b.cfc", "persist.cfc", "service.cfc"}
	for _, name := range excluded {
		absPath, _ := filepath.Abs(filepath.Join(dir, name))

		for _, e := range entries {
			if !e.Resolved {
				continue
			}

			absFile, _ := filepath.Abs(e.File)
			if absFile == absPath {
				t.Errorf("should not match %s: line %d func=%s call=%s", name, e.Line, e.Function, e.Call)
			}
		}
	}

	// Positive: should find controller.cfc::RunReport (same-file caller of GetReport)
	foundRunReport := false

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile == controllerPath && e.Function == "RunReport" {
			foundRunReport = true
		}
	}

	if !foundRunReport {
		t.Errorf("expected controller.cfc::RunReport")
	}

	// Positive: should find report_view.cfm calling RunReport (recursive)
	reportViewPath, _ := filepath.Abs(filepath.Join(dir, "report_view.cfm"))
	foundReportView := false

	for _, e := range entries {
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)
		if absFile == reportViewPath {
			foundReportView = true
		}
	}

	if !foundReportView {
		t.Errorf("expected report_view.cfm calling RunReport (recursive)")
	}

	if t.Failed() {
		for _, e := range entries {
			t.Logf("  %s:%d func=%s call=%s", e.File, e.Line, e.Function, e.Call)
		}
	}
}

// TestTrace_DepthAndViaAnnotateChain verifies that Trace stamps Depth/Via/ViaFile
// on entries reached through recursion, so a caller can tell a direct call to the
// traced target apart from a call to a same-named wrapper that itself, further
// down the chain, reaches the target — and that FormatResult renders this as
// separate "Direct" / "Indirect ... via ..." groups instead of one flat list.
func TestTrace_DepthAndViaAnnotateChain(t *testing.T) {
	dir := testdataDir()
	fs := vfs.OS{}
	roots := []string{dir}

	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))

	opts := Options{
		FuncName:   "GetReport",
		SourceFile: controllerPath,
		VerifyTarget: func(component, fileDir, sourceFile string) bool {
			if component == "controller" {
				resolved, _ := filepath.Abs(filepath.Join(fileDir, "controller.cfc"))

				return resolved == sourceFile
			}

			return false
		},
	}

	entries := Trace(fs, roots, opts)

	var runReportEntry, reportViewEntry *Entry

	for i := range entries {
		e := &entries[i]
		if !e.Resolved {
			continue
		}

		absFile, _ := filepath.Abs(e.File)

		switch {
		case absFile == controllerPath && e.Function == "RunReport":
			runReportEntry = e
		case strings.HasSuffix(absFile, "report_view.cfm"):
			reportViewEntry = e
		}
	}

	if runReportEntry == nil {
		t.Fatal("expected controller.cfc::RunReport entry")
	}

	if runReportEntry.Depth != 0 {
		t.Errorf("expected controller.cfc::RunReport's call to GetReport to be depth 0 (direct), got %d", runReportEntry.Depth)
	}

	if reportViewEntry == nil {
		t.Fatal("expected report_view.cfm entry")
	}

	if reportViewEntry.Depth != 1 {
		t.Errorf("expected report_view.cfm's call to be depth 1 (indirect), got %d", reportViewEntry.Depth)
	}

	if !strings.EqualFold(reportViewEntry.Via, "RunReport") {
		t.Errorf("expected report_view.cfm entry Via = RunReport, got %q", reportViewEntry.Via)
	}

	result := FormatResult(entries, "GetReport", "file://"+controllerPath, roots)

	if !strings.Contains(result.Summary, "Direct calls:") {
		t.Errorf("expected Summary to contain a Direct calls section, got:\n%s", result.Summary)
	}

	if !strings.Contains(result.Summary, "Indirect (1 hop) — via") {
		t.Errorf("expected Summary to contain an Indirect via section, got:\n%s", result.Summary)
	}
}

// TestFind_UnresolvedEntryCarriesReason verifies that when Options.Reason is
// supplied (internal/server wires it to resolve.Resolver.CanResolveCall), an
// entry whose call couldn't be verified against the traced target carries a
// Reason string instead of just Resolved=false, and that FormatResult surfaces
// it in the "[unresolved: ...]" marker instead of a bare "[unresolved]".
func TestFind_UnresolvedEntryCarriesReason(t *testing.T) {
	dir := t.TempDir()

	content := "<cfset someUnknownVar.GetReport()>"
	if err := os.WriteFile(filepath.Join(dir, "caller.cfm"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := vfs.OS{}
	roots := []string{dir}

	opts := Options{
		FuncName: "GetReport",
		Reason: func(call parser.CallSite, _ *parser.ParseResult, _ string) string {
			return "variable '" + call.Variable + "' has no component ref"
		},
	}

	entries := Find(fs, roots, opts)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}

	e := entries[0]
	if e.Resolved {
		t.Fatalf("expected entry to be unresolved, got resolved: %+v", e)
	}

	wantReason := "variable 'someUnknownVar' has no component ref (needed for someUnknownVar.GetReport())"
	if e.Reason != wantReason {
		t.Errorf("expected Reason %q, got %q", wantReason, e.Reason)
	}

	result := FormatResult(entries, "GetReport", "file://"+filepath.Join(dir, "x.cfm"), roots)
	if !strings.Contains(result.Summary, "[unresolved: "+wantReason+"]") {
		t.Errorf("expected Summary to contain reason marker, got:\n%s", result.Summary)
	}
}

package refs

import (
	"path/filepath"
	"runtime"
	"testing"

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
		VerifyTarget: func(component, fileDir, sourceFile string) bool {
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

	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	viewPath, _ := filepath.Abs(filepath.Join(dir, "view.cfm"))
	for _, e := range entries {
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

	// Negative assertions — unrelated files must not appear
	excluded := map[string]string{
		"a.cfc":           filepath.Join(dir, "a.cfc"),
		"b.cfc":           filepath.Join(dir, "b.cfc"),
		"persist.cfc":     filepath.Join(dir, "persist.cfc"),
		"report_view.cfm": filepath.Join(dir, "report_view.cfm"),
	}
	for name, path := range excluded {
		absPath, _ := filepath.Abs(path)
		for _, e := range entries {
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
		VerifyTarget: func(component, fileDir, sourceFile string) bool {
			return false // no qualified calls in this test
		},
	}

	entries := Trace(fs, roots, opts)

	// Negative assertions — only a.cfc entries should appear
	excluded := []string{"b.cfc", "controller.cfc", "service.cfc", "persist.cfc", "view.cfm", "report_view.cfm"}
	for _, name := range excluded {
		absPath, _ := filepath.Abs(filepath.Join(dir, name))
		for _, e := range entries {
			absFile, _ := filepath.Abs(e.File)
			if absFile == absPath {
				t.Errorf("should not match %s: line %d func=%s call=%s", name, e.Line, e.Function, e.Call)
			}
		}
	}

	// Positive: should find a.cfc::Process and a.cfc::GetReport (both call DoWork)
	foundProcess := false
	foundGetReport := false
	for _, e := range entries {
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

	// Negative assertions — unrelated files must not appear
	excluded := []string{"a.cfc", "b.cfc", "persist.cfc", "service.cfc", "view.cfm"}
	for _, name := range excluded {
		absPath, _ := filepath.Abs(filepath.Join(dir, name))
		for _, e := range entries {
			absFile, _ := filepath.Abs(e.File)
			if absFile == absPath {
				t.Errorf("should not match %s: line %d func=%s call=%s", name, e.Line, e.Function, e.Call)
			}
		}
	}

	// Positive: should find controller.cfc::RunReport (same-file caller of GetReport)
	foundRunReport := false
	for _, e := range entries {
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

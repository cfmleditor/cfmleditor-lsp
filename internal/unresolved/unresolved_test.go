package unresolved

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteKnownIssuesIsProjectRelative(t *testing.T) {
	base := filepath.FromSlash("/work/tassweb")
	results := []Call{
		{File: filepath.FromSlash("/work/tassweb/webroot/b.cfm"), Line: 9, Function: "tableCell", Reason: "no qualifier, not in file"},
		{File: filepath.FromSlash("/work/tassweb/packages/a.cfc"), Line: 1, Variable: "svc", Function: "run", Reason: "method 'run' not found in x"},
		{File: filepath.FromSlash("/work/kiosk/c.cfm"), Line: 0, Function: "go", Reason: "no qualifier, not in file"},
	}

	var out bytes.Buffer
	if skipped := WriteKnownIssues(&out, results, base, false, RegenerateHint, "test"); skipped != 1 {
		t.Errorf("skipped %d, want 1 (the sibling project)", skipped)
	}

	var entries []string

	for l := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		if !strings.HasPrefix(l, "#") {
			entries = append(entries, l)
		}
	}

	want := []string{
		"packages/a.cfc:2: svc.run (method 'run' not found in x)",
		"webroot/b.cfm:10: tableCell (no qualifier, not in file)",
	}

	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(entries, "\n"), strings.Join(want, "\n"))
	}

	if strings.Contains(out.String(), "/work/") {
		t.Error("an absolute path was written")
	}

	out.Reset()
	WriteKnownIssues(&out, results, base, true, RegenerateHint, "test")

	if !strings.Contains(out.String(), "\n../kiosk/c.cfm:1: go (no qualifier, not in file)\n") {
		t.Errorf("--include-workspace did not write the sibling as a ../ path:\n%s", out.String())
	}
}

// TestSplitByTargetGivesEachReportItsOwnDirectory covers several generated
// files of one kind: each gets the calls under its own directory, the deepest
// when directories nest, and a call under none is handed back.
func TestSplitByTargetGivesEachReportItsOwnDirectory(t *testing.T) {
	top := filepath.FromSlash("/work/.cfmleditor-unresolved.txt")
	web := filepath.FromSlash("/work/tassweb/.cfmleditor-unresolved.txt")
	calls := []Call{
		{File: filepath.FromSlash("/work/tassweb/a.cfc")},
		{File: filepath.FromSlash("/work/kiosk/b.cfm")},
		{File: filepath.FromSlash("/elsewhere/c.cfm")},
	}

	by, rest := SplitByTarget(calls, []string{top, web})

	if len(by[web]) != 1 || by[web][0].File != calls[0].File {
		t.Errorf("tassweb report: %+v", by[web])
	}

	if len(by[top]) != 1 || by[top][0].File != calls[1].File {
		t.Errorf("top report: %+v", by[top])
	}

	if len(rest) != 1 || rest[0].File != calls[2].File {
		t.Errorf("rest: %+v", rest)
	}
}

func TestIsBuiltin(t *testing.T) {
	for name, want := range map[string]bool{"trim": true, "TRIM": true, "append": true, "someUserDefinedFunctionXyz": false} {
		if got := IsBuiltin(name); got != want {
			t.Errorf("IsBuiltin(%q) = %v, want %v", name, got, want)
		}
	}

	for name, want := range map[string]bool{"append": true, "APPEND": true, "notARealMemberFunctionXyz": false} {
		if got := IsMemberFunction(name); got != want {
			t.Errorf("IsMemberFunction(%q) = %v, want %v", name, got, want)
		}
	}
}

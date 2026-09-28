package unresolved

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
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
	// A tag called as a function is built in too: cfheader( name = "x" ).
	// "cf" alone and a cf-prefixed name no tag has are not.
	for name, want := range map[string]bool{
		"trim": true, "TRIM": true, "append": true, "someUserDefinedFunctionXyz": false,
		"cfheader": true, "CFHTTP": true, "cf": false, "cfNotATag": false,
	} {
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

// TestAMissingBaseIsOneEntryPerFile: every inherited call in a file whose
// extends chain breaks is unchecked for the same reason, so the report holds
// one entry for the file, on its extends line and counting the calls — `toBe`
// among them, since it is chained on `expect`, and `print.line`, since `print`
// is not declared here and so is the base's — rather than one per call. `svc`
// is an argument, so the base cannot explain it, and it is reported as before.
func TestAMissingBaseIsOneEntryPerFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Spec.cfc")
	src := "/** a spec */\ncomponent extends=\"testbox.system.BaseSpec\" {\n" +
		"\tfunction run( svc ) {\n\t\tdescribe( \"x\", function() {\n\t\t\texpect( 1 ).toBe( 1 );\n\t\t\tit( \"y\", function() {} );\n" +
		"\t\t\tsvc.missing();\n\t\t\tprint.line( \"z\" );\n\t\t} );\n\t}\n}\n"

	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	rep := Scan(vfs.OS{}, []string{file}, nil, &Options{})

	var got []string

	for i := range rep.Calls {
		c := &rep.Calls[i]
		got = append(got, fmt.Sprintf("%d: %s (%s)", c.Line+1, c.CallText(), c.Reason))
	}

	want := []string{
		"2: testbox.system.BaseSpec (base component does not resolve; 5 inherited calls not checked)",
		"7: svc.missing (variable 'svc' has no component ref)",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if bases := MissingBases(rep.Calls); len(bases) != 1 || bases[0] != (MissingBase{Component: "testbox.system.BaseSpec", Files: 1, Calls: 5}) {
		t.Errorf("MissingBases: got %+v", bases)
	}
}

// TestCallsOnAComponentWithAMissingBaseAreOneEntry: calls in a file made on
// a component whose chain breaks are one entry for that base, on the first
// of them, beside the file's own missing base, which stays on its extends
// line.
func TestCallsOnAComponentWithAMissingBaseAreOneEntry(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"Service.cfc": "component extends=\"cborm.models.VirtualEntityService\" {}\n",
		"Handler.cfc": "component extends=\"coldbox.system.EventHandler\" {\n" +
			"\tproperty name=\"svc\" inject=\"Service@app\";\n" +
			"\tfunction index( event ) {\n\t\tsvc.findWhere();\n\t\tsvc.save();\n\t\tsetNextEvent();\n\t}\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{filepath.Join(dir, "Service.cfc"), filepath.Join(dir, "Handler.cfc")}
	rep := Scan(vfs.OS{}, files, files[1:], &Options{})

	var got []string

	for i := range rep.Calls {
		c := &rep.Calls[i]
		got = append(got, fmt.Sprintf("%d: %s (%s)", c.Line+1, c.CallText(), c.Reason))
	}

	want := []string{
		"1: coldbox.system.EventHandler (base component does not resolve; 1 inherited call not checked)",
		"4: cborm.models.VirtualEntityService (calls a component whose chain breaks at cborm.models.VirtualEntityService, which does not resolve; 2 calls not checked)",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

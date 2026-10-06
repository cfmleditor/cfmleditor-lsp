package unresolved

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// TestCandidatesAnnotateWithoutResolving: a scan with Candidates keeps every
// finding and offers the liberal matches beside it. A receiver's candidate is
// the component declaring every method called on it — Content, not Other or
// Third, which declare only one each — and its confidence is high when it is the only
// one; a bare call's candidates are the method's definitions anywhere; and a
// component path that names no file offers the files of that name.
func TestCandidatesAnnotateWithoutResolving(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"models/Content.cfc": `component { function getTitle(){} function getBody(){} }`,
		"models/Other.cfc":   `component { function getTitle(){} }`,
		"models/Third.cfc":   `component { function getBody(){} }`,
		"lib/Helpers.cfc":    `component { function helper(){} }`,
		"lib2/Helpers.cfc":   `component { function helper(){} }`,
		"page.cfm": `<cfscript>
function show( item ){
	item.getTitle();
	item.getBody();
	helper();
	nowhere();
	var u = new models.Usr();
	u.save();
}
</cfscript>`,
		"elsewhere/Usr.cfc": `component { function save(){} }`,
	}

	var paths []string

	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}

		paths = append(paths, p)
	}

	page := filepath.Join(dir, "page.cfm")

	plain := Scan(vfs.OS{}, paths, []string{page}, &Options{WorkspaceFolders: []string{dir}})
	annotated := Scan(vfs.OS{}, paths, []string{page}, &Options{WorkspaceFolders: []string{dir}, Candidates: true})

	if len(plain.Calls) != len(annotated.Calls) {
		t.Fatalf("candidates changed the findings: %d without, %d with", len(plain.Calls), len(annotated.Calls))
	}

	by := map[string]*Call{}
	for i := range annotated.Calls {
		by[annotated.Calls[i].Function] = &annotated.Calls[i]
	}

	if c := by["getBody"]; c == nil || c.Category != CategoryVariable || len(c.Candidates) != 1 ||
		filepath.Base(c.Candidates[0].File) != "Content.cfc" || c.Candidates[0].Confidence != ConfidenceHigh {
		t.Errorf("item.getBody(): %+v, want one high candidate, Content.cfc", c)
	}

	if c := by["helper"]; c == nil || c.Category != CategoryMethod || c.CandidateCount != 2 || c.Candidates[0].Confidence != ConfidenceLow || c.Definitions == nil || *c.Definitions != 2 {
		t.Errorf("helper(): %+v, want two low candidates and two definitions", c)
	}

	if c := by["nowhere"]; c == nil || c.CandidateCount != 0 || c.Definitions == nil || *c.Definitions != 0 {
		t.Errorf("nowhere(): %+v, want no candidate and no definition", c)
	}

	if c := by["save"]; c == nil || c.Category != CategoryObject || len(c.Candidates) != 1 || filepath.Base(c.Candidates[0].File) != "Usr.cfc" {
		t.Errorf("u.save(): %+v, want the file named Usr.cfc", c)
	}
}

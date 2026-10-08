package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/unresolved"
	"github.com/cfmleditor/clif/internal/vfs"
)

// TestUnresolvedWithoutAConfig runs a scan the way cmdUnresolved does when no
// .cfmleditor.json governs the directory. Every call below is made on a
// component that exists and calls a method it lacks, so the report names the
// component the call resolved to: "not found in X" means X was found.
//
// What resolves with no config is what the source tree itself says: a
// component beside the caller, a dot-path from the directory given, one that
// starts with that directory's own name, and an Application.cfc mapping.
// What needs a config is the last case, which types a variable from a factory
// call and so has no component ref.
func TestUnresolvedWithoutAConfig(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proj")
	files := map[string]string{
		"models/User.cfc": "component { function real(){} }",
		"lib/Util.cfc":    "component { function real(){} }",
		"svc/Sib.cfc":     "component { function real(){} }",
		"Application.cfc": `component { this.mappings["/mylib"] = expandPath("./lib"); }`,
		"svc/Main.cfc": `component {
  function a(){ var s = new Sib(); s.nope1(); }
  function b(){ var u = new models.User(); u.nope2(); }
  function c(){ var u = new proj.models.User(); u.nope3(); }
  function d(){ var u = new mylib.Util(); u.nope4(); }
  function e(){ var u = getModel("models.User"); u.nope5(); }
}`,
	}

	for rel, src := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	fsys := vfs.OS{}
	opt := unresolvedOptions(nil, []string{root}, &unresolvedFlags{})
	rep := unresolved.Scan(fsys, collectCFMLFiles(fsys, opt.WorkspaceFolders), nil, opt)

	reasons := make(map[string]string)

	for i := range rep.Calls {
		c := &rep.Calls[i]
		if filepath.Base(c.File) == "Main.cfc" {
			reasons[c.Function] = c.Reason
		}
	}

	want := map[string]string{
		"nope1": "method 'nope1' not found in Sib",
		"nope2": "method 'nope2' not found in models.User",
		"nope3": "method 'nope3' not found in proj.models.User",
		"nope4": "method 'nope4' not found in mylib.Util",
		"nope5": "variable 'u' has no component ref",
		// The factory call is itself unknown: nothing declares getModel.
		"getModel": "no qualifier, not in file",
	}

	for fn, reason := range want {
		if reasons[fn] != reason {
			t.Errorf("%s: got %q, want %q", fn, reasons[fn], reason)
		}
	}

	if len(reasons) != len(want) {
		t.Errorf("got %d Main.cfc entries, want %d: %v", len(reasons), len(want), reasons)
	}
}

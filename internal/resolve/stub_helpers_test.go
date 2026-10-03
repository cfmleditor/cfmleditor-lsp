package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
)

// TestAModulesHelperComesFromItsStubWhenTheModuleIsAbsent: ContentBox depends
// on cbmessagebox and does not ship it, so cbMessageBox() — which the module's
// mixins.cfm declares for every handler and view — was 275 findings. The
// contentbox preset brings the module's stub helper, offered last. Without the
// preset, or from a file the helpers do not reach, the call is still reported,
// and a method the module's MessageBox lacks is reported too.
func TestAModulesHelperComesFromItsStubWhenTheModuleIsAbsent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"handlers/Main.cfc": `component { function run(){ cbMessageBox().warn( "x" ); cbMessageBox().nope(); } }`,
		"models/User.cfc":   `component { function run(){ cbMessageBox().warn( "x" ); } }`,
	})

	scope := func(path string) bool { return strings.Contains(filepath.ToSlash(path), "/handlers/") }

	got := reasonsWith(t, &Resolver{HelperScope: scope, Stubs: frameworkapi.For([]string{"contentbox"})}, dir, "handlers/Main.cfc")
	if got["cbMessageBox.warn"] != "" || got["cbMessageBox"] != "" {
		t.Errorf("stubbed helper not found: %v", got)
	}

	if !strings.Contains(got["cbMessageBox.nope"], "not found in") {
		t.Errorf("a method MessageBox lacks was accepted: %q", got["cbMessageBox.nope"])
	}

	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope, Stubs: frameworkapi.For([]string{"coldbox"})}, dir, "handlers/Main.cfc"), map[string]string{
		"cbMessageBox":      "no qualifier, not in file",
		"cbMessageBox.warn": "chained on 'cbMessageBox', which is not found (calling 'warn')",
		"cbMessageBox.nope": "chained on 'cbMessageBox', which is not found (calling 'nope')",
	})
	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope, Stubs: frameworkapi.For([]string{"contentbox"})}, dir, "models/User.cfc"), map[string]string{
		"cbMessageBox":      "no qualifier, not in file",
		"cbMessageBox.warn": "chained on 'cbMessageBox', which is not found (calling 'warn')",
	})
}

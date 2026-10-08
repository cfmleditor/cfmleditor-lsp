package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/frameworkapi"
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

// TestAValidationResultIsTypedFromTheStubbedHelper: ContentBox's handlers write
// `var vResults = validate( … )` and then `vResults.hasErrors()`. cbvalidation
// is not shipped, and validate() is a helper the parse cannot see, so the
// variable is typed at lookup from the stub's declared ValidationResult. A
// method the result lacks is still reported.
func TestAValidationResultIsTypedFromTheStubbedHelper(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"handlers/Main.cfc": `component {
	function save(){
		var vResults = validate( target = rc, excludes = "password" );
		if ( !vResults.hasErrors() ) {}
		vResults.getAllErrors();
		vResults.nope();
	}
}`,
	})

	scope := func(path string) bool { return strings.Contains(filepath.ToSlash(path), "/handlers/") }

	got := reasonsWith(t, &Resolver{HelperScope: scope, Stubs: frameworkapi.For([]string{"contentbox"})}, dir, "handlers/Main.cfc")
	if got["vResults.hasErrors"] != "" || got["vResults.getAllErrors"] != "" {
		t.Errorf("validation result not typed: %v", got)
	}

	if !strings.Contains(got["vResults.nope"], "not found in") {
		t.Errorf("a method ValidationResult lacks was accepted: %q", got["vResults.nope"])
	}
}

// TestCBSecurityHelpersComeFromTheirStubs: jwtAuth() and cbSecure() are
// cbsecurity's helpers, each a WireBox instance, and ContentBox chains on them.
func TestCBSecurityHelpersComeFromTheirStubs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"handlers/Main.cfc": `component { function run(){ jwtAuth().fromUser( u ); cbSecure().secure( "x" ); jwtAuth().nope(); } }`,
	})

	scope := func(path string) bool { return strings.Contains(filepath.ToSlash(path), "/handlers/") }

	got := reasonsWith(t, &Resolver{HelperScope: scope, Stubs: frameworkapi.For([]string{"contentbox"})}, dir, "handlers/Main.cfc")
	if got["jwtAuth.fromUser"] != "" || got["cbSecure.secure"] != "" {
		t.Errorf("cbsecurity helpers not typed: %v", got)
	}

	if !strings.Contains(got["jwtAuth.nope"], "not found in") {
		t.Errorf("a method JwtService lacks was accepted: %q", got["jwtAuth.nope"])
	}
}

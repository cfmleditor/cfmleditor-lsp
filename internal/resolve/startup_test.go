package resolve_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// startupWorkspace is tassweb's shape in miniature: an Application.cfc that
// includes a startup template, which creates the kernel in REQUEST scope and
// asks it for the context; a second template the first includes; and the
// components they name. Pages under the application read REQUEST.context
// without assigning it.
func startupWorkspace(t *testing.T) string {
	t.Helper()

	app := t.TempDir()

	files := map[string]string{
		"Application.cfc": `<cfcomponent><cffunction name="onRequestStart"><cfinclude template="startup.cfm"></cffunction></cfcomponent>`,
		"startup.cfm": `<cfset REQUEST.kernel = createObject("component", "lib.Kernel").init()>
<cfset REQUEST.context = REQUEST.kernel.getContext()>
<cfset REQUEST.loop1 = REQUEST.loop2.a()>
<cfset REQUEST.loop2 = REQUEST.loop1.b()>
<cfinclude template="more.cfm">`,
		"more.cfm": `<cfscript>
SESSION.helper = new lib.Helper();
</cfscript>`,
		"lib/Kernel.cfc":  `<cfcomponent><cffunction name="init"><cfreturn this></cffunction><cffunction name="getContext" returntype="any"><cfreturn createObject("component", "lib.Context")></cffunction></cfcomponent>`,
		"lib/Context.cfc": `<cfcomponent><cffunction name="getUser"></cffunction></cfcomponent>`,
		"lib/Helper.cfc":  `<cfcomponent><cffunction name="help"></cffunction></cfcomponent>`,
	}

	for rel, src := range files {
		p := filepath.Join(app, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return app
}

// reasonsFor parses src as a page at path and resolves every call in it,
// keyed "variable.func".
func reasonsFor(t *testing.T, r *resolve.Resolver, path, src string) map[string]string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(path), src, &parser.ParseOptions{ExtractCalls: true})

	out := map[string]string{}

	calls := pr.AllCalls()
	for i := range calls {
		c := &calls[i]
		out[c.Variable+"."+c.FuncName] = r.CanResolveCall(c, pr, filepath.Dir(path))
	}

	return out
}

// TestSharedScopeFromTheApplicationsStartupTemplate: REQUEST.context is typed
// by the assignment in the template Application.cfc includes, through the
// kernel that template created and the return type of its getContext(), and
// a method the component lacks is still reported — the answer is checked, not
// waved through. SESSION.helper is set in script syntax, one include further.
func TestSharedScopeFromTheApplicationsStartupTemplate(t *testing.T) {
	app := startupWorkspace(t)
	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{app}}

	got := reasonsFor(t, r, filepath.Join(app, "pages", "page.cfm"), `<cfoutput>
<cfset u = REQUEST.context.getUser()>
<cfset x = REQUEST.context.nope()>
<cfset h = SESSION.helper.help()>
<cfset l = REQUEST.loop1.c()>
</cfoutput>`)

	if reason := got["REQUEST.context.getUser"]; reason != "" {
		t.Errorf("REQUEST.context.getUser() did not resolve: %s", reason)
	}

	if reason := got["REQUEST.context.nope"]; !strings.Contains(reason, "not found") {
		t.Errorf("REQUEST.context.nope() should be reported as missing from the context, got %q", reason)
	}

	if reason := got["SESSION.helper.help"]; reason != "" {
		t.Errorf("SESSION.helper.help(), set by a template the startup template includes, did not resolve: %s", reason)
	}

	// Two assignments typed by each other name nothing, and do not hang.
	if reason := got["REQUEST.loop1.c"]; !strings.Contains(reason, "no component ref") {
		t.Errorf("REQUEST.loop1 should have no type, got %q", reason)
	}
}

// TestStartupTemplatesStayWithTheirApplication: the lookup follows the
// Application.cfc governing the calling file, so a file under no application
// learns nothing from one, and an assignment the calling file makes itself
// is still the one that counts.
func TestStartupTemplatesStayWithTheirApplication(t *testing.T) {
	app := startupWorkspace(t)
	outside := t.TempDir()

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{app, outside}}

	got := reasonsFor(t, r, filepath.Join(outside, "page.cfm"), `<cfset u = REQUEST.context.getUser()>`)
	if reason := got["REQUEST.context.getUser"]; !strings.Contains(reason, "no component ref") {
		t.Errorf("a file under no Application.cfc resolved REQUEST.context: %q", reason)
	}

	own := reasonsFor(t, r, filepath.Join(app, "pages", "own.cfm"), `<cfscript>
REQUEST.context = new lib.Helper();
REQUEST.context.help();
</cfscript>`)
	if reason := own["REQUEST.context.help"]; reason != "" {
		t.Errorf("the calling file's own assignment did not win over the startup template's: %s", reason)
	}
}

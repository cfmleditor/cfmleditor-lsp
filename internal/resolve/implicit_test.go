package resolve

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// coldboxLike is a framework's source in miniature: a handler base and a
// renderer, both extending the supertype that declares the helpers.
var coldboxLike = map[string]string{
	"fw/system/FrameworkSupertype.cfc": `component { function getInstance( name ) {} function view() {} }`,
	"fw/system/EventHandler.cfc":       `component extends="fw.system.FrameworkSupertype" {}`,
	"fw/system/web/Renderer.cfc":       `component extends="fw.system.FrameworkSupertype" {}`,
}

// handlerBase stands in for a preset's rule: a .cfc under handlers/ is an
// EventHandler, a .cfm under views/ is rendered by the Renderer.
func handlerBase(path string) string {
	switch p := filepath.ToSlash(path); {
	case strings.Contains(p, "/handlers/") && strings.HasSuffix(p, ".cfc"):
		return "fw.system.EventHandler"
	case strings.Contains(p, "/views/") && strings.HasSuffix(p, ".cfm"):
		return "fw.system.web.Renderer"
	}

	return ""
}

func withFiles(extra map[string]string) map[string]string {
	all := maps.Clone(coldboxLike)
	maps.Copy(all, extra)

	return all
}

// TestAHandlerWithNoExtendsIsItsFrameworksBase: ColdBox 7 handlers need not
// say extends="coldbox.system.EventHandler", and a bare getInstance() in one
// was "no qualifier, not in file". With the preset's rule it is found on the
// implied base, a name no base declares is still reported, a view is rendered
// by the Renderer, and a file that names its own base keeps it.
func TestAHandlerWithNoExtendsIsItsFrameworksBase(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, withFiles(map[string]string{
		"app/handlers/Main.cfc":  `component { function index( event ) { getInstance( "x" ); nope(); } }`,
		"app/handlers/Own.cfc":   `component extends="app.Base" { function index() { getInstance( "x" ); } }`,
		"app/Base.cfc":           `component { function mine() {} }`,
		"app/views/main/idx.cfm": `<cfset view()><cfset nope2()>`,
		"app/models/Plain.cfc":   `component { function f() { getInstance( "x" ); } }`,
	}))

	r := func() *Resolver { return &Resolver{ImplicitExtends: handlerBase} }

	expectReasons(t, reasonsWith(t, r(), dir, "app/handlers/Main.cfc"), map[string]string{
		"getInstance": "",
		"nope":        "not found in extends chain",
	})
	expectReasons(t, reasonsWith(t, r(), dir, "app/handlers/Own.cfc"), map[string]string{
		"getInstance": "not found in extends chain",
	})
	expectReasons(t, reasonsWith(t, r(), dir, "app/views/main/idx.cfm"), map[string]string{
		"view":  "",
		"nope2": "not found in extends chain",
	})
	expectReasons(t, reasonsWith(t, r(), dir, "app/models/Plain.cfc"), map[string]string{
		"getInstance": "no qualifier, not in file",
	})
}

// TestAnImpliedBaseThatDoesNotResolveIsDynamic: without the framework's source
// in the workspace, a preset's resolvers accept calls as dynamic, and so does
// the base it implies — a handler's getInstance() is not a finding its code
// caused. A base the file names itself is still reported when it breaks.
func TestAnImpliedBaseThatDoesNotResolveIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"app/handlers/Main.cfc":     `component { function index() { getInstance( "x" ); log.info( "x" ); } }`,
		"app/handlers/Declared.cfc": `component extends="fw.system.EventHandler" { function index() { getInstance( "x" ); } }`,
	})

	r := func() *Resolver { return &Resolver{ImplicitExtends: handlerBase} }

	expectReasons(t, reasonsWith(t, r(), dir, "app/handlers/Main.cfc"), map[string]string{
		"getInstance": "",
		"log.info":    "",
	})
	expectReasons(t, reasonsWith(t, r(), dir, "app/handlers/Declared.cfc"), map[string]string{
		"getInstance": MissingBaseReason("fw.system.EventHandler"),
	})
}

// TestAnImpliedBaseOfAlternatives: a Wheels view runs inside its controller
// and every mixin the controller integrates, so the implied base is a list.
// A helper is found on whichever alternative declares it; with the framework
// absent the list is missing, not complete — it was read as a chain that
// resolved and lacked linkTo(), 327 findings in one example app.
func TestAnImpliedBaseOfAlternatives(t *testing.T) {
	viewBase := func(path string) string {
		if strings.Contains(filepath.ToSlash(path), "/views/") {
			return "fw.Controller|fw.view.Links"
		}

		return ""
	}

	withFw, without := t.TempDir(), t.TempDir()
	page := `<cfoutput>#linkTo( "x" )# #nope()#</cfoutput>`

	writeFiles(t, withFw, map[string]string{
		"fw/Controller.cfc": `component { function render() {} }`,
		"fw/view/Links.cfc": `component { function linkTo( text ) {} }`,
		"app/views/a.cfm":   page,
	})
	writeFiles(t, without, map[string]string{"app/views/a.cfm": page})

	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: viewBase}, withFw, "app/views/a.cfm"), map[string]string{
		"linkTo": "",
		"nope":   "not found in extends chain",
	})
	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: viewBase}, without, "app/views/a.cfm"), map[string]string{
		"linkTo": "",
		"nope":   "",
	})
}

// TestABreakThroughAnImpliedLinkAnywhereIsDynamic: the implied link need not
// be the file's own. A handler extending the app's base handler, which names
// no base of its own, reaches the framework through an implied link; and a
// call on another component whose implied base is missing — a ContentBox
// patch calling a task runner — is the framework's absence as much as a bare
// call is. A chain every file wrote is still reported where it breaks.
func TestABreakThroughAnImpliedLinkAnywhereIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"app/handlers/Base.cfc":  `component { function mine() {} }`,
		"app/handlers/Main.cfc":  `component extends="app.handlers.Base" { function index() { getInstance( "x" ); mine(); } }`,
		"app/handlers/Wrote.cfc": `component extends="app.Gone" { function index() { getInstance( "x" ); } }`,
		"build/Tool.cfc":         `component { function mine() {} }`,
		"app/models/Uses.cfc": `component {
	function f() {
		var t = new build.Tool();
		t.command( "x" );
		t.mine();
	}
}`,
	})

	hook := func(path string) string {
		switch p := filepath.ToSlash(path); {
		case strings.Contains(p, "/handlers/"):
			return "fw.system.EventHandler"
		case strings.Contains(p, "/build/"):
			return "fw.system.BaseTask"
		}

		return ""
	}

	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: hook}, dir, "app/handlers/Main.cfc"), map[string]string{
		"getInstance": "",
		"mine":        "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: hook}, dir, "app/handlers/Wrote.cfc"), map[string]string{
		"getInstance": MissingBaseReason("app.Gone"),
	})
	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: hook}, dir, "app/models/Uses.cfc"), map[string]string{
		"t.command": "",
		"t.mine":    "",
	})
}

// TestADynamicIfMissingReturnOnABareChainIsDynamic: a dynamicIfMissing
// resolver typing what a bare call returns, when that names no file, is
// dynamic on a chain as it is everywhere else. The bare-chain walk dropped the
// flag, and a ColdBox scheduler's task( "x" ).call() reported the resolver's
// component as one that does not exist.
func TestADynamicIfMissingReturnOnABareChainIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Page.cfc": `component { function make() {} function f() { make().go(); } }`,
	})

	for _, soft := range []bool{true, false} {
		rs := []parser.Resolver{{Match: "make()", Resolve: "missing.Thing", Prefix: "make", DynamicIfMissing: soft}}

		want := ""
		if !soft {
			want = "component 'missing.Thing' does not exist (calling 'go')"
		}

		expectReasons(t, reasonsWith(t, &Resolver{Resolvers: rs}, dir, "Page.cfc"), map[string]string{"make.go": want})
	}
}

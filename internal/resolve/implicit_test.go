package resolve

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"
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

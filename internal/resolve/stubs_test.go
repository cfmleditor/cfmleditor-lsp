package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// eventResolver is the coldbox preset's `event`, spelled out: this package
// cannot import config.
func eventResolver() parser.Resolver {
	return parser.Resolver{
		Match: `^(?:variables\.|arguments\.)?(?:event)$`, Prefix: "event|variables.event|arguments.event",
		Resolve: "coldbox.system.web.context.RequestContext", Anchored: true, DynamicIfMissing: true, NameOnly: true,
	}
}

func coldboxHandlerBase(p string) string {
	if strings.Contains(filepath.ToSlash(p), "/handlers/") {
		return "coldbox.system.EventHandler"
	}

	return ""
}

// TestFrameworkStubsAnswerWhenTheSourceIsAbsent: with ColdBox named and not
// checked out, a handler's event and its inherited methods resolve against
// the bundled API, and a method ColdBox does not have is reported against
// the real component. Without the stubs all of it was accepted unchecked.
func TestFrameworkStubsAnswerWhenTheSourceIsAbsent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"handlers/Main.cfc": `component {
	function index( event, rc, prc ) {
		event.getValue( "x" );
		event.notAMethod();
		getInstance( "UserService" );
		relocate( "main" );
		notAFrameworkMethod();
	}
}`,
	})

	with := &Resolver{Resolvers: []parser.Resolver{eventResolver()}, ImplicitExtends: coldboxHandlerBase, Stubs: frameworkapi.For([]string{"coldbox"})}
	expectReasons(t, reasonsWith(t, with, dir, "handlers/Main.cfc"), map[string]string{
		"event.getValue":      "",
		"event.notAMethod":    "method 'notAMethod' not found in coldbox.system.web.context.RequestContext",
		"getInstance":         "",
		"relocate":            "",
		"notAFrameworkMethod": "not found in extends chain",
	})

	without := &Resolver{Resolvers: []parser.Resolver{eventResolver()}, ImplicitExtends: handlerBase}
	expectReasons(t, reasonsWith(t, without, dir, "handlers/Main.cfc"), map[string]string{
		"event.notAMethod":    "",
		"notAFrameworkMethod": "",
	})
}

// TestTheFrameworksOwnSourceOutranksItsStubs: a workspace with ColdBox
// checked out resolves to that copy, whatever version it is — a stub is only
// ever the answer nothing on disk gave.
func TestTheFrameworksOwnSourceOutranksItsStubs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"coldbox/system/web/context/RequestContext.cfc": `component { function getValue( name ) {} }`,
		"handlers/Main.cfc": `component {
	function index( event ) {
		event.getValue( "x" );
		event.setView( "main/index" );
	}
}`,
	})

	r := &Resolver{Resolvers: []parser.Resolver{eventResolver()}, Stubs: frameworkapi.For([]string{"coldbox"})}
	expectReasons(t, reasonsWith(t, r, dir, "handlers/Main.cfc"), map[string]string{
		"event.getValue": "",
		"event.setView":  "method 'setView' not found in coldbox.system.web.context.RequestContext",
	})
}

// TestAChainBreakingBeyondTheStubsIsTheFrameworksAbsence: BaseTestCase
// extends TestBox's TestCase. With ColdBox named and TestBox not, the chain
// reaches the stubs and then breaks, which is TestBox not being here — not a
// finding against the spec.
func TestAChainBreakingBeyondTheStubsIsTheFrameworksAbsence(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"tests/MainSpec.cfc": `component extends="coldbox.system.testing.BaseTestCase" {
	function run() {
		setup();
		expect( 1 ).toBe( 1 );
	}
}`,
	})

	r := &Resolver{Stubs: frameworkapi.For([]string{"coldbox"})}
	expectReasons(t, reasonsWith(t, r, dir, "tests/MainSpec.cfc"), map[string]string{
		"setup":  "",
		"expect": "",
	})
}

// TestAnInjectedFrameworkObjectIsCheckedAgainstTheStubs: `inject="coldbox:
// requestService"` types the property as ColdBox's own class. With the
// stubs a call on it is checked; without ColdBox or its stubs it is dynamic,
// as a preset's resolvers are — in a subclass too, where the property the call
// is made on was declared by the base.
func TestAnInjectedFrameworkObjectIsCheckedAgainstTheStubs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Base.cfc": `component {
	property name="requestService" inject="coldbox:requestService";
	property name="log" inject="logbox:logger:{this}";
}`,
		"models/Child.cfc": `component extends="Base" {
	function f() {
		variables.requestService.getContext();
		variables.requestService.notAMethod();
		log.info( "x" );
	}
}`,
	})

	with := &Resolver{Stubs: frameworkapi.For([]string{"coldbox"})}
	expectReasons(t, reasonsWith(t, with, dir, "models/Child.cfc"), map[string]string{
		"variables.requestService.getContext": "",
		"variables.requestService.notAMethod": "method 'notAMethod' not found in coldbox.system.web.services.RequestService",
		"log.info":                            "",
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "models/Child.cfc"), map[string]string{
		"variables.requestService.notAMethod": "",
		"log.info":                            "",
	})
}

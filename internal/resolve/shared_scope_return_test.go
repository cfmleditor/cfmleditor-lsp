package resolve

import "testing"

// TestAGetterOfAStartupTypedSharedVariable: a function returning a shared-scope
// variable it does not write returns what the startup templates assign it,
// the type the variable already has as a receiver. Mura's
// getPluginManager() is `return application.pluginManager`.
func TestAGetterOfAStartupTypedSharedVariable(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc": `<cfcomponent><cffunction name="onApplicationStart"><cfinclude template="startup.cfm"></cffunction></cfcomponent>`,
		"startup.cfm":     `<cfset application.svc = createObject("component", "lib.Service")>`,
		"lib/Service.cfc": `component {function work(){}}`,
		"Other.cfc":       `component {function other(){}}`,
		"Base.cfc": `component {
function getSvc(){ return application.svc; }
function getUnassigned(){ return application.nothing; }
function getReplaced(){ application.svc = new Other(); return application.svc; }
}`,
		"Page.cfc": `component extends="Base" {function run(){
 getSvc().work();
 getSvc().nope();
 getUnassigned().work();
 getReplaced().other();
 }}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"getSvc.work":        "",
		"getSvc.nope":        "method 'nope' not found in Service",
		"getUnassigned.work": "method 'getUnassigned' has no component return type (chain to 'work')",
		"getReplaced.other":  "",
	})
}

// TestAParenthesisedReturnIsTheExpression: `return( this );` is `return this;`,
// which is how Mura's jsonSerializer ends every fluent definer, so asString()
// and its siblings had no return type. Parentheses around a value that is
// unknown leave it unknown.
func TestAParenthesisedReturnIsTheExpression(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"S.cfc": `component {
function paren(){ return( this ); }
function viaPrivate(){ return helper(); }
private any function helper(){ return ( ( this ) ); }
function unknown(){ return( mystery ); }
function work(){}
}`,
		"Page.cfc": `component {function run(){var s = new S();
 s.paren().work();
 s.paren().nope();
 s.viaPrivate().work();
 s.unknown().work();
 }}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"s.paren.work":      "",
		"s.paren.nope":      "method 'nope' not found in S",
		"s.viaPrivate.work": "",
		"s.unknown.work":    "method 'unknown' in S has no component return type (chain to 'work')",
	})
}

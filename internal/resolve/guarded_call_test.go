package resolve

import "testing"

// TestACallTheCodeChecksForIsNotMissing: LogBox calls
// variables.config.onShutdown() only inside `if ( structKeyExists(
// variables.config, "onShutdown" ) )`, a convention its config may or may not
// follow. The same call outside the guarded block, after a negated guard or
// in its else, is still checked.
func TestACallTheCodeChecksForIsNotMissing(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Config.cfc": `component { function own() {} }`,
		"Box.cfc": `component {
	function f() {
		var c = new Config();
		if ( structKeyExists( c, "onShutdown" ) ) {
			c.onShutdown( this );
		}
		if ( c.keyExists( "onStart" ) ) c.onStart();
		if ( isDefined( "c.onLoad" ) ) {
			c.onLoad();
		} else {
			c.onLoadElse();
		}
		if ( !structKeyExists( c, "onNegated" ) ) {
			c.onNegated();
		}
		if ( structKeyExists( c, "onAfter" ) ) {
			c.own();
		}
		c.onAfter();
	}
}`,
		"page.cfm": `<cfset c = new Config()>
<cfif structKeyExists( c, "onTag" )>
	<cfif true><cfset x = 1></cfif>
	<cfset c.onTag()>
</cfif>
<cfset c.onTagAfter()>
<cfif structKeyExists( c, "onTagAfter" )></cfif>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Box.cfc"), map[string]string{
		"c.onShutdown": "",
		"c.onStart":    "",
		"c.onLoad":     "",
		"c.onLoadElse": "method 'onLoadElse' not found in Config",
		"c.onNegated":  "method 'onNegated' not found in Config",
		"c.onAfter":    "method 'onAfter' not found in Config",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "page.cfm"), map[string]string{
		"c.onTag":      "",
		"c.onTagAfter": "method 'onTagAfter' not found in Config",
	})
}

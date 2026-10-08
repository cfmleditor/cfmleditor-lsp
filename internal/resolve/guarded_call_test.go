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

// TestATypeCheckSaysWhatTheObjectIs: Mura's pluginManager is handed an event
// or a MuraScope, and inside `<cfif variables.utility.checkForInstanceOf(
// arguments.event, "mura.MuraScope" )>` calls arguments.event.event(), which
// only the MuraScope declares. The call is checked against the component the
// guard names; outside the guard, or under a negated one, it is not.
func TestATypeCheckSaysWhatTheObjectIs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Event.cfc": `component { function getValue() {} }`,
		"Scope.cfc": `component { function event() {} }`,
		"Manager.cfc": `<cfcomponent>
<cffunction name="announce">
	<cfargument name="ev" type="Event">
	<cfif variables.utility.checkForInstanceOf(arguments.ev, "Scope")>
		<cfset arguments.ev.event()>
		<cfset arguments.ev.notOnScope()>
	</cfif>
</cffunction>
<cffunction name="negated">
	<cfargument name="neg" type="Event">
	<cfif not isInstanceOf(arguments.neg, "Scope")>
		<cfset arguments.neg.event()>
	</cfif>
</cffunction>
<cffunction name="after">
	<cfargument name="aft" type="Event">
	<cfif isInstanceOf(arguments.aft, "Scope")></cfif>
	<cfset arguments.aft.event()>
</cffunction>
</cfcomponent>`,
	})

	expectReasons(t, reasonsIn(t, dir, "Manager.cfc"), map[string]string{
		"arguments.ev.event":      "",
		"arguments.ev.notOnScope": "method 'notOnScope' not found in Event",
		"arguments.neg.event":     "method 'event' not found in Event",
		"arguments.aft.event":     "method 'event' not found in Event",
	})
}

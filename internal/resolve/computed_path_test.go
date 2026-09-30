package resolve

import "testing"

// TestAComponentPathComputedAtRunTimeIsDynamic: Lucee's admin builds its
// drivers with createObject( "component", drivernames[ type ] ). The path is
// whichever component the program picks, as an unmapped #...# in a literal
// path is, so calls on the result are dynamic rather than "no component ref"
// — 97 corpus entries on `driver` alone. A call written in the path is still
// recorded, and a literal path is still checked.
func TestAComponentPathComputedAtRunTimeIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Svc.cfc": `component { function go() {} }`,
		"Page.cfc": `component {
	function f( k ) {
		var byKey = createObject( "component", names[ k ] );
		byKey.anything();
		var joined = createObject( "component", "drivers." & k );
		joined.anything();
		var called = createObject( "component", pathFor( k ) ).init();
		called.anything();
		var literal = createObject( "component", "Svc" );
		literal.missing();
		// Java objects were already dynamic, chained or not.
		var field = createObject( "java", "lucee.runtime.type.QueryImpl" ).getClass().getDeclaredField( "x" );
		field.setAccessible( true );
	}
}`,
		"TagPage.cfc": `<cfcomponent>
	<cffunction name="f">
		<cfset var driver = createObject( "component", drivernames[ arguments.k ] )>
		<cfset driver.onBeforeUpdate()>
		<cfset var literal = createObject( "component", "Svc" )>
		<cfset literal.missing()>
	</cffunction>
</cfcomponent>`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"byKey.anything":      "",
		"joined.anything":     "",
		"pathFor":             "no qualifier, not in file",
		"called.anything":     "",
		"literal.missing":     "method 'missing' not found in Svc",
		"field.setAccessible": "",
	})
	expectReasons(t, reasonsIn(t, dir, "TagPage.cfc"), map[string]string{
		"driver.onBeforeUpdate": "",
		"literal.missing":       "method 'missing' not found in Svc",
	})
}

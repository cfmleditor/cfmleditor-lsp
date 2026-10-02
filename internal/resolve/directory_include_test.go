package resolve

import "testing"

// TestATemplateADirectoryListingIncludesSeesItsIncluder: Mura's configBean
// lists dbUpdates/*.cfm beside itself and includes every one, so each update
// script runs in configBean's variables scope and calls its getDbType()
// unqualified. A template outside the listed directory is not included.
func TestATemplateADirectoryListingIncludesSeesItsIncluder(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Config.cfc": `<cfcomponent>
<cffunction name="getDbType"><cfreturn "mysql"></cffunction>
<cffunction name="applyDbUpdates">
	<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#dbUpdates" name="rsUpdates" filter="*.cfm" sort="name asc">
	<cfloop query="rsUpdates">
		<cfinclude template="dbUpdates/#rsUpdates.name#">
	</cfloop>
</cffunction>
</cfcomponent>`,
		"dbUpdates/1.0.cfm": `<cfset t = getDbType()><cfset nope()>`,
		"other/2.0.cfm":     `<cfset t = getDbType()>`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "dbUpdates/1.0.cfm"), map[string]string{
		"getDbType": "",
		"nope":      "no qualifier, not in file",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "other/2.0.cfm"), map[string]string{
		"getDbType": "no qualifier, not in file",
	})
}

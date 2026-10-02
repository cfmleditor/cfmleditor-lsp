package resolve

import "testing"

// lazyCacheManager is Mura's settingsManager cache, cut down to the shape
// that matters: setSites rebuilds variables.sites from a query, copying a
// cached element or reading a new one, and getSite returns an element from
// inside a try/catch whose catch rebuilds the cache under a lock and falls
// back to the 'default' entry.
const lazyCacheManager = `<cfcomponent output="false">
<cffunction name="init" output="false">
	<cfset variables.DAO = new DAO()>
	<cfreturn this>
</cffunction>

<cffunction name="setSites" output="false">
	<cfargument name="missingOnly" default="false">
	<cflock name="setSites" type="exclusive" timeout="200">
		<cfset var rs = "" />
		<cfset var builtSites = structNew()>
		<cfquery name="rs" datasource="x">select siteid from tsettings</cfquery>
		<cfparam name="variables.sites" default="#structNew()#">
		<cfloop query="rs">
			<cfif arguments.missingOnly and structKeyExists(variables.sites, '#rs.siteid#')>
				<cfset builtSites['#rs.siteid#'] = variables.sites['#rs.siteid#'] />
			<cfelse>
				<cfset builtSites['#rs.siteid#'] = variables.DAO.read(rs.siteid) />
			</cfif>
		</cfloop>
		<cfset variables.sites = builtSites>
	</cflock>
</cffunction>

<cffunction name="getSite" output="false">
	<cfargument name="siteid" type="string" />
	<cfif not len(arguments.siteid)>
		<cfset arguments.siteid = 'default'>
	</cfif>
	<cfparam name="variables.sites" default="#structNew()#">
	<cftry>
		<cfreturn variables.sites['#arguments.siteid#'] />
		<cfcatch>
			<cfif application.appInitialized>
				<cflock name="buildSites" timeout="200">
					<cfif structKeyExists(variables.sites, '#arguments.siteid#')>
						<cfreturn variables.sites['#arguments.siteid#'] />
					<cfelse>
						<cfset setSites(missingOnly = true) />
					</cfif>
				</cflock>
				<cfif structKeyExists(variables.sites, '#arguments.siteid#')>
					<cfreturn variables.sites['#arguments.siteid#'] />
				<cfelse>
					<cfreturn variables.sites['default'] />
				</cfif>
			<cfelse>
				<cfreturn variables.sites['default'] />
			</cfif>
		</cfcatch>
	</cftry>
</cffunction>

<cffunction name="getSites" output="false">
	<cfparam name="variables.sites" default="#structNew()#">
	<cfreturn variables.sites />
</cffunction>
</cfcomponent>`

const lazyCacheDAO = `<cfcomponent output="false">
<cffunction name="read" output="false">
	<cfargument name="siteid" type="string" />
	<cfargument name="settingsBean" default="" />
	<cfset var bean = arguments.settingsBean />
	<cfif not isObject(bean)>
		<cfset bean = new SettingsBean()>
	</cfif>
	<cfreturn bean />
</cffunction>
</cfcomponent>`

func TestALazyCacheReturnsItsElementType(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"SettingsBean.cfc":    `component {function getSiteID(){}}`,
		"DAO.cfc":             lazyCacheDAO,
		"SettingsManager.cfc": lazyCacheManager,
		"Page.cfc": `component {function run(){var m = new SettingsManager();
 m.getSite( "x" ).getSiteID();
 m.getSite( "x" ).nope();
 }}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"m.getSite.getSiteID": "",
		"m.getSite.nope":      "method 'nope' not found in SettingsBean",
	})
}

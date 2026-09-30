package parser

import "testing"

// TestATagFunctionsLocalsReachItsCfscriptBlock: a <cfscript> block inside a
// <cffunction> splits the file into regions, and each region's parser began
// with only the function's arguments as locals. `conn`, declared with
// <cfset var> above the block, was then assigned inside it as if unscoped —
// a variables-scope variable, filed for the whole component. A local the
// block declares stays local in the tag region after it, too.
func TestATagFunctionsLocalsReachItsCfscriptBlock(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="download">
	<cfset var conn = "">
	<cfscript>
		conn = new Connection();
		var other = "";
	</cfscript>
	<cfset other = new Other()>
	<cfset shared = new Shared()>
</cffunction>
</cfcomponent>`
	pr := Parse(testURI, src)

	global := map[string]bool{}
	for i := range pr.ComponentRefs {
		global[pr.ComponentRefs[i].Variable] = true
	}

	for name, want := range map[string]bool{"conn": false, "other": false, "shared": true} {
		if global[name] != want {
			t.Errorf("%s at component level: %v, want %v", name, global[name], want)
		}
	}
}

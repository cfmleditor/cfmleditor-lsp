package formatter

import (
	"fmt"
	"testing"
)

func TestCFQueryDebug(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{"queryparam in WHERE", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
WHERE id = <cfqueryparam value="#arguments.id#" cfsqltype="cf_sql_integer">
AND active = 1
</cfquery>`},
		{"multiple queryparams", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
WHERE id = <cfqueryparam value="#arguments.id#" cfsqltype="cf_sql_integer">
AND name = <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">
ORDER BY name
</cfquery>`},
		{"queryparams in VALUES", `<cfquery name="q" datasource="ds">
INSERT INTO product (name, email)
VALUES (
    <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">,
    <cfqueryparam value="#arguments.email#" cfsqltype="cf_sql_varchar">
)
</cfquery>`},
		{"cfif inside cfquery", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
<cfif isDefined("arguments.name")>
WHERE name = <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">
</cfif>
ORDER BY name
</cfquery>`},
		{"cfif with cfelse", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
WHERE 1=1
<cfif len(arguments.name)>
AND name = <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">
<cfelse>
AND name IS NOT NULL
</cfif>
ORDER BY name
</cfquery>`},
		{"cfif with cfelseif", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
WHERE 1=1
<cfif len(arguments.name)>
AND name = <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">
<cfelseif len(arguments.email)>
AND email = <cfqueryparam value="#arguments.email#" cfsqltype="cf_sql_varchar">
<cfelse>
AND active = 1
</cfif>
ORDER BY name
</cfquery>`},
		{"cfloop in query", `<cfquery name="q" datasource="ds">
SELECT *
FROM product
WHERE id IN (<cfloop list="#ids#" index="i"><cfqueryparam value="#i#">,</cfloop>)
</cfquery>`},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			got := format(t, s.src)
			fmt.Printf("\n━━━ %s ━━━\nOutput:\n%s\n", s.name, got)
		})
	}
}

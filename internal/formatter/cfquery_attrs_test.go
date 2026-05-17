package formatter

import (
	"fmt"
	"testing"
)

func TestCFQueryTagAttrs(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{"queryparam attrs preserved", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE id = <cfqueryparam value="#id#" cfsqltype="cf_sql_integer">
</cfquery>`},
		{"queryparam many attrs", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE id = <cfqueryparam value="#id#" cfsqltype="cf_sql_integer" maxlength="10" null="false" list="true">
</cfquery>`},
		{"cfloop attrs", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE id IN (
<cfloop list="#ids#" index="i" delimiters=",">
<cfqueryparam value="#i#" cfsqltype="cf_sql_integer">,
</cfloop>
)
</cfquery>`},
		{"cfif with expression condition", `<cfquery name="q" datasource="ds">
SELECT * FROM product
<cfif len(arguments.name) GT 0>
WHERE name = <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">
</cfif>
</cfquery>`},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			got := format(t, s.src)
			fmt.Printf("\n━━━ %s ━━━\nOutput:\n%s\n", s.name, got)
		})
	}
}

package formatter_test

import (
	"fmt"
	"testing"
)

// TestFormatOutput prints formatted output for visual inspection.
// Run with: go test -v -run TestFormatOutput ./internal/formatter/
func TestFormatOutput(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{"self-closing tag", `<CFSET foo = "bar">`},
		{"tag with attributes", `<cfquery name="q" datasource="ds" maxrows="10" timeout="30">SELECT id, name FROM users WHERE active = 1</cfquery>`},
		{"nested block tags", `<cfoutput><cfloop array="#items#" item="i"><cfset x = i></cfloop></cfoutput>`},
		{"if/elseif/else", `<cfif x EQ 1><cfset y = "one"><cfelseif x EQ 2><cfset y = "two"><cfelse><cfset y = "other"></cfif>`},
		{"cfscript block", `<cfscript>
function greet(required string name) {
var greeting = "Hello, " & name & "!";
if (len(greeting) > 0) {
writeOutput(greeting);
} else {
writeOutput("no greeting");
}
return greeting;
}
</cfscript>`},
		{"component with function", `<cfcomponent><cffunction name="getData" access="public" returntype="query"><cfargument name="id" type="numeric" required="true"><cfquery name="q" datasource="myds">SELECT * FROM items WHERE id = <cfqueryparam value="#arguments.id#" cfsqltype="cf_sql_integer"></cfquery><cfreturn q></cffunction></cfcomponent>`},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			got := format(t, s.src)
			fmt.Printf("\n━━━ %s ━━━\nInput:  %s\nOutput:\n%s\n", s.name, s.src, got)

			// Verify idempotency: formatting the output again should not change it.
			got2 := format(t, got)
			if got != got2 {
				t.Errorf("NOT IDEMPOTENT\nPass 1:\n%s\nPass 2:\n%s", got, got2)
			}
		})
	}
}

package formatter

import (
	"fmt"
	"testing"
)

func TestCFQueryParenthesized(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{"simple column list", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE id IN (1, 2, 3)
</cfquery>`},
		{"column def list", `<cfquery name="q" datasource="ds">
INSERT INTO product (name, email, active) VALUES ('test', 'test@example.com', 1)
</cfquery>`},
		{"VALUES with cfqueryparam", `<cfquery name="q" datasource="ds">
INSERT INTO product (name, email)
VALUES (
    <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">,
    <cfqueryparam value="#arguments.email#" cfsqltype="cf_sql_varchar">
)
</cfquery>`},
		{"subquery in parens", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE id IN (SELECT product_id FROM featured_product)
</cfquery>`},
		{"function call parens", `<cfquery name="q" datasource="ds">
SELECT COUNT(*) FROM product WHERE UPPER(name) = 'TEST'
</cfquery>`},
		{"nested parens", `<cfquery name="q" datasource="ds">
SELECT * FROM product WHERE (active = 1 AND category = 'books') OR (active = 1 AND category = 'music')
</cfquery>`},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			got := format(t, s.src)
			fmt.Printf("\n━━━ %s ━━━\nOutput:\n%s\n", s.name, got)
		})
	}
}

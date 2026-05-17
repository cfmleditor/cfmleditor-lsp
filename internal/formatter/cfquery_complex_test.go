package formatter

import (
	"fmt"
	"testing"
)

func TestCFQueryComplex(t *testing.T) {
	src := `<cfquery name="q" datasource="ds">
SELECT p.product_id, p.title, p.description, p.category, p.sku, p.status, p.created_at, p.updated_at, '' as fiscal_yr, '' as fiscal_qtr, p.weight, p.is_featured, p.vendor_id
<cfif _objConfig.isRegionEnabled(regionCode=variables.region) IS true>
,(SELECT COUNT (*) FROM order_audit a WHERE a.region_code = p.region_code AND a.product_id = p.product_id AND (a.status_flag != <cfqueryparam cfsqltype="cf_sql_char" value="A">
			AND a.status_flag != <cfqueryparam cfsqltype="cf_sql_char" value="I">
			AND (	SELECT COUNT (*)
				FROM order_audit aa
				WHERE aa.region_code = p.region_code
				AND aa.product_id = p.product_id
				AND aa.tran_type = <cfqueryparam cfsqltype="cf_sql_integer" value="1">
				AND aa.status_flag = <cfqueryparam cfsqltype="cf_sql_char" value="U">) = 0)) AS disallow_transfer
<cfelse>
,0 AS disallow_transfer
</cfif>
,is_digital, sort_order
FROM product p
WHERE p.region_code = <cfqueryparam cfsqltype="cf_sql_char" value="#variables.region#" />
AND p.product_id NOT IN (
	SELECT o.product_id
	FROM order_item o
	WHERE o.region_code = <cfqueryparam cfsqltype="cf_sql_char" value="#variables.region#" />
	AND o.order_status != <cfqueryparam cfsqltype="cf_sql_char" value="T" />
)
AND p.deleted_at IS NOT NULL
AND p.next_action = <cfqueryparam cfsqltype="cf_sql_char" value="E" />
ORDER BY product_id
</cfquery>`

	got := format(t, src)
	fmt.Printf("\nOutput:\n%s\n", got)
}

package formatter

import (
	"fmt"
	"testing"
)

func TestCFQueryUpdate(t *testing.T) {
	src := `<cfquery name="q" datasource="ds">
UPDATE enrollment
SET cancel_flag = <cfqueryparam null="no" cfsqltype="cf_sql_char" value="N" />, cancel_date = <cfqueryparam cfsqltype="cf_sql_date" null="yes" value="" />
</cfquery>`

	got := format(t, src)
	fmt.Printf("\nOutput:\n%s\n", got)
}

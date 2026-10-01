package resolve

import "testing"

func TestDocumentedArgumentIsCheckedAgainstItsComponent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/User.cfc": `component { function save() {} }`,
		"Spec.cfc": `component {
/** @employee.doc_generic models.User */
function first(employee) {
    employee.save();
    arguments.employee.missing();
}
function second(employee) { employee.undocumented(); }
}`,
		"TagSpec.cfc": `<cfcomponent>
<!--- @employee.doc_generic models.User --->
<cffunction name="first"><cfargument name="employee">
<cfset employee.save()><cfset arguments.employee.missing()>
</cffunction>
<cffunction name="second"><cfargument name="employee"><cfset employee.undocumented()></cffunction>
</cfcomponent>`,
	})

	for _, file := range []string{"Spec.cfc", "TagSpec.cfc"} {
		expectReasons(t, reasonsIn(t, dir, file), map[string]string{
			"employee.save":              "",
			"arguments.employee.missing": "method 'missing' not found in models.User",
			"employee.undocumented":      "variable 'employee' has no component ref",
		})
	}
}

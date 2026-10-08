package resolve

import "testing"

// TestATemplateIncludedByAComputedPathReadsItsIncluder: Masa's form builder
// names its templates' directory in a string and includes
// "#templatePath#" from functions that set mmRBF first. A template in a
// directory no string names is not reached.
func TestATemplateIncludedByAComputedPathReadsItsIncluder(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Rb.cfc": `component { function getKeyValue( k ){ return k; } }`,
		"core/Manager.cfc": `component {
	variables.templatePath = "/app/core/utilities/formbuilder/templates";
	function render( name ){
		var mmRBF = new Rb();
		var templatePath = variables.templatePath & "/" & arguments.name;
		include "#templatePath#";
	}
}`,
		"core/utilities/formbuilder/templates/field.cfm": `<cfoutput>#mmRBF.getKeyValue( "x" )##mmRBF.nope()#</cfoutput>`,
		"core/utilities/other/templates/field.cfm":       `<cfoutput>#mmRBF.getKeyValue( "x" )#</cfoutput>`,
	})

	r := &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}

	expectReasons(t, reasonsWith(t, r, dir, "core/utilities/formbuilder/templates/field.cfm"), map[string]string{
		"mmRBF.getKeyValue": "",
		"mmRBF.nope":        "method 'nope' not found in Rb",
	})
	expectReasons(t, reasonsWith(t, r, dir, "core/utilities/other/templates/field.cfm"), map[string]string{
		"mmRBF.getKeyValue": "variable 'mmRBF' has no component ref",
	})
}

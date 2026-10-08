package resolve

import "testing"

// TestAColdBoxErrorTemplateReadsProcessExceptionsLocals: ColdBox includes the
// template a config names as customErrorTemplate from inside Bootstrap's
// processException(), by a computed path, where oException is an
// ExceptionBean. A template no config names is still untyped.
func TestAColdBoxErrorTemplateReadsProcessExceptionsLocals(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"box.json":                             `{"slug":"coldbox"}`,
		"system/web/context/ExceptionBean.cfc": `component { function getMessage(){ return ""; } }`,
		"system/Bootstrap.cfc": `component {
	private string function processException( required controller, required exception ){
		var oException = new coldbox.system.web.context.ExceptionBean( arguments.exception );
		var bugReportRelativePath = arguments.controller.getSetting( "CustomErrorTemplate" );
		savecontent variable="local.exceptionReport" {
			include "#bugReportRelativePath#";
		}
		return local.exceptionReport;
	}
}`,
		"system/exceptions/Whoops.cfm":  `<cfoutput>#oException.getMessage()##oException.nope()#</cfoutput>`,
		"system/exceptions/Unnamed.cfm": `<cfoutput>#oException.getMessage()#</cfoutput>`,
		"test-harness/config/ColdBox.cfc": `component {
	function configure(){
		coldbox = { customErrorTemplate : "/coldbox/system/exceptions/Whoops.cfm" };
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "system/exceptions/Whoops.cfm"), map[string]string{
		"oException.getMessage": "",
		"oException.nope":       "method 'nope' not found in ExceptionBean",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "system/exceptions/Unnamed.cfm"), map[string]string{
		"oException.getMessage": "variable 'oException' has no component ref",
	})
}

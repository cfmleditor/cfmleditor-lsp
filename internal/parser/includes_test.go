package parser

import (
	"slices"
	"testing"
)

func TestExtractIncludes(t *testing.T) {
	content := `<cfcomponent>
	<cfinclude template="api-shared.cfm" />
	<cfinclude template = '/tassweb/packages/tass/core/error.cfm'>
	<cfinclude template="#dynamicPath#.cfm">
	<cfinclude template="API-SHARED.cfm">
	<cfmodule template="module.cfm">
	<a href="page.cfm">x</a>
	<!--- <cfinclude template="commented.cfm"> <!--- nested ---> still comment --->
	<cfscript>
		include "script-form.cfm";
		include template="script-named.cfm";
		cfinclude(template="script-call.cfm");
		arr.include("member.cfm");
		include "not-cfml.js";
	</cfscript>
</cfcomponent>`

	want := []string{
		"api-shared.cfm",
		"/tassweb/packages/tass/core/error.cfm",
		"script-form.cfm",
		"script-named.cfm",
		"script-call.cfm",
	}

	if got := ExtractIncludes(content); !slices.Equal(got, want) {
		t.Errorf("ExtractIncludes:\n got  %q\n want %q", got, want)
	}
}

func TestExtractIncludesAfterAComment(t *testing.T) {
	content := "<!--- a\nb ---><cfinclude template=\"after.cfm\">"
	if got := ExtractIncludes(content); !slices.Equal(got, []string{"after.cfm"}) {
		t.Errorf("got %q", got)
	}
}

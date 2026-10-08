package resolve

import (
	"strings"
	"testing"
)

const wheelsWrapperGlobal = `component {
	public void function $include(required string template) {
		$tryIncludeTemplate($resolveGlobalIncludeTemplate(arguments.template));
	}
	public void function $includeAndOutput(required string template) {
		$tryIncludeTemplate($resolveGlobalIncludeTemplate(arguments.template));
	}
	public string function $resolveGlobalIncludeTemplate(required string template) {
		var normalized = Replace(arguments.template, "\", "/", "all");
		if (!Len(normalized)) {
			return normalized;
		}
		if (Left(normalized, 1) == "/") {
			return LCase(normalized);
		}
		return normalized;
	}
	public void function $tryIncludeTemplate(required string template) {
		var resolved = arguments.template;
		var state = {done = false};
		try {
			include "#resolved#";
			state.done = true;
		} catch (any e) {
			rethrow;
		}
	}
	public string function urlFor() { return ""; }
}`

// TestATemplateAWheelsWrapperIncludesIsItsIncluders: public/Application.cfc
// runs `application.wo.$includeAndOutput( template = "/wheels/events/debug.cfm" )`,
// so debug.cfm runs in Global and calls its methods bare. A wrapper whose body
// is not the pinned one, or a computed template, proves nothing.
func TestATemplateAWheelsWrapperIncludesIsItsIncluders(t *testing.T) {
	files := map[string]string{
		"Application.cfc":         `component { this.mappings["/wheels"] = getDirectoryFromPath(getCurrentTemplatePath()) & "wheels/"; }`,
		"wheels/Global.cfc":       wheelsWrapperGlobal,
		"wheels/events/debug.cfm": `<cfoutput>#urlFor()# #missing()#</cfoutput>`,
		"wheels/events/other.cfm": `<cfoutput>#urlFor()#</cfoutput>`,
		"public/Page.cfm":         `<cfscript>application.wo.$includeAndOutput(template = "/wheels/events/debug.cfm"); application.wo.$includeAndOutput(template = "/wheels/events/#x#.cfm");</cfscript>`,
	}

	dir := t.TempDir()
	writeFiles(t, dir, files)
	expectReasons(t, reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "wheels/events/debug.cfm"), map[string]string{
		"urlFor":  "",
		"missing": "no qualifier, not in file",
	})

	if got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "wheels/events/other.cfm")["urlFor"]; got == "" {
		t.Errorf("other.cfm: resolved through a computed template")
	}

	files["wheels/Global.cfc"] = strings.Replace(wheelsWrapperGlobal, `include "#resolved#";`, ``, 1)

	dir = t.TempDir()
	writeFiles(t, dir, files)

	if got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "wheels/events/debug.cfm")["urlFor"]; got == "" {
		t.Errorf("debug.cfm: resolved through a wrapper that includes nothing")
	}
}

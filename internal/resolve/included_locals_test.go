package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
)

// templateReasons resolves the calls in a .cfm template, with every CFML file
// under dir indexed and its includes recorded, as the unresolved scan does.
func templateReasons(t *testing.T, dir, page string) map[string]string {
	t.Helper()

	return templateReasonsWith(t, &Resolver{}, dir, page)
}

// templateReasonsWith is templateReasons with r's own settings kept.
func templateReasonsWith(t *testing.T, r *Resolver, dir, page string) map[string]string {
	t.Helper()

	r.FS, r.WorkspaceFolders, r.Index = vfs.OS{}, []string{dir}, index.New()

	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		if err != nil || !cfpath.IsCFMLFile(p) {
			return err
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		r.Index.IndexFileWithOptions(cfpath.ToURI(p), string(data), &parser.ParseOptions{})
		r.Index.SetIncludes(cfpath.ToURI(p), parser.ExtractIncludes(string(data)))

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, filepath.FromSlash(page))

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true, FuncLookup: r.FuncLookup(filepath.Dir(file))})
	got := map[string]string{}

	calls := pr.AllCalls()
	for i := range calls {
		c := &calls[i]
		got[strings.TrimPrefix(c.Variable+"."+c.FuncName, ".")] = r.CanResolveCall(c, pr, filepath.Dir(file))
	}

	return got
}

// TestATemplateReadsWhatItsIncluderHoldsAtTheInclude: Mura's
// configBean.applyDbUpdates declares a local and includes every
// dbUpdates/*.cfm, each of which calls methods on that local. A template
// included inside a function reads the function's locals and arguments.
func TestATemplateReadsWhatItsIncluderHoldsAtTheInclude(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Util.cfc":  `component { function setTable(){ return this; } }`,
		"Other.cfc": `component { function otherOnly(){} }`,
		"Config.cfc": `<cfcomponent>
<cffunction name="apply">
	<cfset var util = new Util()>
	<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#updates" name="rsUpdates" filter="*.cfm">
	<cfloop query="rsUpdates">
		<cfinclude template="updates/#rsUpdates.name#">
	</cfloop>
</cffunction>
</cfcomponent>`,
		"updates/1.cfm": `<cfscript>util.setTable(); util.missing();</cfscript>`,
		"Report.cfc": `component {
	function run( required Util results ) { include "assets/report.cfm"; }
}`,
		"Report2.cfc": `component {
	function run( required Other results ) { include "assets/report.cfm"; }
}`,
		"assets/report.cfm": `<cfoutput>#results.setTable()#</cfoutput>`,
		"Owner.cfc": `component {
	function run() { var own = new Util(); include "own.cfm"; }
}`,
		"own.cfm": `<cfscript>own = makeOne(); own.setTable();</cfscript>`,
		"Loose.cfc": `component {
	function run( loose ) { include "loose.cfm"; }
}`,
		"Loose2.cfc": `component {
	function run() { var loose = new Util(); include "loose.cfm"; }
}`,
		"loose.cfm": `<cfscript>loose.setTable();</cfscript>`,
	})

	expectReasons(t, templateReasons(t, dir, "updates/1.cfm"), map[string]string{
		"util.setTable": "",
		"util.missing":  "method 'missing' not found in Util",
	})

	// Two includers holding different components: either may have run.
	expectReasons(t, templateReasons(t, dir, "assets/report.cfm"), map[string]string{
		"results.setTable": "",
	})

	// A template that assigns the name reads its own value, not the includer's.
	if got := templateReasons(t, dir, "own.cfm")["own.setTable"]; got == "" {
		t.Errorf("own.setTable: resolved through the includer although the template assigns own")
	}

	// One includer that cannot type the name leaves it untyped, whatever the
	// others hold.
	expectReasons(t, templateReasons(t, dir, "loose.cfm"), map[string]string{
		"loose.setTable": "variable 'loose' has no component ref",
	})
}

// TestAPageADispatcherIncludesByNameSeesTheDispatchersHelpers: Lucee's admin
// web.cfm includes its helpers and then `#current.action#.cfm`, so every page
// beside it runs inside it and calls those helpers bare. A page in another
// directory, or one reached through a computed directory, is not included.
func TestAPageADispatcherIncludesByNameSeesTheDispatchersHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"admin/web.cfm":           `<cfinclude template="web_functions.cfm"><cfif not findOneOf("\/",url.action)><cfinclude template="#url.action#.cfm"></cfif>`,
		"admin/web_functions.cfm": `<cfscript>function printError(e){}</cfscript>`,
		"admin/overview.cfm":      `<cfscript>printError(1); notDeclared();</cfscript>`,
		"admin/sub/page.cfm":      `<cfscript>printError(1);</cfscript>`,
		"other/web.cfm":           `<cfinclude template="../other/web_functions.cfm"><cfinclude template="#d#/page.cfm">`,
		"other/web_functions.cfm": `<cfscript>function otherHelper(){}</cfscript>`,
		"other/x/page.cfm":        `<cfscript>otherHelper();</cfscript>`,
	})

	expectReasons(t, templateReasons(t, dir, "admin/overview.cfm"), map[string]string{
		"printError":  "",
		"notDeclared": "no qualifier, not in file",
	})
	expectReasons(t, templateReasons(t, dir, "admin/sub/page.cfm"), map[string]string{
		"printError": "no qualifier, not in file",
	})
	expectReasons(t, templateReasons(t, dir, "other/x/page.cfm"), map[string]string{
		"otherHelper": "no qualifier, not in file",
	})
}

// TestAReceiversGuessedTypeIsNotAReturnType: the index parses a file without
// looking methods up, so `variables.$ = variables.event.getValue("muraScope")`
// gives $ the event's own type there. Mura's contentRenderer returns $ from
// getMuraScope(), and every call chained on it was checked against the event
// — "createHREF not found in event". The guess is no statement of what
// getValue returns.
func TestAReceiversGuessedTypeIsNotAReturnType(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Event.cfc": `component { function getValue( key ){ return variables.data[ arguments.key ]; } function eventOnly(){} }`,
		"Renderer.cfc": `<cfcomponent>
<cffunction name="init">
	<cfset variables.event = new Event()>
	<cfset variables.$ = variables.event.getValue("muraScope")>
	<cfreturn this>
</cffunction>
<cffunction name="getMuraScope"><cfreturn variables.$></cffunction>
</cfcomponent>`,
		"Page.cfc": `component { function f(){ var r = new Renderer(); r.getMuraScope().createHREF(); } }`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"r.getMuraScope.createHREF": "method 'getMuraScope' in Renderer has no component return type (chain to 'createHREF')",
	})
}

package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
)

func TestStartupBeanRegistrations(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()

		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		return path
	}
	write("Factory.cfc", `component {
 function getBean(string beanName) {}
 function addAlias(string aliasName, string beanName) {}
 function declareBean(string beanName, string dottedPath) {}
 }`)
	write("Other.cfc", `component { function getBean(string id) {} function addAlias(string a, string b) {} }`)
	target := write("beans/ContentBean.cfc", `component { function load() {} }`)
	alternate := write("beans/OtherBean.cfc", `component { function save() {} }`)
	write("Content.cfc", `component { function unrelated() {} }`)
	startup := write("startup.cfm", `<cfscript>
 f = new Factory();
 f.addAlias("Content", "ContentBean");
 f.addAlias(beanName="Content", aliasName="Feed");
 f.addAlias("$", "Feed");
 f.declareBean(dottedPath: "beans.OtherBean", beanName: "declared");
 f.addAlias("fromDeclaration", "declared");
 f.addAlias("ambiguous", "ContentBean");
 if (something) { f.addAlias("ambiguous", "OtherBean"); }
 f.addAlias("cycleA", "cycleB"); f.addAlias("cycleB", "cycleA");
 f.addAlias("computed", "Content" & suffix);
 f.addAlias("interpolated", "#name#");
 f.addAlias("missing", "Absent");
 // f.addAlias("comment", "ContentBean");
 text = 'f.addAlias("quoted", "ContentBean")';
 other = new Other(); other.addAlias("unrelated", "ContentBean");
 g = new Factory(); g.addAlias("crossFactory", "onlyF");
 f.addAlias("onlyF", "ContentBean");
 include "included.cfm";
 </cfscript>
 <cfset f = new Factory()>
 <cfset f.addAlias("tag", "ContentBean")>
 <p>f.addAlias("markup", "ContentBean")</p>
 <script>f.addAlias("javascript", "ContentBean");</script>`)
	write("included.cfm", `<cfset f = new Factory()><cfset f.addAlias("included", "ContentBean")><cfinclude template="startup.cfm">`)

	const genericMatch = `(?i)getBean\(['"]([\w.]+)['"]\)$`

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}, StartupFiles: []string{startup}, Resolvers: []parser.Resolver{{Match: genericMatch, Resolve: "$1", Prefix: "getBean"}}}

	rules := r.BeanResolvers(map[string]string{"": filepath.Join(dir, "beans")})
	for id, want := range map[string]string{"content": target, "feed": target, "$": target, "declared": alternate, "fromDeclaration": alternate, "tag": target, "included": target} {
		if got := parser.ResolveFromCall(`getBean("`+id+`")`, rules); got != want {
			t.Errorf("%s: got %q, want %q", id, got, want)
		}
	}

	for _, id := range []string{"ambiguous", "cycleA", "computed", "interpolated", "missing", "comment", "quoted", "unrelated", "crossFactory", "markup", "javascript"} {
		if got := parser.ResolveFromCall(`getBean("`+id+`")`, rules); filepath.IsAbs(got) {
			t.Errorf("unsafe registration %s resolved to %s", id, got)
		}
	}

	if got := r.ComponentPath("Content", dir); got != filepath.Join(dir, "Content.cfc") {
		t.Errorf("component basename changed: %s", got)
	}

	r.Resolvers = []parser.Resolver{{Match: `getBean("content")`, Resolve: "explicit", Prefix: "getBean"}, {Match: genericMatch, Resolve: "$1", Prefix: "getBean"}}
	if got := parser.ResolveFromCall(`getBean("content")`, r.BeanResolvers(map[string]string{"": filepath.Join(dir, "beans")})); got != "explicit" {
		t.Errorf("explicit rule overridden: %s", got)
	}

	r.StartupFiles = nil
	if got := len(r.BeanResolvers(nil)); got != len(r.Resolvers) {
		t.Errorf("unconfigured discovery: %d", got)
	}
}

// TestARegistrationThatDependsOnTheEngineIsTheirCommonBase: Mura aliases
// contentGateway to contentGatewayAdobe on Adobe ColdFusion and to
// contentGatewayLucee, which extends it, everywhere else. Either way the bean
// is a contentGatewayAdobe, so that is its type; targets neither of which
// extends the other still have none.
func TestARegistrationThatDependsOnTheEngineIsTheirCommonBase(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()

		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		return path
	}
	write("Factory.cfc", `component {
 function getBean(string beanName) {}
 function addAlias(string aliasName, string beanName) {}
 }`)
	base := write("beans/GatewayAdobe.cfc", `component { function getTop() {} }`)
	write("beans/GatewayLucee.cfc", `component extends="GatewayAdobe" { function luceeOnly() {} }`)
	write("beans/Unrelated.cfc", `component { function other() {} }`)
	startup := write("startup.cfm", `<cfscript>
 f = new Factory();
 if ( server.coldfusion.productName eq "ColdFusion Server" ) {
 	f.addAlias("gateway", "GatewayAdobe");
 } else {
 	f.addAlias("gateway", "GatewayLucee");
 }
 if ( x ) { f.addAlias("either", "GatewayLucee"); } else { f.addAlias("either", "Unrelated"); }
 </cfscript>`)

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}, StartupFiles: []string{startup}, Resolvers: []parser.Resolver{{Match: `(?i)getBean\(['"]([\w.]+)['"]\)$`, Resolve: "$1", Prefix: "getBean"}}}

	rules := r.BeanResolvers(map[string]string{"": filepath.Join(dir, "beans")})
	if got := parser.ResolveFromCall(`getBean("gateway")`, rules); got != base {
		t.Errorf("gateway: got %q, want the common base %q", got, base)
	}

	if got := parser.ResolveFromCall(`getBean("either")`, rules); filepath.IsAbs(got) {
		t.Errorf("either: unrelated targets resolved to %q", got)
	}
}

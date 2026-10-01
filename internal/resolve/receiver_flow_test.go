package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestReceiverFlowKeepsRealMethodsAndMissingDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Builder.cfc": `component {function init(){return this;} Product function build(){} function onlyBuilder(){}}`,
		"Product.cfc": `component {function work(){}}`,
		"Content.cfc": `component {function save(){return this;} function work(){}}`,
		"Manager.cfc": `component {Content function getContent(){}}`,
	})
	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
	lookup := r.FuncLookup(dir)
	resolvers := []parser.Resolver{{Match: `(?i)(?:^|\.)getBean\(\s*["']([A-Za-z_][\w.]*)["']\s*\)$`, Resolve: "$1", Prefix: "getBean"}}

	for _, tc := range []struct {
		name, source string
		reasons      map[string]bool
	}{
		{"tag factory", `<cfset item=application.factory.getBean('Builder').build()>
<cfset item.work()><cfset item.onlyBuilder()>
<cfset unknown=application.factory.getBean('Builder').missing()><cfset unknown.work()>`, map[string]bool{"item.work": true, "item.onlyBuilder": false, "unknown.work": false}},
		{"record flow", `component {
function run(rc) {
rc.manager=new Manager();
arguments.rc.contentBean=rc.manager.getContent();
rc.contentBean=arguments.rc.contentBean.save();
rc.contentBean.work();rc.contentBean.missing();
}
}`, map[string]bool{"rc.contentBean.work": true, "rc.contentBean.missing": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr := parser.ParseWithOptions(cfpath.ToURI(filepath.Join(dir, "Consumer.cfc")), tc.source, &parser.ParseOptions{ExtractCalls: true, Resolvers: resolvers, FuncLookup: lookup})
			seen := map[string]bool{}

			for _, call := range pr.AllCalls() {
				key := call.Variable + "." + call.FuncName
				if want, ok := tc.reasons[key]; ok {
					seen[key] = true
					if reason := r.CanResolveCall(&call, pr, dir); (reason == "") != want {
						t.Errorf("%s: %s", key, reason)
					}
				}
			}

			for key := range tc.reasons {
				if !seen[key] {
					t.Errorf("missing call %s", key)
				}
			}
		})
	}
}

package parser

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// writesAfterEdit reads before's writes through cache, then after's, and
// returns what the cache answered for after with what a fresh read gives.
func writesAfterEdit(before, after string) (cached, fresh []collectionWrite) {
	cache := &expressionWriteCache{}

	first := Parse(uri.URI("file:///x/Widget.cfc"), before)

	cache.begin()
	first.collectWrites(cache)

	second := Parse(uri.URI("file:///x/Widget.cfc"), after)

	cache.begin()
	cached = second.collectWrites(cache)
	fresh = second.collectWrites(nil)

	return cached, fresh
}

func TestCachedCfsetWritesAreWhatAFreshReadFinds(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="init">
	<cfset variables.svc = createObject("component", "a.Svc")>
	<cfset var local.items = []>
	<cfset arrayAppend(variables.list, x)>
	<cfset variables.cache.users = variables.svc.load()>
	<cfparam name="variables.flag" default="#false#">
</cffunction>
<cffunction name="other">
	<cfset this.thing = variables.svc>
	<cfset variables.total += 1>
</cffunction>
</cfcomponent>`

	edits := map[string]string{
		// Every offset after the insertion moves; the cached writes must be
		// placed where their <cfset> now is, in the function it is now in.
		"lines inserted above": strings.Replace(src, "<cfcomponent>\n", "<cfcomponent>\n<!--- a --->\n\n\n", 1),
		"one cfset changed":    strings.Replace(src, "variables.svc.load()", "variables.svc.find()", 1),
		"a cfset moved":        strings.Replace(strings.Replace(src, "\t<cfset this.thing = variables.svc>\n", "", 1), "<cffunction name=\"init\">\n", "<cffunction name=\"init\">\n\t<cfset this.thing = variables.svc>\n", 1),
		"function renamed":     strings.Replace(src, `name="other"`, `name="another"`, 1),
	}

	for name, after := range edits {
		cached, fresh := writesAfterEdit(src, after)
		if len(fresh) == 0 {
			t.Fatalf("%s: the fixture has no writes to compare", name)
		}

		if !reflect.DeepEqual(cached, fresh) {
			t.Errorf("%s:\ncached %+v\nfresh  %+v", name, cached, fresh)
		}
	}

	// And over the repo's fixtures, each edited by a line inserted at the top.
	n := 0

	err := filepath.WalkDir("../../testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".cfc") && !strings.HasSuffix(path, ".cfm") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		cached, fresh := writesAfterEdit(string(data), "<!--- edit --->\n"+string(data))
		if !reflect.DeepEqual(cached, fresh) {
			t.Errorf("%s: cached %+v, fresh %+v", path, cached, fresh)
		}

		n++

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if n == 0 {
		t.Fatal("found no fixtures under ../../testdata")
	}
}

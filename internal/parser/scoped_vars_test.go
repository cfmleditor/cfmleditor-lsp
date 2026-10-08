package parser

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/conv"
	"go.lsp.dev/uri"
)

// computeScopedVarsPerScope is the scan as it was before one pass filled every
// scope: run once per scope, keeping only that scope's names. The single scan
// must give each scope exactly what this gave it.
func (pr *ParseResult) computeScopedVarsPerScope(scope Scope) []string {
	seen := make(map[string]bool)

	var names []string

	// Properties default to variables scope
	if scope == ScopeVariables {
		for _, prop := range pr.Properties {
			if !seen[prop.name] {
				seen[prop.name] = true

				names = append(names, prop.name)
			}
		}
	}

	for _, r := range pr.Regions {
		if r.Kind == RegionSkip {
			continue
		}

		var regionVars []VarDef

		if r.Kind == RegionScript {
			sp := newGlobalScriptParser(r.Text, r.StartLine, pr.Scopes)
			sp.parse()
			regionVars = sp.vars
		} else {
			tp := newTagParser(r.Text, "")
			tp.parse()

			for i := range tp.vars {
				tp.vars[i].Line += conv.Uint32(r.StartLine)
			}

			regionVars = tp.vars
		}

		for _, v := range regionVars {
			if v.Scope != scope {
				continue
			}

			fn := findFuncScope(int(v.Line), pr.Scopes)
			if fn.Start != -1 {
				continue // inside a function
			}

			if !seen[v.Name] {
				seen[v.Name] = true

				names = append(names, v.Name)
			}
		}
	}

	// Also include vars from init() body
	initScope := pr.initFuncScope()
	if initScope.Start == -1 {
		return names
	}

	start, end := pr.lineOffsets(initScope.Start, initScope.End)
	if start < 0 {
		return names
	}

	body := pr.Content[start:end]
	regionKind := RegionScript

	for _, r := range pr.Regions {
		if r.StartLine <= initScope.Start {
			regionKind = r.Kind
		}
	}

	var bodyVars []VarDef

	if regionKind == RegionScript {
		sp := newScriptParser(body, "", initScope.Start, nil)
		sp.parse()
		bodyVars = sp.vars
	} else {
		tp := newTagParser(body, "")
		tp.parse()
		bodyVars = tp.vars
	}

	for _, v := range bodyVars {
		if v.Scope == scope && !seen[v.Name] {
			seen[v.Name] = true

			names = append(names, v.Name)
		}
	}

	return names
}

// scopedVarsMustMatch fails when the single scan's answer for any of the three
// accessors differs from the per-scope scan's.
func scopedVarsMustMatch(t *testing.T, name, src string) {
	t.Helper()

	pr := Parse(uri.URI("file:///"+name), src)

	variables := pr.computeScopedVarsPerScope(ScopeVariables)
	this := pr.computeScopedVarsPerScope(ScopeThis)

	if got := pr.VariablesVars(); !slices.Equal(got, variables) {
		t.Errorf("%s: VariablesVars %v, per-scope scan %v", name, got, variables)
	}

	if got := pr.ThisVars(); !slices.Equal(got, this) {
		t.Errorf("%s: ThisVars %v, per-scope scan %v", name, got, this)
	}

	if got, want := pr.GlobalVars(), append(slices.Clip(variables), this...); !slices.Equal(got, want) {
		t.Errorf("%s: GlobalVars %v, per-scope scans %v", name, got, want)
	}
}

func TestOneScanGivesEachScopeWhatItsOwnScanDid(t *testing.T) {
	scopedVarsMustMatch(t, "Mixed.cfc", `component accessors="true" {
	property name="dao";
	variables.cache = {};
	this.version = 1;
	variables.cache = [];
	plain = 2;
	function init() { variables.ready = true; this.name = "x"; var local1 = 1; return this; }
	function other() { variables.inside = 1; this.inside = 2; }
}`)
	scopedVarsMustMatch(t, "Tags.cfc", `<cfcomponent>
	<cfproperty name="svc">
	<cfset variables.a = 1>
	<cfset this.b = 2>
	<cffunction name="init"><cfset variables.c = 3><cfset this.d = 4><cfreturn this></cffunction>
	<cffunction name="f"><cfset variables.e = 5></cffunction>
</cfcomponent>`)

	n := 0

	err := filepath.WalkDir("../../testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".cfc") && !strings.HasSuffix(path, ".cfm") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err == nil {
			scopedVarsMustMatch(t, path, string(data))

			n++
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if n == 0 {
		t.Fatal("found no fixtures under ../../testdata")
	}
}

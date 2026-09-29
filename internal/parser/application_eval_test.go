package parser

import (
	"path/filepath"
	"testing"
)

// TestApplicationMappingsAreEvaluated: most Application.cfc files build a
// mapping from their own directory and a variable set above it, and the
// literal-only match read 20 of the corpus's 110. Each shape is one the
// corpus writes; a value built from something the source does not state is
// declined whole rather than half-read.
func TestApplicationMappingsAreEvaluated(t *testing.T) {
	app := filepath.FromSlash("/w/cli/lucli/tests")
	src := `component {
	local.projectRoot = expandPath("../../../");
	this.mappings["/cli"] = local.projectRoot & "cli/";
	this.mappings["/modules/wheels"] = local.projectRoot & "cli/lucli/";
	variables.here = getDirectoryFromPath( getCurrentTemplatePath() );
	this.mappings[ '/tests' ] = variables.here;
	this.mappings[ "/fw" ] = variables.here & "../framework";
	this.appDir = expandPath( "../app/" );
	this.mappings["/app"] = this.appDir;
	this.mappings["/again"] = this.mappings[ "/app" ];
	this.mappings["/literal"] = expandPath( "./lit" );
	this.mappings["/dynamic"] = url.root & "x";
	this.mappings["/unknown"] = notSetAnywhere & "x";
}`

	want := map[string]string{
		"cli":            "/w/cli",
		"modules/wheels": "/w/cli/lucli",
		"tests":          "/w/cli/lucli/tests",
		"fw":             "/w/cli/lucli/framework",
		"app":            "/w/cli/lucli/app",
		"again":          "/w/cli/lucli/app",
		"literal":        "/w/cli/lucli/tests/lit",
	}

	got := ParseApplicationMappings(src, app)

	for k, v := range want {
		if got[k] != filepath.FromSlash(v) {
			t.Errorf("%s = %q, want %q", k, got[k], filepath.FromSlash(v))
		}
	}

	for _, k := range []string{"dynamic", "unknown"} {
		if v, ok := got[k]; ok {
			t.Errorf("%s = %q, want it declined", k, v)
		}
	}
}

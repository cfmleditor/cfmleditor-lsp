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

// TestAMappingBuiltByARegexReplaceIsEvaluated: ColdBox's test Application.cfc
// strips its own directory name to find the repository root,
// `REReplaceNoCase( this.mappings[ "/tests" ], "tests(\\|/)", "" )`, and
// maps the test harness under it. A pattern Go cannot read, a replacement
// with a backreference or an unknown scope is declined.
func TestAMappingBuiltByARegexReplaceIsEvaluated(t *testing.T) {
	app := filepath.FromSlash("/w/coldbox/tests")
	src := `component {
	this.mappings[ "/tests" ] = getDirectoryFromPath( getCurrentTemplatePath() );
	rootPath = REReplaceNoCase( this.mappings[ "/tests" ], "TESTS(\\|/)", "" );
	this.mappings[ "/cbtestharness" ] = rootPath & "test-harness";
	this.mappings[ "/all" ] = reReplace( "a/x/x/", "x/", "y/", "all" );
	this.mappings[ "/one" ] = reReplace( "a/x/x/", "x/", "y/" );
	this.mappings[ "/cased" ] = reReplace( "a/X/", "x/", "" );
	this.mappings[ "/backref" ] = reReplace( "a/x/", "(x)/", "\1" );
	this.mappings[ "/badpattern" ] = reReplace( "a/x/", "(?<=a)x", "" );
	this.mappings[ "/badscope" ] = reReplace( "a/x/", "x", "", "some" );
}`

	got := ParseApplicationMappings(src, app)

	want := map[string]string{
		"tests":         "/w/coldbox/tests",
		"cbtestharness": "/w/coldbox/test-harness",
		"all":           "/w/coldbox/tests/a/y/y",
		"one":           "/w/coldbox/tests/a/y/x",
		"cased":         "/w/coldbox/tests/a/X",
	}

	for key, path := range want {
		if got[key] != filepath.FromSlash(path) {
			t.Errorf("%s: got %q, want %q", key, got[key], path)
		}
	}

	for _, key := range []string{"backref", "badpattern", "badscope"} {
		if v, ok := got[key]; ok {
			t.Errorf("%s: got %q, want it declined", key, v)
		}
	}
}

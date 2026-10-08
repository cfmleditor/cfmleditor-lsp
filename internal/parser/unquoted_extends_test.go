package parser

import "testing"

// TestAnUnquotedExtendsIsRead: CFML lets an attribute value go unquoted, and
// TestBox's and fw1's own specs write `component extends=testbox.system.BaseSpec {`.
// The script parser took only a quoted value, so the spec had no base and
// every describe() and it() in it was "not found in extends chain".
func TestAnUnquotedExtendsIsRead(t *testing.T) {
	for src, want := range map[string]string{
		"component extends=testbox.system.BaseSpec {\n function run(){} }": "testbox.system.BaseSpec",
		"component extends=Base accessors=true {}":                         "Base",
		`component extends="quoted.Base" {}`:                               "quoted.Base",
		"<cfcomponent extends=a.b.Base></cfcomponent>":                     "a.b.Base",
	} {
		if got := Parse("file:///x.cfc", src).Extends; got != want {
			t.Errorf("%q: extends %q, want %q", src, got, want)
		}
	}

	if pr := Parse("file:///x.cfc", "component extends=Base accessors=true {}"); !pr.Accessors {
		t.Errorf("accessors after an unquoted extends was not read")
	}
}

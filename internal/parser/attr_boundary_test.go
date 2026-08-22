package parser

import "testing"

// TestGetAttrMatchesWholeNames covers a mis-indexing bug: getAttr searched for
// the attribute name as a plain substring and only checked the text *after* the
// match, so a hit inside a longer attribute name counted. "name" matched the
// tail of "displayname=", and a function declared
//
//	<cffunction displayname="Donor Lookup" name="getDonor">
//
// was indexed under the name "Donor Lookup" — breaking goto-definition,
// completion and the unresolved scan for every tag-based CFC that puts
// displayname first.
func TestGetAttrMatchesWholeNames(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		attr string
		want string
	}{
		{
			name: "displayname does not satisfy name",
			tag:  `<cffunction displayname="Donor Lookup" name="getDonor" returntype="struct">`,
			attr: "name",
			want: "getDonor",
		},
		{
			name: "returntype does not satisfy type",
			tag:  `<cfargument returntype="struct" type="numeric" name="id">`,
			attr: "type",
			want: "numeric",
		},
		{
			name: "the longer name is still reachable",
			tag:  `<cffunction displayname="Donor Lookup" name="getDonor">`,
			attr: "displayname",
			want: "Donor Lookup",
		},
		{
			name: "a hyphenated attribute does not satisfy its suffix",
			tag:  `<div data-name="x" name="y">`,
			attr: "name",
			want: "y",
		},
		{
			name: "absent attribute still returns empty",
			tag:  `<cffunction displayname="Donor Lookup">`,
			attr: "returntype",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getAttr(tc.tag, tc.attr); got != tc.want {
				t.Errorf("getAttr(%q, %q) = %q, want %q", tc.tag, tc.attr, got, tc.want)
			}
		})
	}
}

// TestFunctionIndexedUnderItsRealName is the end-to-end version: whatever
// getAttr does, the parse result has to carry the declared name.
func TestFunctionIndexedUnderItsRealName(t *testing.T) {
	src := "<cfcomponent>\n" +
		"<cffunction displayname=\"Donor Lookup\" name=\"getDonor\" returntype=\"struct\">\n" +
		"<cfargument displayname=\"Donor Id\" name=\"donorId\" type=\"numeric\">\n" +
		"</cffunction>\n" +
		"</cfcomponent>"

	pr := Parse("file:///T.cfc", src)
	if len(pr.Funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(pr.Funcs))
	}

	fn := pr.Funcs[0]
	if fn.Name != "getDonor" {
		t.Errorf("function name = %q, want getDonor", fn.Name)
	}

	if len(fn.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(fn.Arguments))
	}

	if fn.Arguments[0].Name != "donorId" {
		t.Errorf("argument name = %q, want donorId", fn.Arguments[0].Name)
	}
}

// TestIsCloseTagFor covers the </cffunction> detection that was dead code: the
// length test excluded the exact 13-byte tag, and the slice compared
// "cffunction>" against "cffunction".
func TestIsCloseTagFor(t *testing.T) {
	cases := []struct {
		tag  string
		want bool
	}{
		{"</cffunction>", true},
		{"</cffunction >", true},
		{"</CFFUNCTION>", true},
		{"</cffunctionfoo>", false},
		{"<cffunction>", false},
		{"</cfcomponent>", false},
		{"</cf>", false},
	}

	for _, tc := range cases {
		if got := isCloseTagFor(tc.tag, "cffunction"); got != tc.want {
			t.Errorf("isCloseTagFor(%q) = %v, want %v", tc.tag, got, tc.want)
		}
	}
}

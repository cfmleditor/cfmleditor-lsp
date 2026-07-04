package docs

import (
	"strings"
	"testing"
)

func TestParamValues(t *testing.T) {
	explicit := Param{Values: []string{"a", "b"}}
	if got := explicit.ParamValues(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("expected explicit Values to be returned as-is, got %v", got)
	}

	boolean := Param{Type: "boolean"}
	if got := boolean.ParamValues(); len(got) != 2 || got[0] != "true" || got[1] != "false" {
		t.Errorf("expected boolean type to infer [true false], got %v", got)
	}

	// Explicit Values takes priority over the boolean-type inference.
	both := Param{Type: "boolean", Values: []string{"yes", "no"}}
	if got := both.ParamValues(); len(got) != 2 || got[0] != "yes" {
		t.Errorf("expected explicit Values to override boolean inference, got %v", got)
	}

	neither := Param{Type: "string"}
	if got := neither.ParamValues(); got != nil {
		t.Errorf("expected nil for a plain string param with no Values, got %v", got)
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("one\ntwo\nthree"); got != "one" {
		t.Errorf("firstLine with newlines = %q, want %q", got, "one")
	}

	if got := firstLine("single line"); got != "single line" {
		t.Errorf("firstLine with no newline = %q, want unchanged input", got)
	}

	if got := firstLine(""); got != "" {
		t.Errorf("firstLine(\"\") = %q, want empty", got)
	}
}

func TestEntry_Doc(t *testing.T) {
	fn := Entry{
		Description: "Trims whitespace.",
		Script:      "trim(str)",
		Returns:     "string",
		Type:        "function",
		Params: []Param{
			{Name: "str", Type: "string", Required: true, Description: "input\nmore detail"},
		},
	}

	doc := fn.Doc()

	for _, want := range []string{
		"Trims whitespace.",
		"**Script:** `trim(str)`",
		"**Parameters:**",
		"`str` (string) *(required)* — input",
		"**Returns:** `string`",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("Entry.Doc() missing %q in:\n%s", want, doc)
		}
	}

	// A function's parameter-detail line should use only the first line of a
	// multi-line description, not the full text.
	if strings.Contains(doc, "more detail") {
		t.Errorf("Entry.Doc() should truncate param description to its first line, got:\n%s", doc)
	}

	tag := Entry{
		Description: "Sets a value.",
		Member:      "obj.set()",
		Type:        "tag",
		Returns:     "ignored-for-tags",
		Params:      []Param{{Name: "value", Type: "string"}},
	}

	tagDoc := tag.Doc()

	if !strings.Contains(tagDoc, "**Member:** `obj.set()`") {
		t.Errorf("Entry.Doc() missing Member line, got:\n%s", tagDoc)
	}

	if !strings.Contains(tagDoc, "**Attributes:**") {
		t.Errorf("expected tag entries to use 'Attributes' heading, got:\n%s", tagDoc)
	}

	if strings.Contains(tagDoc, "**Returns:**") {
		t.Error("Returns section should only be rendered for Type==\"function\", not tags")
	}

	minimal := Entry{Description: "Just a description."}
	if got := minimal.Doc(); got != "Just a description." {
		t.Errorf("minimal entry Doc() = %q, want just the description", got)
	}
}

func TestLookupTag(t *testing.T) {
	e, ok := LookupTag("cfif")
	if !ok || e == nil {
		t.Fatal("expected to find built-in tag cfif")
	}

	if e.Type != "tag" {
		t.Errorf("expected cfif Type=tag, got %q", e.Type)
	}

	// Case-insensitive.
	if _, ok := LookupTag("CFSET"); !ok {
		t.Error("expected case-insensitive tag lookup to find cfset")
	}

	if _, ok := LookupTag("not_a_real_tag_xyz"); ok {
		t.Error("expected no match for a nonexistent tag")
	}
}

func TestLookupFunction(t *testing.T) {
	e, ok := LookupFunction("trim")
	if !ok || e == nil {
		t.Fatal("expected to find built-in function trim")
	}

	if e.Type != "function" {
		t.Errorf("expected trim Type=function, got %q", e.Type)
	}

	if _, ok := LookupFunction("LEN"); !ok {
		t.Error("expected case-insensitive function lookup to find len")
	}

	if _, ok := LookupFunction("not_a_real_function_xyz"); ok {
		t.Error("expected no match for a nonexistent function")
	}
}

func TestAllTagsAndAllFunctions_PartitionByType(t *testing.T) {
	for _, e := range AllTags() {
		if e.Type != "tag" {
			t.Fatalf("AllTags() returned a non-tag entry: %+v", e)
		}
	}

	for _, e := range AllFunctions() {
		if e.Type != "function" {
			t.Fatalf("AllFunctions() returned a non-function entry: %+v", e)
		}
	}

	if len(AllTags()) == 0 || len(AllFunctions()) == 0 {
		t.Error("expected the built-in doc set to contain both tags and functions")
	}
}

func TestTagParams(t *testing.T) {
	if params := TagParams("cfargument"); len(params) == 0 {
		t.Error("expected cfargument to have documented params")
	}

	if params := TagParams("not_a_real_tag_xyz"); params != nil {
		t.Errorf("expected nil params for an unknown tag, got %v", params)
	}
}

func TestLookupBuiltinReturnComponentAndMethod(t *testing.T) {
	comp := LookupBuiltinReturnComponent("fileOpen")
	if comp != "$builtin.fileopen" {
		t.Errorf("LookupBuiltinReturnComponent(fileOpen) = %q, want $builtin.fileopen", comp)
	}

	// Case-insensitive.
	if LookupBuiltinReturnComponent("FILEOPEN") != "$builtin.fileopen" {
		t.Error("expected case-insensitive lookup for LookupBuiltinReturnComponent")
	}

	if got := LookupBuiltinReturnComponent("not_a_builtin_xyz"); got != "" {
		t.Errorf("expected empty string for unmapped function, got %q", got)
	}

	if !LookupBuiltinMethod("fileOpen", "close") {
		t.Error("expected fileOpen to have a close() method")
	}

	if !LookupBuiltinMethod("FILEOPEN", "CLOSE") {
		t.Error("expected case-insensitive method lookup")
	}

	if LookupBuiltinMethod("fileOpen", "notARealMethod") {
		t.Error("expected false for a method fileOpen doesn't have")
	}

	if LookupBuiltinMethod("not_a_builtin_xyz", "close") {
		t.Error("expected false when the function itself isn't a builtin-returning one")
	}
}

func TestHTMLTagParams(t *testing.T) {
	// "img" has specific attrs (src, alt, ...) plus the global attrs.
	imgParams := HTMLTagParams("img")
	if len(imgParams) == 0 {
		t.Fatal("expected img to have params")
	}

	hasGlobal := false
	hasSpecific := false

	for _, p := range imgParams {
		if p.Name == "class" {
			hasGlobal = true
		}

		if p.Name == "src" {
			hasSpecific = true
		}
	}

	if !hasGlobal {
		t.Error("expected HTMLTagParams(img) to include global attrs like 'class'")
	}

	if !hasSpecific {
		t.Error("expected HTMLTagParams(img) to include tag-specific attrs like 'src'")
	}

	// "div" is a real HTML tag with no tag-specific attrs entry — it must still get
	// the global attrs, not nil (only a genuinely unknown tag name returns nil).
	divParams := HTMLTagParams("div")
	if len(divParams) == 0 {
		t.Error("expected HTMLTagParams(div) to fall back to global attrs, not be empty")
	}

	if HTMLTagParams("not_a_real_html_tag_xyz") != nil {
		t.Error("expected nil for a name that isn't a known HTML tag at all")
	}

	// Case-insensitive.
	if len(HTMLTagParams("IMG")) != len(imgParams) {
		t.Error("expected case-insensitive HTMLTagParams lookup")
	}
}

func TestHTMLTags(t *testing.T) {
	tags := HTMLTags()
	if len(tags) == 0 {
		t.Fatal("expected a non-empty set of HTML tags")
	}

	for _, e := range tags {
		if e.Type != "tag" {
			t.Errorf("expected all HTMLTags() entries to have Type=tag, got %+v", e)
		}
	}
}

func TestMemberFunctionsForType(t *testing.T) {
	arrayFuncs := MemberFunctionsForType("array")
	if len(arrayFuncs) == 0 {
		t.Fatal("expected array to have member functions")
	}

	names := make(map[string]bool, len(arrayFuncs))
	for _, mf := range arrayFuncs {
		names[mf.Name] = true
	}

	if !names["append"] {
		t.Errorf("expected arrayAppend to produce member function 'append', got names: %v", names)
	}

	if !names["len"] {
		t.Errorf("expected arrayLen to produce member function 'len', got names: %v", names)
	}

	// Case-insensitive type lookup.
	if len(MemberFunctionsForType("ARRAY")) != len(arrayFuncs) {
		t.Error("expected case-insensitive MemberFunctionsForType lookup")
	}

	if got := MemberFunctionsForType("not_a_real_type_xyz"); got != nil {
		t.Errorf("expected nil for an unknown type, got %v", got)
	}
}

func TestAllMemberFunctions(t *testing.T) {
	all := AllMemberFunctions()
	if len(all) == 0 {
		t.Fatal("expected AllMemberFunctions to be non-empty")
	}

	if len(all) < len(MemberFunctionsForType("array")) {
		t.Error("expected AllMemberFunctions to include at least the array member functions")
	}
}

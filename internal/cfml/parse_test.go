package cfml

import (
	"strings"
	"testing"
)

func TestFindCommentSpans(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // number of comment spans
	}{
		{"cfml comment", `<!--- hidden --->visible`, 1},
		{"block comment", `/* hidden */visible`, 1},
		{"line comment", "// hidden\nvisible", 1},
		{"multiple", "/* a */\n// b\n<!--- c --->", 3},
		{"string not a comment", `x = "// not a comment"`, 0},
		{"single quote string", `x = '/* not a comment */'`, 0},
		{"none", "just code", 0},
		{"nested cfml", "<!--- outer <!--- inner ---> still comment --->visible", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := findCommentSpans(tt.input)
			if len(spans) != tt.want {
				t.Errorf("got %d spans, want %d", len(spans), tt.want)
			}
		})
	}
}

func TestInComment(t *testing.T) {
	content := "code /* comment */ more"
	spans := findCommentSpans(content)
	if inComment(0, spans) {
		t.Error("offset 0 should not be in comment")
	}
	// "/* comment */" starts at 5
	if !inComment(5, spans) {
		t.Error("offset 5 should be in comment")
	}
	if !inComment(10, spans) {
		t.Error("offset 10 should be in comment")
	}
	if inComment(18, spans) {
		t.Error("offset 18 should not be in comment")
	}
}

func TestIsScriptFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"component keyword", "component {\n}", true},
		{"interface keyword", "interface {\n}", true},
		{"property space", "property name=\"x\";\ncomponent {}", true},
		{"property tab", "property\tname=\"x\";", true},
		{"leading whitespace", "  \n\tcomponent {", true},
		{"comment then component", "<!--- header --->\ncomponent {", true},
		{"block comment then component", "/* header */\ncomponent {", true},
		{"tag based", "<cfcomponent>\n</cfcomponent>", false},
		{"empty", "", false},
		{"cffunction", "<cffunction name=\"test\">", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comments := findCommentSpans(tt.input)
			if got := isScriptFile(tt.input, comments); got != tt.want {
				t.Errorf("isScriptFile(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClassifyRegions_ScriptFile(t *testing.T) {
	content := "component {\n\tpublic function init() {}\n}"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionScript {
		t.Error("expected RegionScript")
	}
	if regions[0].Text != content {
		t.Error("expected original text preserved")
	}
}

func TestClassifyRegions_TagFile(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"test\">\n</cffunction>\n</cfcomponent>"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionTag {
		t.Error("expected RegionTag")
	}
}

func TestClassifyRegions_MixedTagWithCFScript(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"tagFunc\">\n</cffunction>\n<cfscript>\nfunction scriptFunc() {}\n</cfscript>\n</cfcomponent>"
	regions := ClassifyRegions(content)
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions, got %d", len(regions))
	}
	if regions[0].Kind != RegionTag {
		t.Errorf("region 0: expected Tag, got %v", regions[0].Kind)
	}
	if regions[1].Kind != RegionScript {
		t.Errorf("region 1: expected Script, got %v", regions[1].Kind)
	}
	if regions[2].Kind != RegionTag {
		t.Errorf("region 2: expected Tag, got %v", regions[2].Kind)
	}
}

func TestClassifyRegions_CommentBeforeComponent(t *testing.T) {
	content := "<!--- file header --->\ncomponent {\nfunction init() {}\n}"
	regions := ClassifyRegions(content)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].Kind != RegionScript {
		t.Error("expected RegionScript when comment precedes component keyword")
	}
}

func TestParseFunctionDefs_ScriptCFC(t *testing.T) {
	content := "component {\n\tpublic string function getUser() {}\n\tprivate function save() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"getUser", "save"})
}

func TestParseFunctionDefs_TagCFC(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"getUser\">\n</cffunction>\n<cffunction name=\"save\">\n</cffunction>\n</cfcomponent>"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"getUser", "save"})
}

func TestParseFunctionDefs_MixedTagAndCFScript(t *testing.T) {
	content := "<cfcomponent>\n<cffunction name=\"tagFunc\">\n</cffunction>\n<cfscript>\nfunction scriptFunc() {}\n</cfscript>\n</cfcomponent>"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"tagFunc", "scriptFunc"})
}

func TestParseFunctionDefs_CommentedOutFunction(t *testing.T) {
	content := "component {\n<!--- function hidden() {} --->\nfunction visible() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_BlockCommentedFunction(t *testing.T) {
	content := "component {\n/* function hidden() {} */\nfunction visible() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_LineCommentedFunction(t *testing.T) {
	content := "component {\n// function hidden() {}\nfunction visible() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_LineNumbers(t *testing.T) {
	content := "component {\n\n\tfunction first() {}\n\n\tfunction second() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if defs[0].Line != 2 {
		t.Errorf("first func line = %d, want 2", defs[0].Line)
	}
	if defs[1].Line != 4 {
		t.Errorf("second func line = %d, want 4", defs[1].Line)
	}
}

func TestParseFunctionDefs_CFScriptLineNumbers(t *testing.T) {
	content := "<cfcomponent>\n<cfscript>\n\nfunction myFunc() {}\n</cfscript>\n</cfcomponent>"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Line != 3 {
		t.Errorf("func line = %d, want 3", defs[0].Line)
	}
}

func TestParseFunctionDefs_MultilineComment(t *testing.T) {
	content := "component {\n/*\nfunction hidden() {}\n*/\nfunction visible() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_InterfaceFile(t *testing.T) {
	content := "interface {\n\tpublic function getData();\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"getData"})
}

func TestParseFunctionDefs_PropertyStart(t *testing.T) {
	content := "property name=\"id\" type=\"numeric\";\ncomponent {\n\tfunction init() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"init"})
}

func TestParseFunctionDefs_ScriptIgnoresTagRegex(t *testing.T) {
	content := "component {\n\tx = '<cffunction name=\"notReal\">';\n\tfunction real() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"real"})
}

func TestParseFunctionDefs_EmptyFile(t *testing.T) {
	defs := ParseFunctionDefs("file:///test.cfc", "")
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
}

func TestParseFunctionDefs_CommentOnlyFile(t *testing.T) {
	defs := ParseFunctionDefs("file:///test.cfc", "<!--- just a comment --->")
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
}

func TestParseFunctionDefs_CommentPreservesLineNumbers(t *testing.T) {
	content := "component {\n<!--- \nmultiline\ncomment\n--->\nfunction afterComment() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Line != 5 {
		t.Errorf("func line = %d, want 5", defs[0].Line)
	}
}

func TestParseFunctionDefs_TagDisplayNameIgnored(t *testing.T) {
	content := `<cffunction displayname="test" name="realName">`
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"realName"})
}

func TestParseFunctionDefs_TagDisplayNameOnly(t *testing.T) {
	content := `<cffunction displayname="test">`
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{})
}

func TestParseFunctionDefs_ScriptArgs(t *testing.T) {
	content := "component {\nfunction save(required string name, numeric age, flag) {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" || !args[0].Required {
		t.Errorf("arg 0 = %+v, want {name, string, required}", args[0])
	}
	if args[1].Name != "age" || args[1].Type != "numeric" || args[1].Required {
		t.Errorf("arg 1 = %+v, want {age, numeric, not required}", args[1])
	}
	if args[2].Name != "flag" || args[2].Type != "" || args[2].Required {
		t.Errorf("arg 2 = %+v, want {flag, \"\", not required}", args[2])
	}
}

func TestParseFunctionDefs_ScriptArgsWithDefaults(t *testing.T) {
	content := "component {\nfunction init(string name = \"test\", numeric count = 0) {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" {
		t.Errorf("arg 0 = %+v", args[0])
	}
	if args[1].Name != "count" || args[1].Type != "numeric" {
		t.Errorf("arg 1 = %+v", args[1])
	}
}

func TestParseFunctionDefs_ScriptNoArgs(t *testing.T) {
	content := "component {\nfunction init() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if len(defs[0].Arguments) != 0 {
		t.Errorf("expected 0 args, got %d", len(defs[0].Arguments))
	}
}

func TestParseFunctionDefs_TagArgs(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="save">
	<cfargument name="name" type="string" required="true">
	<cfargument name="age" type="numeric">
	<cfargument name="flag">
</cffunction>
</cfcomponent>`
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	args := defs[0].Arguments
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0].Name != "name" || args[0].Type != "string" || !args[0].Required {
		t.Errorf("arg 0 = %+v", args[0])
	}
	if args[1].Name != "age" || args[1].Type != "numeric" || args[1].Required {
		t.Errorf("arg 1 = %+v", args[1])
	}
	if args[2].Name != "flag" || args[2].Type != "" || args[2].Required {
		t.Errorf("arg 2 = %+v", args[2])
	}
}

func TestParseFunctionDefs_TagArgsMultipleFunctions(t *testing.T) {
	content := `<cfcomponent>
<cffunction name="first">
	<cfargument name="a">
</cffunction>
<cffunction name="second">
	<cfargument name="b">
	<cfargument name="c">
</cffunction>
</cfcomponent>`
	defs := ParseFunctionDefs("file:///test.cfc", content)
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if len(defs[0].Arguments) != 1 || defs[0].Arguments[0].Name != "a" {
		t.Errorf("first func args = %+v", defs[0].Arguments)
	}
	if len(defs[1].Arguments) != 2 || defs[1].Arguments[0].Name != "b" || defs[1].Arguments[1].Name != "c" {
		t.Errorf("second func args = %+v", defs[1].Arguments)
	}
}

func TestParseFunctionDefs_NestedCFMLComment(t *testing.T) {
	content := "component {\n<!--- outer <!--- function hidden() {} ---> still comment --->\nfunction visible() {}\n}"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func TestParseFunctionDefs_TagCommentedOutFunction(t *testing.T) {
	content := "<cfcomponent>\n<!--- <cffunction name=\"hidden\"> --->\n<cffunction name=\"visible\">\n</cffunction>\n</cfcomponent>"
	defs := ParseFunctionDefs("file:///test.cfc", content)
	assertDefs(t, defs, []string{"visible"})
}

func assertDefs(t *testing.T, defs []FunctionDef, want []string) {
	t.Helper()
	if len(defs) != len(want) {
		names := make([]string, len(defs))
		for i, d := range defs {
			names[i] = d.Name
		}
		t.Fatalf("got %d defs %v, want %d %v", len(defs), names, len(want), want)
	}
	for i, d := range defs {
		if d.Name != want[i] {
			t.Errorf("def[%d].Name = %q, want %q", i, d.Name, want[i])
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && strings.Contains(s, sub)
}

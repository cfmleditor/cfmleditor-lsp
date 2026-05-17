
package formatter

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

func parse(t *testing.T, src string) *sitter.Tree {
	t.Helper()
	return language.Parse(language.CFML, []byte(src), nil)
}

func testOpts() Options {
	opts := DefaultOptions()
	opts.UseTabs = false
	opts.ParseScript = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFScript, src, nil)
	}
	opts.ParseQuery = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFQuery, src, nil)
	}
	opts.ParseCFML = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFML, src, nil)
	}
	return opts
}

func format(t *testing.T, src string) string {
	t.Helper()
	tree := parse(t, src)
	out, err := Format([]byte(src), tree, testOpts())
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	return string(out)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain:\n%q\ngot:\n%q", want, got)
	}
}

func assertNotContains(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", unwanted, got)
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestSelfClosingTagLowerCase(t *testing.T) {
	src := `<CFSET foo = "bar">`
	got := format(t, src)
	assertContains(t, got, "<cfset")
	assertNotContains(t, got, "<CFSET")
}

func TestAttributeDoubleQuotes(t *testing.T) {
	src := `<cfparam name='hello'>`
	got := format(t, src)
	assertContains(t, got, `name="hello"`)
}

func TestBlockTagIndentation(t *testing.T) {
	src := `<cfif x EQ 1><cfset y = 2></cfif>`
	got := format(t, src)
	// cfset should be indented inside cfif
	lines := strings.Split(got, "\n")
	var cfsetLine string
	for _, l := range lines {
		if strings.Contains(l, "<cfset") {
			cfsetLine = l
			break
		}
	}
	if !strings.HasPrefix(cfsetLine, "    ") {
		t.Errorf("expected <cfset> to be indented, got: %q", cfsetLine)
	}
}

func TestNestedIndentation(t *testing.T) {
	src := `<cfoutput><cfloop array="#items#" item="i"><cfset x = i></cfloop></cfoutput>`
	got := format(t, src)
	lines := strings.Split(got, "\n")
	var cfsetLine string
	for _, l := range lines {
		if strings.Contains(l, "<cfset") {
			cfsetLine = l
			break
		}
	}
	// Should be indented by 2 levels (8 spaces)
	if !strings.HasPrefix(cfsetLine, "        ") {
		t.Errorf("expected double indent for nested <cfset>, got: %q", cfsetLine)
	}
}

func TestMultiAttrExpansion(t *testing.T) {
	src := `<cfquery name="q" datasource="ds" maxrows="10" timeout="30" cachedwithin="0.1">SELECT 1</cfquery>`
	got := format(t, src)
	// 5 attrs > default threshold of 4 → should expand
	lines := strings.Split(got, "\n")
	cfqueryLines := 0
	for _, l := range lines {
		tr := strings.TrimSpace(l)
		if strings.HasPrefix(tr, "name=") ||
			strings.HasPrefix(tr, "datasource=") ||
			strings.HasPrefix(tr, "maxrows=") ||
			strings.HasPrefix(tr, "timeout=") ||
			strings.HasPrefix(tr, "cachedwithin=") {
			cfqueryLines++
		}
	}
	if cfqueryLines < 5 {
		t.Errorf("expected expanded attributes on separate lines, got:\n%s", got)
	}
}

func TestInlineAttrNotExpanded(t *testing.T) {
	src := `<cfset x = 1>`
	got := format(t, src)
	lines := strings.Split(got, "\n")
	for _, l := range lines {
		if strings.Contains(l, "<cfset") {
			// Should be a single line tag
			if !strings.Contains(l, "x") {
				t.Errorf("single-attr tag should stay inline, got:\n%s", got)
			}
			return
		}
	}
}

func TestIdempotency(t *testing.T) {
	src := `<cfif condition>
    <cfset x = 1>
</cfif>
`
	got1 := format(t, src)
	tree2 := parse(t, got1)
	got2, err := Format([]byte(got1), tree2, testOpts())
	if err != nil {
		t.Fatalf("second format error: %v", err)
	}
	if got1 != string(got2) {
		t.Errorf("formatter is not idempotent.\nFirst pass:\n%s\nSecond pass:\n%s", got1, string(got2))
	}
}

func TestCFScriptBlock(t *testing.T) {
	src := `<cfscript>
var x = 1;
var y = 2;
</cfscript>`
	got := format(t, src)
	assertContains(t, got, "<cfscript>")
	assertContains(t, got, "</cfscript>")
	lines := strings.Split(got, "\n")
	for _, l := range lines {
		if strings.Contains(l, "var x") || strings.Contains(l, "var y") {
			if !strings.HasPrefix(l, "    ") {
				t.Errorf("cfscript body should be indented, got: %q", l)
			}
		}
	}
}

func TestCommentPreserved(t *testing.T) {
	src := `<!--- This is a CF comment --->
<cfset x = 1>`
	got := format(t, src)
	assertContains(t, got, "<!---")
	assertContains(t, got, "--->")
}

func TestCFQuerySQLClauseLineBreaks(t *testing.T) {
	src := `<cfquery name="q" datasource="ds">SELECT id, name FROM users WHERE active = 1 ORDER BY name</cfquery>`
	got := format(t, src)
	assertContains(t, got, "\n")
	// Each SQL clause keyword should start on its own line
	lines := strings.Split(got, "\n")
	var selectLine, fromLine, whereLine, orderLine bool
	for _, l := range lines {
		tr := strings.TrimSpace(l)
		if strings.HasPrefix(tr, "SELECT") {
			selectLine = true
		}
		if strings.HasPrefix(tr, "FROM") {
			fromLine = true
		}
		if strings.HasPrefix(tr, "WHERE") {
			whereLine = true
		}
		if strings.HasPrefix(tr, "ORDER") {
			orderLine = true
		}
	}
	if !selectLine || !fromLine || !whereLine || !orderLine {
		t.Errorf("expected SQL clauses on separate lines, got:\n%s", got)
	}
}

func TestCFQueryWithCFQueryparam(t *testing.T) {
	src := `<cfquery name="q" datasource="ds">SELECT id FROM users WHERE id = <cfqueryparam cfsqltype="CF_SQL_INTEGER" value="#arguments.id#"></cfquery>`
	got := format(t, src)
	assertContains(t, got, "cfqueryparam")
	assertContains(t, got, "WHERE")
}

func TestTabIndentation(t *testing.T) {
	opts := testOpts()
	opts.UseTabs = true
	src := `<cfif x EQ 1><cfset y = 2></cfif>`
	tree := parse(t, src)
	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	lines := strings.Split(got, "\n")
	var cfsetLine string
	for _, l := range lines {
		if strings.Contains(l, "<cfset") {
			cfsetLine = l
			break
		}
	}
	if !strings.HasPrefix(cfsetLine, "\t") {
		t.Errorf("expected tab indent, got: %q", cfsetLine)
	}
}

func TestEmptyFile(t *testing.T) {
	got := format(t, "")
	if got != "" {
		t.Errorf("expected empty output for empty input, got: %q", got)
	}
}

func TestCFSetWithHashExpression(t *testing.T) {
	src := `<cfset x = "#variables.name#">`
	got := format(t, src)
	assertContains(t, got, `"#variables.name#"`)
}

func TestCFLoopIndentation(t *testing.T) {
	src := `<cfloop from="1" to="10" index="i"><cfset x = i></cfloop>`
	got := format(t, src)
	lines := strings.Split(got, "\n")
	var cfsetLine string
	for _, l := range lines {
		if strings.Contains(l, "<cfset") {
			cfsetLine = l
			break
		}
	}
	if !strings.HasPrefix(cfsetLine, "    ") {
		t.Errorf("expected indented cfset inside cfloop, got: %q", cfsetLine)
	}
}

func TestCFTryCatchFormatting(t *testing.T) {
	src := `<cftry><cfset x = 1><cfcatch type="any"><cfset y = 2></cfcatch></cftry>`
	got := format(t, src)
	assertContains(t, got, "<cftry>")
	assertContains(t, got, "<cfcatch")
	assertContains(t, got, "</cftry>")
}

func TestInlineTextPreservesSpacing(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"cfoutput mixed text and hash", `<cfoutput>Hello #name# world</cfoutput>`, "Hello #name# world"},
		{"html element with inline children", `<p>Hello <strong>world</strong> foo</p>`, "Hello <strong>world</strong> foo"},
		{"html text between tags", `<p>text <em>emphasis</em> more text</p>`, "text <em>emphasis</em> more text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := format(t, tc.src)
			assertContains(t, got, tc.want)
		})
	}
}

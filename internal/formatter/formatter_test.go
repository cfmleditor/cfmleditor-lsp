
package formatter_test

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func parse(t *testing.T, src string) *sitter.Tree {
	t.Helper()
	return parser.Parse(parser.CFML, []byte(src), nil)
}

func testOpts() formatter.Options {
	opts := formatter.DefaultOptions()
	opts.ParseScript = func(src []byte) *sitter.Tree {
		return parser.Parse(parser.CFScript, src, nil)
	}
	return opts
}

func format(t *testing.T, src string) string {
	t.Helper()
	tree := parse(t, src)
	out, err := formatter.Format([]byte(src), tree, testOpts())
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
	src := `<cfquery name="q" datasource="ds" maxrows="10" timeout="30">SELECT 1</cfquery>`
	got := format(t, src)
	// 4 attrs > default threshold of 3 → should expand
	lines := strings.Split(got, "\n")
	cfqueryLines := 0
	for _, l := range lines {
		tr := strings.TrimSpace(l)
		if strings.HasPrefix(tr, "name=") ||
			strings.HasPrefix(tr, "datasource=") ||
			strings.HasPrefix(tr, "maxrows=") ||
			strings.HasPrefix(tr, "timeout=") {
			cfqueryLines++
		}
	}
	if cfqueryLines < 4 {
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
	got2, err := formatter.Format([]byte(got1), tree2, testOpts())
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

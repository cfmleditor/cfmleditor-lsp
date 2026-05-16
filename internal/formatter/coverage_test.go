package formatter

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// handledKinds lists every node kind that formatNode dispatches explicitly
// (not via the default passthrough). Keep this in sync with the switch in
// formatNode.
var handledKinds = map[string]bool{
	"program":              true,
	"component_file":       true,
	"cf_component_content": true,
	"cf_component_open_tag":  true,
	"cf_component_close_tag": true,
	"cf_tag":               true,
	"cf_set_tag":           true,
	"cf_return_tag":        true,
	"cf_selfclose_tag":     true,
	"cf_if_tag":                 true,
	"cf_if_alt":                 true,
	"cf_elseif_tag":             true,
	"cf_else_tag":               true,
	"cf_output_tag":        true,
	"cf_function_tag":      true,
	"cf_query_tag":         true,
	"cf_xml_tag":           true,
	"cf_savecontent_tag":        true,
	"cf_selfclose_void_tag_end": true,
	"cf_script_tag":             true,
	"assignment_expression":          true,
	"binary_expression":              true,
	"unary_expression":               true,
	"ternary_expression":             true,
	"elvis_expression":               true,
	"update_expression":              true,
	"call_expression":                true,
	"member_expression":              true,
	"subscript_expression":           true,
	"new_expression":                 true,
	"sequence_expression":            true,
	"augmented_assignment_expression": true,
	"parenthesized_expression":       true,
	"hash_expression":      true,
	"element":              true,
	"html_text":            true,
	"text":                 true,
	"comment":              true,
}

// TestIdempotencyBroad formats a variety of inputs and asserts that formatting
// the output a second time produces identical results.
func TestIdempotencyBroad(t *testing.T) {
	samples := []struct {
		name string
		src  string
	}{
		{"set", `<cfset x = 1>`},
		{"query simple", `<cfquery name="q" datasource="ds">SELECT 1</cfquery>`},
		{"query with queryparam", `<cfquery name="q" datasource="ds">SELECT * FROM t WHERE id = <cfqueryparam value="#id#" cfsqltype="cf_sql_integer"></cfquery>`},
		{"output with hash", `<cfoutput>#name#</cfoutput>`},
		{"output mixed", `<cfoutput>Hello #name#!</cfoutput>`},
		{"if/elseif/else", `<cfif x EQ 1><cfset y = 1><cfelseif x EQ 2><cfset y = 2><cfelse><cfset y = 3></cfif>`},
		{"loop", `<cfloop array="#items#" item="i"><cfset x = i></cfloop>`},
		{"function", `<cffunction name="f"><cfargument name="a" type="string"><cfreturn a></cffunction>`},
		{"component", `<cfcomponent><cffunction name="g"><cfreturn 1></cffunction></cfcomponent>`},
		{"script", `<cfscript>var x = 1;</cfscript>`},
		{"comment", `<!--- comment --->`},
		{"try/catch", `<cftry><cfset x = 1><cfcatch type="any"><cfset y = 2></cfcatch></cftry>`},
		{"nested component", `<cfcomponent><cffunction name="getData" access="public" returntype="query"><cfargument name="id" type="numeric" required="true"><cfquery name="q" datasource="myds">SELECT * FROM items WHERE id = <cfqueryparam value="#arguments.id#" cfsqltype="cf_sql_integer"></cfquery><cfreturn q></cffunction></cfcomponent>`},
		{"html element", `<div class="test"><p>Hello</p></div>`},
		{"script component", `component { function getData() { return 1; } }`},
	}

	opts := testOpts()
	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			tree1 := language.Parse(language.CFML, []byte(s.src), nil)
			defer tree1.Close()
			out1, err := Format([]byte(s.src), tree1, opts)
			if err != nil {
				t.Fatalf("pass 1: %v", err)
			}

			tree2 := language.Parse(language.CFML, out1, nil)
			defer tree2.Close()
			out2, err := Format(out1, tree2, opts)
			if err != nil {
				t.Fatalf("pass 2: %v", err)
			}

			if string(out1) != string(out2) {
				t.Errorf("not idempotent\nPass 1:\n%s\nPass 2:\n%s", out1, out2)
			}
		})
	}
}

// TestUnhandledNodeKinds walks the parse tree for representative inputs and
// reports named node kinds that have children but fall through to the default
// case in formatNode.
func TestUnhandledNodeKinds(t *testing.T) {
	samples := []string{
		`<cfset x = 1>`,
		`<cfquery name="q" datasource="ds">SELECT 1</cfquery>`,
		`<cfoutput>#x#</cfoutput>`,
		`<cfoutput>Hello #name#!</cfoutput>`,
		`<cfif x EQ 1><cfset y = 1><cfelseif x EQ 2><cfset y = 2><cfelse><cfset y = 3></cfif>`,
		`<cfloop array="#items#" item="i"><cfset x = i></cfloop>`,
		`<cffunction name="f"><cfargument name="a" type="string"><cfreturn a></cffunction>`,
		`<cfcomponent><cffunction name="g"><cfreturn 1></cffunction></cfcomponent>`,
		`<cfscript>var x = 1;</cfscript>`,
		`<!--- comment --->`,
		`<cftry><cfset x = 1><cfcatch type="any"><cfset y = 2></cfcatch></cftry>`,
		`<div class="test"><p>Hello</p></div>`,
		`component { function getData() { return 1; } }`,
	}

	nonLeaf := map[string]bool{}
	leaf := map[string]bool{}

	for _, src := range samples {
		tree := language.Parse(language.CFML, []byte(src), nil)
		defer tree.Close()
		collectUnhandled(tree.RootNode(), nonLeaf, leaf)
	}

	for kind := range nonLeaf {
		t.Logf("WARNING: non-leaf node kind %q uses default passthrough", kind)
	}
	for kind := range leaf {
		t.Logf("info: leaf node kind %q uses default passthrough (OK)", kind)
	}
}

func collectUnhandled(n *sitter.Node, nonLeaf, leaf map[string]bool) {
	if n.IsNamed() && !handledKinds[n.Kind()] {
		if n.ChildCount() > 0 {
			nonLeaf[n.Kind()] = true
		} else {
			leaf[n.Kind()] = true
		}
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		collectUnhandled(n.Child(i), nonLeaf, leaf)
	}
}

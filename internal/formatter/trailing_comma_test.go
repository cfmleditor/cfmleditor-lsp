package formatter

import (
	"strings"
	"testing"
)

// formatGuarded formats src with the whitespace-only guard switched on, the way
// the LSP does. Every case below is one the formatter used to change
// non-whitespace content on, so what the guard did in practice was refuse the
// file: format-on-save silently did nothing to it.
func formatGuarded(t *testing.T, src string) string {
	t.Helper()

	tree := parse(t, src)
	opts := testOpts()
	opts.WhitespaceOnly = true

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return string(out)
}

// TestTrailingCommaPreservedInCollections covers a struct or array literal whose
// last element is followed by a comma — legal in Lucee, Adobe CF and BoxLang, and
// common in hand-maintained lists because adding an entry then touches one line
// rather than two. The literal renderers collect the elements and rejoin them
// with ", ", reconstructing the separators from scratch, so the final comma was
// dropped. 3 files in the corpus, including two Application.cfc cache
// configurations and Lucee's own <cfdump> tag library.
func TestTrailingCommaPreservedInCollections(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "struct inline",
			src:  "<cfscript>\nx = { \"host\": \"h\", \"port\": 1, };\n</cfscript>\n",
			want: "\"port\": 1,",
		},
		{
			name: "array inline",
			src:  "<cfscript>\nx = [ \"one\", \"two\", ];\n</cfscript>\n",
			want: "\"two\",",
		},
		{
			name: "struct multi-line",
			src: "<cfscript>\nx = {\n\t\"alpha\": \"a fairly long value so the literal cannot be collapsed onto one line\",\n" +
				"\t\"beta\": \"another fairly long value so the literal cannot be collapsed\",\n};\n</cfscript>\n",
			want: "\"another fairly long value so the literal cannot be collapsed\",",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			assertContains(t, out, tt.want)
			assertReparses(t, out)
		})
	}
}

// TestTrailingCommaPreservedInParameterList covers the same defect in a function
// signature — `function init( required wirebox, )`. Both parameter renderers
// split on the comma children and rejoined with ", ", so the trailing one was
// dropped. ColdBox's own test harness has one.
func TestTrailingCommaPreservedInParameterList(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "one parameter per line",
			src:  "<cfscript>\ncomponent {\n\tpublic function init(\n\t\trequired wirebox,\n\t) {\n\t\treturn this;\n\t}\n}\n</cfscript>\n",
			want: "required wirebox,",
		},
		{
			name: "inline signature",
			src:  "<cfscript>\ncomponent {\n\tfunction f( required a, ) {}\n}\n</cfscript>\n",
			want: "required a,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			assertContains(t, out, tt.want)
			assertReparses(t, out)
		})
	}
}

// TestTrailingCommaPreservedInCallArguments covers the argument list of a call,
// which is rendered by a third code path with the same defect.
func TestTrailingCommaPreservedInCallArguments(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "inline",
			src:  "<cfscript>\nfoo( 1, 2, );\n</cfscript>\n",
			want: "2,",
		},
		{
			name: "broken over lines",
			src: "<cfscript>\nfoo(\n\t\"the first argument, long enough that the call cannot be collapsed\",\n" +
				"\t\"the second argument, also long enough to prevent collapsing\",\n\t\"a third\",\n\t\"a fourth\",\n);\n</cfscript>\n",
			want: "\"a fourth\",",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			assertContains(t, out, tt.want)
			assertReparses(t, out)
		})
	}
}

// TestTrailingCommaNotWrittenAfterComment is the boundary of the fix above. A
// comment can sit anywhere a parameter can, including after the list's final
// comma, and it is not an element — so the comma belongs after the last
// parameter, not at the end of the rendered list. Writing it at the end put the
// separator inside the comment (`//<cfargument stuff>,`), which the guard
// reported as changed comment text. Lucee's LDEV0285/App4.cfc, which the first
// version of the fix regressed.
func TestTrailingCommaNotWrittenAfterComment(t *testing.T) {
	src := "<cfscript>\ncomponent output=\"false\" {\n\tfunction subFunction(required one,\n\t\ttwo=1,\n\t\t//<cfargument stuff>\n\t) {\n\t\tcontainer(one, two);\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "two = 1,")
	assertNotContains(t, out, "stuff>,")
	assertReparses(t, out)
}

// TestListWithoutTrailingCommaDoesNotGainOne is the other direction: the
// detection has to be driven by the source's own comma, not applied to every
// list. A fix that simply always emitted one would insert a separator the source
// never had, which is the same class of defect pointing the other way.
func TestListWithoutTrailingCommaDoesNotGainOne(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "struct", src: "<cfscript>\nx = { \"host\": \"h\", \"port\": 1 };\n</cfscript>\n"},
		{name: "array", src: "<cfscript>\nx = [ \"one\", \"two\" ];\n</cfscript>\n"},
		{name: "call", src: "<cfscript>\nfoo( 1, 2 );\n</cfscript>\n"},
		{name: "parameters", src: "<cfscript>\ncomponent {\n\tfunction f( required a, string b ) {}\n}\n</cfscript>\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			for _, closer := range []string{",}", ", }", ",]", ", ]", ",)", ", )"} {
				if strings.Contains(out, closer) {
					t.Errorf("output gained a trailing comma (%q):\n%s", closer, out)
				}
			}

			assertReparses(t, out)
		})
	}
}

// TestUnmatchedComponentCloseTagDoesNotPanic covers `</cfcomponent>` with no
// opening tag before it. The grammar accepts it without an ERROR node, and the
// open and close tags are siblings rather than parent and child, so the close
// decremented an indentation level the open had never incremented. strings.Repeat
// panics on a negative count, so the formatter crashed rather than emitting the
// tag — reachable in an editor by deleting the opening line, and hit by
// Lucee's Jira2828.cfc in the corpus.
func TestUnmatchedComponentCloseTagDoesNotPanic(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "close tag alone", src: "</cfcomponent>\n"},
		{name: "close tag with content before it", src: "<cfset x = 1>\n</cfcomponent>\n"},
		{name: "two closes for one open", src: "<cfcomponent>\n<cfset x = 1>\n</cfcomponent>\n</cfcomponent>\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			assertContains(t, out, "</cfcomponent>")
		})
	}
}

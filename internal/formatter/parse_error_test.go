package formatter

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// stdOpts is the option set the CLI and the LSP both build.
func stdOpts() Options {
	opts := DefaultOptions()
	opts.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
	opts.ParseQuery = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
	opts.ParseCFML = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }

	return opts
}

func formatSrc(t *testing.T, src string, opts Options) ([]byte, error) {
	t.Helper()

	tree := language.Parse(language.CFML, []byte(src), nil)
	defer tree.Close()

	return Format([]byte(src), tree, opts)
}

// TestFormatRefusesErrorTree covers the corruption case: the grammar cannot
// parse a body-less <cfinvoke>/<cfhttp> inside <cfcomponent> and produces an
// ERROR node. The node walk has no rendering for one, so it used to fall
// through to a raw emit that ran the tag name and every attribute together
// (`<cfinvokecomponent="..."method="..."`), dropped </cfcomponent> and emitted
// a bogus </cf>. Format must refuse instead of returning that.
func TestFormatRefusesErrorTree(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "body-less cfinvoke in cfcomponent",
			src:  "<cfcomponent>\n\t<cfinvoke component=\"models.Widget\" method=\"render\" returnvariable=\"r\">\n</cfcomponent>\n",
		},
		{
			name: "body-less cfhttp in cfcomponent",
			src:  "<cfcomponent>\n\t<cfhttp url=\"/a\">\n</cfcomponent>\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := formatSrc(t, tc.src, stdOpts())
			if err == nil {
				t.Fatalf("expected a parse error, got output:\n%s", out)
			}

			if out != nil {
				t.Errorf("expected no output alongside the error, got %d bytes", len(out))
			}

			if !strings.Contains(err.Error(), "parse error in document") {
				t.Errorf("expected a document parse error, got %v", err)
			}
		})
	}
}

// TestFormatErrorTreeRefusedRegardlessOfGuard pins the refusal to the ERROR
// node itself, not to the whitespaceOnly guard. The guard only caught this
// by accident (the two streams ended up different lengths) and is off in
// several callers.
func TestFormatErrorTreeRefusedRegardlessOfGuard(t *testing.T) {
	src := "<cfcomponent>\n\t<cfinvoke component=\"a\" method=\"b\">\n</cfcomponent>\n"

	opts := stdOpts()
	opts.WhitespaceOnly = false

	out, err := formatSrc(t, src, opts)
	if err == nil {
		t.Fatalf("expected a parse error with the guard off, got output:\n%s", out)
	}
}

// TestFormatWellFormedTagsStillFormat guards against the ERROR check being
// too eager: the same tags with explicit closing tags must still format.
func TestFormatWellFormedTagsStillFormat(t *testing.T) {
	cases := []string{
		"<cfcomponent>\n\t<cfinvoke component=\"a\" method=\"b\"></cfinvoke>\n</cfcomponent>\n",
		"<cfcomponent>\n\t<cfset x = 1>\n</cfcomponent>\n",
		"<cfif true>\n\t<cfset x = 1>\n</cfif>\n",
	}

	for _, src := range cases {
		if _, err := formatSrc(t, src, stdOpts()); err != nil {
			t.Errorf("format %q: unexpected error %v", src, err)
		}
	}
}

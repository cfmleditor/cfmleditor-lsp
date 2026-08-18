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

// TestFormatPreservesBOM covers a leading UTF-8 BOM being dropped. The BOM sits
// outside every CST node, so the walk never emitted it and every BOM-prefixed
// file in the wild (554 of 5620 in a real-world corpus) failed the guard.
func TestFormatPreservesBOM(t *testing.T) {
	bom := "\ufeff"
	src := bom + "component {\n\tfunction a() {\n\t\treturn 1;\n\t}\n}\n"

	out, err := formatSrc(t, src, stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.HasPrefix(string(out), bom) {
		t.Errorf("BOM was dropped; output starts %q", string(out)[:min(12, len(out))])
	}

	if strings.HasPrefix(strings.TrimPrefix(string(out), bom), bom) {
		t.Error("BOM was duplicated")
	}
}

// TestFormatNoBOMAdded checks the BOM is not invented for files without one.
func TestFormatNoBOMAdded(t *testing.T) {
	out, err := formatSrc(t, "component {\n}\n", stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if strings.HasPrefix(string(out), "\ufeff") {
		t.Error("a BOM was added to a file that had none")
	}
}

// TestFormatKeepsPostParamAttributes covers attributes written after the
// parameter list. They are siblings of the parameters, and were being hoisted
// into the modifier prefix — `function f() localmode="true" {}` came back as
// `localmode="true" function f() {}`, which does not compile.
func TestFormatKeepsPostParamAttributes(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"component {\n\tfunction setup() localmode=\"true\" {}\n}\n", "function setup() localmode=\"true\""},
		{"component {\n\tfunction beforeAll() skip=\"isNotSupported\" {}\n}\n", "function beforeAll() skip=\"isNotSupported\""},
		{"component {\n\tremote function n() restpath=\"getName\" httpmethod=\"GET\" {}\n}\n", "remote function n() restpath=\"getName\" httpmethod=\"GET\""},
	}

	for _, tc := range cases {
		out, err := formatSrc(t, tc.src, stdOpts())
		if err != nil {
			t.Errorf("format %q: %v", tc.src, err)

			continue
		}

		if !strings.Contains(string(out), tc.want) {
			t.Errorf("format %q:\nwant substring %q\ngot:\n%s", tc.src, tc.want, out)
		}
	}
}

// TestFormatKeepsAnonymousReturnTypes covers return types the grammar
// tokenises as anonymous keyword nodes rather than named identifiers. Gating
// the signature prefix on IsNamed dropped them silently.
func TestFormatKeepsAnonymousReturnTypes(t *testing.T) {
	types := []string{
		"any", "array", "binary", "boolean", "component", "date", "guid",
		"numeric", "query", "string", "struct", "uuid", "variablename",
		"void", "xml", "function",
	}

	for _, ty := range types {
		src := "component {\n\tpublic " + ty + " function f() {}\n}\n"

		out, err := formatSrc(t, src, stdOpts())
		if err != nil {
			t.Errorf("format %s: %v", ty, err)

			continue
		}

		want := "public " + ty + " function f()"
		if !strings.Contains(string(out), want) {
			t.Errorf("return type %q lost: want substring %q, got:\n%s", ty, want, out)
		}
	}
}

// TestFormatKeepsAllCatchClauses covers two defects in scriptTry. Every catch
// clause carries the same `handler` field name, so ChildByFieldName returned
// only the first and the rest were deleted along with their bodies; and the
// exception type is a separate `type` field, so rendering only the parameter
// turned `catch (java.lang.Exception e)` into `catch (e)`.
func TestFormatKeepsAllCatchClauses(t *testing.T) {
	src := "component {\n" +
		"\tfunction b() {\n" +
		"\t\ttry { x(); }\n" +
		"\t\tcatch (java.lang.Exception e) { y(); }\n" +
		"\t\tcatch (any e2) { z(); }\n" +
		"\t\tfinally { w(); }\n" +
		"\t}\n}\n"

	out, err := formatSrc(t, src, stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	for _, want := range []string{
		"catch (java.lang.Exception e)",
		"catch (any e2)",
		"y();",
		"z();",
		"finally",
		"w();",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

// TestFormatKeepsDeclarationKeyword covers the component header being
// hardcoded to "component", which rewrote an interface as a component and
// dropped abstract/final modifiers.
func TestFormatKeepsDeclarationKeyword(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"interface {\n\tpublic function foo();\n}\n", "interface"},
		{"interface extends=\"x\" {\n}\n", "interface extends=\"x\""},
		{"abstract component extends=\"y\" {\n}\n", "abstract component extends=\"y\""},
		{"component extends=\"z\" {\n}\n", "component extends=\"z\""},
	}

	for _, tc := range cases {
		out, err := formatSrc(t, tc.src, stdOpts())
		if err != nil {
			t.Errorf("format %q: %v", tc.src, err)

			continue
		}

		if !strings.Contains(string(out), tc.want) {
			t.Errorf("format %q: want substring %q, got:\n%s", tc.src, tc.want, out)
		}
	}

	// An interface must not be silently turned into a component.
	out, err := formatSrc(t, "interface {\n\tpublic function foo();\n}\n", stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if strings.HasPrefix(strings.TrimSpace(string(out)), "component") {
		t.Errorf("interface was rewritten as a component:\n%s", out)
	}
}

// TestFormatPreservesStaticAccessor covers `::` being rendered as `.`.
// member_expression hardcoded the accessor, so Lucee/BoxLang static access
// `Widget::getData()` came back as the instance call `Widget.getData()`.
func TestFormatPreservesStaticAccessor(t *testing.T) {
	src := "component {\n\tfunction a() {\n\t\tvar d = Widget::getData();\n\t\tvar e = Foo::BAR;\n\t}\n}\n"

	out, err := formatSrc(t, src, stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	for _, want := range []string{"Widget::getData()", "Foo::BAR"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

// TestFormatCommentInsideLiteral covers the worst of the literal bugs: array
// and struct literals treated comment children as elements and joined them
// with ", ". Inlining a `//` comment commented out every element after it, so
// `[ // note \n a, b ]` became `[// note, a, b]` — the statement was destroyed.
func TestFormatCommentInsideLiteral(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "leading comment in array",
			src: "component {\n\tfunction a() {\n\t\tvar routes = [\n\t\t\t// leading comment\n" +
				"\t\t\t{ pattern: \"/\", handler: \"home\" },\n\t\t\t{ pattern: \"/x\", handler: \"x\" }\n\t\t];\n\t}\n}\n",
		},
		{
			name: "leading comment in struct",
			src: "component {\n\tfunction a() {\n\t\tvar s = {\n\t\t\t// note\n" +
				"\t\t\tcustomInterceptionPoints: \"a,b\",\n\t\t\tother: 1\n\t\t};\n\t}\n}\n",
		},
		{
			name: "comment between elements",
			src:  "component {\n\tfunction a() {\n\t\tvar s = {\n\t\t\ta: 1,\n\t\t\t// why\n\t\t\tb: 2\n\t\t};\n\t}\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := formatSrc(t, tc.src, stdOpts())
			if err != nil {
				t.Fatalf("format: %v", err)
			}

			// Every line comment must be the last thing on its line, otherwise
			// it has swallowed whatever followed it.
			for _, line := range strings.Split(string(out), "\n") {
				idx := strings.Index(line, "//")
				if idx < 0 {
					continue
				}

				if rest := strings.TrimSpace(line[idx+2:]); strings.Contains(rest, ",") {
					t.Errorf("a line comment swallowed following elements: %q\nfull output:\n%s", line, out)
				}
			}
		})
	}
}

// TestFormatCommentInLiteralIsNotDropped checks the comment survives at all —
// skipping comment children would also have fixed the swallowing, by deleting
// the comment.
func TestFormatCommentInLiteralIsNotDropped(t *testing.T) {
	src := "component {\n\tfunction a() {\n\t\tvar s = {\n\t\t\t// keep me\n\t\t\ta: 1\n\t\t};\n\t}\n}\n"

	out, err := formatSrc(t, src, stdOpts())
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.Contains(string(out), "// keep me") {
		t.Errorf("comment was dropped:\n%s", out)
	}
}

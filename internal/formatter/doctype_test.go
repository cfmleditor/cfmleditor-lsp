package formatter

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// TestDoctypePreserved covers the declaration body being dropped: the grammar
// does not expose it as a child node, so the generic child-walking path
// reassembled the node as "<!DOCTYPE>".
func TestDoctypePreserved(t *testing.T) {
	tests := []struct {
		name    string
		doctype string
	}{
		{
			name:    "html5",
			doctype: `<!DOCTYPE html>`,
		},
		{
			name:    "xhtml transitional",
			doctype: `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">`,
		},
		{
			name:    "html4 strict",
			doctype: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.doctype + "\n<html>\n<body>hi</body>\n</html>\n"

			out := format(t, src)
			if !strings.HasPrefix(out, tt.doctype+"\n") {
				t.Errorf("doctype not preserved verbatim\n got: %q\nwant prefix: %q", out, tt.doctype)
			}
		})
	}
}

// TestDoctypePassesWhitespaceOnlyGuard checks the guard, which is on by default
// in the LSP, no longer rejects the whole document because of the doctype.
func TestDoctypePassesWhitespaceOnlyGuard(t *testing.T) {
	src := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html>
<body>hi</body>
</html>
`

	opts := testOpts()
	opts.WhitespaceOnly = true

	if _, err := Format([]byte(src), parse(t, src), opts); err != nil {
		t.Fatalf("whitespaceOnly guard rejected a doctype document: %v", err)
	}
}

// TestRawTextElementsKeepTagSpacing covers <script>/<style>, whose start tags
// were reassembled from child tokens and lost the space between the tag name
// and its attributes (`<styletype="text/css">`).
func TestRawTextElementsKeepTagSpacing(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "style with attribute",
			src:  "<head>\n<style type=\"text/css\">\n.a { color: red; }\n</style>\n</head>\n",
			want: []string{`<style type="text/css">`, `.a { color: red; }`, `</style>`},
		},
		{
			name: "script with two attributes",
			src:  "<head>\n<script type=\"text/javascript\" src=\"x.js\"></script>\n</head>\n",
			want: []string{`<script type="text/javascript" src="x.js"></script>`},
		},
		{
			name: "script body emitted verbatim",
			src:  "<head>\n<script>\nvar a = 1;\nif (a) { b(); }\n</script>\n</head>\n",
			want: []string{"<script>", "var a = 1;", "if (a) { b(); }", "</script>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := format(t, tt.src)
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, out)
				}
			}
		})
	}
}

// TestRawTextElementRelativeIndentPreserved checks that indentation *inside* a
// script body is normalized as a block rather than flattened.
func TestRawTextElementRelativeIndentPreserved(t *testing.T) {
	src := "<head>\n<script>\n        function a() {\n            return 1;\n        }\n</script>\n</head>\n"

	out := format(t, src)
	if !strings.Contains(out, "function a() {\n") {
		t.Errorf("expected dedented function line\ngot:\n%s", out)
	}

	if !strings.Contains(out, "    return 1;\n") {
		t.Errorf("expected relative indent of body line preserved\ngot:\n%s", out)
	}
}

// TestXMLDeclPreserved covers the sibling failure to the doctype one above, found by
// the corpus harness in fw1's beans.xml.cfm. An xml_decl's parts are children ("<?",
// "xml", tag_attributes, "?>"), and the generic child-walking path joined them with
// nothing between, so <?xml version="1.0"?> came back as <?xmlversion="1.0"?>. Only
// whitespace was removed, so the whitespaceOnly guard passed it and the CLI wrote it
// to disk — leaving a file the grammar can no longer parse.
func TestXMLDeclPreserved(t *testing.T) {
	tests := []struct {
		name string
		decl string
	}{
		{
			name: "version only",
			decl: `<?xml version="1.0"?>`,
		},
		{
			name: "version and encoding",
			decl: `<?xml version="1.0" encoding="utf-8"?>`,
		},
		{
			name: "standalone",
			decl: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.decl + "\n<beans>\n\t<bean id=\"a\" />\n</beans>\n"

			out := format(t, src)
			if !strings.HasPrefix(out, tt.decl+"\n") {
				t.Errorf("xml declaration not preserved verbatim\n got: %q\nwant prefix: %q", out, tt.decl)
			}

			assertReparses(t, out)
		})
	}
}

// assertReparses checks the formatter's output is still parseable. A file that only
// lost whitespace passes the whitespaceOnly guard, so nothing else notices when that
// whitespace was the only thing holding a construct together.
func assertReparses(t *testing.T, out string) {
	t.Helper()

	tree := language.Parse(language.CFML, []byte(out), nil)
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Errorf("formatted output no longer parses:\n%s", out)
	}
}

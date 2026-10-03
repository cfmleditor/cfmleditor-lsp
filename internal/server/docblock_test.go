package server

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

func docCompletion(t *testing.T, srv *Server, text string, line, char uint32) []protocol.CompletionItem {
	t.Helper()

	docURI := uri.URI("file:///docblock.cfc")
	openDoc(t, srv, docURI, text)

	res, err := srv.handleCompletion(context.Background(), makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     protocol.Position{Line: line, Character: char},
	}))
	if err != nil {
		t.Fatal(err)
	}

	list, ok := res.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("result %T", res)
	}

	return list.Items
}

func labels(items []protocol.CompletionItem) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		out = append(out, items[i].Label)
	}

	slices.Sort(out)

	return out
}

// TestSlashStarStarExpandsToADocBlock: `/**` offers one item whose edit
// replaces it with a block for what follows — a tag line per argument of a
// function, a hint only for a property, component or interface — and the hint
// says which.
func TestSlashStarStarExpandsToADocBlock(t *testing.T) {
	for _, tc := range []struct{ name, doc, want string }{
		{"function", "component {\n\t/**\n\tfunction save( required user, flag = false ){}\n}", "/**\n * ${1:Undocumented function}\n *\n * @user ${2:}\n * @flag ${3:}\n */"},
		{"function without arguments", "component {\n\t/**\n\tfunction save(){}\n}", "/**\n * ${1:Undocumented function}\n */"},
		{"property", "component {\n\t/**\n\tproperty name=\"x\";\n}", "/**\n * ${1:Undocumented property}\n */"},
		{"component", "/**\ncomponent {\n}", "/**\n * ${1:Undocumented component}\n */"},
		{"interface", "/**\ninterface {\n}", "/**\n * ${1:Undocumented interface}\n */"},
		{"nothing follows", "component {\n\t/**\n}", "/**\n * ${1:Undocumented unknown}\n */"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, char := uint32(1), uint32(4)
			if tc.name == "component" || tc.name == "interface" {
				line, char = 0, 3
			}

			items := docCompletion(t, newTestServer(), tc.doc, line, char)
			if len(items) != 1 || items[0].Label != "/** */" {
				t.Fatalf("items %v", labels(items))
			}

			edit := textEdit(t, items[0].TextEdit)
			if edit.NewText != tc.want {
				t.Errorf("snippet\n%q\nwant\n%q", edit.NewText, tc.want)
			}

			if edit.Range.End.Character-edit.Range.Start.Character != 3 {
				t.Errorf("the edit does not replace the three characters of /**: %v", edit.Range)
			}
		})
	}
}

// TestDocBlockSettingsShapeTheSnippet: gap false drops the blank line, and an
// extra tag is added to the structures it names.
func TestDocBlockSettingsShapeTheSnippet(t *testing.T) {
	srv := newTestServer()
	srv.DocBlock = config.ResolvedDocBlock{Gap: false, Extra: []config.DocBlockExtra{
		{Name: "author", Default: "me"},
		{Name: "since", Default: "1.0", Types: []string{"component"}},
	}}

	items := docCompletion(t, srv, "component {\n\t/**\n\tfunction save( x ){}\n}", 1, 4)
	got := textEdit(t, items[0].TextEdit).NewText

	if want := "/**\n * ${1:Undocumented function}\n * @x ${2:}\n * @author ${3:me}\n */"; got != want {
		t.Errorf("function\n%q\nwant\n%q", got, want)
	}

	items = docCompletion(t, srv, "/**\ncomponent {\n}", 0, 3)
	got = textEdit(t, items[0].TextEdit).NewText

	if !strings.Contains(got, "@author ${2:me}") || !strings.Contains(got, "@since ${3:1.0}") {
		t.Errorf("component: %q", got)
	}
}

// TestAtInADocBlockOffersTheTagsAttributes: inside a doc block `@` offers the
// attributes of the tag it documents and a function's argument names, and
// `@arg.` the attributes of a cfargument; outside a doc block it offers
// nothing of the kind.
func TestAtInADocBlockOffersTheTagsAttributes(t *testing.T) {
	fn := "component {\n\t/**\n\t * Saves.\n\t * @\n\t */\n\tfunction save( required user, flag = false ){}\n}"
	got := labels(docCompletion(t, newTestServer(), fn, 3, 5))

	for _, want := range []string{"access", "returnType", "user", "flag"} {
		if !slices.Contains(got, want) {
			t.Errorf("function doc block lacks %q: %v", want, got)
		}
	}

	if slices.Contains(got, "name") {
		t.Errorf("name is the tag's own and is not offered: %v", got)
	}

	sub := labels(docCompletion(t, newTestServer(), strings.Replace(fn, "@\n", "@user.\n", 1), 3, 10))
	if !slices.Contains(sub, "required") || !slices.Contains(sub, "type") || slices.Contains(sub, "access") {
		t.Errorf("@user. should offer cfargument's attributes: %v", sub)
	}

	other := labels(docCompletion(t, newTestServer(), strings.Replace(fn, "@\n", "@nope.\n", 1), 3, 10))
	if len(other) != 0 {
		t.Errorf("@nope. is not an argument: %v", other)
	}

	prop := labels(docCompletion(t, newTestServer(), "component {\n\t/**\n\t * @\n\t */\n\tproperty name=\"x\";\n}", 2, 5))
	if !slices.Contains(prop, "type") || !slices.Contains(prop, "default") || slices.Contains(prop, "returnType") {
		t.Errorf("property doc block: %v", prop)
	}

	comp := labels(docCompletion(t, newTestServer(), "/**\n * @\n */\ncomponent {\n}", 1, 4))
	if !slices.Contains(comp, "extends") || !slices.Contains(comp, "output") {
		t.Errorf("component doc block: %v", comp)
	}

	iface := labels(docCompletion(t, newTestServer(), "/**\n * @\n */\ninterface {\n}", 1, 4))
	if !slices.Contains(iface, "extends") && len(iface) == 0 {
		t.Errorf("interface doc block offers nothing")
	}

	// After a doc block has closed, a later `@` is not in it.
	closed := "/** Done. */\ncomponent {\n\t// see @\n\tfunction save(){}\n}"
	// It is not a doc block request at all: the ordinary completion answers.
	if items := docCompletion(t, newTestServer(), closed, 2, 9); len(items) == 0 || slices.Contains(labels(items), "extends") {
		t.Errorf("an @ after a closed doc block was taken for a doc block: %d items", len(items))
	}

	// Not in a doc block: an ordinary comment, and code.
	if items := docCompletion(t, newTestServer(), "component {\n\t// @\n\tfunction save(){}\n}", 1, 5); slices.Contains(labels(items), "access") {
		t.Errorf("a line comment offered tag attributes: %v", labels(items))
	}
}

// TestAnAutoClosedCommentIsReplacedWhole: an editor that closes the comment as
// it opens it leaves `/** */` on the line, and the snippet carries its own
// closer, so the edit must cover the one that is there or the block ends twice.
// A bodiless declaration (an interface's) reads its own arguments, not the next
// one's.
func TestAnAutoClosedCommentIsReplacedWhole(t *testing.T) {
	items := docCompletion(t, newTestServer(), "component {\n\t/** */\n\tfunction save( required user ){}\n}", 1, 4)
	if len(items) != 1 {
		t.Fatalf("items %v", labels(items))
	}

	edit := textEdit(t, items[0].TextEdit)
	if edit.Range.Start.Character != 1 || edit.Range.End.Character != 7 {
		t.Errorf("the edit covers columns %d to %d of \"\\t/** */\", want 1 to 7", edit.Range.Start.Character, edit.Range.End.Character)
	}

	if !strings.Contains(edit.NewText, "@user") {
		t.Errorf("the function after the closed comment was not read: %q", edit.NewText)
	}

	// Without a closer on the line only the opener is replaced.
	open := textEdit(t, docCompletion(t, newTestServer(), "component {\n\t/**\n\tfunction save(){}\n}", 1, 4)[0].TextEdit)
	if open.Range.End.Character-open.Range.Start.Character != 3 {
		t.Errorf("an unclosed /** replaced %d characters", open.Range.End.Character-open.Range.Start.Character)
	}

	// An interface's bodiless functions: the first one's arguments.
	if got := docFunctionArguments("\nfunction a( required x, y );\nfunction b( required z );\n}"); !slices.Equal(got, []string{"x", "y"}) {
		t.Errorf("bodiless declarations: %v", got)
	}
}

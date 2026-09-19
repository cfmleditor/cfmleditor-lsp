package server

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// openInlineComponent indexes source as if it were a file in testdata, without
// writing one. The receivers under test are unresolvable by construction, so the
// fixtures say more inline than they would as six near-identical .cfc files.
func openInlineComponent(t *testing.T, srv *Server, name, content string) uri.URI {
	t.Helper()

	docURI := uri.URI("file://" + filepath.Join(testdataDir(), name))
	srv.setDocument(docURI, content)

	pr := parser.Parse(docURI, content, srv.cfResolvers())
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.ComponentRefs)

	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()

	return docURI
}

// locationURIs flattens whichever shape handleDefinition returned.
func locationURIs(result any) []uri.URI {
	switch v := result.(type) {
	case protocol.Location:
		return []uri.URI{v.URI}
	case []protocol.Location:
		out := make([]uri.URI, 0, len(v))
		for _, l := range v {
			out = append(out, l.URI)
		}

		return out
	default:
		return nil
	}
}

// A qualified call whose receiver cannot be resolved falls back to looking the
// method name up in the index. That fallback used to discard any definition in
// the requesting file, so where a component called its own method through an
// unresolvable receiver — which is every shape below — the answer was nothing at
// all, while the name sat in the index the whole time.
//
// Each case is a receiver the parser cannot type: a call to an unknown factory, a
// scoped variable, an argument declared `any`, a chain, and a bracket index. They
// are the shapes CLAUDE.md already documents as unresolvable, so each is a real
// dead end rather than a contrived one.
func TestQualifiedFallbackFindsDefinitionsInTheRequestingFile(t *testing.T) {
	cases := []struct {
		name string
		src  string
		line uint32
		char uint32
	}{
		{
			"receiver from an unknown factory",
			"component {\n\tfunction doThing() { return 1; }\n\tfunction c() {\n\t\tvar x = unknownFactory();\n\t\treturn x.doThing();\n\t}\n}\n",
			4, 14,
		},
		{
			"scoped receiver with no component ref",
			"component {\n\tfunction doThing() { return 1; }\n\tfunction c() {\n\t\treturn VARIABLES._svc.doThing();\n\t}\n}\n",
			3, 25,
		},
		{
			"argument declared any",
			"component {\n\tfunction doThing() { return 1; }\n\tfunction c(any a) {\n\t\treturn ARGUMENTS.a.doThing();\n\t}\n}\n",
			3, 22,
		},
		{
			"chained receiver",
			"component {\n\tfunction doThing() { return 1; }\n\tfunction c() {\n\t\treturn getX().y.doThing();\n\t}\n}\n",
			3, 19,
		},
		{
			"bracket-indexed receiver",
			"component {\n\tfunction doThing() { return 1; }\n\tfunction c() {\n\t\treturn REQUEST[k].doThing();\n\t}\n}\n",
			3, 20,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newTestdataServer()
			docURI := openInlineComponent(t, srv, "QualifiedFallback.cfc", c.src)

			got := locationURIs(definitionAt(t, srv, docURI, c.line, c.char))
			if len(got) == 0 {
				t.Fatalf("no definition, though doThing() is declared in this very file")
			}

			if got[0] != docURI {
				t.Errorf("expected %s, got %s", docURI, got[0])
			}
		})
	}
}

// The requesting file's own definition sorts last, because the qualifier is
// evidence against it: `x.doThing()` is not a call to this component's
// `doThing()`. Nearest-first would otherwise rank it top — nothing is nearer than
// the same file — and `myObj.init()` would land on the caller's own `init()`
// ahead of every real candidate.
func TestQualifiedFallbackRanksTheRequestingFileLast(t *testing.T) {
	srv := newTestdataServer()

	otherURI := openInlineComponent(t, srv, "QualifiedOther.cfc",
		"component {\n\tfunction doThing() { return 2; }\n}\n")

	docURI := openInlineComponent(t, srv, "QualifiedFallback.cfc",
		"component {\n\tfunction doThing() { return 1; }\n\tfunction c() {\n\t\tvar x = unknownFactory();\n\t\treturn x.doThing();\n\t}\n}\n")

	got := locationURIs(definitionAt(t, srv, docURI, 4, 14))
	if len(got) != 2 {
		t.Fatalf("expected both declarations, got %d: %v", len(got), got)
	}

	if got[0] != otherURI {
		t.Errorf("expected the other file first, got %s", got[0])
	}

	if got[1] != docURI {
		t.Errorf("expected the requesting file last, got %s", got[1])
	}
}

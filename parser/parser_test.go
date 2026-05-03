package parser_test

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/parser"
)

func TestLanguageLoads(t *testing.T) {
	for _, g := range []struct {
		name    string
		grammar parser.Grammar
	}{
		{"CFML", parser.CFML},
		{"CFScript", parser.CFScript},
		{"CFQuery", parser.CFQuery},
	} {
		if parser.Language(g.grammar) == nil {
			t.Errorf("%s language failed to load", g.name)
		}
	}
}

func TestParseCFML(t *testing.T) {
	src := []byte(`component { public void function hello() { return; } }`)
	tree := parser.Parse(parser.CFML, src)
	defer tree.Close()
	root := tree.RootNode()
	if root.ChildCount() == 0 {
		t.Fatal("expected children in CFML parse tree")
	}
}

func TestParseCFScript(t *testing.T) {
	src := []byte(`function greet(name) { return "Hello " & name; }`)
	tree := parser.Parse(parser.CFScript, src)
	defer tree.Close()
	root := tree.RootNode()
	if root.ChildCount() == 0 {
		t.Fatal("expected children in CFScript parse tree")
	}
}

func TestParseCFQuery(t *testing.T) {
	src := []byte(`SELECT id, name FROM users WHERE active = 1`)
	tree := parser.Parse(parser.CFQuery, src)
	defer tree.Close()
	root := tree.RootNode()
	if root.ChildCount() == 0 {
		t.Fatal("expected children in CFQuery parse tree")
	}
}

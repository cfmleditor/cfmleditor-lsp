package language

import (
	"testing"
)

func TestLanguageLoads(t *testing.T) {
	for _, g := range []struct {
		name    string
		grammar Grammar
	}{
		{"CFML", CFML},
		{"CFScript", CFScript},
		{"CFQuery", CFQuery},
	} {
		if Language(g.grammar) == nil {
			t.Errorf("%s language failed to load", g.name)
		}
	}
}

func TestParseCFML(t *testing.T) {
	src := []byte(`component { public void function hello() { return; } }`)
	tree := Parse(CFML, src, nil)
	defer tree.Close()
	if tree.RootNode().ChildCount() == 0 {
		t.Fatal("expected children in CFML parse tree")
	}
}

func TestParseCFScript(t *testing.T) {
	src := []byte(`function greet(name) { return "Hello " & name; }`)
	tree := Parse(CFScript, src, nil)
	defer tree.Close()
	if tree.RootNode().ChildCount() == 0 {
		t.Fatal("expected children in CFScript parse tree")
	}
}

func TestParseCFQuery(t *testing.T) {
	src := []byte(`SELECT id, name FROM users WHERE active = 1`)
	tree := Parse(CFQuery, src, nil)
	defer tree.Close()
	if tree.RootNode().ChildCount() == 0 {
		t.Fatal("expected children in CFQuery parse tree")
	}
}

package parser

import (
	"testing"

	"go.lsp.dev/uri"
)

const testURI = uri.URI("file:///test.cfc")

func TestParseComponentRefs_NewWithParens(t *testing.T) {
	refs := ParseComponentRefs(testURI, `myObj = new models.User()`)
	assertRef(t, refs, 0, "myObj", "models.User")
}

func TestParseComponentRefs_NewWithoutParens(t *testing.T) {
	refs := ParseComponentRefs(testURI, "myObj = new models.User\n")
	assertRef(t, refs, 0, "myObj", "models.User")
}

func TestParseComponentRefs_NewWithoutParensSemicolon(t *testing.T) {
	refs := ParseComponentRefs(testURI, `myObj = new models.User;`)
	assertRef(t, refs, 0, "myObj", "models.User")
}

func TestParseComponentRefs_NewQuotedPath(t *testing.T) {
	refs := ParseComponentRefs(testURI, `x = new "dir.Entity"()`)
	assertRef(t, refs, 0, "x", "dir.Entity")
}

func TestParseComponentRefs_NewQuotedNoParens(t *testing.T) {
	refs := ParseComponentRefs(testURI, "x = new 'dir.Entity'\n")
	assertRef(t, refs, 0, "x", "dir.Entity")
}

func TestParseComponentRefs_CreateObject(t *testing.T) {
	refs := ParseComponentRefs(testURI, `svc = CreateObject("component", "services.OrderService")`)
	assertRef(t, refs, 0, "svc", "services.OrderService")
}

func TestParseComponentRefs_EntityNew(t *testing.T) {
	refs := ParseComponentRefs(testURI, `user = entityNew("User")`)
	assertRef(t, refs, 0, "user", "User")
}

func TestParseComponentRefs_CfObject(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfobject component="dir.Entity" name="obj">`)
	assertRef(t, refs, 0, "obj", "dir.Entity")
}

func TestParseComponentRefs_CfObjectReversed(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfobject name="obj" component="dir.Entity">`)
	assertRef(t, refs, 0, "obj", "dir.Entity")
}

func TestParseComponentRefs_CfInvoke(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfinvoke component="svc.Helper" method="init" returnvariable="h">`)
	assertRef(t, refs, 0, "h", "svc.Helper")
}

func TestParseComponentRefs_CfInvokeReversed(t *testing.T) {
	refs := ParseComponentRefs(testURI, `<cfinvoke returnvariable="h" method="init" component="svc.Helper">`)
	assertRef(t, refs, 0, "h", "svc.Helper")
}

func TestParseComponentRefs_Multiple(t *testing.T) {
	src := "a = new foo.Bar()\nb = createObject(\"component\",\"baz.Qux\")\nc = new Simple\n"
	refs := ParseComponentRefs(testURI, src)
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	if refs[0].Line != 0 || refs[1].Line != 1 || refs[2].Line != 2 {
		t.Errorf("lines: %d, %d, %d", refs[0].Line, refs[1].Line, refs[2].Line)
	}
}

func TestParseComponentRefs_NoMatch(t *testing.T) {
	refs := ParseComponentRefs(testURI, "x = 42\ny = someFunction()")
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %d", len(refs))
	}
}

func assertRef(t *testing.T, refs []ComponentRef, idx int, variable, component string) {
	t.Helper()
	if len(refs) <= idx {
		t.Fatalf("expected at least %d refs, got %d", idx+1, len(refs))
	}
	if refs[idx].Variable != variable || refs[idx].Component != component {
		t.Errorf("ref[%d]: got Variable=%q Component=%q, want %q %q",
			idx, refs[idx].Variable, refs[idx].Component, variable, component)
	}
}

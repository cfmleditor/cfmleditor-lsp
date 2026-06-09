package parser

import (
	"testing"

	"go.lsp.dev/uri"
)

func TestJSDocParamTag(t *testing.T) {
	src := "<cfcomponent>\n<!--- @param {models.User} employee --->\n<cffunction name=\"test\">\n\t<cfargument name=\"employee\" type=\"any\">\n</cffunction>\n</cfcomponent>"

	pr := Parse(uri.URI("file:///test.cfc"), src)
	if len(pr.Funcs) == 0 {
		t.Fatal("no funcs found")
	}

	if pr.Funcs[0].Arguments[0].Type != "models.User" {
		t.Errorf("expected type models.User, got %q", pr.Funcs[0].Arguments[0].Type)
	}
}

func TestJSDocParamScript(t *testing.T) {
	src := "component {\n/** @param {models.User} employee */\nfunction test(any employee) {\n\temployee.doStuff();\n}\n}"

	pr := Parse(uri.URI("file:///test.cfc"), src)
	if len(pr.Funcs) == 0 {
		t.Fatal("no funcs found")
	}

	if pr.Funcs[0].Arguments[0].Type != "models.User" {
		t.Errorf("expected type models.User, got %q", pr.Funcs[0].Arguments[0].Type)
	}
}

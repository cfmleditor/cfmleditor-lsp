package parser

import (
	"testing"

	"go.lsp.dev/uri"
)

func TestUnquotedPropertyAttributesPreserveDottedTypes(t *testing.T) {
	pr := ParseWithOptions(uri.File("/tmp/model/User.cfc"), `component accessors=true {
 property name=contact type=stubs.Contact;
 property name=firstName type=string;
 property name=active type=boolean default=false setter=false;
 }`, &ParseOptions{})
	if !pr.Accessors {
		t.Fatal("unquoted accessors flag lost")
	}

	found := false

	for _, fn := range pr.Funcs {
		if fn.Name == "getContact" {
			found = true

			if fn.ReturnComponent != "stubs.Contact" {
				t.Fatalf("dotted type truncated: %+v", fn)
			}
		}
	}

	if !found {
		t.Fatal("unquoted property name lost")
	}

	attrs := pr.Properties[2].attrs
	if attrs["default"] != "false" || attrs["setter"] != "false" {
		t.Fatalf("unquoted boolean metadata lost: %v", attrs)
	}
}

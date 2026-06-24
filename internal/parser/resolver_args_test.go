package parser_test

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func TestResolverMatchWithArgs(t *testing.T) {
	resolvers := []parser.Resolver{
		{Match: "_objInit.getWebService", Resolve: "tassweb.packages.tass.customobjects.webservice", Prefix: "_objInit.getWebService"},
		{Match: "document", Resolve: "tassreporting.packages.reporting.itext", Prefix: "document"},
	}

	tests := []struct{
		expr string
		want string
	}{
		{"_objInit.getWebService(name=\"Foo\")", "tassweb.packages.tass.customobjects.webservice"},
		{"_objInit.getWebService()", "tassweb.packages.tass.customobjects.webservice"},
		{"createDocument(writer=foo)", ""},  // should NOT match "document" resolver
		{"document", "tassreporting.packages.reporting.itext"},
		{"document()", "tassreporting.packages.reporting.itext"},
	}

	for _, tt := range tests {
		got := parser.ResolveFromCall(tt.expr, resolvers)
		if got != tt.want {
			t.Errorf("ResolveFromCall(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

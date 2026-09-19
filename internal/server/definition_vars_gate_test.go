package server

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/uri"
)

// The switch has to stop the work, not just the answer.
//
// The gate lives in resolveVariableDef rather than in handleDefinition for the
// reason the watched-files switch documents: a gate in the caller is one a
// direct call slips past, and this function is reachable from more than one
// place once anything else wants variable resolution.
func TestVariableDefinitionsCanBeSwitchedOff(t *testing.T) {
	src := "<cfcomponent>\n" +
		"\t<cfset variables.thing = \"foo\">\n" +
		"\t<cfset ref = variables.thing>\n" +
		"</cfcomponent>\n"

	at := func(on bool) any {
		srv := newTestServer()
		srv.WorkspaceFolders = []string{testdataDir()}
		srv.Features = config.ResolveFeatures(&config.Features{VariableDefinitions: &on})

		abs := filepath.Join(testdataDir(), "SwitchTest.cfc")
		docURI := uri.URI("file://" + abs)
		srv.setDocument(docURI, src)

		// Cursor on `thing` in the reference on line 2.
		line, char := cursorPosition(t, src, "ref = variables.|thing")

		return definitionAt(t, srv, docURI, line, char)
	}

	if got := at(true); got == nil {
		t.Fatal("with the feature on, expected a definition")
	}

	if got := at(false); got != nil {
		t.Errorf("with the feature off, expected no definition, got %v", got)
	}
}

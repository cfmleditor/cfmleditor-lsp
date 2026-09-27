package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// typeDefProbe exercises each shape the extension's CFMLTypeDefinitionProvider
// answers, against the testdata workspace's real components.
const typeDefProbe = `component {
	property name="svc" type="services.UserService";

	function init(required models.User owner) {
		variables.users = new services.UserService();
		return this;
	}

	public models.User function getOwner() {
		var u = new models.User(1, "a", "b");
		var n = "name";
		u.getId();
		variables.users.getUser(1);
		n.len();
		getOwner();
		variables.users.createUser(1, "a", "b");
		return u;
	}

	function useArg(required models.User who) {
		who.getId();
		arguments.who.getId();
		this.getOwner();
	}
}
`

// openProbe opens src as a document inside testdata, so dot-paths resolve
// through the testdata server's mappings.
func openProbe(t *testing.T, srv *Server, name, src string) uri.URI {
	t.Helper()

	docURI := uri.File(filepath.Join(testdataDir(), name))
	srv.setDocument(docURI, src)
	pr := parser.Parse(docURI, src, srv.cfResolvers())
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.ComponentRefs)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()

	return docURI
}

// typeDefinitionAt returns the file typeDefinition lands in, or "" for none.
func typeDefinitionAt(t *testing.T, srv *Server, docURI uri.URI, src string, line int, needle string) string {
	t.Helper()

	text := strings.Split(src, "\n")[line]

	col := strings.Index(text, needle)
	if col < 0 {
		t.Fatalf("%q not on line %d: %q", needle, line, text)
	}

	req := makeCall(t, protocol.MethodTextDocumentTypeDefinition, protocol.TypeDefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     protocol.Position{Line: uint32(line), Character: lineCol(text, col+1)},
	})

	res, err := srv.handleTypeDefinition(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	loc, ok := res.(protocol.Location)
	if !ok {
		return ""
	}

	return filepath.Base(loc.URI.Path())
}

// Each shape the extension answers, plus the ones that must answer nothing: a
// string variable, and a call to a function with no declared or inferred
// component return type.
func TestTypeDefinitionScript(t *testing.T) {
	srv := newTestdataServer()
	docURI := openProbe(t, srv, "typedef_probe.cfc", typeDefProbe)

	cases := []struct {
		name   string
		line   int
		needle string
		want   string
	}{
		{"property", 1, "svc", "UserService.cfc"},
		{"argument declaration", 3, "owner", "User.cfc"},
		{"variables-scope assignment", 4, "users", "UserService.cfc"},
		{"function declaration: its return type", 8, "getOwner", "User.cfc"},
		{"local variable", 11, "u.", "User.cfc"},
		{"variables-scope use in another function", 12, "users", "UserService.cfc"},
		{"string variable", 13, "n.", ""},
		{"call: declared return type", 14, "getOwner", "User.cfc"},
		{"call on a receiver: inferred return type", 15, "createUser", "User.cfc"},
		{"method with no component return type", 12, "getUser", ""},
		{"unscoped argument", 20, "who", "User.cfc"},
		{"arguments-scope argument", 21, "who", "User.cfc"},
		{"this-qualified call", 22, "getOwner", "User.cfc"},
	}

	for _, c := range cases {
		if got := typeDefinitionAt(t, srv, docURI, typeDefProbe, c.line, c.needle); got != c.want {
			t.Errorf("%s (line %d, %q): got %q, want %q", c.name, c.line, c.needle, got, c.want)
		}
	}
}

const typeDefTagProbe = `<cfcomponent>
	<cffunction name="run" returntype="models.User">
		<cfargument name="svc" type="services.UserService">
		<cfset var u = createObject("component", "models.User")>
		<cfset arguments.svc.getUser(1)>
		<cfreturn u>
	</cffunction>
</cfcomponent>
`

const typeDefPageProbe = `<cfset user = new models.User(1, "a", "b")>
<cfoutput>#user.getId()#</cfoutput>
`

func TestTypeDefinitionTagSyntaxAndPages(t *testing.T) {
	srv := newTestdataServer()
	tagURI := openProbe(t, srv, "typedef_probe_tag.cfc", typeDefTagProbe)
	pageURI := openProbe(t, srv, "typedef_probe_page.cfm", typeDefPageProbe)

	cases := []struct {
		name   string
		doc    uri.URI
		src    string
		line   int
		needle string
		want   string
	}{
		{"cfargument", tagURI, typeDefTagProbe, 2, "svc", "UserService.cfc"},
		{"cfset var from createObject", tagURI, typeDefTagProbe, 5, "u>", "User.cfc"},
		{"arguments-scope use", tagURI, typeDefTagProbe, 4, "svc", "UserService.cfc"},
		{"page variable", pageURI, typeDefPageProbe, 1, "user", "User.cfc"},
	}

	for _, c := range cases {
		if got := typeDefinitionAt(t, srv, c.doc, c.src, c.line, c.needle); got != c.want {
			t.Errorf("%s (line %d, %q): got %q, want %q", c.name, c.line, c.needle, got, c.want)
		}
	}
}

// The capability is what makes an editor offer the command at all.
func TestTypeDefinitionIsAdvertised(t *testing.T) {
	caps := newTestServer().capabilities()
	if caps.TypeDefinitionProvider == nil {
		t.Fatal("typeDefinitionProvider is not advertised")
	}
}

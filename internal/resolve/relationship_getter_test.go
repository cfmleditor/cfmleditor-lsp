package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// TestASingleValuedRelationshipGetterReturnsItsEntity: a many-to-one or
// one-to-one ORM property names the entity it holds in its cfc attribute, so
// its generated getter returns one. A collection relationship holds an array
// of them, an explicit type is the property's own, and a computed cfc names
// nothing.
func TestASingleValuedRelationshipGetterReturnsItsEntity(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"system/Site.cfc":  `component persistent="true" { function getSlug(){} }`,
		"content/Note.cfc": `component persistent="true" { function getText(){} }`,
		"content/Content.cfc": `component persistent="true" {
property name="site" fieldtype="many-to-one" cfc="system.Site";
property name="note" fieldtype="one-to-one" cfc="Note";
property name="notes" fieldtype="one-to-many" cfc="Note";
property name="typed" fieldtype="many-to-one" cfc="Note" type="system.Site";
property name="computed" fieldtype="many-to-one" cfc="#variables.kind#";
}`,
		// Mura's ORM: not persistent, and its cfc is a bean id.
		"content/Bean.cfc": `component { property name="site" fieldtype="many-to-one" cfc="system.Site"; }`,
		"content/Page.cfc": `component { function run(){ var c = new Content(); var b = new Bean();
 b.getSite().getSlug();
 c.getSite().getSlug();
 c.getSite().nope();
 c.getNote().getText();
 c.getNotes().getText();
 c.getTyped().getSlug();
 c.getComputed().getText();
 }}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "content/Page.cfc"), map[string]string{
		"c.getSite.getSlug":     "",
		"b.getSite.getSlug":     "method 'getSite' in Bean has no component return type (chain to 'getSlug')",
		"c.getSite.nope":        "method 'nope' not found in system.Site",
		"c.getNote.getText":     "",
		"c.getNotes.getText":    "method 'getNotes' in Content has no component return type (chain to 'getText')",
		"c.getTyped.getSlug":    "",
		"c.getComputed.getText": "method 'getComputed' in Content has no component return type (chain to 'getText')",
	})
}

func TestANonPersistentRelationshipGetterUsesAProvenBean(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"model/User.cfc": `component { function login(){} }`,
		"OAuthClient.cfc": `component {
property name="user" fieldtype="many-to-one" cfc="user";
}`,
		"Page.cfc": `component {function run(){new OAuthClient().getUser().login();}}`,
	})

	bean := filepath.Join(dir, "model", "User.cfc")
	pr := parser.ParseWithOptions(cfpath.ToURI(filepath.Join(dir, "OAuthClient.cfc")), `component {
property name="user" fieldtype="many-to-one" cfc="user";
	}`, &parser.ParseOptions{Resolvers: []parser.Resolver{{Match: `getBean\("user"\)`, Resolve: bean, Prefix: "getBean"}}})

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, "getUser") {
			if pr.Funcs[i].ReturnComponent != bean {
				t.Fatalf("getUser return component = %q, want %q", pr.Funcs[i].ReturnComponent, bean)
			}

			return
		}
	}

	t.Fatal("generated getUser not found")
}

func TestANonPersistentCollectionRelationshipUsesAProvenBean(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"model/Role.cfc": `component { function getName(){} }`,
		"User.cfc": `component {
property name="roles" fieldtype="one-to-many" cfc="role";
property name="groups" fieldtype="many-to-many" cfc="group";
property name="unknowns" fieldtype="one-to-many" cfc="unknown";
}`,
		"Page.cfc": `component { function run(){ var user = new User();
for (var role in user.getRoles()) { role.getName(); }
for (var group in user.getGroups()) { group.getName(); }
for (var missing in user.getUnknowns()) { missing.getName(); }
}}`,
		"Page.cfm": `<cfset user = new User()>
<cfloop array="#user.getRoles()#" item="role">
	<cfset role.getName()>
</cfloop>`,
	})

	role := filepath.Join(dir, "model", "Role.cfc")
	r := &Resolver{Index: index.New()}
	r.Index.SetBeans(map[string]string{"role": role, "group": role})

	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{
		"role.getName":    "",
		"group.getName":   "",
		"missing.getName": "variable 'missing' has no component ref",
	})
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfm"), map[string]string{
		"role.getName": "",
	})
}

package resolve

import "testing"

// TestAModuleHelperIsAColdBoxComponents: cbi18n's applicationHelper gives
// every handler $r() and getResource(), and ContentBox's filebrowser calls
// $r fifty times without the module on disk. Outside a component that
// reaches ColdBox the name is nobody's helper.
func TestAModuleHelperIsAColdBoxComponents(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"handlers/Base.cfc": `component extends="coldbox.system.EventHandler" {}`,
		"handlers/Home.cfc": `component extends="Base" {
	function index( event, rc, prc ) {
		$r( "messages.hello@fb" );
		cbfs( "default" );
		notAHelper();
	}
}`,
		"lib/Plain.cfc": `component extends="Other" {
	function f() { $r( "x" ); }
}`,
		"lib/Other.cfc": `component {}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/Home.cfc"), map[string]string{
		"$r":         "",
		"cbfs":       "",
		"notAHelper": "not found in extends chain",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "lib/Plain.cfc"), map[string]string{
		"$r": "not found in extends chain",
	})
}

// TestAMementoGivesGetMemento: mementifier injects getMemento() into an
// object declaring this.memento, as ContentBox's entities do through their
// BaseEntity. An object declaring none has no getMemento.
func TestAMementoGivesGetMemento(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/BaseEntity.cfc": `component { this.memento = { defaultIncludes : [ "*" ] }; }`,
		"models/Role.cfc":       `component extends="BaseEntity" { function own() {} }`,
		"models/Plain.cfc":      `component { function own() {} }`,
		"Page.cfc": `component {
	function f() {
		var role = new models.Role();
		role.getMemento();
		var plain = new models.Plain();
		plain.getMemento();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"role.getMemento":  "",
		"plain.getMemento": "method 'getMemento' not found in models.Plain",
	})
}

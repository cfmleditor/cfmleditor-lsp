package resolve

import "testing"

// TestABareReturnTypeNamesTheComponentBesideIt: ColdBox declares
// `ColdBoxScheduledTask function task( name )`, and CFML finds that component
// beside the declaring one — so the chain on task() is checked there. Only
// there: a same-named file elsewhere is not it, and a CFML type name is the
// engine's type even with a component of that name beside it.
func TestABareReturnTypeNamesTheComponentBesideIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/Scheduler.cfc": `component {
	Task function task( name ) {}
	query function rows() {}
	Other function other() {}
}`,
		"lib/Task.cfc":        `component { function call() {} }`,
		"lib/query.cfc":       `component { function nope() {} }`,
		"elsewhere/Other.cfc": `component { function go() {} }`,
		"Page.cfc": `component {
	function f() {
		var s = new lib.Scheduler();
		s.task( "x" ).call();
		s.task( "x" ).missing();
		s.rows().nope();
		s.other().go();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"s.task.call":    "",
		"s.task.missing": "method 'missing' not found in Task",
		"s.rows.nope":    "method 'rows' in lib.Scheduler has no component return type (chain to 'nope')",
		"s.other.go":     "method 'other' in lib.Scheduler has no component return type (chain to 'go')",
	})
}

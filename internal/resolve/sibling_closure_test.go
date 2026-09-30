package resolve

import "testing"

// TestAnUnscopedAssignmentInAClosureReachesItsSiblings: TestBox's
// beforeEach( function(){ scheduler = … } ) puts scheduler in the
// component's variables scope, where every it() closure reads it.
func TestAnUnscopedAssignmentInAClosureReachesItsSiblings(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Scheduler.cfc":    `component { function task() {} }`,
		"AsyncManager.cfc": `component { Scheduler function newScheduler( name ) { return new Scheduler(); } }`,
		"Spec.cfc": `component {
	function run() {
		describe( "x", function() {
			beforeEach( function() {
				asyncManager = new AsyncManager();
				scheduler = asyncManager.newScheduler( "x" );
				direct = new Scheduler();
			} );
			it( "a", function() {
				scheduler.task();
				scheduler.missing();
				direct.task();
				direct.missing();
			} );
		} );
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Spec.cfc"), map[string]string{
		"scheduler.task":    "",
		"scheduler.missing": "method 'missing' not found in Scheduler",
		"direct.task":       "",
		"direct.missing":    "method 'missing' not found in Scheduler",
	})
}

// TestAReturnedVariableAssignedFromACallAtComponentLevel: ColdBox's
// getDateTimeHelper() assigns variables.cbDateTimeHelper from a call and
// returns it unscoped. The call settles after the parse, at component level,
// and the return is read from there.
func TestAReturnedVariableAssignedFromACallAtComponentLevel(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Helper.cfc":  `component { function iso() {} }`,
		"Factory.cfc": `component { Helper function make() { return new Helper(); } }`,
		"Page.cfc": `component {
	function getHelper() {
		variables.helper = variables.factory.make();
		return helper;
	}
	function f() {
		variables.factory = new Factory();
		getHelper().iso();
		getHelper().missing();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"getHelper.iso":     "",
		"getHelper.missing": "method 'missing' not found in Helper",
	})
}

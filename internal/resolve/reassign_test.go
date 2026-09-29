package resolve

import "testing"

// TestAFunctionLocalReassignedInABranchIsReadAtItsLatestAssignment: `var x`
// in one case and again in the next is two variables in practice, and a call
// after the second is on what the second holds. The function-scoped lookup
// took the first, where the file-level one has always taken the latest at or
// before the call, so every call in the second branch was checked against the
// first branch's component. A forward reference still takes the first.
func TestAFunctionLocalReassignedInABranchIsReadAtItsLatestAssignment(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"A.cfc": `component { function onlyA() {} }`,
		"B.cfc": `component { function onlyB() {} }`,
		"Page.cfc": `component {
	function f( kind ) {
		switch ( kind ) {
			case "a":
				var x = new A();
				x.onlyA();
				break;
			case "b":
				var x = new B();
				x.onlyB();
				break;
		}
	}
	function g() {
		y.onlyA();
		var y = new A();
		var y = new B();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"x.onlyA": "",
		"x.onlyB": "",
		"y.onlyA": "",
	})
}

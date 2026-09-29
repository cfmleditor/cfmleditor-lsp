package resolve

import "testing"

// TestThisAndVariablesRefsAnswerTheirOwnReceivers: `this.SCOPES = new
// Scopes()` beside a struct `variables.scopes` is ColdBox's Injector, and
// reading one scope's ref for the other checked `variables.scopes.x()`
// against Scopes. A receiver written `this.x` or `variables.x` is answered
// from its own scope — file-level refs, and the function-scoped ones a pending
// call makes — and an unqualified one from either.
func TestThisAndVariablesRefsAnswerTheirOwnReceivers(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Scopes.cfc": `component { function beta() {} }`,
		"Built.cfc":  `component { function zeta() {} }`,
		"Inj.cfc": `component {
	function init() {
		this.SCOPES = new Scopes();
		variables.scopes = {};
	}
	Built function make() {
		return new Built();
	}
	function run() {
		this.built = make();
		variables.scopes.alpha();
		this.scopes.beta();
		scopes.gamma();
		variables.built.epsilon();
		this.built.zeta();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Inj.cfc"), map[string]string{
		"variables.scopes.alpha":  "variable 'variables.scopes' has no component ref",
		"this.scopes.beta":        "",
		"scopes.gamma":            "method 'gamma' not found in Scopes",
		"variables.built.epsilon": "variable 'variables.built' has no component ref",
		"this.built.zeta":         "",
	})
}

package resolve

import "testing"

// TestAMockIsItsClass: createMock( "models.User" ) is a User with MockBox's
// methods added, so a method User lacks is a finding and $() is not. A mock
// of a class the workspace does not hold is dynamic, as the mock itself was
// before: a spec mocking a module that is not installed is not a finding.
func TestAMockIsItsClass(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/User.cfc": `component { function save() {} }`,
		"tests/UserSpec.cfc": `component {
	function run() {
		var u = createMock( "models.User" );
		u.save();
		u.$( "save", true );
		u.notAUserMethod();
		var e = createEmptyMock( className = "models.User" );
		e.save();
		var gone = createMock( "cbmailservices.models.Mail" );
		gone.send();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "tests/UserSpec.cfc"), map[string]string{
		"u.save":           "",
		"u.$":              "",
		"u.notAUserMethod": "method 'notAUserMethod' not found in models.User",
		"e.save":           "",
		"gone.send":        "",
	})
}

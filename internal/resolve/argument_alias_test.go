package resolve

import "testing"

func TestConstructorStoresTypedArgumentWithoutLeakingIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/User.cfc": `component { function save() {} }`,
		"Service.cfc": `component {
function init(required models.User employee) {
    variables.user = ARGUMENTS.employee;
    var alias = arguments.employee;
    alias.save();
    alias.missing();
    variables.child = arguments.employee.child;
    variables.text = arguments.employee & "text";
}
function useStored() { user.save(); user.missing(); child.save(); text.save(); }
function unrelated(employee) { employee.untyped(); }
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Service.cfc"), map[string]string{
		"alias.save":       "",
		"alias.missing":    "method 'missing' not found in models.User",
		"user.save":        "",
		"user.missing":     "method 'missing' not found in models.User",
		"child.save":       "variable 'child' has no component ref",
		"text.save":        "variable 'text' has no component ref",
		"employee.untyped": "variable 'employee' has no component ref",
	})
}

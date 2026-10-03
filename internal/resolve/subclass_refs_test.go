package resolve

import "testing"

// TestABaseComponentsVariableIsWhatItsSubclassesHold: an abstract component
// calls variables.svc and never assigns it; the receiver is typed by what every
// concrete subclass holds, as alternatives. A subclass that does not type it,
// or no subclass at all, leaves it untyped.
func TestABaseComponentsVariableIsWhatItsSubclassesHold(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/PageService.cfc":  `component { function pageOnly(){} function shared(){} }`,
		"models/EntryService.cfc": `component { function entryOnly(){} function shared(){} }`,
		"h/Base.cfc": `component {
function run(){
	variables.svc.pageOnly();
	variables.svc.entryOnly();
	svc.shared();
	variables.svc.nope();
}
function shadowed(){
	var svc = getThing();
	svc.pageOnly();
}
}`,
		"h/A.cfc": `component extends="Base" { function init(){ variables.svc = new models.PageService(); return this; } }`,
		"h/B.cfc": `component extends="Base" { function init(){ variables.svc = new models.EntryService(); return this; } }`,

		"h/Partial.cfc": `component { function run(){ variables.svc.pageOnly(); } }`,
		"h/PA.cfc":      `component extends="Partial" { function init(){ variables.svc = new models.PageService(); return this; } }`,
		"h/PB.cfc":      `component extends="Partial" { function other(){} }`,

		"h/Alone.cfc": `component { function run(){ variables.svc.pageOnly(); } }`,

		"h/Top.cfc":   `component { function run(){ variables.svc.pageOnly(); } }`,
		"h/Mid.cfc":   `component extends="Top" { function init(){ variables.svc = new models.PageService(); return this; } }`,
		"h/Leaf1.cfc": `component extends="Mid" { function one(){} }`,
		"h/Leaf2.cfc": `component extends="Mid" { function two(){} }`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "h/Base.cfc"), map[string]string{
		"variables.svc.pageOnly":  "",
		"variables.svc.entryOnly": "",
		"svc.shared":              "",
		"variables.svc.nope":      "method 'nope' not found in models.EntryService|models.PageService",
		"svc.pageOnly":            "variable 'svc' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "h/Partial.cfc"), map[string]string{
		"variables.svc.pageOnly": "variable 'variables.svc' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "h/Alone.cfc"), map[string]string{
		"variables.svc.pageOnly": "variable 'variables.svc' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "h/Top.cfc"), map[string]string{
		"variables.svc.pageOnly": "",
	})
}

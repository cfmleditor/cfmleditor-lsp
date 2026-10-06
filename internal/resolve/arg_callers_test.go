package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func cfmlFilesIn(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		if err == nil && (strings.HasSuffix(p, ".cfc") || strings.HasSuffix(p, ".cfm")) {
			out = append(out, p)
		}

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	return out
}

// TestAnUntypedArgumentHoldsWhatEveryCallerPasses: an argument with no type is
// the component every call of its function passes, as alternatives when they
// differ. A caller that omits it says nothing; one whose receiver or expression
// cannot be typed leaves it untyped. Without the files to search it is off.
func TestAnUntypedArgumentHoldsWhatEveryCallerPasses(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Item.cfc":  `component { function useItM(){} function mixedItem(){} function openM(){} function skipM(){} function inheritedM(){} function forwardedM(){} function conditionalM(){} function siblingM(){} function afterM(){} }`,
		"models/Other.cfc": `component { function mixedOther(){} }`,
		"svc/Base.cfc":     `component { function inherited( required thing ){ arguments.thing.inheritedM(); } }`,
		"svc/Svc.cfc": `component extends="Base" {
function useIt( required thing ){ arguments.thing.useItM(); arguments.thing.nope(); }
function useMixed( required thing ){ arguments.thing.mixedItem(); arguments.thing.mixedOther(); }
function useOpen( required thing ){ arguments.thing.openM(); }
function useSkip( required thing, extra ){ arguments.thing.skipM(); }
function useForwarded( required thing ){ arguments.thing.forwardedM(); }
function useConditional( required thing ){ arguments.thing.conditionalM(); }
function useSibling( required thing ){ arguments.thing.siblingM(); }
function useAfter( required thing ){ arguments.thing.afterM(); }
function callers(){
	var a = new models.Item();
	var forwarded = a;
	useIt( a );
	useForwarded( forwarded );
	if (runtime()) {
		var conditional = a;
	}
	useConditional( conditional );
	if (first()) {
		var sibling = a;
	}
	if (second()) {
		useSibling( sibling );
	}
	useAfter( after );
	var after = a;
	useIt(
		thing = new models.Item()
	);
	useMixed( new models.Item() );
	useMixed( thing = new models.Other() );
	useSkip( a );
	useSkip();
	useOpen( a );
	super.inherited( a );
}
}`,
		"callers/Elsewhere.cfc": `component { function go( x ){ x.useOpen( new models.Other() ); } }`,
	})

	got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "svc/Svc.cfc")

	for _, k := range []string{"arguments.thing.useItM", "arguments.thing.mixedItem", "arguments.thing.mixedOther", "arguments.thing.skipM", "arguments.thing.forwardedM"} {
		if got[k] != "" {
			t.Errorf("%s: %q, want it resolved", k, got[k])
		}
	}

	for _, k := range []string{"arguments.thing.conditionalM", "arguments.thing.siblingM", "arguments.thing.afterM"} {
		if !strings.Contains(got[k], "no component ref") {
			t.Errorf("%s was typed without a reaching straight-line assignment: %q", k, got[k])
		}
	}

	if !strings.Contains(got["arguments.thing.nope"], "method 'nope' not found in") {
		t.Errorf("useIt's argument is not an Item: %q", got["arguments.thing.nope"])
	}

	if !strings.Contains(got["arguments.thing.openM"], "no component ref") {
		t.Errorf("useOpen was typed despite a caller on an untyped receiver: %q", got["arguments.thing.openM"])
	}

	inherited := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "svc/Base.cfc")
	if inherited["arguments.thing.inheritedM"] != "" {
		t.Errorf("a super. call is not a caller: %q", inherited["arguments.thing.inheritedM"])
	}

	off := reasonsWith(t, &Resolver{}, dir, "svc/Svc.cfc")
	if !strings.Contains(off["arguments.thing.useItM"], "no component ref") {
		t.Errorf("inference ran with no files to search: %q", off["arguments.thing.useItM"])
	}
}

func TestCallerAliasesRequireCompleteStraightLineProof(t *testing.T) {
	for _, tc := range []struct {
		name, body, padding string
	}{
		{"unbraced if", "if (runtime())\nvar forwarded = a;\nuseForwarded(forwarded);", ""},
		{"unbraced loop", "while (runtime())\nvar forwarded = a;\nuseForwarded(forwarded);", ""},
		{"unbraced else", "if (runtime()) {} else\nvar forwarded = a;\nuseForwarded(forwarded);", ""},
		{"exhausted sibling paths", "if (first()) {\nvar forwarded = a;\n}\nif (second()) {\nuseForwarded(forwarded);\n}", strings.Repeat("work();\n", 1100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Item.cfc": `component { function ready(){} }`,
				"Svc.cfc": `component {
function useForwarded(thing) { arguments.thing.ready(); }
function callers() {
var a = new Item();
` + tc.body + "\n" + tc.padding + `
}
}`,
			})

			got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "Svc.cfc")
			if reason, exists := got["arguments.thing.ready"]; !exists || !strings.Contains(reason, "no component ref") {
				t.Fatalf("alias resolved without complete straight-line proof: %v", got)
			}
		})
	}
}

func TestCallerAliasesAreNotLimitedByTheRestOfTheFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Item.cfc": `component { function ready(){} }`,
		"Svc.cfc": `component {
function unrelated() {
` + strings.Repeat("work();\n", 1100) + `}
function useForwarded(thing) { arguments.thing.ready(); }
function callers() {
var a = new Item();
var forwarded = a;
useForwarded(forwarded);
}
}`,
	})

	got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "Svc.cfc")
	if reason := got["arguments.thing.ready"]; reason != "" {
		t.Fatalf("a large function elsewhere in the file withheld the alias: %q", reason)
	}
}

// TestACallerPlacedByTheParseIsACaller: Masa's
// `$.getBean( "userManager" ).update( … )` has an untyped variable and a
// receiver the parse already resolved through the getBean resolver, as a
// chained `new Svc()` has none at all. Reading only the variable left such a
// caller unplaced, which withheld the argument for every function sharing
// its name; a caller placed on another component was dismissed although it
// is a call of this one.
func TestACallerPlacedByTheParseIsACaller(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Item.cfc": `component { function ready(){} }`,
		"Svc.cfc":  `component { function use( thing ){ arguments.thing.ready(); } }`,
		"A.cfc":    `component { function run(){ $.getBean( "Svc" ).use( new Item() ); } }`,
		"B.cfc":    `component { function run(){ new Svc().use( new Item() ); } }`,
	})

	getBean := func() []parser.Resolver {
		return []parser.Resolver{{Match: `(?i)(?:^|\.)getBean\(\s*["']([A-Za-z_][\w.]*)["']\s*\)$`, Resolve: "$1", Prefix: "getBean"}}
	}

	got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir), Resolvers: getBean()}, dir, "Svc.cfc")
	expectReasons(t, got, map[string]string{"arguments.thing.ready": ""})

	// The same caller passing something untyped still withholds it.
	writeFiles(t, dir, map[string]string{"C.cfc": `component { function run( x ){ $.getBean( "Svc" ).use( x ); } }`})

	got = reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir), Resolvers: getBean()}, dir, "Svc.cfc")
	if !strings.Contains(got["arguments.thing.ready"], "no component ref") {
		t.Errorf("a placed caller passing an untyped value was ignored: %q", got["arguments.thing.ready"])
	}
}

// TestADollarNamedFunctionsCallersAreFound: Wheels names its internals with a
// leading $, and the caller index read `$build(` as a call to build, so such a
// function's untyped argument was never typed by what its callers pass.
func TestADollarNamedFunctionsCallersAreFound(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Item.cfc": `component { function go(){} }`,
		"Svc.cfc": `component {
function $use( required thing ){ arguments.thing.go(); arguments.thing.nope(); }
function callers(){ $use( new Item() ); }
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "Svc.cfc"), map[string]string{
		"arguments.thing.go":   "",
		"arguments.thing.nope": "method 'nope' not found in Item",
		"$use":                 "",
	})
}

// TestACallOnWhatCannotBeTheComponentIsNotACaller: Mura's settingsDAO.update(bean)
// shares its name with MessageDigest's md.update() and with a plugin's
// pluginCFC.update(), and neither receiver could be typed, so the DAO's
// argument was never inferred. A Java object is never a component, and a
// component built from a computed path ending in a literal file name is that
// file — unless a file of that name extends the declaring component.
func TestACallOnWhatCannotBeTheComponentIsNotACaller(t *testing.T) {
	files := map[string]string{
		"Item.cfc": `component { function go(){} }`,
		"DAO.cfc":  `component { function update( required bean ){ arguments.bean.go(); } }`,
		"Mgr.cfc": `component {
function save(){ var d = new DAO(); d.update( new Item() ); }
function sha(){
	var md = createObject("java", "java.security.MessageDigest").getInstance("SHA-1");
	md.update( 1 );
}
function plugins( dir ){
	var p = createObject("component", "plugins.#dir#.plugin");
	p.update( 1 );
}
}`,
	}

	dir := t.TempDir()
	writeFiles(t, dir, files)
	expectReasons(t, reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "DAO.cfc"), map[string]string{
		"arguments.bean.go": "",
	})

	// A plugin.cfc that extends the DAO could be the receiver.
	files["plugins/x/plugin.cfc"] = `component extends="DAO" {}`

	dir = t.TempDir()
	writeFiles(t, dir, files)

	if got := reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "DAO.cfc")["arguments.bean.go"]; got == "" {
		t.Errorf("typed although a plugin.cfc extending the DAO may be the receiver")
	}
}

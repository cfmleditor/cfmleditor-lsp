package parser

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// TestDelegatesAreRead: WireBox's delegation syntax in both of its forms and
// both syntaxes, spelled as ColdBox's own test harness spells it. Flags are
// written without a value (`inject delegate delegatePrefix`), which both
// parsers used to drop, and a component's `delegates` holds `>` — which
// must not be read as the end of a <cfcomponent> tag.
func TestDelegatesAreRead(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		want []Delegate
	}{
		"script properties": {
			src: `component {
	property name="memory" inject delegate delegatePrefix;
	property name="memory2" inject="Memory" delegate delegatePrefix="mem";
	property name="disk" inject delegate delegateSuffix;
	property name="manager" inject="Worker@hr" delegate delegateExcludes="work";
	property name="workaholic" inject="Worker" delegate="work,rest";
	property name="plain" inject="Plain";
}`,
			want: []Delegate{
				{Target: "memory", Prefix: "memory", Line: 1},
				{Target: "Memory", Prefix: "mem", Line: 2},
				{Target: "disk", Suffix: "disk", Line: 3},
				{Target: "Worker@hr", Excludes: []string{"work"}, Line: 4},
				{Target: "Worker", Includes: []string{"work", "rest"}, Line: 5},
			},
		},
		"script component attribute": {
			src: "component\n\tdelegates=\"Memory, >Memory, ram>memory, <Memory, ram<Memory, Worker=vacation\"\n{\n}",
			want: []Delegate{
				{Target: "Memory"},
				{Target: "Memory", Prefix: "Memory"},
				{Target: "memory", Prefix: "ram"},
				{Target: "Memory", Suffix: "Memory"},
				{Target: "Memory", Suffix: "ram"},
				{Target: "Worker", Includes: []string{"vacation"}},
			},
		},
		"tags": {
			src: "<cfcomponent delegates=\">Memory\">\n" +
				"<cfproperty name=\"disk\" inject delegate hint=\"delegateSuffix\" delegateSuffix>\n" +
				"</cfcomponent>",
			want: []Delegate{
				{Target: "disk", Suffix: "disk", Line: 1},
				{Target: "Memory", Prefix: "Memory"},
			},
		},
		"none past the declaration": {
			src:  "component {\n\tfunction f() { var delegates = \"Memory\"; }\n}",
			want: nil,
		},
	} {
		pr := Parse(uri.URI("file:///Host.cfc"), tc.src)

		if !reflect.DeepEqual(pr.Delegates, tc.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, pr.Delegates, tc.want)
		}
	}
}

// TestADelegateNamesItsMethods: the host's name for a method is
// prefix + method + suffix; an include list is the only methods, an exclude
// list and WireBox's core exclusions never are.
func TestADelegateNamesItsMethods(t *testing.T) {
	d := Delegate{Prefix: "memory", Excludes: []string{"wipe"}}
	for name, want := range map[string]string{
		"memoryRead": "Read", "MEMORYread": "read", "read": "", "memory": "",
		"memoryWipe": "", "memoryInit": "",
	} {
		if got := d.DelegatedMethod(name); got != want {
			t.Errorf("prefix: %s = %q, want %q", name, got, want)
		}
	}

	d = Delegate{Suffix: "Disk", Includes: []string{"read"}}
	for name, want := range map[string]string{"readDisk": "read", "writeDisk": "", "read": ""} {
		if got := d.DelegatedMethod(name); got != want {
			t.Errorf("suffix: %s = %q, want %q", name, got, want)
		}
	}
}

// TestABareInjectIsTheModelNamedByTheProperty: WireBox's default DSL is
// `model`, so `property name="memory" inject;` is the model memory.
func TestABareInjectIsTheModelNamedByTheProperty(t *testing.T) {
	for _, src := range []string{
		"component {\n\tproperty name=\"memory\" inject;\n}",
		"component {\n\tproperty memory inject;\n}",
		"<cfcomponent>\n<cfproperty name=\"memory\" inject>\n</cfcomponent>",
	} {
		pr := Parse(uri.URI("file:///Host.cfc"), src)
		if ref := refNamed(pr.ComponentRefs, "memory"); ref == nil || ref.Component != "memory" {
			t.Errorf("%q: ref %+v, want memory", src, ref)
		}
	}
}

// TestAnORMRelationshipGeneratesItsMethods: CFML's ORM adds has, add and
// remove methods for a relationship property, which ContentBox's entities
// call throughout — 124 corpus calls were reported as not found.
func TestAnORMRelationshipGeneratesItsMethods(t *testing.T) {
	src := `component persistent="true" {
	property name="categories" fieldtype="many-to-many" cfc="Category" singularName="category";
	property name="comments" fieldtype="one-to-many" cfc="Comment";
	property name="site" fieldtype="many-to-one" cfc="Site";
	property name="title";
	function hasSite() { return true; }
}`
	pr := Parse(uri.URI("file:///Entry.cfc"), src)

	var names []string
	for i := range pr.Funcs {
		names = append(names, strings.ToLower(pr.Funcs[i].Name))
	}

	for _, want := range []string{
		"addcategory", "removecategory", "hascategories", "hascategory",
		"addcomments", "removecomments", "hascomments", "hassite",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("no %s in %v", want, names)
		}
	}

	for _, unwanted := range []string{"hastitle", "addtitle"} {
		if slices.Contains(names, unwanted) {
			t.Errorf("%s generated for a plain property", unwanted)
		}
	}

	if n := len(slices.DeleteFunc(slices.Clone(names), func(s string) bool { return s != "hassite" })); n != 1 {
		t.Errorf("hasSite declared %d times, want the explicit one only", n)
	}
}

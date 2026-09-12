package daemon

import (
	"reflect"
	"testing"
)

// fullConfig sets every key SettingsFrom reads, each to a value that is not the
// zero value of the field it lands in, so a field SettingsFrom forgets to fill
// shows up as zero below.
const fullConfig = `{
	"workspaceName": "w",
	"workspacePaths": ["."],
	"workspaceIndexGlobs": ["**/*.cfc"],
	"mappings": {"models": "./models"},
	"expressionMappings": {"#CORE#": "packages.core."},
	"servicePropertyResolvers": {"package": "packages.${name}"},
	"componentResolvers": [{"match": "getService(\"$1\")", "resolve": "svc.$1", "prefix": "getService"}],
	"propertyResolvers": [{"match": "$1", "resolve": "beans.$1", "attribute": "name"}],
	"beanPaths": {"svc": "./services"},
	"formatting": {"enabled": true},
	"linting": {"enabled": true},
	"references": {"enabled": true},
	"completions": {"tagSnippets": true, "functionSnippets": true, "globalFunctionResolution": true}
}`

// TestSettingsFromFillsEveryField is the recurrence guard for the defect this
// function was extracted to fix.
//
// A key that config parses but SettingsFrom never copies reaches no daemon
// session at all, and nothing reports it: a missing setting looks exactly like
// a setting the user did not ask for. `completions` was missing for that
// reason, and the construction sat in package main where no test could see it.
//
// Comparing every field reflectively rather than asserting a hand-written list
// means a field added to server.Settings and forgotten here fails, which is the
// mistake worth catching. server.Settings.Apply has the matching test on the
// next hop.
func TestSettingsFromFillsEveryField(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, fullConfig)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	set := reflect.ValueOf(SettingsFrom(cfg))

	for i := range set.NumField() {
		if set.Field(i).IsZero() {
			t.Errorf("server.Settings.%s is not filled by SettingsFrom", set.Type().Field(i).Name)
		}
	}
}

// TestSettingsFromAppliesCompletionDefaults covers the shape the bug actually
// took. The `completions` block is optional and all three of its settings
// default to on, but their zero value is off — so a config that simply does not
// mention completions has to come out of here with them on.
//
// componentResolvers is set deliberately: it is what made the defect visible
// only for some configs. handleInitialize loads the workspace config itself
// when a session arrives with no resolvers, which quietly repaired these three
// — so adding a single unrelated componentResolver to a working .cfmleditor.json
// was enough to turn off global function resolution for the whole workspace.
func TestSettingsFromAppliesCompletionDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{
		"workspaceName": "w",
		"componentResolvers": [{"match": "getService(\"$1\")", "resolve": "svc.$1", "prefix": "getService"}]
	}`)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	set := SettingsFrom(cfg)
	if !set.TagSnippets || !set.FunctionSnippets || !set.GlobalFunctionResolution {
		t.Errorf("completion defaults lost: tagSnippets=%v functionSnippets=%v globalFunctionResolution=%v",
			set.TagSnippets, set.FunctionSnippets, set.GlobalFunctionResolution)
	}
}

// TestSettingsFromKeepsCompletionsOff is the other direction: the defaults must
// not overwrite a block that deliberately turns something off.
func TestSettingsFromKeepsCompletionsOff(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{
		"workspaceName": "w",
		"completions": {"tagSnippets": true, "functionSnippets": true, "globalFunctionResolution": false}
	}`)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	set := SettingsFrom(cfg)
	if set.GlobalFunctionResolution {
		t.Error("globalFunctionResolution:false was overridden by the default")
	}

	if !set.TagSnippets || !set.FunctionSnippets {
		t.Error("the other two settings in the same block were lost")
	}
}

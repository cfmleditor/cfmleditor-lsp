package config

import (
	"reflect"
	"sort"
	"testing"
)

// setEveryFeature sets every *bool field of Features to v and returns the field
// names it set, so the test enumerates the struct rather than a hand-written
// list that a new switch would quietly fall out of.
func setEveryFeature(f *Features, v bool) []string {
	rv := reflect.ValueOf(f).Elem()
	rt := rv.Type()

	var names []string

	for i := range rt.NumField() {
		if rt.Field(i).Type != reflect.TypeFor[*bool]() {
			continue
		}

		p := new(bool)
		*p = v
		rv.Field(i).Set(reflect.ValueOf(p))
		names = append(names, rt.Field(i).Name)
	}

	sort.Strings(names)

	return names
}

// TestFeatureKeysReachTheServer walks the whole config chain for every switch
// in the block, in both positions.
//
// The switches are a hand-maintained list repeated four times over — the JSON
// struct, mergeFeatures, ResolveFeatures and ResolvedFeatures — and a key
// missing from any one of them fails nothing at runtime. It just stops
// working, which looks exactly like the user not having set it.
func TestFeatureKeysReachTheServer(t *testing.T) {
	for _, v := range []bool{true, false} {
		f := &Features{}

		names := setEveryFeature(f, v)
		if len(names) == 0 {
			t.Fatal("Features has no *bool fields; this test is checking nothing")
		}

		resolved := Resolve(&JSON{Features: f}, t.TempDir()).Features
		rv := reflect.ValueOf(resolved)

		for _, name := range names {
			got := rv.FieldByName(name)
			if !got.IsValid() {
				t.Errorf("ResolvedFeatures has no %s field", name)

				continue
			}

			if got.Interface() != v {
				t.Errorf("Resolve dropped features.%s: got %v, want %v", name, got.Interface(), v)
			}
		}

		// And the reverse direction: nothing in ResolvedFeatures that the JSON
		// block cannot set, which would be a switch with no way to reach it.
		var resolvedNames []string
		for field := range rv.Type().Fields() {
			resolvedNames = append(resolvedNames, field.Name)
		}

		sort.Strings(resolvedNames)

		if !reflect.DeepEqual(names, resolvedNames) {
			t.Errorf("Features and ResolvedFeatures disagree: %v vs %v", names, resolvedNames)
		}
	}
}

// featureDefaults is the expected default for every switch, by field name.
//
// Enumerated rather than assumed, and checked for completeness below, so that
// adding a switch to Features fails here until its default is stated — the
// point of walking the struct reflectively in the first place. Folding is the
// one that is off: see the Features doc comment.
var featureDefaults = map[string]bool{
	"DocumentHighlight":          true,
	"Folding":                    foldingDefault,
	"WatchedFiles":               true,
	"RangeFormatting":            true,
	"VariableDefinitions":        true,
	"Routes":                     true,
	"OutputContextInterpolation": true,
}

// TestFeaturesDefaultToOn is the property that makes these opt-outs. An absent
// block, an empty block, and a nil config must all leave every feature at its
// default; taking the zero value instead would switch them all off on any path
// that skipped ResolveFeatures.
func TestFeaturesDefaultToOn(t *testing.T) {
	if got := reflect.TypeFor[ResolvedFeatures]().NumField(); got != len(featureDefaults) {
		t.Fatalf("ResolvedFeatures has %d fields but %d defaults are stated; add the new switch to featureDefaults", got, len(featureDefaults))
	}

	check := func(t *testing.T, what string, rv reflect.Value) {
		t.Helper()

		for i := range rv.Type().NumField() {
			name := rv.Type().Field(i).Name

			want, stated := featureDefaults[name]
			if !stated {
				t.Errorf("%s has no stated default", name)

				continue
			}

			if got := rv.Field(i).Interface().(bool); got != want { //nolint:forcetypeassert // every field is a bool, asserted above
				t.Errorf("%s: %s defaults to %v, want %v", what, name, got, want)
			}
		}
	}

	cases := map[string]*Features{
		"nil block":   nil,
		"empty block": {},
	}

	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			check(t, name, reflect.ValueOf(ResolveFeatures(f)))
		})
	}

	// Through Resolve as well, since that is the hop a session actually takes.
	check(t, "Resolve", reflect.ValueOf(Resolve(&JSON{}, t.TempDir()).Features))
}

// TestFoldingIsOptIn states folding's default on its own, so that flipping it
// back is a deliberate edit to a test that says what it is for rather than a
// number changing inside a table.
func TestFoldingIsOptIn(t *testing.T) {
	if Resolve(&JSON{}, t.TempDir()).Features.Folding {
		t.Error("folding is on for a config that does not mention it; it is opt-in")
	}

	on := true
	if !Resolve(&JSON{Features: &Features{Folding: &on}}, t.TempDir()).Features.Folding {
		t.Error("folding stayed off despite being asked for")
	}
}

// TestSettingOneFeatureLeavesTheOthersOn is why the fields are pointers. As
// plain bools, naming one switch would read the rest back as false and turn
// them all off — the defect the completions block was fixed for.
func TestSettingOneFeatureLeavesTheOthersOn(t *testing.T) {
	// documentHighlight rather than folding: folding is off by default, so
	// setting it to false would assert nothing.
	off := false
	resolved := Resolve(&JSON{Features: &Features{DocumentHighlight: &off}}, t.TempDir()).Features

	if resolved.DocumentHighlight {
		t.Error("documentHighlight stayed on despite being set to false")
	}

	if !resolved.WatchedFiles || !resolved.RangeFormatting {
		t.Errorf("setting documentHighlight switched off its siblings: %+v", resolved)
	}
}

// TestMergeFeaturesKeepsBothSides: an editor's initializationOptions and a
// project's .cfmleditor.json each set one switch, and both must survive. The
// block-replacing spelling loses whichever side is the base.
func TestMergeFeaturesKeepsBothSides(t *testing.T) {
	off := false

	editor := &JSON{Features: &Features{DocumentHighlight: &off}}
	file := &JSON{Features: &Features{WatchedFiles: &off}}

	resolved := Resolve(Merge(editor, file), t.TempDir()).Features

	if resolved.DocumentHighlight {
		t.Error("the editor's documentHighlight=false was lost when merging with a config file")
	}

	if resolved.WatchedFiles {
		t.Error("the file's watchedFiles=false was lost")
	}

	if !resolved.RangeFormatting {
		t.Errorf("merging switched off a feature neither side named: %+v", resolved)
	}

	// The same merge, with folding named on one side, since an opt-in switch
	// has to survive a merge just as an opt-out does.
	on := true
	both := Resolve(Merge(&JSON{Features: &Features{Folding: &on}}, file), t.TempDir()).Features

	if !both.Folding {
		t.Error("the editor's folding=true was lost when merging with a config file")
	}

	if both.WatchedFiles {
		t.Error("the file's watchedFiles=false was lost when the editor asked for folding")
	}
}

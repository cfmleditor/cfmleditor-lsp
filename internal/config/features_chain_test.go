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
		if rt.Field(i).Type != reflect.TypeOf((*bool)(nil)) {
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
		for i := range rv.Type().NumField() {
			resolvedNames = append(resolvedNames, rv.Type().Field(i).Name)
		}

		sort.Strings(resolvedNames)

		if !reflect.DeepEqual(names, resolvedNames) {
			t.Errorf("Features and ResolvedFeatures disagree: %v vs %v", names, resolvedNames)
		}
	}
}

// TestFeaturesDefaultToOn is the property that makes these opt-outs. An absent
// block, an empty block, and a nil config must all leave every feature on;
// taking the zero value instead would switch them all off on any path that
// skipped ResolveFeatures.
func TestFeaturesDefaultToOn(t *testing.T) {
	cases := map[string]*Features{
		"nil block":   nil,
		"empty block": {},
	}

	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			rv := reflect.ValueOf(ResolveFeatures(f))
			for i := range rv.Type().NumField() {
				if rv.Field(i).Interface() != true {
					t.Errorf("%s defaults to off, want on", rv.Type().Field(i).Name)
				}
			}
		})
	}

	// Through Resolve as well, since that is the hop a session actually takes.
	rv := reflect.ValueOf(Resolve(&JSON{}, t.TempDir()).Features)
	for i := range rv.Type().NumField() {
		if rv.Field(i).Interface() != true {
			t.Errorf("Resolve leaves %s off for a config with no features block", rv.Type().Field(i).Name)
		}
	}
}

// TestSettingOneFeatureLeavesTheOthersOn is why the fields are pointers. As
// plain bools, naming one switch would read the rest back as false and turn
// them all off — the defect the completions block was fixed for.
func TestSettingOneFeatureLeavesTheOthersOn(t *testing.T) {
	off := false
	resolved := Resolve(&JSON{Features: &Features{Folding: &off}}, t.TempDir()).Features

	if resolved.Folding {
		t.Error("folding stayed on despite being set to false")
	}

	if !resolved.DocumentHighlight || !resolved.WatchedFiles || !resolved.RangeFormatting {
		t.Errorf("setting folding switched off its siblings: %+v", resolved)
	}
}

// TestMergeFeaturesKeepsBothSides: an editor's initializationOptions and a
// project's .cfmleditor.json each set one switch, and both must survive. The
// block-replacing spelling loses whichever side is the base.
func TestMergeFeaturesKeepsBothSides(t *testing.T) {
	off := false

	editor := &JSON{Features: &Features{Folding: &off}}
	file := &JSON{Features: &Features{WatchedFiles: &off}}

	resolved := Resolve(Merge(editor, file), t.TempDir()).Features

	if resolved.Folding {
		t.Error("the editor's folding=false was lost when merging with a config file")
	}

	if resolved.WatchedFiles {
		t.Error("the file's watchedFiles=false was lost")
	}

	if !resolved.DocumentHighlight || !resolved.RangeFormatting {
		t.Errorf("merging switched off a feature neither side named: %+v", resolved)
	}
}

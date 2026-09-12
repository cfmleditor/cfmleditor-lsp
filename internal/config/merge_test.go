package config

import (
	"reflect"
	"testing"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestMergeNilSides(t *testing.T) {
	if got := Merge(nil, nil); got != nil {
		t.Errorf("Merge(nil, nil) = %v, want nil", got)
	}

	base := &JSON{JavaStubsPath: "stubs"}
	if got := Merge(base, nil); got != base {
		t.Error("Merge(base, nil) should return base unchanged")
	}

	over := &JSON{JavaStubsPath: "other"}
	if got := Merge(nil, over); got != over {
		t.Error("Merge(nil, over) should return over unchanged")
	}
}

func TestMergeOverrideWinsPerKey(t *testing.T) {
	base := &JSON{
		JavaStubsPath: "base.stubs",
		Linting:       &Linting{Enabled: true},
		Completions:   &Completions{TagSnippets: true},
	}
	over := &JSON{
		JavaStubsPath: "over.stubs",
		Linting:       &Linting{Enabled: false},
	}

	got := Merge(base, over)

	if got.JavaStubsPath != "over.stubs" {
		t.Errorf("JavaStubsPath = %q, want over's value", got.JavaStubsPath)
	}

	if got.Linting == nil || got.Linting.Enabled {
		t.Error("over's linting block should win, including when it disables linting")
	}

	// Completions is untouched by over, so base's must survive.
	if got.Completions == nil || !got.Completions.TagSnippets {
		t.Error("base's completions should survive when over does not set them")
	}
}

func TestMergeUnsetFieldsFallThrough(t *testing.T) {
	base := &JSON{
		Linting:       &Linting{Enabled: true},
		JavaStubsPath: "base.stubs",
		Formatting:    &Formatting{Enabled: boolPtr(true), SelfCloseTags: boolPtr(false)},
	}

	got := Merge(base, &JSON{})

	if got.Linting == nil || !got.Linting.Enabled {
		t.Error("an empty override must not clear base's linting")
	}

	if got.JavaStubsPath != "base.stubs" {
		t.Error("an empty override must not clear base's javaStubsPath")
	}

	if got.Formatting == nil || !BoolDefault(got.Formatting.Enabled, false) {
		t.Error("an empty override must not clear base's formatting")
	}
}

func TestMergeMapsUnionWithOverrideWinning(t *testing.T) {
	base := &JSON{Mappings: map[string]string{"a": "/base/a", "shared": "/base/shared"}}
	over := &JSON{Mappings: map[string]string{"b": "/over/b", "shared": "/over/shared"}}

	got := Merge(base, over)

	want := map[string]string{"a": "/base/a", "b": "/over/b", "shared": "/over/shared"}
	if len(got.Mappings) != len(want) {
		t.Fatalf("Mappings = %v, want %v", got.Mappings, want)
	}

	for k, v := range want {
		if got.Mappings[k] != v {
			t.Errorf("Mappings[%q] = %q, want %q", k, got.Mappings[k], v)
		}
	}
}

func TestMergeDoesNotMutateInputs(t *testing.T) {
	base := &JSON{Mappings: map[string]string{"a": "/base/a"}}
	over := &JSON{Mappings: map[string]string{"a": "/over/a"}}

	_ = Merge(base, over)

	if base.Mappings["a"] != "/base/a" {
		t.Error("Merge mutated base")
	}

	if over.Mappings["a"] != "/over/a" {
		t.Error("Merge mutated over")
	}
}

func TestMergeKeepsResolversFromBothWithOverrideFirst(t *testing.T) {
	base := &JSON{ComponentResolvers: []Resolver{{Match: "base()", Resolve: "b", Prefix: "base"}}}
	over := &JSON{ComponentResolvers: []Resolver{{Match: "over()", Resolve: "o", Prefix: "over"}}}

	got := Merge(base, over)

	if len(got.ComponentResolvers) != 2 {
		t.Fatalf("got %d resolvers, want both sides kept", len(got.ComponentResolvers))
	}

	// Resolvers are tried in order and the first match wins, so the
	// higher-priority side has to lead for its entries to take effect.
	if got.ComponentResolvers[0].Match != "over()" {
		t.Errorf("resolver order = %v, want the override's entry first", got.ComponentResolvers)
	}
}

// TestMergeFormattingCoversEveryField is the recurrence guard. mergeFormatting
// names each field explicitly, so a field added to Formatting and forgotten
// there would silently stop being mergeable — an editor's value for it would be
// discarded whenever the config file had a formatting block at all, with
// nothing to report it.
//
// Every field is set on over to a value that differs from base's, so a
// forgotten one shows up as base's value surviving.
func TestMergeFormattingCoversEveryField(t *testing.T) {
	base := &Formatting{}
	over := &Formatting{}

	baseVal, overVal := reflect.ValueOf(base).Elem(), reflect.ValueOf(over).Elem()

	for i := range overVal.NumField() {
		b, o := baseVal.Field(i), overVal.Field(i)

		switch o.Type() {
		case reflect.TypeOf((*bool)(nil)):
			b.Set(reflect.ValueOf(boolPtr(false)))
			o.Set(reflect.ValueOf(boolPtr(true)))
		case reflect.TypeOf((*int)(nil)):
			one, two := 1, 2
			b.Set(reflect.ValueOf(&one))
			o.Set(reflect.ValueOf(&two))
		case reflect.TypeOf(""):
			b.SetString("base")
			o.SetString("over")
		default:
			t.Fatalf("Formatting.%s has type %s, which mergeFormatting's field lists do not cover",
				overVal.Type().Field(i).Name, o.Type())
		}
	}

	got := reflect.ValueOf(mergeFormatting(base, over)).Elem()

	for i := range got.NumField() {
		name := got.Type().Field(i).Name

		g, o := got.Field(i), overVal.Field(i)
		if g.Kind() == reflect.Pointer {
			if g.IsNil() || !reflect.DeepEqual(g.Elem().Interface(), o.Elem().Interface()) {
				t.Errorf("Formatting.%s did not take over's value — mergeFormatting is missing it", name)
			}

			continue
		}

		if g.Interface() != o.Interface() {
			t.Errorf("Formatting.%s did not take over's value — mergeFormatting is missing it", name)
		}
	}
}

// TestMergeFormattingKeepsUnstatedFields is the half that matters for an editor
// sending a full settings payload: a config file naming one key must leave the
// rest of the editor's block alone.
func TestMergeFormattingKeepsUnstatedFields(t *testing.T) {
	editor := &Formatting{Enabled: boolPtr(true), AttrBreakThreshold: intPtr(7), ScopeCase: "upper"}
	file := &Formatting{LineWidth: intPtr(120)}

	got := mergeFormatting(editor, file)

	if !BoolDefault(got.Enabled, false) {
		t.Error("the file's lineWidth switched the editor's formatting off")
	}

	if IntDefault(got.AttrBreakThreshold, 0) != 7 {
		t.Error("the editor's attrBreakThreshold was discarded")
	}

	if got.ScopeCase != "upper" {
		t.Error("the editor's scopeCase was discarded")
	}

	if IntDefault(got.LineWidth, 0) != 120 {
		t.Error("the file's lineWidth did not win")
	}
}

// TestMergeUsesFormattingFieldMerge checks the wiring, not the helper: Merge
// has to route the formatting block through mergeFormatting rather than
// replacing it. The two tests above exercise the helper directly and so cannot
// see the call site going back to a wholesale swap.
func TestMergeUsesFormattingFieldMerge(t *testing.T) {
	editor := &JSON{Formatting: &Formatting{Enabled: boolPtr(true), AttrBreakThreshold: intPtr(7)}}
	file := &JSON{Formatting: &Formatting{LineWidth: intPtr(120)}}

	got := Merge(editor, file)

	if got.Formatting == nil {
		t.Fatal("Merge dropped the formatting block")
	}

	if IntDefault(got.Formatting.AttrBreakThreshold, 0) != 7 {
		t.Error("Merge replaced the whole formatting block instead of merging it per key")
	}

	if IntDefault(got.Formatting.LineWidth, 0) != 120 {
		t.Error("the file's lineWidth did not win")
	}
}

package config

import "testing"

func boolPtr(b bool) *bool { return &b }

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
		Formatting:    &Formatting{Enabled: true, SelfCloseTags: boolPtr(false)},
	}

	got := Merge(base, &JSON{})

	if got.Linting == nil || !got.Linting.Enabled {
		t.Error("an empty override must not clear base's linting")
	}

	if got.JavaStubsPath != "base.stubs" {
		t.Error("an empty override must not clear base's javaStubsPath")
	}

	if got.Formatting == nil || !got.Formatting.Enabled {
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

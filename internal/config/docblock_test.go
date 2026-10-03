package config

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestResolveDocBlockDefaults(t *testing.T) {
	// An absent block, and an empty one, mean a gap and no extra tags: the
	// extension's defaults, which are not what the zero value says.
	for name, d := range map[string]*DocBlock{"absent": nil, "empty": {}} {
		if r := ResolveDocBlock(d); !r.Gap || len(r.Extra) != 0 {
			t.Errorf("%s: %+v", name, r)
		}
	}

	if r := ResolveDocBlock(&DocBlock{Gap: new(false)}); r.Gap {
		t.Error("an explicit gap:false was not respected")
	}

	if got := Resolve(&JSON{DocBlock: &DocBlock{Extra: []DocBlockExtra{{Name: "author"}}}}, "/proj").DocBlock; !got.Gap || len(got.Extra) != 1 {
		t.Errorf("Resolve did not carry docBlock: %+v", got)
	}
}

func TestDocBlockReadsItsKeys(t *testing.T) {
	var j JSON

	err := json.Unmarshal([]byte(`{"docBlock": {"gap": false, "extra": [{"name": "since", "default": "1.0", "types": ["component"]}]}}`), &j)
	if err != nil {
		t.Fatal(err)
	}

	got := ResolveDocBlock(j.DocBlock)
	if got.Gap || len(got.Extra) != 1 || got.Extra[0].Name != "since" || got.Extra[0].Default != "1.0" || !slices.Equal(got.Extra[0].Types, []string{"component"}) {
		t.Errorf("%+v", got)
	}
}

// TestMergeDocBlockUnionsByNameAndKey: a child naming one extra tag must not
// drop its parent's others, nor a child naming only extras reset the gap.
func TestMergeDocBlockUnionsByNameAndKey(t *testing.T) {
	base := &JSON{DocBlock: &DocBlock{Gap: new(false), Extra: []DocBlockExtra{{Name: "author", Default: "a"}, {Name: "since", Default: "1"}}}}
	over := &JSON{DocBlock: &DocBlock{Extra: []DocBlockExtra{{Name: "since", Default: "2"}, {Name: "see"}}}}

	got := ResolveDocBlock(Merge(base, over).DocBlock)
	if got.Gap {
		t.Error("the child's silence on gap reset it")
	}

	var names []string

	for _, e := range got.Extra {
		names = append(names, e.Name+"="+e.Default)
	}

	if want := []string{"author=a", "since=2", "see="}; !slices.Equal(names, want) {
		t.Errorf("extras %v, want %v", names, want)
	}
}

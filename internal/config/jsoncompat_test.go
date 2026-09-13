package config

import (
	stdjson "encoding/json"
	"testing"

	v2 "github.com/go-json-experiment/json"
)

// Config files are decoded with the standard library on purpose, while the LSP
// wire and every request handler use encoding/json/v2. That looks like an
// inconsistency worth tidying up and is not one: v2 changes two behaviours that
// a hand-written .cfmleditor.json depends on.
//
// The first is silent, which is what makes it dangerous. stdlib matches field
// names case-insensitively; v2 does not, so a key a user spelled with the wrong
// case is dropped without an error and the setting simply never takes effect.
// The second is loud but breaking: v2 rejects a duplicate object member where
// stdlib takes the last one, so a config that works today would stop loading.
//
// Neither applies to the LSP wire, where the client emits spec-conformant JSON
// with exact casing — which is why that side moved and this side has not. If
// this is ever revisited, it needs json.MatchCaseInsensitiveNames(true) and a
// deliberate decision about duplicates, not a plain import swap.
func TestConfigDecodingStaysOnTheStandardLibrary(t *testing.T) {
	t.Run("case-insensitive keys are honoured", func(t *testing.T) {
		const src = `{"WorkSpaceName": "myproject"}`

		var std JSON
		if err := stdjson.Unmarshal([]byte(src), &std); err != nil {
			t.Fatalf("the standard library rejected a mis-cased key: %v", err)
		}

		if std.WorkspaceName != "myproject" {
			t.Fatalf("workspaceName = %q, want myproject — the behaviour config relies on", std.WorkspaceName)
		}

		// And the reason it cannot simply be swapped: v2 drops it in silence.
		var alt JSON
		if err := v2.Unmarshal([]byte(src), &alt); err != nil {
			t.Logf("v2 rejected it outright: %v", err)
		} else if alt.WorkspaceName == "myproject" {
			t.Error("v2 now matches field names case-insensitively; the reason for this split may be gone")
		}
	})

	t.Run("a duplicated key does not break loading", func(t *testing.T) {
		const src = `{"workspaceName": "first", "workspaceName": "second"}`

		var std JSON
		if err := stdjson.Unmarshal([]byte(src), &std); err != nil {
			t.Fatalf("the standard library rejected a duplicated key: %v", err)
		}

		if std.WorkspaceName != "second" {
			t.Errorf("workspaceName = %q, want second (last wins)", std.WorkspaceName)
		}

		var alt JSON
		if err := v2.Unmarshal([]byte(src), &alt); err == nil {
			t.Error("v2 now accepts duplicate members; the reason for this split may be gone")
		}
	})
}

package daemon

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

// TestResolvedFormattingReadsEveryStringKey covers the daemon's own copy of the
// formatting field list. Config.ResolvedFormatting() calls one accessor per
// key, restating names that internal/config already lists twice, so a key added
// to the schema and to the formatter can still arrive empty for every daemon
// session — the mode most projects run in, since it is what a `.cfmleditor.json`
// selects.
//
// The config file is built from the schema's own json tags, so a new key is
// covered the moment it is declared rather than when someone remembers to add
// it here.
func TestResolvedFormattingReadsEveryStringKey(t *testing.T) {
	fv := reflect.ValueOf(&config.Formatting{}).Elem()

	formatting := map[string]string{}
	names := map[string]string{}

	for i := range fv.NumField() {
		if fv.Field(i).Kind() != reflect.String {
			continue
		}

		field := fv.Type().Field(i)

		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("config.Formatting.%s has no json tag", field.Name)
		}

		formatting[tag] = "value-" + field.Name
		names[tag] = field.Name
	}

	if len(names) == 0 {
		t.Fatal("config.Formatting has no string fields; this test is checking nothing")
	}

	body, err := json.Marshal(map[string]any{"workspaceName": "p", "formatting": formatting})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeConfig(t, dir, string(body))

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	rv := reflect.ValueOf(cfg.ResolvedFormatting())

	for tag, name := range names {
		got := rv.FieldByName(name)
		if !got.IsValid() {
			t.Errorf("ResolvedFormatting has no %s field", name)

			continue
		}

		if want := "value-" + name; got.String() != want {
			t.Errorf("Config.ResolvedFormatting() dropped %q: got %q, want %q", tag, got.String(), want)
		}
	}
}

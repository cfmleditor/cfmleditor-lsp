package daemon

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

// TestResolvedFormattingReadsEveryKey covers the daemon's own copy of the
// formatting field list. Config.ResolvedFormatting() calls one accessor per
// key, restating names that internal/config already lists twice, so a key added
// to the schema and to the formatter can still arrive as its default for every
// daemon session — the mode most projects run in, since a `.cfmleditor.json` is
// what selects it.
//
// The config file is built from the schema's own json tags, so a new key is
// covered the moment it is declared rather than when someone remembers to add
// it here. Bools are written both ways for the same reason as in
// internal/config: a dropped one returns its default, which only one of the two
// runs contradicts.
func TestResolvedFormattingReadsEveryKey(t *testing.T) {
	for _, boolVal := range []bool{true, false} {
		fv := reflect.ValueOf(&config.Formatting{}).Elem()

		formatting := map[string]any{}
		want := map[string]any{}

		n := 0

		for i := range fv.NumField() {
			field := fv.Type().Field(i)

			tag := field.Tag.Get("json")
			if tag == "" || tag == "-" {
				t.Fatalf("config.Formatting.%s has no json tag", field.Name)
			}

			switch fv.Field(i).Type() {
			case reflect.TypeOf(""):
				formatting[tag] = "value-" + field.Name
			case reflect.TypeOf((*bool)(nil)):
				formatting[tag] = boolVal
			case reflect.TypeOf((*int)(nil)):
				n += 7
				formatting[tag] = n
			default:
				continue
			}

			want[field.Name] = formatting[tag]
		}

		if len(want) == 0 {
			t.Fatal("config.Formatting has no fields of a kind this test sets; it is checking nothing")
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

		for name, expected := range want {
			got := rv.FieldByName(name)
			if !got.IsValid() {
				t.Errorf("ResolvedFormatting has no %s field", name)

				continue
			}

			if got.Interface() != expected {
				t.Errorf("bools=%v: Config.ResolvedFormatting() dropped %s: got %v, want %v",
					boolVal, name, got.Interface(), expected)
			}
		}
	}
}

package config

import (
	"reflect"
	"testing"
)

// TestFormattingStringKeysReachTheFormatter walks the whole chain a formatting
// string key travels — `.cfmleditor.json` → Formatting → Resolve →
// ResolvedFormatting → FormatterOptions → formatter.Options — and checks the
// value arrives.
//
// Each of those hops is a hand-written list of field copies, and a new key has
// to be added to all of them. mergeFormatting has a reflective test that fails
// when a field is missed there; these two hops had nothing, so a key wired into
// the schema and the formatter but forgotten in between would resolve to the
// empty string and silently do nothing — which reads exactly like the setting
// not working.
func TestFormattingStringKeysReachTheFormatter(t *testing.T) {
	f := &Formatting{}
	fv := reflect.ValueOf(f).Elem()

	var names []string

	for i := range fv.NumField() {
		if fv.Field(i).Kind() != reflect.String {
			continue
		}

		name := fv.Type().Field(i).Name
		fv.Field(i).SetString("value-" + name)
		names = append(names, name)
	}

	if len(names) == 0 {
		t.Fatal("Formatting has no string fields; this test is checking nothing")
	}

	rv := reflect.ValueOf(Resolve(&JSON{Formatting: f}, t.TempDir()).Formatting)
	ov := reflect.ValueOf(Resolve(&JSON{Formatting: f}, t.TempDir()).Formatting.FormatterOptions())

	for _, name := range names {
		want := "value-" + name

		r := rv.FieldByName(name)
		if !r.IsValid() {
			t.Errorf("ResolvedFormatting has no %s field", name)

			continue
		}

		if r.String() != want {
			t.Errorf("Resolve dropped Formatting.%s: got %q, want %q", name, r.String(), want)
		}

		o := ov.FieldByName(name)
		if !o.IsValid() {
			t.Errorf("formatter.Options has no %s field", name)

			continue
		}

		if o.String() != want {
			t.Errorf("FormatterOptions dropped %s: got %q, want %q", name, o.String(), want)
		}
	}
}

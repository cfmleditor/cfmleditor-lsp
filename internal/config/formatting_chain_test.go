package config

import (
	"reflect"
	"sort"
	"testing"
)

// notFormatterOptions are the two formatting keys that deliberately have no
// counterpart on formatter.Options: they gate whether the server formats and
// logs at all, rather than describing how. Naming them here rather than
// skipping anything absent means a new key that fails to reach the formatter
// fails this test instead of passing quietly.
var notFormatterOptions = []string{"Debug", "Enabled"}

// setFormatting fills every field of a Formatting with a value derived from its
// name, and returns what each should resolve to. Bools take the value given,
// since a *bool's own name says nothing about which way it should go.
func setFormatting(f *Formatting, boolVal bool) map[string]any {
	fv := reflect.ValueOf(f).Elem()
	want := map[string]any{}

	n := 0

	for i := range fv.NumField() {
		name := fv.Type().Field(i).Name
		field := fv.Field(i)

		switch field.Type() {
		case reflect.TypeOf(""):
			field.SetString("value-" + name)
			want[name] = "value-" + name
		case reflect.TypeOf((*bool)(nil)):
			v := boolVal

			field.Set(reflect.ValueOf(&v))

			want[name] = boolVal
		case reflect.TypeOf((*int)(nil)):
			n += 7
			v := n

			field.Set(reflect.ValueOf(&v))

			want[name] = n
		}
	}

	return want
}

// TestFormattingKeysReachTheFormatter walks the whole chain a formatting key
// travels — `.cfmleditor.json` → Formatting → Resolve → ResolvedFormatting →
// FormatterOptions → formatter.Options — and checks the value arrives.
//
// Each of those hops is a hand-written list of field copies, and a new key has
// to be added to all of them. mergeFormatting has a reflective test that fails
// when a field is missed there; these two hops had nothing, so a key wired into
// the schema and the formatter but forgotten in between resolved to its default
// and silently did nothing — which reads exactly like the setting not working.
//
// Bools are checked with every field set both ways, because Resolve applies a
// per-field default: a dropped field returns that default, which one of the two
// runs always contradicts. One run could not tell the two apart.
func TestFormattingKeysReachTheFormatter(t *testing.T) {
	for _, boolVal := range []bool{true, false} {
		f := &Formatting{}
		want := setFormatting(f, boolVal)

		if len(want) == 0 {
			t.Fatal("Formatting has no fields of a kind this test sets; it is checking nothing")
		}

		resolved := Resolve(&JSON{Formatting: f}, t.TempDir()).Formatting
		rv := reflect.ValueOf(resolved)
		ov := reflect.ValueOf(resolved.FormatterOptions())

		var missingFromOptions []string

		for name, expected := range want {
			r := rv.FieldByName(name)
			if !r.IsValid() {
				t.Errorf("ResolvedFormatting has no %s field", name)

				continue
			}

			if r.Interface() != expected {
				t.Errorf("bools=%v: Resolve dropped Formatting.%s: got %v, want %v",
					boolVal, name, r.Interface(), expected)
			}

			o := ov.FieldByName(name)
			if !o.IsValid() {
				missingFromOptions = append(missingFromOptions, name)

				continue
			}

			if o.Interface() != expected {
				t.Errorf("bools=%v: FormatterOptions dropped %s: got %v, want %v",
					boolVal, name, o.Interface(), expected)
			}
		}

		sort.Strings(missingFromOptions)

		if !reflect.DeepEqual(missingFromOptions, notFormatterOptions) {
			t.Errorf("formatting keys with no formatter.Options counterpart: got %v, want %v — "+
				"a new key belongs on Options, or in notFormatterOptions with a reason",
				missingFromOptions, notFormatterOptions)
		}
	}
}

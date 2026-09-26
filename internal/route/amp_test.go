package route

import "testing"

// The `&amp;` skip is how a route written inside an HTML-escaped query string is
// found: `?do=a.b&amp;x=1` separates its parameters with `&amp;`, not `&`.
//
// It used to be tested by lowercasing the whole rest of the document and taking
// a prefix of it, which is correct and was 64% of the scan. These pin the
// behaviour so the cheap comparison that replaced it cannot quietly differ:
// case-insensitivity, the boundary at end-of-input, and a near-miss that must
// not be skipped.
func TestAmpEntitySkip(t *testing.T) {
	cfg := Config{QueryParams: []string{"do"}}

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"plain ampersand", `href="page.cfm?x=1&do=a.b.c"`, "a.b.c"},
		{"escaped ampersand", `href="page.cfm?x=1&amp;do=a.b.c"`, "a.b.c"},
		{"escaped, upper case", `href="page.cfm?x=1&AMP;do=a.b.c"`, "a.b.c"},
		{"escaped, mixed case", `href="page.cfm?x=1&AmP;do=a.b.c"`, "a.b.c"},
		{"question mark start", `href="page.cfm?do=a.b.c"`, "a.b.c"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refs := Scan(c.content, &cfg)
			if len(refs) != 1 {
				t.Fatalf("expected 1 ref, got %d from %q", len(refs), c.content)
			}

			if refs[0].Value != c.want {
				t.Errorf("value = %q, want %q", refs[0].Value, c.want)
			}
		})
	}
}

// An '&' at the very end of the input must not read past it.
func TestAmpEntitySkipAtEndOfInput(t *testing.T) {
	for _, content := range []string{"&", "&a", "&am", "&amp", "?", "x?"} {
		if refs := Scan(content, &Config{QueryParams: []string{"do"}}); len(refs) != 0 {
			t.Errorf("Scan(%q) = %v, want none", content, refs)
		}
	}
}

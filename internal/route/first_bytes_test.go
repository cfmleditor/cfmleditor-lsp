package route

import "testing"

// The first-byte set is a fast rejection in front of the real matcher, and the
// failure it invites is silent: a byte missing from the set means the matcher is
// never asked, so the route is not found and nothing reports an error. Route
// names match case-insensitively, so both spellings of the first byte have to be
// in it.
func TestFirstByteSetAdmitsEitherCase(t *testing.T) {
	set := firstBytes([]string{"data-view", "Do", "REDIRECT"})

	for _, c := range []byte{'d', 'D', 'r', 'R'} {
		if !set[c] {
			t.Errorf("first-byte set rejects %q; the matcher will never see a name starting with it", c)
		}
	}

	for _, c := range []byte{'x', 'Z', ' ', '<'} {
		if set[c] {
			t.Errorf("first-byte set admits %q, which begins none of the names", c)
		}
	}
}

func TestFirstByteSetIgnoresEmptyNames(t *testing.T) {
	set := firstBytes([]string{"", "do"})
	if !set['d'] || !set['D'] {
		t.Error("an empty name in the list broke the set")
	}
}

// Scanning must find a route however the source spells the name, which is the
// property the fast path could quietly break.
func TestScanFindsNamesInAnyCase(t *testing.T) {
	cfg := Config{
		Attributes:  []string{"data-view"},
		QueryParams: []string{"do"},
		Properties:  []string{"view"},
		Functions:   []string{"redirect"},
	}

	cases := []struct{ name, content string }{
		{"attribute lower", `<a data-view="a.b.c">x</a>`},
		{"attribute upper", `<a DATA-VIEW="a.b.c">x</a>`},
		{"attribute mixed", `<a Data-View="a.b.c">x</a>`},
		{"query lower", `href="p.cfm?do=a.b.c"`},
		{"query upper", `href="p.cfm?DO=a.b.c"`},
		{"property lower", `{ view: "a.b.c" }`},
		{"property upper", `{ VIEW: "a.b.c" }`},
		{"property quoted upper", `{ "VIEW": "a.b.c" }`},
		{"function lower", `redirect("a.b.c")`},
		{"function upper", `REDIRECT("a.b.c")`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refs := Scan(c.content, cfg)
			if len(refs) == 0 {
				t.Fatalf("no route found in %q", c.content)
			}

			if refs[0].Value != "a.b.c" {
				t.Errorf("value = %q, want a.b.c", refs[0].Value)
			}
		})
	}
}

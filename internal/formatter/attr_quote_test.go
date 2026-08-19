package formatter

import "testing"

// TestNormaliseAttrValueKeepsInnerQuotes covers a corruption found by running
// the formatter over a large CFML corpus. Re-quoting used to swap every inner
// double quote for a single one, which produces an unbalanced literal whenever
// the value already contains the delimiter it is being swapped to:
// `to="#listLen(temp,"'")#"` became `to="#listLen(temp,”')#"`, which the
// grammar then rejects. The quote characters inside a value are now left alone.
func TestNormaliseAttrValueKeepsInnerQuotes(t *testing.T) {
	f := New(DefaultOptions())

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"nested double quotes survive", `"#listLen(temp,"'")#"`, `"#listLen(temp,"'")#"`},
		{"single quotes kept when value holds a double quote", `'a"b'`, `'a"b'`},
		{"single quotes upgraded when safe", `'plain'`, `"plain"`},
		{"already double quoted", `"plain"`, `"plain"`},
		{"unquoted value gains quotes", `plain`, `"plain"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.normaliseAttrValue(tc.in); got != tc.want {
				t.Errorf("normaliseAttrValue(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

package server

import "testing"

// TestContainsFoldStrHandlesNonLetters covers a fold bug in the workspace-symbol
// filter: only the haystack was folded, with |0x20, and compared against a raw
// needle byte. Any byte outside a-z therefore failed to match itself — '_' is
// 0x5F and 0x5F|0x20 is 0x7F — so searching for a symbol containing an
// underscore returned nothing at all.
func TestContainsFoldStrHandlesNonLetters(t *testing.T) {
	cases := []struct {
		s, substr string
		want      bool
	}{
		{"get_user", "get_user", true},
		{"getUserById", "get_user", false},
		{"GET_USER", "get_user", true},
		{"prefix_get_user_suffix", "get_user", true},
		{"getUser", "getuser", true},
		{"getUser", "GETUSER", true},
		{"a-b.c:d", "a-b.c:d", true},
		{"value$", "value$", true},
		// |0x20 on both sides would collide these; an explicit A-Z fold must not.
		{"a@b", "a`b", false},
		{"x[y", "x{y", false},
	}

	for _, tc := range cases {
		if got := containsFoldStr(tc.s, tc.substr); got != tc.want {
			t.Errorf("containsFoldStr(%q, %q) = %v, want %v", tc.s, tc.substr, got, tc.want)
		}
	}
}

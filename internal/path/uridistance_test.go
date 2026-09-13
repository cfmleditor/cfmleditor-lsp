package path

import "testing"

func TestURIDistance(t *testing.T) {
	const base = "file:///ws/app/models/User.cfc"

	cases := []struct {
		name  string
		other string
		want  int
	}{
		{"same file", base, 0},
		{"same directory", "file:///ws/app/models/Order.cfc", 0},
		{"one deeper", "file:///ws/app/models/audit/Trail.cfc", 1},
		{"one shallower", "file:///ws/app/Base.cfc", 1},
		{"sibling directory", "file:///ws/app/services/UserService.cfc", 2},
		{"two up, one down", "file:///ws/lib/Util.cfc", 3},
		{"nothing in common", "file:///other/Thing.cfc", 4},
		// A shared byte run that stops part-way through a segment shares no
		// directory below the one before it.
		{"prefix collision", "file:///ws/app/modelsX/User.cfc", 2},
		// The rest of this package treats paths case-insensitively.
		{"case differs", "file:///WS/App/Models/Order.cfc", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := URIDistance(base, tc.other); got != tc.want {
				t.Errorf("URIDistance(base, %s) = %d, want %d", tc.other, got, tc.want)
			}

			// Symmetric, since it measures from the shared directory.
			if got := URIDistance(tc.other, base); got != tc.want {
				t.Errorf("reversed = %d, want %d — not symmetric", got, tc.want)
			}
		})
	}
}

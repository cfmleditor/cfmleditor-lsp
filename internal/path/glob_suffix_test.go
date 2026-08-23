package path

import "testing"

// A "**" glob's suffix used to be matched against the basename alone, so any
// suffix with more than one segment silently matched nothing — a
// workspaceIndexGlobs entry like "src/**/models/*.cfc" indexed no files and
// reported no error.
func TestMatchesGlobMultiSegmentSuffix(t *testing.T) {
	tests := []struct {
		name string
		path string
		glob string
		want bool
	}{
		{"multi-segment suffix matches at depth", "/app/src/a/b/models/User.cfc", "/app/src/**/models/*.cfc", true},
		{"multi-segment suffix matches immediately", "/app/src/models/User.cfc", "/app/src/**/models/*.cfc", true},
		{"wrong directory rejected", "/app/src/a/views/User.cfc", "/app/src/**/models/*.cfc", false},
		{"wrong extension rejected", "/app/src/a/models/User.cfm", "/app/src/**/models/*.cfc", false},
		{"single-segment suffix still works", "/app/src/a/b/User.cfc", "/app/src/**/*.cfc", true},
		{"single-segment suffix rejects other extensions", "/app/src/a/b/User.cfm", "/app/src/**/*.cfc", false},
		{"empty suffix matches everything under base", "/app/src/a/b/User.cfc", "/app/src/**", true},
		{"outside base rejected", "/other/src/a/models/User.cfc", "/app/src/**/models/*.cfc", false},
		{"a star does not cross a separator", "/app/src/a/b/models/User.cfc", "/app/src/**/models/*/*.cfc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesGlob(tt.path, []string{tt.glob}); got != tt.want {
				t.Errorf("MatchesGlob(%q, %q) = %v, want %v", tt.path, tt.glob, got, tt.want)
			}
		})
	}
}

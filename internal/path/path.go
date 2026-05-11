// Package path resolves CFML component dot-paths to filesystem paths.
package path

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath resolves a CFML dot-path (e.g. "models.User") to an absolute
// .cfc file path. It checks mappings first (matching the first segment), then
// falls back to resolving relative to baseDir. Returns empty string if not found.
func ResolvePath(dotPath string, baseDir string, mappings map[string]string) string {
	parts := strings.SplitN(dotPath, ".", 2)
	// Check mappings for first segment
	if mappings != nil {
		if mapped, ok := mappings[parts[0]]; ok {
			var rel string
			if len(parts) == 2 {
				rel = strings.ReplaceAll(parts[1], ".", string(filepath.Separator)) + ".cfc"
			} else {
				rel = parts[0] + ".cfc"
			}
			abs := filepath.Join(mapped, rel)
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	// Fall back to relative resolution
	rel := strings.ReplaceAll(dotPath, ".", string(filepath.Separator)) + ".cfc"
	abs := filepath.Join(baseDir, rel)
	if _, err := os.Stat(abs); err == nil {
		return abs
	}
	return ""
}

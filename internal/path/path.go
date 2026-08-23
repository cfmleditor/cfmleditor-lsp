// Package path resolves CFML component dot-paths to filesystem paths.
package path

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// DefaultFS is the filesystem used by this package. Override for testing or WASM.
var DefaultFS vfs.FS = vfs.OS{}

var (
	appMappingsCache   = make(map[string]map[string]string)
	appMappingsCacheMu sync.RWMutex
)

// ParseApplicationMappings delegates to parser.ParseApplicationMappings.
func ParseApplicationMappings(content string, appDir string) map[string]string {
	return parser.ParseApplicationMappings(content, appDir)
}

// readApplicationFile reads Application.cfc or Application.cfm from appDir.
func readApplicationFile(appDir string) []byte {
	for _, name := range []string{"Application.cfc", "Application.cfm"} {
		data, err := DefaultFS.ReadFile(filepath.Join(appDir, name))
		if err == nil {
			return data
		}
	}

	return nil
}

// LoadAppMappings returns cached Application.cfc mappings for appDir,
// loading and parsing them on first access.
func LoadAppMappings(appDir string) map[string]string {
	appMappingsCacheMu.RLock()

	if m, ok := appMappingsCache[appDir]; ok {
		appMappingsCacheMu.RUnlock()

		return m
	}

	appMappingsCacheMu.RUnlock()

	content := readApplicationFile(appDir)
	if content == nil {
		return nil
	}

	m := parser.ParseApplicationMappings(string(content), appDir)

	appMappingsCacheMu.Lock()
	appMappingsCache[appDir] = m
	appMappingsCacheMu.Unlock()

	return m
}

// InvalidateAppMappingsCache clears the cached Application.cfc mappings.
func InvalidateAppMappingsCache() {
	appMappingsCacheMu.Lock()
	appMappingsCache = make(map[string]map[string]string)
	appMappingsCacheMu.Unlock()
}

// ParseAppBeanPaths delegates to parser.ParseAppBeanPaths.
func ParseAppBeanPaths(content string, appDir string) map[string]string {
	return parser.ParseAppBeanPaths(content, appDir)
}

// LoadAppBeanPaths returns cached Application.cfc bean paths for appDir.
func LoadAppBeanPaths(appDir string) map[string]string {
	appMappingsCacheMu.RLock()

	key := appDir + "|beans"
	if m, ok := appMappingsCache[key]; ok {
		appMappingsCacheMu.RUnlock()

		return m
	}

	appMappingsCacheMu.RUnlock()

	content := readApplicationFile(appDir)
	if content == nil {
		return nil
	}

	m := parser.ParseAppBeanPaths(string(content), appDir)

	appMappingsCacheMu.Lock()
	appMappingsCache[key] = m
	appMappingsCacheMu.Unlock()

	return m
}

// ParseOrmLocations delegates to parser.ParseOrmLocations.
func ParseOrmLocations(content string, appDir string) []string {
	return parser.ParseOrmLocations(content, appDir)
}

// LoadOrmLocations returns cached ORM cfcLocation paths for appDir.
func LoadOrmLocations(appDir string) []string {
	appMappingsCacheMu.RLock()

	key := appDir + "|orm"
	if m, ok := appMappingsCache[key]; ok {
		appMappingsCacheMu.RUnlock()

		out := make([]string, 0, len(m))
		for _, v := range m {
			out = append(out, v)
		}

		return out
	}

	appMappingsCacheMu.RUnlock()

	content := readApplicationFile(appDir)
	if content == nil {
		return nil
	}

	locs := parser.ParseOrmLocations(string(content), appDir)

	m := make(map[string]string, len(locs))
	for i, l := range locs {
		m[string(rune('0'+i))] = l
	}

	appMappingsCacheMu.Lock()
	appMappingsCache[key] = m
	appMappingsCacheMu.Unlock()

	return locs
}

// ResolvePath resolves a CFML dot-path (e.g. "models.User") to an absolute
// .cfc file path. It checks mappings first (matching the first segment, case-insensitively),
// then falls back to resolving relative to baseDir. CFML component paths are conventionally
// case-insensitive, so every path segment is matched without regard to case. Returns empty
// string if not found.
func ResolvePath(dotPath string, baseDir string, mappings map[string]string) string {
	// Normalize CF slash-paths (e.g. /tassweb/packages/foo) to dot-paths
	if strings.Contains(dotPath, "/") {
		dotPath = strings.TrimPrefix(dotPath, "/")
		dotPath = strings.ReplaceAll(dotPath, "/", ".")
	}

	parts := strings.SplitN(dotPath, ".", 2)
	if mappings != nil {
		if mapped, ok := lookupFold(mappings, parts[0]); ok {
			var segments []string
			if len(parts) == 2 {
				segments = strings.Split(parts[1], ".")
			} else {
				segments = []string{parts[0]}
			}

			segments[len(segments)-1] += ".cfc"

			if abs := resolveSegments(mapped, segments); abs != "" {
				return abs
			}
		}
	}

	segments := strings.Split(dotPath, ".")
	segments[len(segments)-1] += ".cfc"

	return resolveSegments(baseDir, segments)
}

// lookupFold returns the value for the first key in m that matches key
// case-insensitively, and whether one was found.
func lookupFold(m map[string]string, key string) (string, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}

	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}

	return "", false
}

// resolveSegments walks from baseDir through each path segment (the last segment being the
// filename, already suffixed with .cfc), returning the real, case-correct absolute path if
// found, or "" if any segment is missing.
//
// Each segment is resolved via a directory listing rather than a direct Stat: on a
// case-insensitive filesystem (APFS, NTFS default), Stat succeeds for any case variant of an
// existing name, which would silently accept whatever case the dot-path happened to use
// instead of the real on-disk case. Listing the directory and matching entries — exact case
// first, then case-insensitively — is the only way to recover the true name on every platform.
func resolveSegments(baseDir string, segments []string) string {
	dir := baseDir

	for _, seg := range segments {
		entries, err := DefaultFS.ReadDir(dir)
		if err != nil {
			return ""
		}

		found := ""

		for _, e := range entries {
			if e.Name() == seg {
				found = e.Name()

				break
			}
		}

		if found == "" {
			for _, e := range entries {
				if strings.EqualFold(e.Name(), seg) {
					found = e.Name()

					break
				}
			}
		}

		if found == "" {
			return ""
		}

		dir = filepath.Join(dir, found)
	}

	return dir
}

// ResolveMappings resolves relative paths in a map to absolute using baseDir.
func ResolveMappings(raw map[string]string, baseDir string) map[string]string {
	if len(raw) == 0 {
		return nil
	}

	out := make(map[string]string, len(raw))

	for k, v := range raw {
		if filepath.IsAbs(v) {
			out[k] = v
		} else {
			out[k] = filepath.Join(baseDir, v)
		}
	}

	return out
}

// matchGlobSuffix reports whether the trailing segments of path match the
// segment pattern suffix — the part of a glob that follows "**".
//
// Both callers used to match suffix against filepath.Base(path) alone, which
// works for the common "**/*.cfc" and silently matches nothing the moment the
// suffix has more than one segment: "src/**/models/*.cfc" compares
// "models/*.cfc" against "User.cfc" and never fires, so a workspaceIndexGlobs
// entry written that way indexes no files and reports no error.
//
// filepath.Match is applied per segment, so a "*" still does not cross a
// separator; it is only the "**" that spans depth.
func matchGlobSuffix(suffix, path string) bool {
	want := strings.Split(suffix, string(filepath.Separator))
	got := strings.Split(path, string(filepath.Separator))

	if len(got) < len(want) {
		return false
	}

	got = got[len(got)-len(want):]

	for i, w := range want {
		if matched, _ := filepath.Match(w, got[i]); !matched {
			return false
		}
	}

	return true
}

// ExpandGlob expands a glob pattern, supporting ** for recursive matching.
func ExpandGlob(pattern string) []string {
	if !strings.Contains(pattern, "**") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil
		}

		return matches
	}

	before, after, _ := strings.Cut(pattern, "**")
	base := filepath.Clean(before)
	suffix := after
	suffix = strings.TrimPrefix(suffix, string(filepath.Separator))

	var out []string

	_ = DefaultFS.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if suffix == "" {
			out = append(out, path)

			return nil
		}

		if matchGlobSuffix(suffix, path) {
			out = append(out, path)
		}

		return nil
	})

	return out
}

// IsCFMLFile returns true if the path/URI refers to a CFML file (.cfc, .cfm, .cfml, .cfs).
func IsCFMLFile(path string) bool {
	if len(path) < 4 {
		return false
	}

	end := path[len(path)-1]
	switch end | 0x20 {
	case 'c': // .cfc
		return len(path) > 4 && path[len(path)-4] == '.' &&
			(path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	case 'm': // .cfm
		return path[len(path)-4] == '.' && (path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	case 'l': // .cfml
		return len(path) > 5 && path[len(path)-5] == '.' && (path[len(path)-4]|0x20) == 'c' &&
			(path[len(path)-3]|0x20) == 'f' && (path[len(path)-2]|0x20) == 'm'
	case 's': // .cfs
		return path[len(path)-4] == '.' && (path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	}

	return false
}

// IsCFCFile returns true if the path/URI refers to a CFC file.
func IsCFCFile(path string) bool {
	return len(path) > 4 && path[len(path)-4] == '.' &&
		(path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f' && (path[len(path)-1]|0x20) == 'c'
}

// IsBinary returns true if data appears to be binary (contains null bytes in the first 512 bytes).
func IsBinary(data []byte) bool {
	n := min(len(data), 512)

	for i := range n {
		if data[i] == 0 {
			return true
		}
	}

	return false
}

// MatchesGlob checks whether a file path matches any of the given glob patterns.
func MatchesGlob(filePath string, globs []string) bool {
	for _, g := range globs {
		if !strings.Contains(g, "**") {
			if matched, _ := filepath.Match(g, filePath); matched {
				return true
			}

			if strings.HasPrefix(filePath, g+"/") || filePath == g {
				return true
			}

			continue
		}

		before, after, _ := strings.Cut(g, "**")
		base := filepath.Clean(before)
		suffix := after
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))

		if !strings.HasPrefix(filePath, base+"/") && filePath != base {
			continue
		}

		if suffix == "" {
			return true
		}

		if matchGlobSuffix(suffix, filePath) {
			return true
		}
	}

	return false
}

// SamePath reports whether two filesystem paths refer to the same file,
// tolerating case differences (common on APFS/NTFS) and cosmetic
// differences like trailing slashes or "./" segments.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// CfcNameFromURI extracts the CFC filename without extension from a URI.
func CfcNameFromURI(fileURI string) string {
	path := strings.TrimPrefix(fileURI, "file://")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	base := filepath.Base(path)

	return strings.TrimSuffix(base, filepath.Ext(base))
}

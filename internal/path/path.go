// Package path resolves CFML component dot-paths to filesystem paths.
package path

import (
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
// .cfc file path. It checks mappings first (matching the first segment), then
// falls back to resolving relative to baseDir. Returns empty string if not found.
func ResolvePath(dotPath string, baseDir string, mappings map[string]string) string {
	parts := strings.SplitN(dotPath, ".", 2)
	if mappings != nil {
		if mapped, ok := mappings[parts[0]]; ok {
			var rel string
			if len(parts) == 2 {
				rel = strings.ReplaceAll(parts[1], ".", string(filepath.Separator)) + ".cfc"
			} else {
				rel = parts[0] + ".cfc"
			}

			abs := filepath.Join(mapped, rel)
			if _, err := DefaultFS.Stat(abs); err == nil {
				return realPath(abs)
			}
		}
	}

	rel := strings.ReplaceAll(dotPath, ".", string(filepath.Separator)) + ".cfc"

	abs := filepath.Join(baseDir, rel)
	if _, err := DefaultFS.Stat(abs); err == nil {
		return realPath(abs)
	}

	return ""
}

// realPath returns the actual case-correct path from the filesystem.
func realPath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := DefaultFS.ReadDir(dir)
	if err != nil {
		return path
	}

	for _, e := range entries {
		if strings.EqualFold(e.Name(), base) {
			return filepath.Join(dir, e.Name())
		}
	}

	return path
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

// ExpandGlob expands a glob pattern, supporting ** for recursive matching.
func ExpandGlob(pattern string) []string {
	if !strings.Contains(pattern, "**") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil
		}

		return matches
	}

	idx := strings.Index(pattern, "**")
	base := filepath.Clean(pattern[:idx])
	suffix := pattern[idx+2:]
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

		if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
			out = append(out, path)
		}

		return nil
	})

	return out
}

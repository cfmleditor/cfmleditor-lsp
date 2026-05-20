// Package path resolves CFML component dot-paths to filesystem paths.
package path

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	appMappingsCache   = make(map[string]map[string]string)
	appMappingsCacheMu sync.RWMutex
)

// mappingRe matches: this.mappings["/key"] = expandPath("./path") or this.mappings["/key"] = "path"
var mappingRe = regexp.MustCompile(`(?i)this\.mappings\[\s*["']([^"']+)["']\s*\]\s*=\s*(?:expandPath\(\s*["']([^"']+)["']\s*\)|["']([^"']+)["'])`)

// beanPathRe matches: this.beanPaths["namespace"] = expandPath("./path") or this.beanPaths["namespace"] = "path"
var beanPathRe = regexp.MustCompile(`(?i)this\.beanPaths\[\s*["']([^"']*)["']\s*\]\s*=\s*(?:expandPath\(\s*["']([^"']+)["']\s*\)|["']([^"']+)["'])`)

// diLocationsRe matches: variables.framework.diLocations = "path1,path2"
var diLocationsRe = regexp.MustCompile(`(?i)(?:variables\.)?framework\.diLocations\s*=\s*["']([^"']+)["']`)

// ormCfcLocationRe matches: cfcLocation = "path" or cfcLocation: "path" (inside ormSettings struct)
var ormCfcLocationRe = regexp.MustCompile(`(?i)cfcLocation\s*[:=]\s*["']([^"']+)["']`)

// ormCfcLocationArrayRe matches: cfcLocation = ["path1","path2"] or cfcLocation: ["path1","path2"]
var ormCfcLocationArrayRe = regexp.MustCompile(`(?i)cfcLocation\s*[:=]\s*\[([^\]]+)\]`)

// ParseApplicationMappings extracts this.mappings from Application.cfc content.
// appDir is the directory containing Application.cfc, used to resolve relative paths.
// Returns a map of mapping key (without leading /) to absolute directory path.
func ParseApplicationMappings(content string, appDir string) map[string]string {
	matches := mappingRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make(map[string]string, len(matches))
	for _, m := range matches {
		key := strings.TrimPrefix(m[1], "/")
		if key == "" {
			continue
		}
		// m[2] is expandPath value, m[3] is plain string value
		val := m[2]
		if val == "" {
			val = m[3]
		}
		if !filepath.IsAbs(val) {
			val = filepath.Join(appDir, val)
		}
		out[key] = filepath.Clean(val)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

	var content []byte
	for _, name := range []string{"Application.cfc", "Application.cfm"} {
		data, err := os.ReadFile(filepath.Join(appDir, name))
		if err == nil {
			content = data
			break
		}
	}
	if content == nil {
		return nil
	}

	m := ParseApplicationMappings(string(content), appDir)

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

// ParseAppBeanPaths extracts bean path declarations from Application.cfc content.
// Supports:
//   - this.beanPaths["namespace"] = expandPath("./path") or "path"
//   - variables.framework.diLocations = "path1,path2" (FW/1 convention, unnamespaced)
func ParseAppBeanPaths(content string, appDir string) map[string]string {
	out := make(map[string]string)

	// Explicit this.beanPaths["ns"] = ...
	for _, m := range beanPathRe.FindAllStringSubmatch(content, -1) {
		ns := m[1]
		val := m[2]
		if val == "" {
			val = m[3]
		}
		if !filepath.IsAbs(val) {
			val = filepath.Join(appDir, val)
		}
		out[ns] = filepath.Clean(val)
	}

	// FW/1 diLocations (comma-separated, all unnamespaced)
	if len(out) == 0 {
		if m := diLocationsRe.FindStringSubmatch(content); m != nil {
			for _, p := range strings.Split(m[1], ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				abs := p
				if !filepath.IsAbs(p) {
					abs = filepath.Join(appDir, p)
				}
				// Use path basename as namespace if multiple, empty if single
				if strings.Contains(m[1], ",") {
					ns := filepath.Base(abs)
					out[ns] = filepath.Clean(abs)
				} else {
					out[""] = filepath.Clean(abs)
				}
			}
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
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

	var content []byte
	for _, name := range []string{"Application.cfc", "Application.cfm"} {
		data, err := os.ReadFile(filepath.Join(appDir, name))
		if err == nil {
			content = data
			break
		}
	}
	if content == nil {
		return nil
	}

	m := ParseAppBeanPaths(string(content), appDir)

	appMappingsCacheMu.Lock()
	appMappingsCache[key] = m
	appMappingsCacheMu.Unlock()
	return m
}

// ParseOrmLocations extracts this.ormSettings.cfcLocation from Application.cfc content.
// Returns a list of absolute directory paths.
func ParseOrmLocations(content string, appDir string) []string {
	// Try array form first: cfcLocation: ["path1", "path2"]
	if m := ormCfcLocationArrayRe.FindStringSubmatch(content); m != nil {
		var out []string
		for _, part := range strings.Split(m[1], ",") {
			part = strings.TrimSpace(part)
			part = strings.Trim(part, `"'`)
			if part == "" {
				continue
			}
			if !filepath.IsAbs(part) {
				part = filepath.Join(appDir, part)
			}
			out = append(out, filepath.Clean(part))
		}
		if len(out) > 0 {
			return out
		}
	}
	// Try single string: cfcLocation: "path"
	if m := ormCfcLocationRe.FindStringSubmatch(content); m != nil {
		p := m[1]
		if !filepath.IsAbs(p) {
			p = filepath.Join(appDir, p)
		}
		return []string{filepath.Clean(p)}
	}
	return nil
}

// LoadOrmLocations returns cached ORM cfcLocation paths for appDir.
func LoadOrmLocations(appDir string) []string {
	appMappingsCacheMu.RLock()
	key := appDir + "|orm"
	if m, ok := appMappingsCache[key]; ok {
		appMappingsCacheMu.RUnlock()
		// Convert map values to slice
		out := make([]string, 0, len(m))
		for _, v := range m {
			out = append(out, v)
		}
		return out
	}
	appMappingsCacheMu.RUnlock()

	var content []byte
	for _, name := range []string{"Application.cfc", "Application.cfm"} {
		data, err := os.ReadFile(filepath.Join(appDir, name))
		if err == nil {
			content = data
			break
		}
	}
	if content == nil {
		return nil
	}

	locs := ParseOrmLocations(string(content), appDir)

	// Store as map for cache compatibility
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

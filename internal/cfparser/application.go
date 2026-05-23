package cfparser

import (
	"path/filepath"
	"regexp"
	"strings"
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

// ParseAppBeanPaths extracts bean path declarations from Application.cfc content.
// Supports:
//   - this.beanPaths["namespace"] = expandPath("./path") or "path"
//   - variables.framework.diLocations = "path1,path2" (FW/1 convention, unnamespaced)
func ParseAppBeanPaths(content string, appDir string) map[string]string {
	out := make(map[string]string)

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

// ParseOrmLocations extracts this.ormSettings.cfcLocation from Application.cfc content.
// Returns a list of absolute directory paths.
func ParseOrmLocations(content string, appDir string) []string {
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
	if m := ormCfcLocationRe.FindStringSubmatch(content); m != nil {
		p := m[1]
		if !filepath.IsAbs(p) {
			p = filepath.Join(appDir, p)
		}
		return []string{filepath.Clean(p)}
	}
	return nil
}

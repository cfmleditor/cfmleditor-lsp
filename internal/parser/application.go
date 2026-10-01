package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

// beanPathRe matches: this.beanPaths["namespace"] = expandPath("./path") or this.beanPaths["namespace"] = "path".
var beanPathRe = regexp.MustCompile(`(?i)this\.beanPaths\[\s*["']([^"']*)["']\s*\]\s*=\s*(?:expandPath\(\s*["']([^"']+)["']\s*\)|["']([^"']+)["'])`)

// diLocationsRe matches: variables.framework.diLocations = "path1,path2".
var diLocationsRe = regexp.MustCompile(`(?i)(?:variables\.)?framework\.diLocations\s*=\s*["']([^"']+)["']`)

// fw1AppRe is an Application.cfc that is an FW/1 application.
var fw1AppRe = regexp.MustCompile(`(?i)extends\s*=\s*["']framework\.one["']`)

// ormCfcLocationRe matches: cfcLocation = "path" or cfcLocation: "path" (inside ormSettings struct).
var ormCfcLocationRe = regexp.MustCompile(`(?i)cfcLocation\s*[:=]\s*["']([^"']+)["']`)

// ormCfcLocationArrayRe matches: cfcLocation = ["path1","path2"] or cfcLocation: ["path1","path2"].
var ormCfcLocationArrayRe = regexp.MustCompile(`(?i)cfcLocation\s*[:=]\s*\[([^\]]+)\]`)

// ParseApplicationMappings extracts this.mappings from Application.cfc content.
// appDir is the directory containing Application.cfc, used to resolve relative paths.
// Returns a map of mapping key (without leading /) to absolute directory path.
func ParseApplicationMappings(content string, appDir string) map[string]string {
	s := &mappingSourceState{mappings: map[string]string{}, env: map[string]string{}, base: filepath.Join(appDir, "Application.cfc")}
	s.content(content, s.base, 0)

	if len(s.mappings) == 0 {
		return nil
	}

	return s.mappings
}

// mappingTargetRe is `this.mappings["/key"]`, in any case and either quote.
var mappingTargetRe = regexp.MustCompile(`(?i)^this\.mappings\[\s*["']([^"']+)["']\s*\]$`)

// envName is how a variable is remembered: lowercased, without the local. or
// variables. that CFML lets a component's pseudo-constructor leave off.
func envName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, scope := range []string{"local.", "variables."} {
		name = strings.TrimPrefix(name, scope)
	}

	return name
}

// cleanMappingPath is a mapping's directory: a relative one is the
// application's, as the literal form has always been read.
func cleanMappingPath(p, appDir string) string {
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(appDir, p)
	}

	return filepath.Clean(p)
}

// evalPathExpr evaluates the path expressions an Application.cfc builds a
// mapping from: string literals joined by &, expandPath(), the file's own
// directory by getDirectoryFromPath( getCurrentTemplatePath() ) (or
// getBaseTemplatePath(), which from Application.cfc is a page beside it, as
// near as can be said without a request), a variable assigned above, and an
// earlier mapping. Anything else — a url.x, a ternary, a function of a
// server's own — is not a path the source states, and the whole expression
// is declined rather than half-read.
func evalPathExpr(expr string, env map[string]string, appDir string) (string, bool) {
	if len(expr) > 8192 || strings.Count(expr, "(") > 32 {
		return "", false
	}

	var b strings.Builder

	for _, term := range splitConcat(expr) {
		v, ok := evalPathTerm(term, env, appDir)
		if !ok {
			return "", false
		}

		b.WriteString(v)
	}

	return b.String(), b.Len() > 0
}

func evalPathTerm(term string, env map[string]string, appDir string) (string, bool) {
	term = strings.TrimSpace(term)

	if lit, ok := stringLiteral(term); ok {
		return lit, true
	}

	if arg, ok := callArg(term, "expandPath"); ok {
		v, ok := evalPathExpr(arg, env, appDir)
		if !ok {
			return "", false
		}

		// expandPath resolves against the template's directory and keeps a
		// trailing separator, which the next & usually relies on.
		abs := v
		if !filepath.IsAbs(filepath.FromSlash(v)) {
			abs = filepath.Join(appDir, filepath.FromSlash(v))
		}

		if strings.HasSuffix(v, "/") || strings.HasSuffix(v, "\\") {
			abs += string(filepath.Separator)
		}

		return abs, true
	}

	if arg, ok := callArg(term, "getDirectoryFromPath"); ok {
		v, ok := evalPathExpr(arg, env, appDir)
		if !ok {
			return "", false
		}

		return filepath.Dir(filepath.FromSlash(v)) + string(filepath.Separator), true
	}

	for _, fn := range []string{"getCurrentTemplatePath", "getBaseTemplatePath"} {
		if arg, ok := callArg(term, fn); ok && strings.TrimSpace(arg) == "" {
			if value := env["@template"]; value != "" && fn == "getCurrentTemplatePath" {
				return value, true
			}

			if value := env["@baseTemplate"]; value != "" && fn == "getBaseTemplatePath" {
				return value, true
			}

			return filepath.Join(appDir, "Application.cfc"), true
		}
	}

	if value, ok := slicedMappingPath(term, env, appDir); ok {
		return value, true
	}

	if m := mappingTargetRe.FindStringSubmatch(term); m != nil {
		v, ok := env["mapping:"+strings.ToLower(strings.Trim(strings.TrimSpace(m[1]), "/"))]

		return v, ok
	}

	if isDottedName(term) {
		v, ok := env[envName(term)]

		return v, ok
	}

	return "", false
}

// splitConcat splits an expression at the & operators outside strings and
// parentheses.
func splitConcat(expr string) []string {
	var (
		parts []string
		depth int
		quote byte
		start int
	)

	for i := range len(expr) {
		c := expr[i]

		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '&' && depth == 0:
			parts = append(parts, expr[start:i])
			start = i + 1
		}
	}

	return append(parts, expr[start:])
}

// stringLiteral is the text of a quoted literal holding no interpolation.
func stringLiteral(term string) (string, bool) {
	if len(term) < 2 || (term[0] != '"' && term[0] != '\'') || term[len(term)-1] != term[0] {
		return "", false
	}

	body := term[1 : len(term)-1]
	if strings.ContainsRune(body, '#') || strings.IndexByte(body, term[0]) >= 0 {
		return "", false
	}

	return body, true
}

// callArg is the argument text of term when term is a call to fn, alone.
func callArg(term, fn string) (string, bool) {
	if len(term) < len(fn)+2 || !strings.EqualFold(term[:len(fn)], fn) {
		return "", false
	}

	rest := strings.TrimSpace(term[len(fn):])
	if len(rest) < 2 || rest[0] != '(' || rest[len(rest)-1] != ')' {
		return "", false
	}

	return rest[1 : len(rest)-1], true
}

func isDottedName(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if c := s[i]; c != '.' && c != '_' && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}

	return true
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

	// An FW/1 application that names no diLocations has DI/1 search
	// "model,controllers" (framework/one.cfc's default).
	locations := ""
	if m := diLocationsRe.FindStringSubmatch(content); m != nil {
		locations = m[1]
	} else if fw1AppRe.MatchString(content) {
		locations = "model,controllers"
	}

	if len(out) == 0 && locations != "" {
		for p := range strings.SplitSeq(locations, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			abs := p
			if !filepath.IsAbs(p) {
				abs = filepath.Join(appDir, p)
			}

			if strings.Contains(locations, ",") {
				out[filepath.Base(abs)] = filepath.Clean(abs)
			} else {
				out[""] = filepath.Clean(abs)
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

		for part := range strings.SplitSeq(m[1], ",") {
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

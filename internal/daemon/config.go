// Package daemon manages multi-session daemon mode and configuration.
package daemon

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// configJSON is the on-disk shape of .cfmleditor.json.
type configJSON struct {
	WorkspaceName       string              `json:"workspaceName"`
	WorkspacePaths      []string            `json:"workspacePaths"`
	WorkspaceIndexGlobs []string            `json:"workspaceIndexGlobs"`
	Mappings            map[string]string   `json:"mappings"`
	ComponentResolvers  []componentResolver `json:"componentResolvers"`
	Formatting          *formattingConfig   `json:"formatting"`
	Debug               bool                `json:"debug"`
}

type componentResolver struct {
	Match   string `json:"match"`
	Resolve string `json:"resolve"`
	Prefix  string `json:"prefix"`
}

type formattingConfig struct {
	Enabled                bool   `json:"enabled"`
	Debug                  bool   `json:"debug"`
	SelfCloseTags          *bool  `json:"selfCloseTags"`
	WhitespaceOnly         *bool  `json:"whitespaceOnly"`
	QueryFormat            *bool  `json:"queryFormat"`
	LowercaseTags          *bool  `json:"lowercaseTags"`
	LowercaseAttributes    *bool  `json:"lowercaseAttributes"`
	DoubleQuoteAttributes  *bool  `json:"doubleQuoteAttributes"`
	QueryUppercaseKeywords *bool  `json:"queryUppercaseKeywords"`
	ScopeCase              string `json:"scopeCase"`
	CommaPosition          string `json:"commaPosition"`
	QueryCommaPosition     string `json:"queryCommaPosition"`
	LineWidth              *int   `json:"lineWidth"`
	AttrBreakThreshold     *int   `json:"attrBreakThreshold"`
	IndentWidth            *int   `json:"indentWidth"`
}

// Config represents a .cfmleditor.json file.
type Config struct {
	Path string // absolute path to the config file itself
	Name string // project name used to derive the daemon socket
}

// FindConfig looks for .cfmleditor.json starting from dir, then one level up.
func FindConfig(dir string) (*Config, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	d := abs
	for {
		p := filepath.Join(d, ".cfmleditor.json")
		data, err := os.ReadFile(p)
		if err == nil {
			var raw configJSON
			if json.Unmarshal(data, &raw) == nil {
				if raw.WorkspaceName == "" {
					raw.WorkspaceName = filepath.Dir(p)
				}
				return &Config{Path: p, Name: raw.WorkspaceName}, nil
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return &Config{Path: "", Name: filepath.Base(abs)}, nil
}

// SocketPath returns a deterministic Unix socket path derived from the project name.
func (c *Config) SocketPath() string {
	h := sha256.Sum256([]byte(c.Name))
	name := fmt.Sprintf("cfmleditor-%x.sock", h[:8])
	return filepath.Join(socketDir(), name)
}

func (c *Config) raw() *configJSON {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return nil
	}
	var raw configJSON
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	return &raw
}

// WorkspaceFolders returns the resolved absolute paths of the project folders.
func (c *Config) WorkspaceFolders() []string {
	raw := c.raw()
	if raw == nil {
		return nil
	}
	dir := filepath.Dir(c.Path)
	out := make([]string, 0, len(raw.WorkspacePaths))
	for _, p := range raw.WorkspacePaths {
		out = append(out, filepath.Join(dir, p))
	}
	return out
}

// Mappings returns component path mappings with values resolved to absolute paths.
func (c *Config) Mappings() map[string]string {
	raw := c.raw()
	if raw == nil || len(raw.Mappings) == 0 {
		return nil
	}
	dir := filepath.Dir(c.Path)
	out := make(map[string]string, len(raw.Mappings))
	for key, val := range raw.Mappings {
		if filepath.IsAbs(val) {
			out[key] = val
		} else {
			out[key] = filepath.Join(dir, val)
		}
	}
	return out
}

// Debug returns whether debug logging is enabled in config.
func (c *Config) Debug() bool {
	raw := c.raw()
	return raw != nil && raw.Debug
}

// FormattingEnabled returns whether formatting is enabled in config.
func (c *Config) FormattingEnabled() bool {
	raw := c.raw()
	return raw != nil && raw.Formatting != nil && raw.Formatting.Enabled
}

// FormattingDebug returns whether formatting debug checks are enabled.
func (c *Config) FormattingDebug() bool {
	raw := c.raw()
	return raw != nil && raw.Formatting != nil && raw.Formatting.Debug
}

// FormattingSelfCloseTags returns whether void/implicit-end HTML tags should be self-closed.
// Defaults to true if not specified.
func (c *Config) FormattingSelfCloseTags() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.SelfCloseTags == nil {
		return true
	}
	return *raw.Formatting.SelfCloseTags
}

// FormattingWhitespaceOnly returns whether the formatter should reject non-whitespace changes.
// Defaults to true if not specified.
func (c *Config) FormattingWhitespaceOnly() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.WhitespaceOnly == nil {
		return true
	}
	return *raw.Formatting.WhitespaceOnly
}

// FormattingQueryFormat returns whether cfquery content should be formatted. Default false.
func (c *Config) FormattingQueryFormat() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.QueryFormat == nil {
		return false
	}
	return *raw.Formatting.QueryFormat
}

// FormattingLowercaseTags returns whether CF tag names should be lowercased. Default true.
func (c *Config) FormattingLowercaseTags() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.LowercaseTags == nil {
		return true
	}
	return *raw.Formatting.LowercaseTags
}

// FormattingLowercaseAttributes returns whether attribute names should be lowercased. Default true.
func (c *Config) FormattingLowercaseAttributes() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.LowercaseAttributes == nil {
		return true
	}
	return *raw.Formatting.LowercaseAttributes
}

// FormattingDoubleQuoteAttributes returns whether attribute values should be double-quoted. Default true.
func (c *Config) FormattingDoubleQuoteAttributes() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.DoubleQuoteAttributes == nil {
		return true
	}
	return *raw.Formatting.DoubleQuoteAttributes
}

// FormattingQueryUppercaseKeywords returns whether SQL keywords should be uppercased. Default true.
func (c *Config) FormattingQueryUppercaseKeywords() bool {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.QueryUppercaseKeywords == nil {
		return true
	}
	return *raw.Formatting.QueryUppercaseKeywords
}

// FormattingScopeCase returns the scope case setting ("upper", "lower", or "leave"). Default "leave".
func (c *Config) FormattingScopeCase() string {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.ScopeCase == "" {
		return "leave"
	}
	return raw.Formatting.ScopeCase
}

// FormattingCommaPosition returns the comma position setting ("before" or "after"). Default "after".
func (c *Config) FormattingCommaPosition() string {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.CommaPosition == "" {
		return "after"
	}
	return raw.Formatting.CommaPosition
}

// FormattingQueryCommaPosition returns the SQL comma position setting. Defaults to commaPosition value.
func (c *Config) FormattingQueryCommaPosition() string {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.QueryCommaPosition == "" {
		return ""
	}
	return raw.Formatting.QueryCommaPosition
}

// FormattingLineWidth returns the configured line width, or 0 if not set.
func (c *Config) FormattingLineWidth() int {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.LineWidth == nil {
		return 0
	}
	return *raw.Formatting.LineWidth
}

// FormattingAttrBreakThreshold returns the configured attr break threshold, or 0 if not set.
func (c *Config) FormattingAttrBreakThreshold() int {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.AttrBreakThreshold == nil {
		return 0
	}
	return *raw.Formatting.AttrBreakThreshold
}

// FormattingIndentWidth returns the configured indent width, or 0 if not set.
func (c *Config) FormattingIndentWidth() int {
	raw := c.raw()
	if raw == nil || raw.Formatting == nil || raw.Formatting.IndentWidth == nil {
		return 0
	}
	return *raw.Formatting.IndentWidth
}

// ComponentResolvers returns the configured component resolver patterns as [match, resolve, prefix] triples.
func (c *Config) ComponentResolvers() [][3]string {
	raw := c.raw()
	if raw == nil || len(raw.ComponentResolvers) == 0 {
		return nil
	}
	out := make([][3]string, 0, len(raw.ComponentResolvers))
	for _, r := range raw.ComponentResolvers {
		if r.Match != "" && r.Resolve != "" {
			out = append(out, [3]string{r.Match, r.Resolve, r.Prefix})
		}
	}
	return out
}

// IndexGlobs returns absolute glob patterns for workspace indexing,
// replacing the leading folder name with the corresponding resolved workspace
// folder path. For example, if workspacePaths contains "../tassweb" and
// workspaceIndexGlobs contains "tassweb/**/*.cfc", the result is
// "/abs/path/to/tassweb/**/*.cfc".
func (c *Config) IndexGlobs() []string {
	raw := c.raw()
	if raw == nil || len(raw.WorkspaceIndexGlobs) == 0 {
		return nil
	}
	dir := filepath.Dir(c.Path)
	// Build map from folder base name to resolved absolute path
	folderMap := make(map[string]string)
	for _, p := range raw.WorkspacePaths {
		resolved := filepath.Join(dir, p)
		base := filepath.Base(resolved)
		folderMap[base] = resolved
	}
	out := make([]string, 0, len(raw.WorkspaceIndexGlobs))
	for _, g := range raw.WorkspaceIndexGlobs {
		// First path component is the folder name
		parts := strings.SplitN(g, "/", 2)
		if resolved, ok := folderMap[parts[0]]; ok {
			if len(parts) == 2 {
				out = append(out, resolved+"/"+parts[1])
			} else {
				out = append(out, resolved)
			}
		}
	}
	return out
}

// expandGlob expands a glob pattern, handling ** for recursive directory matching.
func expandGlob(pattern string) []string {
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
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
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

	if err != nil {
		// Handle error (e.g., log it or return it)
		log.Fatalf("failed to write file: %s", err)
	}
	return out
}

func socketDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.TempDir(), "cfmleditor-lsp")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "cfmleditor-lsp")
		}
		return filepath.Join(os.TempDir(), "cfmleditor-lsp")
	default:
		if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
			return filepath.Join(d, "cfmleditor-lsp")
		}
		return filepath.Join(os.TempDir(), "cfmleditor-lsp")
	}
}

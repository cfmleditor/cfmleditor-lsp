package daemon

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// configJSON is the on-disk shape of .cfmleditor.json.
type configJSON struct {
	WorkspaceName      string   `json:"workspaceName"`
	WorkspacePaths     []string `json:"workspacePaths"`
	WorkspaceIndexGlobs []string `json:"workspaceIndexGlobs"`
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
	for _, d := range []string{abs, filepath.Dir(abs)} {
		p := filepath.Join(d, ".cfmleditor.json")
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var raw configJSON
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		if raw.WorkspaceName == "" {
			continue
		}
		return &Config{Path: p, Name: raw.WorkspaceName}, nil
	}
	return nil, nil
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

// IndexGlobs returns the workspace index globs resolved to absolute paths by
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
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
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

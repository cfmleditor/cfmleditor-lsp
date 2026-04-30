package daemon

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// configJSON is the on-disk shape of .cfmleditor.json.
type configJSON struct {
	Name             string   `json:"name"`
	SharedIndexPaths []string `json:"sharedIndexPaths"`
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
		if raw.Name == "" {
			continue
		}
		return &Config{Path: p, Name: raw.Name}, nil
	}
	return nil, nil
}

// SocketPath returns a deterministic Unix socket path derived from the project name.
func (c *Config) SocketPath() string {
	h := sha256.Sum256([]byte(c.Name))
	name := fmt.Sprintf("cfmleditor-%x.sock", h[:8])
	return filepath.Join(socketDir(), name)
}

// SharedIndexPaths returns additional directories to index, resolved relative to the config file.
func (c *Config) SharedIndexPaths() []string {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return nil
	}
	var raw configJSON
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	dir := filepath.Dir(c.Path)
	out := make([]string, 0, len(raw.SharedIndexPaths))
	for _, p := range raw.SharedIndexPaths {
		out = append(out, filepath.Join(dir, p))
	}
	return out
}

func socketDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.TempDir(), "cfmleditor-lsp")
	case "windows":
		// AF_UNIX sockets supported on Windows 10 1803+.
		// Use LocalAppData for a per-user, non-volatile directory.
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

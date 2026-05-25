//go:build !wasm

package daemon

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SocketPath returns the Unix socket path for this project's daemon.
func (c *Config) SocketPath() string {
	h := sha256.Sum256([]byte(c.Name))
	name := fmt.Sprintf("cfmleditor-%x.sock", h[:8])

	return filepath.Join(socketDir(), name)
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

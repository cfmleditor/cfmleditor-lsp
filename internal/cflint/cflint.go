// Package cflint provides CFLint binary management and execution.
package cflint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.lsp.dev/protocol"
)

const (
	fallbackVersion = "1.5.10"
	releasesAPI     = "https://api.github.com/repos/cfmleditor/CFLint/releases/latest"
	downloadBase    = "https://github.com/cfmleditor/CFLint/releases/download/"
)

// Result represents CFLint JSON output.
type Result struct {
	Issues []Issue `json:"issues"`
}

// Issue represents a single CFLint issue.
type Issue struct {
	Severity  string     `json:"severity"`
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	Locations []Location `json:"locations"`
}

// Location represents where an issue occurs.
type Location struct {
	File     string `json:"file"`
	FileName string `json:"fileName"`
	Function string `json:"function"`
	Column   int    `json:"column"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Variable string `json:"variable"`
}

// Runner manages the CFLint binary.
type Runner struct {
	binPath string
}

// NewRunner creates a Runner, downloading the binary if needed.
func NewRunner() (*Runner, error) {
	binPath, err := ensureBinary()
	if err != nil {
		return nil, err
	}

	return &Runner{binPath: binPath}, nil
}

// Scan runs CFLint on the given file and returns LSP diagnostics.
func (r *Runner) Scan(ctx context.Context, filePath string) ([]protocol.Diagnostic, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "-stdin", filepath.Base(filePath), "-json", "-stdout", "-q")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	cmd.Stdin = strings.NewReader(string(content))
	cmd.Dir = filepath.Dir(filePath)

	var stderr strings.Builder

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if len(out) == 0 {
			return nil, fmt.Errorf("cflint failed: %w; stderr: %s", err, stderr.String())
		}
	}

	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing cflint output: %w\nstdout: %s\nstderr: %s", err, string(out), stderr.String())
	}

	return toDiagnostics(result), nil
}

func toDiagnostics(result Result) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	for _, issue := range result.Issues {
		sev := mapSeverity(issue.Severity)

		for _, loc := range issue.Locations {
			line := loc.Line
			if line > 0 {
				line--
			}

			col := loc.Column
			if col > 0 {
				col--
			}

			diags = append(diags, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(line), Character: uint32(col)},
					End:   protocol.Position{Line: uint32(line), Character: uint32(col)},
				},
				Severity: sev,
				Source:   "cflint",
				Code:     issue.ID,
				Message:  loc.Message,
			})
		}
	}

	return diags
}

func mapSeverity(s string) protocol.DiagnosticSeverity {
	switch strings.ToUpper(s) {
	case "ERROR", "FATAL":
		return protocol.DiagnosticSeverityError
	case "WARNING":
		return protocol.DiagnosticSeverityWarning
	case "INFO", "COSMETIC":
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityHint
	}
}

func binaryName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	switch {
	case os == "darwin" && arch == "arm64":
		return "cflint-macos-aarch64"
	case os == "linux" && arch == "arm64":
		return "cflint-linux-aarch64"
	case os == "linux" && arch == "amd64":
		return "cflint-linux-amd64"
	case os == "windows" && arch == "amd64":
		return "cflint-windows-amd64.exe"
	default:
		return ""
	}
}

func cacheDir(version string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	p := filepath.Join(dir, "cfmleditor-lsp", "cflint", version)

	return p, os.MkdirAll(p, 0o755)
}

func latestVersion() string {
	resp, err := http.Get(releasesAPI) //nolint:gosec // trusted URL
	if err != nil {
		return fallbackVersion
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fallbackVersion
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.TagName == "" {
		return fallbackVersion
	}

	return release.TagName
}

func ensureBinary() (string, error) {
	// Prefer a local binary on PATH
	if p, err := exec.LookPath("cflint"); err == nil {
		return p, nil
	}

	name := binaryName()
	if name == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	version := latestVersion()

	dir, err := cacheDir(version)
	if err != nil {
		return "", err
	}

	binPath := filepath.Join(dir, name)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	url := downloadBase + version + "/" + name

	resp, err := http.Get(url) //nolint:gosec // trusted URL
	if err != nil {
		return "", fmt.Errorf("downloading cflint: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading cflint: HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(binPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}

	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(binPath)

		return "", err
	}

	return binPath, nil
}

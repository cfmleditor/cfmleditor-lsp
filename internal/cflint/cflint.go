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
	"time"

	"go.lsp.dev/protocol"
)

const (
	// Only used when the releases API cannot be reached; the normal path
	// queries it and takes whatever is current. Worth refreshing occasionally
	// anyway, so an offline first run does not start several releases behind.
	fallbackVersion = "1.5.14"
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
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
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
				Source:   protocol.NewOptional("cflint"),
				Code:     protocol.String(issue.ID),
				Message:  protocol.String(loc.Message),
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

// downloadClient replaces http.DefaultClient, which has no timeout at all: a
// server that accepts the connection and then stalls leaves the goroutine — and
// the linting it was setting up — hung for the life of the process.
var downloadClient = &http.Client{Timeout: 5 * time.Minute}

func latestVersion() string {
	resp, err := downloadClient.Get(releasesAPI) //nolint:gosec // trusted URL
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

	resp, err := downloadClient.Get(url) //nolint:gosec // trusted URL
	if err != nil {
		return "", fmt.Errorf("downloading cflint: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading cflint: HTTP %d", resp.StatusCode)
	}

	// Download beside the target and rename into place. Writing binPath directly
	// meant an interruption — the process killed, the machine losing power, two
	// sessions downloading at once — left a truncated file at exactly the path
	// the Stat above accepts as a cached binary, so linting stayed broken for
	// every later run until someone deleted it by hand. Rename is atomic within
	// a directory, so binPath either does not exist or is a complete download.
	tmp, err := os.CreateTemp(dir, name+".part-*")
	if err != nil {
		return "", err
	}

	tmpPath := tmp.Name()

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath) // no-op once the rename below has succeeded
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", err
	}

	if err := tmp.Close(); err != nil {
		return "", err
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", err
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		return "", err
	}

	return binPath, nil
}

// Package cflint provides CFLint binary management and execution.
package cflint

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.lsp.dev/protocol"
)

const (
	// Only used when the releases API cannot be reached; the normal path
	// queries it and takes whatever is current. Worth refreshing occasionally
	// anyway, so an offline first run does not start several releases behind.
	fallbackVersion = "1.5.16"
	latestRelease   = "https://github.com/cfmleditor/CFLint/releases/latest"
)

// downloadBase is where release assets are fetched from. A var rather than a
// const so a test can point it somewhere that is not the internet.
var downloadBase = "https://github.com/cfmleditor/CFLint/releases/download/"

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

	// minRank is the least severe CFLint level still reported, as a rank from
	// severityRank. noSeverityFloor reports everything, which is the default.
	minRank int
}

// NewRunner creates a Runner, downloading the binary if needed.
//
// minSeverity is a CFLint level name (`linting.minSeverity`); issues less
// severe than it are dropped. An empty or unrecognised value reports
// everything — see MinSeverityRank, which is where a caller should check a
// configured value if it wants to warn about a typo.
func NewRunner(minSeverity string) (*Runner, error) {
	binPath, err := ensureBinary()
	if err != nil {
		return nil, err
	}

	rank, ok := MinSeverityRank(minSeverity)
	if !ok {
		rank = noSeverityFloor
	}

	return &Runner{binPath: binPath, minRank: rank}, nil
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

	return toDiagnostics(result, r.minRank), nil
}

func toDiagnostics(result Result, minRank int) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	eachDiagnostic(result, minRank, func(_ string, d protocol.Diagnostic) {
		diags = append(diags, d)
	})

	return diags
}

// eachDiagnostic calls fn with every issue location that meets the floor, and
// the file CFLint names for it.
//
// A line or column below 1 is taken as the start: CFLint reports a negative
// column for some tag-file issues, which cast to a uint32 put the range past
// the end of the line.
func eachDiagnostic(result Result, minRank int, fn func(file string, d protocol.Diagnostic)) {
	for _, issue := range result.Issues {
		if !meetsFloor(issue.Severity, minRank) {
			continue
		}

		sev := mapSeverity(issue.Severity)

		for i := range issue.Locations {
			loc := &issue.Locations[i]

			line := max(loc.Line-1, 0)
			col := max(loc.Column-1, 0)

			fn(loc.File, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(line), Character: uint32(col)}, //nolint:gosec // clamped to 0 above
					End:   protocol.Position{Line: uint32(line), Character: uint32(col)}, //nolint:gosec // as above
				},
				Severity: sev,
				Source:   protocol.NewOptional("cflint"),
				Code:     protocol.String(issue.ID),
				Message:  protocol.String(loc.Message),
			})
		}
	}
}

// filesPerRun is how many files one CFLint run for ScanFiles is given. The
// list goes on the command line, and Windows caps a command line at 32K
// characters.
const filesPerRun = 100

// ScanFiles runs CFLint over files, from disk, and returns each file's
// diagnostics keyed by its cleaned path, for a report over a whole project. A
// file CFLint finds nothing in has no key.
//
// Files are passed by name, several to a run and a few runs at once, rather
// than as a folder: a folder scan descends into every dot-directory, and a git
// worktree under .claude/ is a second copy of the project.
func (r *Runner) ScanFiles(ctx context.Context, files []string) (map[string][]protocol.Diagnostic, error) {
	out := make(map[string][]protocol.Diagnostic)

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)

	sem := make(chan struct{}, 4)

	for start := 0; start < len(files); start += filesPerRun {
		batch := files[start:min(start+filesPerRun, len(files))]

		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := r.run(ctx, "", "-file", strings.Join(batch, ","), "-json", "-stdout", "-q")

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if firstErr == nil {
					firstErr = err
				}

				return
			}

			eachDiagnostic(result, r.minRank, func(file string, d protocol.Diagnostic) {
				file = filepath.Clean(file)
				out[file] = append(out[file], d)
			})
		}()
	}

	wg.Wait()

	if firstErr == nil {
		firstErr = ctx.Err()
	}

	return out, firstErr
}

// run runs the binary with args, in dir when it is not empty, and decodes its
// JSON report.
func (r *Runner) run(ctx context.Context, dir string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, r.binPath, args...)
	cmd.Dir = dir

	var stderr strings.Builder

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return Result{}, fmt.Errorf("cflint failed: %w; stderr: %s", err, stderr.String())
	}

	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return Result{}, fmt.Errorf("parsing cflint output: %w\nstdout: %s\nstderr: %s", err, string(out), stderr.String())
	}

	return result, nil
}

// noSeverityFloor is the minRank that reports every issue, however trivial. It
// is deliberately larger than any rank in severityRank, so the comparison in
// meetsFloor needs no special case for "no floor configured".
const noSeverityFloor = 1 << 30

// severityRank orders CFLint's levels, most severe first. The order is
// com.cflint.Levels', which is the scale CFLint's own rule docs use, so
// `linting.minSeverity` is written in the vocabulary a user reads there rather
// than in LSP severities.
//
// The filter deliberately runs on this raw scale rather than on what
// mapSeverity returns. mapSeverity currently folds INFO and COSMETIC up onto
// Warning so they stay visible, which makes the mapped severities too coarse to
// filter on — a floor of WARNING applied after the fold would keep every INFO
// it was meant to drop.
var severityRank = map[string]int{
	"FATAL":    0,
	"CRITICAL": 1,
	"ERROR":    2,
	"WARNING":  3,
	"CAUTION":  4,
	"INFO":     5,
	"COSMETIC": 6,
}

// MinSeverityRank turns a configured `linting.minSeverity` into a rank for
// NewRunner, reporting whether the name was recognised. An empty string is not
// an error — it is the default, meaning no floor — but it is not recognised
// either, so a caller can tell "unset" from "set to something valid" and warn
// only about a genuine typo.
func MinSeverityRank(name string) (int, bool) {
	rank, ok := severityRank[strings.ToUpper(strings.TrimSpace(name))]

	return rank, ok
}

// meetsFloor reports whether an issue at the given CFLint level is severe
// enough to report.
//
// A severity CFLint's enum does not list — UNKNOWN, or a level added upstream
// — is always reported, whatever the floor. The alternative is that an
// unrecognised level is silently dropped by a setting that never mentioned it,
// which is the same disappearing-diagnostic failure mapSeverity's default arm
// exists to avoid.
func meetsFloor(severity string, minRank int) bool {
	rank, ok := MinSeverityRank(severity)
	if !ok {
		return true
	}

	return rank <= minRank
}

// mapSeverity folds CFLint's eight levels onto the LSP's four. The full set is
// com.cflint.Levels: FATAL, CRITICAL, ERROR, WARNING, CAUTION, INFO, COSMETIC,
// UNKNOWN.
//
// Nothing here may return Hint, and nothing may return Information: an editor
// shows neither by default. VS Code draws a Hint as a faint underline and keeps
// it out of the Problems panel entirely, and its Problems filter hides
// Information unless "Show Infos" is ticked -- so a CFLint issue mapped to
// either is reported by the server, counted in the "cflint scan complete" log
// line, and then invisible, which reads as a diagnostic that was never
// produced. Every level therefore lands on Error or Warning, including the
// default arm, so an unrecognised severity surfaces rather than disappearing.
//
// INFO and COSMETIC being Warnings is the interim part: they are advisory in
// CFLint's own scale, and the honest mapping is Information. Restoring that
// means changing the one arm below -- and is worth doing together with a
// config key, so the choice belongs to the workspace rather than to this
// switch.
func mapSeverity(s string) protocol.DiagnosticSeverity {
	switch strings.ToUpper(s) {
	case "ERROR", "FATAL", "CRITICAL":
		return protocol.DiagnosticSeverityError
	case "WARNING", "CAUTION":
		return protocol.DiagnosticSeverityWarning
	case "INFO", "COSMETIC":
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

func binaryName() string {
	return binaryNameFor(runtime.GOOS, runtime.GOARCH)
}

// binaryNameFor is binaryName with the platform passed in, so the mapping can
// be tested from a machine that is only ever one of them. An asset that is
// named wrong here is not a build error anywhere: it is an HTTP 404 at the
// first lint, on someone else's hardware.
func binaryNameFor(goos, goarch string) string {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return "cflint-macos-aarch64"
	case goos == "darwin" && goarch == "amd64":
		return "cflint-macos-amd64"
	case goos == "linux" && goarch == "arm64":
		return "cflint-linux-aarch64"
	case goos == "linux" && goarch == "amd64":
		return "cflint-linux-amd64"
	case goos == "windows" && goarch == "amd64":
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

// releaseClient reads a redirect rather than following it, which is what makes
// latestVersion work. Deliberately not downloadClient: an asset URL redirects
// to the object store, so downloads have to follow them.
var releaseClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func latestVersion() string {
	return latestVersionFrom(latestRelease)
}

// latestVersionFrom reads the tag `latest` points at, from GitHub's own
// redirect.
//
// Deliberately not the releases API. That endpoint is rate limited to 60
// requests an hour per IP for unauthenticated callers, shared by everyone
// behind one NAT, and an office that hits it spends the rest of the hour
// pinned to fallbackVersion with nothing saying why. `/releases/latest`
// redirects to `/releases/tag/<tag>` with no API involved and no limit.
func latestVersionFrom(url string) string {
	resp, err := releaseClient.Get(url) //nolint:gosec // trusted URL
	if err != nil {
		return fallbackVersion
	}

	defer func() { _ = resp.Body.Close() }()

	// The redirect is the answer, so the body is of no interest.
	_, _ = io.Copy(io.Discard, resp.Body)

	tag := tagFromRedirect(resp.Header.Get("Location"))
	if tag == "" {
		return fallbackVersion
	}

	return tag
}

// tagFromRedirect pulls the tag out of the URL `/releases/latest` redirects to,
// and returns an empty string for anything that is not one — an empty Location
// from a response that was not a redirect at all, or a redirect somewhere
// unexpected, both of which have to fall back rather than build a download URL
// out of nonsense.
func tagFromRedirect(location string) string {
	const marker = "/releases/tag/"

	at := strings.Index(location, marker)
	if at < 0 {
		return ""
	}

	tag := location[at+len(marker):]
	if end := strings.IndexAny(tag, "?#"); end >= 0 {
		tag = tag[:end]
	}

	unescaped, err := neturl.PathUnescape(tag)
	if err != nil {
		return tag
	}

	return unescaped
}

// assetKind is how a release asset has to be unpacked.
type assetKind int

const (
	// rawBinary is the executable itself, published with no container.
	rawBinary assetKind = iota
	// tarGz and zipped hold the executable under its plain name.
	tarGz
	zipped
)

// maxDownloadSize caps what is written out of an asset. CFLint is a GraalVM
// image, so ~90 MB is normal and this is generous — it is here so a corrupt or
// hostile archive cannot fill the disk while claiming to be a linter.
const maxDownloadSize = 512 << 20

// errAssetMissing is the one download failure worth trying another asset for.
var errAssetMissing = errors.New("asset not published for this release")

// asset is one candidate download for a platform's CFLint build.
type asset struct {
	name string
	kind assetKind
}

// assetsFor lists what to try, best first.
//
// The compressed assets are ~28 MB against ~90 MB for the raw binary — a
// GraalVM image is mostly zeroes and repeated metadata — so they are worth
// preferring for a download that happens in the middle of somebody's first
// edit. They are recent, though, and not every published build has one yet, so
// the raw binary stays as the fallback rather than the lint simply failing.
func assetsFor(goos, goarch string) []asset {
	name := binaryNameFor(goos, goarch)
	if name == "" {
		return nil
	}

	if goos == "windows" {
		return []asset{
			{name: strings.TrimSuffix(name, ".exe") + ".zip", kind: zipped},
			{name: name, kind: rawBinary},
		}
	}

	return []asset{
		{name: name + ".tar.gz", kind: tarGz},
		{name: name, kind: rawBinary},
	}
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

	return binaryWithFallback(latestVersion(), name)
}

// binaryWithFallback fetches the binary for a version, and settles for the
// version compiled in when that release has nothing for this platform.
//
// Two things make the newest release a version this platform may not be able
// to download from, and only the second is rare.
//
// A release's assets upload one platform at a time, so for the minutes between
// the release appearing and the last upload finishing, the newest version
// genuinely 404s for whichever platforms have not landed yet. macOS is
// routinely last. Any client that asks in that window sees exactly what a
// dropped platform looks like.
//
// And a release really can ship without one: 1.5.16 went out with no
// cflint-macos-amd64 at all, because the Intel job targeted a retired runner
// image, never got a runner, and was auto-cancelled while the other four
// uploaded and left the release looking complete. Upstream has since fixed the
// job and backfilled the asset, but nothing stops it happening again.
//
// The version in fallbackVersion is one this build is known to have been
// released against, so it is a better answer than an error in both cases.
//
// An earlier version of this comment cited 1.5.17 as having dropped macOS. It
// did not — that reading came from listing its assets while they were still
// uploading, which is the first case above rather than the second.
//
// Only a missing asset falls back. A refused connection or a timeout would
// fail the same way against any version, and trying a second one only doubles
// the wait.
func binaryWithFallback(version, name string) (string, error) {
	binPath, err := binaryForVersion(version, name)
	if err == nil {
		return binPath, nil
	}

	if !errors.Is(err, errAssetMissing) || version == fallbackVersion {
		return "", err
	}

	binPath, fallbackErr := binaryForVersion(fallbackVersion, name)
	if fallbackErr != nil {
		// The error from the version actually asked for is the one worth
		// reporting; the fallback failing too is a detail of the attempt.
		return "", err
	}

	return binPath, nil
}

// binaryForVersion returns the cached binary for one release, downloading it
// if this is the first time that version has been asked for.
func binaryForVersion(version, name string) (string, error) {
	dir, err := cacheDir(version)
	if err != nil {
		return "", err
	}

	binPath := filepath.Join(dir, name)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	assets := assetsFor(runtime.GOOS, runtime.GOARCH)
	if len(assets) == 0 {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var missing error

	for _, a := range assets {
		err := fetchAsset(downloadBase+version+"/"+a.name, binPath, a.kind)
		if err == nil {
			return binPath, nil
		}

		// Only a release that does not carry this asset is worth asking for
		// another one. A refused connection or a timeout would fail the same
		// way twice.
		if errors.Is(err, errAssetMissing) {
			missing = err

			continue
		}

		return "", err
	}

	return "", fmt.Errorf("cflint %s: %w", version, missing)
}

// fetchAsset downloads one asset and leaves the executable at binPath.
//
// Written beside the target and renamed into place. Writing binPath directly
// meant an interruption — the process killed, the machine losing power, two
// sessions downloading at once — left a truncated file at exactly the path
// ensureBinary accepts as a cached binary, so linting stayed broken for every
// later run until someone deleted it by hand. Rename is atomic within a
// directory, so binPath either does not exist or is a complete download.
func fetchAsset(url, binPath string, kind assetKind) error {
	resp, err := downloadClient.Get(url) //nolint:gosec // trusted URL
	if err != nil {
		return fmt.Errorf("downloading cflint: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", url, errAssetMissing)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading cflint: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Dir(binPath)

	tmp, err := os.CreateTemp(dir, filepath.Base(binPath)+".part-*")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath) // no-op once the rename below has succeeded
	}()

	if err := writeBinary(tmp, resp.Body, kind, dir); err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	return os.Rename(tmpPath, binPath)
}

// writeBinary copies the executable out of a downloaded asset.
func writeBinary(dst io.Writer, src io.Reader, kind assetKind, tmpDir string) error {
	switch kind {
	case rawBinary:
		return copyBinary(dst, src)
	case tarGz:
		return copyFromTarGz(dst, src)
	case zipped:
		return copyFromZip(dst, src, tmpDir)
	default:
		return fmt.Errorf("unknown asset kind %d", kind)
	}
}

// copyBinary copies up to maxDownloadSize, and treats reaching it as a failure
// rather than quietly installing a truncated executable.
func copyBinary(dst io.Writer, src io.Reader) error {
	written, err := io.Copy(dst, io.LimitReader(src, maxDownloadSize+1))
	if err != nil {
		return err
	}

	if written > maxDownloadSize {
		return fmt.Errorf("cflint is larger than %d bytes, refusing it", int64(maxDownloadSize))
	}

	return nil
}

// copyFromTarGz pulls the first regular file out of the archive. The release
// archives hold exactly one, the executable under its plain name, so this does
// not have to know what the platform calls it.
func copyFromTarGz(dst io.Writer, src io.Reader) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("reading cflint archive: %w", err)
	}

	defer func() { _ = gz.Close() }()

	archive := tar.NewReader(gz)

	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("cflint archive held no executable")
		}

		if err != nil {
			return fmt.Errorf("reading cflint archive: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			return copyBinary(dst, archive)
		}
	}
}

// copyFromZip does the same for a zip, which cannot be streamed: the central
// directory is at the end, so the archive is spooled to disk first.
func copyFromZip(dst io.Writer, src io.Reader, tmpDir string) error {
	spool, err := os.CreateTemp(tmpDir, "cflint-zip-*")
	if err != nil {
		return err
	}

	spoolPath := spool.Name()

	defer func() {
		_ = spool.Close()
		_ = os.Remove(spoolPath)
	}()

	size, err := io.Copy(spool, io.LimitReader(src, maxDownloadSize+1))
	if err != nil {
		return err
	}

	if size > maxDownloadSize {
		return fmt.Errorf("cflint archive is larger than %d bytes, refusing it", int64(maxDownloadSize))
	}

	archive, err := zip.NewReader(spool, size)
	if err != nil {
		return fmt.Errorf("reading cflint archive: %w", err)
	}

	var executable *zip.File

	for _, file := range archive.File {
		if !file.FileInfo().IsDir() {
			executable = file

			break
		}
	}

	if executable == nil {
		return errors.New("cflint archive held no executable")
	}

	entry, err := executable.Open()
	if err != nil {
		return fmt.Errorf("reading cflint archive: %w", err)
	}

	defer func() { _ = entry.Close() }()

	return copyBinary(dst, entry)
}

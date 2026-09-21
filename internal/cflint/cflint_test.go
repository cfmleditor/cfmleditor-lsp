package cflint

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.lsp.dev/protocol"
)

func TestMapSeverity(t *testing.T) {
	cases := map[string]protocol.DiagnosticSeverity{
		"ERROR":    protocol.DiagnosticSeverityError,
		"error":    protocol.DiagnosticSeverityError, // case-insensitive
		"FATAL":    protocol.DiagnosticSeverityError,
		"CRITICAL": protocol.DiagnosticSeverityError,
		"critical": protocol.DiagnosticSeverityError,
		"WARNING":  protocol.DiagnosticSeverityWarning,
		"CAUTION":  protocol.DiagnosticSeverityWarning,
		"caution":  protocol.DiagnosticSeverityWarning,
		"INFO":     protocol.DiagnosticSeverityWarning,
		"COSMETIC": protocol.DiagnosticSeverityWarning,
		"UNKNOWN":  protocol.DiagnosticSeverityWarning,
		"":         protocol.DiagnosticSeverityWarning,
	}

	for input, want := range cases {
		if got := mapSeverity(input); got != want {
			t.Errorf("mapSeverity(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestEveryCFLintLevelIsVisible states CFLint's severity enum in full and pins
// the rule that makes a CFLint issue reach the user at all: an editor shows
// neither Hint nor Information by default, so a level mapped to either is
// published, logged, and then invisible -- indistinguishable from a diagnostic
// that was never produced. The list is com.cflint.Levels, plus the empty string
// a missing severity arrives as.
func TestEveryCFLintLevelIsVisible(t *testing.T) {
	levels := []string{
		"FATAL", "CRITICAL", "ERROR", "WARNING", "CAUTION", "INFO", "COSMETIC", "UNKNOWN", "",
	}

	for _, level := range levels {
		got := mapSeverity(level)
		if got != protocol.DiagnosticSeverityError && got != protocol.DiagnosticSeverityWarning {
			t.Errorf("mapSeverity(%q) = %v; every level must map to Error or Warning, "+
				"since Hint and Information are hidden by default", level, got)
		}
	}
}

func TestToDiagnostics_LineAndColumnConvertedToZeroBased(t *testing.T) {
	result := Result{Issues: []Issue{
		{
			Severity: "WARNING",
			ID:       "W1",
			Locations: []Location{
				{Line: 10, Column: 5, Message: "trouble"},
			},
		},
	}}

	diags := toDiagnostics(result, noSeverityFloor)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}

	d := diags[0]
	if d.Range.Start.Line != 9 {
		t.Errorf("expected 1-based line 10 converted to 0-based 9, got %d", d.Range.Start.Line)
	}

	if d.Range.Start.Character != 4 {
		t.Errorf("expected 1-based column 5 converted to 0-based 4, got %d", d.Range.Start.Character)
	}

	if d.Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("expected warning severity, got %v", d.Severity)
	}
}

func TestToDiagnostics_ZeroLineAndColumnNotDecrementedBelowZero(t *testing.T) {
	// CFLint can report line/column as 0 for file-level issues; the -1 conversion must
	// guard against underflowing to -1 (which would wrap to a huge uint32).
	result := Result{Issues: []Issue{
		{Severity: "INFO", ID: "I1", Locations: []Location{{Line: 0, Column: 0, Message: "file level"}}},
	}}

	diags := toDiagnostics(result, noSeverityFloor)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}

	if diags[0].Range.Start.Line != 0 {
		t.Errorf("expected line 0 to stay 0, got %d", diags[0].Range.Start.Line)
	}

	if diags[0].Range.Start.Character != 0 {
		t.Errorf("expected column 0 to stay 0, got %d", diags[0].Range.Start.Character)
	}
}

func TestToDiagnostics_MultipleLocationsPerIssue(t *testing.T) {
	result := Result{Issues: []Issue{
		{
			Severity: "ERROR",
			ID:       "E1",
			Locations: []Location{
				{Line: 1, Column: 1, Message: "first"},
				{Line: 2, Column: 1, Message: "second"},
			},
		},
	}}

	diags := toDiagnostics(result, noSeverityFloor)
	if len(diags) != 2 {
		t.Fatalf("expected one diagnostic per location, got %d", len(diags))
	}

	if diags[0].Range.Start.Line != 0 || diags[1].Range.Start.Line != 1 {
		t.Errorf("expected locations to map independently, got lines %d and %d", diags[0].Range.Start.Line, diags[1].Range.Start.Line)
	}
}

func TestToDiagnostics_EmptyResult(t *testing.T) {
	if diags := toDiagnostics(Result{}, noSeverityFloor); len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty result, got %d", len(diags))
	}
}

// TestMinSeverityRankOrdersCFLintsScale pins the order itself. The ranks are
// only meaningful relative to each other, so the test compares neighbours
// rather than asserting the numbers, which are free to change.
func TestMinSeverityRankOrdersCFLintsScale(t *testing.T) {
	ordered := []string{"FATAL", "CRITICAL", "ERROR", "WARNING", "CAUTION", "INFO", "COSMETIC"}

	for i := range len(ordered) - 1 {
		more, ok := MinSeverityRank(ordered[i])
		if !ok {
			t.Fatalf("MinSeverityRank(%q) not recognised", ordered[i])
		}

		less, ok := MinSeverityRank(ordered[i+1])
		if !ok {
			t.Fatalf("MinSeverityRank(%q) not recognised", ordered[i+1])
		}

		if more >= less {
			t.Errorf("%s should outrank %s, got %d and %d", ordered[i], ordered[i+1], more, less)
		}
	}

	if _, ok := MinSeverityRank(""); ok {
		t.Error("an empty minSeverity must report false, so a caller can tell unset from valid")
	}

	if _, ok := MinSeverityRank("nonsense"); ok {
		t.Error("an unrecognised minSeverity must report false")
	}

	if _, ok := MinSeverityRank("  warning  "); !ok {
		t.Error("minSeverity should tolerate case and surrounding space")
	}
}

// TestMinSeverityDropsLessSevereIssues is the point of the setting: a floor of
// WARNING keeps FATAL through WARNING and drops CAUTION, INFO and COSMETIC.
func TestMinSeverityDropsLessSevereIssues(t *testing.T) {
	result := Result{Issues: []Issue{
		{Severity: "FATAL", ID: "F", Locations: []Location{{Line: 1, Message: "fatal"}}},
		{Severity: "CRITICAL", ID: "C", Locations: []Location{{Line: 2, Message: "critical"}}},
		{Severity: "ERROR", ID: "E", Locations: []Location{{Line: 3, Message: "error"}}},
		{Severity: "WARNING", ID: "W", Locations: []Location{{Line: 4, Message: "warning"}}},
		{Severity: "CAUTION", ID: "T", Locations: []Location{{Line: 5, Message: "caution"}}},
		{Severity: "INFO", ID: "I", Locations: []Location{{Line: 6, Message: "info"}}},
		{Severity: "COSMETIC", ID: "S", Locations: []Location{{Line: 7, Message: "cosmetic"}}},
	}}

	if diags := toDiagnostics(result, noSeverityFloor); len(diags) != 7 {
		t.Errorf("no floor should report every issue, got %d of 7", len(diags))
	}

	warning, _ := MinSeverityRank("WARNING")

	kept := toDiagnostics(result, warning)
	if len(kept) != 4 {
		t.Fatalf("floor of WARNING should keep 4 issues, got %d", len(kept))
	}

	for _, d := range kept {
		switch msg := fmt.Sprint(d.Message); msg {
		case "caution", "info", "cosmetic":
			t.Errorf("%q is below the WARNING floor and should have been dropped", msg)
		}
	}

	if only := toDiagnostics(result, 0); len(only) != 1 {
		t.Errorf("floor of FATAL should keep 1 issue, got %d", len(only))
	}
}

// TestMinSeverityKeepsUnrecognisedLevels guards the same disappearing-diagnostic
// failure mapSeverity's default arm does. A level CFLint's enum does not list is
// not something the user's floor ever ruled on, so silently dropping it would
// hide an issue behind a setting that never mentioned it.
func TestMinSeverityKeepsUnrecognisedLevels(t *testing.T) {
	result := Result{Issues: []Issue{
		{Severity: "UNKNOWN", ID: "U", Locations: []Location{{Line: 1, Message: "unknown"}}},
		{Severity: "", ID: "B", Locations: []Location{{Line: 2, Message: "blank"}}},
		{Severity: "COSMETIC", ID: "S", Locations: []Location{{Line: 3, Message: "cosmetic"}}},
	}}

	fatal, _ := MinSeverityRank("FATAL")

	kept := toDiagnostics(result, fatal)
	if len(kept) != 2 {
		t.Fatalf("unrecognised levels should survive the strictest floor, got %d of 2", len(kept))
	}

	for _, d := range kept {
		if fmt.Sprint(d.Message) == "cosmetic" {
			t.Error("COSMETIC is recognised and below the floor; it should have been dropped")
		}
	}
}

// TestBinaryNameForEveryPublishedPlatform pins the asset names against what
// cfmleditor/CFLint actually publishes. The mapping is easy to get wrong in a
// way nothing local catches: CFLint says "macos" where Go says "darwin", and
// "aarch64" where Go says "arm64", so a mistake compiles, passes review, and
// then 404s at the first lint on hardware the author does not have.
func TestBinaryNameForEveryPublishedPlatform(t *testing.T) {
	cases := map[[2]string]string{
		{"darwin", "arm64"}:  "cflint-macos-aarch64",
		{"darwin", "amd64"}:  "cflint-macos-amd64",
		{"linux", "arm64"}:   "cflint-linux-aarch64",
		{"linux", "amd64"}:   "cflint-linux-amd64",
		{"windows", "amd64"}: "cflint-windows-amd64.exe",
	}

	for platform, want := range cases {
		if got := binaryNameFor(platform[0], platform[1]); got != want {
			t.Errorf("binaryNameFor(%q, %q) = %q, want %q", platform[0], platform[1], got, want)
		}
	}
}

// A platform CFLint publishes nothing for has to come back empty rather than
// guess: ensureBinary turns that into "unsupported platform", which is the
// truth, where a guessed name would be an HTTP 404 that reads like the release
// is broken.
func TestBinaryNameForUnpublishedPlatform(t *testing.T) {
	for _, platform := range [][2]string{
		{"windows", "arm64"},
		{"linux", "386"},
		{"freebsd", "amd64"},
	} {
		if got := binaryNameFor(platform[0], platform[1]); got != "" {
			t.Errorf("binaryNameFor(%q, %q) = %q, want an empty string", platform[0], platform[1], got)
		}
	}
}

// TestAssetsForPrefersCompressed states the order the downloader asks in. The
// compressed assets are ~28 MB against ~90 MB, and they appeared only
// recently, so the raw binary has to stay reachable behind them.
func TestAssetsForPrefersCompressed(t *testing.T) {
	cases := map[[2]string][]asset{
		{"darwin", "amd64"}: {
			{name: "cflint-macos-amd64.tar.gz", kind: tarGz},
			{name: "cflint-macos-amd64", kind: rawBinary},
		},
		{"linux", "arm64"}: {
			{name: "cflint-linux-aarch64.tar.gz", kind: tarGz},
			{name: "cflint-linux-aarch64", kind: rawBinary},
		},
		// The zip holds `cflint.exe`, and the raw asset *is* `cflint.exe`, so
		// the archive name is the binary's without the extension.
		{"windows", "amd64"}: {
			{name: "cflint-windows-amd64.zip", kind: zipped},
			{name: "cflint-windows-amd64.exe", kind: rawBinary},
		},
	}

	for platform, want := range cases {
		got := assetsFor(platform[0], platform[1])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("assetsFor(%q, %q) = %v, want %v", platform[0], platform[1], got, want)
		}
	}

	if got := assetsFor("plan9", "amd64"); got != nil {
		t.Errorf("assetsFor on an unpublished platform = %v, want nil", got)
	}
}

// TestFetchAssetUnpacks covers the three shapes an asset arrives in. The
// executable has to end up at binPath whichever one it came from, because
// everything downstream only knows that path.
func TestFetchAssetUnpacks(t *testing.T) {
	want := []byte("#!/bin/sh\necho CFLint\n")

	cases := map[string]struct {
		body []byte
		kind assetKind
	}{
		"raw":    {body: want, kind: rawBinary},
		"tar.gz": {body: tarGzOf(t, "cflint", want), kind: tarGz},
		"zip":    {body: zipOf(t, "cflint.exe", want), kind: zipped},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()

			binPath := filepath.Join(t.TempDir(), "cflint-test")
			if err := fetchAsset(server.URL, binPath, tc.kind); err != nil {
				t.Fatalf("fetchAsset: %v", err)
			}

			got, err := os.ReadFile(binPath) //nolint:gosec // test temp dir
			if err != nil {
				t.Fatalf("reading the binary: %v", err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("binary = %q, want %q", got, want)
			}

			// Downloaded and unpacked is no use if it cannot be executed.
			info, err := os.Stat(binPath)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}

			if info.Mode().Perm()&0o111 == 0 {
				t.Errorf("mode = %v, want an executable", info.Mode().Perm())
			}
		})
	}
}

// A release without the compressed asset has to fall through to the raw
// binary rather than failing: 1.5.16 shipped without an arm64 macOS tar.gz,
// which is exactly this case on real hardware.
func TestFetchAssetReportsAMissingAssetSeparately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	err := fetchAsset(server.URL, filepath.Join(t.TempDir(), "cflint-test"), tarGz)
	if !errors.Is(err, errAssetMissing) {
		t.Errorf("fetchAsset on a 404 = %v, want errAssetMissing", err)
	}
}

// Any other failure is not worth retrying with a different asset, and must not
// be mistaken for one that is.
func TestFetchAssetDoesNotTreatOtherFailuresAsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := fetchAsset(server.URL, filepath.Join(t.TempDir(), "cflint-test"), rawBinary)
	if err == nil || errors.Is(err, errAssetMissing) {
		t.Errorf("fetchAsset on a 500 = %v, want a plain error", err)
	}
}

// An archive that unpacks to nothing must fail rather than leave an empty file
// where a linter should be.
func TestFetchAssetRejectsAnArchiveWithNoBinary(t *testing.T) {
	empty := tarGzOf(t, "", nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(empty)
	}))
	defer server.Close()

	binPath := filepath.Join(t.TempDir(), "cflint-test")
	if err := fetchAsset(server.URL, binPath, tarGz); err == nil {
		t.Error("fetchAsset on an empty archive = nil, want an error")
	}

	if _, err := os.Stat(binPath); err == nil {
		t.Error("a failed download left a file behind at the cached path")
	}
}

func tarGzOf(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	archive := tar.NewWriter(gz)

	if name != "" {
		header := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatalf("writing the tar header: %v", err)
		}

		if _, err := archive.Write(content); err != nil {
			t.Fatalf("writing the tar entry: %v", err)
		}
	}

	if err := archive.Close(); err != nil {
		t.Fatalf("closing the tar: %v", err)
	}

	if err := gz.Close(); err != nil {
		t.Fatalf("closing the gzip: %v", err)
	}

	return buf.Bytes()
}

func zipOf(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	archive := zip.NewWriter(&buf)

	entry, err := archive.Create(name)
	if err != nil {
		t.Fatalf("creating the zip entry: %v", err)
	}

	if _, err := entry.Write(content); err != nil {
		t.Fatalf("writing the zip entry: %v", err)
	}

	if err := archive.Close(); err != nil {
		t.Fatalf("closing the zip: %v", err)
	}

	return buf.Bytes()
}

// TestTagFromRedirect states the shapes GitHub's redirect arrives in, and the
// ones that have to be refused. A tag read out of a response that was not a
// redirect would become a download URL for a release that does not exist.
func TestTagFromRedirect(t *testing.T) {
	cases := map[string]string{
		"https://github.com/cfmleditor/CFLint/releases/tag/1.5.17":       "1.5.17",
		"https://github.com/cfmleditor/CFLint/releases/tag/1.5.17?x=1":   "1.5.17",
		"https://github.com/cfmleditor/CFLint/releases/tag/1.5.17#notes": "1.5.17",
		"https://github.com/cfmleditor/CFLint/releases/tag/v1.0%2Bbuild": "v1.0+build",
		"": "",
		"https://github.com/cfmleditor/CFLint/releases": "",
		"https://example.com/":                          "",
	}

	for location, want := range cases {
		if got := tagFromRedirect(location); got != want {
			t.Errorf("tagFromRedirect(%q) = %q, want %q", location, got, want)
		}
	}
}

// TestLatestVersionFrom covers the request itself: the redirect has to be read
// rather than followed, and anything else has to land on fallbackVersion
// rather than failing the lint.
func TestLatestVersionFrom(t *testing.T) {
	t.Run("reads the tag from the redirect", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "https://github.com/cfmleditor/CFLint/releases/tag/1.5.17")
			w.WriteHeader(http.StatusFound)
		}))
		defer server.Close()

		if got := latestVersionFrom(server.URL); got != "1.5.17" {
			t.Errorf("latestVersionFrom = %q, want 1.5.17", got)
		}
	})

	// Rate limiting used to arrive as an HTTP 403 from the API. Whatever the
	// shape, an answer that is not a redirect to a tag means carrying on with
	// the version that is compiled in.
	t.Run("falls back when the answer is not a redirect", func(t *testing.T) {
		for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))

			got := latestVersionFrom(server.URL)
			server.Close()

			if got != fallbackVersion {
				t.Errorf("latestVersionFrom on HTTP %d = %q, want %q", status, got, fallbackVersion)
			}
		}
	})

	t.Run("falls back when nobody answers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := server.URL
		server.Close()

		if got := latestVersionFrom(url); got != fallbackVersion {
			t.Errorf("latestVersionFrom against a closed server = %q, want %q", got, fallbackVersion)
		}
	})
}

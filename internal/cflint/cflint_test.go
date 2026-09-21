package cflint

import (
	"fmt"
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

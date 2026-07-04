package cflint

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestMapSeverity(t *testing.T) {
	cases := map[string]protocol.DiagnosticSeverity{
		"ERROR":    protocol.DiagnosticSeverityError,
		"error":    protocol.DiagnosticSeverityError, // case-insensitive
		"FATAL":    protocol.DiagnosticSeverityError,
		"WARNING":  protocol.DiagnosticSeverityWarning,
		"INFO":     protocol.DiagnosticSeverityInformation,
		"COSMETIC": protocol.DiagnosticSeverityInformation,
		"UNKNOWN":  protocol.DiagnosticSeverityHint,
		"":         protocol.DiagnosticSeverityHint,
	}

	for input, want := range cases {
		if got := mapSeverity(input); got != want {
			t.Errorf("mapSeverity(%q) = %v, want %v", input, got, want)
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

	diags := toDiagnostics(result)
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

	diags := toDiagnostics(result)
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

	diags := toDiagnostics(result)
	if len(diags) != 2 {
		t.Fatalf("expected one diagnostic per location, got %d", len(diags))
	}

	if diags[0].Range.Start.Line != 0 || diags[1].Range.Start.Line != 1 {
		t.Errorf("expected locations to map independently, got lines %d and %d", diags[0].Range.Start.Line, diags[1].Range.Start.Line)
	}
}

func TestToDiagnostics_EmptyResult(t *testing.T) {
	if diags := toDiagnostics(Result{}); len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty result, got %d", len(diags))
	}
}

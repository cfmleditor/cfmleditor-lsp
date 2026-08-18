package formatter

import (
	"strings"
	"testing"
)

// TestGuardAllowsNormalizationInsertions covers the canonicalisation the
// formatter performs deliberately: optional semicolons and braces around
// single-statement bodies. These are insertions on the output side only.
func TestGuardAllowsNormalizationInsertions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{"semicolon added", "var x = 1", "var x = 1;"},
		{"braces added", "if (x) return 1;", "if (x) { return 1; }"},
		{"both", "if (x) return 1", "if (x) {\n\treturn 1;\n}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true); err != nil {
				t.Errorf("guard rejected a deliberate normalisation: %v", err)
			}
		})
	}
}

// TestGuardStillCatchesRealChanges pins down what the widened allowance must
// not swallow. Every one of these is a defect the guard found in real code.
func TestGuardStillCatchesRealChanges(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{"dropped return type", "public query function f() {}", "public function f() {}"},
		{"dropped catch type", "catch (any e) { x(); }", "catch (e) { x(); }"},
		{"dropped catch clause", "catch (A e) { y(); } catch (B e) { z(); }", "catch (A e) { y(); }"},
		{"interface rewritten", "interface { }", "component { }"},
		{"static access rewritten", "Widget::getData()", "Widget.getData()"},
		{"BOM dropped", "\ufeffcomponent {}", "component {}"},
		{"invented closing tag", "<cfmodule template=\"a\">", "<cfmodule template=\"a\"></cfmodule>"},
		{"comment swallowed in literal", "[\n// note\na, b]", "[// note, a, b]"},
		{"dropped semicolon", "var x = 1;", "var x = 1"},
		{"dropped brace", "if (x) { return 1; }", "if (x) return 1;"},
		{"unmatched brace added", "f()", "f() }"},
		{"identifier changed", "var total = 1", "var totl = 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true); err == nil {
				t.Errorf("guard accepted a real change: %q -> %q", tc.src, tc.out)
			}
		})
	}
}

// TestGuardBraceBalance checks the counting half of the allowance: a matched
// pair is fine, a stray brace is not.
func TestGuardBraceBalance(t *testing.T) {
	if err := checkWhitespaceOnly([]byte("if (x) f();"), []byte("if (x) { f(); }"), true); err != nil {
		t.Errorf("matched pair rejected: %v", err)
	}

	err := checkWhitespaceOnly([]byte("if (x) f();"), []byte("if (x) { f();"), true)
	if err == nil {
		t.Error("unmatched opening brace accepted")
	} else if !strings.Contains(err.Error(), "unmatched") {
		t.Errorf("expected an unmatched-brace error, got %v", err)
	}
}

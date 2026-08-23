package formatter

import (
	"bytes"
	"testing"
)

// scriptRegionsOf finds tag offsets in a case-folded copy and uses them to
// slice the original. bytes.ToLower folds by Unicode rune, and some runes
// change byte length doing so — "İ" (U+0130) is two bytes and lowercases to
// one — so a single such character before a <cfscript> shifted every later
// span and the whitespace-only guard compared the wrong regions.
func TestScriptRegionsSurviveLengthChangingUnicode(t *testing.T) {
	if got, want := len(bytes.ToLower([]byte("İ"))), len("İ"); got == want {
		t.Skip("bytes.ToLower no longer changes length for this input")
	}

	src := []byte("<p>İİİ</p>\n<cfscript>\nx = 1;\n</cfscript>\n")

	spans := scriptRegionsOf(src)
	if len(spans) != 1 {
		t.Fatalf("expected one script span, got %d", len(spans))
	}

	got := string(src[spans[0].start:spans[0].end])
	if !bytes.Contains([]byte(got), []byte("x = 1;")) {
		t.Errorf("span does not cover the script body; got %q", got)
	}

	if bytes.Contains([]byte(got), []byte("<p>")) {
		t.Errorf("span leaked into the markup before it; got %q", got)
	}
}

// The ASCII fold must still find tags written in any case.
func TestScriptRegionsAreCaseInsensitive(t *testing.T) {
	for _, src := range []string{
		"<CFSCRIPT>\nx = 1;\n</CFSCRIPT>\n",
		"<CfScript>\nx = 1;\n</cfscript>\n",
		"<cfscript>\nx = 1;\n</CFSCRIPT>\n",
	} {
		spans := scriptRegionsOf([]byte(src))
		if len(spans) != 1 {
			t.Errorf("%q: expected one span, got %d", src, len(spans))
		}
	}
}

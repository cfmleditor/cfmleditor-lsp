package parser

import (
	"math/rand"
	"strings"
	"testing"
)

// indexCFTagSlow and buildLineIdxSlow are the implementations these two
// replaced, kept as the thing the fast versions are checked against.
//
// A rewrite for speed is the kind that changes an answer by one byte at an edge
// nobody writes a fixture for — the end of a file, an empty needle, a '<' with
// nothing after it — so the test is a differential one over generated input
// rather than a handful of cases chosen by whoever did the rewrite.

func indexCFTagSlow(s, suffix string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1+len(suffix) <= len(s) && strings.EqualFold(s[i+1:i+1+len(suffix)], suffix) {
			return i
		}
	}

	return -1
}

func buildLineIdxSlow(src string) []int {
	n := 1

	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			n++
		}
	}

	idx := make([]int, 1, n)

	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			idx = append(idx, i+1)
		}
	}

	return idx
}

func TestIndexCFTagMatchesTheScanItReplaced(t *testing.T) {
	fixed := []struct{ s, suffix string }{
		{"", "cfset"},
		{"<", "cfset"},
		{"<c", "cfset"},
		{"<cfset", "cfset"},
		{"<cfset>", "cfset"},
		{"<CFSET>", "cfset"},
		{"no tags here", "cfset"},
		{"<a><b><cfset>", "cfset"},
		{"<<<<cfset", "cfset"},
		{"<cfsettings><cfset>", "cfset"},
		{"anything", ""},
		{"<x", ""},
		{"", ""},
		{"<cfİ>", "cfi"},
		{"text<", "c"},
	}

	for _, c := range fixed {
		if got, want := indexCFTag(c.s, c.suffix), indexCFTagSlow(c.s, c.suffix); got != want {
			t.Errorf("indexCFTag(%q, %q) = %d, want %d", c.s, c.suffix, got, want)
		}
	}

	rng := rand.New(rand.NewSource(1))
	alphabet := []byte("<>cfsetCFSET /\"\n\ta")
	needles := []string{"cfset", "cffunction", "/cffunction", "c", "cf", "", "cfscript>"}

	for range 4000 {
		var b strings.Builder
		for range rng.Intn(60) {
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}

		s := b.String()
		suffix := needles[rng.Intn(len(needles))]

		if got, want := indexCFTag(s, suffix), indexCFTagSlow(s, suffix); got != want {
			t.Fatalf("indexCFTag(%q, %q) = %d, want %d", s, suffix, got, want)
		}
	}
}

func TestBuildLineIdxMatchesTheScanItReplaced(t *testing.T) {
	fixed := []string{
		"",
		"\n",
		"\n\n\n",
		"no newline",
		"trailing\n",
		"\nleading",
		"a\nb\nc",
		"a\r\nb\r\n",
	}

	for _, src := range fixed {
		assertSameInts(t, src, buildLineIdx(src), buildLineIdxSlow(src))
	}

	rng := rand.New(rand.NewSource(2))
	alphabet := []byte("ab\n\n \t")

	for range 2000 {
		var b strings.Builder
		for range rng.Intn(80) {
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}

		src := b.String()
		assertSameInts(t, src, buildLineIdx(src), buildLineIdxSlow(src))
	}
}

func assertSameInts(t *testing.T, src string, got, want []int) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("buildLineIdx(%q) has %d entries, want %d", src, len(got), len(want))
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("buildLineIdx(%q)[%d] = %d, want %d", src, i, got[i], want[i])
		}
	}
}

func hasScriptTagSlow(content string) bool {
	for i := 0; i+7 < len(content); i++ {
		if content[i] == '<' && strings.EqualFold(content[i+1:i+7], "script") {
			switch content[i+7] {
			case '>', '/', ' ', '\t', '\n', '\r':
				return true
			}
		}
	}

	return false
}

func containsCFTagSlow(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '<' && toLowerByte(s[i+1]) == 'c' && toLowerByte(s[i+2]) == 'f' {
			return true
		}
	}

	return false
}

func TestScanPredicatesMatchTheLoopsTheyReplaced(t *testing.T) {
	fixed := []string{
		"", "<", "<s", "<script", "<script>", "<SCRIPT >", "<scripting>",
		"<script\n", "<script\t", "<script/", "<scriptx>", "text<script",
		"<cf", "<CF", "<c", "<x><cfset>", "a<b<cfif>", "<<cf", "<cf>",
	}

	for _, c := range fixed {
		if got, want := hasScriptTag(c), hasScriptTagSlow(c); got != want {
			t.Errorf("hasScriptTag(%q) = %v, want %v", c, got, want)
		}

		if got, want := containsCFTag(c), containsCFTagSlow(c); got != want {
			t.Errorf("containsCFTag(%q) = %v, want %v", c, got, want)
		}
	}

	rng := rand.New(rand.NewSource(3))
	alphabet := []byte("<>scriptSCRIPTcf /\n\t")

	for range 6000 {
		var b strings.Builder
		for range rng.Intn(40) {
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}

		c := b.String()

		if got, want := hasScriptTag(c), hasScriptTagSlow(c); got != want {
			t.Fatalf("hasScriptTag(%q) = %v, want %v", c, got, want)
		}

		if got, want := containsCFTag(c), containsCFTagSlow(c); got != want {
			t.Fatalf("containsCFTag(%q) = %v, want %v", c, got, want)
		}
	}
}

func BenchmarkIndexCFTag(b *testing.B) {
	src := strings.Repeat(`<div class="x"><span>text</span></div>`, 400) + "<cffunction name=\"x\">"

	b.ReportAllocs()

	for b.Loop() {
		_ = indexCFTag(src, "cffunction")
	}
}

func BenchmarkBuildLineIdx(b *testing.B) {
	src := strings.Repeat("<cfset x = 1>\n", 2000)

	b.ReportAllocs()

	for b.Loop() {
		_ = buildLineIdx(src)
	}
}

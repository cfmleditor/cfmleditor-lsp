package server

import (
	"math/rand"
	"strings"
	"testing"
)

// identSpanFrom skips to the positions a case-insensitive match can begin at
// instead of asking EqualFold about every byte. These check it against the
// definition it is a faster spelling of: the same scan with nothing skipped.
func identSpanNaive(text, name string, from int) (start, end int, ok bool) {
	if name == "" {
		return 0, 0, false
	}

	for i := max(from, 0); i+len(name) <= len(text); i++ {
		if !strings.EqualFold(text[i:i+len(name)], name) {
			continue
		}

		if isIdentByte(byteAt(text, i-1)) || isIdentByte(byteAt(text, i+len(name))) {
			continue
		}

		return i, i + len(name), true
	}

	return 0, 0, false
}

var identSpanCases = []struct {
	name       string
	text, want string
}{
	{"plain", "var getData = 1;", "getData"},
	{"case differs", "var GETDATA = 1;", "getData"},
	{"longer identifier must not match", "getDataSet(); getData();", "getData"},
	{"prefixed identifier must not match", "xgetData; getData;", "getData"},
	{"underscore first byte", "_helper(); x_helper();", "_helper"},
	{"digit first byte", "a 1st 1st", "1st"},
	{"name absent", "nothing here at all", "getData"},
	{"first letter everywhere, no match", strings.Repeat("g ", 400), "getData"},
	{"first letter everywhere, match at end", strings.Repeat("g ", 400) + "getData", "getData"},
	{"uppercase first letter only", strings.Repeat("G ", 400) + "GETDATA", "getData"},
	{"name longer than text", "ab", "abcdef"},
	{"empty text", "", "getData"},
	{"non-ascii first byte", "café(); CAFÉ();", "café"},
	{"non-ascii in text", "naïve getData naïve", "getData"},
	{"adjacent matches", "getData getData getData", "getData"},
	{"match at position zero", "getData()", "getData"},
	{"single byte name", "a b a", "a"},
}

func TestIdentSpanFromMatchesTheUnskippedScan(t *testing.T) {
	for _, tc := range identSpanCases {
		t.Run(tc.name, func(t *testing.T) {
			// Every starting offset, including ones past the end and below
			// zero, since documentHighlight resumes at the end of each match.
			for from := -2; from <= len(tc.text)+2; from++ {
				ws, we, wok := identSpanNaive(tc.text, tc.want, from)

				gs, ge, gok := identSpanFrom(tc.text, tc.want, from)
				if gs != ws || ge != we || gok != wok {
					t.Fatalf("identSpanFrom(%q, %q, %d) = (%d, %d, %v), want (%d, %d, %v)",
						tc.text, tc.want, from, gs, ge, gok, ws, we, wok)
				}
			}
		})
	}
}

// The same property against text and names no hand-written case would produce,
// over an alphabet small enough that matches and near-misses are common.
func TestIdentSpanFromMatchesTheUnskippedScanOnRandomInput(t *testing.T) {
	const alphabet = "aAbB_1 .é"

	rnd := rand.New(rand.NewSource(1))

	build := func(n int) string {
		var b strings.Builder

		for range n {
			r := []rune(alphabet)[rnd.Intn(len([]rune(alphabet)))]
			b.WriteRune(r)
		}

		return b.String()
	}

	for range 3000 {
		text := build(rnd.Intn(40))
		name := build(1 + rnd.Intn(4))
		from := rnd.Intn(len(text) + 3)

		ws, we, wok := identSpanNaive(text, name, from)

		gs, ge, gok := identSpanFrom(text, name, from)
		if gs != ws || ge != we || gok != wok {
			t.Fatalf("identSpanFrom(%q, %q, %d) = (%d, %d, %v), want (%d, %d, %v)",
				text, name, from, gs, ge, gok, ws, we, wok)
		}
	}
}

// TestIdentSpanFromScalesLinearly is the guard on how the skipping is spelled
// rather than on whether it happens.
//
// The natural spelling — IndexByte for each case of the first byte, take the
// lower of the two — is quadratic on exactly the input this request sees most.
// A document is usually full of a letter in one case and has none of it in the
// other, and the absent case's IndexByte scans to the end of the text every
// time it is asked, once per candidate position. Both answers are correct, so
// only a scaling test separates them: with the cursors dropped and each case
// re-searched from scratch this reports about 8x, against the 1.1x below.
func TestIdentSpanFromScalesLinearly(t *testing.T) {
	// Full of lowercase g, with no capital G anywhere and no match to find, so
	// every position is a candidate and none of them ends the scan.
	build := func(lines int) string {
		return strings.Repeat("getting going again; gathering gulps;\n", lines)
	}

	measure := func(lines int) float64 {
		text := build(lines)

		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				if _, _, ok := identSpanFrom(text, "gxyzzy", 0); ok {
					b.Fatal("unexpected match")
				}
			}
		})

		return float64(res.NsPerOp()) / float64(len(text))
	}

	small := measure(200)
	large := measure(1600)

	t.Logf("ns per byte of text: 200 lines %.3f, 1600 lines %.3f (ratio %.2f)", small, large, large/small)

	if small == 0 {
		t.Fatal("measured nothing — the benchmark did not run")
	}

	// Eight times the input. Linear work is flat per byte; a rescan per
	// candidate lands near 8x. The bound is on the per-byte cost, so a busy
	// runner moves both numbers together.
	if ratio := large / small; ratio > 3 {
		t.Errorf("identSpanFrom costs %.3fns/byte at 200 lines and %.3fns/byte at 1600 (ratio %.2f, want <= 3) "+
			"— the scan is being restarted per candidate position", small, large, ratio)
	}
}

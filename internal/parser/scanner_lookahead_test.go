package parser

import (
	"math/rand"
	"testing"
)

// The scanner caches one token: PeekSkipComments scans it and the matching
// NextSkipComments takes it without scanning again. Every parser in this
// package reaches the scanner through that pair, so a cache that is wrong about
// position, line, or LastBlockComment is wrong about every parse.
//
// These are differential tests rather than expected-token tables. The property
// that matters is not what any one token is, it is that peeking cannot change
// what the scanner goes on to produce — and a table of expected tokens states
// the tokens, not that property.

// scanSteps is what a driver produces: the token stream it saw, and
// LastBlockComment as it stood after each token.
type scanStep struct {
	tok     Token
	comment string
}

// drive takes tokens, peeking first at the positions peekAt reports true for.
func drive(src string, peekAt func(i int) bool, clearAt func(i int) bool) []scanStep {
	sc := NewScanner(src)

	var steps []scanStep

	for i := 0; ; i++ {
		if peekAt(i) {
			// A peek may be repeated; a second one must answer like the first.
			if got, again := sc.PeekSkipComments(), sc.PeekSkipComments(); got != again {
				panic("repeated peek disagreed with itself")
			}
		}

		// parseFunction reads and clears LastBlockComment between the peek that
		// found the `function` keyword and the next that takes the name, so the
		// clear has to be part of the sequences under test.
		if clearAt(i) {
			sc.LastBlockComment = ""
		}

		tok := sc.NextSkipComments()
		steps = append(steps, scanStep{tok: tok, comment: sc.LastBlockComment})

		if tok.Kind == TokEOF {
			return steps
		}
	}
}

func never(int) bool  { return false }
func always(int) bool { return true }

// lookaheadSources cover what the cache has to survive: block comments (which
// assign LastBlockComment), the other comment kinds and newlines (which are
// skipped but move the line counter), strings holding comment-like bytes, and
// two identical block comments in a row — the case a value comparison, rather
// than a record of whether the scan assigned at all, would get wrong.
var lookaheadSources = []struct {
	name string
	src  string
}{
	{"plain", "component extends=\"base.Thing\" {\n\tvar x = 1;\n}"},
	{"block comment", "/** doc */\nfunction f() {\n\treturn 1;\n}"},
	{"two identical block comments", "/*d*/ a /*d*/ b"},
	{"differing block comments", "/*one*/ a /*two*/ b"},
	{"line and cf comments", "// note\na <!--- tag note ---> b\n// tail"},
	{"comment-like bytes in strings", "x = \"/* not a comment */\"; y = '<!--- nor this --->';"},
	{"nested cf comment", "a <!--- outer <!--- inner ---> ---> b"},
	{"unterminated block comment", "a /* runs to the end"},
	{"crlf", "a = 1;\r\n/*c*/\r\nb = 2;\r\n"},
	{"empty", ""},
	{"only comments", "/*a*/ // b\n<!--- c --->"},
}

func TestPeekDoesNotChangeTheTokenStream(t *testing.T) {
	for _, src := range lookaheadSources {
		t.Run(src.name, func(t *testing.T) {
			for _, clearing := range []struct {
				name  string
				clear func(int) bool
			}{
				{"no clear", never},
				{"clearing every step", always},
				{"clearing every third step", func(i int) bool { return i%3 == 0 }},
			} {
				t.Run(clearing.name, func(t *testing.T) {
					want := drive(src.src, never, clearing.clear)

					for _, peeking := range []struct {
						name string
						peek func(int) bool
					}{
						{"peeking every step", always},
						{"peeking every other step", func(i int) bool { return i%2 == 0 }},
						{"peeking every third step", func(i int) bool { return i%3 == 0 }},
					} {
						t.Run(peeking.name, func(t *testing.T) {
							got := drive(src.src, peeking.peek, clearing.clear)
							compareSteps(t, want, got)
						})
					}
				})
			}
		})
	}
}

// TestPeekDoesNotChangeTheTokenStreamUnderRandomSequences is the same property
// against peek and clear positions no hand-written pattern would line up on.
func TestPeekDoesNotChangeTheTokenStreamUnderRandomSequences(t *testing.T) {
	for _, src := range lookaheadSources {
		t.Run(src.name, func(t *testing.T) {
			for seed := range int64(50) {
				rnd := rand.New(rand.NewSource(seed))

				// Decided up front and replayed, so the reference run and the
				// peeking run clear at exactly the same steps.
				clears := map[int]bool{}
				peeks := map[int]bool{}

				for i := range 256 {
					clears[i] = rnd.Intn(4) == 0
					peeks[i] = rnd.Intn(2) == 0
				}

				clearAt := func(i int) bool { return clears[i] }

				want := drive(src.src, never, clearAt)
				got := drive(src.src, func(i int) bool { return peeks[i] }, clearAt)

				if t.Failed() {
					return
				}

				compareSteps(t, want, got)
			}
		})
	}
}

func compareSteps(t *testing.T, want, got []scanStep) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("token count = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].tok != want[i].tok {
			t.Errorf("token %d = %+v, want %+v", i, got[i].tok, want[i].tok)
		}

		if got[i].comment != want[i].comment {
			t.Errorf("LastBlockComment after token %d = %q, want %q", i, got[i].comment, want[i].comment)
		}
	}
}

// TestRestoreDropsTheLookahead pins the one place the cache can outlive the
// position it describes. Restore moves the scanner somewhere a peeked token no
// longer sits in front of, so the cached token has to go with it.
func TestRestoreDropsTheLookahead(t *testing.T) {
	sc := NewScanner("alpha beta gamma")

	start := sc.Save()

	if got := sc.PeekSkipComments(); got.Value != "alpha" {
		t.Fatalf("peek = %q, want alpha", got.Value)
	}

	if got := sc.NextSkipComments(); got.Value != "alpha" {
		t.Fatalf("next = %q, want alpha", got.Value)
	}

	// Peek past the restore point, then go back. The stale token is "beta"; the
	// scanner is now at the start and owes "alpha".
	if got := sc.PeekSkipComments(); got.Value != "beta" {
		t.Fatalf("peek = %q, want beta", got.Value)
	}

	sc.Restore(start)

	if got := sc.NextSkipComments(); got.Value != "alpha" {
		t.Errorf("after Restore, next = %q, want alpha — the lookahead outlived the position", got.Value)
	}
}

// TestNextDropsTheLookahead covers the other direction. The raw Next returns
// comments and newlines, so it cannot hand back the token a skipping peek
// cached; it scans from the position the peek left alone and consumes that
// token itself. A cache left standing then replays the same token and jumps the
// scanner back over the bytes Next had already moved past — so the token comes
// out twice and the one after it is lost.
func TestNextDropsTheLookahead(t *testing.T) {
	sc := NewScanner("alpha beta")

	if got := sc.PeekSkipComments(); got.Value != "alpha" {
		t.Fatalf("peek = %q, want alpha", got.Value)
	}

	if got := sc.Next(); got.Value != "alpha" {
		t.Fatalf("raw Next = %q, want alpha", got.Value)
	}

	if got := sc.NextSkipComments(); got.Value != "beta" {
		t.Errorf("next = %q, want beta — the lookahead survived the raw Next and repeated its token", got.Value)
	}
}

// TestPeekKeepsTheLookahead is the companion: the raw Peek does not advance, so
// the cached token is still the one in front of the scanner afterwards and
// dropping it would only cost a re-scan.
func TestPeekKeepsTheLookahead(t *testing.T) {
	sc := NewScanner("\n\nalpha beta")

	if got := sc.PeekSkipComments(); got.Value != "alpha" {
		t.Fatalf("peek = %q, want alpha", got.Value)
	}

	if got := sc.Peek(); got.Kind != TokNewline {
		t.Fatalf("raw Peek = %+v, want a newline", got)
	}

	if got := sc.NextSkipComments(); got.Value != "alpha" {
		t.Errorf("next = %q, want alpha", got.Value)
	}

	if got := sc.NextSkipComments(); got.Value != "beta" {
		t.Errorf("next = %q, want beta", got.Value)
	}
}

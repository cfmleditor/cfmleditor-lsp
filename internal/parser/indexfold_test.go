package parser

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// indexFoldFromByteLoop is indexFoldFrom as it was before it searched with
// IndexByte: every start tested in turn. The two must agree on every input.
func indexFoldFromByteLoop(s, substr string, from int) int {
	if substr == "" {
		return max(from, 0)
	}

	first := lowerASCII(substr[0])

	for i := max(from, 0); i+len(substr) <= len(s); i++ {
		if lowerASCII(s[i]) != first {
			continue
		}

		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}

	return -1
}

func TestIndexFoldFromMatchesTheByteLoop(t *testing.T) {
	alphabet := []string{"a", "A", "b", "B", "g", "G", "e", "E", "t", "T", "u", "U", "q", "$", ".", "(", " ", "é", "É", "_", "1", "k", "K", "s", "\u212a", "\u017f"}
	rng := rand.New(rand.NewPCG(1, 2))

	word := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(alphabet[rng.IntN(len(alphabet))])
		}

		return b.String()
	}

	for range 200000 {
		s, sub := word(rng.IntN(60)), word(rng.IntN(9))
		from := rng.IntN(len(s)+3) - 1

		if got, want := indexFoldFrom(s, sub, from), indexFoldFromByteLoop(s, sub, from); got != want {
			t.Fatalf("indexFoldFrom(%q, %q, %d) = %d, byte loop %d", s, sub, from, got, want)
		}
	}
}

func BenchmarkIndexFold(b *testing.B) {
	hay := strings.Repeat("<cfset variables.thing = someObject.doSomething( arg1, arg2 )>\n", 2000) + "getInclude"

	b.Run("rarestByte", func(b *testing.B) {
		for b.Loop() {
			_ = indexFoldFrom(hay, "include", 0)
		}
	})
	b.Run("byteLoop", func(b *testing.B) {
		for b.Loop() {
			_ = indexFoldFromByteLoop(hay, "include", 0)
		}
	})
}

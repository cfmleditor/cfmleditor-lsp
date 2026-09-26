package parser

import (
	"strings"
	"testing"
)

// buildChain is what every call site does: reset to the first identifier, then
// append a dot and an identifier per hop, then take the string.
func buildChain(segments int) string {
	var c chainBuilder

	c.reset("svc")

	for range segments - 1 {
		c.writeDot()
		c.writeString("repo")
	}

	return c.String()
}

// A chain that fits the builder's array costs exactly one allocation, whatever
// its length: the string it returns, which the caller keeps.
//
// strings.Builder starts from nothing and doubles, so it allocated at each
// growth on the way to a string it then had to build anyway: measured, 2 for
// the ordinary `a.b.c` and 4 by eight segments. The count here is flat because
// the array absorbs the growth.
func TestChainBuilderAllocatesOnceForAChainThatFits(t *testing.T) {
	for _, segments := range []int{2, 3, 5, 8, 12} {
		got := testing.AllocsPerRun(200, func() {
			if buildChain(segments) == "" {
				t.Fatal("empty chain")
			}
		})

		if got > 1 {
			t.Errorf("a %d-segment chain allocated %.0f times, want 1 — the buffer is being grown", segments, got)
		}
	}
}

// A single identifier is not accumulated at all: String hands back the string
// the scanner already produced. Most chains are this, which is why it matters
// more than the case above.
func TestChainBuilderDoesNotAllocateForASingleIdentifier(t *testing.T) {
	got := testing.AllocsPerRun(200, func() {
		var c chainBuilder

		c.reset("svc")

		if c.String() == "" {
			t.Fatal("empty chain")
		}
	})

	if got != 0 {
		t.Errorf("a one-segment chain allocated %.0f times, want 0", got)
	}
}

// The array is a fast path, not a limit. A chain past it still has to come out
// whole.
func TestChainBuilderSpillsCorrectly(t *testing.T) {
	seg := strings.Repeat("x", 20)

	for _, segments := range []int{1, 2, 4, 5, 6, 20} {
		var c chainBuilder

		c.reset(seg)

		want := seg

		var wantSb75 strings.Builder

		for range segments - 1 {
			c.writeDot()
			c.writeString(seg)

			wantSb75.WriteString("." + seg)
		}

		want += wantSb75.String()

		if got := c.String(); got != want {
			t.Errorf("%d segments (%d bytes, array holds %d): got %q, want %q",
				segments, len(want), chainBufLen, got, want)
		}
	}
}

// reset has to clear the accumulated bytes as well as the held first segment,
// or a reused builder carries the previous chain's tail.
func TestChainBuilderResetClearsEverything(t *testing.T) {
	var c chainBuilder

	// Long enough to spill, so the reset has both buffers to clear.
	long := strings.Repeat("y", chainBufLen)

	c.reset("first")
	c.writeDot()
	c.writeString(long)

	c.reset("second")

	if got := c.String(); got != "second" {
		t.Errorf("after reset, String() = %q, want %q", got, "second")
	}

	c.writeDot()
	c.writeString("tail")

	if got, want := c.String(), "second.tail"; got != want {
		t.Errorf("after reset and appends, String() = %q, want %q", got, want)
	}
}

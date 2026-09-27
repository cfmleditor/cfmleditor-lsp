package conv

import (
	"math"
	"testing"
)

func TestConversionsClampInsteadOfWrapping(t *testing.T) {
	for _, c := range []struct {
		in   int
		want uint32
	}{{0, 0}, {42, 42}, {-1, 0}, {math.MinInt, 0}, {math.MaxUint32, math.MaxUint32}, {math.MaxUint32 + 1, math.MaxUint32}} {
		if got := Uint32(c.in); got != c.want {
			t.Errorf("Uint32(%d) = %d, want %d", c.in, got, c.want)
		}
	}

	if got := Uint32FromUint(math.MaxUint32 + 7); got != math.MaxUint32 {
		t.Errorf("Uint32FromUint overflowed to %d", got)
	}

	for _, c := range []struct {
		in   int
		want int32
	}{{-5, -5}, {math.MaxInt32 + 1, math.MaxInt32}, {math.MinInt32 - 1, math.MinInt32}} {
		if got := Int32(c.in); got != c.want {
			t.Errorf("Int32(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

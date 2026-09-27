// Package conv holds the range-checked integer conversions the rest of the code
// uses where Go arithmetic in int meets a narrower type: an LSP position's
// uint32 line and column, above all.
//
// A bare uint32(n) wraps a negative n to about four billion and truncates an
// oversized one, silently. These clamp instead, so a value that should never
// be out of range becomes the nearest valid one rather than a position past
// the end of every document. gosec's G115 flags the bare form.
//
// The package imports nothing but math, so any package may use it, the
// deliberately dependency-light parser included.
package conv

import "math"

// Uint32 converts n to a uint32, clamping it to [0, math.MaxUint32].
func Uint32(n int) uint32 {
	if n < 0 {
		return 0
	}

	if n > math.MaxUint32 {
		return math.MaxUint32
	}

	return uint32(n)
}

// Uint32FromUint converts n to a uint32, clamping it to math.MaxUint32.
func Uint32FromUint(n uint) uint32 {
	if n > math.MaxUint32 {
		return math.MaxUint32
	}

	return uint32(n)
}

// Int32 converts n to an int32, clamping it to [math.MinInt32, math.MaxInt32].
func Int32(n int) int32 {
	if n < math.MinInt32 {
		return math.MinInt32
	}

	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}

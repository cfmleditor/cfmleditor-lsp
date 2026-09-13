package textdiff

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// apply rebuilds b from a and the hunks, which is the property every case here
// is really asserting.
func apply(a, b []string, hunks []Hunk) []string {
	var out []string

	pos := 0

	for _, h := range hunks {
		out = append(out, a[pos:h.AStart]...)
		out = append(out, b[h.BStart:h.BEnd]...)
		pos = h.AEnd
	}

	return append(out, a[pos:]...)
}

func checkApply(t *testing.T, a, b []string) []Hunk {
	t.Helper()

	hunks := Hunks(a, b)

	// slices.Equal, not reflect.DeepEqual: an empty result comes back nil and
	// the two are the same document.
	if got := apply(a, b, hunks); !slices.Equal(got, b) {
		t.Fatalf("applying hunks gave %q, want %q (hunks %+v)", got, b, hunks)
	}

	// No hunk may be empty on both sides, and they must be ordered and
	// non-overlapping — a caller filtering by line range relies on both.
	prev := -1

	for _, h := range hunks {
		if h.AStart == h.AEnd && h.BStart == h.BEnd {
			t.Errorf("empty hunk %+v", h)
		}

		if h.AStart < prev {
			t.Errorf("hunk %+v starts before the previous one ended (%d)", h, prev)
		}

		prev = h.AEnd
	}

	return hunks
}

func TestHunks(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []Hunk
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"}, nil},
		{"both empty", nil, nil, nil},
		{
			"one line changed in the middle",
			[]string{"a", "b", "c"},
			[]string{"a", "B", "c"},
			[]Hunk{{1, 2, 1, 2}},
		},
		{
			"insertion",
			[]string{"a", "c"},
			[]string{"a", "b", "c"},
			[]Hunk{{1, 1, 1, 2}},
		},
		{
			"deletion",
			[]string{"a", "b", "c"},
			[]string{"a", "c"},
			[]Hunk{{1, 2, 1, 1}},
		},
		{
			"two separate changes stay two hunks",
			[]string{"a", "b", "c", "d", "e"},
			[]string{"a", "B", "c", "D", "e"},
			[]Hunk{{1, 2, 1, 2}, {3, 4, 3, 4}},
		},
		{
			"adjacent delete and insert coalesce into one replacement",
			[]string{"a", "b", "c", "d"},
			[]string{"a", "X", "Y", "Z", "d"},
			[]Hunk{{1, 3, 1, 4}},
		},
		{
			"everything changed",
			[]string{"a", "b"},
			[]string{"x", "y"},
			[]Hunk{{0, 2, 0, 2}},
		},
		{"a emptied", []string{"a", "b"}, nil, []Hunk{{0, 2, 0, 0}}},
		{"b from empty", nil, []string{"a", "b"}, []Hunk{{0, 0, 0, 2}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkApply(t, tc.a, tc.b)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Hunks = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestHunksRoundTrip is the property the hunks exist for, over inputs nobody
// would think to write by hand: whatever the edit, applying the hunks to a must
// reproduce b exactly.
func TestHunksRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // deterministic fixture generation, not security

	for range 400 {
		n := rng.IntN(14)

		a := make([]string, n)
		for i := range a {
			a[i] = strconv.Itoa(rng.IntN(5))
		}

		b := slices.Clone(a)

		for range rng.IntN(5) {
			switch rng.IntN(3) {
			case 0: // replace
				if len(b) > 0 {
					b[rng.IntN(len(b))] = strconv.Itoa(rng.IntN(5))
				}
			case 1: // insert
				at := rng.IntN(len(b) + 1)
				b = slices.Insert(b, at, strconv.Itoa(rng.IntN(5)))
			case 2: // delete
				if len(b) > 0 {
					at := rng.IntN(len(b))
					b = slices.Delete(b, at, at+1)
				}
			}
		}

		checkApply(t, a, b)
	}
}

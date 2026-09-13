// Package textdiff computes the minimal set of line replacements turning one
// sequence of lines into another.
//
// It exists for range formatting, which has to answer "which lines did the
// formatter actually change, and what did each become". The whole document is
// formatted — that is the only way the result can be guaranteed identical to
// what format-on-save would produce — and this then says which parts of the
// result belong to the lines the user selected.
package textdiff

// Hunk replaces A[AStart:AEnd] with B[BStart:BEnd]. Both ranges are half-open,
// and either may be empty: an empty A range is an insertion before AStart, an
// empty B range a deletion.
type Hunk struct {
	AStart, AEnd int
	BStart, BEnd int
}

// Hunks returns the replacements that turn a into b, in order, none of them
// overlapping and none of them empty on both sides.
//
// The comparison is by exact string equality, so a caller that wants two lines
// treated as "the same line" — range formatting keys lines by their trimmed
// text, since a reindented line is still that line — normalises before calling
// and keeps the raw text alongside.
func Hunks(a, b []string) []Hunk {
	// Trimming the common ends first is not just an optimisation. The Myers
	// walk below is O((N+M)·D) in the edit distance, and formatting typically
	// leaves the head and tail of a document alone, so this is what keeps D
	// proportional to the edited region rather than to the file.
	lo := 0
	for lo < len(a) && lo < len(b) && a[lo] == b[lo] {
		lo++
	}

	hiA, hiB := len(a), len(b)
	for hiA > lo && hiB > lo && a[hiA-1] == b[hiB-1] {
		hiA--
		hiB--
	}

	// Nothing left on either side: the sequences were identical.
	remA, remB := hiA-lo, hiB-lo
	if remA == 0 && remB == 0 {
		return nil
	}

	hunks := myers(a[lo:hiA], b[lo:hiB])
	for i := range hunks {
		hunks[i].AStart += lo
		hunks[i].AEnd += lo
		hunks[i].BStart += lo
		hunks[i].BEnd += lo
	}

	return hunks
}

// myers runs Myers' greedy shortest-edit-script algorithm, recording one
// frontier per edit distance so the path can be walked back afterwards.
func myers(a, b []string) []Hunk {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}

	maxD := n + m
	offset := maxD
	v := make([]int, 2*maxD+1)
	trace := make([][]int, 0, maxD+1)

	for d := 0; d <= maxD; d++ {
		trace = append(trace, append([]int(nil), v...))

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1] // down: an insertion from b
			} else {
				x = v[offset+k-1] + 1 // right: a deletion from a
			}

			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			v[offset+k] = x

			if x >= n && y >= m {
				return backtrack(trace, n, m, offset)
			}
		}
	}

	// Unreachable: a path always exists within n+m edits.
	return []Hunk{{AStart: 0, AEnd: n, BStart: 0, BEnd: m}}
}

// backtrack walks the recorded frontiers from the end back to the origin,
// collecting the single-line deletions and insertions, then coalesces runs of
// them that touch into one replacement each.
func backtrack(trace [][]int, n, m, offset int) []Hunk {
	type step struct{ aStart, aEnd, bStart, bEnd int }

	var steps []step

	x, y := n, m

	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y

		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[offset+prevK]
		prevY := prevX - prevK

		// The diagonal run before this edit is unchanged lines; skip it.
		for x > prevX && y > prevY {
			x--
			y--
		}

		if x > prevX {
			steps = append(steps, step{aStart: x - 1, aEnd: x, bStart: y, bEnd: y})
		} else {
			steps = append(steps, step{aStart: x, aEnd: x, bStart: y - 1, bEnd: y})
		}

		x, y = prevX, prevY
	}

	// steps came out end-first; walk them forwards, merging any that meet.
	var hunks []Hunk

	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]

		if len(hunks) > 0 {
			last := &hunks[len(hunks)-1]
			if last.AEnd == s.aStart && last.BEnd == s.bStart {
				last.AEnd = s.aEnd
				last.BEnd = s.bEnd

				continue
			}
		}

		hunks = append(hunks, Hunk{AStart: s.aStart, AEnd: s.aEnd, BStart: s.bStart, BEnd: s.bEnd})
	}

	return hunks
}

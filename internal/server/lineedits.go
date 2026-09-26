package server

import (
	"sort"
	"strings"
	"unicode/utf16"

	"go.lsp.dev/protocol"
)

// lineEdits returns TextEdits that turn before into after, each replacing a run
// of whole lines, instead of one edit replacing the document.
//
// A formatting response used to be the whole formatted file. On a 65,000-line
// component that is 3.6MB over the wire, which the editor then diffs against
// the buffer itself to find what changed, and undoing it replays every change
// that diff found. Sending the changed runs of lines gives it only those.
//
// The runs come from a patience diff: lines that occur exactly once on each
// side and in the same order anchor the two texts together, and the text
// between two anchors is diffed again the same way. A stretch with no such
// line becomes one edit. That keeps it close to linear on a large file, where
// an edit-distance diff is quadratic in the number of changed lines, and a
// reformat changes most of them. The edits are correct whatever the diff
// finds, since each replaces a stretch of before with the matching stretch of
// after; only how finely they are cut depends on it.
func lineEdits(before, after string) []protocol.TextEdit {
	d := lineDiff{a: splitLines(before), b: splitLines(after)}
	d.ka, d.kb = trimmedKeys(d.a), trimmedKeys(d.b)
	d.patience(0, len(d.a), 0, len(d.b))

	return d.edits
}

// lineDiff matches lines on their trimmed text, as range formatting does (see
// rangeEdits): a reindented line is still that line, so it anchors the diff and
// comes back as a one-line edit, where matching raw lines would leave a
// reindented block with nothing to anchor on and send it back whole. Lines that
// match on their trimmed text but differ raw are replaced one by one.
type lineDiff struct {
	a, b   []string // the lines, each with its "\n"
	ka, kb []string // the same lines trimmed
	edits  []protocol.TextEdit
	last   int // a index where the last edit ends, to merge one that follows on
}

func trimmedKeys(lines []string) []string {
	keys := make([]string, len(lines))
	for i, l := range lines {
		keys[i] = strings.TrimSpace(l)
	}

	return keys
}

// replace records that a[alo:ahi] becomes b[blo:bhi]. Calls arrive in order,
// and one that starts where the last ended extends it: a run of changed lines
// is one edit, not one per line, which on a file the formatter changes
// throughout would make the response bigger than the document.
func (d *lineDiff) replace(alo, ahi, blo, bhi int) {
	if alo == ahi && blo == bhi {
		return
	}

	if n := len(d.edits); n > 0 && d.last == alo {
		d.edits[n-1].Range.End = linePos(d.a, ahi)
		d.edits[n-1].NewText += strings.Join(d.b[blo:bhi], "")
		d.last = ahi

		return
	}

	d.last = ahi
	d.edits = append(d.edits, protocol.TextEdit{
		Range:   protocol.Range{Start: linePos(d.a, alo), End: linePos(d.a, ahi)},
		NewText: strings.Join(d.b[blo:bhi], ""),
	})
}

// same records that a[i] and b[j] are the same line, replacing it if its raw
// text differs.
func (d *lineDiff) same(i, j int) {
	if d.a[i] != d.b[j] {
		d.replace(i, i+1, j, j+1)
	}
}

func (d *lineDiff) patience(alo, ahi, blo, bhi int) {
	for alo < ahi && blo < bhi && d.ka[alo] == d.kb[blo] {
		d.same(alo, blo)
		alo++
		blo++
	}

	tail := 0
	for alo < ahi-tail && blo < bhi-tail && d.ka[ahi-tail-1] == d.kb[bhi-tail-1] {
		tail++
	}

	ahi, bhi = ahi-tail, bhi-tail

	switch anchors := uniqueCommon(d.ka, d.kb, alo, ahi, blo, bhi); {
	case alo == ahi || blo == bhi || len(anchors) == 0:
		d.replace(alo, ahi, blo, bhi)
	default:
		for _, m := range anchors {
			d.patience(alo, m.a, blo, m.b)
			d.same(m.a, m.b)
			alo, blo = m.a+1, m.b+1
		}

		d.patience(alo, ahi, blo, bhi)
	}

	for k := range tail {
		d.same(ahi+k, bhi+k)
	}
}

// splitLines splits s after each "\n", so joining the pieces gives s back. A
// final line with no newline is its own piece; an empty s has none.
func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// linePos is the position where line i of lines starts, or the end of the
// document when i is past the last line.
func linePos(lines []string, i int) protocol.Position {
	if i < len(lines) {
		return protocol.Position{Line: uint32(i)}
	}

	if i == 0 {
		return protocol.Position{}
	}

	last := lines[len(lines)-1]
	if strings.HasSuffix(last, "\n") {
		return protocol.Position{Line: uint32(len(lines))}
	}

	return protocol.Position{Line: uint32(len(lines) - 1), Character: uint32(len(utf16.Encode([]rune(last))))}
}

type lineMatch struct{ a, b int }

// uniqueCommon finds the lines that occur exactly once in a[alo:ahi] and once
// in b[blo:bhi], and returns the longest run of them that is in the same order
// on both sides (by patience sorting), ordered by position.
func uniqueCommon(a, b []string, alo, ahi, blo, bhi int) []lineMatch {
	type count struct{ na, nb, ia, ib int }

	counts := make(map[string]*count, ahi-alo)

	for i := alo; i < ahi; i++ {
		c := counts[a[i]]
		if c == nil {
			c = &count{}
			counts[a[i]] = c
		}

		c.na++
		c.ia = i
	}

	for i := blo; i < bhi; i++ {
		if c := counts[b[i]]; c != nil {
			c.nb++
			c.ib = i
		}
	}

	var matches []lineMatch

	for i := alo; i < ahi; i++ {
		if c := counts[a[i]]; c.na == 1 && c.nb == 1 {
			matches = append(matches, lineMatch{a: i, b: c.ib})
		}
	}

	// Longest increasing subsequence of b over matches, which are in a order.
	var tails []int // index into matches of the smallest tail for each length

	prev := make([]int, len(matches))

	for i, m := range matches {
		k := sort.Search(len(tails), func(j int) bool { return matches[tails[j]].b > m.b })
		if k > 0 {
			prev[i] = tails[k-1]
		} else {
			prev[i] = -1
		}

		if k == len(tails) {
			tails = append(tails, i)
		} else {
			tails[k] = i
		}
	}

	if len(tails) == 0 {
		return nil
	}

	out := make([]lineMatch, len(tails))
	for i, k := len(tails)-1, tails[len(tails)-1]; i >= 0; i, k = i-1, prev[k] {
		out[i] = matches[k]
	}

	return out
}

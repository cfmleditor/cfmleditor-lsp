package parser

import (
	"slices"
	"strings"
)

// flowBlocks records, for a full parse, which block each assignment in a
// function was made in and which blocks were open at each return, so a return
// can tell an assignment that always runs before it from one that may not.
//
// A block is a brace group in CFScript (an if or else body, a case list, a
// loop body, a try or a catch) and the body of the matching tag in tag syntax
// (<cfif>, <cfelseif>, <cfelse>, <cfloop>, <cftry>, <cfcatch>, <cfcase>,
// <cfdefaultcase>), plus the function body itself. An assignment whose block
// is open at the return encloses it, so it ran; any other may not have.
//
// A block is named by the file offset just past its opener, so the blocks of
// every region of one file are distinct. Only the stack is per parser: the
// record is shared by the regions of a script file, whose lines are already
// the file's, and merged with a shift for a tag region, whose are not.
//
// It lives for the parse alone: resolvePendingCalls is its last reader.
type flowBlocks struct {
	base int
	// shift is added to every line noted, for a tag region whose lines are
	// its own rather than the file's.
	shift uint32
	stack []uint32
	// kinds names the tag that opened each block on stack, for a tag parser
	// matching a close tag to its opener. A script parser leaves it empty.
	kinds []string
	// ends is, for each braceless body open on stack (one in kinds marked
	// kindBraceless), the offset of the semicolon that closes it, innermost last.
	ends []int
	rec  *flowRecord
}

// kindBraceless marks a block opened by a statement written without braces.
const kindBraceless = "{;"

// flowRecord is what the parsers of one file's regions record between them.
type flowRecord struct {
	notes  []refNote
	sorted bool
	// returns lists each return's line and where the blocks open at it sit
	// in open, one flat slice rather than a slice per return.
	returns []returnNote
	open    []uint32
}

type returnNote struct {
	line     uint32
	from, to int
}

// refNote is the block an assignment was made in, named by the variable it
// assigns and its line, which is all a ComponentRef carries to be found by.
// The name is the ref's own string, not a lowercased copy: notes are made for
// every ref of a parse and read for the few a return asks about.
type refNote struct {
	name        string
	line, block uint32
}

// blockUnknown is the block of an assignment two statements on one line made
// in different blocks; such a ref is read as always running, as every ref was
// before blocks were recorded.
const blockUnknown = ^uint32(0)

func newFlowBlocks(base int) *flowBlocks {
	return &flowBlocks{base: base, rec: &flowRecord{sorted: true}}
}

// region is a tracker for another region of the same file, sharing its
// record: base is the region's offset in the file, shift the line its lines
// count from when they are not already the file's.
func (fb *flowBlocks) region(base int, shift uint32) *flowBlocks {
	return &flowBlocks{base: base, shift: shift, rec: fb.rec}
}

// open pushes the block whose opener ends at offset in the parser's text.
func (fb *flowBlocks) open(offset int, kind string) {
	fb.stack = append(fb.stack, uint32(fb.base+offset+1)) //nolint:gosec // file offsets are far below 4GB
	fb.kinds = append(fb.kinds, kind)
}

// close pops the innermost block, or for a tag, the innermost one kind opened
// and everything left open inside it; a close with nothing to match is
// ignored.
func (fb *flowBlocks) close(kind string) {
	i := len(fb.stack) - 1
	if kind != "" {
		for i >= 0 && fb.kinds[i] != kind {
			i--
		}
	}

	if i < 0 {
		return
	}

	fb.stack, fb.kinds = fb.stack[:i], fb.kinds[:i]
}

// openBraceless pushes the block of the one statement an `if`, `else`, `for` or
// `while` governs when it is written without braces, closed by the semicolon
// at offset end.
func (fb *flowBlocks) openBraceless(offset, end int) {
	fb.open(offset, kindBraceless)
	fb.ends = append(fb.ends, end)
}

// closeEnded closes every braceless body whose semicolon is at or before
// offset, the offset of the token about to be read: a statement parser may
// have consumed the semicolon itself, and a chain
// (`if ( a ) if ( b ) x = …;`) ends all its bodies at once.
func (fb *flowBlocks) closeEnded(offset int) {
	for len(fb.ends) > 0 && offset >= fb.ends[len(fb.ends)-1] {
		fb.ends = fb.ends[:len(fb.ends)-1]
		fb.stack, fb.kinds = fb.stack[:len(fb.stack)-1], fb.kinds[:len(fb.kinds)-1]
	}
}

// dropBraceless discards braceless bodies still open, for a body that ended
// before their semicolon was reached.
func (fb *flowBlocks) dropBraceless() {
	for len(fb.ends) > 0 {
		fb.ends = fb.ends[:len(fb.ends)-1]
		fb.stack, fb.kinds = fb.stack[:len(fb.stack)-1], fb.kinds[:len(fb.kinds)-1]
	}
}

// innermost is the block statements are being made in, 0 outside any.
func (fb *flowBlocks) innermost() uint32 {
	if len(fb.stack) == 0 {
		return 0
	}

	return fb.stack[len(fb.stack)-1]
}

// note records that the assignment to name on line was made in block.
func (fb *flowBlocks) note(name string, line, block uint32) {
	r := fb.rec
	line += fb.shift
	r.sorted = r.sorted && (len(r.notes) == 0 || r.notes[len(r.notes)-1].line <= line)
	r.notes = append(r.notes, refNote{name, line, block})
}

// noteReturn records the blocks open at a return on line.
func (fb *flowBlocks) noteReturn(line uint32) {
	r := fb.rec
	from := len(r.open)
	r.open = append(r.open, fb.stack...)
	r.returns = append(r.returns, returnNote{line + fb.shift, from, len(r.open)})
}

// openAt is the blocks open at the return on line, the latest recorded.
func (fb *flowBlocks) openAt(line uint32) []uint32 {
	r := fb.rec
	for _, n := range slices.Backward(r.returns) {
		if n.line == line {
			return r.open[n.from:n.to]
		}
	}

	return nil
}

// blockOf is the block the assignment ref records was made in: 0 when none
// was noted, and blockUnknown when two on its line disagree.
func (fb *flowBlocks) blockOf(ref *ComponentRef) uint32 {
	r := fb.rec
	if !r.sorted {
		slices.SortStableFunc(r.notes, func(a, b refNote) int { return int(a.line) - int(b.line) })
		r.sorted = true
	}

	i, _ := slices.BinarySearchFunc(r.notes, ref.Line, func(n refNote, line uint32) int { return int(n.line) - int(line) })

	var block uint32

	found := false

	for ; i < len(r.notes) && r.notes[i].line == ref.Line; i++ {
		if !strings.EqualFold(r.notes[i].name, ref.Variable) {
			continue
		}

		if found && r.notes[i].block != block {
			return blockUnknown
		}

		block, found = r.notes[i].block, true
	}

	return block
}

// always reports whether ref runs before a return with the blocks open
// listed: it was made outside any block, in one of them, or where the blocks
// are not known.
func (fb *flowBlocks) always(ref *ComponentRef, open []uint32) bool {
	b := fb.blockOf(ref)

	return b == 0 || b == blockUnknown || slices.Contains(open, b)
}

// reaching is the refs for name in refs, among those admit accepts, that may
// hold its value at a return on line: the latest one that always runs before
// it, and every one after that which may not. Without a record of the
// blocks, or of the return's, it is refReaching's one ref. Nil when there is
// none.
func (fb *flowBlocks) reaching(refs []ComponentRef, name string, line uint32, admit func(*ComponentRef) bool) []*ComponentRef {
	var open []uint32

	if fb != nil {
		open = fb.openAt(line)
	}

	if len(open) == 0 {
		if ref := refReaching(refs, name, line, admit); ref != nil {
			return []*ComponentRef{ref}
		}

		return nil
	}

	var before []*ComponentRef

	for i := range refs {
		if ref := &refs[i]; ref.Line <= line && strings.EqualFold(ref.Variable, name) && admit(ref) {
			before = append(before, ref)
		}
	}

	if len(before) == 0 {
		if ref := refReaching(refs, name, line, admit); ref != nil {
			return []*ComponentRef{ref}
		}

		return nil
	}

	slices.SortStableFunc(before, func(a, b *ComponentRef) int { return int(a.Line) - int(b.Line) })

	var out []*ComponentRef

	for _, ref := range before {
		if fb.always(ref, open) {
			out = out[:0]
		}

		out = append(out, ref)
	}

	return out
}

// agreedComponent is what refs hold between them, comp reading each: "$any"
// when one is dynamic, the component when they all agree, and nothing when
// they do not or one of them is untyped.
func agreedComponent(refs []*ComponentRef, comp func(*ComponentRef) string) string {
	if len(refs) == 0 {
		return ""
	}

	agreed := comp(refs[0])
	disagree := false

	for _, ref := range refs[1:] {
		c := comp(ref)
		if c == "$any" {
			return "$any"
		}

		if !strings.EqualFold(agreed, c) {
			disagree = true
		}
	}

	if agreed == "$any" {
		return "$any"
	}

	if disagree {
		return ""
	}

	return agreed
}

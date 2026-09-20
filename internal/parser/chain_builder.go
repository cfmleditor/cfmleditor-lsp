package parser

// chainBuilder accumulates a dotted receiver chain — "svc", "svc.repo",
// "REQUEST[].method" — without allocating when the chain turns out to be a
// single identifier, which is what most of them are.
//
// A plain strings.Builder allocates its buffer on the first write, so every
// `x = y` in a file paid for one whether or not anything was ever appended to
// it. On the script benchmark that was 30% of all allocated objects in the
// builder's grow, plus 12% in WriteString.
//
// The first token is held as the string the scanner already produced, and
// nothing is accumulated unless something is appended after it. String()
// returns the held string in that case, so the result is identical either way.
//
// A chain that does get appended to accumulates into an array inside the
// builder, which is a local at all eleven call sites and so is stack space.
// strings.Builder starts from nothing and doubles, so a three-segment chain —
// `svc.repo.find`, the ordinary shape — grew its buffer two or three times on
// the way to a string it then had to allocate anyway. The array holds every
// chain a CFML receiver realistically spells; append falls back to the heap
// above that, and String() is the one allocation either way.
type chainBuilder struct {
	first  string
	joined bool
	n      int
	arr    [chainBufLen]byte
	spill  []byte // nil unless the chain outgrew arr
}

// chainBufLen is past any dotted receiver chain in the corpus. It costs stack,
// not heap, so the bound is about how often the fallback runs rather than what
// it costs to be generous.
//
// The array is addressed by length rather than held as a slice of itself.
// `c.rest = c.arr[:0]` is the shorter spelling and it moves every one of the
// eleven builders to the heap: a struct holding a pointer into itself defeats
// escape analysis, and the struct is now large. That cost more than the growth
// it saved — 443 allocations a parse against 403.
const chainBufLen = 96

// reset starts a new chain whose first segment is s.
func (c *chainBuilder) reset(s string) {
	c.first = s
	c.joined = false
	c.n = 0
	c.spill = nil
}

func (c *chainBuilder) start() {
	if c.joined {
		return
	}

	c.joined = true

	c.append(c.first)
}

func (c *chainBuilder) writeString(s string) {
	c.start()
	c.append(s)
}

// writeDot appends the separator between two chain segments. A dot is the only
// byte any caller ever appended, so it is named for what it does rather than
// taking one.
func (c *chainBuilder) writeDot() {
	c.start()
	c.append(".")
}

// Lowercase names, deliberately: writeByte(byte) without an error return is the
// signature of io.ByteWriter with the error missing, which go vet flags as a
// method that looks like an interface's and is not. These are not writers and
// nothing should reach for them as one.

// String is the chain so far.
func (c *chainBuilder) String() string {
	if !c.joined {
		return c.first
	}

	if c.spill != nil {
		return string(c.spill)
	}

	return string(c.arr[:c.n])
}

// append adds s to whichever of the two buffers is in use, moving to the heap
// the first time the array runs out.
func (c *chainBuilder) append(s string) {
	if c.spill != nil {
		c.spill = append(c.spill, s...)

		return
	}

	if c.n+len(s) <= len(c.arr) {
		c.n += copy(c.arr[c.n:], s)

		return
	}

	c.spill = make([]byte, 0, (c.n+len(s))*2)
	c.spill = append(c.spill, c.arr[:c.n]...)
	c.spill = append(c.spill, s...)
}

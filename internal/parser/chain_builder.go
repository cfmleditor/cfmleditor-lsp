package parser

import "strings"

// chainBuilder accumulates a dotted receiver chain — "svc", "svc.repo",
// "REQUEST[].method" — without allocating when the chain turns out to be a
// single identifier, which is what most of them are.
//
// A plain strings.Builder allocates its buffer on the first write, so every
// `x = y` in a file paid for one whether or not anything was ever appended to
// it. On the script benchmark that was 30% of all allocated objects in the
// builder's grow, plus 12% in WriteString.
//
// The first token is held as the string the scanner already produced, and the
// builder is only started if something is appended after it. String() returns
// the held string in that case, so the result is identical either way.
type chainBuilder struct {
	first  string
	rest   strings.Builder
	joined bool
}

// reset starts a new chain whose first segment is s.
func (c *chainBuilder) reset(s string) {
	c.first = s
	c.joined = false
	c.rest.Reset()
}

func (c *chainBuilder) start() {
	if c.joined {
		return
	}

	c.joined = true

	c.rest.WriteString(c.first)
}

func (c *chainBuilder) writeString(s string) {
	c.start()
	c.rest.WriteString(s)
}

// writeDot appends the separator between two chain segments. A dot is the only
// byte any caller ever appended, so it is named for what it does rather than
// taking one.
func (c *chainBuilder) writeDot() {
	c.start()
	c.rest.WriteByte('.')
}

// Lowercase names, deliberately: WriteByte(byte) without an error return is the
// signature of io.ByteWriter with the error missing, which go vet flags as a
// method that looks like an interface's and is not. These are not writers and
// nothing should reach for them as one.

// String is the chain so far.
func (c *chainBuilder) String() string {
	if !c.joined {
		return c.first
	}

	return c.rest.String()
}

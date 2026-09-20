package parser

// foldScratch is stack space for lowerFold. Sized past any identifier these
// switches compare against, and past any realistic CFML identifier, so the
// heap path below is the exception rather than the common case.
type foldScratch [64]byte

// lowerFold lowercases s into f and returns the bytes, for the case-insensitive
// switches through this package: `switch string(buf.lowerFold(v))` compares
// against the case literals without building the lowercased string.
//
// strings.ToLower returns its argument untouched when there is nothing to
// lower, so these switches looked free on all-lowercase source and were not:
// the parse path folds identifiers, and an identifier with a capital in it —
// getUser, userDAO, CacheManager, every ARGUMENTS and VARIABLES — allocated a
// string that the switch compared once and dropped. That was 655,000 objects on
// the script benchmark, 22% of everything a parse allocated, and none of it
// outlived the comparison.
//
// The declaration has to be a local `var buf foldScratch` at each call site
// rather than a field on the parser. Escape analysis then keeps the array on
// the stack, since the returned slice reaches no further than the string
// conversion in the switch; a shared buffer would cost nothing less and would
// be clobbered by any nested fold. Holding the result across a call that folds
// again is the one thing this must not be used for, which is why every caller
// spells it inside the switch expression rather than binding it to a name.
func (f *foldScratch) lowerFold(s string) []byte {
	if len(s) > len(f) {
		out := make([]byte, len(s))
		for i := range len(s) {
			out[i] = toLowerByte(s[i])
		}

		return out
	}

	for i := range len(s) {
		f[i] = toLowerByte(s[i])
	}

	return f[:len(s)]
}

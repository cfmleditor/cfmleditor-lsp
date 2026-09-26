package formatter

import "testing"

// New takes its options by pointer and fills in defaults for the zero fields.
// It must fill them into a copy of its own: the caller's struct is the
// caller's, and writing through the pointer would make every later use of it
// see defaults it never set.
func TestNewDoesNotWriteToTheCallersOptions(t *testing.T) {
	var opts Options

	f := New(&opts)

	if opts.IndentWidth != 0 || opts.LineWidth != 0 || opts.AttrBreakThreshold != 0 {
		t.Errorf("New wrote its defaults into the caller's options: indent %d, width %d, threshold %d",
			opts.IndentWidth, opts.LineWidth, opts.AttrBreakThreshold)
	}

	if f.opts.IndentWidth == 0 || f.opts.LineWidth == 0 || f.opts.AttrBreakThreshold == 0 {
		t.Errorf("New's own copy did not get its defaults: %+v", f.opts)
	}
}

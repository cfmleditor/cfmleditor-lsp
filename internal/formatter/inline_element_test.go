package formatter

import (
	"strings"
	"testing"
)

// TestTightElementsStayInline covers content written flush against both tags
// being split across three lines, which introduces whitespace the author left
// out — and which a browser renders for inline elements.
func TestTightElementsStayInline(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "div with class",
			body: `<div class="systemerror-title">Sorry</div>`,
			want: `<div class="systemerror-title">Sorry</div>`,
		},
		{
			name: "title",
			body: `<title>System Maintenance</title>`,
			want: `<title>System Maintenance</title>`,
		},
		{
			name: "empty element",
			body: `<div></div>`,
			want: `<div></div>`,
		},
		{
			name: "nested tight elements",
			body: `<div><span>a</span><span>b</span></div>`,
			want: `<div><span>a</span><span>b</span></div>`,
		},
		{
			name: "tight content containing cf tags",
			body: `<div class="a">Down. <cfif b><cfdump var="#c#"></cfif></div>`,
			want: `<div class="a">Down. <cfif b><cfdump var="#c#"></cfif></div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := format(t, "<body>\n"+tt.body+"\n</body>\n")
			if !strings.Contains(out, tt.want) {
				t.Errorf("tight element was broken up\n got:\n%s\nwant line: %s", out, tt.want)
			}
		})
	}
}

// TestLooseElementsStillExpand checks the rule keys off whitespace actually
// present in the source, rather than collapsing every short element.
//
// Each case carries a sibling and a block child so the enclosing <body> takes
// the block path — a <body> holding a single single-line child is emitted as
// one inline run by an older code path, which would mask what is under test.
func TestLooseElementsStillExpand(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		notWant string
	}{
		{
			name:    "newlines around content",
			body:    "<div id=\"x\">\nSpaced\n</div>",
			notWant: "<div id=\"x\">Spaced</div>",
		},
		{
			name:    "spaces around content",
			body:    "<div id=\"x\"> padded </div>",
			notWant: "<div id=\"x\"> padded </div>",
		},
		{
			name:    "space after start tag only",
			body:    "<div id=\"x\"> padded</div>",
			notWant: "<div id=\"x\"> padded</div>",
		},
		{
			name:    "space before end tag only",
			body:    "<div id=\"x\">padded </div>",
			notWant: "<div id=\"x\">padded </div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := format(t, "<body>\n"+tt.body+"\n<p>\nsibling\n</p>\n</body>\n")
			if strings.Contains(out, tt.notWant) {
				t.Errorf("loose element should have been expanded, got %q in:\n%s", tt.notWant, out)
			}
		})
	}
}

// TestTightElementIdempotent guards against a tight element oscillating
// between inline and expanded across passes.
func TestTightElementIdempotent(t *testing.T) {
	src := "<body>\n<div class=\"t\">Sorry</div>\n<div>\nSpaced\n</div>\n</body>\n"

	first := format(t, src)
	if second := format(t, first); first != second {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

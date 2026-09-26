package route

import (
	"strings"
	"testing"
)

// scanConfig is a realistic convention, so the scan does the work a real one does.
func scanConfig() Config {
	return Config{
		Attributes:  []string{"data-view", "data-read", "data-process"},
		QueryParams: []string{"do"},
		Properties:  []string{"read", "view", "process"},
		Functions:   []string{"redirect", "print", "getRoute", "setRequestContext"},
	}
}

// page returns n copies of a block that looks like the CFML this scans: links
// with escaped ampersands, which is what makes the '&' path matter.
func page(n int) string {
	block := `<cfoutput>
	<a href="index.cfm?x=1&amp;do=admin.report.view">report</a>
	<a href="index.cfm?y=2&amp;z=3&amp;do=admin.report.read">read</a>
	<cfset foo = "bar & baz">
	<div data-view="admin.thing.view" data-read="admin.thing.read">x</div>
</cfoutput>
`

	return strings.Repeat(block, n)
}

// filler is content with no routes in it, but plenty of the characters that
// drive the scan: ampersands, question marks, quotes and angle brackets. A real
// file is mostly this.
func filler(n int) string {
	return strings.Repeat(`<cfset msg = "Tom & Jerry? yes & no"><a href="x.cfm?a=1&amp;b=2">t</a>
`, n)
}

// Scanning content that holds no routes must cost the same however much of it
// there is.
//
// This is the defect that cost three seconds per request, stated as a property.
// One line lowercased the whole remainder of the document to test a
// four-character prefix, once per '?' or '&' — so the work was the document's
// size multiplied by how many of those it contained, while the answer stayed
// empty. Every test that checked the refs passed, because the refs were right.
//
// Ten times the filler, the same routes, and the allocation must not follow the
// filler. It grew 100-fold before the fix.
func TestScanDoesNotAllocateOverContentWithNoRoutes(t *testing.T) {
	cfg := scanConfig()
	routes := page(2)

	alloc := func(content string) int64 {
		r := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				_ = Scan(content, &cfg)
			}
		})

		return r.AllocedBytesPerOp()
	}

	small := alloc(routes + filler(200))
	large := alloc(routes + filler(2000))

	t.Logf("10x the route-free filler: %d -> %d bytes/op (%.1fx)", small, large, float64(large)/float64(small))

	// Some growth is honest — the scan walks more bytes. Tripling is not: that
	// is cost proportional to the content rather than to what is found in it.
	if float64(large) > float64(small)*3 {
		t.Errorf("10x of route-free content took allocation from %d to %d bytes/op (%.1fx); "+
			"the scan is copying content rather than reading it", small, large, float64(large)/float64(small))
	}
}

// Cost must grow with the document, not with the document times itself.
//
// Eight times the input for roughly eight times the work. The ratio is checked
// rather than a duration, so a busy machine moves both numbers together and the
// test still means what it says.
func TestScanScalesLinearly(t *testing.T) {
	cfg := scanConfig()
	small, large := page(500), page(4000)

	ratio := float64(len(large)) / float64(len(small))

	bench := func(content string) int64 {
		r := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				_ = Scan(content, &cfg)
			}
		})

		return r.NsPerOp()
	}

	// Warm, so the first run's page faults are not charged to the small one.
	bench(small)

	smallNs, largeNs := bench(small), bench(large)
	got := float64(largeNs) / float64(smallNs)

	t.Logf("input %.0fx -> time %.1fx (%dns vs %dns)", ratio, got, smallNs, largeNs)

	// Linear would be 8x. Quadratic would be 64x. Three times the input ratio
	// leaves room for cache effects and constant factors while still failing
	// long before anything quadratic.
	if got > ratio*3 {
		t.Errorf("%.0fx the input took %.1fx the time; expected roughly linear", ratio, got)
	}
}

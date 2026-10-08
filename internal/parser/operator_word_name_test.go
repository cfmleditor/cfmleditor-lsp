package parser

import "testing"

// TestAnOperatorWordCanNameAReceiver: CFML lets `mod` and the comparison
// words name a variable, and cfwheels' specs hold their module in one —
// `variables.mod = new cli.lucli.Module(); mod.generate( … )`. Read as the
// operator, the receiver was dropped and the call recorded as a bare
// generate(), which resolved against the spec's own extends chain. A dot
// after the word is what decides it; `a mod .5` still records nothing.
func TestAnOperatorWordCanNameAReceiver(t *testing.T) {
	tests := []struct{ name, src string }{
		{"statement", `component { function run() { mod.generate(p); } }`},
		{"in a closure", `component { function run() { it("x", () => { mod.generate(p); }); } }`},
		{"assignment", `component { function run() { x = mod.generate(p); } }`},
		{"var", `component { function run() { var x = mod.generate(p); } }`},
		{"return", `component { function run() { return mod.generate(p); } }`},
		{"argument", `component { function run() { f(mod.generate(p)); } }`},
		{"component level", `component { mod.generate(p); }`},
		{"comparison word", `component { function run() { eq.generate(p); } }`},
		{"tag", `<cfcomponent><cffunction name="run"><cfset mod.generate(p)><cfset x = mod.generate(1)></cffunction></cfcomponent>`},
		{"tag", `<cfcomponent><cffunction name="run"><cfset x = mod.generate(1)><cfset mod.generate(p)></cffunction></cfcomponent>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := ParseWithOptions("file:///x.cfc", tt.src, &ParseOptions{ExtractCalls: true})

			want := 1
			if tt.name == "tag" {
				want = 2
			}

			got := 0

			for _, c := range pr.AllCalls() {
				if c.FuncName != "generate" {
					continue
				}

				if c.Variable == "" {
					t.Errorf("generate recorded with no receiver")
				}

				got++
			}

			if got != want {
				t.Errorf("generate recorded %d times, want %d", got, want)
			}
		})
	}

	pr := ParseWithOptions("file:///x.cfc", `component { function run() { x = a mod .5; y = b mod c.d(); } }`, &ParseOptions{ExtractCalls: true})
	if calls := pr.AllCalls(); len(calls) != 1 || calls[0].Variable != "c" || calls[0].FuncName != "d" {
		t.Errorf("operator use: calls %+v, want only c.d", calls)
	}
}

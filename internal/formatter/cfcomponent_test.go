package formatter

import (
	"strings"
	"testing"
)

func TestCFComponentPreserved(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			"simple component",
			`<cfcomponent><cffunction name="g"><cfreturn 1></cffunction></cfcomponent>`,
			[]string{"<cfcomponent>", "</cfcomponent>"},
		},
		{
			"component with attributes",
			`<cfcomponent extends="base.Component" output="false"><cffunction name="g"><cfreturn 1></cffunction></cfcomponent>`,
			[]string{`<cfcomponent extends="base.Component" output="false">`, "</cfcomponent>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format(t, tt.src)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("output missing %q\ngot:\n%s", w, got)
				}
			}
		})
	}
}

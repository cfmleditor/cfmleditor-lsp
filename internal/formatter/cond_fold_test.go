package formatter

import (
	"strings"
	"testing"
)

// TestFoldedConditionKeepsArgumentPadding is tassweb's one unstable file,
// packages/general/persist.cfc. A call too wide for the line has its arguments
// split onto lines of their own, and a condition folds its lines back onto one.
// Folding joined every line with a space, so the split came back as
// `ArrayFind( a, … )` — padding an argument list does not get — and only once
// the call was wide enough to split. While the closure still spanned lines in
// the source it was not, so the first format wrote the tight form and the
// second wrote the padded one.
//
// The call must render as the argument-list setting says in every mode, and
// the output must be a fixed point.
func TestFoldedConditionKeepsArgumentPadding(t *testing.T) {
	cases := map[string]string{
		"cfif": "<cfif NOT ArrayFind(arrAlerts[i][\"arrDevices\"], function(str){\n" +
			"\t\t\t\t\t\treturn str.d_token == trim(qryAlert.d_token);} )>\n\tx\n</cfif>\n",
		"cfelseif": "<cfif a>\n\tx\n<cfelseif NOT ArrayFind(arrAlerts[i][\"arrDevices\"], function(str){\n" +
			"\t\t\t\t\t\treturn str.d_token == trim(qryAlert.d_token);} )>\n\ty\n</cfif>\n",
		"cfscript if": "<cfscript>\nif (NOT ArrayFind(arrAlerts[i][\"arrDevices\"], function(str){\n" +
			"\t\t\t\t\t\treturn str.d_token == trim(qryAlert.d_token);} )) {\n\tx();\n}\n</cfscript>\n",
	}

	for name, src := range cases {
		for _, spacing := range []string{"", "pad", "tight"} {
			want := "ArrayFind(arrAlerts"
			if spacing == "pad" {
				want = "ArrayFind( arrAlerts"
			}

			once := formatWithParens(t, src, spacing)
			if !strings.Contains(once, want) {
				t.Errorf("%s, parenSpacing=%q: want %q in\n%s", name, spacing, want, once)
			}

			if twice := formatWithParens(t, once, spacing); once != twice {
				t.Errorf("%s, parenSpacing=%q: not idempotent\n--- once ---\n%s\n--- twice ---\n%s",
					name, spacing, once, twice)
			}
		}
	}
}

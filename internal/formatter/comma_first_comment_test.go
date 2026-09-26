package formatter

import "testing"

// TestCommaFirstParameterWithTrailingComment covers a `//` comment trailing a
// parameter in a comma-first list, as spreadsheet-cfml and Preside write them:
//
//	,required any image //path or object
//	,string imageType
//
// The comma that separates the two is only read on the next line, and it was
// appended to the parameter's text after the comment — `image //path or
// object,` — where the comment swallowed it. The guard refused the file. The
// comment now comes out on its own line after the parameter, as it already did
// for the comma-last spelling, and the output formats to itself. A comment on
// the last parameter has no comma to swallow and stays on its line.
func TestCommaFirstParameterWithTrailingComment(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		wants []string
	}{
		{
			name:  "comma first",
			src:   "<cfscript>\ncomponent {\n\tnumeric function f(\n\t\trequired workbook\n\t\t,required any image //path or object\n\t\t,string imageType\n\t) {\n\t\treturn 1;\n\t}\n}\n</cfscript>\n",
			wants: []string{"required any image,\n", "//path or object\n", "string imageType\n"},
		},
		{
			name:  "two comments, and one on the last parameter",
			src:   "<cfscript>\ncomponent {\n\tfunction f(\n\t\trequired a // one\n\t\t// two\n\t\t,boolean b=true //false = off\n\t) {\n\t\treturn 1;\n\t}\n}\n</cfscript>\n",
			wants: []string{"required a,\n", "// one\n", "// two\n", "boolean b = true //false = off\n"},
		},
		{
			name:  "comma last is unchanged",
			src:   "<cfscript>\ncomponent {\n\tfunction f(\n\t\trequired a, // why\n\t\tstring b\n\t) {\n\t\treturn 1;\n\t}\n}\n</cfscript>\n",
			wants: []string{"required a,\n", "// why\n", "string b\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			for _, want := range tt.wants {
				assertContains(t, out, want)
			}

			assertReparses(t, out)

			if again := formatGuarded(t, out); again != out {
				t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}

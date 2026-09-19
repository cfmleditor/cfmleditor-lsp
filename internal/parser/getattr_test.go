package parser

import "testing"

// getAttr used to search a lowercased copy of the tag and then index the
// original with the offsets it found. That holds only while folding preserves
// length, and it does not: U+0130 (İ) is two bytes and lowercases to one, so
// every offset after it in the copy points a byte early in the original.
//
// The result is not a miss, which would be obvious — it is a value read from
// one byte too soon, so an attribute comes back with its opening quote attached
// or its last character missing, and whatever consumed it carries that on
// silently. A Turkish hint or displayname is all it takes.
func TestGetAttrSurvivesAFoldThatChangesLength(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		attr string
		want string
	}{
		{
			"attribute after a length-changing fold",
			`<cffunction hint="İ" name="getDonor">`,
			"name", "getDonor",
		},
		{
			"several before the one wanted",
			`<cffunction displayname="İİİ" hint="İ" returntype="İ" name="getDonor">`,
			"name", "getDonor",
		},
		{
			"the folding character inside the value itself",
			`<cfargument name="İstanbul" type="string">`,
			"type", "string",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := getAttr(c.tag, c.attr); got != c.want {
				t.Errorf("getAttr(%q, %q) = %q, want %q", c.tag, c.attr, got, c.want)
			}
		})
	}
}

func TestIndexFoldFrom(t *testing.T) {
	cases := []struct {
		s, substr  string
		from, want int
	}{
		{"hello world", "WORLD", 0, 6},
		{"hello world", "world", 7, -1},
		{"aAaA", "aa", 1, 1},
		{"abc", "", 0, 0},
		{"abc", "abcd", 0, -1},
		{"İstanbul", "stan", 0, 2},
	}

	for _, c := range cases {
		if got := indexFoldFrom(c.s, c.substr, c.from); got != c.want {
			t.Errorf("indexFoldFrom(%q, %q, %d) = %d, want %d", c.s, c.substr, c.from, got, c.want)
		}
	}
}

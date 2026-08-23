package formatter

import (
	"strings"
	"testing"
)

func formatQuery(t *testing.T, src string) string {
	t.Helper()

	tree := parse(t, src)
	opts := testOpts()
	opts.QueryFormat = true

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return string(out)
}

// queryFormat is opt-in and had no test pinning comma preservation, though the
// SQL walker defers a trailing comma across several branches and clears it in
// eight places. These shapes exercise the ones that set and flush it.
func TestQueryFormattingPreservesCommas(t *testing.T) {
	cases := []string{
		"<cfquery name=\"q\">\nSELECT a, b, c FROM t WHERE x = 1\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a, b FROM t ORDER BY a, b\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a FROM t JOIN u ON t.id = u.id WHERE a IN (1, 2, 3)\n</cfquery>\n",
		"<cfquery name=\"q\">\nINSERT INTO t (a, b) VALUES (1, 2)\n</cfquery>\n",
		"<cfquery name=\"q\">\nUPDATE t SET a = 1, b = 2 WHERE id = 3\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a, b FROM t GROUP BY a, b HAVING count(*) > 1\n</cfquery>\n",
		"<cfquery name=\"q\">\nWITH c AS (SELECT a, b FROM t) SELECT a, b FROM c\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a, b FROM t UNION SELECT c, d FROM u\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a, (SELECT max(x) FROM u WHERE u.id = t.id), b FROM t\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT a, b\nFROM t\nLEFT OUTER JOIN u ON t.id = u.id\nWHERE a = 1\nORDER BY a, b\n</cfquery>\n",
		"<cfquery name=\"q\">\nSELECT DISTINCT a, b, c, d, e, f, g, h FROM some_very_long_table_name_here WHERE a = 1\n</cfquery>\n",
	}

	for i, src := range cases {
		out := formatQuery(t, src)
		inC := strings.Count(src, ",")
		outC := strings.Count(out, ",")

		if inC != outC {
			t.Errorf("case %d: comma count %d -> %d\nIN:\n%s\nOUT:\n%s", i, inC, outC, src, out)
		}
	}
}

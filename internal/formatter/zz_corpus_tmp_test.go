package formatter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func corpusOpts() Options {
	o := DefaultOptions()
	o.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
	o.ParseQuery = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
	o.ParseCFML = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }
	o.WhitespaceOnly = true
	return o
}

// classify reduces an error message to a bucket key.
var (
	reNear   = regexp.MustCompile(`near "[^"]*"`)
	reLine   = regexp.MustCompile(`line \d+`)
	reCol    = regexp.MustCompile(`col \d+`)
	reQuoted = regexp.MustCompile(`near \\?"?.*$`)
)

func classify(err error) string {
	m := err.Error()
	m = reNear.ReplaceAllString(m, "near ...")
	m = reLine.ReplaceAllString(m, "line N")
	m = reCol.ReplaceAllString(m, "col N")
	m = reQuoted.ReplaceAllString(m, "near ...")
	return m
}

func TestZZCorpus(t *testing.T) {
	root := os.Getenv("CORPUS")
	if root == "" {
		t.Skip("no CORPUS")
	}
	var files []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		e := strings.ToLower(filepath.Ext(p))
		if e == ".cfm" || e == ".cfc" || e == ".cfml" {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)

	opts := corpusOpts()
	noGuard := corpusOpts()
	noGuard.WhitespaceOnly = false

	type bucket struct {
		count   int
		samples []string
	}
	buckets := map[string]*bucket{}
	perProject := map[string][3]int{} // ok, parseErr, guardFail

	ok, parseErr, guardFail, nonIdem, panics := 0, 0, 0, 0, 0
	var nonIdemFiles []string

	add := func(key, sample string) {
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		b.count++
		if len(b.samples) < 3 {
			b.samples = append(b.samples, sample)
		}
	}

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, f)
		proj := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		st := perProject[proj]

		func() {
			defer func() {
				if r := recover(); r != nil {
					panics++
					add(fmt.Sprintf("PANIC: %v", r), rel)
				}
			}()
			tree := language.Parse(language.CFML, src, nil)
			out, ferr := Format(src, tree, opts)
			tree.Close()

			if ferr != nil {
				if strings.Contains(ferr.Error(), "parse error") {
					parseErr++
					st[1]++
					add("PARSE: "+classify(ferr), rel)
				} else {
					guardFail++
					st[2]++
					// re-run without the guard to get the real divergence
					tree2 := language.Parse(language.CFML, src, nil)
					raw, e2 := Format(src, tree2, noGuard)
					tree2.Close()
					key := "GUARD: unknown"
					if e2 == nil {
						if d := divergenceKinds(src, raw); d != "" {
							key = "GUARD: " + namedClass(d)
						}
					}
					add(key, rel)
				}
				perProject[proj] = st
				return
			}

			ok++
			st[0]++
			perProject[proj] = st

			tree3 := language.Parse(language.CFML, out, nil)
			out2, e := Format(out, tree3, noGuard)
			tree3.Close()
			if e == nil && string(out) != string(out2) {
				nonIdem++
				if len(nonIdemFiles) < 15 {
					nonIdemFiles = append(nonIdemFiles, rel)
				}
			}
		}()
	}

	fmt.Printf("\n=== %d files | %d ok | %d parse-refused | %d guard-rejected | %d panics | %d non-idempotent ===\n\n",
		len(files), ok, parseErr, guardFail, panics, nonIdem)

	fmt.Printf("%-22s %7s %7s %7s\n", "project", "ok", "parse", "guard")
	var projs []string
	for p := range perProject {
		projs = append(projs, p)
	}
	sort.Strings(projs)
	for _, p := range projs {
		st := perProject[p]
		fmt.Printf("%-22s %7d %7d %7d\n", p, st[0], st[1], st[2])
	}

	type kv struct {
		k string
		b *bucket
	}
	var all []kv
	for k, b := range buckets {
		all = append(all, kv{k, b})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].b.count > all[j].b.count })

	fmt.Printf("\n--- failure buckets ---\n")
	for i, e := range all {
		if i >= 40 {
			fmt.Printf("... %d more buckets\n", len(all)-40)
			break
		}
		fmt.Printf("%6d  %s\n          e.g. %s\n", e.b.count, e.k, strings.Join(e.b.samples, ", "))
	}

	if len(nonIdemFiles) > 0 {
		fmt.Printf("\n--- non-idempotent samples ---\n")
		for _, f := range nonIdemFiles {
			fmt.Printf("  %s\n", f)
		}
	}
}

// divergenceKinds summarises what actually changed, applying the same
// self-close and quote allowances the guard does so buckets show real causes.
func divergenceKinds(a, b []byte) string {
	as := normalizeStream(a)
	bs := normalizeStream(b)

	var kinds []string
	i, j := 0, 0
	seen := map[string]bool{}
	for i < len(as) && j < len(bs) {
		if as[i] == bs[j] {
			i++
			j++
			continue
		}
		// mirror checkWhitespaceOnly's allowSelfClose allowances
		if as[i] == '/' && i+1 < len(as) && as[i+1] == '>' && bs[j] == '>' {
			i++
			continue
		}
		if bs[j] == '/' && j+1 < len(bs) && bs[j+1] == '>' && as[i] == '>' {
			j++
			continue
		}
		if as[i] == '"' && bs[j] != '"' {
			i++
			continue
		}
		if bs[j] == '"' && as[i] != '"' {
			j++
			continue
		}
		if as[i] == '\'' && bs[j] != '\'' {
			i++
			continue
		}
		if bs[j] == '\'' && as[i] != '\'' {
			j++
			continue
		}
		k := fmt.Sprintf("src %q -> out %q", peek(as, i), peek(bs, j))
		if !seen[k] {
			seen[k] = true
			kinds = append(kinds, k)
		}
		if len(kinds) >= 1 {
			break
		}
		if resync(as, bs, &i, &j) {
			continue
		}
		break
	}
	if len(kinds) == 0 {
		if len(as) != len(bs) {
			return fmt.Sprintf("length %d -> %d (tail differs)", len(as), len(bs))
		}
		return ""
	}
	return strings.Join(kinds, " ; ")
}

// namedClass maps a raw divergence description onto a named bug class.
func namedClass(d string) string {
	switch {
	case strings.HasPrefix(d, "src \"\\ufeff"):
		return "A: UTF-8 BOM stripped"
	case regexp.MustCompile(`^src "any[a-z_$]`).MatchString(d):
		return "B: catch (any e) -> catch (e), type dropped"
	case strings.Contains(d, `-> out ";`), strings.Contains(d, `-> out ";}`):
		return "C: semicolon inserted"
	case strings.Contains(d, `-> out "localmode=`), regexp.MustCompile(`-> out "(output|localmode|hint|description|returnformat|access|modifier)=`).MatchString(d):
		return "D: function attribute hoisted before `function` keyword"
	case strings.Contains(d, `-> out "{`):
		return "E: brace inserted around single-statement body"
	case strings.Contains(d, `-> out ",`):
		return "F: leading comma inserted in literal"
	case strings.HasPrefix(d, `src "::`):
		return "G: :: static access rewritten to ."
	case strings.Contains(d, "admin>"), strings.Contains(d, "</cfcase>"), strings.Contains(d, "</cfif>"):
		return "H: tag re-nesting in cf tag files"
	case strings.HasPrefix(d, "length "):
		return "I: length mismatch (tail differs)"
	}
	return "Z: unclassified -- " + d
}

func normalizeStream(src []byte) string {
	var sb strings.Builder
	i := 0
	for i < len(src) {
		n := skipWSAndComments(src, i)
		if n > i {
			i = n
			continue
		}
		sb.WriteByte(toLower(src[i]))
		i++
	}
	return sb.String()
}

func peek(s string, i int) string {
	hi := i + 24
	if hi > len(s) {
		hi = len(s)
	}
	return s[i:hi]
}

func resync(as, bs string, i, j *int) bool {
	for w := 1; w <= 120; w++ {
		for di := 0; di <= w; di++ {
			dj := w - di
			if *i+di+12 <= len(as) && *j+dj+12 <= len(bs) && as[*i+di:*i+di+12] == bs[*j+dj:*j+dj+12] {
				*i += di
				*j += dj
				return true
			}
		}
	}
	return false
}

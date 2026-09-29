// Command cfstubgen generates the framework API stubs in
// internal/frameworkapi/stubs from each framework's own source.
//
//	cfstubgen -fetch target/framework-src          # what `make framework-stubs` runs
//	cfstubgen coldbox=/src/coldbox-platform ...     # from a checkout already on disk
//
// For each framework it starts from the components the preset names
// (config.PresetComponents), follows their extends chains and the components
// their methods return, as far as the framework's source reaches, and writes
// each as a script-syntax component holding every method's signature and doc
// comment with an empty body. Properties are kept, since the parser makes
// their accessors and a typed one says what a subclass's variable holds.
// Functions a component takes from an included template — how Wheels builds
// its controller — are folded into it.
//
// `make framework-stubs` clones each frameworkapi.Sources entry at its commit
// and runs this; the output is committed.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

func main() {
	out := flag.String("out", "internal/frameworkapi/stubs", "directory to write the stubs to")
	fetch := flag.String("fetch", "", "clone every frameworkapi.Sources entry at its commit under this directory and generate from all of them")

	flag.Parse()

	args := flag.Args()

	if *fetch != "" {
		for i := range frameworkapi.Sources {
			src := &frameworkapi.Sources[i]

			dir, err := checkout(src, *fetch)
			if err != nil {
				fail(err)
			}

			args = append(args, src.Framework+"="+dir)
		}
	}

	for _, arg := range args {
		fw, dir, ok := strings.Cut(arg, "=")
		if !ok {
			fail(fmt.Errorf("argument %q is not framework=dir", arg))
		}

		src, ok := sourceFor(fw)
		if !ok {
			fail(fmt.Errorf("no frameworkapi.Sources entry for %q", fw))
		}

		if err := generate(src, dir, *out); err != nil {
			fail(err)
		}
	}
}

// checkout makes <dir>/<framework> the source at src.Commit, fetching only
// that commit, and reuses a checkout already there.
func checkout(src *frameworkapi.Source, dir string) (string, error) {
	repo := filepath.Join(dir, src.Framework)

	if head, err := git(repo, "rev-parse", "HEAD"); err == nil && strings.TrimSpace(head) == src.Commit {
		return repo, nil
	}

	if err := os.RemoveAll(repo); err != nil {
		return "", err
	}

	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "fetch", "-q", "--depth", "1", src.Repo, src.Commit},
		{"-C", repo, "checkout", "-q", "FETCH_HEAD"},
	} {
		if _, err := git("", args...); err != nil {
			return "", fmt.Errorf("%s: %w", src.Framework, err)
		}
	}

	return repo, nil
}

func git(dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}

	out, err := exec.CommandContext(context.Background(), "git", args...).CombinedOutput() //nolint:gosec // the arguments are Sources entries, committed to this repository
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}

	return string(out), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cfstubgen:", err)
	os.Exit(1)
}

func sourceFor(fw string) (*frameworkapi.Source, bool) {
	for i := range frameworkapi.Sources {
		if strings.EqualFold(frameworkapi.Sources[i].Framework, fw) {
			return &frameworkapi.Sources[i], true
		}
	}

	return nil, false
}

type generator struct {
	src  *frameworkapi.Source
	root string // the directory Prefix stands for
	out  string // <out>/<framework>/<prefix>
	done map[string]bool
	todo []string
	// names indexes the source's components by file name, for byName.
	names map[string][]string
}

func generate(src *frameworkapi.Source, repo, out string) error {
	g := &generator{
		src:  src,
		root: filepath.Join(repo, filepath.FromSlash(src.Dir)),
		out:  filepath.Join(out, src.Framework, src.Prefix),
		done: map[string]bool{},
	}

	if err := os.RemoveAll(filepath.Join(out, src.Framework)); err != nil {
		return err
	}

	for _, c := range slices.Concat(config.PresetComponents(src.Framework), src.Extra) {
		if abs, _ := g.resolve(c, g.root); abs != "" {
			g.todo = append(g.todo, abs)
		} else if strings.HasPrefix(strings.ToLower(c), strings.ToLower(src.Prefix)+".") {
			return fmt.Errorf("%s: %s is not in %s", src.Framework, c, g.root)
		}
	}

	n := 0

	for len(g.todo) > 0 {
		abs := g.todo[0]
		g.todo = g.todo[1:]

		if g.done[abs] {
			continue
		}

		g.done[abs] = true

		if err := g.emit(abs); err != nil {
			return err
		}

		n++
	}

	fmt.Printf("%s: %d components\n", src.Framework, n)

	return nil
}

// resolve finds the component a dot-path names from a file in dir: through
// the prefix, relative to dir, or as a bare word beside it. It returns the
// file and its fully qualified dot-path, or "" when the framework's source
// does not hold it.
func (g *generator) resolve(dotted, dir string) (abs, qualified string) {
	dotted = strings.TrimSpace(dotted)
	if dotted == "" || strings.ContainsAny(dotted, "#$/ ") {
		return "", ""
	}

	segs := strings.Split(dotted, ".")

	if len(segs) > 1 && strings.EqualFold(segs[0], g.src.Prefix) {
		abs = findFold(g.root, segs[1:])
	}

	if abs == "" {
		abs = findFold(dir, segs)
	}

	if abs == "" {
		return "", ""
	}

	return abs, g.qualify(abs)
}

func (g *generator) qualify(abs string) string {
	rel, err := filepath.Rel(g.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}

	rel = strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))

	return g.src.Prefix + "." + strings.ReplaceAll(rel, "/", ".")
}

// findFold walks segs from dir matching names case-insensitively, the last
// with .cfc added.
func findFold(dir string, segs []string) string {
	for i, seg := range segs {
		if i == len(segs)-1 {
			seg += ".cfc"
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}

		found := ""

		for _, e := range entries {
			if strings.EqualFold(e.Name(), seg) {
				found = e.Name()

				if e.Name() == seg {
					break
				}
			}
		}

		if found == "" {
			return ""
		}

		dir = filepath.Join(dir, found)
	}

	if info, err := os.Stat(dir); err != nil || info.IsDir() {
		return ""
	}

	return dir
}

// source is one parsed file: its lines, for docs and modifiers, and its
// functions.
type source struct {
	// publicOnly keeps a mixin's private methods out: Wheels integrates only
	// the public ones.
	publicOnly bool
	path       string
	text       string
	lines      []string
	pr         *parser.ParseResult
}

func load(path string) (*source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := strings.TrimPrefix(string(data), "\ufeff")

	return &source{
		path:  path,
		text:  text,
		lines: strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"),
		pr:    parser.Parse(uri.URI("file://"+filepath.ToSlash(path)), text),
	}, nil
}

func (g *generator) emit(abs string) error {
	src, err := load(abs)
	if err != nil {
		return err
	}

	var b strings.Builder

	rel, _ := filepath.Rel(filepath.Dir(g.root), abs)
	if g.src.Dir == "" {
		rel, _ = filepath.Rel(g.root, abs)
	}

	fmt.Fprintf(&b, "// Generated by cmd/cfstubgen from %s@%.12s %s — do not edit.\n",
		g.src.Repo, g.src.Commit, filepath.ToSlash(rel))

	if doc := componentDoc(src); doc != "" {
		b.WriteString(docBlock(doc, ""))
	}

	b.WriteString("component")

	if ext := src.pr.Extends; ext != "" {
		if base, q := g.resolve(ext, filepath.Dir(abs)); base != "" {
			g.todo = append(g.todo, base)
			ext = q
		}

		fmt.Fprintf(&b, " extends=%q", ext)
	}

	b.WriteString(" {\n")

	for _, p := range properties(src) {
		b.WriteString(p)
	}

	seen := map[string]bool{}
	g.writeFuncs(&b, src, seen)

	for _, inc := range g.includes(src, map[string]bool{abs: true}) {
		g.writeFuncs(&b, inc, seen)
	}

	for _, mixin := range g.integrated(src) {
		g.writeFuncs(&b, mixin, seen)
	}

	b.WriteString("}\n")

	target := filepath.Join(g.out, filepath.FromSlash(strings.TrimPrefix(
		strings.ReplaceAll(g.qualify(abs), ".", "/"), g.src.Prefix+"/")+".cfc"))

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}

	return os.WriteFile(target, []byte(b.String()), 0o600)
}

// includes are the templates a file includes, and theirs, that the
// framework's source holds.
func (g *generator) includes(src *source, seen map[string]bool) []*source {
	var out []*source

	for _, inc := range parser.ExtractIncludes(src.text) {
		var p string

		if rest, ok := strings.CutPrefix(inc, "/"); ok {
			seg, tail, _ := strings.Cut(rest, "/")
			if !strings.EqualFold(seg, g.src.Prefix) {
				continue
			}

			p = filepath.Join(g.root, filepath.FromSlash(tail))
		} else {
			p = filepath.Join(filepath.Dir(src.path), filepath.FromSlash(inc))
		}

		if seen[p] {
			continue
		}

		seen[p] = true

		s, err := load(p)
		if err != nil {
			continue
		}

		out = append(out, s)
		out = append(out, g.includes(s, seen)...)
	}

	return out
}

var integrateRe = regexp.MustCompile(`\$integrateComponents\(\s*["']([\w.]+)["']\s*\)`)

// integrated are the components a Wheels class mixes in at run time —
// `$integrateComponents("wheels.controller")` adds the public methods of
// every component in that package — so the stub declares them.
func (g *generator) integrated(src *source) []*source {
	var out []*source

	for _, m := range integrateRe.FindAllStringSubmatch(src.text, -1) {
		segs := strings.Split(m[1], ".")
		if len(segs) < 2 || !strings.EqualFold(segs[0], g.src.Prefix) {
			continue
		}

		dir := filepath.Join(append([]string{g.root}, segs[1:]...)...)

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".cfc") {
				continue
			}

			if s, err := load(filepath.Join(dir, e.Name())); err == nil {
				s.publicOnly = true
				out = append(out, s)
			}
		}
	}

	return out
}

func (g *generator) writeFuncs(b *strings.Builder, src *source, seen map[string]bool) {
	for i := range src.pr.Funcs {
		def := &src.pr.Funcs[i]
		if seen[strings.ToLower(def.Name)] || !declares(src, def) {
			continue
		}

		access := accessOf(src, def)
		if src.publicOnly && access == "private" {
			continue
		}

		seen[strings.ToLower(def.Name)] = true

		if doc := funcDoc(src, def); doc != "" {
			b.WriteString(docBlock(doc, "\t"))
		}

		b.WriteString("\t")

		if access != "" {
			b.WriteString(access + " ")
		}

		if ret := g.returnType(src, def); ret != "" {
			b.WriteString(ret + " ")
		}

		fmt.Fprintf(b, "function %s(", def.Name)

		for j, a := range def.Arguments {
			if j > 0 {
				b.WriteString(",")
			}

			b.WriteString(" ")

			if a.Required {
				b.WriteString("required ")
			}

			if validType(a.Type) {
				b.WriteString(a.Type + " ")
			}

			b.WriteString(a.Name)
		}

		if len(def.Arguments) > 0 {
			b.WriteString(" ")
		}

		b.WriteString(") {}\n")
	}
}

// declares reports whether def is written in the file, rather than an
// accessor the parser made for a property — those come back from the
// property itself.
func declares(src *source, def *parser.FunctionDef) bool {
	if int(def.Line) >= len(src.lines) {
		return false
	}

	line := strings.ToLower(src.lines[def.Line])

	return strings.Contains(line, "function") && strings.Contains(line, strings.ToLower(def.Name))
}

var validTypeRe = regexp.MustCompile(`^[A-Za-z_][\w.]*(\[\])?$`)

func validType(t string) bool { return t != "" && validTypeRe.MatchString(t) }

// returnType is def's return type, qualified when it names a component in
// the framework's source so the stub's reader need not find it beside the
// stub: a declared one, or the component the parse inferred from its return
// statements.
func (g *generator) returnType(src *source, def *parser.FunctionDef) string {
	dir := filepath.Dir(src.path)

	if t := def.ReturnType; validType(t) {
		if abs, q := g.resolve(t, dir); abs != "" {
			g.todo = append(g.todo, abs)

			return q
		}

		return t
	}

	switch rc := def.ReturnComponent; {
	case rc == "":
	case rc == src.path || strings.EqualFold(filepath.Clean(rc), filepath.Clean(src.path)):
		return g.qualify(src.path)
	default:
		if abs, q := g.resolve(rc, dir); abs != "" {
			g.todo = append(g.todo, abs)

			return q
		}

		if validType(rc) && strings.Contains(rc, ".") {
			return rc
		}
	}

	return g.docReturn(src, def)
}

var docReturnRe = regexp.MustCompile(`(?im)^@returns?\s+([A-Za-z_]\w*(?:\.\w+)+)\b`)

// docReturn is the component def's doc comment says it returns, when its
// declaration says nothing: ColdBox writes `function execute(...)` and
// documents `@return coldbox.system.context.RequestContext`. Only a dotted
// name counts — a bare word is `struct` far more often than a component — and
// only one the framework's source holds: by its path, or, for a path the doc
// spells wrong (that one: the class lives in web.context), by its file name
// when exactly one file in the source has it.
func (g *generator) docReturn(src *source, def *parser.FunctionDef) string {
	m := docReturnRe.FindStringSubmatch(funcDoc(src, def))
	if m == nil {
		return ""
	}

	abs, q := g.resolve(m[1], filepath.Dir(src.path))
	if abs == "" {
		abs = g.byName(m[1][strings.LastIndexByte(m[1], '.')+1:])
		q = g.qualify(abs)
	}

	if abs == "" || q == "" || isInterface(abs) {
		return ""
	}

	g.todo = append(g.todo, abs)

	return q
}

var interfaceRe = regexp.MustCompile(`(?im)^\s*interface\b|<cfinterface\b`)

// isInterface reports whether the file declares an interface. A doc comment
// naming one says less than the call returns: CacheFactory's getCache() is
// documented as an ICacheProvider, and every provider it hands back has
// getOrSet(), which the interface does not declare. Typing the call by it
// would report that as missing.
func isInterface(abs string) bool {
	data, err := os.ReadFile(abs)

	return err == nil && interfaceRe.Match(data)
}

// byName is the one file in the framework's source named name.cfc, or "".
func (g *generator) byName(name string) string {
	if g.names == nil {
		g.names = map[string][]string{}

		_ = filepath.WalkDir(g.root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".cfc") {
				k := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
				g.names[k] = append(g.names[k], p)
			}

			return nil
		})
	}

	if hits := g.names[strings.ToLower(name)]; len(hits) == 1 {
		return hits[0]
	}

	return ""
}

var (
	accessRe    = regexp.MustCompile(`(?i)\b(public|private|package|remote)\b[^(]*\bfunction\b`)
	tagAccessRe = regexp.MustCompile(`(?i)\baccess\s*=\s*["']?(public|private|package|remote)`)
)

func accessOf(src *source, def *parser.FunctionDef) string {
	tag := tagText(src, int(def.Line))
	if tag != "" {
		if m := tagAccessRe.FindStringSubmatch(tag); m != nil {
			return strings.ToLower(m[1])
		}

		return ""
	}

	if m := accessRe.FindStringSubmatch(src.lines[def.Line]); m != nil {
		return strings.ToLower(m[1])
	}

	return ""
}

// tagText is the <cffunction> tag starting on line, or "" for a script
// function.
func tagText(src *source, line int) string {
	if !strings.Contains(strings.ToLower(src.lines[line]), "<cffunction") {
		return ""
	}

	var b strings.Builder

	for i := line; i < len(src.lines) && i < line+20; i++ {
		b.WriteString(src.lines[i])
		b.WriteString(" ")

		if strings.Contains(src.lines[i], ">") {
			break
		}
	}

	return b.String()
}

var hintRe = regexp.MustCompile(`(?is)\bhint\s*=\s*"([^"]*)"|\bhint\s*=\s*'([^']*)'`)

func attrHint(tag string) string {
	m := hintRe.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}

	return strings.TrimSpace(m[1] + m[2])
}

// funcDoc is the documentation written for def: the doc comment above a
// script function, or a tag function's hint and its arguments'.
func funcDoc(src *source, def *parser.FunctionDef) string {
	if tag := tagText(src, int(def.Line)); tag != "" {
		var parts []string

		if h := attrHint(tag); h != "" {
			parts = append(parts, h)
		}

		for _, a := range def.Arguments {
			if a.Hint != "" {
				parts = append(parts, "@"+a.Name+" "+a.Hint)
			}
		}

		return strings.Join(parts, "\n")
	}

	return commentAbove(src.lines, int(def.Line))
}

// commentAbove is the /** */ or /* */ block ending just above line, blank
// lines and annotation lines (`@foo bar` in no comment) aside, without its
// delimiters and leading stars.
func commentAbove(lines []string, line int) string {
	end := line - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}

	if end < 0 || !strings.HasSuffix(strings.TrimSpace(lines[end]), "*/") {
		return ""
	}

	start := end
	for start >= 0 && !strings.Contains(lines[start], "/*") {
		start--
	}

	if start < 0 {
		return ""
	}

	var out []string

	for i := start; i <= end; i++ {
		l := strings.TrimSpace(lines[i])
		if i == start {
			_, l, _ = strings.Cut(l, "/*")
			l = strings.TrimLeft(l, "*")
		}

		l = strings.TrimSuffix(l, "*/")
		l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))

		if strings.Trim(l, "*/=- ") == "" && strings.ContainsAny(l, "*=") {
			continue // a ruler
		}

		out = append(out, l)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

var componentRe = regexp.MustCompile(`(?im)^\s*(abstract\s+|final\s+)?component\b|<cfcomponent\b`)

func componentDoc(src *source) string {
	loc := componentRe.FindStringIndex(src.text)
	if loc == nil {
		return ""
	}

	line := strings.Count(src.text[:loc[0]], "\n")
	if strings.Contains(strings.ToLower(src.lines[line]), "<cfcomponent") {
		return attrHint(tagText2(src.lines, line, "<cfcomponent"))
	}

	return commentAbove(src.lines, line)
}

func tagText2(lines []string, line int, _ string) string {
	var b strings.Builder

	for i := line; i < len(lines) && i < line+20; i++ {
		b.WriteString(lines[i] + " ")

		if strings.Contains(lines[i], ">") {
			break
		}
	}

	return b.String()
}

var (
	scriptPropRe = regexp.MustCompile(`(?ims)^[ \t]*property\s+[^;{}]*?;`)
	tagPropRe    = regexp.MustCompile(`(?is)<cfproperty\b([^>]*?)/?>`)
)

// properties are the file's property declarations, as script statements
// with their doc comments.
func properties(src *source) []string {
	script := scriptPropRe.FindAllStringIndex(src.text, -1)
	tags := tagPropRe.FindAllStringSubmatch(src.text, -1)
	out := make([]string, 0, len(script)+len(tags))

	for _, loc := range script {
		line := strings.Count(src.text[:loc[0]], "\n")
		stmt := strings.Join(strings.Fields(src.text[loc[0]:loc[1]]), " ")

		p := ""
		if doc := commentAbove(src.lines, line); doc != "" {
			p = docBlock(doc, "\t")
		}

		out = append(out, p+"\t"+stmt+"\n")
	}

	for _, m := range tags {
		out = append(out, "\tproperty "+strings.Join(strings.Fields(m[1]), " ")+";\n")
	}

	return out
}

func docBlock(doc, indent string) string {
	var b strings.Builder

	b.WriteString(indent + "/**\n")

	for l := range strings.SplitSeq(doc, "\n") {
		l = strings.ReplaceAll(l, "*/", "* /")
		if l == "" {
			b.WriteString(indent + " *\n")
		} else {
			b.WriteString(indent + " * " + l + "\n")
		}
	}

	b.WriteString(indent + " */\n")

	return b.String()
}

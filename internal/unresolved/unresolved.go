// Package unresolved finds the calls the resolver cannot follow to a
// definition, and writes them as a known-issues file. The `cfmleditor-lsp
// unresolved` command and the server's cfmleditor.exportUnresolved command
// both run it, so the report is the same whichever wrote it.
package unresolved

import (
	"cmp"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/knownissues"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// Call is one call the resolver could not follow.
type Call struct {
	File     string `json:"file"`
	Line     uint32 `json:"line"`
	Caller   string `json:"caller,omitempty"`
	Variable string `json:"variable,omitempty"`
	Function string `json:"function"`
	Reason   string `json:"reason"`
	Text     string `json:"text"`
	// Unchecked is, on the one entry for a file whose extends chain breaks,
	// how many inherited calls that leaves unchecked. Function holds the
	// base that does not resolve.
	Unchecked int `json:"unchecked,omitempty"`
}

// Options is what a scan resolves with: the .cfmleditor.json settings that
// shape parsing and resolution.
type Options struct {
	Resolvers                []parser.Resolver
	Mappings                 map[string]string
	StartupFiles             []string
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string
	PropertyResolvers        []parser.PropertyResolver
	BeanPaths                map[string]string // configured beanPaths; Application.cfc's are added
	WorkspaceFolders         []string
	ImplicitExtends          func(path string) string // frameworks' implicit bases; see config.ImplicitExtends
	HelperScope              func(path string) bool   // files frameworks mix helpers into; see config.HelperScope
	Stubs                    *frameworkapi.Set        // frameworks' API when their source is absent; see internal/frameworkapi
	InterpolateAll           bool                     // features.outputContextInterpolation off
	GlobalDefs               bool                     // accept a bare call any indexed file defines
	InferArgs                bool                     // type an untyped argument from what every caller passes it (resolve.Resolver.InferArgsFiles)
	Verbose                  io.Writer
}

// Report is a scan's result.
type Report struct {
	Calls     []Call
	Resolved  int
	Indexed   int
	Scanned   int
	IndexTime time.Duration
	ScanTime  time.Duration
}

// Scan indexes files, then reports the unresolved calls in targets, or in
// every one of files when targets is empty.
func Scan(fsys vfs.FS, files, targets []string, opt *Options) Report {
	started := time.Now()
	resolver := NewResolver(fsys, files, opt)
	rep := Report{Indexed: len(files), IndexTime: time.Since(started)}

	if len(targets) == 0 {
		targets = files
	}

	rep.Scanned = len(targets)
	started = time.Now()

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	sem := make(chan struct{}, runtime.GOMAXPROCS(0))

	for _, f := range targets {
		wg.Add(1)

		sem <- struct{}{}

		go func(file string) {
			defer wg.Done()
			defer func() { <-sem }()

			calls, resolved := scanFile(fsys, resolver, file, opt)

			mu.Lock()

			rep.Calls = append(rep.Calls, calls...)
			rep.Resolved += resolved
			mu.Unlock()
		}(f)
	}

	wg.Wait()

	rep.ScanTime = time.Since(started)

	// Every field takes part, so two calls on one line come out in the same
	// order whichever goroutine finished first: a report is diffed against
	// an earlier one.
	sort.Slice(rep.Calls, func(i, j int) bool {
		a, b := &rep.Calls[i], &rep.Calls[j]

		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Variable, b.Variable),
			cmp.Compare(a.Function, b.Function),
			cmp.Compare(a.Reason, b.Reason),
			cmp.Compare(a.Caller, b.Caller),
			cmp.Compare(a.Text, b.Text),
			cmp.Compare(a.Unchecked, b.Unchecked),
		) < 0
	})

	return rep
}

// NewResolver is the resolver a scan checks calls with, its index built from
// files. The explain command builds its resolver here too, so a trace and the
// report it explains cannot disagree: explain used to build its own, without
// beanPaths or the setter and constructor policies, and rejected calls the
// report accepted.
func NewResolver(fsys vfs.FS, files []string, opt *Options) *resolve.Resolver {
	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          opt.Resolvers,
		Mappings:           opt.Mappings,
		StartupFiles:       opt.StartupFiles,
		BeanPaths:          opt.BeanPaths,
		ExpressionMappings: opt.ExpressionMappings,
		WorkspaceFolders:   opt.WorkspaceFolders,
		ImplicitExtends:    opt.ImplicitExtends,
		HelperScope:        opt.HelperScope,
		Stubs:              opt.Stubs,
	}

	if opt.InferArgs {
		resolver.InferArgsFiles = files
	}

	resolver.Resolvers = resolver.BeanResolvers(opt.BeanPaths)
	// Discovery lazily indexes factory metadata with the original rules.
	// The scan must index every return using the augmented rules.
	resolver.Index = index.New()
	loadBeans(resolver, opt)

	// Each file is read and parsed on its own, and what a parse looks up (the
	// bean map and the DI policies) is settled before the first one starts,
	// so the files are indexed in parallel. Serially this was most of a
	// scan's wall time on a large workspace.
	work := make(chan string)

	var wg sync.WaitGroup

	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for f := range work {
				indexOne(fsys, resolver, f, opt)
			}
		})
	}

	for _, f := range files {
		work <- f
	}

	close(work)
	wg.Wait()

	return resolver
}

func indexOne(fsys vfs.FS, resolver *resolve.Resolver, f string, opt *Options) {
	data, err := fsys.ReadFile(f)
	if err != nil || cfpath.IsBinary(data) {
		return
	}

	content := string(data)
	resolver.IndexCallerFile(f, content)

	fileURI := uri.URI("file://" + f)

	// A template is not indexed for its functions here, but what it
	// includes is part of the include graph a bare call resolves through:
	// a page that includes a helper can call what the helper declares.
	if !cfpath.IsCFCFile(f) {
		resolver.Index.SetIncludes(fileURI, parser.ExtractIncludes(content))

		return
	}

	resolver.Index.IndexFileWithOptions(fileURI, content, &parser.ParseOptions{Resolvers: resolver.Resolvers, SetterLookup: resolver.SetterLookup(f), ConstructorLookup: resolver.ConstructorLookup(f), BeanLookup: resolver.InjectionBeanLookup(f), PropertyBeanLookup: resolver.InjectionPropertyLookup(f), PropertyResolvers: opt.PropertyResolvers})
}

// loadBeans gives the resolver's index the bean map the server would build
// for the same workspace, so an injected property is typed the same way in
// the report as in the editor. The report used to ignore beanPaths and
// propertyResolvers altogether.
func loadBeans(resolver *resolve.Resolver, opt *Options) {
	appDirs := make([]string, 0, len(opt.WorkspaceFolders))
	for _, root := range opt.WorkspaceFolders {
		appDirs = append(appDirs, resolver.FindApplicationRoot(root))
	}

	if all := cfpath.BeanPathsFor(opt.BeanPaths, appDirs); len(all) > 0 {
		resolver.Index.SetBeans(cfpath.BuildBeanMap(all, resolver.FS))
	}
}

func scanFile(fsys vfs.FS, resolver *resolve.Resolver, file string, opt *Options) (out []Call, resolved int) {
	data, err := fsys.ReadFile(file)
	if err != nil || cfpath.IsBinary(data) {
		return nil, 0
	}

	baseDir := filepath.Dir(file)
	pr := Parse(resolver, file, string(data), opt)
	calls := pr.AllCalls()

	// Calls into a base that does not resolve are one finding per file and
	// base, not one per call: see resolve.MissingBaseReason. The base is the
	// file's own, for an inherited call, or a receiver's, for a call on a
	// component whose chain breaks.
	var bases baseGroups

	for i := range calls {
		call := &calls[i]

		reason := resolver.CanResolveCall(call, pr, baseDir)
		if reason == "" {
			resolved++

			if opt.Verbose != nil {
				_, _ = fmt.Fprintf(opt.Verbose, "  ✓ %s:%d: %s.%s\n", filepath.Base(file), call.Line+1, call.Variable, call.FuncName)
			}

			continue
		}

		if IsBuiltin(call.FuncName) {
			continue
		}

		if opt.GlobalDefs && resolver.Index.CountFunctions(call.FuncName) > 0 {
			resolved++

			if opt.Verbose != nil {
				_, _ = fmt.Fprintf(opt.Verbose, "  ✓ %s:%d: %s (global def)\n", filepath.Base(file), call.Line+1, call.FuncName)
			}

			continue
		}

		if base, ok := resolve.MissingBaseOf(reason); ok {
			bases.add(base, call)

			continue
		}

		out = append(out, Call{
			File:     file,
			Line:     call.Line,
			Caller:   call.Caller,
			Variable: call.Variable,
			Function: call.FuncName,
			Reason:   reason,
			Text:     call.Text,
		})
	}

	if len(bases.order) > 0 {
		own := ""
		if pr.Extends != "" {
			own = resolver.MissingBase(pr.Extends, baseDir)
		}

		for _, g := range bases.order {
			if strings.EqualFold(g.base, own) {
				out = append(out, missingBaseCall(file, string(data), pr.Extends, g.base, g.calls))
			} else {
				out = append(out, receiverBaseCall(file, g))
			}
		}
	}

	return out, resolved
}

// baseGroup is the calls in one file left unchecked by one missing base, and
// the first of them.
type baseGroup struct {
	first *parser.CallSite
	base  string
	calls int
}

// baseGroups collects baseGroup by base, in the order each base is first met.
type baseGroups struct {
	by    map[string]*baseGroup
	order []*baseGroup
}

func (b *baseGroups) add(base string, call *parser.CallSite) {
	key := strings.ToLower(base)
	if g, ok := b.by[key]; ok {
		g.calls++

		return
	}

	if b.by == nil {
		b.by = map[string]*baseGroup{}
	}

	g := &baseGroup{base: base, calls: 1, first: call}
	b.by[key] = g
	b.order = append(b.order, g)
}

// receiverBaseCall is the one entry for the calls in a file made on
// components whose extends chain breaks at a base that is not the file's own:
// ContentBox's services extend cborm's VirtualEntityService, and with cborm
// missing every findWhere and save on one is unchecked. It sits on the first
// such call.
func receiverBaseCall(file string, g *baseGroup) Call {
	return Call{
		File:      file,
		Line:      g.first.Line,
		Caller:    g.first.Caller,
		Function:  g.base,
		Reason:    fmt.Sprintf("calls a component whose chain breaks at %s, which does not resolve; %d %s not checked", g.base, g.calls, plural(g.calls, "call", "calls")),
		Text:      g.first.Text,
		Unchecked: g.calls,
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

// missingBaseCall is the one entry for a file whose extends chain breaks at
// base, on the line of the file's own extends, which is where the chain
// starts: base may be further up it, in a component the file never names.
func missingBaseCall(file, content, extends, base string, unchecked int) Call {
	line := uint32(0)
	lowerExtends := strings.ToLower(extends)

	for i, l := range strings.Split(content, "\n") {
		if l = strings.ToLower(l); strings.Contains(l, "extends") && strings.Contains(l, lowerExtends) {
			line = conv.Uint32(i)

			break
		}
	}

	noun := plural(unchecked, "call", "calls")

	reason := fmt.Sprintf("base component does not resolve; %d inherited %s not checked", unchecked, noun)
	if !strings.EqualFold(extends, base) {
		reason = fmt.Sprintf("extends %s, whose chain breaks at %s, which does not resolve; %d inherited %s not checked", extends, base, unchecked, noun)
	}

	return Call{
		File:      file,
		Line:      line,
		Function:  base,
		Reason:    reason,
		Text:      "extends " + extends,
		Unchecked: unchecked,
	}
}

// MissingBase is one base component that does not resolve, across a report.
type MissingBase struct {
	Component string
	Files     int
	Calls     int
}

// MissingBases totals the report's missing bases, most calls first. Each is
// usually one mapping or workspace path away from checking every call it
// accounts for.
func MissingBases(calls []Call) []MissingBase {
	by := map[string]*MissingBase{}

	for i := range calls {
		c := &calls[i]
		if c.Unchecked == 0 {
			continue
		}

		key := strings.ToLower(c.Function)
		if by[key] == nil {
			by[key] = &MissingBase{Component: c.Function}
		}

		by[key].Files++
		by[key].Calls += c.Unchecked
	}

	out := make([]MissingBase, 0, len(by))
	for _, m := range by {
		out = append(out, *m)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}

		return out[i].Component < out[j].Component
	})

	return out
}

// IsBuiltin reports whether name is a built-in function or member function,
// which the report leaves out.
func IsBuiltin(name string) bool {
	if docs.IsBuiltinFunction(name) {
		return true
	}

	if parser.IsMemberMethod(name) {
		return true
	}

	return IsMemberFunction(name)
}

var memberFuncSet = func() map[string]bool {
	m := make(map[string]bool)
	for _, mf := range docs.AllMemberFunctions() {
		m[strings.ToLower(mf.Name)] = true
	}

	return m
}()

// IsMemberFunction reports whether name is a documented member function.
func IsMemberFunction(name string) bool {
	return memberFuncSet[strings.ToLower(name)]
}

// CallText is how a call is written in the report: variable.function.
func (c *Call) CallText() string {
	if c.Variable != "" {
		return c.Variable + "." + c.Function
	}

	return c.Function
}

// RegenerateHint is the report header's how-to-regenerate line when it is
// written to its configured files. It is the same whichever writes it, so a
// report regenerated from the editor and from the command line diffs clean.
const RegenerateHint = "cfmleditor-lsp unresolved --write <project>, or the editor's cfmleditor.exportUnresolved command"

// WriteKnownIssues writes calls as a known-issues file: the unresolved line
// format with paths relative to baseDir, sorted by path and line, under a #
// header saying how to regenerate and use it. Listed under knownIssues in a
// .cfmleditor.json, the server publishes each entry as a diagnostic.
//
// A path is never written absolute: the file is committed to one project and
// read on other machines, where it would not name the same file. A call
// outside baseDir, a workspacePaths neighbour, is left out and counted, unless
// includeWorkspace asks for it, when it is written as a ../ path the way
// workspacePaths name it. One that cannot be made relative at all, on another
// volume, is always left out.
//
// regenerate is the header's how-to-regenerate line. The header carries no
// date, so regenerating an unchanged report changes nothing a diff would show.
func WriteKnownIssues(w io.Writer, calls []Call, baseDir string, includeWorkspace bool, regenerate, version string) (skipped int) {
	type row struct {
		path string
		line int
		text string
	}

	rows := make([]row, 0, len(calls))

	for i := range calls {
		c := &calls[i]

		rel, ok := relativePath(baseDir, c.File)
		if !ok || (isOutside(rel) && !includeWorkspace) {
			skipped++

			continue
		}

		rows = append(rows, row{path: filepath.ToSlash(rel), line: int(c.Line) + 1, text: c.CallText() + " (" + c.Reason + ")"})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].path != rows[j].path {
			return rows[i].path < rows[j].path
		}

		if rows[i].line != rows[j].line {
			return rows[i].line < rows[j].line
		}

		return rows[i].text < rows[j].text
	})

	_, _ = fmt.Fprintf(w, "# Calls cfmleditor-lsp cannot resolve, one per line: path:line: call (reason).\n")
	_, _ = fmt.Fprintf(w, "# Paths are relative to this file's directory. List the file under knownIssues in\n")
	_, _ = fmt.Fprintf(w, "# .cfmleditor.json to show the entries as editor diagnostics. Regenerate with:\n")
	_, _ = fmt.Fprintf(w, "#   %s\n", regenerate)
	_, _ = fmt.Fprintf(w, "# %d entries; cfmleditor-lsp %s.\n", len(rows), version)

	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s:%d: %s\n", r.path, r.line, r.text)
	}

	return skipped
}

// SplitByTarget assigns each call to the target file whose directory holds it,
// the deepest when directories nest, for writing one report per target. A
// call under none of them is returned in rest.
func SplitByTarget(calls []Call, targets []string) (byTarget map[string][]Call, rest []Call) {
	byTarget = make(map[string][]Call, len(targets))
	for _, t := range targets {
		byTarget[t] = nil
	}

	for i := range calls {
		c := &calls[i]

		best := knownissues.DeepestTarget(c.File, targets)
		if best == "" {
			rest = append(rest, *c)

			continue
		}

		byTarget[best] = append(byTarget[best], *c)
	}

	return byTarget, rest
}

func relativePath(base, file string) (string, bool) {
	rel, err := filepath.Rel(base, file)
	if err != nil || filepath.IsAbs(rel) {
		return "", false
	}

	return rel, true
}

func isOutside(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Parse is file parsed as a scan checks it: with the resolver's lookups, so
// what a variable is assigned from is typed the same way in both commands.
func Parse(resolver *resolve.Resolver, file, content string, opt *Options) *parser.ParseResult {
	funcLookup := resolver.FuncLookup(filepath.Dir(file))

	pr := parser.ParseWithOptions(uri.URI("file://"+file), content, &parser.ParseOptions{
		Resolvers:                resolver.Resolvers,
		ExpressionMappings:       opt.ExpressionMappings,
		ServicePropertyResolvers: opt.ServicePropertyResolvers,
		PropertyResolvers:        opt.PropertyResolvers,
		BeanLookup:               resolver.InjectionBeanLookup(file),
		PropertyBeanLookup:       resolver.InjectionPropertyLookup(file),
		SetterLookup:             resolver.SetterLookup(file),
		ConstructorLookup:        resolver.ConstructorLookup(file),
		InterpolateAllText:       opt.InterpolateAll,
		ExtractCalls:             true,
		ScanAllScopes:            true,
		FuncLookup:               funcLookup,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
	})

	pr.FuncLookup = funcLookup

	return pr
}

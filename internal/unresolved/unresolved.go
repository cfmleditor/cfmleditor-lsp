// Package unresolved finds the calls the resolver cannot follow to a
// definition, and writes them as a known-issues file. The `cfmleditor-lsp
// unresolved` command and the server's cfmleditor.exportUnresolved command
// both run it, so the report is the same whichever wrote it.
package unresolved

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
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
}

// Options is what a scan resolves with: the .cfmleditor.json settings that
// shape parsing and resolution.
type Options struct {
	Resolvers                []parser.Resolver
	Mappings                 map[string]string
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string
	WorkspaceFolders         []string
	InterpolateAll           bool // features.outputContextInterpolation off
	GlobalDefs               bool // accept a bare call any indexed file defines
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
func Scan(fsys vfs.FS, files, targets []string, opt Options) Report {
	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          opt.Resolvers,
		Mappings:           opt.Mappings,
		ExpressionMappings: opt.ExpressionMappings,
		WorkspaceFolders:   opt.WorkspaceFolders,
	}

	started := time.Now()

	for _, f := range files {
		data, err := fsys.ReadFile(f)
		if err != nil || cfpath.IsBinary(data) {
			continue
		}

		fileURI := uri.URI("file://" + f)

		// A template is not indexed for its functions here, but what it
		// includes is part of the include graph a bare call resolves through:
		// a page that includes a helper can call what the helper declares.
		if !cfpath.IsCFCFile(f) {
			resolver.Index.SetIncludes(fileURI, parser.ExtractIncludes(string(data)))

			continue
		}

		resolver.Index.IndexFile(fileURI, string(data))
	}

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

	sem := make(chan struct{}, 8)

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

	sort.Slice(rep.Calls, func(i, j int) bool {
		if rep.Calls[i].File != rep.Calls[j].File {
			return rep.Calls[i].File < rep.Calls[j].File
		}

		return rep.Calls[i].Line < rep.Calls[j].Line
	})

	return rep
}

func scanFile(fsys vfs.FS, resolver *resolve.Resolver, file string, opt Options) (out []Call, resolved int) {
	data, err := fsys.ReadFile(file)
	if err != nil || cfpath.IsBinary(data) {
		return nil, 0
	}

	fileURI := uri.URI("file://" + file)
	baseDir := filepath.Dir(file)

	funcLookup := func(component, funcName string) string {
		fd := resolver.ResolveFunc(component, funcName, baseDir)
		if fd == nil {
			return ""
		}

		if fd.ReturnComponent != "" {
			return fd.ReturnComponent
		}

		if fd.ReturnType != "" && strings.Contains(fd.ReturnType, ".") {
			return fd.ReturnType
		}

		return ""
	}

	pr := parser.ParseWithOptions(fileURI, string(data), &parser.ParseOptions{
		Resolvers:                opt.Resolvers,
		ExpressionMappings:       opt.ExpressionMappings,
		ServicePropertyResolvers: opt.ServicePropertyResolvers,
		InterpolateAllText:       opt.InterpolateAll,
		ExtractCalls:             true,
		ScanAllScopes:            true,
		FuncLookup:               funcLookup,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
	})

	pr.FuncLookup = funcLookup

	for _, call := range pr.AllCalls() {
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

	return out, resolved
}

// IsBuiltin reports whether name is a built-in function or member function,
// which the report leaves out.
func IsBuiltin(name string) bool {
	if _, ok := docs.LookupFunction(name); ok {
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
func (c Call) CallText() string {
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

	for _, c := range calls {
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

	for _, c := range calls {
		best := knownissues.DeepestTarget(c.File, targets)
		if best == "" {
			rest = append(rest, c)

			continue
		}

		byTarget[best] = append(byTarget[best], c)
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

package unresolved

import (
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/cfmleditor/clif/internal/frameworkapi"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
	"go.lsp.dev/uri"
)

// Candidates are the liberal matches a scan run with Options.Candidates
// attaches to a finding. They never resolve it: the finding stays in the
// report, and each candidate says why it was offered and how far it can be
// trusted. They are the unreliable connections a strict resolver refuses, for
// a person to weigh — the method may be declared somewhere the resolver cannot
// reach, or the receiver may be a component its use describes.

// Category says which link a finding is missing.
const (
	CategoryObject     = "object"      // a component path names no file, or a chain breaks at one
	CategoryVariable   = "variable"    // a receiver whose component is unknown
	CategoryReturnType = "return-type" // a chained call on a method that declares no component
	CategoryMethod     = "method"      // a method not found where it was looked for
)

// Confidence levels, most trusted first.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// maxCandidates is how many candidates a finding lists; CandidateCount
// says how many there were.
const maxCandidates = 8

// Candidate is one possible answer for a finding.
type Candidate struct {
	File       string   `json:"file"`
	Line       uint32   `json:"line,omitempty"`
	Confidence string   `json:"confidence"`
	Basis      []string `json:"basis"`
}

// Category classifies a finding's reason.
func Category(reason string) string {
	switch {
	case strings.HasPrefix(reason, "variable "):
		return CategoryVariable
	case strings.Contains(reason, "has no component return type"):
		return CategoryReturnType
	case strings.HasPrefix(reason, "component "), strings.HasPrefix(reason, "chained on "),
		strings.Contains(reason, "does not resolve"), strings.Contains(reason, "chain breaks"):
		return CategoryObject
	default:
		return CategoryMethod
	}
}

var (
	methodNotFoundRe = regexp.MustCompile(`^method '[^']+' not found in (.+)$`)
	componentRe      = regexp.MustCompile(`^component '([^']+)' does not exist`)
)

// annotateAll sets the category and candidates of every finding, after the
// scan: annotating during it would read an index the parallel scan is still
// filling lazily, so the same workspace gave different candidates from run to
// run. Every file is indexed first, and each file with findings is parsed
// again; the findings themselves are the scan's, untouched.
func annotateAll(fsys vfs.FS, resolver *resolve.Resolver, files []string, calls []Call, opt *Options) {
	parallel(files, func(f string) { resolver.EnsureIndexed(f) })

	byFile := map[string][]int{}

	var order []string

	for i := range calls {
		f := calls[i].File
		if _, ok := byFile[f]; !ok {
			order = append(order, f)
		}

		byFile[f] = append(byFile[f], i)
	}

	parallel(order, func(file string) {
		data, err := fsys.ReadFile(file)
		if err != nil {
			return
		}

		pr := Parse(resolver, file, string(data), opt)
		sites := pr.AllCalls()
		a := &annotator{resolver: resolver, pr: pr, calls: sites, file: file}

		for _, i := range byFile[file] {
			c := &calls[i]
			if c.Unchecked > 0 || c.Category != "" {
				c.Category = CategoryObject

				continue
			}

			if site := matchingSite(sites, c); site != nil {
				a.annotate(c, site)
			} else {
				c.Category = Category(c.Reason)
			}
		}
	})
}

// matchingSite is the call site a finding was made from.
func matchingSite(sites []parser.CallSite, c *Call) *parser.CallSite {
	for i := range sites {
		s := &sites[i]
		if s.Line == c.Line && s.FuncName == c.Function && s.Variable == c.Variable && s.Caller == c.Caller {
			return s
		}
	}

	return nil
}

func parallel(items []string, fn func(string)) {
	work := make(chan string)

	var wg sync.WaitGroup

	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for it := range work {
				fn(it)
			}
		})
	}

	for _, it := range items {
		work <- it
	}

	close(work)
	wg.Wait()
}

// annotator holds what one file's findings are annotated from.
type annotator struct {
	resolver *resolve.Resolver
	pr       *parser.ParseResult
	calls    []parser.CallSite
	file     string
}

// annotate sets c's category and candidates.
func (a *annotator) annotate(c *Call, call *parser.CallSite) {
	c.Category = Category(c.Reason)

	defs := a.resolver.Index.CountFunctions(call.FuncName)
	c.Definitions = &defs

	var cands []Candidate

	switch c.Category {
	case CategoryVariable, CategoryReturnType:
		cands = a.receiverCandidates(call)
	case CategoryMethod:
		cands = a.methodCandidates(call, c.Reason)
	case CategoryObject:
		cands = a.objectCandidates(c.Reason)
	}

	c.CandidateCount = len(cands)
	if len(cands) > maxCandidates {
		cands = cands[:maxCandidates]
	}

	c.Candidates = cands
}

// receiverMethods are the methods the function calls on the receiver of
// call, the evidence a candidate component must answer: every one of them.
// A member function of a built-in type says nothing about a component.
func (a *annotator) receiverMethods(call *parser.CallSite) []string {
	var out []string

	for i := range a.calls {
		c := &a.calls[i]
		if !strings.EqualFold(c.Variable, call.Variable) || !strings.EqualFold(c.Caller, call.Caller) || len(c.Chain) != len(call.Chain) {
			continue
		}

		if len(c.Chain) > 0 && !strings.EqualFold(c.Chain[len(c.Chain)-1], call.Chain[len(call.Chain)-1]) {
			continue
		}

		if IsMemberFunction(c.FuncName) && !strings.EqualFold(c.FuncName, call.FuncName) {
			continue
		}

		if !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, c.FuncName) }) {
			out = append(out, c.FuncName)
		}
	}

	if len(out) == 0 {
		out = []string{call.FuncName}
	}

	return out
}

// receiverCandidates are the components that declare every method the
// function calls on the receiver, ranked by how well the receiver's name
// matches the component's and by nearness.
func (a *annotator) receiverCandidates(call *parser.CallSite) []Candidate {
	methods := a.receiverMethods(call)

	// The pool is the files declaring the rarest of the methods.
	rarest, fewest := "", -1

	for _, m := range methods {
		if n := a.resolver.Index.CountFunctions(m); n > 0 && (fewest < 0 || n < fewest) {
			rarest, fewest = m, n
		}
	}

	if rarest == "" {
		return nil
	}

	seen := map[string]bool{}

	var out []Candidate

	name := receiverName(call)

	for _, def := range a.resolver.Index.Lookup(rarest) {
		if !def.URI.IsFile() {
			continue
		}

		path := def.URI.Path()
		if seen[pathKey(path)] || !strings.EqualFold(filepath.Ext(path), ".cfc") || frameworkapi.IsStub(path) {
			continue
		}

		seen[pathKey(path)] = true

		if !a.answersAll(path, methods) {
			continue
		}

		basis := []string{"declares " + describeMethods(methods)}

		affinity := nameAffinity(name, path)
		if affinity {
			basis = append(basis, "named like the receiver '"+name+"'")
		}

		out = append(out, Candidate{File: path, Basis: basis})
	}

	return rank(out, a.file, len(methods), func(c *Candidate) bool { return nameAffinity(name, c.File) })
}

// answersAll reports whether the component at path, with its extends chain,
// declares every method.
func (a *annotator) answersAll(path string, methods []string) bool {
	for _, m := range methods {
		if a.resolver.LookupFuncWithExtends(path, m) == nil {
			return false
		}
	}

	return true
}

// methodCandidates are the definitions of the method anywhere the index
// knows: for a method missing from a known component, the ones in its
// subclasses rank higher; for a bare call, the ones nearest the caller.
func (a *annotator) methodCandidates(call *parser.CallSite, reason string) []Candidate {
	known := ""
	if m := methodNotFoundRe.FindStringSubmatch(reason); m != nil {
		known = a.resolver.ComponentPath(m[1], filepath.Dir(a.file))
	}

	var out []Candidate

	for _, def := range a.resolver.Index.Lookup(call.FuncName) {
		if !def.URI.IsFile() || frameworkapi.IsStub(def.URI.Path()) {
			continue
		}

		path := def.URI.Path()
		basis := []string{"declares " + def.Name}

		if known != "" && descendsFrom(a.resolver, path, known) {
			basis = append(basis, "extends "+filepath.Base(known))
		}

		if cfpath.SamePath(filepath.Dir(path), filepath.Dir(a.file)) {
			basis = append(basis, "beside the calling file")
		}

		out = append(out, Candidate{File: path, Line: def.Line, Basis: basis})
	}

	return rank(out, a.file, 1, func(c *Candidate) bool {
		return slices.ContainsFunc(c.Basis, func(b string) bool { return strings.HasPrefix(b, "extends ") })
	})
}

// objectCandidates are the files named like the last segment of a component
// path that names no file.
func (a *annotator) objectCandidates(reason string) []Candidate {
	m := componentRe.FindStringSubmatch(reason)
	if m == nil {
		return nil
	}

	last := m[1]
	if i := strings.LastIndexByte(last, '.'); i >= 0 {
		last = last[i+1:]
	}

	if last == "" || strings.ContainsAny(last, "#$") {
		return nil
	}

	var out []Candidate

	for _, path := range a.resolver.Index.FindFilesByBasename(last) {
		if frameworkapi.IsStub(path) {
			continue
		}

		out = append(out, Candidate{File: path, Basis: []string{"file named " + last + ".cfc"}})
	}

	return rank(out, a.file, 1, func(*Candidate) bool { return false })
}

// rank orders candidates — preferred ones, then the nearest — and sets each
// one's confidence: high for a sole candidate that answers several methods or
// is preferred, medium for a sole candidate otherwise or the one preferred
// among several, low for the rest.
func rank(out []Candidate, from string, evidence int, preferred func(*Candidate) bool) []Candidate {
	slices.SortStableFunc(out, func(x, y Candidate) int {
		px, py := preferred(&x), preferred(&y)
		if px != py {
			if px {
				return -1
			}

			return 1
		}

		dx, dy := distance(from, x.File), distance(from, y.File)
		if dx != dy {
			return dx - dy
		}

		return strings.Compare(x.File, y.File)
	})

	preferredCount := 0

	for i := range out {
		if preferred(&out[i]) {
			preferredCount++
		}
	}

	for i := range out {
		c := &out[i]

		switch {
		case len(out) == 1 && (evidence > 1 || preferred(c)):
			c.Confidence = ConfidenceHigh
		case len(out) == 1, preferredCount == 1 && preferred(c):
			c.Confidence = ConfidenceMedium
		default:
			c.Confidence = ConfidenceLow
		}
	}

	return out
}

// distance is how many directories separate two files.
func distance(a, b string) int {
	x := strings.Split(filepath.ToSlash(filepath.Dir(a)), "/")
	y := strings.Split(filepath.ToSlash(filepath.Dir(b)), "/")

	common := 0
	for common < len(x) && common < len(y) && strings.EqualFold(x[common], y[common]) {
		common++
	}

	return len(x) + len(y) - 2*common
}

// receiverName is the receiver's last name: rc.contentBean is contentBean.
func receiverName(call *parser.CallSite) string {
	name := call.Variable
	if len(call.Chain) > 0 {
		name = parser.CallHopName(call.Chain[len(call.Chain)-1])
		name = strings.TrimPrefix(strings.TrimPrefix(name, "get"), "Get")
	}

	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}

	return strings.TrimSuffix(strings.TrimLeft(name, "_"), "[]")
}

// nameAffinity reports whether a receiver called name is named like the
// component at path: contentBean and contentBean.cfc, oUser and User.cfc,
// userService and UserService.cfc.
func nameAffinity(name, path string) bool {
	if name == "" {
		return false
	}

	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	n := strings.ToLower(name)

	if n == stem || n == stem+"bean" || stem == n+"bean" {
		return true
	}

	// Hungarian o: oUser.
	return len(name) > 1 && name[0] == 'o' && name[1] >= 'A' && name[1] <= 'Z' && n[1:] == stem
}

func describeMethods(methods []string) string {
	if len(methods) == 1 {
		return methods[0] + "()"
	}

	return strings.Join(methods, "(), ") + "()"
}

func descendsFrom(r *resolve.Resolver, path, ancestor string) bool {
	seen := map[string]bool{}

	for p := path; p != "" && !seen[p] && len(seen) < 16; {
		seen[p] = true

		if cfpath.SamePath(p, ancestor) {
			return p != path
		}

		ext, ok := r.Index.ExtendsForFile(uri.File(p))
		if !ok || ext == "" {
			return false
		}

		p = r.ComponentPath(ext, filepath.Dir(p))
	}

	return false
}

func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

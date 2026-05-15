package cfparser

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.lsp.dev/uri"
)

// Logger is an optional interface for parse diagnostics.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

// ParseResult caches a single parse of a file. It extracts function signatures
// and component refs eagerly, but defers function body parsing until requested.
type ParseResult struct {
	URI     uri.URI
	Content string
	Regions []Region
	Funcs   []FunctionDef
	Refs    []ComponentRef
	Scopes  []FuncScope
	Extends string // dot-path of parent component (from extends attribute)
	Log     Logger // optional logger for timing and errors

	// Lazy global var caches (protected by mu).
	mu            sync.Mutex
	globalVars    []string
	globalDone    bool
	variablesVars []string
	varsDone      bool
	thisVars      []string
	thisDone      bool

	// funcVars caches per-function variable lists keyed by "start:end".
	funcVarsMu sync.Mutex
	funcVars   map[string][]string
}

// Parse performs a full file parse: extracts function signatures, component refs,
// and function scopes. Function bodies are NOT parsed for variables until requested.
func Parse(fileURI uri.URI, content string) *ParseResult {
	pr := &ParseResult{
		URI:      fileURI,
		Content:  content,
		funcVars: make(map[string][]string),
	}
	start := time.Now()
	pr.Regions = ClassifyRegions(content)
	pr.extractSignatures()
	pr.logInfo("parse complete", "uri", string(fileURI), "funcs", len(pr.Funcs), "refs", len(pr.Refs), "dur", time.Since(start))
	return pr
}

// extractSignatures does a shallow parse: function names/args, component refs, scopes.
func (pr *ParseResult) extractSignatures() {
	defer func() {
		if r := recover(); r != nil {
			pr.logWarn("parse panic in extractSignatures", "uri", string(pr.URI), "error", fmt.Sprint(r))
		}
	}()
	for _, r := range pr.Regions {
		if r.Kind == RegionScript {
			sp := newShallowScriptParser(r.Text, string(pr.URI), r.StartLine)
			sp.parse()
			pr.Funcs = append(pr.Funcs, sp.funcs...)
			pr.Refs = append(pr.Refs, sp.refs...)
			pr.Scopes = append(pr.Scopes, sp.scopes...)
			if sp.extends != "" {
				pr.Extends = sp.extends
			}
		} else {
			tp := newTagParser(r.Text, string(pr.URI))
			tp.parse()
			for i := range tp.funcs {
				tp.funcs[i].Line += uint32(r.StartLine)
			}
			for i := range tp.refs {
				tp.refs[i].Line += uint32(r.StartLine)
			}
			pr.Funcs = append(pr.Funcs, tp.funcs...)
			pr.Refs = append(pr.Refs, tp.refs...)
			scopes := findTagFuncScopes(r.Text, r.StartLine)
			pr.Scopes = append(pr.Scopes, scopes...)
			if tp.extends != "" {
				pr.Extends = tp.extends
			}
		}
	}
}

// GlobalVars returns this.x and variables.x names declared outside any function.
func (pr *ParseResult) GlobalVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if !pr.globalDone {
		pr.globalVars = pr.computeGlobalVars()
		pr.globalDone = true
	}
	return pr.globalVars
}

// VariablesVars returns variables-scoped names from outside functions.
func (pr *ParseResult) VariablesVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if !pr.varsDone {
		pr.variablesVars = pr.computeScopedVars(ScopeVariables)
		pr.varsDone = true
	}
	return pr.variablesVars
}

// ThisVars returns this-scoped property names from outside functions.
func (pr *ParseResult) ThisVars() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if !pr.thisDone {
		pr.thisVars = pr.computeScopedVars(ScopeThis)
		pr.thisDone = true
	}
	return pr.thisVars
}

// FuncVars returns local/arguments variable names within the function at [start, end].
// Results are cached; call InvalidateFunc to force re-parse.
func (pr *ParseResult) FuncVars(funcStart, funcEnd int) []string {
	key := funcKey(funcStart, funcEnd)
	pr.funcVarsMu.Lock()
	if cached, ok := pr.funcVars[key]; ok {
		pr.funcVarsMu.Unlock()
		return cached
	}
	pr.funcVarsMu.Unlock()

	vars := pr.parseFuncBody(funcStart, funcEnd)

	pr.funcVarsMu.Lock()
	pr.funcVars[key] = vars
	pr.funcVarsMu.Unlock()
	return vars
}

// InvalidateFunc clears the cached variables for a specific function,
// forcing re-parse on next FuncVars call.
func (pr *ParseResult) InvalidateFunc(funcStart, funcEnd int) {
	key := funcKey(funcStart, funcEnd)
	pr.funcVarsMu.Lock()
	delete(pr.funcVars, key)
	pr.funcVarsMu.Unlock()
}

// parseFuncBody parses a single function body for variable declarations.
func (pr *ParseResult) parseFuncBody(funcStart, funcEnd int) (names []string) {
	defer func() {
		if r := recover(); r != nil {
			pr.logWarn("parse panic in parseFuncBody", "uri", string(pr.URI), "funcStart", funcStart, "error", fmt.Sprint(r))
		}
	}()
	start, end := lineOffsets(pr.Content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}
	body := pr.Content[start:end]

	t := time.Now()
	sp := newScriptParser(body, "", 0)
	sp.parse()

	seen := make(map[string]bool)
	for _, v := range sp.vars {
		if v.Scope == ScopeLocal || v.Scope == ScopeArguments {
			if !seen[v.Name] {
				seen[v.Name] = true
				names = append(names, v.Name)
			}
		}
	}
	pr.logInfo("parseFuncBody", "uri", string(pr.URI), "funcStart", funcStart, "vars", len(names), "dur", time.Since(t))
	return names
}

// computeGlobalVars extracts global-scope variables (variables.x, this.x, plain assigns).
func (pr *ParseResult) computeGlobalVars() []string {
	vars := pr.computeScopedVars(ScopeVariables)
	vars = append(vars, pr.computeScopedVars(ScopeThis)...)
	return vars
}

// computeScopedVars extracts variables of a specific scope from outside functions.
func (pr *ParseResult) computeScopedVars(scope Scope) []string {
	seen := make(map[string]bool)
	var names []string

	for _, r := range pr.Regions {
		var regionVars []VarDef
		if r.Kind == RegionScript {
			sp := newGlobalScriptParser(r.Text, r.StartLine, pr.Scopes)
			sp.parse()
			regionVars = sp.vars
		} else {
			tp := newTagParser(r.Text, "")
			tp.parse()
			for i := range tp.vars {
				tp.vars[i].Line += uint32(r.StartLine)
			}
			regionVars = tp.vars
		}

		for _, v := range regionVars {
			if v.Scope != scope {
				continue
			}
			fs := findFuncScope(int(v.Line), pr.Scopes)
			if fs.Start != -1 {
				continue // inside a function
			}
			if !seen[v.Name] {
				seen[v.Name] = true
				names = append(names, v.Name)
			}
		}
	}
	return names
}

func funcKey(start, end int) string {
	return strings.Join([]string{itoa(start), itoa(end)}, ":")
}

func itoa(n int) string {
	if n < 0 {
		return "-" + uitoa(uint(-n))
	}
	return uitoa(uint(n))
}

func uitoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// lineOffsets converts line numbers to byte offsets.
func lineOffsets(content string, startLine, endLine int) (int, int) {
	start := 0
	line := 0
	for line < startLine {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return -1, -1
		}
		start += idx + 1
		line++
	}
	end := start
	for line <= endLine {
		idx := strings.IndexByte(content[end:], '\n')
		if idx < 0 {
			end = len(content)
			break
		}
		end += idx + 1
		line++
	}
	return start, end
}

func (pr *ParseResult) logInfo(msg string, keysAndValues ...interface{}) {
	if pr.Log != nil {
		pr.Log.Info(msg, keysAndValues...)
	}
}

func (pr *ParseResult) logWarn(msg string, keysAndValues ...interface{}) {
	if pr.Log != nil {
		pr.Log.Warn(msg, keysAndValues...)
	}
}

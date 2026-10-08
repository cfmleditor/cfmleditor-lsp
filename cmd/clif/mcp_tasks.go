package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/clif/internal/cflint"
	"github.com/cfmleditor/clif/internal/codemap/mcp"
	"github.com/cfmleditor/clif/internal/daemon"
	"github.com/cfmleditor/clif/internal/refs"
	"github.com/cfmleditor/clif/internal/unresolved"
	"github.com/cfmleditor/clif/internal/vfs"
)

// The MCP server's task tools run what the CLI subcommands run, through the
// same functions, so a tool and its command cannot disagree. Each checks its
// paths first: a path that does not exist would otherwise scan nothing and
// answer "no findings", which reads as a clean bill of health.

// absPaths makes every path absolute and fails on one that does not exist.
func absPaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("%s does not exist (paths are relative to %s)", p, mustGetwd())
		}

		out = append(out, abs)
	}

	return out, nil
}

func mustGetwd() string {
	wd, _ := os.Getwd()

	return wd
}

// unresolvedCall is unresolved.Call with a 1-based line, as editors and the
// CLI's text output number them, and as explain_call expects one.
type unresolvedCall struct {
	File      string `json:"file"`
	Line      uint32 `json:"line"`
	Caller    string `json:"caller,omitempty"`
	Call      string `json:"call"`
	Reason    string `json:"reason"`
	Unchecked int    `json:"unchecked,omitempty"`
}

type missingBase struct {
	Component string `json:"component"`
	Files     int    `json:"files"`
	Calls     int    `json:"calls"`
}

func mcpUnresolved(paths []string, globalDefs bool, limit int) (any, error) {
	abs, err := absPaths(paths)
	if err != nil {
		return nil, err
	}

	rep, cfg, searchDir := scanUnresolved(abs, &unresolvedFlags{globalDefs: globalDefs})

	calls := make([]unresolvedCall, 0, len(rep.Calls))

	for i := range rep.Calls {
		c := &rep.Calls[i]

		calls = append(calls, unresolvedCall{
			File: c.File, Line: c.Line + 1, Caller: c.Caller,
			Call: c.CallText(), Reason: c.Reason, Unchecked: c.Unchecked,
		})
	}

	out := mcp.Limited("calls", calls, limit)
	out["resolved"] = rep.Resolved
	out["filesScanned"] = rep.Scanned

	if cfg != nil {
		out["config"] = cfg.Path
	}

	if bases := unresolved.MissingBases(rep.Calls); len(bases) > 0 {
		list := make([]missingBase, 0, len(bases))
		for _, b := range bases {
			list = append(list, missingBase{Component: b.Component, Files: b.Files, Calls: b.Calls})
		}

		out["missingBases"] = list
		out["missingBasesNote"] = "Base components that do not resolve; a mapping or workspace path for each checks the calls it leaves unchecked."
	}

	if hint := presetHint(cfg, searchDir); hint != "" {
		out["hint"] = hint
	}

	return out, nil
}

// reference is refs.Entry with a 1-based line.
type reference struct {
	File      string `json:"file"`
	Line      uint32 `json:"line"`
	Function  string `json:"function,omitempty"`
	Variable  string `json:"variable,omitempty"`
	Call      string `json:"call,omitempty"`
	Component string `json:"component,omitempty"`
	Resolved  bool   `json:"resolved"`
	Reason    string `json:"reason,omitempty"`
}

func mcpFindRefs(target string, paths []string, limit int) (any, error) {
	abs, err := absPaths(paths)
	if err != nil {
		return nil, err
	}

	target = strings.TrimSpace(target)
	resolvers := loadResolversFromConfig(abs)

	var (
		entries []refs.Entry
		scanned int
	)

	if strings.Contains(target, ".") {
		entries, scanned = refs.FindComponentRefs(vfs.OS{}, abs, target, resolvers)
	} else {
		entries, scanned = refs.FindCalls(vfs.OS{}, abs, target, resolvers)
	}

	list := make([]reference, 0, len(entries))

	for i := range entries {
		e := &entries[i]

		list = append(list, reference{
			File: e.File, Line: e.Line + 1, Function: e.Function, Variable: e.Variable,
			Call: e.Call, Component: e.Component, Resolved: e.Resolved, Reason: e.Reason,
		})
	}

	out := mcp.Limited("references", list, limit)
	out["target"] = target
	out["filesScanned"] = scanned

	return out, nil
}

// newMCPLinter builds the lint tool. The runner is made on the first call,
// since making it downloads CFLint when it is not cached, and a server most
// sessions never ask to lint should not pay that at startup. Its minimum
// severity is the config's above the first path of that call.
func newMCPLinter() mcp.Linter {
	var (
		mu     sync.Mutex
		runner *cflint.Runner
	)

	return func(paths []string, limit int) (any, error) {
		abs, err := absPaths(paths)
		if err != nil {
			return nil, err
		}

		mu.Lock()
		defer mu.Unlock()

		if runner == nil {
			minSeverity := ""

			dir := abs[0]
			if info, err := os.Stat(dir); err == nil && !info.IsDir() {
				dir = filepath.Dir(dir)
			}

			if cfg, _ := daemon.FindConfig(dir); cfg != nil {
				minSeverity = cfg.LintMinSeverity()
			}

			r, err := cflint.NewRunner(context.Background(), minSeverity)
			if err != nil {
				return nil, fmt.Errorf("cflint unavailable: %w", err)
			}

			runner = r
		}

		files := collectCFMLFiles(vfs.OS{}, abs)
		if len(files) == 0 {
			return nil, errors.New("no CFML files under the paths given")
		}

		found, err := runner.ScanFiles(context.Background(), files)
		if err != nil {
			return nil, fmt.Errorf("cflint failed: %w", err)
		}

		out := mcp.Limited("findings", cflint.Findings(found), limit)
		out["filesLinted"] = len(files)

		return out, nil
	}
}

package main

import (
	"slices"
	"strings"
)

// subcommand is one entry in the CLI: what runs it and what its --help says.
// Holding both in one table is what keeps every subcommand answering --help:
// each used to parse its own arguments, and all but graph, mcp and routes took
// "--help" for a path, so `unresolved --help` scanned the current directory.
type subcommand struct {
	run   func(args []string)
	usage string
}

// subcommands returns the table main dispatches through. A function rather
// than a package variable, since a variable naming the cmd functions would be
// an initialisation cycle for any of them that reads it.
func subcommands() map[string]subcommand {
	return map[string]subcommand{
		"parse":      {cmdParse, parseUsage},
		"scan":       {cmdScan, scanUsage},
		"format":     {cmdFormat, formatUsage},
		"deps":       {cmdDeps, depsUsage},
		"graph":      {cmdGraph, graphUsage},
		"mcp":        {cmdMCP, mcpUsage},
		"routes":     {cmdRoutes, routesUsage},
		"refs":       {cmdRefs, refsUsage},
		"unresolved": {cmdUnresolved, unresolvedUsage},
		"cflint":     {cmdCFLint, cflintUsage},
		"explain":    {cmdExplain, explainUsage},
	}
}

// wantsHelp reports whether args ask for help anywhere in them, so
// `unresolved . --help` answers as `unresolved --help` does.
func wantsHelp(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return a == "-h" || a == "--help"
	})
}

const parseUsage = `usage: cfmleditor-lsp parse <file-or-dir> [...]

Parse CFML files and report, per file, how long the parse took and how many
functions and component references it found, then the totals.
`

const scanUsage = `usage: cfmleditor-lsp scan <file-or-dir> [...]

Parse CFML files with the tree-sitter grammar and report every file it could
not parse, with the position of each error.
`

const formatUsage = `usage: cfmleditor-lsp format [-w] [--allow-non-whitespace] [--root <dir>] <file> [...]

Format CFML files to stdout, or in place with -w. The formatting settings are
read from each file's governing .cfmleditor.json.

  -w                      rewrite the file in place
  --allow-non-whitespace  permit changes beyond whitespace (off by default)
  --root <dir>            read formatting config from this directory's
                          .cfmleditor.json instead of each file's own
`

const depsUsage = `usage: cfmleditor-lsp deps [--mermaid] <dir-or-file> [...]

Print the transitive dependency graph of the given files, as JSON.

  --mermaid   print a Mermaid diagram instead
`

const refsUsage = `usage: cfmleditor-lsp refs [--mermaid] <component-or-function> <dir> [...]

Find references to a component (a dot-path) or a function (a bare name) in
the given directories, as JSON.

  --mermaid   print a Mermaid diagram instead

  e.g. cfmleditor-lsp refs packages.finance.service ./src
       cfmleditor-lsp refs getReport ./src
`

const explainUsage = `usage: cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]

Trace how each call site on a line resolved, or why it did not: which rule
typed the receiver, which resolver fired for each hop of a chain, and whether
the method was found. The line is 1-based. A call-substring keeps only the
calls whose text contains it.

  --root <dir>   read .cfmleditor.json and index files from this directory,
                 as unresolved's directory argument does (default: the
                 file's own directory, whose nearest config can differ from
                 the one a project-wide scan used)

  e.g. cfmleditor-lsp explain directcontent.cfc 104
       cfmleditor-lsp explain directcontent.cfc 104 createTemplate
       cfmleditor-lsp explain --root ../tassweb/webroot directcontent.cfc 104
`

// positional returns a as a path or other plain argument, or exits naming it
// when it looks like an option the command did not recognise. Without it a
// mistyped flag was taken for a path: `cflint --wirte .` linted a directory
// called --wirte, and `parse -x` reported that no such file exists.
func positional(a, usage string) string {
	if strings.HasPrefix(a, "-") && a != "-" {
		fatalf("unknown option %q\n\n%s", a, usage)
	}

	return a
}

// Package main is the entry point for the cfmleditor-lsp server.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/server"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/uri"
)

var version = "dev"

func main() {
	// Subcommand routing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "parse":
			cmdParse(os.Args[2:])

			return
		case "scan":
			cmdScan(os.Args[2:])

			return
		case "format":
			cmdFormat(os.Args[2:])

			return
		case "deps":
			cmdDeps(os.Args[2:])

			return
		case "refs":
			cmdRefs(os.Args[2:])

			return
		case "version":
			fmt.Printf("cfmleditor-lsp %s\n", version)

			return
		case "help", "--help", "-h":
			printHelp()

			return
		}
	}

	// Default: run LSP server
	runServer()
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `cfmleditor-lsp %s

Commands:
  (default)    Run the LSP server over stdio
  parse        Parse CFML files and report timing
  scan         Scan CFML files and report parse errors
  format       Format CFML files (stdout or in-place with -w)
  version      Print version
  help         Show this help

Parse usage:
  cfmleditor-lsp parse <file-or-dir> [...]

Scan usage:
  cfmleditor-lsp scan <file-or-dir> [...]

Format usage:
  cfmleditor-lsp format [-w] <file> [...]
`, version)
}

func runServer() {
	fmt.Fprintf(os.Stderr, "cfmleditor-lsp %s\n", version)

	cwd, _ := os.Getwd()
	cfg, _ := daemon.FindConfig(cwd)

	debug := cfg != nil && cfg.Debug()
	log := cflog.NewLogger(debug)

	if debug {
		log.Info("debug mode enabled")
	}

	if cfg != nil {
		sock := cfg.SocketPath()

		// Try to connect to an existing daemon
		if err := daemon.Proxy(sock); err == nil {
			return
		}

		// No daemon running — become the daemon and serve this client over stdio
		log.Info("starting daemon mode", cflog.String("socket", sock))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sharedIndex := index.New()
		ct := daemon.NewConnTracker()
		folders := cfg.WorkspaceFolders()
		globs := cfg.IndexGlobs()
		mappings := cfg.Mappings()
		resolverPairs := cfg.ComponentResolvers()
		propResolverPairs := cfg.PropertyResolvers()
		beanPaths := cfg.BeanPaths()

		// Serve the socket listener in the background
		fmtCfg := config.ResolvedFormatting{
			Enabled:                cfg.FormattingEnabled(),
			Debug:                  cfg.FormattingDebug(),
			SelfCloseTags:          cfg.FormattingSelfCloseTags(),
			WhitespaceOnly:         cfg.FormattingWhitespaceOnly(),
			QueryFormat:            cfg.FormattingQueryFormat(),
			LowercaseTags:          cfg.FormattingLowercaseTags(),
			LowercaseAttributes:    cfg.FormattingLowercaseAttributes(),
			DoubleQuoteAttributes:  cfg.FormattingDoubleQuoteAttributes(),
			QueryUppercaseKeywords: cfg.FormattingQueryUppercaseKeywords(),
			ScopeCase:              cfg.FormattingScopeCase(),
			CommaPosition:          cfg.FormattingCommaPosition(),
			QueryCommaPosition:     cfg.FormattingQueryCommaPosition(),
			LineWidth:              cfg.FormattingLineWidth(),
			AttrBreakThreshold:     cfg.FormattingAttrBreakThreshold(),
			IndentWidth:            cfg.FormattingIndentWidth(),
		}
		go func() {
			_ = daemon.Serve(ctx, sock, log, sharedIndex, ct, folders, globs, mappings, resolverPairs, propResolverPairs, beanPaths, fmtCfg)
		}()

		// Serve this editor session over stdio with the shared index
		ct.Add()

		stream := jsonrpc2.NewStream(vfs.Stdio())
		conn := jsonrpc2.NewConn(stream)
		srv := server.NewServer(conn, log, sharedIndex)
		srv.Version = version
		srv.WorkspaceFolders = folders
		srv.IndexGlobs = globs
		srv.Mappings = mappings

		for _, p := range resolverPairs {
			srv.ComponentResolvers = append(srv.ComponentResolvers, config.Resolver{Match: p[0], Resolve: p[1], Prefix: p[2]})
		}

		for _, p := range propResolverPairs {
			srv.PropertyResolvers = append(srv.PropertyResolvers, config.PropResolver{Match: p[0], Resolve: p[1], Attribute: p[2]})
		}

		srv.BeanPaths = beanPaths
		srv.Formatting = fmtCfg
		srv.Linting = cfg.Linting()
		conn.Go(ctx, srv.Handler())

		go func() {
			select {
			case <-conn.Done():
			case <-ctx.Done():
				_ = os.Stdin.Close()

				<-conn.Done()
			}

			ct.Remove()
		}()

		// Shut down when all clients have disconnected
		<-ct.Done()
		cancel()

		return
	}

	// No config found — standalone mode
	stream := jsonrpc2.NewStream(vfs.Stdio())
	conn := jsonrpc2.NewConn(stream)
	srv := server.NewServer(conn, log)
	srv.Version = version
	conn.Go(context.Background(), srv.Handler())
	<-conn.Done()
}

func cmdParse(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp parse <file-or-dir> [...]\n")
		os.Exit(1)
	}

	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, err)
			os.Exit(1)
		}

		if info.IsDir() {
			filepath.Walk(arg, func(path string, _ os.FileInfo, err error) error { //nolint:errcheck
				if err != nil {
					return nil //nolint:nilerr
				}

				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".cfc" || ext == ".cfm" || ext == ".cfml" || ext == ".cfs" {
					files = append(files, path)
				}

				return nil
			})
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no CFML files found\n")
		os.Exit(1)
	}

	var totalDur time.Duration

	var totalFuncs, totalRefs, totalFiles int

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", f, err)

			continue
		}

		if cfpath.IsBinary(content) {
			continue
		}

		absPath, _ := filepath.Abs(f)
		fileURI := uri.URI("file://" + absPath)
		start := time.Now()
		pr := parser.Parse(fileURI, string(content))
		dur := time.Since(start)

		totalDur += dur
		totalFuncs += len(pr.Funcs)
		totalRefs += len(pr.Refs)
		totalFiles++

		fmt.Printf("  %s  funcs=%d refs=%d scopes=%d  %v\n", f, len(pr.Funcs), len(pr.Refs), len(pr.Scopes), dur)
	}

	avg := time.Duration(0)
	if totalFiles > 0 {
		avg = totalDur / time.Duration(totalFiles)
	}

	fmt.Printf("\n  total: %d files, %d funcs, %d refs in %v (avg %v/file)\n",
		totalFiles, totalFuncs, totalRefs, totalDur, avg)
}

func cmdFormat(args []string) {
	write := false

	var files []string

	for _, arg := range args {
		if arg == "-w" {
			write = true
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp format [-w] <file> [...]\n")
		os.Exit(1)
	}

	opts := formatter.DefaultOptions()
	opts.ParseScript = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFScript, src, nil)
	}
	opts.ParseQuery = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFQuery, src, nil)
	}
	opts.ParseCFML = func(src []byte) *sitter.Tree {
		return language.Parse(language.CFML, src, nil)
	}

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", f, err)
			os.Exit(1)
		}

		tree := language.Parse(language.CFML, content, nil)
		out, err := formatter.Format(content, tree, opts)
		tree.Close()

		if err != nil {
			fmt.Fprintf(os.Stderr, "error formatting %s: %v\n", f, err)
			os.Exit(1)
		}

		if write {
			if err := os.WriteFile(f, out, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", f, err)
				os.Exit(1)
			}

			fmt.Fprintf(os.Stderr, "formatted %s\n", f)
		} else {
			_, _ = os.Stdout.Write(out)
		}
	}
}

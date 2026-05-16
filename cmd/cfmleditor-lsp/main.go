// Package main is the entry point for the cfmleditor-lsp server.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/server"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	// Subcommand routing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "parse":
			cmdParse(os.Args[2:])
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
  version      Print version
  help         Show this help

Parse usage:
  cfmleditor-lsp parse <file-or-dir> [...]
`, version)
}

func runServer() {
	fmt.Fprintf(os.Stderr, "cfmleditor-lsp %s\n", version)
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()
	cwd, _ := os.Getwd()
	cfg, _ := daemon.FindConfig(cwd)

	if cfg != nil {
		sock := cfg.SocketPath()

		// Try to connect to an existing daemon
		if err := daemon.Proxy(sock); err == nil {
			return
		}

		// No daemon running — become the daemon and serve this client over stdio
		logger.Info("starting daemon mode", zap.String("socket", sock))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sharedIndex := index.New()
		ct := daemon.NewConnTracker()
		folders := cfg.WorkspaceFolders()
		globs := cfg.IndexGlobs()
		mappings := cfg.Mappings()
		resolverPairs := cfg.ComponentResolvers()

		// Serve the socket listener in the background
		go func() { _ = daemon.Serve(ctx, sock, logger, sharedIndex, ct, folders, globs, mappings, resolverPairs) }()

		// Serve this editor session over stdio with the shared index
		ct.Add()
		stream := jsonrpc2.NewStream(newStdio())
		conn := jsonrpc2.NewConn(stream)
		srv := server.NewServer(conn, logger, sharedIndex)
		srv.Version = version
		srv.WorkspaceFolders = folders
		srv.IndexGlobs = globs
		srv.Mappings = mappings
		for _, p := range resolverPairs {
			srv.ComponentResolvers = append(srv.ComponentResolvers, server.ComponentResolver{Match: p[0], Resolve: p[1], Prefix: p[2]})
		}
		conn.Go(ctx, srv.Handler())
		go func() {
			<-conn.Done()
			ct.Remove()
		}()

		// Shut down when all clients have disconnected
		<-ct.Done()
		cancel()
		return
	}

	// No config found — standalone mode
	stream := jsonrpc2.NewStream(newStdio())
	conn := jsonrpc2.NewConn(stream)
	srv := server.NewServer(conn, logger)
	srv.Version = version
	conn.Go(context.Background(), srv.Handler())
	<-conn.Done()
}

type stdio struct{}

func newStdio() stdio { return stdio{} }

// Read reads from stdin.
func (s stdio) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

// Write writes to stdout.
func (s stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

// Close is a no-op.
func (s stdio) Close() error { return nil }

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

		absPath, _ := filepath.Abs(f)
		fileURI := uri.URI("file://" + absPath)
		start := time.Now()
		pr := cfparser.Parse(fileURI, string(content))
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

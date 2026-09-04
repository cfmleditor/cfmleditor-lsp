// Package main is the entry point for the cfmleditor-lsp server.
package main

import (
	"bytes"
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
		case "unresolved":
			cmdUnresolved(os.Args[2:])

			return
		case "explain":
			cmdExplain(os.Args[2:])

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
  unresolved   Scan for unresolved component/method calls
  refs         Find references to a component or function
  deps         Print component dependency info
  explain      Explain how a call site's component was resolved
  version      Print version
  help         Show this help

Parse usage:
  cfmleditor-lsp parse <file-or-dir> [...]

Scan usage:
  cfmleditor-lsp scan <file-or-dir> [...]

Format usage:
  cfmleditor-lsp format [-w] [--allow-non-whitespace] [--root <dir>] <file> [...]
    -w                      rewrite the file in place
    --allow-non-whitespace  permit changes beyond whitespace (off by default)
    --root <dir>            read formatting config from this directory's
                            .cfmleditor.json instead of each file's own

Explain usage:
  cfmleditor-lsp explain <file> <line> [call-substring]
  e.g. cfmleditor-lsp explain directcontent.cfc 104
       cfmleditor-lsp explain directcontent.cfc 104 createTemplate
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
		fmtCfg := cfg.ResolvedFormatting()

		propResolvers := make([]config.PropResolver, 0, len(cfg.PropertyResolvers()))
		for _, p := range cfg.PropertyResolvers() {
			propResolvers = append(propResolvers, config.PropResolver{Match: p[0], Resolve: p[1], Attribute: p[2]})
		}

		// One settings value configures every session, whether it arrives over
		// stdio here or over the socket later. Keeping the two in step by hand
		// is what dropped expressionMappings and servicePropertyResolvers from
		// every editor after the first.
		settings := server.Settings{
			WorkspaceFolders:         cfg.WorkspaceFolders(),
			IndexGlobs:               cfg.IndexGlobs(),
			Mappings:                 cfg.Mappings(),
			ExpressionMappings:       cfg.ExpressionMappings(),
			ServicePropertyResolvers: cfg.ServicePropertyResolvers(),
			ComponentResolvers:       cfg.ComponentResolvers(),
			PropertyResolvers:        propResolvers,
			BeanPaths:                cfg.BeanPaths(),
			Formatting:               fmtCfg,
			Linting:                  cfg.Linting(),
		}

		go func() {
			_ = daemon.Serve(ctx, sock, log, sharedIndex, ct, settings)
		}()

		// Serve this editor session over stdio with the shared index
		ct.Add()

		stream := jsonrpc2.NewStream(vfs.Stdio())
		conn := jsonrpc2.NewConn(stream)
		srv := server.NewServer(conn, log, sharedIndex)
		srv.Version = version
		settings.Apply(srv)
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
		totalRefs += len(pr.ComponentRefs)
		totalFiles++

		fmt.Printf("  %s  funcs=%d refs=%d scopes=%d  %v\n", f, len(pr.Funcs), len(pr.ComponentRefs), len(pr.Scopes), dur)
	}

	avg := time.Duration(0)
	if totalFiles > 0 {
		avg = totalDur / time.Duration(totalFiles)
	}

	fmt.Printf("\n  total: %d files, %d funcs, %d refs in %v (avg %v/file)\n",
		totalFiles, totalFuncs, totalRefs, totalDur, avg)
}

func cmdFormat(args []string) {
	var (
		write              bool
		allowNonWhitespace bool
		configRoot         string
		files              []string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-w":
			write = true
		case "--allow-non-whitespace":
			allowNonWhitespace = true
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --root needs a directory\n")
				os.Exit(1)
			}

			configRoot = args[i+1]
			i++
		default:
			files = append(files, args[i])
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp format [-w] [--allow-non-whitespace] [--root <dir>] <file> [...]\n")
		os.Exit(1)
	}

	optionsFor := formatOptionsFor(configRoot, allowNonWhitespace)

	failed := false

	for _, f := range files {
		if err := formatOneFile(f, optionsFor(f), write); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", f, err)

			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
}

// formatOptionsFor returns a lookup that maps a file to the formatter options
// its governing .cfmleditor.json asks for, memoised per config directory.
//
// The subcommand used to format from formatter.DefaultOptions() alone, so it
// ignored every key under "formatting" and produced different bytes than the
// editor did for the same file. Config is discovered from each file's own
// directory upwards, matching how the LSP picks a config, unless --root names
// one explicitly (same semantics as `explain --root`).
func formatOptionsFor(configRoot string, allowNonWhitespace bool) func(path string) formatter.Options {
	cache := make(map[string]formatter.Options)

	return func(path string) formatter.Options {
		dir := configRoot

		if dir == "" {
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}

			dir = filepath.Dir(abs)
		}

		if opts, ok := cache[dir]; ok {
			return opts
		}

		fmtCfg := config.DefaultResolvedFormatting()
		if cfg, err := daemon.FindConfig(dir); err == nil && cfg != nil {
			fmtCfg = cfg.ResolvedFormatting()
		}

		opts := fmtCfg.FormatterOptions()

		// The flag can only loosen the guard, never tighten it away: a config
		// that turns whitespaceOnly off has already accepted the risk.
		if allowNonWhitespace {
			opts.WhitespaceOnly = false
		}

		opts.ParseScript = func(src []byte) *sitter.Tree {
			return language.Parse(language.CFScript, src, nil)
		}
		opts.ParseQuery = func(src []byte) *sitter.Tree {
			return language.Parse(language.CFQuery, src, nil)
		}
		opts.ParseCFML = func(src []byte) *sitter.Tree {
			return language.Parse(language.CFML, src, nil)
		}

		cache[dir] = opts

		return opts
	}
}

// formatOneFile formats a single file, writing it back in place when write is
// set. A file is only ever rewritten after Format returned successfully, so a
// refused format leaves the original untouched.
func formatOneFile(path string, opts formatter.Options, write bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tree := language.Parse(language.CFML, content, nil)
	defer tree.Close()

	// Refuse a file the grammar could not parse, exactly as the LSP handler
	// does. Formatting an incomplete CST is how a grammar gap turns into
	// deleted source, and the whitespaceOnly guard does not catch every shape
	// of that (see FORMATTER-ISSUES.md).
	if err := formatter.ParseError(tree, content); err != nil {
		return err
	}

	out, err := formatter.Format(content, tree, opts)
	if err != nil {
		return err
	}

	if !write {
		_, _ = os.Stdout.Write(out)

		return nil
	}

	// Rewriting a file the formatter left alone costs nothing but a bumped
	// mtime, which is enough to wake every file watcher and rebuild watching
	// the tree.
	if bytes.Equal(content, out) {
		return nil
	}

	if err := writeFileInPlace(path, content, out); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "formatted %s\n", path)

	return nil
}

// writeFileInPlace replaces path's contents with out, preserving the file's
// permission bits and leaving the original intact if the write fails partway.
//
// os.WriteFile would do neither: it hardcodes the new mode, so a 0600 or 0755
// file silently became 0644, and it truncates in place, so an error partway
// through leaves a half-written source file with no copy of the original
// anywhere. Writing a sibling temp file and renaming it over the target makes
// the replacement atomic for any concurrent reader.
func writeFileInPlace(path string, original, out []byte) error {
	// A symlinked file should have its target rewritten; renaming onto the link
	// itself would replace the link with a regular file.
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}

	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".fmt-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	// Harmless once the rename succeeded, and the only cleanup on every path
	// that does not.
	defer func() { _ = os.Remove(tmpName) }()

	if err := writeAndSync(tmp, out); err != nil {
		return err
	}

	// CreateTemp always makes the file 0600, so the mode has to be restored
	// from the file being replaced rather than inherited.
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return err
	}

	// Re-read rather than trusting the content from before Format ran: a write
	// landing on top of an edit made in the meantime would discard it silently.
	if current, err := os.ReadFile(target); err == nil && !bytes.Equal(current, original) {
		return fmt.Errorf("file changed on disk while formatting, not overwriting")
	}

	return os.Rename(tmpName, target)
}

// writeAndSync writes out to f and flushes it to disk before closing.
//
// Without the fsync, a crash shortly after the rename can leave the target
// visible but empty, since the rename is durable before the data is.
func writeAndSync(f *os.File, out []byte) error {
	if _, err := f.Write(out); err != nil {
		_ = f.Close()

		return err
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()

		return err
	}

	return f.Close()
}

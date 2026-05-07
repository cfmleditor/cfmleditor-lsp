package main

import (
	"context"
	"os"

	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/server"
	"go.lsp.dev/jsonrpc2"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

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

		// Serve the socket listener in the background
		go daemon.Serve(ctx, sock, logger, sharedIndex, ct, folders, globs)

		// Serve this editor session over stdio with the shared index
		ct.Add()
		stream := jsonrpc2.NewStream(newStdio())
		conn := jsonrpc2.NewConn(stream)
		srv := server.NewServer(conn, logger, sharedIndex)
		srv.WorkspaceFolders = folders
		srv.IndexGlobs = globs
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
	conn.Go(context.Background(), srv.Handler())
	<-conn.Done()
}

type stdio struct{}

func newStdio() stdio { return stdio{} }

func (s stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (s stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (s stdio) Close() error                { return nil }

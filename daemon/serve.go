package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/server"
	"go.lsp.dev/jsonrpc2"
	"go.uber.org/zap"
)

// Serve listens on the given Unix socket path and serves LSP sessions sharing
// a single Index. It blocks until ctx is cancelled. If a ConnTracker is
// provided, each socket connection is tracked.
func Serve(ctx context.Context, sockPath string, logger *zap.Logger, idx *server.Index, ct *ConnTracker, folders []string, globs []string) error {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		return err
	}
	os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	defer func() {
		ln.Close()
		os.Remove(sockPath)
	}()

	logger.Info("daemon listening", zap.String("socket", sockPath))

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		c, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return nil
			default:
				return err
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.Close()
			if ct != nil {
				ct.Add()
				defer ct.Remove()
			}
			stream := jsonrpc2.NewStream(c)
			conn := jsonrpc2.NewConn(stream)
			srv := server.NewServer(conn, logger, idx)
			srv.WorkspaceFolders = folders
			srv.IndexGlobs = globs
			conn.Go(ctx, srv.Handler())
			<-conn.Done()
		}()
	}
}

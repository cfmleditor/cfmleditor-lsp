//go:build !wasm

package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/server"
	"go.lsp.dev/jsonrpc2"
)

// Serve listens on the given Unix socket path and serves LSP sessions sharing
// a single Index. It blocks until ctx is cancelled. If a ConnTracker is
// provided, each socket connection is tracked.
func Serve(ctx context.Context, sockPath string, log cflog.Logger, idx *index.Index, ct *ConnTracker, settings server.Settings) error {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		return err
	}

	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}

	defer func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
	}()

	log.Info("daemon listening", cflog.String("socket", sockPath))

	var wg sync.WaitGroup

	go func() {
		<-ctx.Done()

		_ = ln.Close()
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

		wg.Go(func() {
			defer func() { _ = c.Close() }()

			if ct != nil {
				ct.Add()
				defer ct.Remove()
			}

			stream := jsonrpc2.NewStream(c)
			conn := jsonrpc2.NewConn(stream)
			srv := server.NewServer(conn, log, idx)
			settings.Apply(srv)
			conn.Go(ctx, srv.Handler())

			select {
			case <-conn.Done():
			case <-ctx.Done():
				_ = c.Close()

				<-conn.Done()
			}
		})
	}
}

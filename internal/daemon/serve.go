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
func Serve(ctx context.Context, sockPath string, log cflog.Logger, idx *index.Index, ct *ConnTracker, settings *server.Settings) error {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		return err
	}

	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}

	// Closing the listener is the whole of the cleanup: a UnixListener that
	// created its own socket file unlinks that file on Close, and the
	// goroutine below closes it the moment ctx is cancelled — while this
	// daemon still holds the binding, so the name it removes is certainly its
	// own. TestUnixListenerUnlinksOnClose pins that behaviour, since the
	// reasoning here depends on it.
	//
	// There used to be an os.Remove(sockPath) here too, after wg.Wait() had
	// drained every connection. It could never remove this daemon's socket:
	// Close had already unlinked it, possibly long before. What it removed was
	// whatever had taken the path since — a successor's socket. That is a
	// stray unlink by construction rather than a race lost occasionally.
	//
	// It happens because a closed listener leaves the path free while this
	// daemon is still draining. An editor starting then dials, fails,
	// concludes no daemon is running, and binds its own socket there. The
	// successor never notices it was unlinked: it goes on serving the client
	// it already has over an inode with no name, so from the inside it is a
	// working daemon, while from the outside every editor started afterwards
	// dials nothing, becomes a daemon of its own, and builds its own copy of
	// the workspace index. The visible cost is memory and repeated indexing
	// that read as a leak rather than a lifecycle bug.
	//
	// Comparing os.Stat identity before removing is the fix that looks right
	// and is not: a successor binding straight after the unlink is routinely
	// handed the inode just freed, so os.SameFile calls the two sockets the
	// same file. It passed on macOS and failed on Linux, where inode reuse is
	// prompt. There is nothing to compare — the correct number of removes on
	// this path is none.
	defer func() { _ = ln.Close() }()

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

package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"go.lsp.dev/jsonrpc2"
	"go.uber.org/zap"
)

func TestConnTrackerShutdownOnLastDisconnect(t *testing.T) {
	ct := NewConnTracker()
	ct.Add()
	ct.Add()

	ct.Remove()
	select {
	case <-ct.Done():
		t.Fatal("Done closed with one connection remaining")
	default:
	}

	ct.Remove()
	select {
	case <-ct.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after all connections removed")
	}
}

func TestConnTrackerSafeDoubleZero(t *testing.T) {
	ct := NewConnTracker()
	ct.Add()
	ct.Remove() // closes Done
	ct.Add()
	ct.Remove() // should not panic
}

func TestServeMultipleClients(t *testing.T) {
	sock := shortSock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zap.NewNop()
	idx := index.New()
	ct := NewConnTracker()

	// Simulate the stdio client that main.go adds before Serve starts
	ct.Add()

	go Serve(ctx, sock, logger, idx, ct, nil, nil)
	waitForSocket(t, sock)

	// Two socket clients connect (like additional editor windows)
	c1, rpc1 := dialRPC(t, ctx, sock)
	defer c1.Close()
	c2, rpc2 := dialRPC(t, ctx, sock)
	defer c2.Close()

	time.Sleep(50 * time.Millisecond)

	// Disconnect first socket client — daemon should stay alive
	rpc1.Close()
	c1.Close()
	time.Sleep(100 * time.Millisecond)
	select {
	case <-ct.Done():
		t.Fatal("daemon shut down with clients still connected")
	default:
	}

	// Disconnect second socket client — stdio client still holds it open
	rpc2.Close()
	c2.Close()
	time.Sleep(100 * time.Millisecond)
	select {
	case <-ct.Done():
		t.Fatal("daemon shut down with stdio client still connected")
	default:
	}

	// Stdio client disconnects — daemon should shut down
	ct.Remove()
	select {
	case <-ct.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not shut down after all clients disconnected")
	}
}

func TestProxyConnectsToExistingDaemon(t *testing.T) {
	sock := shortSock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zap.NewNop()
	idx := index.New()

	// No ConnTracker — we just verify the RPC layer works
	go Serve(ctx, sock, logger, idx, nil, nil, nil)
	waitForSocket(t, sock)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("proxy dial failed: %v", err)
	}
	defer conn.Close()

	stream := jsonrpc2.NewStream(conn)
	rpc := jsonrpc2.NewConn(stream)
	rpc.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		return reply(ctx, nil, nil)
	})

	var result json.RawMessage
	_, err = rpc.Call(ctx, "initialize", json.RawMessage(`{"capabilities":{}}`), &result)
	if err != nil {
		t.Fatalf("initialize call failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty initialize result")
	}
}

// helpers

func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "cfe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

func waitForSocket(t *testing.T, sock string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", sock); err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket never became available")
}

func dialRPC(t *testing.T, ctx context.Context, sock string) (net.Conn, jsonrpc2.Conn) {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	stream := jsonrpc2.NewStream(c)
	rpc := jsonrpc2.NewConn(stream)
	rpc.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		return reply(ctx, nil, nil)
	})
	return c, rpc
}

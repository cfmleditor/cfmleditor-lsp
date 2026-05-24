//go:build !wasm

package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
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

//nolint:revive // t is required by testing convention
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

	go func() { _ = Serve(ctx, sock, logger, idx, ct, nil, nil, nil, nil, nil, nil, config.ResolvedFormatting{}) }()
	waitForSocket(t, sock)

	// Connect 6 socket clients
	const total = 6
	conns := make([]net.Conn, total)
	rpcs := make([]jsonrpc2.Conn, total)
	for i := range total {
		conns[i], rpcs[i] = dialRPC(t, ctx, sock)
		defer func() { _ = conns[i].Close() }()
	}
	time.Sleep(50 * time.Millisecond)

	// Disconnect half — daemon must stay alive
	for i := range total / 2 {
		_ = rpcs[i].Close()
		_ = conns[i].Close()
	}
	time.Sleep(100 * time.Millisecond)
	select {
	case <-ct.Done():
		t.Fatal("daemon shut down with clients still connected")
	default:
	}

	// Disconnect the rest
	for i := total / 2; i < total; i++ {
		_ = rpcs[i].Close()
		_ = conns[i].Close()
	}
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
	go func(){ _ = Serve(ctx, sock, logger, idx, nil, nil, nil, nil, nil, nil, nil, config.ResolvedFormatting{}) }()
	waitForSocket(t, sock)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("proxy dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	stream := jsonrpc2.NewStream(conn)
	rpc := jsonrpc2.NewConn(stream)
	rpc.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error { //nolint:revive // req required by handler signature
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

func TestDaemonSurvivesAbruptClientDisconnect(t *testing.T) {
	sock := shortSock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idx := index.New()
	ct := NewConnTracker()
	ct.Add() // stdio slot

	go func() { _ = Serve(ctx, sock, zap.NewNop(), idx, ct, nil, nil, nil, nil, nil, nil, config.ResolvedFormatting{}) }()
	waitForSocket(t, sock)

	// Connect a client and immediately close the raw connection (simulates crash)
	c, _ := net.Dial("unix", sock)
	_ = c.Close()

	time.Sleep(100 * time.Millisecond)

	// Daemon must still be alive — stdio slot is still connected
	select {
	case <-ct.Done():
		t.Fatal("daemon shut down after abrupt client disconnect")
	default:
	}

	// A new client can still connect and make calls
	_, rpc := dialRPC(t, ctx, sock)
	callRPC(t, ctx, rpc, "initialize", `{"capabilities":{}}`)
}

func TestDaemonShutdownClosesSocketClients(t *testing.T) {
	sock := shortSock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idx := index.New()
	ct := NewConnTracker()
	ct.Add() // stdio slot

	go func() { _ = Serve(ctx, sock, zap.NewNop(), idx, ct, nil, nil, nil, nil, nil, nil, config.ResolvedFormatting{}) }()
	waitForSocket(t, sock)

	// Connect a client
	_, rpc := dialRPC(t, ctx, sock)
	callRPC(t, ctx, rpc, "initialize", `{"capabilities":{}}`)

	// Cancel context (simulates daemon shutdown)
	cancel()

	// The client's connection should close within a reasonable time
	select {
	case <-rpc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("socket client connection did not close after daemon shutdown")
	}
}

func TestMultipleConnectionsShareIndex(t *testing.T) {
	sock := shortSock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idx := index.New()
	ct := NewConnTracker()
	ct.Add() // stdio slot

	go func() { _ = Serve(ctx, sock, zap.NewNop(), idx, ct, nil, nil, nil, nil, nil, nil, config.ResolvedFormatting{}) }()
	waitForSocket(t, sock)

	// Client 1 opens a CFC file — this indexes it into the shared index
	_, rpc1 := dialRPC(t, ctx, sock)
	callRPC(t, ctx, rpc1, "initialize", `{"capabilities":{}}`)
	callRPC(t, ctx, rpc1, "textDocument/didOpen", `{
		"textDocument":{
			"uri":"file:///project/User.cfc",
			"languageId":"cfml",
			"version":1,
			"text":"component {\n  public void function getUser() {}\n}"
		}
	}`)

	// Give indexing a moment
	time.Sleep(50 * time.Millisecond)

	// Client 2 connects and queries workspace symbols — should see getUser
	_, rpc2 := dialRPC(t, ctx, sock)
	callRPC(t, ctx, rpc2, "initialize", `{"capabilities":{}}`)

	var symbols []json.RawMessage
	raw := callRPC(t, ctx, rpc2, "workspace/symbol", `{"query":"getUser"}`)
	if err := json.Unmarshal(raw, &symbols); err != nil {
		t.Fatalf("unmarshal symbols: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("client 2 could not find symbol indexed by client 1")
	}
}

// helpers

func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "cfe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { 
		_ = os.RemoveAll(dir)
	})
	return filepath.Join(dir, "d.sock")
}

func waitForSocket(t *testing.T, sock string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", sock); err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket never became available")
}

//nolint:revive // context-as-argument: keeping t first for test helper consistency
func dialRPC(t *testing.T, ctx context.Context, sock string) (net.Conn, jsonrpc2.Conn) {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	stream := jsonrpc2.NewStream(c)
	rpc := jsonrpc2.NewConn(stream)
	rpc.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error { //nolint:revive // req required by handler signature
		return reply(ctx, nil, nil)
	})
	return c, rpc
}

//nolint:revive // context-as-argument: keeping t first for test helper consistency
func callRPC(t *testing.T, ctx context.Context, rpc jsonrpc2.Conn, method, params string) json.RawMessage {
	t.Helper()
	var result json.RawMessage
	_, err := rpc.Call(ctx, method, json.RawMessage(params), &result)
	if err != nil {
		t.Fatalf("%s failed: %v", method, err)
	}
	return result
}

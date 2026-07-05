package server

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// poisonedContext simulates go.lsp.dev/jsonrpc2's incomingRequest: real
// jsonrpc2 pools and resets a request's context (parent = nil) as soon as
// the synchronous handler returns (conn.go: putIncomingRequest). Any method
// call on the context afterward panics ("cannot create context from nil
// parent"). A handler that hands this context to a goroutine that outlives
// the handler is using it after it's been invalidated.
type poisonedContext struct {
	poisoned *atomic.Bool
}

func (p *poisonedContext) check() {
	if p.poisoned.Load() {
		panic("cannot create context from nil parent")
	}
}

func (p *poisonedContext) Deadline() (time.Time, bool) {
	p.check()

	return time.Time{}, false
}

func (p *poisonedContext) Done() <-chan struct{} {
	p.check()

	return nil
}

func (p *poisonedContext) Err() error {
	p.check()

	return nil
}

func (p *poisonedContext) Value(any) any {
	p.check()

	return nil
}

// fakeConn is a minimal jsonrpc2.Conn that exercises whatever context it's
// given, the way a real connection implementation would (e.g. checking
// ctx.Err() before writing).
type fakeConn struct {
	onNotify func(ctx context.Context, method string)
}

func (f *fakeConn) Call(ctx context.Context, _ string, _, _ any) (jsonrpc2.ID, error) {
	_ = ctx.Err()

	return jsonrpc2.ID{}, nil
}

func (f *fakeConn) Notify(ctx context.Context, method string, _ any) error {
	_ = ctx.Err()

	if f.onNotify != nil {
		f.onNotify(ctx, method)
	}

	return nil
}

func (f *fakeConn) Go(context.Context, jsonrpc2.Handler) {}
func (f *fakeConn) Close() error                         { return nil }
func (f *fakeConn) Done() <-chan struct{}                { return nil }
func (f *fakeConn) Err() error                           { return nil }

// recordingLogger captures Error() messages so the test can detect a
// recovered goroutine panic (safeGo logs it rather than letting it crash
// the process).
type recordingLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}

func (l *recordingLogger) Error(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, msg)
}

func (l *recordingLogger) hasError(msg string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, e := range l.errors {
		if e == msg {
			return true
		}
	}

	return false
}

// TestScanWorkspaceDoesNotUseRequestContextAfterHandlerReturns guards against
// regressing the "cannot create context from nil parent" panic seen in
// production: cfmleditor.scanWorkspace runs in a detached goroutine
// (s.safeGo) that outlives handleExecuteCommand, so it must not use the
// request's ctx once the handler returns.
func TestScanWorkspaceDoesNotUseRequestContextAfterHandlerReturns(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.cfm"), []byte("<cfoutput><cfif>unclosed"), 0o644)

	done := make(chan struct{})

	fc := &fakeConn{
		onNotify: func(_ context.Context, method string) {
			if method == protocol.MethodWindowShowMessage {
				close(done)
			}
		},
	}
	logger := &recordingLogger{}

	srv := NewServer(fc, logger)
	srv.WorkspaceFolders = []string{dir}

	var poisoned atomic.Bool

	ctx := &poisonedContext{poisoned: &poisoned}

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command: "cfmleditor.scanWorkspace",
	})

	if _, err := srv.handleExecuteCommand(ctx, req); err != nil {
		t.Fatal(err)
	}

	// Simulate jsonrpc2 pooling/resetting the incoming request's context
	// immediately after the handler returns.
	poisoned.Store(true)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if logger.hasError("goroutine panic") {
			t.Fatal("scanWorkspace panicked using a stale request context; the detached goroutine must use context.Background(), not the handler's ctx")
		}

		t.Fatal("scanWorkspace did not complete in time")
	}

	if logger.hasError("goroutine panic") {
		t.Fatal("scanWorkspace panicked using a stale request context; the detached goroutine must use context.Background(), not the handler's ctx")
	}
}

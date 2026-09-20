package daemon

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/server"
)

// TestConnTrackerReArmsAfterZero is the defect that dropped an editor's LSP
// connection for no visible reason. Done() was closed once and never reopened,
// so a daemon that touched zero clients was committed to dying even though a
// client had already arrived — it was accepted, counted, and then cut off.
func TestConnTrackerReArmsAfterZero(t *testing.T) {
	t.Parallel()

	ct := NewConnTracker()

	ct.Add()
	ct.Remove() // the transient zero

	// The next editor arrives before the daemon has finished going away.
	ct.Add()

	select {
	case <-ct.Done():
		t.Fatal("Done still closed with a connection active: the arriving client would be killed")
	default:
	}

	if got := ct.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1", got)
	}

	// And the re-armed channel must still be able to fire, or the daemon
	// outlives its last client forever instead.
	ct.Remove()

	select {
	case <-ct.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after the re-armed tracker reached zero again")
	}
}

// TestWaitForLastClientIgnoresATransientZero is the other half: re-arming the
// channel cannot call back a waiter that has already woken on the closed one,
// so the waiter has to serve out a grace period and re-check the count.
//
// The head start below is load-bearing. Without it the test races its own
// subject: if the waiter reaches Done() after the Add on line "handover"
// below, it reads the re-armed open channel, blocks, and the test passes
// against a WaitForLastClient that does nothing but a single receive. Written
// that way it passed with the fix reverted about as often as not.
func TestWaitForLastClientIgnoresATransientZero(t *testing.T) {
	t.Parallel()

	const grace = 300 * time.Millisecond

	ct := NewConnTracker()
	ct.Add()

	returned := make(chan struct{})

	go func() {
		WaitForLastClient(t.Context(), ct, grace)
		close(returned)
	}()

	// Let the waiter park on the open channel, so the close below is something
	// it observes rather than something it may or may not have got to yet.
	time.Sleep(100 * time.Millisecond)

	select {
	case <-returned:
		t.Fatal("WaitForLastClient returned while a client was still connected")
	default:
	}

	// Handover: the last client goes and a replacement joins inside the grace.
	ct.Remove()
	ct.Add()

	select {
	case <-returned:
		t.Fatal("WaitForLastClient returned on a transient zero; the daemon would shut down under a live client")
	case <-time.After(grace * 3):
	}

	// The replacement leaving is a real last-client departure.
	ct.Remove()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForLastClient did not return after the real last client left")
	}
}

// TestWaitForLastClientReturnsOnCancel keeps the loop from outliving its ctx.
func TestWaitForLastClientReturnsOnCancel(t *testing.T) {
	t.Parallel()

	ct := NewConnTracker()
	ct.Add() // never removed

	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})

	go func() {
		WaitForLastClient(ctx, ct, time.Hour)
		close(returned)
	}()

	cancel()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForLastClient ignored a cancelled context")
	}
}

// TestServeCleansUpItsSocket is the regression guard for the second half of
// this lifecycle bug — narrower than the bug, deliberately, and the reason is
// worth writing down.
//
// Serve used to unlink the socket on its way out, after wg.Wait() had drained
// every connection. The listener closes as soon as ctx is cancelled, so there
// was a window where the path existed but nothing answered on it. An editor
// starting in that window dials, fails, concludes no daemon is running,
// removes the stale path and binds its own socket there — and the departing
// daemon then removed *that* socket by path. The successor kept serving the
// client it already had, over an inode with no name, so it looked healthy from
// the inside while every later editor dialled nothing, became a daemon of its
// own, and built its own copy of the workspace index.
//
// The fix is an ordering: unlink while the listener is still open, where the
// name is ours by definition and cannot yet be anyone else's.
//
// That ordering is not what this test checks, because it cannot be checked
// honestly here. Serve closes its own connections when ctx is cancelled, so
// the drain is microseconds wide however many clients are attached, and the
// two orderings differ only across it. Sampling for the window means a test
// that usually misses it — and a flaky test claiming to guard an ordering is
// worse than an untested ordering that is correct by construction. Two
// attempts are recorded in the history: one compared os.Stat identity, whose
// own guard caught that a successor is routinely handed the inode just freed
// (passed on macOS, failed on Linux); one spun on dial until it failed, which
// a full listen backlog also produces while the listener is still open.
//
// What is left is the part that is both testable and the real regression risk
// now that cleanup has moved off the exit path: that it still happens at all.
func TestServeCleansUpItsSocket(t *testing.T) {
	t.Parallel()

	sock := shortSock(t)

	ctx, cancel := context.WithCancel(context.Background())

	served := make(chan struct{})

	go func() {
		_ = Serve(ctx, sock, cflog.NewLogger(false), index.New(), nil, server.Settings{})

		close(served)
	}()

	waitForSocket(t, sock)

	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("socket missing while the daemon is up: %v", err)
	}

	cancel()

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}

	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket left behind after shutdown (stat err = %v); a stale path is what makes the next client rebuild the index", err)
	}
}

// TestUnixListenerUnlinksOnClose pins the standard-library behaviour Serve's
// socket cleanup rests on: a UnixListener that created its own file unlinks
// that file when it is closed.
//
// Serve does no explicit removal on shutdown *because* of this. If a future Go
// release changed it, or someone called SetUnlinkOnClose(false), the daemon
// would leave its socket behind and the next client would find a path it
// cannot dial — recoverable, since a stale socket is replaced, but it would
// silently undo the reasoning in Serve rather than fail anywhere near it.
func TestUnixListenerUnlinksOnClose(t *testing.T) {
	t.Parallel()

	sock := shortSock(t)

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("listener did not create its socket file: %v", err)
	}

	_ = ln.Close()

	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("Close did not unlink the socket (stat err = %v); Serve relies on it doing so", err)
	}
}

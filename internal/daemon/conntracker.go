package daemon

import (
	"context"
	"sync"
	"time"
)

// ConnTracker tracks active connections and signals when all have disconnected.
//
// Done() is level-triggered rather than one-shot: it closes when the count
// reaches zero and is *reopened* when a connection arrives afterwards. It has
// to be, because reaching zero is not the same as being finished. A daemon
// serves several editors, and closing one window while opening another leaves
// a window — microseconds wide, but hit routinely in practice — where the
// count is zero and a client is already connecting.
//
// While Done() was one-shot the daemon took that transient zero as final. The
// arriving client was accepted, counted, and then killed anyway when the
// shutdown it could not cancel went through: its next write got EPIPE, the
// editor reported "connection to server is erroring" and restarted the server,
// and the restarted process found no daemon on the socket and became one. The
// symptom was an editor dropping its LSP connection for no reason a user could
// see, and it cost a full workspace re-index each time.
//
// Reopening the channel is necessary but not sufficient: a shutdown that has
// already woken on the closed channel cannot be called back by swapping the
// field underneath it. The waiter has to re-check Count() after a grace
// period and go back to waiting if a client turned up — see the loop in
// cmd/cfmleditor-lsp/main.go, which is the other half of this.
type ConnTracker struct {
	mu    sync.Mutex
	count int
	done  chan struct{}
}

// NewConnTracker creates a ConnTracker ready to track connections.
func NewConnTracker() *ConnTracker {
	return &ConnTracker{done: make(chan struct{})}
}

// Add increments the active connection count, re-arming Done() if a previous
// drop to zero had closed it.
func (ct *ConnTracker) Add() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Only the 0 -> 1 transition can find Done() closed, and a fresh channel
	// is what lets the next drop to zero signal again. Without it a daemon
	// that ever reached zero could never report reaching it a second time, so
	// the second editor to disconnect would leave the process running forever.
	if ct.count == 0 {
		select {
		case <-ct.done:
			ct.done = make(chan struct{})
		default:
		}
	}

	ct.count++
}

// Remove decrements the count and closes Done() when it reaches zero.
func (ct *ConnTracker) Remove() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if ct.count == 0 {
		return
	}

	ct.count--
	if ct.count == 0 {
		select {
		case <-ct.done:
		default:
			close(ct.done)
		}
	}
}

// Count returns the number of active connections. A waiter woken by Done()
// uses it to tell a final zero from a transient one.
func (ct *ConnTracker) Count() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	return ct.count
}

// Done returns a channel that is closed while no connections are active. The
// channel is replaced when a connection arrives, so a caller must re-read it
// on each wait rather than hold on to one — see the type's comment.
func (ct *ConnTracker) Done() <-chan struct{} {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	return ct.done
}

// DrainGrace is how long the daemon waits, after its last client goes, for
// another to arrive before it shuts down. It covers an editor handing over to
// a replacement window, which is the case that made the transient zero worth
// distinguishing at all; it is not a timeout anything correctness-critical
// depends on, so it is generous rather than tuned.
const DrainGrace = 2 * time.Second

// WaitForLastClient blocks until ct has been at zero connections for grace,
// or ctx is cancelled. A zero that a client arrives during does not end the
// wait — see ConnTracker for why a transient one is routine and what taking
// it as final used to cost.
func WaitForLastClient(ctx context.Context, ct *ConnTracker, grace time.Duration) {
	for {
		// Re-read Done() each time round: Add replaces the channel, so a
		// handle kept from the previous iteration is the stale closed one and
		// would spin.
		select {
		case <-ct.Done():
		case <-ctx.Done():
			return
		}

		select {
		case <-time.After(grace):
		case <-ctx.Done():
			return
		}

		if ct.Count() == 0 {
			return
		}
	}
}

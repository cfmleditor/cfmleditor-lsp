package daemon

import "sync"

// ConnTracker tracks active connections and signals when all have disconnected.
type ConnTracker struct {
	mu   sync.Mutex
	count int
	done  chan struct{}
}

func NewConnTracker() *ConnTracker {
	return &ConnTracker{done: make(chan struct{})}
}

// Add increments the active connection count.
func (ct *ConnTracker) Add() {
	ct.mu.Lock()
	ct.count++
	ct.mu.Unlock()
}

// Remove decrements the count and closes Done() when it reaches zero.
func (ct *ConnTracker) Remove() {
	ct.mu.Lock()
	ct.count--
	if ct.count == 0 {
		close(ct.done)
	}
	ct.mu.Unlock()
}

// Done returns a channel that is closed when all tracked connections have ended.
func (ct *ConnTracker) Done() <-chan struct{} {
	return ct.done
}

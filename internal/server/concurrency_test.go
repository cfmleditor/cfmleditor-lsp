package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// A ParseResult is mutated in place — by ApplyEdit and ApplyFullReplace, and
// also by its read accessors, which memoise lazily. The server shares one per
// document between the LSP read goroutine and its own timers, so a burst of
// typing arms the deferred-reindex timer and 200ms later it calls
// ApplyFullReplace on exactly the ParseResult the next request is reading.
//
// The loop below is one sequential client, which is how jsonrpc2 dispatches a
// connection (the handler never calls Async), plus a goroutine doing what the
// rapid-change timer body does. Under -race this reported reparseShallow
// against computeScopedVars before the per-document lock existed.
func TestDidChangeAgainstRapidChangeTimer(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///concurrent.cfm")
	body := "<cfscript>\nx = 1;\nfunction f() { return 1; }\n</cfscript>"
	srv.setDocument(docURI, body)

	req := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: body},
		},
	})

	stop := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			default:
			}

			srv.mu.Lock()
			delete(srv.changeCount, docURI)
			delete(srv.changeWindowStart, docURI)
			srv.mu.Unlock()
		}
	}()

	deadline := time.Now().Add(750 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := srv.handleDidChange(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}

	close(stop)
	wg.Wait()

	// Let any still-pending timer run against a document that is still open.
	time.Sleep(300 * time.Millisecond)
}

// The two timer kinds used to share one slot per document in s.cacheTimers, so
// the first ordinary keystroke after a burst armed a cache rebuild that
// Stop()ped the pending reindex sitting in that slot and dropped it — the
// pasted text never reached the index.
func TestReindexAndCacheTimersDoNotShareASlot(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///timers.cfm")
	body := "<cfscript>\nfunction f() { var a = 1; return a; }\n</cfscript>"
	srv.setDocument(docURI, body)

	burst := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: body},
		},
	})

	// Enough changes inside the window to trip the rapid-change path.
	for range 8 {
		if _, err := srv.handleDidChange(context.Background(), burst); err != nil {
			t.Fatal(err)
		}
	}

	srv.mu.RLock()
	_, hasReindex := srv.reindexTimers[docURI]
	srv.mu.RUnlock()

	if !hasReindex {
		t.Fatal("a rapid-change burst did not arm a deferred reindex")
	}

	// A cache rebuild armed afterwards must not disturb it.
	srv.debounceCacheRebuild(docURI, body, 1)

	srv.mu.RLock()
	_, stillThere := srv.reindexTimers[docURI]
	_, hasCache := srv.cacheTimers[docURI]
	srv.mu.RUnlock()

	if !stillThere {
		t.Error("arming a cache rebuild cancelled the pending reindex")
	}

	if !hasCache {
		t.Error("cache rebuild timer was not armed")
	}
}

// lockDoc hands out plain sync.Mutexes, so a locked function calling another
// locked function on the same goroutine hangs rather than nesting. didSave's
// cache rebuild is the path where that is easiest to reintroduce: it takes the
// lock and then delegates to the from-ParseResult variant. Nothing in the
// ordinary suite would notice, because didSave does this work in a goroutine it
// never waits for — hence the explicit timeout here.
func TestFileCompletionCacheRebuildDoesNotSelfDeadlock(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///rebuild.cfc")
	srv.setDocument(docURI, "component { variables.x = 1; function f() { return 1; } }")

	done := make(chan struct{})

	go func() {
		defer close(done)

		srv.rebuildFileCompletionCache(docURI)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("rebuildFileCompletionCache deadlocked on the document lock")
	}

	// And again with a ParseResult already cached, which takes the other branch.
	done2 := make(chan struct{})

	go func() {
		defer close(done2)

		srv.rebuildFileCompletionCache(docURI)
	}()

	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("rebuildFileCompletionCache deadlocked with a cached ParseResult")
	}
}

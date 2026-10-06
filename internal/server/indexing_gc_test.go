package server

import (
	"runtime/debug"
	"testing"
)

// An indexing scan collects more often while it runs and puts GOGC back when
// the last scan running ends; a lower setting, or the collector off, is left
// as the user set it.
func TestIndexingLowersGOGCOnlyWhileAScanRuns(t *testing.T) {
	orig := debug.SetGCPercent(100)
	defer debug.SetGCPercent(orig)

	first := indexingGC()
	second := indexingGC()

	if got := debug.SetGCPercent(indexingGCPercent); got != indexingGCPercent {
		t.Errorf("during a scan GOGC is %d, want %d", got, indexingGCPercent)
	}

	first()

	if got := debug.SetGCPercent(indexingGCPercent); got != indexingGCPercent {
		t.Errorf("with a scan still running GOGC is %d, want %d", got, indexingGCPercent)
	}

	second()

	if got := debug.SetGCPercent(100); got != 100 {
		t.Errorf("after the scans GOGC is %d, want 100", got)
	}

	for _, user := range []int{20, -1} {
		debug.SetGCPercent(user)

		done := indexingGC()

		if got := debug.SetGCPercent(user); got != user {
			t.Errorf("GOGC %d was changed to %d during a scan", user, got)
		}

		done()

		if got := debug.SetGCPercent(100); got != user {
			t.Errorf("GOGC %d was %d after a scan", user, got)
		}
	}
}

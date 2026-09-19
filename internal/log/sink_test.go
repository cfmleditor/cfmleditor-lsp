package log_test

import (
	"strings"
	"sync"
	"testing"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

type capture struct {
	mu      sync.Mutex
	records []string
	levels  []int
}

func (c *capture) Log(level int, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.levels = append(c.levels, level)
	c.records = append(c.records, msg)
}

func (c *capture) snapshot() ([]int, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]int(nil), c.levels...), append([]string(nil), c.records...)
}

// TestSinkReceivesEveryLevelWithItsLSPType. The whole point is that the client
// can tell a routine line from a failure: every record reaching it as stderr was
// reported at error level, so the version banner and a crash looked alike. A
// level mapped wrongly here puts the distinction back where it was.
func TestSinkReceivesEveryLevelWithItsLSPType(t *testing.T) {
	logger, ok := cflog.NewLogger(true).(cflog.Teeable)
	if !ok {
		t.Fatal("the logger is no longer Teeable; nothing can forward to the client")
	}

	sink := &capture{}
	logger.Attach(sink)

	logger.Error("boom")
	logger.Warn("careful")
	logger.Info("hello")
	logger.Debug("details")

	levels, records := sink.snapshot()

	// LSP MessageType: 1 error, 2 warning, 3 info, 4 log.
	wantLevels := []int{1, 2, 3, 4}
	wantMsgs := []string{"boom", "careful", "hello", "details"}

	if len(levels) != len(wantLevels) {
		t.Fatalf("got %d records %v, want %d", len(levels), records, len(wantLevels))
	}

	for i := range wantLevels {
		if levels[i] != wantLevels[i] || records[i] != wantMsgs[i] {
			t.Errorf("record %d = (%d, %q), want (%d, %q)", i, levels[i], records[i], wantLevels[i], wantMsgs[i])
		}
	}
}

// TestSinkCarriesTheStructuredFields, or the Output panel shows a bare message
// where stderr showed the detail that made it useful.
func TestSinkCarriesTheStructuredFields(t *testing.T) {
	logger := cflog.NewLogger(false).(cflog.Teeable)
	sink := &capture{}
	logger.Attach(sink)

	logger.Info("indexing complete", "files", 4646, "dur", "1.5s")

	_, records := sink.snapshot()
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	for _, want := range []string{"indexing complete", "files=4646", "dur=1.5s"} {
		if !strings.Contains(records[0], want) {
			t.Errorf("record %q is missing %q", records[0], want)
		}
	}
}

// TestDetachStops. A notification written to a connection that has gone away is
// at best ignored and at worst an error logged about logging, so shutdown has to
// be able to stop this cleanly.
func TestDetachStops(t *testing.T) {
	logger := cflog.NewLogger(false).(cflog.Teeable)
	sink := &capture{}

	logger.Attach(sink)
	logger.Info("before")
	logger.Attach(nil)
	logger.Info("after")

	_, records := sink.snapshot()
	if len(records) != 1 || records[0] != "before" {
		t.Errorf("records = %v, want just [before]", records)
	}
}

// TestLoggingIsSafeFromManyGoroutines: the sink is swapped at startup and
// shutdown while every request goroutine is writing through it.
func TestLoggingIsSafeFromManyGoroutines(t *testing.T) {
	logger, ok := cflog.NewLogger(false).(cflog.Teeable)
	if !ok {
		t.Fatal("the logger is no longer Teeable")
	}

	sink := &capture{}
	logger.Attach(sink)

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 50 {
				logger.Info("concurrent")
			}
		}()
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for range 50 {
			logger.Attach(sink)
			logger.Attach(nil)
		}
	}()

	wg.Wait()
}

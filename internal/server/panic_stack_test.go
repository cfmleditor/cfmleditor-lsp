package server

import (
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

// fieldLogger keeps each Error record's fields, which recordingLogger drops.
type fieldLogger struct {
	mu      sync.Mutex
	records map[string][]any
}

func (l *fieldLogger) Debug(string, ...any) {}
func (l *fieldLogger) Info(string, ...any)  {}
func (l *fieldLogger) Warn(string, ...any)  {}

func (l *fieldLogger) Error(msg string, kv ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.records[msg] = kv
}

func (l *fieldLogger) field(msg, key string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	kv, ok := l.records[msg]
	if !ok {
		return "", false
	}

	for _, v := range kv {
		f, isField := v.(zap.Field)
		if !isField || f.Key != key {
			continue
		}

		enc := zapcore.NewMapObjectEncoder()
		f.AddTo(enc)
		s, _ := enc.Fields[key].(string)

		return s, true
	}

	return "", true
}

// TestRecoveredGoroutinePanicRecordsItsStack. A recovered panic keeps the
// server running, so its log record is the only trace it leaves, and it held
// the panic's value alone — which names no function. safeGo is the recover
// behind every background job; the handler and the two timers log the same
// way and share the field.
func TestRecoveredGoroutinePanicRecordsItsStack(t *testing.T) {
	logger := &fieldLogger{records: map[string][]any{}}
	s := NewServer(nil, cflog.Logger(logger))

	done := make(chan struct{})

	s.safeGo("test", func() {
		defer close(done)

		backgroundJobThatPanics()
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the goroutine did not finish")
	}

	// The record is written by safeGo's deferred recover, which runs after
	// the job's own deferred close.
	var stack string

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got, logged := logger.field("goroutine panic", "stack"); logged {
			stack = got

			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(stack, "backgroundJobThatPanics") {
		t.Errorf("the goroutine panic record does not carry a stack naming the job:\n%s", stack)
	}
}

func backgroundJobThatPanics() {
	panic("job failed")
}

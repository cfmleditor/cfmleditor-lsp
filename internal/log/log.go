// Package log defines the shared Logger interface for the project.
package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Logger is the common logging interface used across all packages.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// String constructs a string field.
func String(key, val string) zap.Field { return zap.String(key, val) }

// Strings constructs a string slice field.
func Strings(key string, val []string) zap.Field { return zap.Strings(key, val) }

// Int constructs an integer field.
func Int(key string, val int) zap.Field { return zap.Int(key, val) }

// Bool constructs a boolean field.
func Bool(key string, val bool) zap.Field { return zap.Bool(key, val) }

// Duration constructs a duration field.
func Duration(key string, val time.Duration) zap.Field { return zap.Duration(key, val) }

// Any constructs a field with an arbitrary value.
func Any(key string, val any) zap.Field { return zap.Any(key, val) }

// Err constructs an error field.
func Err(err error) zap.Field { return zap.Error(err) }

// Uint32 constructs an unsigned 32-bit integer field.
func Uint32(key string, val uint32) zap.Field { return zap.Uint32(key, val) }

// NewLogger creates a Logger. When debug is true, logs at Debug level with
// human-readable output; otherwise logs at Info level with JSON output.
func NewLogger(debug bool) Logger {
	var (
		l   *zap.Logger
		err error
	)
	if debug {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)

		l = zap.NewNop()
	}

	return &zapLogger{l: l.WithOptions(zap.AddCallerSkip(1)).Sugar()}
}

type zapLogger struct {
	l *zap.SugaredLogger

	// Guards sink alone. A log call must never block on anything else, and the
	// sink is swapped once at startup and once at shutdown against readers on
	// every request goroutine.
	mu   sync.RWMutex
	sink Sink
}

func (z *zapLogger) Debug(msg string, kv ...any) {
	z.l.Debugw(msg, kv...)
	z.forward(lspLog, msg, kv)
}

func (z *zapLogger) Info(msg string, kv ...any) {
	z.l.Infow(msg, kv...)
	z.forward(lspInfo, msg, kv)
}

func (z *zapLogger) Warn(msg string, kv ...any) {
	z.l.Warnw(msg, kv...)
	z.forward(lspWarning, msg, kv)
}

func (z *zapLogger) Error(msg string, kv ...any) {
	z.l.Errorw(msg, kv...)
	z.forward(lspError, msg, kv)
}

// Sink receives log records for somewhere other than stderr.
//
// It exists because an LSP client has no way to tell a routine line on a
// server's stderr from a failure: vscode-languageclient logs every one of them
// at error level, so a healthy startup announced itself as
// "[error] cfmleditor-lsp dev" and every subsequent line looked like a fault
// too. The protocol's own channel for this is window/logMessage, which carries
// a severity the client can render honestly.
type Sink interface {
	// Log forwards one record. level is an LSP MessageType: 1 error, 2 warning,
	// 3 info, 4 log.
	Log(level int, msg string)
}

// Teeable is a Logger that can forward its records to a Sink as well as writing
// them to stderr.
//
// Both, not one or the other. Stderr is the only place a message can go before
// the client connects or after it disappears, and a crash during startup is
// exactly when the reason matters most — routing everything through the
// connection would lose precisely the records worth keeping.
type Teeable interface {
	Logger

	// Attach starts forwarding to sink. Passing nil stops it.
	Attach(sink Sink)
}

// LSP MessageType values, as window/logMessage defines them.
const (
	lspError   = 1
	lspWarning = 2
	lspInfo    = 3
	lspLog     = 4
)

func (z *zapLogger) Attach(sink Sink) {
	z.mu.Lock()
	z.sink = sink
	z.mu.Unlock()
}

// forward sends one record on, formatting the key/value pairs the way the
// structured log does so a reader of the Output panel sees the same detail.
func (z *zapLogger) forward(level int, msg string, kv []any) {
	z.mu.RLock()
	sink := z.sink
	z.mu.RUnlock()

	if sink == nil {
		return
	}

	if len(kv) == 0 {
		sink.Log(level, msg)

		return
	}

	var b strings.Builder

	b.WriteString(msg)

	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}

	sink.Log(level, b.String())
}

// Fatalf logs a message to stderr and exits the process.
func Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

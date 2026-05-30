// Package log defines the shared Logger interface for the project.
package log

import (
	"fmt"
	"os"
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

type zapLogger struct{ l *zap.SugaredLogger }

func (z *zapLogger) Debug(msg string, kv ...any) { z.l.Debugw(msg, kv...) }
func (z *zapLogger) Info(msg string, kv ...any)  { z.l.Infow(msg, kv...) }
func (z *zapLogger) Warn(msg string, kv ...any)  { z.l.Warnw(msg, kv...) }
func (z *zapLogger) Error(msg string, kv ...any) { z.l.Errorw(msg, kv...) }

// Fatalf logs a message to stderr and exits the process.
func Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

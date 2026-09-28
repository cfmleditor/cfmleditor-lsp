package log

// Diagnostics for a server nobody can see: a copy of the log on disk, and a
// panic that reaches it.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// FileEnv names the environment variable that sends a copy of every log record
// to a file, in addition to stderr.
//
// It exists because stderr is not a reliable place to find out why the server
// stopped. An LSP client owns that pipe: when the client goes away — or decides
// the server has failed and stops reading — a write to it fails, and the last
// thing the server had to say goes nowhere. That is exactly the moment the
// reason matters. A file is written by the server, for the server, and survives
// whatever happened to the connection.
const FileEnv = "CFMLEDITOR_LSP_LOG"

var (
	fileMu  sync.Mutex
	logFile *os.File
)

// openLogFile returns the file named by FileEnv, or nil when unset.
//
// Failure to open it is reported on stderr and otherwise ignored: a diagnostic
// aid must never be the reason the thing it is diagnosing will not start.
func openLogFile() *os.File {
	path := os.Getenv(FileEnv)
	if path == "" {
		return nil
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	if logFile != nil {
		return logFile
	}

	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o700) //nolint:gosec // the path is the user's own CFMLEDITOR_LSP_LOG
	}

	// Private: a debug log can hold source text.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // the path is the user's own CFMLEDITOR_LSP_LOG
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfmleditor-lsp: cannot open %s=%s: %v\n", FileEnv, path, err)

		return nil
	}

	logFile = f

	// The runtime's own crash report goes to the file too. A panic in a
	// goroutine nothing recovers, and a fatal error — a fault in tree-sitter's
	// C code, a concurrent map write, a stack overflow, running out of memory —
	// ends the process without passing through CapturePanic or any logger, and
	// all the runtime writes is a traceback on stderr. That was the one record
	// of the crashes this file exists to catch, and it went to a pipe the
	// client may already have stopped reading.
	//
	// For a fatal error the runtime prints its one-line reason ("fatal error:
	// stack overflow", "fatal error: concurrent map writes") to stderr alone,
	// before it switches output here. The file gets the traceback, which names
	// the fault in its frames — runtime.throw and runtime.newstack for a stack
	// overflow, maps.fatal for a concurrent map write — and the function it
	// happened in.
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		fmt.Fprintf(os.Stderr, "cfmleditor-lsp: cannot send crash output to %s: %v\n", path, err)
	}

	return f
}

// FilePath is the file records are being copied to, or "" when none is.
func FilePath() string {
	fileMu.Lock()
	defer fileMu.Unlock()

	if logFile == nil {
		return ""
	}

	return logFile.Name()
}

// fileCore is the zap core writing to the log file, or nil when there is none.
func fileCore(debugMode bool) zapcore.Core {
	f := openLogFile()
	if f == nil {
		return nil
	}

	level := zapcore.InfoLevel
	if debugMode {
		level = zapcore.DebugLevel
	}

	// The file is always the human-readable encoding, whatever stderr is using.
	// Its reader is someone looking for a reason, not a log pipeline.
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	return zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), zapcore.Lock(f), level)
}

// PanicStack is the current goroutine's stack as a log field, for the record
// of a recovered panic.
//
// Called from the deferred function that recovered, it still shows the frames
// that panicked: they are not unwound until that function returns. Without it
// the record held only the panic's value — "index out of range [3] with length
// 3" — which names no function and cannot be traced back to one.
func PanicStack() Field { return zap.String("stack", string(debug.Stack())) }

// WritePanic records a panic and its stack everywhere it can, then returns.
//
// With a log file configured, a panic that CapturePanic lets continue is then
// written to it a second time, by the runtime (see SetCrashOutput above). This
// record is the one with the time and the place it was caught.
//
// Directly to the file rather than through the logger: a panic means the
// process is in a state nothing should be trusted in, and the whole point of
// this record is that it survives. It also goes to stderr, for the case where
// someone is watching and no file was configured.
func WritePanic(where string, v any) {
	stack := debug.Stack()
	header := fmt.Sprintf("\n=== cfmleditor-lsp PANIC in %s at %s ===\npanic: %v\n",
		where, time.Now().Format(time.RFC3339), v)

	fmt.Fprint(os.Stderr, header)
	_, _ = os.Stderr.Write(stack)

	fileMu.Lock()
	f := logFile
	fileMu.Unlock()

	if f == nil {
		return
	}

	_, _ = f.WriteString(header)
	_, _ = f.Write(stack)
	_ = f.Sync()
}

// CapturePanic records any panic unwinding through the caller and then lets it
// continue.
//
// It re-panics rather than swallowing: the per-request handlers already recover
// where recovery is the right answer, and a panic reaching here is one none of
// them expected. Turning that into a quiet return would leave the process alive
// in a state nothing has reasoned about. The value here is the record, not the
// rescue.
//
// Usage: defer CapturePanic("runServer")().
func CapturePanic(where string) func() {
	return func() {
		r := recover()
		if r == nil {
			return
		}

		WritePanic(where, r)

		panic(r)
	}
}

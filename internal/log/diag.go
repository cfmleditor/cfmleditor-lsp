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
		_ = os.MkdirAll(dir, 0o755)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfmleditor-lsp: cannot open %s=%s: %v\n", FileEnv, path, err)

		return nil
	}

	logFile = f

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

// WritePanic records a panic and its stack everywhere it can, then returns.
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
// Usage: defer CapturePanic("runServer")()
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

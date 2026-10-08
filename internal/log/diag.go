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
const FileEnv = "CLIF_LOG"

// LegacyFileEnv is FileEnv's name while this server was cfmleditor-lsp, read
// when FileEnv is unset so an editor configured with it keeps its log.
const LegacyFileEnv = "CFMLEDITOR_LSP_LOG"

// Getenv returns the variable name, or legacy when name is unset: each
// CLIF_ variable was CFMLEDITOR_ something before the rename, and a setup that
// names the old one keeps working.
func Getenv(name, legacy string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}

	return os.Getenv(legacy)
}

var (
	fileMu  sync.Mutex
	logFile *os.File
)

// openLogFile returns the file named by FileEnv, or nil when unset.
//
// Failure to open it is reported on stderr and otherwise ignored: a diagnostic
// aid must never be the reason the thing it is diagnosing will not start.
func openLogFile() *os.File {
	path := Getenv(FileEnv, LegacyFileEnv)
	if path == "" {
		return nil
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	if logFile != nil {
		return logFile
	}

	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
	}

	// Private: a debug log can hold source text.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // the path is the user's own CLIF_LOG
	if err != nil {
		fmt.Fprintf(os.Stderr, "clif: cannot open %s=%s: %v\n", FileEnv, path, err)

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

// crashFileName is the file crash reports go to, under the server's own cache
// directory, when FileEnv names no log file.
const crashFileName = "crash.log"

var (
	crashMu   sync.Mutex
	crashFile *os.File
)

// DefaultCrashPath is where crash reports go when FileEnv is unset: the
// server's directory in the user cache directory, beside the cflint/ it
// already keeps there. On macOS that is ~/Library/Caches/clif.
func DefaultCrashPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "clif", crashFileName), nil
}

// EnableCrashReports sends every crash record to a file, whether or not anyone
// configured one, and returns its path — "" when no file could be opened.
//
// The file is FileEnv's when that is set, and DefaultCrashPath's otherwise.
// Only the server calls this: a CLI command's crash lands on the terminal that
// ran it, and a test binary must not write to the user's cache directory.
//
// Three kinds of record reach it. The runtime's own crash report, through
// debug.SetCrashOutput: a panic in a goroutine nothing recovers, and a fatal
// error — a fault in tree-sitter's C code, a concurrent map write, a stack
// overflow, running out of memory — end the process without passing through
// any code of ours, and all the runtime writes is a traceback on stderr, which
// belongs to the editor and is lost when it stops reading. Then the panic
// CapturePanic records before it lets one end the process (WritePanic), and
// every panic a handler recovered from (Recovered), which leaves the server
// running and was otherwise only a log record.
//
// Each start appends one line naming the version, the process and the time, so
// a traceback below it can be tied to a run: the runtime's report carries none
// of the three. A file holding nothing else has seen no crash.
//
// For a fatal error the runtime prints its one-line reason ("fatal error:
// stack overflow", "fatal error: concurrent map writes") to stderr alone,
// before it switches output here. The file gets the traceback, which names the
// fault in its frames — runtime.throw and runtime.newstack for a stack
// overflow, maps.fatal for a concurrent map write — and the function it
// happened in.
func EnableCrashReports(version string) string {
	crashMu.Lock()
	defer crashMu.Unlock()

	if crashFile != nil {
		return crashFile.Name()
	}

	f := openLogFile()
	if f == nil {
		f = openDefaultCrashFile()
	}

	if f == nil {
		return ""
	}

	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		fmt.Fprintf(os.Stderr, "clif: cannot send crash output to %s: %v\n", f.Name(), err)
	}

	crashFile = f

	_, _ = fmt.Fprintf(f, "--- clif %s started, pid %d, %s ---\n",
		version, os.Getpid(), time.Now().Format(time.RFC3339))

	return f.Name()
}

// openDefaultCrashFile opens DefaultCrashPath for appending, or reports on
// stderr why it could not: a diagnostic aid must never stop the server
// starting.
func openDefaultCrashFile() *os.File {
	path, err := DefaultCrashPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "clif: no directory for crash reports: %v\n", err)

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "clif: cannot create %s: %v\n", filepath.Dir(path), err)

		return nil
	}

	// Private: a stack can name the files being edited.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // the path is built from os.UserCacheDir, not from input
	if err != nil {
		fmt.Fprintf(os.Stderr, "clif: cannot open %s: %v\n", path, err)

		return nil
	}

	return f
}

// CrashFilePath is the file crash records go to, or "" before
// EnableCrashReports has opened one.
func CrashFilePath() string {
	crashMu.Lock()
	defer crashMu.Unlock()

	if crashFile == nil {
		return ""
	}

	return crashFile.Name()
}

// writeCrashRecord appends one record to the crash file, if there is one.
//
// Directly to the file rather than through the logger: a panic means the
// process is in a state nothing should be trusted in, and the whole point of
// this record is that it survives.
func writeCrashRecord(header string, stack []byte) {
	crashMu.Lock()
	defer crashMu.Unlock()

	if crashFile == nil {
		return
	}

	_, _ = crashFile.WriteString(header)
	_, _ = crashFile.Write(stack)
	_ = crashFile.Sync()
}

// Recovered records a panic the caller recovered from and is carrying on
// after: an Error through l with the panic's value and its stack, and the same
// in the crash file.
//
// Call it from the deferred function that recovered, where the stack still
// shows the frames that panicked — they are not unwound until that function
// returns. Without the stack the record held only the value, "index out of
// range [3] with length 3", which names no function. l may be nil.
func Recovered(l Logger, msg string, r any, kv ...any) {
	stack := debug.Stack()

	if l != nil {
		l.Error(msg, append(kv, Any("panic", r), String("stack", string(stack)))...)
	}

	// Also when the crash file is the log file and l has just written this
	// record there: l's copy carries the stack as one escaped field, and this
	// one is the traceback a person can read.
	writeCrashRecord(fmt.Sprintf("\n=== clif recovered panic: %s at %s ===\npanic: %v\n",
		msg, time.Now().Format(time.RFC3339), r), stack)
}

// WritePanic records a panic and its stack everywhere it can, then returns.
//
// A panic that CapturePanic lets continue is then written to the crash file a
// second time, by the runtime (see EnableCrashReports). This record is the one
// with the time and the place it was caught. It also goes to stderr, for the
// case where someone is watching.
func WritePanic(where string, v any) {
	stack := debug.Stack()
	header := fmt.Sprintf("\n=== clif PANIC in %s at %s ===\npanic: %v\n",
		where, time.Now().Format(time.RFC3339), v)

	fmt.Fprint(os.Stderr, header)
	_, _ = os.Stderr.Write(stack)

	writeCrashRecord(header, stack)
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

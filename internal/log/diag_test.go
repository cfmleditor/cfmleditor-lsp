package log

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// crashChildEnv names what a child process of TestCrashReachesTheCrashFile is
// to do. Set only in that child.
const crashChildEnv = "CFMLEDITOR_LSP_TEST_CRASH"

// TestCrashReachesTheCrashFile is the reason EnableCrashReports exists. Neither
// crash passes through CapturePanic or any logger: a goroutine the server did
// not wrap in a recover, and a fatal error, which nothing can recover. Before,
// all either left was a traceback on stderr, which belongs to the editor.
//
// Both with a log file configured and without one, because the point is that
// a crash is recorded whether or not anyone asked. Each runs in a child
// process: each ends the process it happens in, and a test binary must not
// point its own crash output at the user's cache directory.
func TestCrashReachesTheCrashFile(t *testing.T) {
	if mode := os.Getenv(crashChildEnv); mode != "" {
		child(mode)

		return
	}

	// A fatal error's one-line reason goes to stderr only (see
	// EnableCrashReports), so that case is identified by its traceback.
	cases := map[string]string{
		"goroutine-panic": "panic: boom from a goroutine",
		"fatal":           "internal/log.recurse",
		"recovered":       "recovered panic: parse panic in test",
	}

	for _, configured := range []bool{true, false} {
		for mode, want := range cases {
			name := mode + "/default-file"
			if configured {
				name = mode + "/" + FileEnv
			}

			t.Run(name, func(t *testing.T) {
				got, out := runChild(t, mode, configured)

				for _, w := range []string{want, "goroutine ", "--- cfmleditor-lsp test-version started, pid "} {
					if !strings.Contains(got, w) {
						t.Errorf("the crash file does not hold %q; it holds:\n%s\nstderr was:\n%s", w, got, out)
					}
				}
			})
		}
	}
}

// runChild runs the crash named, and returns the crash file's contents and the
// child's output.
func runChild(t *testing.T, mode string, configured bool) (crashFile, output string) {
	t.Helper()

	home := t.TempDir()
	env := append(os.Environ(), crashChildEnv+"="+mode,
		"HOME="+home, "XDG_CACHE_HOME=", "LocalAppData="+home)

	var crashPath string

	if configured {
		crashPath = filepath.Join(t.TempDir(), "lsp.log")
		env = append(env, FileEnv+"="+crashPath)
	} else {
		// The path DefaultCrashPath gives under the child's environment.
		t.Setenv("HOME", home)
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("LocalAppData", home)

		p, err := DefaultCrashPath()
		if err != nil {
			t.Fatal(err)
		}

		crashPath = p

		env = append(env, FileEnv+"=")
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCrashReachesTheCrashFile$")

	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if _, crashed := errors.AsType[*exec.ExitError](err); !crashed && mode != "recovered" {
		t.Fatalf("the child did not crash (err %v):\n%s", err, out)
	}

	got, err := os.ReadFile(crashPath)
	if err != nil {
		t.Fatalf("reading the crash file %s: %v\nstderr was:\n%s", crashPath, err, out)
	}

	return string(got), string(out)
}

// child does what the server does at startup, then has the crash named.
func child(mode string) {
	EnableCrashReports("test-version")

	switch mode {
	case "goroutine-panic":
		done := make(chan struct{})

		go func() {
			panic("boom from a goroutine")
		}()

		<-done
	case "fatal":
		debug.SetMaxStack(1 << 20)

		depth = recurse(0)
	case "recovered":
		func() {
			defer func() {
				if r := recover(); r != nil {
					Recovered(nil, "parse panic in test", r)
				}
			}()

			panicsHere()
		}()
	}
}

// depth keeps recurse's result, so the call is not one the compiler or a
// linter can discard.
var depth int

// recurse overflows the stack: n only grows, so the base case never holds.
func recurse(n int) int {
	if n < 0 {
		return 0
	}

	return recurse(n+1) + 1
}

// fieldCapture keeps the fields of the one Error record it is given.
type fieldCapture struct {
	msg string
	kv  []any
}

func (c *fieldCapture) Debug(string, ...any) {}
func (c *fieldCapture) Info(string, ...any)  {}
func (c *fieldCapture) Warn(string, ...any)  {}

func (c *fieldCapture) Error(msg string, kv ...any) { c.msg, c.kv = msg, kv }

func (c *fieldCapture) field(key string) string {
	for _, v := range c.kv {
		if f, ok := v.(zap.Field); ok && f.Key == key {
			enc := zapcore.NewMapObjectEncoder()
			f.AddTo(enc)

			s, _ := enc.Fields[key].(string)

			return s
		}
	}

	return ""
}

// TestRecoveredRecordsTheStack pins what a recovered panic's record gains.
// The stack is read in the deferred function that recovered, before its
// frames are unwound, so it must still name the function that panicked; read
// anywhere later it would only name the recover site.
func TestRecoveredRecordsTheStack(t *testing.T) {
	c := &fieldCapture{}

	func() {
		defer func() {
			if r := recover(); r != nil {
				Recovered(c, "handler panic", r, String("method", "textDocument/hover"))
			}
		}()

		panicsHere()
	}()

	if c.msg != "handler panic" || c.field("method") != "textDocument/hover" {
		t.Errorf("the record lost its message or fields: %q %v", c.msg, c.kv)
	}

	if !strings.Contains(c.field("stack"), "panicsHere") {
		t.Errorf("the stack does not name the function that panicked:\n%s", c.field("stack"))
	}
}

func panicsHere() {
	panic(errors.New("boom"))
}

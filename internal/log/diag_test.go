package log

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

// crashChildEnv names the crash a child process of TestCrashReachesTheLogFile
// is to have. Set only in that child.
const crashChildEnv = "CFMLEDITOR_LSP_TEST_CRASH"

// TestCrashReachesTheLogFile is the reason SetCrashOutput is called. Neither
// crash passes through CapturePanic or any logger: a goroutine the server did
// not wrap in a recover, and a fatal error, which nothing can recover. Before,
// all either left was a traceback on stderr, which belongs to the client.
//
// Each runs in a child process, because each ends the process it happens in.
func TestCrashReachesTheLogFile(t *testing.T) {
	if mode := os.Getenv(crashChildEnv); mode != "" {
		crash(mode)

		return
	}

	// A fatal error's one-line reason goes to stderr only (see openLogFile), so
	// that case is identified by the frames of its traceback.
	cases := map[string]string{
		"goroutine-panic": "panic: boom from a goroutine",
		"fatal":           "internal/log.recurse",
	}

	for mode, want := range cases {
		t.Run(mode, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "lsp.log")

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCrashReachesTheLogFile$")

			cmd.Env = append(os.Environ(), crashChildEnv+"="+mode, FileEnv+"="+logPath)

			out, err := cmd.CombinedOutput()
			if _, crashed := errors.AsType[*exec.ExitError](err); !crashed {
				t.Fatalf("the child did not crash (err %v):\n%s", err, out)
			}

			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("reading the log file: %v", err)
			}

			if !strings.Contains(string(got), want) {
				t.Errorf("the log file does not hold %q; it holds:\n%s\nstderr was:\n%s", want, got, out)
			}

			if !strings.Contains(string(got), "goroutine ") {
				t.Errorf("the log file holds no traceback:\n%s", got)
			}
		})
	}
}

// crash opens the log file the way the server does, then has the crash named.
func crash(mode string) {
	NewLogger(false)

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

// TestPanicStackShowsWhereThePanicHappened pins what the recovered-panic
// records gain. The stack is read in the deferred function that recovered,
// before its frames are unwound, so it must still name the function that
// panicked; read anywhere later it would only name the recover site.
func TestPanicStackShowsWhereThePanicHappened(t *testing.T) {
	var stack string

	func() {
		defer func() {
			if r := recover(); r != nil {
				enc := zapcore.NewMapObjectEncoder()
				PanicStack().AddTo(enc)
				stack, _ = enc.Fields["stack"].(string)
			}
		}()

		panicsHere()
	}()

	if !strings.Contains(stack, "panicsHere") {
		t.Errorf("the stack does not name the function that panicked:\n%s", stack)
	}
}

func panicsHere() {
	panic(errors.New("boom"))
}

package log

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// TestNewLogger_BothModesProduceWorkingLoggers exercises NewLogger's real branch (debug ->
// zap.NewDevelopment, else zap.NewProduction) and confirms every Logger method can be called
// without panicking, regardless of which zap config was selected.
func TestNewLogger_BothModesProduceWorkingLoggers(t *testing.T) {
	for _, debug := range []bool{true, false} {
		l := NewLogger(debug)
		if l == nil {
			t.Fatalf("NewLogger(%v) returned nil", debug)
		}

		l.Debug("debug msg", String("k", "v"))
		l.Info("info msg", Int("n", 1))
		l.Warn("warn msg", Bool("b", true))
		l.Error("error msg", Err(nil))
	}
}

func TestFieldConstructors(_ *testing.T) {
	// These just need to not panic and to be usable as zap.Field values; the actual
	// encoding is zap's responsibility, not ours.
	_ = String("k", "v")
	_ = Strings("k", []string{"a", "b"})
	_ = Int("k", 1)
	_ = Bool("k", true)
	_ = Duration("k", 0)
	_ = Any("k", struct{}{})
	_ = Err(nil)
	_ = Uint32("k", 0)
}

// TestFatalf_ExitsWithStatus1 uses the standard Go pattern for testing os.Exit-calling code:
// re-exec this same test binary in a subprocess with an env var set, so Fatalf's os.Exit(1)
// terminates the subprocess (not the real test run), and assert on the subprocess's exit code.
func TestFatalf_ExitsWithStatus1(t *testing.T) {
	if os.Getenv("CFMLEDITOR_LSP_TEST_FATALF") == "1" {
		Fatalf("boom %d", 42)

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalf_ExitsWithStatus1")

	cmd.Env = append(os.Environ(), "CFMLEDITOR_LSP_TEST_FATALF=1")

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected Fatalf to exit the process, got err=%v", err)
	}

	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}
}

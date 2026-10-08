package log

import "testing"

func TestGetenvFallsBackToTheLegacyName(t *testing.T) {
	t.Setenv(LegacyFileEnv, "/old.log")

	if got := Getenv(FileEnv, LegacyFileEnv); got != "/old.log" {
		t.Errorf("with only the legacy variable: %q", got)
	}

	t.Setenv(FileEnv, "/new.log")

	if got := Getenv(FileEnv, LegacyFileEnv); got != "/new.log" {
		t.Errorf("with both: %q, want the new one", got)
	}

	// Set but empty is a choice, not an absence.
	t.Setenv(FileEnv, "")

	if got := Getenv(FileEnv, LegacyFileEnv); got != "" {
		t.Errorf("with the new one set empty: %q", got)
	}
}

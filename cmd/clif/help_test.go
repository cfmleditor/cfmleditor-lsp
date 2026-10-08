package main

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestEverySubcommandHasUsage(t *testing.T) {
	for name, cmd := range subcommands() {
		if !strings.HasPrefix(cmd.usage, "usage: clif "+name+" ") {
			t.Errorf("%s: usage does not start with its own usage line:\n%s", name, cmd.usage)
		}
	}
}

// TestMainDispatchesOnlyThroughTheTable fails on a subcommand added to main's
// switch rather than to subcommands(), which is how one would skip --help.
func TestMainDispatchesOnlyThroughTheTable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	_, body, _ := strings.Cut(string(src), "func main()")
	body, _, _ = strings.Cut(body, "\n}\n")

	for _, m := range regexp.MustCompile(`case ([^:]+):`).FindAllStringSubmatch(body, -1) {
		for name := range strings.SplitSeq(m[1], ",") {
			name = strings.Trim(strings.TrimSpace(name), `"`)
			switch name {
			case "version", "--version", "-version", "-v", "help", "--help", "-h":
			default:
				t.Errorf("main dispatches %q itself; add it to subcommands() so it answers --help", name)
			}
		}
	}
}

func TestWantsHelp(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{".", "--help"}, true},
		{[]string{"--json", "."}, false},
		{[]string{}, false},
		{[]string{"--helpful"}, false},
	}

	for _, c := range cases {
		if got := wantsHelp(c.args); got != c.want {
			t.Errorf("wantsHelp(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

// TestEverySubcommandRejectsAnUnknownOption runs each subcommand in a child
// process, since rejecting one exits, and fails on any that takes a mistyped
// flag for a path.
func TestEverySubcommandRejectsAnUnknownOption(t *testing.T) {
	if args := os.Getenv("CLIF_CLI_TEST_ARGS"); args != "" {
		os.Args = append([]string{"clif"}, strings.Split(args, "\x1f")...)

		main()
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for name := range subcommands() {
		t.Run(name, func(t *testing.T) {
			// The bogus flag comes first, so a command that rejects it never
			// reaches the arguments after it.
			args := strings.Join([]string{name, "--bogus", "a", "1"}, "\x1f")

			cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestEverySubcommandRejectsAnUnknownOption$")

			cmd.Env = append(os.Environ(), "CLIF_CLI_TEST_ARGS="+args)

			out, err := cmd.CombinedOutput()

			var exit *exec.ExitError
			// clif cflint and clif suppressions keep 1 for findings (a rise,
			// for suppressions), so their usage errors exit 2.
			want := 1
			if name == "cflint" || name == "suppressions" {
				want = exitCFLintError
			}

			if !errors.As(err, &exit) || exit.ExitCode() != want {
				t.Fatalf("exit = %v, want status %d; output:\n%s", err, want, out)
			}

			if !strings.Contains(string(out), `unknown option "--bogus"`) {
				t.Errorf("output does not name the option:\n%s", out)
			}
		})
	}
}

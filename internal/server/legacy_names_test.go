package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// TestLegacyCommandNamesStillRun: every command was cfmleditor.X before the
// rename, and the cfmleditor extension and users' keybindings still send that.
func TestLegacyCommandNamesStillRun(t *testing.T) {
	srv := newTestServer()

	for _, name := range []string{"resolveRoute", "showResolvers"} {
		run := func(command string) (any, error) {
			req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{Command: command})

			return srv.handleExecuteCommand(context.Background(), req)
		}

		current, currentErr := run("clif." + name)
		legacy, legacyErr := run("cfmleditor." + name)

		if legacyErr != nil && strings.Contains(legacyErr.Error(), "unknown command") {
			t.Fatalf("cfmleditor.%s is an unknown command", name)
		}

		if !reflect.DeepEqual(current, legacy) || (currentErr == nil) != (legacyErr == nil) {
			t.Errorf("%s: clif. gave %v, %v; cfmleditor. gave %v, %v", name, current, currentErr, legacy, legacyErr)
		}
	}
}

// TestOnlyTheNewCommandNamesAreAdvertised: a command picker lists what is
// advertised, and listing both names would show every command twice.
func TestOnlyTheNewCommandNamesAreAdvertised(t *testing.T) {
	cmds := newTestServer().capabilities().ExecuteCommandProvider.Commands
	if len(cmds) == 0 {
		t.Fatal("no commands advertised")
	}

	for _, c := range cmds {
		if !strings.HasPrefix(c, "clif.") {
			t.Errorf("advertised %q; want only clif. names", c)
		}
	}
}

// TestAClifConfigOutranksALegacyOne: a directory holding both is configured by
// .clif.json, and one holding only .cfmleditor.json still is.
func TestAClifConfigOutranksALegacyOne(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	legacy := filepath.Join(dir, ".cfmleditor.json")
	write(t, legacy, `{"workspaceName":"old"}`)

	srv := newTestServer()
	if p, cfg := srv.findConfigUpwards(dir); cfg == nil || p != legacy {
		t.Fatalf("with only the legacy file, found %q; want %q", p, legacy)
	}

	current := filepath.Join(dir, ".clif.json")
	write(t, current, `{"workspaceName":"new"}`)

	if p, cfg := srv.findConfigUpwards(dir); cfg == nil || p != current {
		t.Errorf("with both, found %q; want %q", p, current)
	}

	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}

	if p, _ := srv.findConfigUpwards(dir); p != current {
		t.Errorf("with only .clif.json, found %q", p)
	}
}

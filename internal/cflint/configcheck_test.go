package cflint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCheckConfigsRejectsWhatCFLintIgnores: each broken form was checked
// against CFLint 1.5.17, which lints with its default rules, says nothing and
// exits 0 for every one of them; each valid form it applied.
func TestCheckConfigsRejectsWhatCFLintIgnores(t *testing.T) {
	cases := []struct {
		name, rc string
		ok       bool
	}{
		{"valid", `{"includes":[{"code":"AVOID_USING_CFDUMP_TAG"}],"inheritParent":false}`, true},
		{"unknown key", `{"includes":[],"bogus":1}`, true},
		{"byte-order mark", "\xef\xbb\xbf{\"includes\":[]}", true},
		{"line comment", "{ // c\n\"includes\":[]}", false},
		{"block comment", `{ /* c */ "includes":[]}`, false},
		{"trailing comma", `{"includes":[],}`, false},
		{"single quotes", `{'includes':[]}`, false},
		{"unquoted keys", `{includes:[]}`, false},
		{"empty", ``, false},
		{"truncated", `{"includes":[{"code":"X"}`, false},
	}

	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".cflintrc"), c.rc)
		writeFile(t, filepath.Join(dir, "a.cfm"), "x")

		err := CheckConfigs([]string{filepath.Join(dir, "a.cfm")})
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}

		if err != nil && !strings.Contains(err.Error(), filepath.Join(dir, ".cflintrc")) {
			t.Errorf("%s: the error does not name the file: %v", c.name, err)
		}
	}
}

// TestCheckConfigsReadsWhatCFLintReads: CFLint walks up from a file's folder
// and stops after a .cflintrc setting "inheritParent": false, so a broken file
// above that is never read and is not an error; below it, or with
// inheritParent true or unset, it is read and is one.
func TestCheckConfigsReadsWhatCFLintReads(t *testing.T) {
	cases := []struct {
		child string
		ok    bool
	}{
		{`{"includes":[],"inheritParent":false}`, true},
		{`{"includes":[],"inheritParent":true}`, false},
		{`{"includes":[]}`, false},
		{"", false}, // no .cflintrc in the folder: the parent's is read
	}

	for _, c := range cases {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".cflintrc"), `{`)

		file := filepath.Join(root, "sub", "deeper", "a.cfm")
		writeFile(t, file, "x")

		if c.child != "" {
			writeFile(t, filepath.Join(root, "sub", ".cflintrc"), c.child)
		}

		if err := CheckConfigs([]string{file}); (err == nil) != c.ok {
			t.Errorf("child %q under a broken parent: err = %v, want ok=%v", c.child, err, c.ok)
		}
	}
}

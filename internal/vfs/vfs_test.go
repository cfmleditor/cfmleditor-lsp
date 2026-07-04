package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

// compile-time check that OS satisfies FS.
var _ FS = OS{}

func TestOS_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")

	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := (OS{}).ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "hello" {
		t.Errorf("ReadFile content = %q, want %q", got, "hello")
	}

	if _, err := (OS{}).ReadFile(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("expected an error reading a nonexistent file")
	}
}

func TestOS_Stat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")

	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := (OS{}).Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if info.Size() != 5 {
		t.Errorf("Stat size = %d, want 5", info.Size())
	}

	if _, err := (OS{}).Stat(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("expected an error stat-ing a nonexistent file")
	}
}

func TestOS_ReadDir(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := (OS{}).ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("ReadDir returned %d entries, want 2", len(entries))
	}
}

func TestOS_Walk(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var visited []string

	err := (OS{}).Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			visited = append(visited, filepath.Base(path))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(visited) != 1 || visited[0] != "nested.txt" {
		t.Errorf("Walk visited = %v, want [nested.txt]", visited)
	}
}

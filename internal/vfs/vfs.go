// Package vfs defines a filesystem interface for portability across native and WASM builds.
package vfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FS abstracts filesystem operations.
type FS interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Walk(root string, fn filepath.WalkFunc) error
}

// OS is the native filesystem implementation using the os package.
type OS struct{}

// ReadFile reads the named file.
func (OS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Stat returns file info for the named path.
func (OS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// ReadDir reads the named directory.
func (OS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

// Walk walks the file tree rooted at root.
func (OS) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

package frameworkapi

import (
	"io/fs"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// overlay serves the stubs under Root and passes every other path to base.
type overlay struct{ base vfs.FS }

// Wrap is base with the stubs mounted at Root. Wrapping twice is harmless.
func Wrap(base vfs.FS) vfs.FS {
	if o, ok := base.(overlay); ok {
		return o
	}

	return overlay{base: base}
}

func (o overlay) ReadFile(p string) ([]byte, error) {
	if e, ok := embedded(p); ok {
		return stubs.ReadFile(e)
	}

	return o.base.ReadFile(p)
}

func (o overlay) Stat(p string) (fs.FileInfo, error) {
	if e, ok := embedded(p); ok {
		return fs.Stat(stubs, e)
	}

	return o.base.Stat(p)
}

func (o overlay) ReadDir(p string) ([]fs.DirEntry, error) {
	if e, ok := embedded(p); ok {
		return fs.ReadDir(stubs, e)
	}

	return o.base.ReadDir(p)
}

func (o overlay) Walk(root string, fn filepath.WalkFunc) error {
	e, ok := embedded(root)
	if !ok {
		return o.base.Walk(root, fn)
	}

	return fs.WalkDir(stubs, e, func(p string, d fs.DirEntry, err error) error {
		full := filepath.Join(Root, filepath.FromSlash(p[len("stubs"):]))
		if err != nil {
			return fn(full, nil, err)
		}

		info, err := d.Info()

		return fn(full, info, err)
	})
}
